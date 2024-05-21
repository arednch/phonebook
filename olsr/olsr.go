package olsr

import (
	"bufio"
	"bytes"
	"encoding/json"
	"regexp"
	"strings"

	"github.com/arednch/phonebook/data"
	"github.com/arednch/phonebook/importer"
)

const (
	commentPfx      = "#"
	filterNonPhones = true
)

var (
	hostsRE         = regexp.MustCompile(`([0-9\.]+)\s+(\S+)\s?#\s*(.*)`)
	phonesRE        = regexp.MustCompile(`([0-9\.]+)\s+([0-9]+)\s?#\s*(.*)`)
	phoneHostnameRE = regexp.MustCompile(`^[0-9]+$`)
)

func ReadFromURL(url string) (map[string]*data.OLSR, error) {
	b, err := importer.ReadFromURL(url)
	if err != nil {
		return nil, err
	}

	var sysinfo data.SysInfo
	if err := json.Unmarshal(b, &sysinfo); err != nil {
		return nil, err
	}

	d := map[string]*data.OLSR{}
	for _, host := range sysinfo.Hosts {
		if filterNonPhones && !phoneHostnameRE.MatchString(host.Name) {
			continue
		}

		o := &data.OLSR{
			IP:       host.IP,
			Hostname: host.Name,
		}
		d[host.Name] = o
	}

	return d, nil
}

func ReadFromFile(path string) (map[string]*data.OLSR, error) {
	b, err := importer.ReadFromFile(path)
	if err != nil {
		return nil, err
	}

	scanner := bufio.NewScanner(bytes.NewReader(b))
	scanner.Split(bufio.ScanLines)

	d := map[string]*data.OLSR{}
	for scanner.Scan() {
		line := scanner.Text()
		line = strings.TrimSpace(line)

		switch {
		case line == "":
			continue
		case strings.HasPrefix(line, commentPfx):
			continue
		}

		var parts []string
		if filterNonPhones {
			parts = phonesRE.FindStringSubmatch(line)
		} else {
			parts = hostsRE.FindStringSubmatch(line)
		}
		if len(parts) < 3 {
			continue
		}

		o := &data.OLSR{
			IP:       parts[1],
			Hostname: parts[2],
		}
		if len(parts) > 3 {
			o.Comment = parts[3]
		}
		d[o.Hostname] = o
	}

	return d, nil
}
