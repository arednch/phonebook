package data

import (
	"fmt"
	"sync"
)

const (
	SIPSeparator = "@"

	AREDNDomain    = "local.mesh"
	AREDNLocalNode = "localnode.local.mesh" // AREDN default for local node
)

type ByName []*Entry

func (e ByName) sortKeyForEntry(entry *Entry) string {
	if entry.OLSR != nil {
		// Mark active entries so they appear first.
		return fmt.Sprintf("*%s %s %s", entry.LastName, entry.FirstName, entry.Callsign)
	}
	return fmt.Sprintf("%s %s %s", entry.LastName, entry.FirstName, entry.Callsign)
}
func (e ByName) Len() int           { return len(e) }
func (e ByName) Swap(i, j int)      { e[i], e[j] = e[j], e[i] }
func (e ByName) Less(i, j int) bool { return e.sortKeyForEntry(e[i]) < e.sortKeyForEntry(e[j]) }

type ByCallsign []*Entry

func (e ByCallsign) Len() int           { return len(e) }
func (e ByCallsign) Swap(i, j int)      { e[i], e[j] = e[j], e[i] }
func (e ByCallsign) Less(i, j int) bool { return e[i].Callsign < e[j].Callsign }

// Source

type Records struct {
	Mu      *sync.RWMutex
	Entries []*Entry
}

type Entry struct {
	FirstName   string
	LastName    string
	Callsign    string
	IPAddress   string
	PhoneNumber string

	// Optional data
	Email  string
	Club   string
	Mobile string
	Street string
	City   string

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
