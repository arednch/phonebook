package configuration

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/digineo/go-uci"
)

const (
	uciConfig = "phonebook" // under what name the phonebook config lives
)

type Config struct {
	// Generally applicable.
	Source     string `json:"source"`
	OLSRFile   string `json:"olsr_file"`
	SysInfoURL string `json:"sysinfo_url"`
	Server     bool   `json:"server"`
	LDAPServer bool   `json:"ldap_server"`
	Debug      bool   `json:"debug"`

	// Only relevant when running in non-server / ad-hoc mode.
	Path           string   `json:"path"`
	Formats        []string `json:"formats"`
	Targets        []string `json:"targets"`
	Resolve        bool     `json:"resolve"`
	IndicateActive bool     `json:"indicate_active"`
	FilterInactive bool     `json:"filter_inactive"`
	ActivePfx      string   `json:"active_pfx"`

	// Only relevant when running in server mode.
	Port          int `json:"port"`
	ReloadSeconds int `json:"reload_seconds"`
	Reload        time.Duration
	// Only relevant when LDAP server is on.
	LDAPPort int    `json:"ldap_port"`
	LDAPUser string `json:"ldap_user"`
	LDAPPwd  string `json:"ldap_pwd"`
}

func ReadFromUCI(path string) (*Config, error) {
	u := uci.NewTree(path)
	if err := u.LoadConfig(uciConfig, true); err != nil {
		return nil, fmt.Errorf("unable to read config %q: %s", path, err)
	}
	cfg := &Config{}
	// Generally applicable.
	if values, ok := u.Get(uciConfig, "main", "source"); ok {
		cfg.Source = values[0]
	}
	if values, ok := u.Get(uciConfig, "main", "olsr_file"); ok {
		cfg.OLSRFile = values[0]
	}
	if values, ok := u.Get(uciConfig, "main", "sysinfo_url"); ok {
		cfg.SysInfoURL = values[0]
	}
	if value, ok := u.GetBool(uciConfig, "main", "server"); ok {
		cfg.Server = value
	}
	if value, ok := u.GetBool(uciConfig, "main", "ldap_server"); ok {
		cfg.LDAPServer = value
	}
	if value, ok := u.GetBool(uciConfig, "main", "debug"); ok {
		cfg.Debug = value
	}
	// Only relevant when running in non-server / ad-hoc mode.
	if values, ok := u.Get(uciConfig, "main", "path"); ok {
		cfg.Path = values[0]
	}
	if values, ok := u.Get(uciConfig, "main", "formats"); ok {
		cfg.Formats = values
	}
	if values, ok := u.Get(uciConfig, "main", "targets"); ok {
		cfg.Targets = values
	}
	if value, ok := u.GetBool(uciConfig, "main", "resolve"); ok {
		cfg.Resolve = value
	}
	if value, ok := u.GetBool(uciConfig, "main", "indicate_active"); ok {
		cfg.IndicateActive = value
	}
	if value, ok := u.GetBool(uciConfig, "main", "filter_inactive"); ok {
		cfg.FilterInactive = value
	}
	if values, ok := u.Get(uciConfig, "main", "active_pfx"); ok {
		cfg.ActivePfx = values[0]
	}
	// Only relevant when running in server mode.
	if values, ok := u.Get(uciConfig, "main", "port"); ok {
		i, err := strconv.Atoi(values[0])
		if err != nil {
			return nil, fmt.Errorf("unable to convert 'port' value %q to integer: %s", values[0], err)
		}
		cfg.Port = i
	}
	if values, ok := u.Get(uciConfig, "main", "reload_seconds"); ok {
		i, err := strconv.Atoi(values[0])
		if err != nil {
			return nil, fmt.Errorf("unable to convert 'reload_seconds' value %q to integer: %s", values[0], err)
		}
		cfg.ReloadSeconds = i
	}
	// Only relevant when LDAP server is on.
	if values, ok := u.Get(uciConfig, "main", "ldap_port"); ok {
		i, err := strconv.Atoi(values[0])
		if err != nil {
			return nil, fmt.Errorf("unable to convert 'ldap_port' value %q to integer: %s", values[0], err)
		}
		cfg.LDAPPort = i
	}
	if values, ok := u.Get(uciConfig, "main", "ldap_user"); ok {
		cfg.LDAPUser = values[0]
	}
	if values, ok := u.Get(uciConfig, "main", "ldap_pwd"); ok {
		cfg.LDAPPwd = values[0]
	}
	return cfg, nil
}

func ReadFromJSON(path string) (*Config, error) {
	f, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var conf Config
	if err := json.Unmarshal(f, &conf); err != nil {
		return nil, err
	}

	return &conf, nil
}
