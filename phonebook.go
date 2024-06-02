package main

import (
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/mark-rushakoff/ldapserver"

	"github.com/arednch/phonebook/configuration"
	"github.com/arednch/phonebook/data"
	"github.com/arednch/phonebook/exporter"
	"github.com/arednch/phonebook/importer"
	"github.com/arednch/phonebook/ldap"
	"github.com/arednch/phonebook/olsr"
	"github.com/arednch/phonebook/server"
)

var (
	// Generally applicable flags.
	conf       = flag.String("conf", "", "Config file to read settings from instead of parsing flags.")
	source     = flag.String("source", "", "Path or URL to fetch the phonebook CSV from.")
	olsrFile   = flag.String("olsr", "/tmp/run/hosts_olsr", "Path to the OLSR hosts file.")
	sysInfoURL = flag.String("sysinfo", "", "URL of sysinfo JSON API. Usually: http://localnode.local.mesh/cgi-bin/sysinfo.json?hosts=1")
	daemonize  = flag.Bool("server", false, "Phonebook acts as a server when set to true.")
	ldapServer = flag.Bool("ldap_server", false, "Phonebook also runs an LDAP server when in server mode.")
	debug      = flag.Bool("debug", false, "Turns on verbose logging to stdout when set to true.")

	// Only relevant when running in non-server / ad-hoc mode.
	path           = flag.String("path", "", "Folder to write the phonebooks to locally.")
	formats        = flag.String("formats", "combined", "Comma separated list of formats to export. Supported: pbx,direct,combined")
	targets        = flag.String("targets", "", "Comma separated list of targets to export. Supported: generic,yealink,cisco,snom,grandstream,vcard")
	resolve        = flag.Bool("resolve", false, "Resolve hostnames to IPs when set to true using OLSR data.")
	indicateActive = flag.Bool("indicate_active", false, "Prefixes active participants in the phonebook with -active_pfx.")
	filterInactive = flag.Bool("filter_inactive", false, "Filters inactive participants to not show in the phonebook.")
	activePfx      = flag.String("active_pfx", "*", "Prefix to add when -indicate_active is set.")

	// Only relevant when running in server mode.
	port     = flag.Int("port", 8080, "Port to listen on (when running as a server).")
	reload   = flag.Duration("reload", time.Hour, "Duration after which to try to reload the phonebook source.")
	ldapPort = flag.Int("ldap_port", 3890, "Port to listen on for the LDAP server (when running as a server AND LDAP server is on as well).")
	ldapUser = flag.String("ldap_user", "aredn", "Username to provide to connect to the LDAP server.")
	ldapPwd  = flag.String("ldap_pwd", "aredn", "Password to provide to connect to the LDAP server.")
)

const (
	sipSeparator = "@"
)

var (
	records *data.Records

	exporters map[string]exporter.Exporter
)

func refreshRecords(source, olsrFile, sysInfoURL string, debug bool) error {
	if debug {
		fmt.Printf("Reading phonebook from %q\n", source)
	}
	rec, err := importer.ReadPhonebook(source)
	if err != nil {
		return err
	}

	var hostData map[string]*data.OLSR
	switch {
	case olsrFile == "" && sysInfoURL == "":
		fmt.Println("not reading network information: neither OLSR file nor sysinfo URL specified")
		return nil

	case sysInfoURL != "":
		hostData, err = olsr.ReadFromURL(sysInfoURL)
		if err != nil {
			return err
		}

	case olsrFile != "":
		if _, err := os.Stat(olsrFile); err != nil {
			fmt.Printf("not reading network information: OLSR file %q does not exist\n", olsrFile)
			return nil
		}
		hostData, err = olsr.ReadFromFile(olsrFile)
		if err != nil {
			return err
		}
	}

	for _, e := range rec {
		addrParts := strings.Split(e.IPAddress, sipSeparator)
		if len(addrParts) != 2 {
			continue
		}
		hostname := addrParts[1]
		o, ok := hostData[strings.Split(hostname, ".")[0]]
		if !ok {
			continue
		}
		e.OLSR = o
	}

	records.Mu.Lock()
	defer records.Mu.Unlock()
	records.Entries = rec

	return nil
}

func exportOnce(path, activePfx string, formats, targets []string, resolve, indicateActive, filterInactive, debug bool) error {
	for _, outTgt := range targets {
		if debug {
			fmt.Printf("Exporting for target %q\n", outTgt)
		}
		outTgt := strings.ToLower(strings.TrimSpace(outTgt))
		exp, ok := exporters[outTgt]
		if !ok {
			return fmt.Errorf("unknown target %q", outTgt)
		}

		for _, outFmt := range formats {
			if debug {
				fmt.Printf("Exporting for format %q\n", outFmt)
			}
			switch strings.ToLower(strings.TrimSpace(outFmt)) {
			case "d", "direct": // Direct calling phonebook.
				body, err := exp.Export(records.Entries, exporter.FormatDirect, activePfx, resolve, indicateActive, filterInactive, debug)
				if err != nil {
					return err
				}
				outpath := filepath.Join(path, fmt.Sprintf("phonebook_%s_direct.xml", outTgt))
				os.WriteFile(outpath, body, 0644)
			case "p", "pbx": // PBX calling phonebook.
				body, err := exp.Export(records.Entries, exporter.FormatPBX, activePfx, resolve, indicateActive, filterInactive, debug)
				if err != nil {
					return err
				}
				outpath := filepath.Join(path, fmt.Sprintf("phonebook_%s_pbx.xml", outTgt))
				os.WriteFile(outpath, body, 0644)
			case "c", "combined":
				body, err := exp.Export(records.Entries, exporter.FormatCombined, activePfx, resolve, indicateActive, filterInactive, debug)
				if err != nil {
					return err
				}
				outpath := filepath.Join(path, fmt.Sprintf("phonebook_%s_combined.xml", outTgt))
				os.WriteFile(outpath, body, 0644)
			default:
				return fmt.Errorf("unknown format: %q", outFmt)
			}
		}
	}

	return nil
}

func runServer(cfg *configuration.Config) error {
	if cfg.Source == "" {
		return errors.New("source needs to be set")
	}

	if cfg.LDAPServer {
		ldapSrv := &ldap.Server{
			Config:  cfg,
			Records: records,
		}
		s := ldapserver.NewServer()
		s.Bind = ldapSrv.Bind
		s.Search = ldapSrv.Search

		go func() {
			if err := s.ListenAndServe(fmt.Sprintf(":%d", cfg.LDAPPort)); err != nil {
				fmt.Printf("LDAP server failed: %s", err)
			}
		}()
	}

	go func() {
		for {
			if err := refreshRecords(cfg.Source, cfg.OLSRFile, cfg.SysInfoURL, cfg.Debug); err != nil {
				fmt.Printf("error refreshing data from upstream: %s\n", err)
			}
			time.Sleep(cfg.Reload)
		}
	}()

	srv := &server.Server{
		Config:    cfg,
		Records:   records,
		Exporters: exporters,
	}
	http.HandleFunc("/phonebook", srv.ServePhonebook)
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.Port))
	if err != nil {
		return err
	}

	return http.Serve(listener, nil)
}

func runLocal(cfg *configuration.Config) error {
	if err := refreshRecords(cfg.Source, cfg.OLSRFile, cfg.SysInfoURL, cfg.Debug); err != nil {
		return err
	}
	if err := exportOnce(cfg.Path, cfg.ActivePfx, cfg.Formats, cfg.Targets, cfg.Resolve, cfg.IndicateActive, cfg.FilterInactive, cfg.Debug); err != nil {
		return err
	}

	return nil
}

func main() {
	// Parse flags globally.
	flag.Parse()
	records = &data.Records{
		Mu: &sync.RWMutex{},
	}
	exporters = map[string]exporter.Exporter{
		"generic":     &exporter.Generic{},
		"cisco":       &exporter.Cisco{},
		"yealink":     &exporter.Yealink{},
		"snom":        &exporter.Snom{},
		"grandstream": &exporter.Grandstream{},
		"vcard":       &exporter.VCard{},
	}

	var cfg *configuration.Config
	if *conf != "" {
		if c, err := configuration.Read(*conf); err != nil {
			fmt.Printf("unable to read config: %s\n", err)
			os.Exit(1)
		} else {
			c.Reload = time.Duration(c.ReloadSeconds) * time.Second
			cfg = c
		}
	} else {
		cfg = &configuration.Config{
			Source:         *source,
			OLSRFile:       *olsrFile,
			SysInfoURL:     *sysInfoURL,
			Server:         *daemonize,
			LDAPServer:     *ldapServer,
			Debug:          *debug,
			Path:           *path,
			Formats:        strings.Split(*formats, ","),
			Targets:        strings.Split(*targets, ","),
			Resolve:        *resolve,
			IndicateActive: *indicateActive,
			FilterInactive: *filterInactive,
			ActivePfx:      *activePfx,
			Port:           *port,
			Reload:         *reload,
			LDAPPort:       *ldapPort,
			LDAPUser:       *ldapUser,
			LDAPPwd:        *ldapPwd,
		}
	}

	if cfg.Source == "" {
		fmt.Println("source needs to be set")
		os.Exit(1)
	}

	if cfg.Server {
		if *debug {
			fmt.Println("Running phonebook in server mode")
		}
		if err := runServer(cfg); err != nil {
			fmt.Printf("unable to run server: %s\n", err)
			os.Exit(1)
		}
	} else {
		if *debug {
			fmt.Println("Running phonebook as a one-time export")
		}

		if cfg.Path == "" {
			fmt.Println("path needs to be set")
			os.Exit(1)
		}
		if len(cfg.Formats) == 0 {
			fmt.Println("formats need to be set")
			os.Exit(1)
		}
		if len(cfg.Targets) == 0 {
			fmt.Println("targets need to be set")
			os.Exit(1)
		}

		if err := runLocal(cfg); err != nil {
			fmt.Printf("unable to run: %s\n", err)
			os.Exit(1)
		}
	}
}
