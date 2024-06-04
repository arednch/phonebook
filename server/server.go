package server

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/arednch/phonebook/configuration"
	"github.com/arednch/phonebook/data"
	"github.com/arednch/phonebook/exporter"
	"github.com/digineo/go-uci"
)

type ReloadFunc func(source, olsrFile, sysInfoURL string, debug bool) error

type Server struct {
	Config     *configuration.Config
	ConfigPath string // optional when using UCI config

	Records   *data.Records
	Exporters map[string]exporter.Exporter

	ReloadFn ReloadFunc
}

func (s *Server) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	if s.ConfigPath == "" {
		http.Error(w, "phonebook was not started with a config path set ('-conf' flag)", http.StatusBadRequest)
		return
	}

	u := uci.NewTree(s.ConfigPath)
	if err := u.LoadConfig(configuration.UCIConfig, true); err != nil {
		if s.Config.Debug {
			fmt.Printf("/config: unable read config: %s\n", err)
		}
		http.Error(w, "unable to read config", http.StatusBadRequest)
		return
	}

	src := r.FormValue("source")
	if src != "" {
		if exist := u.SetType("phonebook", "main", "source", uci.TypeOption, src); !exist {
			if s.Config.Debug {
				fmt.Println("/config: unable to set 'source': section or file does not exist")
			}
			http.Error(w, "unable to set 'source': section or file does not exist", http.StatusBadRequest)
			return
		}
		s.Config.Source = src // reflecting change in loaded config to avoid having to restart
	}

	if err := u.Commit(); err != nil {
		if s.Config.Debug {
			fmt.Printf("/config: unable to commit config: %s\n", err)
		}
		http.Error(w, "unable to commit config", http.StatusBadRequest)
		return
	}

	fmt.Fprintf(w, "phonebook config updated in %q", s.ConfigPath)
	if s.Config.Debug {
		fmt.Printf("/config: phonebook config updated in %q\n", s.ConfigPath)
	}
}

func (s *Server) ReloadPhonebook(w http.ResponseWriter, r *http.Request) {
	if err := s.ReloadFn(s.Config.Source, s.Config.OLSRFile, s.Config.SysInfoURL, s.Config.Debug); err != nil {
		if s.Config.Debug {
			fmt.Printf("/reload: unable to reload phonebook: %s\n", err)
		}
		http.Error(w, "unable to reload phonebook", http.StatusBadRequest)
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
