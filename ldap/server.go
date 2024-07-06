package ldap

import (
	"encoding/binary"
	"fmt"
	"net"
	"regexp"
	"sort"
	"strings"

	"github.com/mark-rushakoff/ldapserver"

	"github.com/arednch/phonebook/configuration"
	"github.com/arednch/phonebook/data"
)

var (
	filterRE = regexp.MustCompile(`\(\w*=\*?([a-zA-Z0-9]+)\*?\)`)
)

func CookieToIdx(c []byte) uint32 {
	return binary.LittleEndian.Uint32(c)
}

func IdxToCookie(idx uint32) []byte {
	c := make([]byte, 4)
	binary.LittleEndian.PutUint32(c, idx)
	return c
}

type Server struct {
	Config *configuration.Config

	Records *data.Records
}

func (s *Server) Bind(bindDN, bindSimplePw string, conn net.Conn) (ldapserver.LDAPResultCode, error) {
	if bindDN == s.Config.LDAPUser && bindSimplePw == s.Config.LDAPPwd {
		if s.Config.Debug {
			fmt.Printf("LDAP/Bind: Request for DN %q (valid credentials)\n", bindDN)
		}
		return ldapserver.LDAPResultSuccess, nil
	}
	if s.Config.Debug {
		fmt.Printf("LDAP/Bind: Request for DN %q (invalid credentials)\n", bindDN)
	}
	return ldapserver.LDAPResultInvalidCredentials, nil
}

func (s *Server) Search(boundDN string, searchReq ldapserver.SearchRequest, conn net.Conn) (ldapserver.ServerSearchResult, error) {
	var searchQuery string
	if searchReq.Filter != "" {
		parts := filterRE.FindStringSubmatch(searchReq.Filter)
		if len(parts) > 1 {
			searchQuery = strings.ToLower(parts[1])
		}
	}
	if s.Config.Debug {
		fmt.Printf("LDAP/Search: Search filter %q, searching for %q\n", searchReq.Filter, searchQuery)
	}
	s.Records.Mu.RLock()
	defer s.Records.Mu.RUnlock()
	sort.Sort(data.ByName(s.Records.Entries))

	// Populate a (sorted) list of results for the given search query.
	entries := []*ldapserver.Entry{}
	for _, entry := range s.Records.Entries {
		if s.Config.FilterInactive && entry.OLSR == nil {
			if s.Config.Debug {
				fmt.Printf("LDAP/Search: Filtering inactive entry: %+v\n", entry)
			}
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
		if searchQuery != "" && !strings.Contains(strings.ToLower(name), searchQuery) {
			if s.Config.Debug {
				fmt.Printf("LDAP/Search: Filtering entry %q not matching search: %+v\n", name, entry)
			}
			continue
		}

		telAttrs := map[string]bool{}
		for _, frmt := range s.Config.Formats {
			switch frmt {
			case "direct":
				if s.Config.Resolve && entry.OLSR != nil {
					telAttrs[entry.OLSR.IP] = true
				} else {
					telAttrs[entry.DirectCallAddress()] = true
				}
			case "pbx":
				telAttrs[entry.PhoneNumber] = true
			default:
				if s.Config.Resolve && entry.OLSR != nil {
					telAttrs[entry.OLSR.IP] = true
					telAttrs[entry.PhoneNumber] = true
				} else {
					telAttrs[entry.DirectCallAddress()] = true
					telAttrs[entry.PhoneNumber] = true
				}
			}
		}

		attrs := []*ldapserver.EntryAttribute{
			{Name: "objectClass", Values: []string{"person"}},

			{Name: "displayname", Values: []string{name}},
			{Name: "cn", Values: []string{name}},
			{Name: "meshname", Values: []string{name}},
			{Name: "firstname", Values: []string{entry.FirstName}},
			{Name: "gn", Values: []string{entry.FirstName}},
			{Name: "lastname", Values: []string{entry.LastName}},
			{Name: "sn", Values: []string{entry.LastName}},
			{Name: "callsign", Values: []string{entry.Callsign}},

			{Name: "telephoneNumber", Values: []string{entry.PhoneNumber}},
			{Name: "telephoneHostname", Values: []string{entry.DirectCallAddress()}},
		}
		if entry.OLSR != nil {
			attrs = append(attrs, &ldapserver.EntryAttribute{Name: "telephoneIP", Values: []string{entry.OLSR.IP}})
		}

		// Populate Linphone default as a single address.
		for k := range telAttrs {
			attrs = append(attrs, []*ldapserver.EntryAttribute{
				{Name: "sipPhone", Values: []string{k}},
			}...)
		}

		entries = append(entries, &ldapserver.Entry{
			DN:         fmt.Sprintf("sn=%s,%s", name, searchReq.BaseDN),
			Attributes: attrs,
		})
	}

	// If there's no search size limit or fewer entries than the search size limit, we return them immediately.
	if searchReq.SizeLimit <= 0 || len(entries) <= searchReq.SizeLimit {
		return ldapserver.ServerSearchResult{
			Entries:    entries,
			Referrals:  []string{},
			Controls:   []ldapserver.Control{},
			ResultCode: ldapserver.LDAPResultSuccess,
		}, nil
	}

	// Based on the overall results, filter down the ones to return based on size limit and paging.
	// First, get a potentially already existing paging control.
	var ctrl *ldapserver.ControlPaging
	for _, c := range searchReq.Controls {
		if c.GetControlType() == ldapserver.ControlTypePaging {
			ctrl = c.(*ldapserver.ControlPaging)
			break
		}
	}
	if ctrl == nil {
		ctrl = ldapserver.NewControlPaging(uint32(searchReq.SizeLimit))
	}

	// Determine where we left of last time.
	start := uint32(0)
	if ctrl.Cookie != nil {
		start = CookieToIdx(ctrl.Cookie)
	}
	results := []*ldapserver.Entry{}
	for i, entry := range entries {
		if uint32(i) < start {
			// Ignore results that we already returned.
			continue
		}
		if len(results) >= searchReq.SizeLimit {
			if s.Config.Debug {
				fmt.Printf("LDAP/Search: Reached search size limit provided by client (%d). Returning %d out of %d results.\n", searchReq.SizeLimit, len(results), len(entries))
			}
			break
		}
		results = append(results, entry)
	}
	ctrl.SetCookie(IdxToCookie(start + uint32(len(results))))
	return ldapserver.ServerSearchResult{
		Entries:   results,
		Referrals: []string{},
		Controls: []ldapserver.Control{
			ctrl,
		},
		ResultCode: ldapserver.LDAPResultSuccess,
	}, nil
}
