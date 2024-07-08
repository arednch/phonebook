package olsr

import (
	"bufio"
	"bytes"
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

func ReadFromSysInfo(sysinfo *data.SysInfo) (map[string]*data.OLSR, error) {
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
		d[o.Hostname] = o
	}

	return d, nil
}
