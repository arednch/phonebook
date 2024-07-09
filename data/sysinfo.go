package data

import (
	"sync"
	"time"
)

type RuntimeInfo struct {
	Mu      *sync.RWMutex
	Updated time.Time

	SysInfo *SysInfo
}

type SysInfo struct {
	APIVersion string `json:"api_version"`

	Node        string       `json:"node"`
	NodeDetails *NodeDetails `json:"node_details"`
	System      *System      `json:"sysinfo"`

	Longitude  string `json:"lon"`
	Latitude   string `json:"lat"`
	Gridsquare string `json:"grid_square"`

	Hosts []*Host `json:"hosts"`
}

type System struct {
	Uptime string `json:"uptime"`
}

type Host struct {
	Name string `json:"name"`
	IP   string `json:"ip"`
}

type NodeDetails struct {
	Model           string `json:"model"`
	MeshGateway     string `json:"mesh_gateway"`
	BoardID         string `json:"board_id"`
	FirmwareMfg     string `json:"firmware_mfg"`
	FirmwareVersion string `json:"firmware_version"`
}
