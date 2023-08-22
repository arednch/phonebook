package exporter

import (
	"encoding/xml"

	"github.com/finfinack/phonebook/data"
)

type Snom struct{}

func (s *Snom) Export(entries []*data.Entry, pbx bool) ([]byte, error) {
	return xml.MarshalIndent(struct {
		*data.GenericPhoneBook
		XMLName struct{} `xml:"SnomIPPhoneDirectory"`
	}{
		GenericPhoneBook: export(entries, pbx),
	}, "", "    ")
}
