package data

type SysInfo struct {
	APIVersion string `json:"api_version"`

	Node            string            `json:"node"`
	NodeDetails     *NodeDetails      `json:"node_details"`
	System          *System           `json:"sysinfo"`
	AREDNInterfaces []*AREDNInterface `json:"interfaces"`

	Longitude  string `json:"lon"`
	Latitude   string `json:"lat"`
	Gridsquare string `json:"grid_square"`

	Hosts  []*Host `json:"hosts"`
	MeshRF *MeshRF `json:"meshrf"`
}

type System struct {
	Uptime string    `json:"uptime"`
	Loads  []float64 `json:"loads"`
}

type AREDNInterface struct {
	Name string `json:"name"`
	MAC  string `json:"mac"`
	IP   string `json:"ip"`
}

type Host struct {
	Name string `json:"name"`
	IP   string `json:"ip"`
}

type MeshRF struct {
	SSID      string `json:"ssid"`
	Channel   string `json:"channel"`
	Status    string `json:"status"`
	Frequency string `json:"freq"`
	ChannelBw string `json:"chanbw"`
}

type Tunnels struct {
	ActiveTunnelCount string `json:"active_tunnel_count"`
}

type NodeDetails struct {
	Model           string `json:"model"`
	MeshGateway     string `json:"mesh_gateway"`
	BoardID         string `json:"board_id"`
	FirmwareMfg     string `json:"firmware_mfg"`
	FirmwareVersion string `json:"firmware_version"`
}
