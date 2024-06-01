package exporter

import (
	"bufio"
	"bytes"

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

		name := NameForEntry(entry, indicateActive, activePfx)
		if name == "" {
			continue // ignore empty contacts
		}

		card := vcard.Card{}
		card.SetValue(vcard.FieldFormattedName, name)
		for _, tel := range TelefoneForEntry(entry, resolve, format) {
			card.AddValue(vcard.FieldTelephone, tel)
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
