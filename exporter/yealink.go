package exporter

import (
	"encoding/xml"
	"fmt"

	"github.com/finack/phonebook/data"
)

type YealinkPhoneBook struct {
	Entry []*YealinkEntry `xml:"DirectoryEntry"`
}
type YealinkEntry struct {
	Name      string `xml:"Name"`
	Telephone string `xml:"Telephone"`
}

type Yealink struct{}

func (y *Yealink) Export(entries []*data.Entry, pbx bool) ([]byte, error) {
	var targetEntries []*YealinkEntry
	for _, entry := range entries {
		tel := entry.IPAddress
		if pbx {
			tel = entry.PhoneNumber
		}
		targetEntries = append(targetEntries, &YealinkEntry{
			Name:      fmt.Sprintf("%s, %s (%s)", entry.LastName, entry.FirstName, entry.Callsign),
			Telephone: tel,
		})
	}

	return xml.MarshalIndent(struct {
		*YealinkPhoneBook
		XMLName struct{} `xml:"YealinkIPPhoneDirectory"`
	}{
		YealinkPhoneBook: &YealinkPhoneBook{Entry: targetEntries},
	}, "", "    ")
}
