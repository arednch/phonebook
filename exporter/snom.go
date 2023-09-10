package exporter

import (
	"encoding/xml"

	"github.com/arednch/phonebook/data"
)

type Snom struct{}

func (s *Snom) Export(entries []*data.Entry, format Format, activePfx string, resolve, indicateActive, filterInactive bool) ([]byte, error) {
	return xml.MarshalIndent(struct {
		*data.GenericPhoneBook
		XMLName struct{} `xml:"SnomIPPhoneDirectory"`
	}{
		GenericPhoneBook: export(entries, format, activePfx, resolve, indicateActive, filterInactive),
	}, "", "    ")
}
