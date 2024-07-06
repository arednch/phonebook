package data

type WebInfo struct {
	Version string `json:"version"`
}

type WebIndex struct {
	Version string
}

type WebReload struct {
	Version string
	Source  string
	Success bool
}
