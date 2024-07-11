package sip

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/arednch/phonebook/configuration"
	"github.com/arednch/phonebook/data"
)

const (
	registerExpiration = 10 * time.Minute

	// UDP Port where phones are expected to listen on.
	expectedPhoneSIPPort = 5060
)

type Server struct {
	Config *configuration.Config

	Records       *data.Records
	RegisterCache *data.TTLCache[string, *data.SIPClient]

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
		buf := make([]byte, 4096)
		n, addr, err := pc.ReadFrom(buf)
		if err != nil || n == 0 {
			continue
		}

		go func(pc net.PacketConn, addr net.Addr, buf []byte) {
			if s.Config.Debug {
				fmt.Printf("SIP/Request:\n%+v\n", string(buf))
			}

			req := &data.SIPRequest{}
			if err := req.Parse(buf); err != nil {
				return
			}

			resp, err := s.handleRequest(req)
			if err != nil || resp == nil {
				return
			}

			if s.Config.Debug {
				fmt.Printf("SIP/Response:\n%+v\n", string(resp.Serialize()))
			}

			pc.WriteTo(resp.Serialize(), addr)
		}(pc, addr, buf[:n])
	}
}

func (s Server) SendSIPMessage(req *data.SIPRequest) (*data.SIPResponse, error) {
	raddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", req.To().URI.Host, expectedPhoneSIPPort))
	if err != nil {
		return nil, fmt.Errorf("error resolving the destination address: %s", err)
	}

	conn, err := net.DialUDP("udp", nil, raddr)
	if err != nil {
		return nil, fmt.Errorf("error connecting: %s", err)
	}
	defer conn.Close()

	if _, err := conn.Write(req.Serialize()); err != nil {
		return nil, fmt.Errorf("error sending message: %s", err)
	}
	if s.Config.Debug {
		fmt.Printf("SIP/SendSIPMessage:\n%+v\n", string(req.Serialize()))
	}

	received := make([]byte, 1024)
	if _, err = conn.Read(received); err != nil {
		return nil, fmt.Errorf("unable to receive response: %s", err)
	}
	resp := &data.SIPResponse{}
	if err := resp.Parse(received); err != nil {
		return nil, fmt.Errorf("unable to parse body: %s", err)
	}
	return resp, nil
}

func (s *Server) handleRequest(req *data.SIPRequest) (*data.SIPResponse, error) {
	switch req.Method {
	case "REGISTER":
		return s.handleRegister(req)
	case "INVITE":
		return s.handleInvite(req)
	case "ACK":
		return s.handleAck(req)
	case "MESSAGE":
		return s.handleMessage(req)
	case "":
		return nil, nil // we are not reacting to empty requests
	default:
		return data.NewSIPResponseFromRequest(req, http.StatusMethodNotAllowed, "Method Not Allowed"), nil
	}
}

func (s *Server) handleRegister(req *data.SIPRequest) (*data.SIPResponse, error) {
	if client := data.NewSIPClientFromRegister(req); client != nil {
		s.RegisterCache.Set(client.Key(), client, registerExpiration)
		if s.Config.Debug {
			fmt.Printf("SIP/REGISTER: received REGISTER message from %s\n", client.Key())
		}
	}

	resp := data.NewSIPResponseFromRequest(req, http.StatusOK, "OK")
	resp.AddHeader("Expires", strconv.Itoa(int(registerExpiration.Seconds())))
	return resp, nil
}

func (s *Server) handleAck(_ *data.SIPRequest) (*data.SIPResponse, error) {
	return nil, nil
}

func (s *Server) handleInvite(req *data.SIPRequest) (*data.SIPResponse, error) {
	if s.Config.Debug {
		fmt.Printf("SIP/INVITE: received INVITE message from %s to %s\n", req.From(), req.To())
	}

	// Check if this is a call directed at a local identity (hostname or IP). If not, ignore it.
	// This also helps reducing retry storms for some clients (e.g. Linphone).
	if s.LocalIdentities != nil {
		if local, ok := s.LocalIdentities[strings.ToLower(req.To().URI.Host)]; !ok || !local {
			if s.Config.Debug {
				fmt.Printf("  - Ignoring call to non-local server: %s\n", req.To())
			}
			return data.NewSIPResponseFromRequest(req, http.StatusNotFound, "Not Found"), nil
		}
	}

	var redirect *data.SIPAddress
	to := req.To()
	// Look up the phone number and try to find the right host in our records and redirect the call there.
	s.Records.Mu.RLock()
	for _, entry := range s.Records.Entries {
		if entry.PhoneNumber != to.URI.User {
			continue
		}

		host := entry.PhoneFQDN()
		if s.Config.Resolve && entry.OLSR != nil {
			host = entry.OLSR.IP
		}

		// We found an entry in the phonebook to redirect to.
		redirect = &data.SIPAddress{
			DisplayName: entry.Callsign,
			URI: &data.SIPURI{
				User: entry.PhoneNumber,
				Host: host,
			},
			Params: make(map[string]string),
		}
		break
	}
	s.Records.Mu.RUnlock()

	// If we can't find it in the phonebook, we try locally registered clients.
	if redirect == nil {
		reg, ok := s.RegisterCache.Get(to.URI.User)
		if ok {
			redirect = reg.Address.Clone()
			redirect.URI.Params = make(map[string]string)
			redirect.Params = make(map[string]string)
		}
	}

	if redirect == nil {
		if s.Config.Debug {
			fmt.Printf("  - Couldn't find redirect destination for %s\n", req.To())
		}
		// As a last resort, we're giving up and tell the client that we can't route that call.
		return data.NewSIPResponseFromRequest(req, http.StatusNotFound, "Not Found"), nil
	}

	resp := data.NewSIPResponseFromRequest(req, http.StatusFound, "Moved Temporarily")
	resp.AddHeader("Contact", redirect.String())
	redirect.Params["reason"] = "unconditional"
	resp.AddHeader("Diversion", redirect.String())
	return resp, nil
}

func (s *Server) handleMessage(req *data.SIPRequest) (*data.SIPResponse, error) {
	if s.Config.Debug {
		fmt.Printf("SIP/MESSAGE: received MESSAGE message from %s to %s\n", req.From(), req.To())
	}
	return s.SendSIPMessage(req)
}
