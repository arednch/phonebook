package server

import (
	"net/http"
	"sort"
	"strings"

	"github.com/arednch/phonebook/data"
)

func (s *Server) Index(w http.ResponseWriter, r *http.Request) {
	var exp []string
	for k := range s.Exporters {
		exp = append(exp, k)
	}
	sort.Strings(exp)

	registered := make(map[string]string)
	if s.RegisterCache != nil {
		for _, k := range s.RegisterCache.Keys() {
			v, ok := s.RegisterCache.Get(k)
			if !ok {
				continue
			}
			registered[k] = v.UA
		}
	}

	s.Records.Mu.RLock()
	defer s.Records.Mu.RUnlock()

	recs := make(map[string]string)
	for _, e := range s.Records.Entries {
		var pfx string
		if s.Config.IndicateActive && e.OLSR != nil {
			pfx = s.Config.ActivePfx
		}
		recs[e.DisplayName(pfx)] = e.PhoneNumber
	}

	data := data.WebIndex{
		WebDefault: *s.prepareDefaultData("Overview", true),
		UpdateURLs: strings.Join(s.Config.UpdateURLs, "\n"),
		Sources:    strings.Join(s.Config.Sources, "\n"),
		Records:    recs,
		Exporters:  exp,
		Registered: registered,
	}
	if err := s.Tmpls.ExecuteTemplate(w, "index.html", data); err != nil {
		http.Error(w, "unable to write response", http.StatusInternalServerError)
	}
}
