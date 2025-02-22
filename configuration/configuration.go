package configuration

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/go-cmp/cmp"
)

const (
	MinimalReloadSeconds = 60               // one minute
	MaxReloadSeconds     = 2 * 24 * 60 * 60 // two days

	CountryPfxDigits = 3
	// Maximal length of local phone numbers (i.e. w/o country prefix)
	// Numbers which are of this length or shorter will be treated as local numbers.
	LocalPhoneNumberMax = 7
	// Minimal length of local phone numbers (i.e. w/o country prefix)
	// This guarantees that numbers with the minimal length are not assumed to be
	// local numbers even if they have a country prefix.
	LocalPhoneNumberMin = LocalPhoneNumberMax - CountryPfxDigits + 1
)

type Config struct {
	// Generally applicable.
	Sources         []string `json:"sources"`
	OLSRFile        string   `json:"olsr_file"`
	SysInfoURL      string   `json:"sysinfo_url"`
	Server          bool     `json:"server,omitempty"`
	LDAPServer      bool     `json:"ldap_server"`
	SIPServer       bool     `json:"sip_server"`
	WebServer       bool     `json:"web_server"`
	IncludeRoutable bool     `json:"include_routable"`
	CountryPrefix   string   `json:"country_prefix"`

	Debug                       bool `json:"debug"`
	AllowRuntimeConfigChanges   bool `json:"allow_runtime_config_changes"`
	AllowPermanentConfigChanges bool `json:"allow_permanent_config_changes"`

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
	Cache         string        `json:"cache"`
	ReloadSeconds int           `json:"reload_seconds"`
	Reload        time.Duration `json:"-"`
	WebUser       string        `json:"web_user"`
	WebPwd        string        `json:"web_pwd"`
	UpdateURLs    []string      `json:"update_urls"`
	// Only relevant when LDAP server is on.
	LDAPPort int    `json:"ldap_port"`
	LDAPUser string `json:"ldap_user"`
	LDAPPwd  string `json:"ldap_pwd"`
	// Only relevant when SIP server is on.
	SIPPort int `json:"sip_port"`
}

func (c *Config) IsValid() error {
	// Sources
	if err := ValidateSources(c.Sources); err != nil {
		return err
	}

	// Country Prefix
	if err := ValidateCountryPrefix(c.CountryPrefix); err != nil {
		return err
	}

	// Check server and non-server specific configs/flags.
	if c.Server {
		// Validation only relevant for server.
		if c.Reload.Seconds() < MinimalReloadSeconds {
			return fmt.Errorf("reload config/flag too low (<%d): %d", MinimalReloadSeconds, int(c.Reload.Seconds()))
		}
		if c.Reload.Seconds() > MaxReloadSeconds {
			return fmt.Errorf("reload config/flag too high (>%d): %d", MaxReloadSeconds, int(c.Reload.Seconds()))
		}
	} else {
		if c.Path == "" {
			return errors.New("path needs to be set")
		}
		if len(c.Formats) == 0 {
			return errors.New("formats need to be set")
		}
		if len(c.Targets) == 0 {
			return errors.New("targets need to be set")
		}
	}

	return nil
}

func (c *Config) IsLocalNumber(pn string) bool {
	return len(pn) > LocalPhoneNumberMax
}

func (c *Config) GetLocalNumber(pn string) string {
	if c.IsLocalNumber(pn) {
		return pn
	}
	return pn[3:]
}

func (c *Config) GetGlobalNumber(pn string) string {
	if c.IsLocalNumber(pn) {
		return c.CountryPrefix + pn
	}
	return pn
}

func (c *Config) Diff(other *Config) (string, error) {
	c1, err := ConvertToJSON(*c, true)
	if err != nil {
		return "", fmt.Errorf("error converting config (self): %s", err)
	}

	c2, err := ConvertToJSON(*other, true)
	if err != nil {
		return "", fmt.Errorf("error converting config (other): %s", err)
	}

	return cmp.Diff(string(c1), string(c2)), nil
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
		conf.WebPwd = "***"
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

func ValidateCountryPrefix(pfx string) error {
	if pfx == "" {
		return errors.New("country prefix needs to be set")
	}
	if len(pfx) != CountryPfxDigits {
		return fmt.Errorf("country prefix must be %d digits but isn't: %s", CountryPfxDigits, pfx)
	}
	if p, err := strconv.Atoi(pfx); err != nil {
		return fmt.Errorf("country prefix is not a number: %s", pfx)
	} else if p < 0 {
		return fmt.Errorf("country prefix must be positive number: %s", pfx)
	}
	return nil
}

func ValidateSources(srcs []string) error {
	if len(srcs) == 0 {
		return errors.New("at least one source needs to be set")
	}
	for _, s := range srcs {
		if strings.HasPrefix(s, "/") {
			continue
		}
		if err := ValidateURL(s); err != nil {
			return fmt.Errorf("source needs to be a URL (http://, https://) or a local file path (/ prefix): %q", s)
		}
	}
	return nil
}

func ValidateURL(u string) error {
	if _, err := url.Parse(u); err != nil {
		return errors.New("invalid URL")
	}
	if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
		return errors.New("URL is not a http:// or https:// URL")
	}
	return nil
}
