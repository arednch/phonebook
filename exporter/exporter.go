package exporter

import (
	"encoding/xml"
	"fmt"

	"github.com/finfinack/phonebook/data"
)

const (
	activePfx = "[A] "
)

type Exporter interface {
	Export([]*data.Entry, bool, bool, bool, bool) ([]byte, error)
}

func export(entries []*data.Entry, direct, resolve, indicateActive, filterInactive bool) *data.GenericPhoneBook {
	var targetEntries []*data.GenericEntry
	for _, entry := range entries {
		if filterInactive && entry.OLSR == nil {
			continue // ignoring inactive entry (no OLSR data)
		}

		var pfx string
		if indicateActive && entry.OLSR != nil {
			pfx = activePfx
		}
		name := fmt.Sprintf("%s%s, %s (%s)", pfx, entry.LastName, entry.FirstName, entry.Callsign)

		var tel string
		switch {
		case direct && resolve && entry.OLSR != nil:
			tel = entry.OLSR.IP
		case direct:
			tel = entry.IPAddress
		default:
			tel = entry.PhoneNumber
		}
		targetEntries = append(targetEntries, &data.GenericEntry{
			Name:      name,
			Telephone: tel,
		})
	}

	return &data.GenericPhoneBook{Entry: targetEntries}
}

type Generic struct{}

func (g *Generic) Export(entries []*data.Entry, direct, resolve, indicateActive, filterInactive bool) ([]byte, error) {
	return xml.MarshalIndent(struct {
		*data.GenericPhoneBook
		XMLName struct{} `xml:"IPPhoneDirectory"`
	}{
		GenericPhoneBook: export(entries, direct, resolve, indicateActive, filterInactive),
	}, "", "    ")
}
