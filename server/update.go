package server

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/arednch/phonebook/configuration"
	"github.com/arednch/phonebook/data"
	"github.com/arednch/phonebook/importer"
)

func (s *Server) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	data := data.WebUpdateConfig{
		WebDefault: *s.prepareDefaultData("Update Config", false),
		Success:    true,
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
	rawUpdates := r.FormValue("updates")
	var upds []string
	for _, u := range strings.Split(strings.TrimSpace(rawUpdates), "\n") {
		u = strings.Trim(u, " \n\r")
		if err := configuration.ValidateURL(u); err != nil {
			if s.Config.Debug {
				fmt.Printf("/updateconfig: specified update URL is not valid: %s\n", err)
			}
			data.Success = false
			data.Messages = append(data.Messages, "specified update URL is not valid")
			if err := s.Tmpls.ExecuteTemplate(w, "updateconfig.html", data); err != nil {
				http.Error(w, "unable to write response", http.StatusInternalServerError)
			}
			return
		}
		upds = append(upds, u)
	}

	rawSrc := r.FormValue("sources")
	var srcs []string
	for _, src := range strings.Split(strings.TrimSpace(rawSrc), "\n") {
		src = strings.Trim(src, " \n\r")
		switch {
		case strings.HasPrefix(src, "http"):
			if err := configuration.ValidateURL(src); err != nil {
				if s.Config.Debug {
					fmt.Printf("/updateconfig: specified source are not all readable: %s\n", err)
				}
				data.Success = false
				data.Messages = append(data.Messages, "specified sources cannot all be read, make sure they exist and are either a valid, absolute file path or an http/https URL")
				if err := s.Tmpls.ExecuteTemplate(w, "updateconfig.html", data); err != nil {
					http.Error(w, "unable to write response", http.StatusInternalServerError)
				}
				return
			}
		case strings.HasPrefix(src, "/"):
			if _, err := importer.ReadPhonebook(src, s.Config.Cache, s.Client); err != nil {
				if s.Config.Debug {
					fmt.Printf("/updateconfig: specified source are not all readable: %s\n", err)
				}
				data.Success = false
				data.Messages = append(data.Messages, "specified sources cannot all be read, make sure they exist and are either a valid, absolute file path or an http/https URL")
				if err := s.Tmpls.ExecuteTemplate(w, "updateconfig.html", data); err != nil {
					http.Error(w, "unable to write response", http.StatusInternalServerError)
				}
				return
			}
		default:
			if s.Config.Debug {
				fmt.Printf("/updateconfig: specified source formats are not all readable: %s\n", src)
			}
			data.Success = false
			data.Messages = append(data.Messages, "specified sources formats cannot all be read, make sure they exist and are either a valid, absolute file path or an http/https URL")
			if err := s.Tmpls.ExecuteTemplate(w, "updateconfig.html", data); err != nil {
				http.Error(w, "unable to write response", http.StatusInternalServerError)
			}
			return
		}
		srcs = append(srcs, src)
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
			data.Success = false
			data.Messages = append(data.Messages, "invalid reload value")
			if err := s.Tmpls.ExecuteTemplate(w, "updateconfig.html", data); err != nil {
				http.Error(w, "unable to write response", http.StatusInternalServerError)
			}
			return
		}
		if reload < configuration.MinimalReloadSeconds || reload > configuration.MaxReloadSeconds {
			if s.Config.Debug {
				fmt.Printf("/updateconfig: reload value too high or low (<%d or >%d): %s\n", configuration.MinimalReloadSeconds, configuration.MaxReloadSeconds, rs)
			}
			data.Success = false
			data.Messages = append(data.Messages, fmt.Sprintf("reload value too high or low (<%d or >%d)", configuration.MinimalReloadSeconds, configuration.MaxReloadSeconds))
			if err := s.Tmpls.ExecuteTemplate(w, "updateconfig.html", data); err != nil {
				http.Error(w, "unable to write response", http.StatusInternalServerError)
			}
			return
		}
	}

	apfx := r.FormValue("apfx")
	apfx = strings.ToLower(strings.TrimSpace(apfx))
	if apfx != "" && len(apfx) > 1 {
		if s.Config.Debug {
			fmt.Printf("/updateconfig: invalid active prefix value (can only be one character): %s\n", apfx)
		}
		data.Success = false
		data.Messages = append(data.Messages, "invalid country prefix value (can only be one character)")
		if err := s.Tmpls.ExecuteTemplate(w, "updateconfig.html", data); err != nil {
			http.Error(w, "unable to write response", http.StatusInternalServerError)
		}
		return
	}

	cpfx := r.FormValue("cpfx")
	cpfx = strings.ToLower(strings.TrimSpace(cpfx))
	if cpfx != "" {
		if err := configuration.ValidateCountryPrefix(cpfx); err != nil {
			if s.Config.Debug {
				fmt.Printf("/updateconfig: invalid country prefix value: %s\n", err)
			}
			data.Success = false
			data.Messages = append(data.Messages, "invalid country prefix value")
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
		data.Success = false
		data.Messages = append(data.Messages, "invalid debug value")
		if err := s.Tmpls.ExecuteTemplate(w, "updateconfig.html", data); err != nil {
			http.Error(w, "unable to write response", http.StatusInternalServerError)
		}
		return
	}

	rt := r.FormValue("routable")
	rt = strings.ToLower(strings.TrimSpace(rt))
	if rt != "" && rt != "true" && rt != "false" {
		if s.Config.Debug {
			fmt.Printf("/updateconfig: invalid routable value: %s\n", rt)
		}
		data.Success = false
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
	if len(srcs) > 0 {
		changed = true
		s.Config.Sources = srcs
		if cfg != nil {
			cfg.Sources = srcs
		}
		data.Messages = append(data.Messages, fmt.Sprintf("- sources now set to %s", srcs))
		if s.Config.Debug {
			fmt.Printf("/updateconfig: sources now set to %s\n", srcs)
		}
	}

	if len(upds) > 0 {
		changed = true
		s.Config.UpdateURLs = upds
		if cfg != nil {
			cfg.UpdateURLs = upds
		}
		data.Messages = append(data.Messages, fmt.Sprintf("- update URLs now set to %s", upds))
		if s.Config.Debug {
			fmt.Printf("/updateconfig: update URLs now set to %s\n", upds)
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

	if apfx != "" {
		changed = true
		s.Config.ActivePfx = apfx
		if cfg != nil {
			cfg.ActivePfx = apfx
		}
		data.Messages = append(data.Messages, fmt.Sprintf("- active prefix set to %q", apfx))
		if s.Config.Debug {
			fmt.Printf("/updateconfig: active prefix set to %q\n", apfx)
		}
	}

	if cpfx != "" {
		changed = true
		s.Config.CountryPrefix = cpfx
		if cfg != nil {
			cfg.CountryPrefix = cpfx
		}
		data.Messages = append(data.Messages, fmt.Sprintf("- country prefix set to %q", cpfx))
		if s.Config.Debug {
			fmt.Printf("/updateconfig: country prefix set to %q\n", cpfx)
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

		data.Messages = append(data.Messages, fmt.Sprintf("- debug now set to %t", debug))
		if s.Config.Debug {
			fmt.Printf("/updateconfig: debug now set to %t\n", debug)
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

		data.Messages = append(data.Messages, fmt.Sprintf("- include_routable now set to %t", routable))
		if s.Config.Debug {
			fmt.Printf("/updateconfig: include_routable now set to %t\n", routable)
		}
	}

	if webuser != "" {
		changed = true
		s.Config.WebUser = webuser
		if cfg != nil {
			cfg.WebUser = webuser
		}

		data.Messages = append(data.Messages, fmt.Sprintf("- web_user now set to %q", webuser))
		if s.Config.Debug {
			fmt.Printf("/updateconfig: web_user now set to %q\n", webuser)
		}
	}

	if webpwd != "" {
		changed = true
		s.Config.WebPwd = webpwd
		if cfg != nil {
			cfg.WebPwd = webpwd
		}

		data.Messages = append(data.Messages, "- web_pwd now set")
		if s.Config.Debug {
			fmt.Printf("/updateconfig: web_user now set to %q\n", webuser)
		}
		if s.Config.Debug {
			fmt.Printf("/updateconfig: web_pwd now set\n")
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
