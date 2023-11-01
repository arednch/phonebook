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
	eofSignal = "ENDOFFILE"
)

func readFromURL(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func readFromFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func ReadPhonebook(path string) ([]*data.Entry, error) {
	var blob []byte
	var err error
	switch {
	case strings.HasPrefix(path, "http://"):
		fallthrough
	case strings.HasPrefix(path, "https://"):
		blob, err = readFromURL(path)
	default:
		blob, err = readFromFile(path)
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

		records = append(records, &data.Entry{
			FirstName:   strings.TrimSpace(r[0]),
			LastName:    strings.TrimSpace(r[1]),
			Callsign:    strings.TrimSpace(r[2]),
			IPAddress:   strings.TrimSpace(r[3]),
			PhoneNumber: strings.TrimSpace(r[4]),
		})
	}

	return records, nil
}
