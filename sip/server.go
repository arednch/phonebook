package sip

import (
	"fmt"
	"strings"

	"github.com/emiago/sipgo"
	"github.com/emiago/sipgo/sip"

	"github.com/arednch/phonebook/configuration"
	"github.com/arednch/phonebook/data"
)

type Server struct {
	Config *configuration.Config

	Records *data.Records

	UA  *sipgo.UserAgent
	Srv *sipgo.Server

	// Local hostnames and IPs to react to.
	LocalIdentities map[string]bool
}

func (s *Server) OnRegister(req *sip.Request, tx sip.ServerTransaction) {
	if s.Config.Debug {
		fmt.Printf("SIP/Register: received REGISTER message from %s\n", req.Source())
	}
	// Respond with OK in all cases. No credentials are checked.
	if err := tx.Respond(sip.NewResponseFromRequest(req, sip.StatusOK, "OK", nil)); err != nil {
		fmt.Printf("SIP/Register: error sending response: %s\n", err)
	}
}

func (s *Server) OnInvite(req *sip.Request, tx sip.ServerTransaction) {
	if s.Config.Debug {
		fmt.Printf("SIP/Invite: received INVITE message from %q to %q\n", req.From(), req.To())
	}

	// Check if this is a call directed at a local identity (hostname or IP). If not, ignore it.
	// This also helps reducing retry storms for some clients (e.g. Linphone).
	if s.LocalIdentities != nil {
		if local, ok := s.LocalIdentities[req.To().Address.Host]; !ok || !local {
			if err := tx.Respond(sip.NewResponseFromRequest(req, sip.StatusNotFound, "Not Found", nil)); err != nil {
				fmt.Printf("SIP/Invite: error sending response: %s\n", err)
			}
			return
		}
	}

	// Look up the phone number and try to find the right host in our records and redirect the call there.
	var redirect *sip.Uri
	to := req.To()
	s.Records.Mu.RLock()
	for _, entry := range s.Records.Entries {
		if entry.PhoneNumber != to.Address.User {
			continue
		}

		host := fmt.Sprintf("%s.local.mesh", entry.PhoneNumber)
		parts := strings.Split(entry.IPAddress, data.SIPSeparator)
		if len(parts) > 1 {
			if s.Config.Resolve && entry.OLSR != nil {
				host = entry.OLSR.IP
			} else {
				host = parts[1]
			}
		}

		// We found an entry in the phonebook to redirect to.
		redirect = &sip.Uri{
			User: entry.PhoneNumber,
			Host: host,
			// Port: 5060,
			// UriParams: sip.HeaderParams{
			// 	"Transport": "udp",
			// },
		}
		break
	}
	s.Records.Mu.RUnlock()

	if redirect != nil {
		resp := sip.NewResponseFromRequest(req, sip.StatusMovedTemporarily, "Moved Temporarily", nil)
		// resp.RemoveHeader("Via")
		resp.AppendHeaderAfter(&sip.ContactHeader{
			DisplayName: "AREDN Direct IP Call Transfer",
			Address:     *redirect,
		}, "To")
		if err := tx.Respond(resp); err != nil {
			fmt.Printf("SIP/Invite: error sending response: %s\n", err)
		}
		return
	}

	// As a last resort, we're giving up and tell the client that we can't route that call.
	if err := tx.Respond(sip.NewResponseFromRequest(req, sip.StatusNotFound, "Not Found", nil)); err != nil {
		fmt.Printf("SIP/Invite: error sending response: %s\n", err)
	}
}

func (s *Server) OnBye(req *sip.Request, tx sip.ServerTransaction) {
	if err := tx.Respond(sip.NewResponseFromRequest(req, sip.StatusOK, "OK", nil)); err != nil {
		fmt.Printf("SIP/Bye: error sending response: %s\n", err)
	}
}
