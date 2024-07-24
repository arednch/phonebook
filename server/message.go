package server

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/arednch/phonebook/data"
)

func (s *Server) SendMessage(w http.ResponseWriter, r *http.Request) {
	d := data.WebMessage{
		WebDefault: *s.prepareDefaultData("Send Message", false),
		Success:    true,
	}

	from := r.FormValue("from")
	from = strings.ToLower(strings.TrimSpace(from))
	if from == "" {
		if s.Config.Debug {
			fmt.Printf("/message: 'from' not specified: %s\n", from)
		}
		d.Success = false
		d.Message = "'from' not specified"
		if err := s.Tmpls.ExecuteTemplate(w, "message.html", d); err != nil {
			http.Error(w, "unable from write response", http.StatusInternalServerError)
		}
		return
	}

	s.Records.Mu.RLock()
	defer s.Records.Mu.RUnlock()

	if checkExistenceBeforeSending {
		if _, ok := s.RegisterCache.Get(from); !ok {
			if s.Config.Debug {
				fmt.Printf("/message: 'from' not in locally registered phones: %s\n", from)
			}
			d.Success = false
			d.Message = "'from' phone number is not locally registered"
			if err := s.Tmpls.ExecuteTemplate(w, "message.html", d); err != nil {
				http.Error(w, "unable to write response", http.StatusInternalServerError)
			}
			return
		}
	}

	to := r.FormValue("to")
	to = strings.ToLower(strings.TrimSpace(to))
	if to == "" {
		if s.Config.Debug {
			fmt.Printf("/message: 'to' not specified: %s\n", to)
		}
		d.Success = false
		d.Message = "'to' not specified"
		if err := s.Tmpls.ExecuteTemplate(w, "message.html", d); err != nil {
			http.Error(w, "unable to write response", http.StatusInternalServerError)
		}
		return
	}

	if checkExistenceBeforeSending {
		found := false
		for _, e := range s.Records.Entries {
			if e.PhoneNumber == to {
				found = true
				break
			}
		}
		if !found {
			if s.Config.Debug {
				fmt.Printf("/message: destination specified not found in phonebook: %s\n", to)
			}
			d.Success = false
			d.Message = "destination specified not found in phonebook"
			if err := s.Tmpls.ExecuteTemplate(w, "message.html", d); err != nil {
				http.Error(w, "unable to write response", http.StatusInternalServerError)
			}
			return
		}
	}

	msg := r.FormValue("msg")
	msg = strings.ToLower(strings.TrimSpace(msg))
	if to == "" {
		if s.Config.Debug {
			fmt.Printf("/message: 'msg' not specified: %s\n", to)
		}
		d.Success = false
		d.Message = "'msg' not specified"
		if err := s.Tmpls.ExecuteTemplate(w, "message.html", d); err != nil {
			http.Error(w, "unable to write response", http.StatusInternalServerError)
		}
		return
	}

	fe := &data.Entry{PhoneNumber: from}
	te := &data.Entry{PhoneNumber: to}
	for _, e := range s.Records.Entries {
		if from == e.PhoneNumber {
			fe = e
		}
		if to == e.PhoneNumber {
			te = e
		}
	}

	d.From = fmt.Sprintf("%s, %s", fe.DisplayName(""), from)
	d.To = fmt.Sprintf("%s, %s", te.DisplayName(""), to)
	d.Message = msg
	t := &data.SIPAddress{
		DisplayName: te.DisplayName(""),
		URI: &data.SIPURI{
			User: to,
			Host: te.PhoneFQDN(),
		},
	}
	f := &data.SIPAddress{
		DisplayName: te.DisplayName(""),
		URI: &data.SIPURI{
			User: from,
			Host: fe.PhoneFQDN(),
		},
	}
	hdrs := []*data.SIPHeader{
		{
			Name:  "Content-Type",
			Value: "text/plain",
		},
	}
	req := data.NewSIPRequest("MESSAGE", f, t, 1, hdrs, []byte(msg))
	if resp, err := s.SendSIPMessage(req); err != nil {
		if s.Config.Debug {
			fmt.Printf("/message: message could not be sent: %s\n", err)
		}
		d.Success = false
		d.Message = "message could not be sent"
	} else if resp.StatusCode != http.StatusOK {
		if s.Config.Debug {
			fmt.Printf("/message: message response not successful (%d)\n", resp.StatusCode)
		}
		d.Success = false
		d.Message = fmt.Sprintf("message sent but response not ok (%d)", resp.StatusCode)
	}
	if err := s.Tmpls.ExecuteTemplate(w, "message.html", d); err != nil {
		http.Error(w, "unable to write response", http.StatusInternalServerError)
	}
}
