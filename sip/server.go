package sip

import (
	"context"
	"fmt"
	"net"
	"net/http"

	"github.com/arednch/phonebook/configuration"
	"github.com/arednch/phonebook/data"
)

type Server struct {
	Config *configuration.Config

	Records *data.Records

	// Local hostnames and IPs to react to.
	LocalIdentities map[string]bool
}

func (s *Server) ListenAndServe(ctx context.Context, proto, addr string) error {
	pc, err := net.ListenPacket(proto, addr)
	if err != nil {
		return fmt.Errorf("SIP: unable to listen: %s", err)
	}
	defer pc.Close()

	for {
		buf := make([]byte, 1024)
		n, addr, err := pc.ReadFrom(buf)
		if err != nil {
			continue
		}
		go func(pc net.PacketConn, addr net.Addr, buf []byte) {
			req := &data.SIPRequest{}
			if err := req.Parse(buf); err != nil {
				return
			}

			resp, err := s.handleRequest(req)
			if err != nil {
				return
			}
			if resp == nil {
				return
			}

			pc.WriteTo(resp.Serialize(), addr)
		}(pc, addr, buf[:n])
	}
}

func (s *Server) handleRequest(req *data.SIPRequest) (*data.SIPResponse, error) {
	var resp *data.SIPResponse

	switch req.Method {
	case "REGISTER":
		return s.handleRegister(req)
	case "INVITE":
		return s.handleInvite(req)
	case "ACK":
		return s.handleAck(req)
	case "BYE":
		return s.handleBye(req)
	default:
		resp = data.NewSIPResponseFromRequest(req, http.StatusMethodNotAllowed, "Method Not Allowed")
	}

	return resp, nil
}

func (s *Server) handleRegister(req *data.SIPRequest) (*data.SIPResponse, error) {
	resp := data.NewSIPResponseFromRequest(req, http.StatusOK, "OK")
	return resp, nil
}

func (s *Server) handleAck(_ *data.SIPRequest) (*data.SIPResponse, error) {
	return nil, nil
}

func (s *Server) handleBye(req *data.SIPRequest) (*data.SIPResponse, error) {
	resp := data.NewSIPResponseFromRequest(req, http.StatusOK, "OK")
	return resp, nil
}

func (s *Server) handleInvite(_ *data.SIPRequest) (*data.SIPResponse, error) {
	return nil, nil
}

// func (s *Server) OnInvite(req *sip.Request, tx sip.ServerTransaction) {
// 	if s.Config.Debug {
// 		fmt.Printf("SIP/INVITE: received INVITE message from %q to %q\n", req.From(), req.To())
// 	}

// 	// Check if this is a call directed at a local identity (hostname or IP). If not, ignore it.
// 	// This also helps reducing retry storms for some clients (e.g. Linphone).
// 	if s.LocalIdentities != nil {
// 		if local, ok := s.LocalIdentities[strings.ToLower(req.To().Address.Host)]; !ok || !local {
// 			if err := tx.Respond(sip.NewResponseFromRequest(req, sip.StatusNotFound, "Not Found", nil)); err != nil {
// 				fmt.Printf("SIP/INVITE: error sending response: %s\n", err)
// 			}
// 			return
// 		}
// 	}

// 	// Look up the phone number and try to find the right host in our records and redirect the call there.
// 	var redirect *sip.Uri
// 	to := req.To()
// 	s.Records.Mu.RLock()
// 	for _, entry := range s.Records.Entries {
// 		if entry.PhoneNumber != to.Address.User {
// 			continue
// 		}

// 		host := fmt.Sprintf("%s.local.mesh", entry.PhoneNumber)
// 		parts := strings.Split(entry.IPAddress, data.SIPSeparator)
// 		if len(parts) > 1 {
// 			if s.Config.Resolve && entry.OLSR != nil {
// 				host = entry.OLSR.IP
// 			} else {
// 				host = parts[1]
// 			}
// 		}

// 		// We found an entry in the phonebook to redirect to.
// 		redirect = &sip.Uri{
// 			User: entry.PhoneNumber,
// 			Host: host,
// 			// Port: 5060,
// 			// UriParams: sip.HeaderParams{
// 			// 	"Transport": "udp",
// 			// },
// 		}
// 		break
// 	}
// 	s.Records.Mu.RUnlock()

// 	if redirect != nil {
// 		resp := sip.NewResponseFromRequest(req, sip.StatusMovedTemporarily, "Moved Temporarily", nil)
// 		resp.AppendHeaderAfter(&sip.ContactHeader{
// 			DisplayName: redirect.User,
// 			Address:     *redirect,
// 		}, "To")
// 		resp.AppendHeaderAfter(sip.NewHeader("Diversion", fmt.Sprintf("\"%s\" <%s>;reason=unconditional", redirect.User, redirect.String())), "To")
// 		if err := tx.Respond(resp); err != nil {
// 			fmt.Printf("SIP/INVITE: error sending response: %s\n", err)
// 		}
// 		return
// 	}

// 	// As a last resort, we're giving up and tell the client that we can't route that call.
// 	if err := tx.Respond(sip.NewResponseFromRequest(req, sip.StatusNotFound, "Not Found", nil)); err != nil {
// 		fmt.Printf("SIP/INVITE: error sending response: %s\n", err)
// 	}
// }
