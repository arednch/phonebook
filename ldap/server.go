package ldap

import (
	"fmt"
	"net"
	"regexp"
	"strings"

	"github.com/mark-rushakoff/ldapserver"

	"github.com/arednch/phonebook/configuration"
	"github.com/arednch/phonebook/data"
)

var (
	filterRE = regexp.MustCompile(`\(\w*=\*?([a-zA-Z0-9]+)\*?\)`)
)

type Server struct {
	Config *configuration.Config

	Records *data.Records
}

func (s *Server) Bind(bindDN, bindSimplePw string, conn net.Conn) (ldapserver.LDAPResultCode, error) {
	if bindDN == s.Config.LDAPUser && bindSimplePw == s.Config.LDAPPwd {
		return ldapserver.LDAPResultSuccess, nil
	}
	return ldapserver.LDAPResultInvalidCredentials, nil
}

func (s *Server) Search(boundDN string, searchReq ldapserver.SearchRequest, conn net.Conn) (ldapserver.ServerSearchResult, error) {
	var parts []string
	if searchReq.Filter != "" {
		parts = filterRE.FindStringSubmatch(searchReq.Filter)
	}

	entries := []*ldapserver.Entry{}
	for _, entry := range s.Records.Entries {
		if s.Config.FilterInactive && entry.OLSR == nil {
			continue // ignoring inactive entry (no OLSR data)
		}

		var pfx string
		if s.Config.IndicateActive && entry.OLSR != nil {
			pfx = s.Config.ActivePfx
		}
		var name string
		switch {
		case entry.LastName == "" && entry.FirstName == "" && entry.Callsign == "":
			continue // there's no point in adding an empty contact
		case entry.LastName == "" && entry.FirstName == "":
			name = fmt.Sprintf("%s%s", pfx, entry.Callsign)
		case entry.LastName == "":
			name = fmt.Sprintf("%s%s (%s)", pfx, entry.FirstName, entry.Callsign)
		case entry.FirstName == "":
			name = fmt.Sprintf("%s%s (%s)", pfx, entry.LastName, entry.Callsign)
		default:
			name = fmt.Sprintf("%s%s, %s (%s)", pfx, entry.LastName, entry.FirstName, entry.Callsign)
		}

		if searchReq.Filter != "" && len(parts) > 1 && !strings.Contains(strings.ToLower(name), strings.ToLower(parts[1])) {
			continue
		}

		attrs := []*ldapserver.EntryAttribute{
			{Name: "objectClass", Values: []string{"person"}},
			{Name: "displayName", Values: []string{name}},
			{Name: "cn", Values: []string{name}},
			{Name: "firstname", Values: []string{entry.FirstName}},
			{Name: "gn", Values: []string{entry.FirstName}},
			{Name: "lastname", Values: []string{entry.LastName}},
			{Name: "sn", Values: []string{entry.LastName}},
			{Name: "callsign", Values: []string{entry.Callsign}},

			{Name: "telephoneNumber", Values: []string{entry.PhoneNumber}},
			{Name: "telephoneHostname", Values: []string{entry.IPAddress}},
		}

		if entry.OLSR != nil {
			attrs = append(attrs, []*ldapserver.EntryAttribute{
				{Name: "telephoneIP", Values: []string{entry.OLSR.IP}},
			}...)
		}

		entries = append(entries, &ldapserver.Entry{
			DN:         fmt.Sprintf("cn=%s,dc=aredn,%s", name, searchReq.BaseDN),
			Attributes: attrs,
		})
	}

	fmt.Println(len(entries))
	return ldapserver.ServerSearchResult{
		Entries:    entries,
		Referrals:  []string{},
		Controls:   []ldapserver.Control{},
		ResultCode: ldapserver.LDAPResultSuccess,
	}, nil
}
