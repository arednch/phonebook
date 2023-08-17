package exporter

import (
	"fmt"

	"github.com/finack/phonebook/data"
)

type Exporter interface {
	Export([]*data.Entry, bool) ([]byte, error)
}

type GenericPhoneBook struct {
	Entry []*GenericEntry `xml:"DirectoryEntry"`
}

type GenericEntry struct {
	Name      string `xml:"Name"`
	Telephone string `xml:"Telephone"`
}

func export(entries []*data.Entry, pbx bool) *GenericPhoneBook {
	var targetEntries []*GenericEntry
	for _, entry := range entries {
		tel := entry.IPAddress
		if pbx {
			tel = entry.PhoneNumber
		}
		targetEntries = append(targetEntries, &GenericEntry{
			Name:      fmt.Sprintf("%s, %s (%s)", entry.LastName, entry.FirstName, entry.Callsign),
			Telephone: tel,
		})
	}

	return &GenericPhoneBook{Entry: targetEntries}
}
