package server

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/arednch/phonebook/configuration"
	"github.com/arednch/phonebook/data"
)

func (s *Server) ShowConfig(w http.ResponseWriter, r *http.Request) {
	data := data.WebShowConfig{
		WebDefault: *s.prepareDefaultData("Show Config", false),
		Success:    true,
	}

	t := r.FormValue("type")
	t = strings.ToLower(strings.TrimSpace(t))
	if t == "" {
		if s.Config.Debug {
			fmt.Printf("/showconfig: 'type' not specified: %+v\n", r)
		}
		data.Success = false
		data.Messages = append(data.Messages, "'type' must be specified: [disk,runtime,diff]")
		if err := s.Tmpls.ExecuteTemplate(w, "showconfig.html", data); err != nil {
			http.Error(w, "unable to write response", http.StatusInternalServerError)
		}
		return
	}

	var cfg *configuration.Config
	switch {
	case t == "d" || t == "disk" || t == "diff":
		if s.ConfigPath == "" {
			data.Success = false
			data.Messages = append(data.Messages, "phonebook was not started with a config path set ('-conf' flag) so config file can't be loaded")
			if err := s.Tmpls.ExecuteTemplate(w, "showconfig.html", data); err != nil {
				http.Error(w, "unable to write response", http.StatusInternalServerError)
			}
			return
		}
		var err error
		if cfg, err = configuration.ReadFromJSON(s.ConfigPath); err != nil {
			if s.Config.Debug {
				fmt.Printf("/showconfig: unable to read config: %s\n", err)
			}
			data.Success = false
			data.Messages = append(data.Messages, "unable to read config")
			if err := s.Tmpls.ExecuteTemplate(w, "showconfig.html", data); err != nil {
				http.Error(w, "unable to write response", http.StatusInternalServerError)
			}
			return
		}
	case t == "r" || t == "runtime":
		cfg = s.Config
	default:
		if s.Config.Debug {
			fmt.Printf("/showconfig: 'type' %q not as expected: %+v\n", t, r)
		}
		data.Success = false
		data.Messages = append(data.Messages, "'type' must be specified: [disk,runtime]")
		if err := s.Tmpls.ExecuteTemplate(w, "showconfig.html", data); err != nil {
			http.Error(w, "unable to write response", http.StatusInternalServerError)
		}
		return
	}

	if t != "diff" {
		config, err := configuration.ConvertToJSON(*cfg, true)
		if err != nil {
			if s.Config.Debug {
				fmt.Printf("/showconfig: unable to convert config: %s\n", err)
			}
			data.Success = false
			data.Messages = append(data.Messages, "unable to convert config")
			if err := s.Tmpls.ExecuteTemplate(w, "showconfig.html", data); err != nil {
				http.Error(w, "unable to write response", http.StatusInternalServerError)
			}
			return
		}

		data.Content = string(config)
		if err := s.Tmpls.ExecuteTemplate(w, "showconfig.html", data); err != nil {
			http.Error(w, "unable to write response", http.StatusInternalServerError)
		}
		return
	}

	diffs, err := s.Config.Diff(cfg)
	if err != nil {
		if s.Config.Debug {
			fmt.Printf("/showconfig: unable to diff configs: %s\n", err)
		}
		data.Success = false
		data.Messages = append(data.Messages, "unable to diff config")
		if err := s.Tmpls.ExecuteTemplate(w, "showconfig.html", data); err != nil {
			http.Error(w, "unable to write response", http.StatusInternalServerError)
		}
		return
	}

	data.Diff = true
	data.Content = diffs
	if err := s.Tmpls.ExecuteTemplate(w, "showconfig.html", data); err != nil {
		http.Error(w, "unable to write response", http.StatusInternalServerError)
	}
}
