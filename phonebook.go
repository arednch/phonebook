package main

import (
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/golang/glog"

	"github.com/finack/phonebook/data"
	"github.com/finack/phonebook/exporter"
	"github.com/finack/phonebook/importer"
)

var (
	source  = flag.String("source", "", "Path or URL to fetch the phonebook CSV from.")
	path    = flag.String("path", "", "Folder to write the phonebooks to locally.")
	formats = flag.String("formats", "", "Comma separated list of formats to export. Supported: generic,yealink,cisco,snom")
	server  = flag.Bool("server", true, "Phonebook acts as a server when set to true.")
	port    = flag.Int("port", 8080, "Port to listen on (when running as a server).")
	reload  = flag.Duration("reload", time.Hour, "Duration after which to try to reload the phonebook source.")
)

var (
	recordsMu *sync.RWMutex
	records   []*data.Entry

	exporters map[string]exporter.Exporter
)

func refreshRecords(source string) error {
	rec, err := importer.ReadPhonebook(source)
	if err != nil {
		return err
	}

	recordsMu.Lock()
	defer recordsMu.Unlock()
	records = rec

	return nil
}

func servePhonebook(w http.ResponseWriter, r *http.Request) {
	format := r.FormValue("format")
	if format == "" {
		http.Error(w, "'format' must be specified.", http.StatusBadRequest)
		return
	}
	p := r.FormValue("pbx")
	if p == "" {
		http.Error(w, "'pbx' must be specified (true/false).", http.StatusBadRequest)
		return
	}
	pbx := p == "true" || p == "pbx"

	outFmt := strings.ToLower(strings.TrimSpace(format))
	exp, ok := exporters[outFmt]
	if !ok {
		http.Error(w, "Unknown format.", http.StatusBadRequest)
		return
	}

	body, err := exp.Export(records, pbx)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	io.WriteString(w, string(body))
}

func exportOnce(source string, path string, formats []string) error {
	for _, outFmt := range formats {
		outFmt := strings.ToLower(strings.TrimSpace(outFmt))
		exp, ok := exporters[outFmt]
		if !ok {
			glog.Exitf("unknown exporter %q", outFmt)
		}

		// Direct calling phonebook.
		body, err := exp.Export(records, false)
		if err != nil {
			return err
		}
		outpath := filepath.Join(path, fmt.Sprintf("phonebook_%s_direct.xml", outFmt))
		os.WriteFile(outpath, body, 0644)

		// PBX calling phonebook.
		body, err = exp.Export(records, true)
		if err != nil {
			return err
		}
		outpath = filepath.Join(path, fmt.Sprintf("phonebook_%s_pbx.xml", outFmt))
		os.WriteFile(outpath, body, 0644)
	}

	return nil
}

func main() {
	// Parse flags globally.
	flag.Parse()
	recordsMu = &sync.RWMutex{}
	exporters = map[string]exporter.Exporter{
		"generic": &exporter.Generic{},
		"cisco":   &exporter.Cisco{},
		"yealink": &exporter.Yealink{},
		"snom":    &exporter.Snom{},
	}

	if *source == "" {
		glog.Exit("-source flag needs to be set")
	}

	if !*server {
		if *path == "" {
			glog.Exit("-path flag needs to be set")
		}
		if *formats == "" {
			glog.Exit("-formats flag needs to be set")
		}

		if err := refreshRecords(*source); err != nil {
			glog.Exit(err)
		}
		exportFormats := strings.Split(*formats, ",")
		if err := exportOnce(*source, *path, exportFormats); err != nil {
			glog.Exit(err)
		}
	}

	go func() {
		for {
			if err := refreshRecords(*source); err != nil {
				glog.Warningf("error refreshing data from upstream: %s", err)
			}
			time.Sleep(*reload)
		}
	}()

	http.HandleFunc("/phonebook", servePhonebook)
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		glog.Exit(err)
	}
	http.Serve(listener, nil)
}
