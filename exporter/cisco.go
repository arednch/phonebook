package exporter

import (
	"encoding/xml"
	"fmt"

	"github.com/finack/phonebook/data"
)

type CiscoPhoneBook struct {
	Entry []*CiscoEntry `xml:"DirectoryEntry"`
}

type CiscoEntry struct {
	Name      string `xml:"Name"`
	Telephone string `xml:"Telephone"`
}

type Cisco struct{}

func (c *Cisco) Export(entries []*data.Entry, pbx bool) ([]byte, error) {
	var targetEntries []*CiscoEntry
	for _, entry := range entries {
		tel := entry.IPAddress
		if pbx {
			tel = entry.PhoneNumber
		}
		targetEntries = append(targetEntries, &CiscoEntry{
			Name:      fmt.Sprintf("%s, %s (%s)", entry.LastName, entry.FirstName, entry.Callsign),
			Telephone: tel,
		})
	}

	return xml.MarshalIndent(struct {
		*CiscoPhoneBook
		XMLName struct{} `xml:"CiscoIPPhoneDirectory"`
	}{
		CiscoPhoneBook: &CiscoPhoneBook{Entry: targetEntries},
	}, "", "    ")
}
