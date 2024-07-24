package server

import (
	"fmt"
	"net/http"
	"time"

	"github.com/arednch/phonebook/data"
)

func (s *Server) ReloadPhonebook(w http.ResponseWriter, r *http.Request) {
	data := data.WebReload{
		WebDefault: *s.prepareDefaultData("Reload", false),
		Success:    true,
	}
	if src, err := s.ReloadFn(s.Config, s.Client); err != nil {
		data.Success = false
		if s.Config.Debug {
			fmt.Printf("/reload: unable to reload phonebook: %s\n", err)
		}
	} else {
		data.Source = src
		data.Updated = s.Records.Updated.Format(time.RFC3339)
		if s.Config.Debug {
			fmt.Printf("/reload: phonebook reloaded from %q\n", src)
		}
	}
	if err := s.Tmpls.ExecuteTemplate(w, "reload.html", data); err != nil {
		http.Error(w, "unable to write response", http.StatusInternalServerError)
	}
}
