package exporter

import (
	"encoding/xml"

	"github.com/finfinack/phonebook/data"
)

type Cisco struct{}

func (c *Cisco) Export(entries []*data.Entry, format Format, activePfx string, resolve, indicateActive, filterInactive bool) ([]byte, error) {
	return xml.MarshalIndent(struct {
		*data.GenericPhoneBook
		XMLName struct{} `xml:"CiscoIPPhoneDirectory"`
	}{
		GenericPhoneBook: export(entries, format, activePfx, resolve, indicateActive, filterInactive),
	}, "", "    ")
}
