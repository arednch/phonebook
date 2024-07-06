package data

type WebInfo struct {
	Version Version
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
