package exporter

import (
	"encoding/xml"
	"fmt"

	"github.com/finfinack/phonebook/data"
)

type Exporter interface {
	Export([]*data.Entry, bool) ([]byte, error)
}

func export(entries []*data.Entry, pbx bool) *data.GenericPhoneBook {
	var targetEntries []*data.GenericEntry
	for _, entry := range entries {
		tel := entry.IPAddress
		if pbx {
			tel = entry.PhoneNumber
		}
		targetEntries = append(targetEntries, &data.GenericEntry{
			Name:      fmt.Sprintf("%s, %s (%s)", entry.LastName, entry.FirstName, entry.Callsign),
			Telephone: tel,
		})
	}

	return &data.GenericPhoneBook{Entry: targetEntries}
}

type Generic struct{}

func (g *Generic) Export(entries []*data.Entry, pbx bool) ([]byte, error) {
	return xml.MarshalIndent(struct {
		*data.GenericPhoneBook
		XMLName struct{} `xml:"IPPhoneDirectory"`
	}{
		GenericPhoneBook: export(entries, pbx),
	}, "", "    ")
}
