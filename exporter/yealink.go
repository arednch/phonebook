package exporter

import (
	"encoding/xml"

	"github.com/finfinack/phonebook/data"
)

type Yealink struct{}

func (y *Yealink) Export(entries []*data.Entry, direct, resolve, indicateActive, filterInactive bool) ([]byte, error) {
	return xml.MarshalIndent(struct {
		*data.GenericPhoneBook
		XMLName struct{} `xml:"YealinkIPPhoneDirectory"`
	}{
		GenericPhoneBook: export(entries, direct, resolve, indicateActive, filterInactive),
	}, "", "    ")
}
