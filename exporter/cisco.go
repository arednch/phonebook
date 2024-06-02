package exporter

import (
	"encoding/xml"

	"github.com/arednch/phonebook/data"
)

type CiscoPhonebook struct {
	Title  string `xml:"Title"`
	Prompt string `xml:"Prompt"`
}

type Cisco struct{}

func (c *Cisco) Export(entries []*data.Entry, format Format, activePfx string, resolve, indicateActive, filterInactive, debug bool) ([]byte, error) {
	return xml.MarshalIndent(struct {
		*CiscoPhonebook
		*data.GenericPhoneBook
		XMLName struct{} `xml:"CiscoIPPhoneDirectory"`
	}{
		GenericPhoneBook: export(entries, format, activePfx, resolve, indicateActive, filterInactive, debug),
		CiscoPhonebook: &CiscoPhonebook{
			Title:  "Cisco Coporate Directory",
			Prompt: "Select the User",
		},
	}, "", "    ")
}
