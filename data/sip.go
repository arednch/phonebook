package data

import (
	"bufio"
	"bytes"
	"fmt"
	"strconv"
	"strings"
)

type SIPMessage interface {
	Parse([]byte) error
	Serialize() ([]byte, error)
}

type SIPRequest struct {
	Method     string
	URI        string
	SIPVersion string // Set to 2.0 version by default
	Headers    []*SIPHeader

	// Not implemented at least for now.
	// Body []byte
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

	for _, hdr := range req.Headers {
		switch hdr.Name {
		case "Record-Route":
			fallthrough
		case "Via":
			fallthrough
		case "From":
			fallthrough
		case "To":
			fallthrough
		case "Call-ID":
			fallthrough
		case "CSeq":
			h := hdr.Clone()
			resp.Headers = append(resp.Headers, &h)
		case "Timestamp":
			if statusCode == 100 {
				h := hdr.Clone()
				resp.Headers = append(resp.Headers, &h)
			}
		default:
			continue
		}
	}

	return resp
}

type SIPResponse struct {
	SIPVersion    string // Set to 2.0 version by default
	StatusCode    int
	StatusMessage string
	Headers       []*SIPHeader

	// Not implemented at least for now.
	// Body []byte
}

func (r *SIPResponse) Serialize() string {
	buf := strings.Builder{}

	// Status line
	buf.WriteString(r.SIPVersion)
	buf.WriteString(" ")
	buf.WriteString(strconv.Itoa(r.StatusCode))
	buf.WriteString(" ")
	buf.WriteString(r.StatusMessage)
	buf.WriteString("\n")

	// Headers
	for _, hdr := range r.Headers {
		buf.WriteString(hdr.serialize())
		buf.WriteString("\n")
	}

	buf.WriteString("\n")

	// Body
	// Not implemented yet.

	return buf.String()
}

type SIPHeader struct {
	Name  string
	Value string
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

	return nil
}

func (h *SIPHeader) serialize() string {
	return fmt.Sprintf("%s: %s", h.Name, h.Value)
}
