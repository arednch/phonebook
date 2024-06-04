package exporter

import (
	"bytes"
	"encoding/xml"
	"fmt"

	"github.com/arednch/phonebook/data"
)

type Yealink struct{}

func (y *Yealink) Export(entries []*data.Entry, format Format, activePfx string, resolve, indicateActive, filterInactive, debug bool) ([]byte, error) {
	b, err := xml.MarshalIndent(struct {
		*data.GenericPhoneBook
		XMLName struct{} `xml:"YealinkIPPhoneDirectory"`
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
