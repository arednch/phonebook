package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/golang/glog"

	"github.com/finack/phonebook/exporter"
	"github.com/finack/phonebook/importer"
)

var (
	source  = flag.String("source", "", "Path or URL to fetch the phonebook CSV from.")
	path    = flag.String("path", "/www", "Folder to write the phonebooks to locally.")
	formats = flag.String("formats", "", "Comma separated list of formats to export. Supported: yealink,cisco")
)

func main() {
	// Parse flags globally.
	flag.Parse()

	if *source == "" {
		glog.Exit("-source flag needs to be set")
	}
	if *formats == "" {
		glog.Exit("-formats flag needs to be set")
	}

	exporters := map[string]exporter.Exporter{}
	exportFormats := strings.Split(*formats, ",")
	for _, exp := range exportFormats {
		exp = strings.TrimSpace(exp)
		exp = strings.ToLower(exp)
		switch exp {
		case "cisco":
			exporters["cisco"] = &exporter.Cisco{}
		case "yealink":
			exporters["yealink"] = &exporter.Yealink{}
		default:
			glog.Exitf("unknown exporter %q", exp)
		}
	}

	records, err := importer.ReadPhonebook(*source)
	if err != nil {
		glog.Exit(err)
	}

	for n, exp := range exporters {
		// Direct calling phonebook.
		body, err := exp.Export(records, false)
		if err != nil {
			glog.Exit(err)
		}
		outpath := filepath.Join(*path, fmt.Sprintf("phonebook_%s_direct.xml", n))
		os.WriteFile(outpath, body, 0644)

		// PBX calling phonebook.
		body, err = exp.Export(records, true)
		if err != nil {
			glog.Exit(err)
		}
		outpath = filepath.Join(*path, fmt.Sprintf("phonebook_%s_pbx.xml", n))
		os.WriteFile(outpath, body, 0644)
	}
}
