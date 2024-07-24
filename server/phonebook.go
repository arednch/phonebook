package server

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/arednch/phonebook/exporter"
)

func (s *Server) ServePhonebook(w http.ResponseWriter, r *http.Request) {
	f := r.FormValue("format")
	f = strings.ToLower(strings.TrimSpace(f))
	if f == "" {
		if s.Config.Debug {
			fmt.Printf("/phonebook: 'format' not specified: %+v\n", r)
		}
		http.Error(w, "'format' must be specified: [direct,pbx,combined]", http.StatusBadRequest)
		return
	}
	var format exporter.Format
	switch f {
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
	target = strings.ToLower(strings.TrimSpace(target))
	if target == "" {
		if s.Config.Debug {
			fmt.Printf("/phonebook: 'target' not specified: %+v\n", r)
		}
		http.Error(w, "'target' must be specified: [generic,cisco,snom,yealink,grandstream,vcard]", http.StatusBadRequest)
		return
	}
	exp, ok := s.Exporters[target]
	if !ok {
		if s.Config.Debug {
			fmt.Printf("/phonebook: 'target' %q unknown: %+v\n", target, r)
		}
		http.Error(w, "Unknown target.", http.StatusBadRequest)
		return
	}

	var resolve bool
	res := r.FormValue("resolve")
	res = strings.ToLower(strings.TrimSpace(res))
	if res == "true" {
		resolve = true
	}

	var indicateActive bool
	ia := r.FormValue("ia")
	ia = strings.ToLower(strings.TrimSpace(ia))
	if ia == "true" {
		indicateActive = true
	}

	var filterInactive bool
	fi := r.FormValue("fi")
	fi = strings.ToLower(strings.TrimSpace(fi))
	if fi == "true" {
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
