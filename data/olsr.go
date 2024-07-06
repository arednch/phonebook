package data

import (
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
		OLSR:        o,
	}
}
