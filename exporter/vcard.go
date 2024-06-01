package exporter

import (
	"bufio"
	"bytes"
	"fmt"

	"github.com/emersion/go-vcard"

	"github.com/arednch/phonebook/data"
)

type VCard struct{}

func (v *VCard) Export(entries []*data.Entry, format Format, activePfx string, resolve, indicateActive, filterInactive bool) ([]byte, error) {
	var b bytes.Buffer
	out := bufio.NewWriter(&b)
	enc := vcard.NewEncoder(out)

	for _, entry := range entries {
		if filterInactive && entry.OLSR == nil {
			continue // ignoring inactive entry (no OLSR data)
		}

		var pfx string
		if indicateActive && entry.OLSR != nil {
			pfx = activePfx
		}
		var name string
		switch {
		case entry.LastName == "" && entry.FirstName == "" && entry.Callsign == "":
			continue // there's no point in adding an empty contact
		case entry.LastName == "" && entry.FirstName == "":
			name = fmt.Sprintf("%s%s", pfx, entry.Callsign)
		case entry.LastName == "":
			name = fmt.Sprintf("%s%s (%s)", pfx, entry.FirstName, entry.Callsign)
		case entry.FirstName == "":
			name = fmt.Sprintf("%s%s (%s)", pfx, entry.LastName, entry.Callsign)
		default:
			name = fmt.Sprintf("%s%s, %s (%s)", pfx, entry.LastName, entry.FirstName, entry.Callsign)
		}

		card := vcard.Card{}
		card.SetValue(vcard.FieldFormattedName, name)

		switch format {
		case "direct":
			if resolve && entry.OLSR != nil {
				card.SetValue(vcard.FieldTelephone, entry.OLSR.IP)
			} else {
				card.SetValue(vcard.FieldTelephone, entry.IPAddress)
			}
		case "pbx":
			card.SetValue(vcard.FieldTelephone, entry.PhoneNumber)
		default:
			if resolve && entry.OLSR != nil {
				card.SetValue(vcard.FieldTelephone, entry.OLSR.IP)
				card.AddValue(vcard.FieldTelephone, entry.PhoneNumber)
			} else {
				card.SetValue(vcard.FieldTelephone, entry.IPAddress)
				card.AddValue(vcard.FieldTelephone, entry.PhoneNumber)
			}
		}

		// set the value of a field and other parameters by using card.Set
		card.Set(vcard.FieldName, &vcard.Field{
			Value: name,
			Params: map[string][]string{
				vcard.ParamSortAs: {
					entry.LastName,
					entry.Callsign,
					entry.FirstName,
				},
			},
		})

		vcard.ToV4(card) // make the vCard version 4 compliant
		if err := enc.Encode(card); err != nil {
			return nil, err
		}
	}

	out.Flush()
	return b.Bytes(), nil
}
