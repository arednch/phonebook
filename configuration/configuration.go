package configuration

import (
	"encoding/json"
	"os"
	"time"
)

type Config struct {
	Source  string   `json:"source"`
	Path    string   `json:"path"`
	Formats []string `json:"formats"`
	Server  bool     `json:"server"`
	Port    int      `json:"port"`

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
