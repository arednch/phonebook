package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/arednch/phonebook/configuration"
	"github.com/arednch/phonebook/data"
	"github.com/arednch/phonebook/exporter"
	"github.com/arednch/phonebook/importer"
	"github.com/arednch/phonebook/olsr"
)

var (
	// Generally applicable flags.
	conf     = flag.String("conf", "", "Config file to read settings from instead of parsing flags.")
	source   = flag.String("source", "", "Path or URL to fetch the phonebook CSV from.")
	olsrFile = flag.String("olsr", "/tmp/run/hosts_olsr.stable", "Path to the OLSR hosts file.")
	server   = flag.Bool("server", false, "Phonebook acts as a server when set to true.")

	// Only relevant when running in non-server / ad-hoc mode.
	path           = flag.String("path", "", "Folder to write the phonebooks to locally.")
	formats        = flag.String("formats", "combined", "Comma separated list of formats to export. Supported: pbx,direct,combined")
	targets        = flag.String("targets", "", "Comma separated list of targets to export. Supported: generic,yealink,cisco,snom")
	resolve        = flag.Bool("resolve", false, "Resolve hostnames to IPs when set to true using OSLR data.")
	indicateActive = flag.Bool("indicate_active", false, "Prefixes active participants in the phonebook with -active_pfx.")
	filterInactive = flag.Bool("filter_inactive", false, "Filters inactive participants to not show in the phonebook.")
	activePfx      = flag.String("active_pfx", "*", "Prefix to add when -indicate_active is set.")

	// Only relevant when running in server mode.
	port   = flag.Int("port", 8080, "Port to listen on (when running as a server).")
	reload = flag.Duration("reload", time.Hour, "Duration after which to try to reload the phonebook source.")
)

const (
	sipSeparator = "@"
)

var (
	recordsMu *sync.RWMutex
	records   []*data.Entry

	exporters map[string]exporter.Exporter
)

func refreshRecords(source, olsrFile string) error {
	rec, err := importer.ReadPhonebook(source)
	if err != nil {
		return err
	}

	if _, err := os.Stat(olsrFile); err == nil {
		oslrData, err := olsr.Read(olsrFile)
		if err != nil {
			return err
		}
		for _, e := range rec {
			addrParts := strings.Split(e.IPAddress, sipSeparator)
			if len(addrParts) != 2 {
				continue
			}
			hostname := addrParts[1]
			o, ok := oslrData[strings.Split(hostname, ".")[0]]
			if !ok {
				continue
			}
			e.OLSR = o
		}
	}

	recordsMu.Lock()
	defer recordsMu.Unlock()
	records = rec

	return nil
}

type Server struct {
	Config *configuration.Config
}

func (s *Server) ServePhonebook(w http.ResponseWriter, r *http.Request) {
	f := r.FormValue("format")
	if f == "" {
		http.Error(w, "'format' must be specified: [direct,pbx,combined]", http.StatusBadRequest)
		return
	}
	var format exporter.Format
	switch strings.ToLower(strings.TrimSpace(f)) {
	case "d", "direct":
		format = exporter.FormatDirect
	case "p", "pbx":
		format = exporter.FormatPBX
	case "c", "combined":
		format = exporter.FormatCombined
	default:
		http.Error(w, "'format' must be specified: [direct,pbx,combined]", http.StatusBadRequest)
		return
	}

	target := r.FormValue("target")
	if target == "" {
		http.Error(w, "'target' must be specified: [generic,cisco,snom,yealink]", http.StatusBadRequest)
		return
	}
	outTgt := strings.ToLower(strings.TrimSpace(target))
	exp, ok := exporters[outTgt]
	if !ok {
		http.Error(w, "Unknown target.", http.StatusBadRequest)
		return
	}

	var resolve bool
	res := r.FormValue("resolve")
	if strings.ToLower(strings.TrimSpace(res)) == "true" {
		resolve = true
	}

	var indicateActive bool
	ia := r.FormValue("ia")
	if strings.ToLower(strings.TrimSpace(ia)) == "true" {
		indicateActive = true
	}

	var filterInactive bool
	fi := r.FormValue("fi")
	if strings.ToLower(strings.TrimSpace(fi)) == "true" {
		filterInactive = true
	}

	body, err := exp.Export(records, format, s.Config.ActivePfx, resolve, indicateActive, filterInactive)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	io.WriteString(w, string(body))
}

func exportOnce(source, path, activePfx string, formats, targets []string, resolve, indicateActive, filterInactive bool) error {
	for _, outTgt := range targets {
		outTgt := strings.ToLower(strings.TrimSpace(outTgt))
		exp, ok := exporters[outTgt]
		if !ok {
			return fmt.Errorf("unknown target %q", outTgt)
		}

		for _, outFmt := range formats {
			switch strings.ToLower(strings.TrimSpace(outFmt)) {
			case "d", "direct": // Direct calling phonebook.
				body, err := exp.Export(records, exporter.FormatDirect, activePfx, resolve, indicateActive, filterInactive)
				if err != nil {
					return err
				}
				outpath := filepath.Join(path, fmt.Sprintf("phonebook_%s_direct.xml", outTgt))
				os.WriteFile(outpath, body, 0644)
			case "p", "pbx": // PBX calling phonebook.
				body, err := exp.Export(records, exporter.FormatPBX, activePfx, resolve, indicateActive, filterInactive)
				if err != nil {
					return err
				}
				outpath := filepath.Join(path, fmt.Sprintf("phonebook_%s_pbx.xml", outTgt))
				os.WriteFile(outpath, body, 0644)
			case "c", "combined":
				body, err := exp.Export(records, exporter.FormatCombined, activePfx, resolve, indicateActive, filterInactive)
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

	go func() {
		for {
			if err := refreshRecords(cfg.Source, cfg.OLSRFile); err != nil {
				fmt.Printf("error refreshing data from upstream: %s\n", err)
			}
			time.Sleep(cfg.Reload)
		}
	}()

	srv := Server{cfg}
	http.HandleFunc("/phonebook", srv.ServePhonebook)
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.Port))
	if err != nil {
		return err
	}
	return http.Serve(listener, nil)
}

func runLocal(cfg *configuration.Config) error {
	if err := refreshRecords(cfg.Source, cfg.OLSRFile); err != nil {
		return err
	}
	if err := exportOnce(cfg.Source, cfg.Path, cfg.ActivePfx, cfg.Formats, cfg.Targets, cfg.Resolve, cfg.IndicateActive, cfg.FilterInactive); err != nil {
		return err
	}

	return nil
}

func main() {
	// Parse flags globally.
	flag.Parse()
	recordsMu = &sync.RWMutex{}
	exporters = map[string]exporter.Exporter{
		"generic": &exporter.Generic{},
		"cisco":   &exporter.Cisco{},
		"yealink": &exporter.Yealink{},
		"snom":    &exporter.Snom{},
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
			Server:         *server,
			Path:           *path,
			Formats:        strings.Split(*formats, ","),
			Targets:        strings.Split(*targets, ","),
			Resolve:        *resolve,
			IndicateActive: *indicateActive,
			FilterInactive: *filterInactive,
			ActivePfx:      *activePfx,
			Port:           *port,
			Reload:         *reload,
		}
	}

	if cfg.Source == "" {
		fmt.Println("source needs to be set")
		os.Exit(1)
	}

	if cfg.Server {
		if err := runServer(cfg); err != nil {
			fmt.Printf("unable to run server: %s\n", err)
			os.Exit(1)
		}
	} else {
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
