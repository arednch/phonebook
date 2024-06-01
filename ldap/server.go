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
			name = fmt.Sprintf("%s%s %s (%s)", pfx, entry.LastName, entry.FirstName, entry.Callsign)
		}

		// provide a super simple way to filter entries
		if searchReq.Filter != "" && len(parts) > 1 && !strings.Contains(strings.ToLower(name), strings.ToLower(parts[1])) {
			continue
		}

		var telAttrs []string
		for _, frmt := range s.Config.Formats {
			switch frmt {
			case "direct":
				if s.Config.Resolve && entry.OLSR != nil {
					telAttrs = append(telAttrs, entry.OLSR.IP)
				} else {
					telAttrs = append(telAttrs, entry.IPAddress)
				}
			case "pbx":
				telAttrs = append(telAttrs, entry.PhoneNumber)
			default:
				if s.Config.Resolve && entry.OLSR != nil {
					telAttrs = append(telAttrs, entry.OLSR.IP, entry.PhoneNumber)
				} else {
					telAttrs = append(telAttrs, entry.IPAddress, entry.PhoneNumber)
				}
			}
		}

		attrs := []*ldapserver.EntryAttribute{
			{Name: "objectClass", Values: []string{"person"}},

			{Name: "displayName", Values: []string{name}},
			{Name: "cn", Values: []string{name}},
			{Name: "meshName", Values: []string{name}},
			{Name: "firstname", Values: []string{entry.FirstName}},
			{Name: "gn", Values: []string{entry.FirstName}},
			{Name: "lastname", Values: []string{entry.LastName}},
			{Name: "sn", Values: []string{entry.LastName}},
			{Name: "callsign", Values: []string{entry.Callsign}},

			{Name: "telephoneNumber", Values: []string{entry.PhoneNumber}},
			{Name: "telephoneHostname", Values: []string{entry.IPAddress}},
		}
		if entry.OLSR != nil {
			attrs = append(attrs, &ldapserver.EntryAttribute{Name: "telephoneIP", Values: []string{entry.OLSR.IP}})
		}

		// Populate Linphone default as a single address.
		for _, tel := range telAttrs {
			attrs = append(attrs, []*ldapserver.EntryAttribute{
				{Name: "sipPhone", Values: []string{tel}},
			}...)
		}

		entries = append(entries, &ldapserver.Entry{
			DN:         fmt.Sprintf("sn=%s,%s", name, searchReq.BaseDN),
			Attributes: attrs,
		})
		// limit search results to first X hits
		if searchReq.SizeLimit > 0 && len(entries) >= searchReq.SizeLimit {
			break
		}
	}
	return ldapserver.ServerSearchResult{
		Entries:    entries,
		Referrals:  []string{},
		Controls:   []ldapserver.Control{},
		ResultCode: ldapserver.LDAPResultSuccess,
	}, nil
}