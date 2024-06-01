package exporter

import (
	"encoding/xml"
	"fmt"

	"github.com/arednch/phonebook/data"
)

const (
	FormatCombined = Format("combined")
	FormatDirect   = Format("direct")
	FormatPBX      = Format("pbx")
)

func NameForEntry(entry *data.Entry, indicateActive bool, activePfx string) string {
	var pfx string
	if indicateActive && entry.OLSR != nil {
		pfx = activePfx
	}
	switch {
	case entry.LastName == "" && entry.FirstName == "" && entry.Callsign == "":
		return ""
	case entry.LastName == "" && entry.FirstName == "":
		return fmt.Sprintf("%s%s", pfx, entry.Callsign)
	case entry.LastName == "":
		return fmt.Sprintf("%s%s (%s)", pfx, entry.FirstName, entry.Callsign)
	case entry.FirstName == "":
		return fmt.Sprintf("%s%s (%s)", pfx, entry.LastName, entry.Callsign)
	default:
		return fmt.Sprintf("%s%s, %s (%s)", pfx, entry.LastName, entry.FirstName, entry.Callsign)
	}
}

func TelefoneForEntry(entry *data.Entry, resolve bool, format Format) []string {
	switch format {
	case "direct":
		if resolve && entry.OLSR != nil {
			return []string{entry.OLSR.IP}
		} else {
			return []string{entry.IPAddress}
		}
	case "pbx":
		return []string{entry.PhoneNumber}
	default:
		if resolve && entry.OLSR != nil {
			return []string{
				entry.OLSR.IP,
				entry.PhoneNumber,
			}
		} else {
			return []string{
				entry.IPAddress,
				entry.PhoneNumber,
			}
		}
	}
}

type Format string

type Exporter interface {
	Export([]*data.Entry, Format, string, bool, bool, bool) ([]byte, error)
}

func export(entries []*data.Entry, format Format, activePfx string, resolve, indicateActive, filterInactive bool) *data.GenericPhoneBook {
	var targetEntries []*data.GenericEntry
	for _, entry := range entries {
		if filterInactive && entry.OLSR == nil {
			continue // ignoring inactive entry (no OLSR data)
		}

		name := NameForEntry(entry, indicateActive, activePfx)
		if name == "" {
			continue // ignore empty contacts
		}
		targetEntries = append(targetEntries, &data.GenericEntry{
			Name:      name,
			Telephone: TelefoneForEntry(entry, resolve, format),
		})
	}

	return &data.GenericPhoneBook{Entry: targetEntries}
}

type Generic struct{}

func (g *Generic) Export(entries []*data.Entry, format Format, activePfx string, resolve, indicateActive, filterInactive bool) ([]byte, error) {
	return xml.MarshalIndent(struct {
		*data.GenericPhoneBook
		XMLName struct{} `xml:"IPPhoneDirectory"`
	}{
		GenericPhoneBook: export(entries, format, activePfx, resolve, indicateActive, filterInactive),
	}, "", "    ")
}
