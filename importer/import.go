package importer

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/arednch/phonebook/data"
)

const (
	headerFirstName   = "first_name"
	headerLastName    = "name"
	headerCallsign    = "callsign"
	headerPhoneNumber = "telephone"
	headerPrivate     = "privat"
)

func ReadFromURL(url string, cache string, client *http.Client) ([]byte, error) {
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	resp.Body.Close()

	if cache == "" {
		return body, nil
	}

	if err := os.WriteFile(cache, body, 0664); err != nil {
		fmt.Printf("Unable to write downloaded file to cache: %s\n", err)
	} else {
		fmt.Printf("Locally cached downloaded file: %q\n", cache)
	}
	return body, nil
}

func ReadFromFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func ReadPhonebook(path string, cache string, client *http.Client) ([]*data.Entry, error) {
	var blob []byte
	var err error
	switch {
	case strings.HasPrefix(path, "http://"):
		fallthrough
	case strings.HasPrefix(path, "https://"):
		blob, err = ReadFromURL(path, cache, client)
	case strings.HasPrefix(path, "/"):
		blob, err = ReadFromFile(path)
	default:
		err = errors.New("unknown or unsupported path scheme (needs to be a valid, absolute file path or http/https URL)")
	}
	if err != nil {
		return nil, err
	}

	reader := csv.NewReader(bytes.NewBuffer(blob))
	// read and index headers
	hdrs, err := reader.Read()
	if err != nil {
		return nil, err
	}
	headers := make(map[string]int)
	for i, v := range hdrs {
		headers[strings.ToLower(v)] = i
	}
	firstIdx, ok := headers[headerFirstName]
	if !ok {
		return nil, errors.New("unable to locate first name column in CSV")
	}
	lastIdx, ok := headers[headerLastName]
	if !ok {
		return nil, errors.New("unable to locate last name column in CSV")
	}
	callIdx, ok := headers[headerCallsign]
	if !ok {
		return nil, errors.New("unable to locate callsign column in CSV")
	}
	phoneIdx, ok := headers[headerPhoneNumber]
	if !ok {
		return nil, errors.New("unable to locate phone number column in CSV")
	}
	privateIdx, privateIdxAvailable := headers[headerPrivate]

	var records []*data.Entry
	for {
		r, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		// skip if we encounter the first empty line
		if strings.TrimSpace(r[firstIdx]) == "" && strings.TrimSpace(r[lastIdx]) == "" &&
			strings.TrimSpace(r[callIdx]) == "" && strings.TrimSpace(r[phoneIdx]) == "" {
			break
		}
		// check if entry is marked as private and if so, skip it
		if privateIdxAvailable && strings.ToLower(strings.TrimSpace(r[privateIdx])) == "y" {
			continue
		}

		entry := &data.Entry{
			FirstName:   strings.TrimSpace(r[firstIdx]),
			LastName:    strings.TrimSpace(r[lastIdx]),
			Callsign:    strings.TrimSpace(r[callIdx]),
			PhoneNumber: strings.TrimSpace(r[phoneIdx]),
		}
		records = append(records, entry)
	}

	return records, nil
}

func ReadSysInfoFromURL(url string, client *http.Client) (*data.SysInfo, error) {
	b, err := ReadFromURL(url, "", client)
	if err != nil {
		return nil, err
	}

	var sysinfo data.SysInfo
	if err := json.Unmarshal(b, &sysinfo); err != nil {
		return nil, err
	}

	return &sysinfo, nil
}

func ReadUpdatesFromURL(urls []string, client *http.Client) ([]*data.Update, error) {
	for _, url := range urls {
		b, err := ReadFromURL(url, "", client)
		if err != nil {
			continue
		}

		var updates data.Updates
		if err := json.Unmarshal(b, &updates); err != nil {
			continue
		}

		for _, u := range updates.Updates {
			u.Type = strings.TrimSpace(strings.ToLower(u.Type))
		}

		return updates.Updates, nil
	}
	return nil, errors.New("no URLs or none returned any updates")
}
