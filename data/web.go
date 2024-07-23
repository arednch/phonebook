package data

import "time"

type WebDefault struct {
	Title   string
	Version *Version `json:"version"`
	Updated string
	Updates []*Update
}

type WebInfo struct {
	WebDefault
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
	WebDefault

	Registered map[string]string
	Records    map[string]string
	UpdateURLs string
	Sources    string
	Exporters  []string
}

type WebMessage struct {
	WebDefault

	Success bool
	From    string
	To      string
	Message string
}

type WebReload struct {
	WebDefault

	Source  string
	Success bool
}

type WebShowConfig struct {
	WebDefault

	Messages []string
	Content  string
	Diff     bool
	Success  bool
}

type WebUpdateConfig struct {
	WebDefault

	Messages []string
	Success  bool
}
