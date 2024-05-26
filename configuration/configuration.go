package configuration

import (
	"encoding/json"
	"os"
	"time"
)

type Config struct {
	// Generally applicable.
	Source     string `json:"source"`
	OLSRFile   string `json:"olsr_file"`
	SysInfoURL string `json:"sysinfo_url"`
	Server     bool   `json:"server"`
	LDAPServer bool   `json:"ldap_server"`

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

func Read(path string) (*Config, error) {
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
