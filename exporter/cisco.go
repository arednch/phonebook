package exporter

import (
	"bytes"
	"encoding/xml"
	"fmt"

	"github.com/arednch/phonebook/data"
)

type CiscoPhonebook struct {
	Title  string `xml:"Title"`
	Prompt string `xml:"Prompt"`
}

type Cisco struct{}

func (c *Cisco) Export(entries []*data.Entry, format Format, activePfx string, resolve, indicateActive, filterInactive, debug bool) ([]byte, error) {
	b, err := xml.MarshalIndent(struct {
		*CiscoPhonebook
		*data.GenericPhoneBook
		XMLName struct{} `xml:"CiscoIPPhoneDirectory"`
	}{
		GenericPhoneBook: export(entries, format, activePfx, resolve, indicateActive, filterInactive, debug),
		CiscoPhonebook: &CiscoPhonebook{
			Title:  "Cisco Coporate Directory",
			Prompt: "Select the User",
		},
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
