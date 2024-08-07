package exporter

import (
	"bytes"
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
	case entry.LastName == "" && entry.FirstName == "" && entry.Callsign == "" && entry.PhoneNumber == "":
		return ""
	case entry.LastName == "" && entry.FirstName == "" && entry.Callsign == "":
		return fmt.Sprintf("%s%s", pfx, entry.PhoneNumber)
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
			return []string{entry.DirectCallAddress()}
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
				entry.DirectCallAddress(),
				entry.PhoneNumber,
			}
		}
	}
}

type Format string

type Exporter interface {
	Export(entries []*data.Entry, format Format, activePfx string, resolve, indicateActive, filterInactive, debug bool) ([]byte, error)
}

func export(entries []*data.Entry, format Format, activePfx string, resolve, indicateActive, filterInactive, debug bool) *data.GenericPhoneBook {
	var targetEntries []*data.GenericEntry
	for _, entry := range entries {
		if filterInactive && entry.OLSR == nil {
			if debug {
				fmt.Printf("Export/Generic: Filtering inactive entry: %+v\n", entry)
			}
			continue // ignoring inactive entry (no OLSR data)
		}

		name := NameForEntry(entry, indicateActive, activePfx)
		if name == "" {
			if debug {
				fmt.Printf("Export/Generic: Ignoring entry with empty contact: %+v\n", entry)
			}
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

func (g *Generic) Export(entries []*data.Entry, format Format, activePfx string, resolve, indicateActive, filterInactive, debug bool) ([]byte, error) {
	b, err := xml.MarshalIndent(struct {
		*data.GenericPhoneBook
		XMLName struct{} `xml:"IPPhoneDirectory"`
	}{
		GenericPhoneBook: export(entries, format, activePfx, resolve, indicateActive, filterInactive, debug),
	}, "", "    ")
	if err != nil {
		return nil, fmt.Errorf("unable to convert to XML: %s", err)
	}

	w := &bytes.Buffer{}
	w.WriteString(xml.Header)
	if _, err := w.Write(b); err != nil {
		return nil, fmt.Errorf("unable to write XML: %s", err)
	}
	return w.Bytes(), nil
}
