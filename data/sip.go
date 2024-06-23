package data

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

type SIPRequest struct {
	Method     string
	URI        string
	SIPVersion string // Set to 2.0 version by default
	Headers    []*SIPHeader

	// Not implemented at least for now.
	// Body []byte
}

func (r *SIPRequest) From() *SIPAddress {
	for _, hdr := range r.Headers {
		if strings.ToLower(hdr.Name) != "from" {
			continue
		}
		return hdr.Address
	}
	return nil
}

func (r *SIPRequest) To() *SIPAddress {
	for _, hdr := range r.Headers {
		if strings.ToLower(hdr.Name) != "to" {
			continue
		}
		return hdr.Address
	}
	return nil
}

func (r *SIPRequest) Parse(data []byte) error {
	var i int
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := scanner.Text()
		line = strings.TrimSpace(line)
		i += 1

		// Check if we reached the end of the header.
		// Parsing the body is not implemented yet.
		if line == "" {
			return nil
		}

		// This should be the first line of the received request.
		if i == 1 {
			if err := r.parseSIPRequestStart(line); err != nil {
				return fmt.Errorf("error parsing request header: %s", err)
			}
		} else {
			if err := r.parseSIPHeader(line); err != nil {
				return fmt.Errorf("error parsing request header: %s", err)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error parsing data: %s", err)
	}
	return nil
}

func (r *SIPRequest) parseSIPRequestStart(line string) error {
	parts := strings.Split(line, " ")
	if len(parts) != 3 {
		return fmt.Errorf("SIP request start line should have 3 parts: %s", line)
	}

	r.Method = strings.ToUpper(parts[0])
	r.URI = parts[1]
	r.SIPVersion = parts[2]

	return nil
}

func (r *SIPRequest) parseSIPHeader(line string) error {
	hdr := &SIPHeader{}
	if err := hdr.parse(line); err != nil {
		return err
	}
	r.Headers = append(r.Headers, hdr)
	return nil
}

func NewSIPResponseFromRequest(req *SIPRequest, statusCode int, statusMsg string) *SIPResponse {
	resp := &SIPResponse{
		SIPVersion:    req.SIPVersion,
		StatusCode:    statusCode,
		StatusMessage: statusMsg,
	}

	copyHeader("Record-Route", req, resp)
	copyHeader("Via", req, resp)
	copyHeader("From", req, resp)
	copyHeader("To", req, resp)
	copyHeader("Call-ID", req, resp)
	copyHeader("CSeq", req, resp)
	if statusCode == 100 {
		copyHeader("Timestamp", req, resp)
	}

	return resp
}

func copyHeader(name string, req *SIPRequest, resp *SIPResponse) {
	name = strings.ToLower(name)
	for _, h := range req.Headers {
		if name != strings.ToLower(h.Name) {
			continue
		}
		hdr := h.Clone()
		resp.Headers = append(resp.Headers, &hdr)
		break
	}
}

type SIPResponse struct {
	SIPVersion    string // Set to 2.0 version by default
	StatusCode    int
	StatusMessage string
	Headers       []*SIPHeader

	Body []byte
}

func (r *SIPResponse) Serialize() []byte {
	buf := bytes.Buffer{}

	// Status line
	buf.WriteString(r.SIPVersion)
	buf.WriteString(" ")
	buf.WriteString(strconv.Itoa(r.StatusCode))
	buf.WriteString(" ")
	buf.WriteString(r.StatusMessage)
	buf.WriteString("\r\n")

	// Headers
	for _, hdr := range r.Headers {
		buf.WriteString(hdr.serialize())
		buf.WriteString("\r\n")
	}
	buf.WriteString("Content-Length: 0\r\n")

	// Empty line
	buf.WriteString("\r\n")

	// Body
	if r.Body != nil {
		buf.Write(r.Body)
	}

	return buf.Bytes()
}

type SIPHeader struct {
	Name  string
	Value string

	Address *SIPAddress // Optionally set when header has an address.
}

func (h *SIPHeader) Clone() SIPHeader {
	return SIPHeader{
		Name:  h.Name,
		Value: h.Value,
	}
}

func (h *SIPHeader) parse(line string) error {
	idx := strings.Index(line, ":")
	if idx == -1 {
		return fmt.Errorf("field name with no value in header: %s", line)
	}

	h.Name = strings.TrimSpace(line[:idx])
	h.Value = strings.TrimSpace(line[idx+1:])

	switch strings.ToLower(h.Name) {
	case "to", "from", "contact":
		addr := &SIPAddress{}
		if err := addr.Parse(h.Value); err == nil {
			h.Address = addr
		}
	}

	return nil
}

func (h *SIPHeader) serialize() string {
	return fmt.Sprintf("%s: %s", h.Name, h.Value)
}

type SIPAddress struct {
	DisplayName string
	URI         *SIPURI
	Params      map[string]string
}

func (a *SIPAddress) String() string {
	return a.URI.String()
}

func (a *SIPAddress) Parse(line string) error {
	l := strings.TrimSpace(line)
	if l == "" {
		return errors.New("empty address")
	}

	var uri string
	a.DisplayName, uri = findDisplayName(l)
	a.URI = parseSIPURI(uri)

	return nil
}

// https://datatracker.ietf.org/doc/html/rfc3261#section-19.1.1
// sip:user:password@host:port;uri-parameters?headers
func parseSIPURI(l string) *SIPURI {
	l = strings.TrimSpace(l)
	l = strings.TrimPrefix(l, "sip:")
	l = strings.Split(l, ";")[0] // ignore parameters for now

	uri := &SIPURI{}
	parts := strings.Split(l, "@")
	if len(parts) > 1 {
		user := strings.Split(parts[0], ":")
		uri.User = user[0] // ignoring the possibility that there may be a password

		host := strings.Split(parts[1], ":")
		uri.Host = host[0]
		if len(host) > 1 {
			p, _ := strconv.Atoi(host[1])
			uri.Port = p
		}
	} else {
		host := strings.Split(parts[0], ":")
		uri.Host = host[0]
		if len(host) > 1 {
			p, _ := strconv.Atoi(host[1])
			uri.Port = p
		}
	}

	return uri
}

func findDisplayName(l string) (string, string) {
	startQuote := -1
	endQuote := -1
	for i, c := range l {
		if c == '"' {
			if startQuote < 0 {
				startQuote = i
			} else {
				endQuote = i
			}
			continue
		}

		// https://datatracker.ietf.org/doc/html/rfc3261#section-20.10
		// When the header field value contains a display name, the URI
		// including all URI parameters is enclosed in "<" and ">".  If no "<"
		// and ">" are present, all parameters after the URI are header
		// parameters, not URI parameters.
		if c == '<' {
			dn := strings.TrimSpace(l[:i])
			uri := l[i+1:]
			if endQuote > 0 {
				dn = l[startQuote+1 : endQuote]
			}

			for i, c := range uri {
				if c == '>' {
					uri = uri[:i]
					break
				}
			}

			return dn, uri
		}

		if c == ';' {
			if startQuote > 0 {
				continue
			}
			// detect early
			// uri can be without <> in that case there all after ; are header params
			return "", findURI(l)
		}
	}
	return "", findURI(l)
}

func findURI(l string) string {
	for i, c := range l {
		if c == ';' {
			return l[:i]
		}
	}
	return l
}

func (a *SIPAddress) Serialize() []byte {
	return nil
}

type SIPURI struct {
	User   string // The user part of the URI. The 'joe' in sip:joe@example.com
	Host   string // The host part of the URI. This can be a domain, or a string representation of an IP address.
	Port   int    // The port part of the URI. This is optional, and can be empty.
	Params map[string]string
}

func (u *SIPURI) String() string {
	host := u.Host
	if u.Port > 0 {
		host = fmt.Sprintf("%s:%d", host, u.Port)
	}
	if u.User == "" {
		return fmt.Sprintf("<sip:%s>", host)
	}
	return fmt.Sprintf("<sip:%s@%s>", u.User, host)
}
