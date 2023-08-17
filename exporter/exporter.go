package exporter

import "github.com/finack/phonebook/data"

type Exporter interface {
	Export([]*data.Entry, bool) ([]byte, error)
}
