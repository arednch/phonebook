package exporter

import (
	"encoding/xml"

	"github.com/finack/phonebook/data"
)

type Cisco struct{}

func (c *Cisco) Export(entries []*data.Entry, pbx bool) ([]byte, error) {
	return xml.MarshalIndent(struct {
		*data.GenericPhoneBook
		XMLName struct{} `xml:"CiscoIPPhoneDirectory"`
	}{
		GenericPhoneBook: export(entries, pbx),
	}, "", "    ")
}
