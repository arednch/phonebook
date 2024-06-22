package server

import (
	"fmt"
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

func (s *Server) ShowConfig(w http.ResponseWriter, r *http.Request) {
	config, err := configuration.ConvertToJSON(*s.Config, true)
	if err != nil {
		if s.Config.Debug {
			fmt.Printf("/showconfig: unable to convert config: %s\n", err)
		}
		http.Error(w, "unable to convert config", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(config)
}

func (s *Server) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	var permanent bool
	perm := r.FormValue("perm")
	if strings.ToLower(strings.TrimSpace(perm)) == "true" {
		permanent = true
	}

	var changed bool
	var cfg *configuration.Config
	switch {
	case s.ConfigPath == "":
		fmt.Fprintln(w, "phonebook was not started with a config path set ('-conf' flag) so config file won't be updated")
	case !permanent:
		fmt.Fprintln(w, "phonebook config changes are not going to be written to disk")
	default:
		var err error
		if cfg, err = configuration.ReadFromJSON(s.ConfigPath); err != nil {
			if s.Config.Debug {
				fmt.Printf("/updateconfig: unable to read config: %s\n", err)
			}
			http.Error(w, "unable to read config", http.StatusInternalServerError)
			return
		}
		fmt.Fprintln(w, "phonebook config changes will be reflected in", s.ConfigPath)
	}

	// Check for supported fields to update.
	src := r.FormValue("source")
	if src != "" {
		changed = true
		s.Config.Source = src
		if cfg != nil {
			cfg.Source = src
		}
		fmt.Fprintf(w, "- \"source\" now set (but not validated!): %q\n", src)
		if s.Config.Debug {
			fmt.Printf("/updateconfig: \"source\" now set (but not validated): %q\n", src)
		}
	}

	// Exit early if we didn't make any changes (avoid unnecessary disk writes etc).
	if !changed {
		fmt.Fprintln(w, "no changes were made")
		if s.Config.Debug {
			fmt.Println("/updateconfig: no changes were made")
		}
		return
	}

	// Finally writing the changes if there are any.
	if cfg != nil {
		if err := configuration.WriteToJSON(cfg, s.ConfigPath, false); err != nil {
			if s.Config.Debug {
				fmt.Printf("/updateconfig: unable to write config: %s\n", err)
			}
			http.Error(w, "unable to write config", http.StatusInternalServerError)
			return
		}

		fmt.Fprintf(w, "phonebook config updated in %q\n", s.ConfigPath)
		if s.Config.Debug {
			fmt.Printf("/updateconfig: phonebook config updated in %q\n", s.ConfigPath)
		}
	} else {
		fmt.Fprintln(w, "only phonebook runtime (!) config updated")
		if s.Config.Debug {
			fmt.Println("/updateconfig: only phonebook runtime (!) config updated")
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
	w.WriteHeader(http.StatusOK)
	w.Write(body)
}
