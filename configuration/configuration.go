package configuration

import (
	"encoding/json"
	"os"
	"time"
)

type Config struct {
	// Generally applicable.
	Source  string   `json:"source"`
	Path    string   `json:"path"`
	Server  bool     `json:"server"`
	Resolve bool     `json:"resolve"`
	Formats []string `json:"formats"`

	// Only relevant when running in non-server / ad-hoc mode.
	Targets []string `json:"targets"`

	// Only relevant when running in server mode.
	Port          int `json:"port"`
	ReloadSeconds int `json:"reload_seconds"`
	Reload        time.Duration
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
