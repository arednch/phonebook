package server

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/arednch/phonebook/configuration"
	"github.com/arednch/phonebook/data"
	"github.com/arednch/phonebook/exporter"
	"github.com/arednch/phonebook/importer"
)

type ReloadFunc func(cfg *configuration.Config) error

func NewServer(cfg *configuration.Config, cfgPath, version string, records *data.Records, exporters map[string]exporter.Exporter, refreshRecords ReloadFunc, tmpls *template.Template) *Server {
	return &Server{
		Version:    version,
		Config:     cfg,
		ConfigPath: cfgPath,
		Records:    records,
		Exporters:  exporters,
		ReloadFn:   refreshRecords,
		Tmpls:      tmpls,
	}
}

type Server struct {
	Version    string
	Config     *configuration.Config
	ConfigPath string // optional when using config file

	Records   *data.Records
	Exporters map[string]exporter.Exporter

	ReloadFn ReloadFunc

	Tmpls *template.Template
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

func (s *Server) Index(w http.ResponseWriter, r *http.Request) {
	data := data.WebIndex{
		Version: s.Version,
	}
	if err := s.Tmpls.ExecuteTemplate(w, "index.html", data); err != nil {
		http.Error(w, "unable to write response", http.StatusInternalServerError)
	}
}

func (s *Server) Info(w http.ResponseWriter, r *http.Request) {
	data, err := json.Marshal(&data.WebInfo{
		Version: s.Version,
	})
	if err != nil {
		http.Error(w, "unable to marshal info", http.StatusInternalServerError)
		return
	}
	w.Write(data)
}

func (s *Server) ShowConfig(w http.ResponseWriter, r *http.Request) {
	data := data.WebShowConfig{
		Version: s.Version,
		Success: true,
	}

	t := r.FormValue("type")
	t = strings.ToLower(strings.TrimSpace(t))
	if t == "" {
		data.Success = false
		if s.Config.Debug {
			fmt.Printf("/showconfig: 'type' not specified: %+v\n", r)
		}
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
			data.Success = false
			if s.Config.Debug {
				fmt.Printf("/showconfig: unable to read config: %s\n", err)
			}
			data.Messages = append(data.Messages, "unable to read config")
			if err := s.Tmpls.ExecuteTemplate(w, "showconfig.html", data); err != nil {
				http.Error(w, "unable to write response", http.StatusInternalServerError)
			}
			return
		}
	case t == "r" || t == "runtime":
		cfg = s.Config
	default:
		data.Success = false
		if s.Config.Debug {
			fmt.Printf("/showconfig: 'type' %q not as expected: %+v\n", t, r)
		}
		data.Messages = append(data.Messages, "'type' must be specified: [disk,runtime]")
		if err := s.Tmpls.ExecuteTemplate(w, "showconfig.html", data); err != nil {
			http.Error(w, "unable to write response", http.StatusInternalServerError)
		}
		return
	}

	if t != "diff" {
		config, err := configuration.ConvertToJSON(*cfg, true)
		if err != nil {
			data.Success = false
			if s.Config.Debug {
				fmt.Printf("/showconfig: unable to convert config: %s\n", err)
			}
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
		data.Success = false
		if s.Config.Debug {
			fmt.Printf("/showconfig: unable to diff configs: %s\n", err)
		}
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

func (s *Server) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	data := data.WebUpdateConfig{
		Version: s.Version,
		Success: true,
	}

	if !s.Config.AllowRuntimeConfigChanges {
		data.Success = false
		if s.Config.Debug {
			fmt.Println("/updateconfig: updating config is not allowed by config")
		}
		data.Messages = append(data.Messages, "updating config is not allowed by config flag (-allow_runtime_config_changes)")
		if err := s.Tmpls.ExecuteTemplate(w, "updateconfig.html", data); err != nil {
			http.Error(w, "unable to write response", http.StatusInternalServerError)
		}
		return
	}

	var permanent bool
	perm := r.FormValue("perm")
	perm = strings.ToLower(strings.TrimSpace(perm))
	if perm == "true" {
		permanent = true
	}
	if permanent && !s.Config.AllowPermanentConfigChanges {
		data.Success = false
		if s.Config.Debug {
			fmt.Println("/updateconfig: updating config on disk is not allowed by config")
		}
		data.Messages = append(data.Messages, "updating config on disk is not allowed by config flag (-allow_permanent_config_changes)")
		if err := s.Tmpls.ExecuteTemplate(w, "updateconfig.html", data); err != nil {
			http.Error(w, "unable to write response", http.StatusInternalServerError)
		}
		return
	}

	var changed bool
	var cfg *configuration.Config
	switch {
	case s.ConfigPath == "":
		data.Messages = append(data.Messages, "phonebook was not started with a config path set ('-conf' flag) so config file won't be updated")
	case !permanent:
		data.Messages = append(data.Messages, "phonebook config changes are not going to be written to disk")
	default:
		var err error
		if cfg, err = configuration.ReadFromJSON(s.ConfigPath); err != nil {
			data.Success = false
			if s.Config.Debug {
				fmt.Printf("/updateconfig: unable to read config: %s\n", err)
			}
			data.Messages = append(data.Messages, "unable to read config")
			if err := s.Tmpls.ExecuteTemplate(w, "updateconfig.html", data); err != nil {
				http.Error(w, "unable to write response", http.StatusInternalServerError)
			}
			return
		}
		data.Messages = append(data.Messages, fmt.Sprintf("phonebook config changes will be reflected in %q", s.ConfigPath))
	}

	// Check for supported fields to update and verify.
	src := r.FormValue("source")
	src = strings.TrimSpace(src)
	if src != "" {
		if _, err := importer.ReadPhonebook(src); err != nil {
			data.Success = false
			if s.Config.Debug {
				fmt.Printf("/updateconfig: specified source is not readable: %s\n", err)
			}
			data.Messages = append(data.Messages, "specified source cannot be read, make sure it exists and is either a valid, absolute file path or http/https URL")
			if err := s.Tmpls.ExecuteTemplate(w, "updateconfig.html", data); err != nil {
				http.Error(w, "unable to write response", http.StatusInternalServerError)
			}
			return
		}
	}

	var reload int
	rs := r.FormValue("reload")
	rs = strings.TrimSpace(rs)
	if rs != "" {
		var err error
		reload, err = strconv.Atoi(rs)
		if err != nil {
			data.Success = false
			if s.Config.Debug {
				fmt.Printf("/updateconfig: invalid reload value: %s\n", rs)
			}
			data.Messages = append(data.Messages, "invalid reload value")
			if err := s.Tmpls.ExecuteTemplate(w, "updateconfig.html", data); err != nil {
				http.Error(w, "unable to write response", http.StatusInternalServerError)
			}
			return
		}
		if reload < configuration.MinimalReloadSeconds || reload > configuration.MaxReloadSeconds {
			data.Success = false
			if s.Config.Debug {
				fmt.Printf("/updateconfig: reload value too high or low (<%d or >%d): %s\n", configuration.MinimalReloadSeconds, configuration.MaxReloadSeconds, rs)
			}
			data.Messages = append(data.Messages, fmt.Sprintf("reload value too high or low (<%d or >%d)", configuration.MinimalReloadSeconds, configuration.MaxReloadSeconds))
			if err := s.Tmpls.ExecuteTemplate(w, "updateconfig.html", data); err != nil {
				http.Error(w, "unable to write response", http.StatusInternalServerError)
			}
			return
		}
	}

	dbg := r.FormValue("debug")
	dbg = strings.ToLower(strings.TrimSpace(dbg))
	if dbg != "" && dbg != "true" && dbg != "false" {
		if s.Config.Debug {
			fmt.Printf("/updateconfig: invalid debug value: %s\n", dbg)
		}
		data.Messages = append(data.Messages, "invalid debug value")
		if err := s.Tmpls.ExecuteTemplate(w, "updateconfig.html", data); err != nil {
			http.Error(w, "unable to write response", http.StatusInternalServerError)
		}
		return
	}

	rt := r.FormValue("routable")
	rt = strings.ToLower(strings.TrimSpace(rt))
	if rt != "" && rt != "true" && rt != "false" {
		data.Success = false
		if s.Config.Debug {
			fmt.Printf("/updateconfig: invalid routable value: %s\n", rt)
		}
		data.Messages = append(data.Messages, "invalid routable value")
		if err := s.Tmpls.ExecuteTemplate(w, "updateconfig.html", data); err != nil {
			http.Error(w, "unable to write response", http.StatusInternalServerError)
		}
		return
	}

	webuser := r.FormValue("webuser")
	webuser = strings.TrimSpace(webuser)

	webpwd := r.FormValue("webpwd")
	webpwd = strings.TrimSpace(webpwd)

	// Update supported fields (assume fields are validated by now).
	if src != "" {
		changed = true
		s.Config.Source = src
		if cfg != nil {
			cfg.Source = src
		}
		data.Messages = append(data.Messages, fmt.Sprintf("- \"source\" now set to %q", src))
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
		data.Messages = append(data.Messages, fmt.Sprintf("- reload duration now set to %d seconds (%s)", reload, rd))
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

		data.Messages = append(data.Messages, fmt.Sprintf("- \"debug\" now set to %t", debug))
		if s.Config.Debug {
			fmt.Printf("/updateconfig: \"debug\" now set to %t\n", debug)
		}
	}

	if rt != "" {
		routable := false
		if rt == "true" {
			routable = true
		}

		changed = true
		s.Config.IncludeRoutable = routable
		if cfg != nil {
			cfg.IncludeRoutable = routable
		}

		data.Messages = append(data.Messages, fmt.Sprintf("- \"include_routable\" now set to %t", routable))
		if s.Config.Debug {
			fmt.Printf("/updateconfig: \"include_routable\" now set to %t\n", routable)
		}
	}

	if webuser != "" {
		changed = true
		s.Config.WebUser = webuser
		if cfg != nil {
			cfg.WebUser = webuser
		}

		data.Messages = append(data.Messages, fmt.Sprintf("- \"web_user\" now set to %q", webuser))
		if s.Config.Debug {
			fmt.Printf("/updateconfig: \"web_user\" now set to %q\n", webuser)
		}
	}

	if webpwd != "" {
		changed = true
		s.Config.WebPwd = webpwd
		if cfg != nil {
			cfg.WebPwd = webpwd
		}

		data.Messages = append(data.Messages, "- \"web_pwd\" now set")
		if s.Config.Debug {
			fmt.Printf("/updateconfig: \"web_user\" now set to %q\n", webuser)
		}
		if s.Config.Debug {
			fmt.Printf("/updateconfig: \"web_pwd\" now set\n")
		}
	}

	// Exit early if we didn't make any changes (avoid unnecessary disk writes etc).
	if !changed {
		data.Messages = append(data.Messages, "no changes were made")
		if s.Config.Debug {
			fmt.Println("/updateconfig: no changes were made")
		}
		if err := s.Tmpls.ExecuteTemplate(w, "updateconfig.html", data); err != nil {
			http.Error(w, "unable to write response", http.StatusInternalServerError)
		}
		return
	}

	// Finally writing the changes if there are any.
	if cfg != nil {
		if err := configuration.WriteToJSON(cfg, s.ConfigPath, false); err != nil {
			data.Success = false
			if s.Config.Debug {
				fmt.Printf("/updateconfig: unable to write config: %s\n", err)
			}
			data.Messages = append(data.Messages, "unable to write config")
			if err := s.Tmpls.ExecuteTemplate(w, "updateconfig.html", data); err != nil {
				http.Error(w, "unable to write response", http.StatusInternalServerError)
			}
			return
		}

		data.Messages = append(data.Messages, fmt.Sprintf("phonebook config updated in %q", s.ConfigPath))
		if s.Config.Debug {
			fmt.Printf("/updateconfig: phonebook config updated in %q\n", s.ConfigPath)
		}
	} else {
		data.Messages = append(data.Messages, "only phonebook runtime (!) config updated")
		if s.Config.Debug {
			fmt.Println("/updateconfig: only phonebook runtime (!) config updated")
		}
	}
	if err := s.Tmpls.ExecuteTemplate(w, "updateconfig.html", data); err != nil {
		http.Error(w, "unable to write response", http.StatusInternalServerError)
	}
}

func (s *Server) ReloadPhonebook(w http.ResponseWriter, r *http.Request) {
	data := data.WebReload{
		Version: s.Version,
		Source:  s.Config.Source,
		Success: true,
	}
	if err := s.ReloadFn(s.Config); err != nil {
		data.Success = false
		if s.Config.Debug {
			fmt.Printf("/reload: unable to reload phonebook: %s\n", err)
		}
	} else if s.Config.Debug {
		fmt.Printf("/reload: phonebook reloaded locally from %q\n", s.Config.Source)
	}
	if err := s.Tmpls.ExecuteTemplate(w, "reload.html", data); err != nil {
		http.Error(w, "unable to write response", http.StatusInternalServerError)
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
