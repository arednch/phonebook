package server

import (
	"encoding/json"
	"net/http"

	"github.com/arednch/phonebook/data"
)

func (s *Server) Info(w http.ResponseWriter, r *http.Request) {
	s.Records.Mu.RLock()
	defer s.Records.Mu.RUnlock()
	info := &data.WebInfo{
		WebDefault: *s.prepareDefaultData("Info", false),
		RecordStats: data.RecordStats{
			Count:   len(s.Records.Entries),
			Updated: s.Records.Updated,
		},
	}

	if s.RegisterCache != nil {
		info.Registered = make(map[string]string)
		for _, k := range s.RegisterCache.Keys() {
			v, ok := s.RegisterCache.Get(k)
			if !ok {
				continue
			}
			info.Registered[k] = v.UA
		}
	}

	if s.RuntimeInfo != nil && s.RuntimeInfo.SysInfo != nil {
		s.RuntimeInfo.Mu.RLock()
		defer s.RuntimeInfo.Mu.RUnlock()
		info.Runtime = data.Runtime{
			Updated: s.RuntimeInfo.Updated,
		}
		if s.RuntimeInfo.SysInfo.Node != "" {
			info.Runtime.Node = s.RuntimeInfo.SysInfo.Node
		}
		if s.RuntimeInfo.SysInfo.System.Uptime != "" {
			info.Runtime.Uptime = s.RuntimeInfo.SysInfo.System.Uptime
		}
		if s.RuntimeInfo.SysInfo.NodeDetails != nil {
			info.Runtime.Details = *s.RuntimeInfo.SysInfo.NodeDetails
		}
	}

	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		http.Error(w, "unable to marshal info", http.StatusInternalServerError)
		return
	}
	w.Write(data)
}
