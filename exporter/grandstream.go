package exporter

import (
	"encoding/xml"
	"fmt"

	"github.com/arednch/phonebook/data"
)

const (
	GrandstreamDefaultIPCallAccountIdx = 0
	GrandstreamDefaultPBXAccountIdx    = 1
)

type GrandstreamPhonebook struct {
	Entry []*GrandstreamEntry `xml:"Contact"`
}

type GrandstreamEntry struct {
	FirstName string              `xml:"FirstName"`
	LastName  string              `xml:"LastName"`
	Phone     []*GrandstreamPhone `xml:"Phone"`
}

type GrandstreamPhone struct {
	AccountIndex int    `xml:"accountindex"` // 0 to 5
	PhoneNumber  string `xml:"phonenumber"`
}

type Grandstream struct{}

func (g *Grandstream) Export(entries []*data.Entry, format Format, activePfx string, resolve, indicateActive, filterInactive bool) ([]byte, error) {
	var targetEntries []*GrandstreamEntry
	for _, entry := range entries {
		if filterInactive && entry.OLSR == nil {
			continue // ignoring inactive entry (no OLSR data)
		}

		var pfx string
		if indicateActive && entry.OLSR != nil {
			pfx = activePfx
		}
		var firstname, lastname string
		switch {
		case entry.LastName == "" && entry.FirstName == "" && entry.Callsign == "":
			continue // there's no point in adding an empty contact
		case entry.LastName == "" && entry.FirstName == "":
			firstname = fmt.Sprintf("%s%s", pfx, entry.Callsign)
		case entry.LastName == "":
			firstname = fmt.Sprintf("%s%s (%s)", pfx, entry.FirstName, entry.Callsign)
		case entry.FirstName == "":
			firstname = fmt.Sprintf("%s%s", pfx, entry.Callsign)
			lastname = entry.LastName
		default:
			firstname = fmt.Sprintf("%s%s (%s)", pfx, entry.FirstName, entry.Callsign)
			lastname = entry.LastName
		}

		var tel []*GrandstreamPhone
		switch format {
		case "direct":
			if resolve && entry.OLSR != nil {
				tel = []*GrandstreamPhone{{
					AccountIndex: GrandstreamDefaultIPCallAccountIdx,
					PhoneNumber:  entry.OLSR.IP,
				}}
			} else {
				tel = []*GrandstreamPhone{{
					AccountIndex: GrandstreamDefaultIPCallAccountIdx,
					PhoneNumber:  entry.IPAddress,
				}}
			}
		case "pbx":
			tel = []*GrandstreamPhone{{
				AccountIndex: GrandstreamDefaultPBXAccountIdx,
				PhoneNumber:  entry.PhoneNumber,
			}}
		default:
			if resolve && entry.OLSR != nil {
				tel = []*GrandstreamPhone{
					{
						AccountIndex: GrandstreamDefaultIPCallAccountIdx,
						PhoneNumber:  entry.OLSR.IP,
					}, {
						AccountIndex: GrandstreamDefaultPBXAccountIdx,
						PhoneNumber:  entry.PhoneNumber,
					},
				}
			} else {
				tel = []*GrandstreamPhone{
					{
						AccountIndex: GrandstreamDefaultIPCallAccountIdx,
						PhoneNumber:  entry.IPAddress,
					}, {
						AccountIndex: GrandstreamDefaultPBXAccountIdx,
						PhoneNumber:  entry.PhoneNumber,
					},
				}
			}
		}
		targetEntries = append(targetEntries, &GrandstreamEntry{
			FirstName: firstname,
			LastName:  lastname,
			Phone:     tel,
		})
	}

	return xml.MarshalIndent(struct {
		*GrandstreamPhonebook
		XMLName struct{} `xml:"AddressBook"`
	}{
		GrandstreamPhonebook: &GrandstreamPhonebook{Entry: targetEntries},
	}, "", "    ")
}

/*
<AddressBook>
	<Contact>
		<FirstName>John</FirstName>
		<LastName>Doe</LastName>
		<Phone type="Other">
			<phonenumber>178230@178230.local.mesh</phonenumber>
			<accountindex>0</accountindex>
		</Phone>
	</Contact>
</AddressBook>
*/
