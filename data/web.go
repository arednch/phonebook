package data

import "time"

type WebInfo struct {
	Version     *Version          `json:"version"`
	Registered  map[string]string `json:"registered_phones,omitempty"`
	RecordStats RecordStats       `json:"records_stats,omitempty"`
	Runtime     Runtime           `json:"runtime,omitempty"`
}

type Runtime struct {
	Node    string      `json:"node,omitempty"`
	Uptime  string      `json:"uptime,omitempty"`
	Details NodeDetails `json:"details,omitempty"`
	Updated time.Time   `json:"updated"`
}

type RecordStats struct {
	Count   int       `json:"count"`
	Updated time.Time `json:"updated"`
}

type WebIndex struct {
	Version *Version
	Updated string

	Registered map[string]string
	Records    map[string]string
	Updates    []*Update
	UpdateURLs string
	Sources    string
	Exporters  []string
}

type WebMessage struct {
	Version *Version

	Success bool
	From    string
	To      string
	Message string
}

type WebReload struct {
	Version *Version
	Updated string

	Source  string
	Success bool
}

type WebShowConfig struct {
	Version *Version

	Messages []string
	Content  string
	Diff     bool
	Success  bool
}

type WebUpdateConfig struct {
	Version *Version

	Messages []string
	Success  bool
}
