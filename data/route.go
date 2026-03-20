package data

import (
	"strings"
)

type RouteEntry struct {
	IP       string
	Hostname string
}

func NewEntryFromRoute(o *RouteEntry) *Entry {
	pn := strings.Split(o.Hostname, ".")[0]
	return &Entry{
		PhoneNumber: pn,
		Route:       o,
	}
}
