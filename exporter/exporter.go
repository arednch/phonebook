package exporter

import (
	"encoding/xml"
	"fmt"

	"github.com/finfinack/phonebook/data"
)

const (
	activePfx = "[A] "

	FormatCombined = Format("combined")
	FormatDirect   = Format("direct")
	FormatPBX      = Format("pbx")
)

type Format string

type Exporter interface {
	Export([]*data.Entry, Format, bool, bool, bool) ([]byte, error)
}

func export(entries []*data.Entry, format Format, resolve, indicateActive, filterInactive bool) *data.GenericPhoneBook {
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

		var tel []string
		switch format {
		case "direct":
			if resolve && entry.OLSR != nil {
				tel = []string{entry.OLSR.IP}
			} else {
				tel = []string{entry.IPAddress}
			}
		case "pbx":
			tel = []string{entry.PhoneNumber}
		default:
			if resolve && entry.OLSR != nil {
				tel = []string{
					entry.OLSR.IP,
					entry.PhoneNumber,
				}
			} else {
				tel = []string{
					entry.IPAddress,
					entry.PhoneNumber,
				}
			}
		}
		targetEntries = append(targetEntries, &data.GenericEntry{
			Name:      name,
			Telephone: tel,
		})
	}

	return &data.GenericPhoneBook{Entry: targetEntries}
}

type Generic struct{}

func (g *Generic) Export(entries []*data.Entry, format Format, resolve, indicateActive, filterInactive bool) ([]byte, error) {
	return xml.MarshalIndent(struct {
		*data.GenericPhoneBook
		XMLName struct{} `xml:"IPPhoneDirectory"`
	}{
		GenericPhoneBook: export(entries, format, resolve, indicateActive, filterInactive),
	}, "", "    ")
}
