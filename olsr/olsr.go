package olsr

import (
	"bufio"
	"os"
	"regexp"
	"strings"

	"github.com/finfinack/phonebook/data"
)

const (
	commentPfx      = "#"
	filterNonPhones = true
)

var (
	hostsRE  = regexp.MustCompile(`([0-9\.]+)\s+(\S+)\s?#\s*(.*)`)
	phonesRE = regexp.MustCompile(`([0-9\.]+)\s+([0-9]+)\s?#\s*(.*)`)
)

func Read(path string) (map[string]*data.OLSR, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	scanner := bufio.NewScanner(f)
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
		d[parts[2]] = o
	}
	f.Close()

	return d, nil
}
