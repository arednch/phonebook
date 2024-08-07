package data

import (
	"fmt"
	"sync"
	"time"
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

type Version struct {
	Version   string `json:"version"`
	CommitSHA string `json:"commit_sha"`
}

// Source

type Records struct {
	Mu      *sync.RWMutex
	Updated time.Time
	Entries []*Entry
}

type Entry struct {
	FirstName   string
	LastName    string
	Callsign    string
	PhoneNumber string

	// Metadata
	OLSR *OLSR // if present, the participant seems to be active
}

func (e *Entry) DisplayName(pfx string) string {
	switch {
	case e.LastName == "" && e.FirstName == "" && e.Callsign == "":
		return pfx + e.PhoneNumber
	case e.LastName == "" && e.FirstName == "":
		return pfx + e.Callsign
	case e.LastName == "":
		return fmt.Sprintf("%s%s (%s)", pfx, e.FirstName, e.Callsign)
	case e.FirstName == "":
		return fmt.Sprintf("%s%s (%s)", pfx, e.LastName, e.Callsign)
	default:
		return fmt.Sprintf("%s%s %s (%s)", pfx, e.LastName, e.FirstName, e.Callsign)
	}
}

func (e *Entry) DirectCallAddress() string {
	return e.PhoneNumber + "@" + e.PhoneFQDN()
}

func (e *Entry) PhoneFQDN() string {
	return e.PhoneNumber + "." + AREDNDomain
}

// Target

type GenericPhoneBook struct {
	Entry []*GenericEntry `xml:"DirectoryEntry"`
}

type GenericEntry struct {
	Name      string   `xml:"Name"`
	Telephone []string `xml:"Telephone"`
}
