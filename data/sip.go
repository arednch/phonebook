package data

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"strconv"
	"strings"
)

const (
	DefaultSIPVersion  = "SIP/2.0"
	DefaultMaxForwards = "30"

	SIPNewline = "\r\n"
)

func GenerateCallID(host string) string {
	return fmt.Sprintf("%d@%s", rand.Int(), host)
}

func GenerateFromTag() string {
	return strconv.Itoa(rand.Int())
}

type SIPMessage struct {
	SIPVersion string // Set to 2.0 version by default
	Headers    []*SIPHeader
	Body       []byte
}

func (m *SIPMessage) From() *SIPAddress {
	for _, hdr := range m.Headers {
		if strings.ToLower(hdr.Name) != "from" {
			continue
		}
		return hdr.Address
	}
	return nil
}

func (m *SIPMessage) To() *SIPAddress {
	for _, hdr := range m.Headers {
		if strings.ToLower(hdr.Name) != "to" {
			continue
		}
		return hdr.Address
	}
	return nil
}

func (m *SIPMessage) Contact() *SIPAddress {
	for _, hdr := range m.Headers {
		if strings.ToLower(hdr.Name) != "contact" {
			continue
		}
		return hdr.Address
	}
	return nil
}

func (m *SIPMessage) AddHeader(name, value string) {
	m.Headers = append(m.Headers, &SIPHeader{
		Name:  name,
		Value: value,
	})
}

func (m *SIPMessage) FindHeaders(name string) []*SIPHeader {
	var hdrs []*SIPHeader
	name = strings.ToLower(name)
	for _, hdr := range m.Headers {
		if strings.ToLower(hdr.Name) == name {
			hdrs = append(hdrs, hdr)
		}
	}
	return hdrs
}

func (m *SIPMessage) RemoveHeaders(name string) {
	var hdrs []*SIPHeader
	name = strings.ToLower(name)
	for _, hdr := range m.Headers {
		if strings.ToLower(hdr.Name) == name {
			continue
		}
		hdrs = append(hdrs, hdr)
	}
	m.Headers = hdrs
}

func (m *SIPMessage) ContentLength(update bool) (int, error) {
	len := len(m.Body)
	for _, hdr := range m.FindHeaders("Content-Length") {
		l, err := strconv.Atoi(hdr.Value)
		if err != nil {
			continue
		}
		if update && len != l {
			hdr.Value = strconv.Itoa(len)
		}
		return l, nil
	}
	return 0, errors.New("no content length found")
}

func NewSIPRequest(method string, from, to *SIPAddress, seq int, hdrs []*SIPHeader, body []byte) *SIPRequest {
	resp := &SIPRequest{
		SIPMessage: SIPMessage{
			SIPVersion: DefaultSIPVersion,
			Headers: []*SIPHeader{
				{
					Name:  "Via",
					Value: fmt.Sprintf("%s/UDP %s", DefaultSIPVersion, from.URI.Host),
				}, {
					Name:    "From",
					Value:   from.String(),
					Address: from,
				}, {
					Name:    "To",
					Value:   to.String(),
					Address: to,
				}, {
					Name:  "Call-ID",
					Value: GenerateCallID(from.URI.Host),
				}, {
					Name:  "CSeq",
					Value: fmt.Sprintf("%d %s", seq, method),
				}, {
					Name:  "Max-Forwards",
					Value: DefaultMaxForwards,
				},
			},
			Body: body,
		},
		Method: method,
		URI:    to.URI.String(),
	}
	resp.Headers = append(resp.Headers, hdrs...)
	return resp
}

type SIPRequest struct {
	SIPMessage
	Method string
	URI    string
}

func (r *SIPRequest) Parse(data []byte) error {
	var i int
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := scanner.Text()
		line = strings.TrimSpace(line)
		i += 1

		// Check if we reached the end of the header.
		if line == "" {
			break
		}

		// This should be the first line of the received request.
		if i == 1 {
			if err := r.parseSIPRequestStart(line); err != nil {
				return fmt.Errorf("error parsing request start: %s", err)
			}
		} else {
			if err := r.parseSIPHeader(line); err != nil {
				return fmt.Errorf("error parsing request header: %s", err)
			}
		}
	}
	// Check if we have a body and if not, return already
	if len, _ := r.ContentLength(false); len == 0 {
		if err := scanner.Err(); err != nil {
			return fmt.Errorf("error parsing headers: %s", err)
		}
		return nil
	}

	for scanner.Scan() {
		r.Body = append(r.Body, scanner.Bytes()...)
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error parsing body: %s", err)
	}
	return nil
}

func (r *SIPRequest) Write(w io.Writer, dbg bool) (int, error) {
	out := r.Serialize(true)
	n, err := w.Write(out)
	if err != nil {
		return n, fmt.Errorf("unable to write: %s", err)
	}
	if dbg {
		fmt.Printf("SIP/Request/Write headers (%d bytes):\n%+v\n", n, string(out))
	}
	return n, nil
}

func (r *SIPRequest) Serialize(withBody bool) []byte {
	buf := bytes.Buffer{}

	// Status line
	buf.WriteString(r.Method)
	buf.WriteString(" ")
	buf.WriteString(r.To().URI.String())
	buf.WriteString(" ")
	buf.WriteString(r.SIPVersion)
	buf.WriteString(SIPNewline)

	// Headers
	for _, hdr := range r.Headers {
		buf.WriteString(hdr.serialize())
		buf.WriteString(SIPNewline)
	}
	buf.WriteString("Content-Length: ")
	buf.WriteString(strconv.Itoa(len(r.Body)))
	buf.WriteString(SIPNewline)

	if !withBody {
		return buf.Bytes()
	}

	// Empty line
	buf.WriteString(SIPNewline)

	// Body
	if r.Body != nil {
		buf.Write(r.Body)
	}

	return buf.Bytes()
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
		SIPMessage: SIPMessage{
			SIPVersion: req.SIPVersion,
		},
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
	SIPMessage
	StatusCode    int
	StatusMessage string
}

func (r *SIPResponse) Parse(data []byte) error {
	var i int
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := scanner.Text()
		line = strings.TrimSpace(line)
		i += 1

		// Check if we reached the end of the header.
		if line == "" {
			break
		}

		// This should be the first line of the received response.
		if i == 1 {
			if err := r.parseSIPResponseStatus(line); err != nil {
				return fmt.Errorf("error parsing response status: %s", err)
			}
		} else {
			if err := r.parseSIPHeader(line); err != nil {
				return fmt.Errorf("error parsing request header: %s", err)
			}
		}
	}
	// Check if we have a body and if not, return already
	if len, _ := r.ContentLength(false); len == 0 {
		if err := scanner.Err(); err != nil {
			return fmt.Errorf("error parsing headers: %s", err)
		}
		return nil
	}

	for scanner.Scan() {
		r.Body = append(r.Body, scanner.Bytes()...)
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error parsing body: %s", err)
	}
	return nil
}

func (r *SIPResponse) parseSIPResponseStatus(line string) error {
	parts := strings.Split(line, " ")
	if len(parts) != 3 {
		return fmt.Errorf("SIP request start line should have 3 parts: %s", line)
	}

	sc, err := strconv.Atoi(parts[1])
	if err != nil {
		return fmt.Errorf("unable to convert response code: %s", err)
	}

	r.SIPVersion = strings.ToUpper(parts[0])
	r.StatusCode = sc
	r.StatusMessage = parts[2]

	return nil
}

func (r *SIPResponse) parseSIPHeader(line string) error {
	hdr := &SIPHeader{}
	if err := hdr.parse(line); err != nil {
		return err
	}
	r.Headers = append(r.Headers, hdr)
	return nil
}

func (r *SIPResponse) Write(w io.Writer, dbg bool) (int, error) {
	out := r.Serialize(true)
	n, err := w.Write(out)
	if err != nil {
		return n, fmt.Errorf("unable to write: %s", err)
	}
	if dbg {
		fmt.Printf("SIP/Response/Write headers (%d bytes):\n%+v\n", n, string(out))
	}
	return n, nil
}

func (r *SIPResponse) Serialize(withBody bool) []byte {
	buf := bytes.Buffer{}

	// Status line
	buf.WriteString(r.SIPVersion)
	buf.WriteString(" ")
	buf.WriteString(strconv.Itoa(r.StatusCode))
	buf.WriteString(" ")
	buf.WriteString(r.StatusMessage)
	buf.WriteString(SIPNewline)

	// Headers
	for _, hdr := range r.Headers {
		buf.WriteString(hdr.serialize())
		buf.WriteString(SIPNewline)
	}
	buf.WriteString("Content-Length: ")
	buf.WriteString(strconv.Itoa(len(r.Body)))
	buf.WriteString(SIPNewline)

	if !withBody {
		return buf.Bytes()
	}

	// Empty line
	buf.WriteString(SIPNewline)

	// Body
	if r.Body != nil {
		buf.Write(r.Body)
	}

	return buf.Bytes()
}

type SIPHeader struct {
	Name  string
	Value string

	// Optionally set when header has an address.
	// Note: This is parsed but not used during serialization!
	Address *SIPAddress
}

func (h *SIPHeader) Clone() SIPHeader {
	var addr *SIPAddress
	if h.Address != nil {
		addr = h.Address.Clone()
	}
	return SIPHeader{
		Name:    h.Name,
		Value:   h.Value,
		Address: addr,
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
		addr := &SIPAddress{
			Params: make(map[string]string),
		}
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

func (a *SIPAddress) Clone() *SIPAddress {
	p := make(map[string]string)
	for k, v := range a.Params {
		p[k] = v
	}
	var uri *SIPURI
	if a.URI != nil {
		uri = a.URI.Clone()
	}
	return &SIPAddress{
		DisplayName: a.DisplayName,
		Params:      p,
		URI:         uri,
	}
}

func (a *SIPAddress) String() string {
	uri := fmt.Sprintf("<%s>", a.URI.String())
	if a.DisplayName != "" {
		uri = fmt.Sprintf("\"%s\" <%s>", a.DisplayName, a.URI.String())
	}
	if len(a.Params) > 0 {
		return uri + ";" + paramsToString(a.Params)
	}
	return uri
}

func (a *SIPAddress) Parse(line string) error {
	l := strings.TrimSpace(line)
	if l == "" {
		return errors.New("empty address")
	}

	var uri string
	a.DisplayName, uri = findDisplayName(l)
	a.URI = parseSIPURI(uri)

	var parts []string
	if strings.Contains(l, ">") {
		parts = strings.Split(l, ">;")
	} else {
		parts = strings.Split(l, ";")
	}
	if len(parts) > 1 {
		a.Params = parseParameters(parts[1])
	}

	return nil
}

// https://datatracker.ietf.org/doc/html/rfc3261#section-19.1.1
// sip:user:password@host:port;uri-parameters?headers
func parseSIPURI(l string) *SIPURI {
	l = strings.TrimSpace(l)
	l = strings.TrimPrefix(l, "sip:")
	l = strings.TrimPrefix(l, "sips:")
	lp := strings.Split(l, ";")

	uri := &SIPURI{}
	parts := strings.Split(lp[0], "@")
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
	if len(lp) > 1 {
		uri.Params = parseParameters(lp[1])
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

func parseParameters(l string) map[string]string {
	var start int
	var k string
	params := make(map[string]string)
	for i, c := range l {
		switch c {
		case '=':
			k = l[start:i]
			start = i + 1
		case ';':
			if k == "" {
				params[l[start:i]] = ""
			} else {
				params[k] = l[start:i]
			}
			k = ""
			start = i + 1
		}
	}
	if k != "" {
		params[k] = l[start:]
	}
	return params
}

func paramsToString(params map[string]string) string {
	var p []string
	for k, v := range params {
		// we should probably also encode this / check for character set
		// for now we rely on users to set the right one
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if v == "" {
			p = append(p, k)
		} else {
			p = append(p, fmt.Sprintf("%s=%s", k, v))
		}
	}
	if len(p) == 0 {
		return ""
	}
	return strings.Join(p, ";")
}

type SIPURI struct {
	User   string // The user part of the URI. The 'joe' in sip:joe@example.com
	Host   string // The host part of the URI. This can be a domain, or a string representation of an IP address.
	Port   int    // The port part of the URI. This is optional, and can be empty.
	Params map[string]string
}

func (u *SIPURI) Clone() *SIPURI {
	p := make(map[string]string)
	for k, v := range u.Params {
		p[k] = v
	}
	return &SIPURI{
		User:   u.User,
		Host:   u.Host,
		Port:   u.Port,
		Params: p,
	}
}

func (u *SIPURI) String() string {
	host := u.Host
	if u.Port > 0 {
		host = fmt.Sprintf("%s:%d", host, u.Port)
	}
	user := fmt.Sprintf("sip:%s@%s", u.User, host)
	if u.User == "" {
		user = fmt.Sprintf("sip:%s", host)
	}
	if len(u.Params) > 1 {
		return user + ";" + paramsToString(u.Params)
	}
	return user
}

type SIPClient struct {
	Address *SIPAddress
	UA      string
}

func (c *SIPClient) Key() string {
	return c.Address.URI.User
}

func NewSIPClientFromRegister(req *SIPRequest) *SIPClient {
	if req.Method != "REGISTER" {
		return nil
	}

	addr := req.Contact()
	addr.URI.Params = make(map[string]string)
	addr.Params = make(map[string]string)
	client := &SIPClient{
		Address: addr,
	}

	for _, hdr := range req.FindHeaders("User-Agent") {
		client.UA = hdr.Value
		break
	}

	return client
}
