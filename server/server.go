package server

import (
	"crypto/sha256"
	"crypto/subtle"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/arednch/phonebook/configuration"
	"github.com/arednch/phonebook/data"
	"github.com/arednch/phonebook/exporter"
	"github.com/arednch/phonebook/importer"
)

type ReloadFunc func(source, olsrFile, sysInfoURL string, debug bool) error

type Server struct {
	Config     *configuration.Config
	ConfigPath string // optional when using config file

	Records   *data.Records
	Exporters map[string]exporter.Exporter

	ReloadFn ReloadFunc
}

func (s *Server) BasicAuth(next http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := r.BasicAuth()
		if ok {
			usernameHash := sha256.Sum256([]byte(username))
			passwordHash := sha256.Sum256([]byte(password))
			expectedUsernameHash := sha256.Sum256([]byte(s.Config.WebUser))
			expectedPasswordHash := sha256.Sum256([]byte(s.Config.WebPwd))

			usernameMatch := (subtle.ConstantTimeCompare(usernameHash[:], expectedUsernameHash[:]) == 1)
			passwordMatch := (subtle.ConstantTimeCompare(passwordHash[:], expectedPasswordHash[:]) == 1)

			if usernameMatch && passwordMatch {
				next.ServeHTTP(w, r)
				return
			}
		}

		w.Header().Set("WWW-Authenticate", `Basic realm="restricted", charset="UTF-8"`)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
	})
}

func (s *Server) ShowConfig(w http.ResponseWriter, r *http.Request) {
	t := r.FormValue("type")
	t = strings.ToLower(strings.TrimSpace(t))
	if t == "" {
		if s.Config.Debug {
			fmt.Printf("/showconfig: 'type' not specified: %+v\n", r)
		}
		http.Error(w, "'type' must be specified: [disk,runtime,diff]", http.StatusBadRequest)
		return
	}

	var cfg *configuration.Config
	switch {
	case t == "d" || t == "disk" || t == "diff":
		if s.ConfigPath == "" {
			http.Error(w, "phonebook was not started with a config path set ('-conf' flag) so config file can't be loaded", http.StatusInternalServerError)
			return
		}
		var err error
		if cfg, err = configuration.ReadFromJSON(s.ConfigPath); err != nil {
			if s.Config.Debug {
				fmt.Printf("/showconfig: unable to read config: %s\n", err)
			}
			http.Error(w, "unable to read config", http.StatusInternalServerError)
			return
		}
	case t == "r" || t == "runtime":
		cfg = s.Config
	default:
		if s.Config.Debug {
			fmt.Printf("/showconfig: 'type' %q not as expected: %+v\n", t, r)
		}
		http.Error(w, "'type' must be specified: [disk,runtime]", http.StatusBadRequest)
		return
	}

	if t != "diff" {
		config, err := configuration.ConvertToJSON(*cfg, true)
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
		return
	}

	diffs, err := s.Config.Diff(cfg)
	if err != nil {
		if s.Config.Debug {
			fmt.Printf("/showconfig: unable to diff configs: %s\n", err)
		}
		http.Error(w, "unable to diff config", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(diffs))
}

func (s *Server) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	if !s.Config.AllowRuntimeConfigChanges {
		if s.Config.Debug {
			fmt.Println("/updateconfig: updating config is not allowed by config")
		}
		http.Error(w, "updating config is not allowed by config flag (-allow_runtime_config_changes)", http.StatusInternalServerError)
		return
	}

	var permanent bool
	perm := r.FormValue("perm")
	perm = strings.ToLower(strings.TrimSpace(perm))
	if perm == "true" {
		permanent = true
	}
	if permanent && !s.Config.AllowPermanentConfigChanges {
		if s.Config.Debug {
			fmt.Println("/updateconfig: updating config on disk is not allowed by config")
		}
		http.Error(w, "updating config on disk is not allowed by config flag (-allow_permanent_config_changes)", http.StatusInternalServerError)
		return
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

	// Check for supported fields to update and verify.
	src := r.FormValue("source")
	src = strings.TrimSpace(src)
	if _, err := importer.ReadPhonebook(src); err != nil {
		if s.Config.Debug {
			fmt.Printf("/updateconfig: specified source is not readable: %s\n", err)
		}
		http.Error(w, "specified source cannot be read, make sure it exists and is either a valid, absolute file path or http/https URL", http.StatusInternalServerError)
		return
	}

	var reload int
	rs := r.FormValue("reload")
	rs = strings.TrimSpace(rs)
	if rs != "" {
		var err error
		reload, err = strconv.Atoi(rs)
		if err != nil {
			if s.Config.Debug {
				fmt.Printf("/updateconfig: invalid reload value: %s\n", rs)
			}
			http.Error(w, "invalid reload value", http.StatusInternalServerError)
			return
		}
		if reload < configuration.MinimalReloadSeconds || reload > configuration.MaxReloadSeconds {
			if s.Config.Debug {
				fmt.Printf("/updateconfig: reload value too high or low (<%d or >%d): %s\n", configuration.MinimalReloadSeconds, configuration.MaxReloadSeconds, rs)
			}
			http.Error(w, "reload value too high or low", http.StatusInternalServerError)
			return
		}
	}

	dbg := r.FormValue("debug")
	dbg = strings.ToLower(strings.TrimSpace(dbg))
	if dbg != "" && dbg != "true" && dbg != "false" {
		if s.Config.Debug {
			fmt.Printf("/updateconfig: invalid debug value: %s\n", dbg)
		}
		http.Error(w, "invalid debug value", http.StatusInternalServerError)
		return
	}

	// Update supported fields (assume fields are validated by now).
	if src != "" {
		changed = true
		s.Config.Source = src
		if cfg != nil {
			cfg.Source = src
		}
		fmt.Fprintf(w, "- \"source\" now set to %q\n", src)
		if s.Config.Debug {
			fmt.Printf("/updateconfig: \"source\" now set to %q\n", src)
		}
	}

	if rs != "" {
		changed = true
		rd := time.Duration(reload) * time.Second
		s.Config.ReloadSeconds = reload
		s.Config.Reload = rd
		if cfg != nil {
			cfg.ReloadSeconds = reload
			cfg.Reload = rd
		}
		fmt.Fprintf(w, "- reload duration now set to %d seconds (%s)\n", reload, rd)
		if s.Config.Debug {
			fmt.Printf("/updateconfig: reload duration now set to %d seconds (%s)\n", reload, rd)
		}
	}

	if dbg != "" {
		debug := false
		if dbg == "true" {
			debug = true
		}

		changed = true
		s.Config.Debug = debug
		if cfg != nil {
			cfg.Debug = debug
		}

		fmt.Fprintf(w, "- \"debug\" now set to %t\n", debug)
		if s.Config.Debug {
			fmt.Printf("/updateconfig: \"debug\" now set to %t\n", debug)
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
