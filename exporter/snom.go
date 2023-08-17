package exporter

import (
	"encoding/xml"

	"github.com/finack/phonebook/data"
)

type Snom struct{}

func (s *Snom) Export(entries []*data.Entry, pbx bool) ([]byte, error) {
	return xml.MarshalIndent(struct {
		*GenericPhoneBook
		XMLName struct{} `xml:"SnomIPPhoneDirectory"`
	}{
		GenericPhoneBook: export(entries, pbx),
	}, "", "    ")
}
