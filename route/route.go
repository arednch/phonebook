package route

import (
	"regexp"

	"github.com/arednch/phonebook/data"
)

const (
	filterNonPhones = true
)

var (
	phoneHostnameRE = regexp.MustCompile(`^[0-9]+$`)
)

func ReadFromSysInfo(sysinfo *data.SysInfo) (map[string]*data.RouteEntry, error) {
	d := map[string]*data.RouteEntry{}
	for _, host := range sysinfo.Hosts {
		if filterNonPhones && !phoneHostnameRE.MatchString(host.Name) {
			continue
		}

		o := &data.RouteEntry{
			IP:       host.IP,
			Hostname: host.Name,
		}
		d[host.Name] = o
	}

	return d, nil
}
