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
	Server     bool   `json:"server,omitempty"`
	LDAPServer bool   `json:"ldap_server"`
	SIPServer  bool   `json:"sip_server"`
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
	Port          int           `json:"port"`
	ReloadSeconds int           `json:"reload_seconds"`
	Reload        time.Duration `json:"-"`
	// Only relevant when LDAP server is on.
	LDAPPort int    `json:"ldap_port"`
	LDAPUser string `json:"ldap_user"`
	LDAPPwd  string `json:"ldap_pwd"`
	// Only relevant when SIP server is on.
	SIPPort int `json:"sip_port"`
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

func ConvertToJSON(conf Config, censorSensitive bool) ([]byte, error) {
	if censorSensitive {
		conf.LDAPPwd = "***"
	}
	data, err := json.MarshalIndent(&conf, "", "  ")
	if err != nil {
		return nil, err
	}

	return data, nil
}

func WriteToJSON(conf *Config, path string, censorSensitive bool) error {
	data, err := ConvertToJSON(*conf, censorSensitive)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
