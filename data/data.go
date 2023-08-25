package data

// Source

type Entry struct {
	FirstName   string
	LastName    string
	Callsign    string
	IPAddress   string
	PhoneNumber string

	// Metadata
	OLSR *OLSR // if present, the participant seems to be active
}

// Target

type GenericPhoneBook struct {
	Entry []*GenericEntry `xml:"DirectoryEntry"`
}

type GenericEntry struct {
	Name      string   `xml:"Name"`
	Telephone []string `xml:"Telephone"`
}
