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

func (c *Cisco) Export(entries []*data.Entry, format Format, activePfx string, resolve, indicateActive, filterInactive bool) ([]byte, error) {
	return xml.MarshalIndent(struct {
		*CiscoPhonebook
		*data.GenericPhoneBook
		XMLName struct{} `xml:"CiscoIPPhoneDirectory"`
	}{
		GenericPhoneBook: export(entries, format, activePfx, resolve, indicateActive, filterInactive),
		CiscoPhonebook: &CiscoPhonebook{
			Title:  "Cisco Coporate Directory",
			Prompt: "Select the User",
		},
	}, "", "    ")
}

/*
<CiscoIPPhoneDirectory>
  <Title>Cisco Coporate Directory</Title>
  <Prompt>Select the User</Prompt>
  <DirectoryEntry>
    <Name>HB9HFM Yealink</Name>
    <Telephone>178230@178230.local.mesh</Telephone>
  </DirectoryEntry>
</CiscoIPPhoneDirectory>
*/
