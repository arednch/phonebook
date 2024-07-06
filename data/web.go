package data

import "time"

type WebInfo struct {
	Version     Version     `json:"version"`
	Registered  []string    `json:"registered_phones,omitempty"`
	RecordStats RecordStats `json:"records_stats,omitempty"`
	Runtime     Runtime     `json:"runtime,omitempty"`
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
	Version string
}

type WebReload struct {
	Version string
	Source  string
	Success bool
}

type WebShowConfig struct {
	Version  string
	Messages []string
	Content  string
	Diff     bool
	Success  bool
}

type WebUpdateConfig struct {
	Version  string
	Messages []string
	Success  bool
}
