package importer

import (
	"bytes"
	"encoding/csv"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/arednch/phonebook/data"
)

const (
	eofSignal   = "ENDOFFILE"
	privateMark = "Y"
)

func ReadFromURL(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func ReadFromFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func ReadPhonebook(path string) ([]*data.Entry, error) {
	var blob []byte
	var err error
	switch {
	case strings.HasPrefix(path, "http://"):
		fallthrough
	case strings.HasPrefix(path, "https://"):
		blob, err = ReadFromURL(path)
	default:
		blob, err = ReadFromFile(path)
	}
	if err != nil {
		return nil, err
	}

	reader := csv.NewReader(bytes.NewBuffer(blob))

	var count int
	var records []*data.Entry
	for {
		r, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		count++
		// skip header
		if count == 1 {
			continue
		}
		// phonebook's last line seems to contain an EOF flag
		if strings.EqualFold(r[0], eofSignal) {
			break
		}
		// also skip if we encounter the first empty line
		if strings.TrimSpace(r[0]) == "" && strings.TrimSpace(r[1]) == "" &&
			strings.TrimSpace(r[2]) == "" && strings.TrimSpace(r[3]) == "" &&
			strings.TrimSpace(r[4]) == "" {
			break
		}
		// check if entry is marked as private and if so, skip it
		if len(r) > 11 && strings.EqualFold(strings.TrimSpace(r[11]), privateMark) {
			continue
		}

		entry := &data.Entry{
			FirstName:   strings.TrimSpace(r[0]),
			LastName:    strings.TrimSpace(r[1]),
			Callsign:    strings.TrimSpace(r[2]),
			IPAddress:   strings.TrimSpace(r[3]),
			PhoneNumber: strings.TrimSpace(r[4]),
		}
		if len(r) > 9 {
			entry.Email = strings.TrimSpace(r[5])
			entry.Club = strings.TrimSpace(r[6])
			entry.Mobile = strings.TrimSpace(r[7])
			entry.Street = strings.TrimSpace(r[8])
			entry.City = strings.TrimSpace(r[9])
		}
		records = append(records, entry)
	}

	return records, nil
}
