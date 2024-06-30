package data

import (
	"fmt"
	"strings"
)

type OLSR struct {
	IP       string
	Hostname string
	Comment  string
}

func NewEntryFromOLSR(o *OLSR) *Entry {
	pn := strings.Split(o.Hostname, ".")[0]
	return &Entry{
		PhoneNumber: pn,
		IPAddress:   fmt.Sprintf("%s@%s.%s", pn, pn, AREDNDomain),
		OLSR:        o,
	}
}
