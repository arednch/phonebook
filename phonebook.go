package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
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
	"github.com/arednch/phonebook/sip"
)

var (
	// Generally applicable flags.
	conf            = flag.String("conf", "", "OpenWRT UCI tree path to read config from instead of parsing flags.")
	source          = flag.String("source", "", "Path or URL to fetch the phonebook CSV from.")
	olsrFile        = flag.String("olsr", "/tmp/run/hosts_olsr", "Path to the OLSR hosts file.")
	sysInfoURL      = flag.String("sysinfo", "", "URL of sysinfo JSON API. Usually: http://localnode.local.mesh/cgi-bin/sysinfo.json?hosts=1")
	daemonize       = flag.Bool("server", false, "Phonebook acts as a server when set to true.")
	ldapServer      = flag.Bool("ldap_server", false, "Phonebook also runs an LDAP server when in server mode.")
	sipServer       = flag.Bool("sip_server", false, "Phonebook also runs a SIP server when in server mode.")
	debug           = flag.Bool("debug", false, "Turns on verbose logging to stdout when set to true.")
	allowRtCfgChg   = flag.Bool("allow_runtime_config_changes", false, "Allows runtime config changes via web server when set to true.")
	allowPermCfgChg = flag.Bool("allow_permanent_config_changes", false, "Allows permanent config changes via web server when set to true.")
	includeRoutable = flag.Bool("include_routable", false, "Also include routable phone numbers not in the phonebook.")

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
	webUser  = flag.String("web_user", "", "Username to protect many of the web endpoints with (BasicAuth). Default: None")
	webPwd   = flag.String("web_pwd", "", "Password to protect many of the web endpoints with (BasicAuth). Default: None")
	ldapPort = flag.Int("ldap_port", 3890, "Port to listen on for the LDAP server (when running as a server AND LDAP server is on as well).")
	ldapUser = flag.String("ldap_user", "aredn", "Username to provide to connect to the LDAP server.")
	ldapPwd  = flag.String("ldap_pwd", "aredn", "Password to provide to connect to the LDAP server.")
	sipPort  = flag.Int("sip_port", 5060, "Port to listen on for SIP traffic (when running as a server AND SIP server is on as well).")
)

const (
	defaultExtension = ".xml"
)

var (
	records   *data.Records
	exporters map[string]exporter.Exporter

	extensions = map[string]string{
		"vcard": ".vcf",
	}
	ignoredIdentityPfxs = []string{
		"127.0.0.",
		"fe80::",
		"::1",
	}
)

func mergePhonebookWithRouting(records []*data.Entry, hostData map[string]*data.OLSR, cfg *configuration.Config) []*data.Entry {
	addedOLSR := make(map[string]bool)
	// First find all phonebook entries with an OLSR entry.
	for _, e := range records {
		hostname := strings.Split(e.PhoneNumber, ".")[0]
		o, ok := hostData[hostname]
		if ok {
			e.OLSR = o
			addedOLSR[hostname] = true
			continue
		}
	}
	if cfg.Debug {
		fmt.Printf("Merged phonebook with routing data. Found %d matches for %d entries in %d known hosts.\n", len(records), len(addedOLSR), len(hostData))
	}
	if !cfg.IncludeRoutable {
		return records
	}

	// Then find the OLSR entries which have no phonebook entry and create one for them if configured to do so.
	var routableEntries []*data.Entry
	for hn, o := range hostData {
		if _, ok := addedOLSR[hn]; ok {
			continue // ignore entries which are already covered by the phonebook
		}
		if _, err := strconv.Atoi(hn); err != nil {
			continue // ignore entries which do not seem to be a phonenumber
		}
		if cfg.Debug {
			fmt.Printf("  - adding routable entry %s (%s)\n", o.Hostname, o.IP)
		}
		routableEntries = append(routableEntries, data.NewEntryFromOLSR(o))
	}
	if cfg.Debug {
		fmt.Printf("Merged added another %d routable entries.\n", len(routableEntries))
	}

	return append(records, routableEntries...)
}

func refreshRecords(cfg *configuration.Config) error {
	if cfg.Debug {
		fmt.Printf("Reading phonebook from %q\n", cfg.Source)
	}
	rec, err := importer.ReadPhonebook(cfg.Source)
	if err != nil {
		return fmt.Errorf("error reading phonebook: %s", err)
	}

	var hostData map[string]*data.OLSR
	switch {
	case cfg.OLSRFile == "" && cfg.SysInfoURL == "":
		fmt.Println("not reading network information: neither OLSR file nor sysinfo URL specified")
		return nil

	case cfg.SysInfoURL != "":
		hostData, err = olsr.ReadFromURL(cfg.SysInfoURL)
		if err != nil {
			return fmt.Errorf("error reading OLSR data from sysinfo URL %q: %s", cfg.SysInfoURL, err)
		}

	case cfg.OLSRFile != "":
		if _, err := os.Stat(cfg.OLSRFile); err != nil {
			fmt.Printf("not reading network information: OLSR file %q does not exist\n", cfg.OLSRFile)
			return nil
		}
		hostData, err = olsr.ReadFromFile(cfg.OLSRFile)
		if err != nil {
			return fmt.Errorf("error reading OLSR data from file %q: %s", cfg.OLSRFile, err)
		}
	}

	rec = mergePhonebookWithRouting(rec, hostData, cfg)

	records.Mu.Lock()
	defer records.Mu.Unlock()
	records.Entries = rec

	return nil
}

func exportOnce(cfg *configuration.Config) error {
	records.Mu.RLock()
	defer records.Mu.RUnlock()
	sort.Sort(data.ByName(records.Entries))

	for _, outTgt := range cfg.Targets {
		if cfg.Debug {
			fmt.Printf("Exporting for target %q\n", outTgt)
		}
		outTgt := strings.ToLower(strings.TrimSpace(outTgt))
		exp, ok := exporters[outTgt]
		if !ok {
			return fmt.Errorf("unknown target %q", outTgt)
		}

		ext, ok := extensions[outTgt]
		if !ok {
			ext = defaultExtension
		}

		for _, outFmt := range cfg.Formats {
			if cfg.Debug {
				fmt.Printf("Exporting for format %q\n", outFmt)
			}
			switch strings.ToLower(strings.TrimSpace(outFmt)) {
			case "d", "direct": // Direct calling phonebook.
				body, err := exp.Export(records.Entries, exporter.FormatDirect, cfg.ActivePfx, cfg.Resolve, cfg.IndicateActive, cfg.FilterInactive, cfg.Debug)
				if err != nil {
					return err
				}
				outpath := filepath.Join(cfg.Path, fmt.Sprintf("phonebook_%s_direct%s", outTgt, ext))
				os.WriteFile(outpath, body, 0644)
			case "p", "pbx": // PBX calling phonebook.
				body, err := exp.Export(records.Entries, exporter.FormatPBX, cfg.ActivePfx, cfg.Resolve, cfg.IndicateActive, cfg.FilterInactive, cfg.Debug)
				if err != nil {
					return err
				}
				outpath := filepath.Join(cfg.Path, fmt.Sprintf("phonebook_%s_pbx%s", outTgt, ext))
				os.WriteFile(outpath, body, 0644)
			case "c", "combined":
				body, err := exp.Export(records.Entries, exporter.FormatCombined, cfg.ActivePfx, cfg.Resolve, cfg.IndicateActive, cfg.FilterInactive, cfg.Debug)
				if err != nil {
					return err
				}
				outpath := filepath.Join(cfg.Path, fmt.Sprintf("phonebook_%s_combined%s", outTgt, ext))
				os.WriteFile(outpath, body, 0644)
			default:
				return fmt.Errorf("unknown format: %q", outFmt)
			}
		}
	}

	return nil
}

func ignoreIdentityPfx(id string) bool {
	for _, pfx := range ignoredIdentityPfxs {
		if strings.HasPrefix(id, pfx) {
			return true
		}
	}
	return false
}

func getLocalIdentities() (map[string]bool, error) {
	identities := map[string]bool{
		data.AREDNLocalNode: true,
	}

	if hn, err := os.Hostname(); err != nil {
		return nil, fmt.Errorf("unable to look up hostname: %s", err)
	} else {
		hn = strings.ToLower(hn)
		hn = strings.Trim(hn, ".")
		if !ignoreIdentityPfx(hn) {
			identities[hn] = true
			if !strings.HasSuffix(hn, data.AREDNDomain) {
				identities[fmt.Sprintf("%s.%s", hn, data.AREDNDomain)] = true
			}
		}
	}

	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("unable to look up interfaces: %s", err)
	}
	for _, i := range ifaces {
		addrs, err := i.Addrs()
		if err != nil {
			return nil, fmt.Errorf("unable to look up addresses for interface %s: %s", i.Name, err)
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if !ignoreIdentityPfx(ip.String()) {
				identities[ip.String()] = true
			}
		}
	}

	return identities, nil
}

func runServer(ctx context.Context, cfg *configuration.Config, cfgPath string) error {
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
				fmt.Printf("LDAP server failed: %s\n", err)
			}
		}()
	}

	if cfg.SIPServer {
		identities, err := getLocalIdentities()
		if err != nil {
			identities = nil
			if cfg.Debug {
				fmt.Printf("unable to look up local identities, using empty set: %s\n", err)
			}
		} else if cfg.Debug {
			fmt.Println("using local SIP identities:")
			for k := range identities {
				fmt.Printf("  - %s\n", k)
			}
		}
		s := &sip.Server{
			Config:          cfg,
			Records:         records,
			RegisterCache:   data.NewTTL[string, *data.SIPClient](),
			LocalIdentities: identities,
		}

		go func() {
			if cfg.Debug {
				fmt.Println("Starting SIP Listener")
			}
			if err := s.ListenAndServe(ctx, "udp", fmt.Sprintf(":%d", cfg.SIPPort)); err != nil {
				fmt.Printf("SIP server failed: %s\n", err)
			}
		}()
	}

	go func() {
		for {
			if err := refreshRecords(cfg); err != nil {
				fmt.Printf("error refreshing data from upstream: %s\n", err)
			}
			time.Sleep(cfg.Reload)
		}
	}()

	srv := &server.Server{
		Config:     cfg,
		ConfigPath: cfgPath,
		Records:    records,
		Exporters:  exporters,
		ReloadFn:   refreshRecords,
	}
	http.HandleFunc("/phonebook", srv.ServePhonebook)
	if cfg.WebUser != "" && cfg.WebPwd != "" {
		if cfg.Debug {
			fmt.Println("protecting most web endpoints with configured basicAuth user/pwd")
		}
		http.HandleFunc("/reload", srv.BasicAuth(srv.ReloadPhonebook))
		http.HandleFunc("/showconfig", srv.BasicAuth(srv.ShowConfig))
		http.HandleFunc("/updateconfig", srv.BasicAuth(srv.UpdateConfig))
	} else {
		if cfg.Debug {
			fmt.Println("not protecting any of the web endpoints with basicAuth as not both user/pwd were set")
		}
		http.HandleFunc("/reload", srv.ReloadPhonebook)
		http.HandleFunc("/showconfig", srv.ShowConfig)
		http.HandleFunc("/updateconfig", srv.UpdateConfig)
	}
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.Port))
	if err != nil {
		return err
	}

	return http.Serve(listener, nil)
}

func runLocal(cfg *configuration.Config) error {
	if err := refreshRecords(cfg); err != nil {
		return err
	}
	if err := exportOnce(cfg); err != nil {
		return err
	}

	return nil
}

func main() {
	ctx := context.Background()
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
		if c, err := configuration.ReadFromJSON(*conf); err != nil {
			fmt.Printf("unable to read config: %s\n", err)
			os.Exit(1)
		} else {
			c.Reload = time.Duration(c.ReloadSeconds) * time.Second
			cfg = c
		}
	} else {
		cfg = &configuration.Config{
			Source:                      *source,
			OLSRFile:                    *olsrFile,
			SysInfoURL:                  *sysInfoURL,
			Server:                      *daemonize,
			LDAPServer:                  *ldapServer,
			SIPServer:                   *sipServer,
			Debug:                       *debug,
			AllowRuntimeConfigChanges:   *allowRtCfgChg,
			AllowPermanentConfigChanges: *allowPermCfgChg,
			Path:                        *path,
			Formats:                     strings.Split(*formats, ","),
			Targets:                     strings.Split(*targets, ","),
			Resolve:                     *resolve,
			IndicateActive:              *indicateActive,
			FilterInactive:              *filterInactive,
			ActivePfx:                   *activePfx,
			IncludeRoutable:             *includeRoutable,
			Port:                        *port,
			Reload:                      *reload,
			WebUser:                     *webUser,
			WebPwd:                      *webPwd,
			LDAPPort:                    *ldapPort,
			LDAPUser:                    *ldapUser,
			LDAPPwd:                     *ldapPwd,
			SIPPort:                     *sipPort,
		}
	}
	// Detect when flag is set to run as a server even when reading config.
	if *daemonize {
		cfg.Server = *daemonize
	}

	if cfg.Source == "" {
		fmt.Println("source needs to be set")
		os.Exit(1)
	}

	if cfg.Server {
		// Validation only relevant for server.
		if cfg.Reload.Seconds() < configuration.MinimalReloadSeconds {
			fmt.Printf("reload config/flag too low (<%d): %d\n", configuration.MinimalReloadSeconds, int(cfg.Reload.Seconds()))
			os.Exit(1)
		}
		if cfg.Reload.Seconds() > configuration.MaxReloadSeconds {
			fmt.Printf("reload config/flag too high (>%d): %d\n", configuration.MaxReloadSeconds, int(cfg.Reload.Seconds()))
			os.Exit(1)
		}

		if *debug {
			fmt.Println("Running phonebook in server mode")
		}
		if err := runServer(ctx, cfg, *conf); err != nil {
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
