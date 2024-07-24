package server

import (
	"crypto/sha256"
	"crypto/subtle"
	"html/template"
	"net/http"
	"time"

	"github.com/arednch/phonebook/configuration"
	"github.com/arednch/phonebook/data"
	"github.com/arednch/phonebook/exporter"
)

const (
	// When true, checks whether the sender exists before sending the message out.
	checkExistenceBeforeSending = false
)

type ReloadFunc func(cfg *configuration.Config, client *http.Client) (string, error)
type SendSIPMessage func(*data.SIPRequest) (*data.SIPResponse, error)

func NewServer(
	cfg *configuration.Config, cfgPath string, version *data.Version, records *data.Records, runtimeInfo *data.RuntimeInfo,
	exporters map[string]exporter.Exporter, updates *data.Updates, refreshRecords ReloadFunc, sendSIPMessage SendSIPMessage,
	registerCache *data.TTLCache[string, *data.SIPClient], tmpls *template.Template, client *http.Client) *Server {
	return &Server{
		Version:        version,
		Config:         cfg,
		ConfigPath:     cfgPath,
		Records:        records,
		RuntimeInfo:    runtimeInfo,
		Updates:        updates,
		Exporters:      exporters,
		RegisterCache:  registerCache,
		ReloadFn:       refreshRecords,
		SendSIPMessage: sendSIPMessage,
		Tmpls:          tmpls,
		Client:         client,
	}
}

type Server struct {
	Version    *data.Version
	Config     *configuration.Config
	ConfigPath string // optional when using config file
	Client     *http.Client

	RuntimeInfo   *data.RuntimeInfo
	Records       *data.Records
	Updates       *data.Updates
	Exporters     map[string]exporter.Exporter
	RegisterCache *data.TTLCache[string, *data.SIPClient]

	ReloadFn       ReloadFunc
	SendSIPMessage SendSIPMessage

	Tmpls *template.Template
}

func (s *Server) BasicAuth(next http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := r.BasicAuth()
		if ok {
			usernameHash := sha256.Sum256([]byte(username))
			passwordHash := sha256.Sum256([]byte(password))
			expectedUsernameHash := sha256.Sum256([]byte(s.Config.WebUser))
			expectedPasswordHash := sha256.Sum256([]byte(s.Config.WebPwd))

			usernameMatch := (subtle.ConstantTimeCompare(usernameHash[:], expectedUsernameHash[:]) == 1)
			passwordMatch := (subtle.ConstantTimeCompare(passwordHash[:], expectedPasswordHash[:]) == 1)

			if usernameMatch && passwordMatch {
				next.ServeHTTP(w, r)
				return
			}
		}

		w.Header().Set("WWW-Authenticate", `Basic realm="restricted", charset="UTF-8"`)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
	})
}

func (s *Server) prepareDefaultData(title string, includeUpdates bool) *data.WebDefault {
	updated := "-"
	if s.Records.Updated.Unix() != 0 {
		updated = s.Records.Updated.Format(time.RFC3339)
	}

	var updates []*data.Update
	if includeUpdates {
		s.Updates.Mu.RLock()
		defer s.Updates.Mu.RUnlock()
		updates = s.Updates.Updates
	}
	return &data.WebDefault{
		Title:   title,
		Version: s.Version,
		Updated: updated,
		Updates: updates,
	}
}
