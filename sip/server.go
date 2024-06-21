package sip

import (
	"fmt"

	"github.com/emiago/sipgo"
	"github.com/emiago/sipgo/sip"

	"github.com/arednch/phonebook/configuration"
)

type Server struct {
	Config *configuration.Config

	UA  *sipgo.UserAgent
	Srv *sipgo.Server
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
		fmt.Printf("SIP/Invite: received INVITE message from %s\n", req.Source())
	}
	if err := tx.Respond(sip.NewResponseFromRequest(req, sip.StatusOK, "OK", nil)); err != nil {
		fmt.Printf("SIP/Invite: error sending response: %s\n", err)
	}
}

func (s *Server) OnBye(req *sip.Request, tx sip.ServerTransaction) {
	if err := tx.Respond(sip.NewResponseFromRequest(req, sip.StatusOK, "OK", nil)); err != nil {
		fmt.Printf("SIP/Bye: error sending response: %s\n", err)
	}
}

func (s *Server) OnAck(req *sip.Request, tx sip.ServerTransaction) {
	if err := tx.Respond(sip.NewResponseFromRequest(req, sip.StatusOK, "OK", nil)); err != nil {
		fmt.Printf("SIP/Ack: error sending response: %s\n", err)
	}
}
