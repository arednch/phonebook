package server

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/arednch/phonebook/configuration"
	"github.com/arednch/phonebook/data"
	"github.com/arednch/phonebook/exporter"
)

type ReloadFunc func(source, olsrFile, sysInfoURL string, debug bool) error

type Server struct {
	Config     *configuration.Config
	ConfigPath string // optional when using config file

	Records   *data.Records
	Exporters map[string]exporter.Exporter

	ReloadFn ReloadFunc
}

func (s *Server) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	var cfg *configuration.Config
	if s.ConfigPath == "" {
		fmt.Fprintln(w, "phonebook was not started with a config path set ('-conf' flag) so config file won't be updated")
	} else {
		var err error
		if cfg, err = configuration.ReadFromJSON(s.ConfigPath); err != nil {
			if s.Config.Debug {
				fmt.Printf("/config: unable to read config: %s\n", err)
			}
			http.Error(w, "unable to read config", http.StatusInternalServerError)
			return
		}
		fmt.Fprintln(w, "phonebook config changes will be reflected in", s.ConfigPath)
	}

	src := r.FormValue("source")
	if src != "" {
		s.Config.Source = src
		fmt.Fprintf(w, "- source now set (but not validated!): %q\n", src)
		if s.Config.Debug {
			fmt.Printf("/config: source now set (but not validated): %q\n", src)
		}
	}

	if cfg != nil {
		if err := configuration.WriteToJSON(cfg, s.ConfigPath); err != nil {
			if s.Config.Debug {
				fmt.Printf("/config: unable to write config: %s\n", err)
			}
			http.Error(w, "unable to write config", http.StatusInternalServerError)
			return
		}

		fmt.Fprintf(w, "phonebook config updated in %q\n", s.ConfigPath)
		if s.Config.Debug {
			fmt.Printf("/config: phonebook config updated in %q\n", s.ConfigPath)
		}
	} else {
		fmt.Fprintln(w, "only phonebook runtime (!) config updated")
		if s.Config.Debug {
			fmt.Println("/config: only phonebook runtime (!) config updated")
		}
	}
}

func (s *Server) ReloadPhonebook(w http.ResponseWriter, r *http.Request) {
	if err := s.ReloadFn(s.Config.Source, s.Config.OLSRFile, s.Config.SysInfoURL, s.Config.Debug); err != nil {
		if s.Config.Debug {
			fmt.Printf("/reload: unable to reload phonebook: %s\n", err)
		}
		http.Error(w, "unable to reload phonebook", http.StatusInternalServerError)
		return
	}
	fmt.Fprintf(w, "phonebook reloaded locally from %q", s.Config.Source)
	if s.Config.Debug {
		fmt.Printf("/reload: phonebook reloaded locally from %q\n", s.Config.Source)
	}
}

func (s *Server) ServePhonebook(w http.ResponseWriter, r *http.Request) {
	f := r.FormValue("format")
	if f == "" {
		if s.Config.Debug {
			fmt.Printf("/phonebook: 'format' not specified: %+v\n", r)
		}
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
		if s.Config.Debug {
			fmt.Printf("/phonebook: 'format' %q not as expected: %+v\n", f, r)
		}
		http.Error(w, "'format' must be specified: [direct,pbx,combined]", http.StatusBadRequest)
		return
	}

	target := r.FormValue("target")
	if target == "" {
		if s.Config.Debug {
			fmt.Printf("/phonebook: 'target' not specified: %+v\n", r)
		}
		http.Error(w, "'target' must be specified: [generic,cisco,snom,yealink,grandstream]", http.StatusBadRequest)
		return
	}
	outTgt := strings.ToLower(strings.TrimSpace(target))
	exp, ok := s.Exporters[outTgt]
	if !ok {
		if s.Config.Debug {
			fmt.Printf("/phonebook: 'target' %q unknown: %+v\n", target, r)
		}
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

	body, err := exp.Export(s.Records.Entries, format, s.Config.ActivePfx, resolve, indicateActive, filterInactive, s.Config.Debug)
	if err != nil {
		if s.Config.Debug {
			fmt.Printf("/phonebook: export failed: %s\n", err)
		}
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	io.WriteString(w, string(body))
}
