package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"html/template"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mark-rushakoff/ldapserver"

	"github.com/arednch/phonebook/configuration"
	"github.com/arednch/phonebook/data"
	"github.com/arednch/phonebook/exporter"
	"github.com/arednch/phonebook/importer"
	"github.com/arednch/phonebook/ldap"
	"github.com/arednch/phonebook/olsr"
	"github.com/arednch/phonebook/server"
	"github.com/arednch/phonebook/sip"
)

var (
	// Generally applicable flags.
	conf            = flag.String("conf", "", "Path to the JSON config file instead of parsing flags.")
	sources         = flag.String("sources", "", "Comma separated paths or URLs to fetch the phonebook CSV from.")
	olsrFile        = flag.String("olsr", "/tmp/run/hosts_olsr", "Path to the OLSR hosts file.")
	sysInfoURL      = flag.String("sysinfo", "", "URL of sysinfo JSON API. Usually: http://localnode.local.mesh/cgi-bin/sysinfo.json?hosts=1")
	daemonize       = flag.Bool("server", false, "Phonebook acts as a server when set to true.")
	ldapServer      = flag.Bool("ldap_server", false, "Phonebook also runs an LDAP server when in server mode.")
	sipServer       = flag.Bool("sip_server", false, "Phonebook also runs a SIP server when in server mode.")
	webServer       = flag.Bool("web_server", true, "Phonebook runs a webserver when in server mode.")
	debug           = flag.Bool("debug", false, "Turns on verbose logging to stdout when set to true.")
	allowRtCfgChg   = flag.Bool("allow_runtime_config_changes", false, "Allows runtime config changes via web server when set to true.")
	allowPermCfgChg = flag.Bool("allow_permanent_config_changes", false, "Allows permanent config changes via web server when set to true.")
	includeRoutable = flag.Bool("include_routable", false, "Also include routable phone numbers not in the phonebook.")
	countryPfx      = flag.String("country_prefix", "", "Three digit country prefix for phone numbers.")

	// Only relevant when running in non-server / ad-hoc mode.
	path           = flag.String("path", "", "Folder to write the phonebooks to locally.")
	formats        = flag.String("formats", "combined", "Comma separated list of formats to export. Supported: pbx,direct,combined")
	targets        = flag.String("targets", "", "Comma separated list of targets to export. Supported: generic,yealink,cisco,snom,grandstream,vcard")
	resolve        = flag.Bool("resolve", false, "Resolve hostnames to IPs when set to true using OLSR data.")
	indicateActive = flag.Bool("indicate_active", false, "Prefixes active participants in the phonebook with -active_pfx.")
	filterInactive = flag.Bool("filter_inactive", false, "Filters inactive participants to not show in the phonebook.")
	activePfx      = flag.String("active_pfx", "*", "Prefix to add when -indicate_active is set.")

	// Only relevant when running in server mode.
	port       = flag.Int("port", 8081, "Port to listen on (when running as a server).")
	cache      = flag.String("cache", "/www/phonebook.csv", "Path to a local folder to cache the downloaded CSV in.")
	reload     = flag.Duration("reload", time.Hour, "Duration after which to try to reload the phonebook source.")
	webUser    = flag.String("web_user", "", "Username to protect many of the web endpoints with (BasicAuth). Default: None")
	webPwd     = flag.String("web_pwd", "", "Password to protect many of the web endpoints with (BasicAuth). Default: None")
	ldapPort   = flag.Int("ldap_port", 3890, "Port to listen on for the LDAP server (when running as a server AND LDAP server is on as well).")
	ldapUser   = flag.String("ldap_user", "aredn", "Username to provide to connect to the LDAP server.")
	ldapPwd    = flag.String("ldap_pwd", "aredn", "Password to provide to connect to the LDAP server.")
	sipPort    = flag.Int("sip_port", 5060, "Port to listen on for SIP traffic (when running as a server AND SIP server is on as well).")
	updateURLs = flag.String("update_urls", "", "Comma separated list of URLs to pull optional information from. Used for update notifications and such.")
)

const (
	defaultExtension = ".xml"
	sysInfoReload    = 5 * time.Minute
	updateInfoReload = 24 * time.Hour
	httpTimeout      = 10 * time.Second
)

var (
	// Compile time flags (LDFLAGS)
	Version   = "dev"
	CommitSHA = "-"

	runtimeInfo *data.RuntimeInfo
	records     *data.Records
	updates     *data.Updates
	exporters   map[string]exporter.Exporter

	extensions = map[string]string{
		"vcard": ".vcf",
	}
	ignoredIdentityPfxs = []string{
		"127.0.0.",
		"fe80::",
		"::1",
	}

	//go:embed templates/*
	webFS embed.FS
)

func mergePhonebookWithRouting(records []*data.Entry, hostData map[string]*data.OLSR, cfg *configuration.Config) []*data.Entry {
	addedOLSR := make(map[string]bool)
	// First find all phonebook entries with an OLSR entry.
	for _, e := range records {
		hostname := strings.Split(e.PhoneNumber, ".")[0]
		o, ok := hostData[hostname]
		if ok {
			e.OLSR = o
			addedOLSR[hostname] = true
			continue
		}
	}
	if cfg.Debug {
		fmt.Printf("Merged phonebook with routing data. Found %d matches for %d entries in %d known hosts.\n", len(records), len(addedOLSR), len(hostData))
	}
	if !cfg.IncludeRoutable {
		return records
	}

	// Then find the OLSR entries which have no phonebook entry and create one for them if configured to do so.
	var routableEntries []*data.Entry
	for hn, o := range hostData {
		if _, ok := addedOLSR[hn]; ok {
			continue // ignore entries which are already covered by the phonebook
		}
		if _, err := strconv.Atoi(hn); err != nil {
			continue // ignore entries which do not seem to be a phonenumber
		}
		if cfg.Debug {
			fmt.Printf("  - adding routable entry %s (%s)\n", o.Hostname, o.IP)
		}
		routableEntries = append(routableEntries, data.NewEntryFromOLSR(o))
	}
	if cfg.Debug {
		fmt.Printf("Merged added another %d routable entries.\n", len(routableEntries))
	}

	return append(records, routableEntries...)
}

func refreshSysinfo(cfg *configuration.Config, client *http.Client) error {
	si, err := importer.ReadSysInfoFromURL(cfg.SysInfoURL, client)
	if err != nil {
		return fmt.Errorf("error reading sysinfo from %q: %s", cfg.SysInfoURL, err)
	}
	runtimeInfo.Mu.Lock()
	defer runtimeInfo.Mu.Unlock()
	runtimeInfo.SysInfo = si
	runtimeInfo.Updated = time.Now()
	return nil
}

func refreshUpdates(cfg *configuration.Config, client *http.Client) error {
	u, _ := importer.ReadUpdatesFromURL(cfg.UpdateURLs, client)
	if u == nil {
		return nil // no update available
	}
	updates.Mu.Lock()
	defer updates.Mu.Unlock()
	updates.Updates = u
	updates.Updated = time.Now()
	return nil
}

func refreshRecordsAndExport(cfg *configuration.Config, client *http.Client) (string, error) {
	updatedFrom, err := refreshRecords(cfg, client)
	if err != nil {
		return "", fmt.Errorf("unable to refresh records: %s", err)
	}
	if cfg.Path == "" {
		if cfg.Debug {
			fmt.Printf("not exported phonebook because path is not set")
		}
		return updatedFrom, nil
	}
	if err := exportOnce(cfg); err != nil {
		return updatedFrom, fmt.Errorf("unable to export: %s", err)
	}
	return updatedFrom, nil
}

func refreshRecords(cfg *configuration.Config, client *http.Client) (string, error) {
	var updatedFrom string
	var err error
	var rec []*data.Entry
	for _, src := range cfg.Sources {
		if cfg.Debug {
			fmt.Printf("Read phonebook from %q\n", src)
		}
		rec, err = importer.ReadPhonebook(src, cfg.Cache, client)
		if err == nil {
			updatedFrom = src
			break
		}
	}
	// File can't be loaded from the network, try to read it from cache.
	if rec == nil && cfg.Cache != "" {
		rec, err = importer.ReadPhonebook(cfg.Cache, "", client)
		if err == nil {
			if cfg.Debug {
				fmt.Printf("Read phonebook from cache: %q\n", cfg.Cache)
			}
			updatedFrom = cfg.Cache
		}
	}
	// File is not even in cache yet so we have no choice but try later.
	if rec == nil {
		return "", fmt.Errorf("error reading phonebook: %s", err)
	}

	runtimeInfo.Mu.Lock()
	defer runtimeInfo.Mu.Unlock()
	var hostData map[string]*data.OLSR
	switch {
	case cfg.OLSRFile == "" && runtimeInfo.SysInfo == nil:
		fmt.Println("not reading network information: neither OLSR file nor sysinfo available")

	case runtimeInfo.SysInfo != nil:
		hostData, err = olsr.ReadFromSysInfo(runtimeInfo.SysInfo)
		if err != nil {
			fmt.Printf("error reading OLSR data from sysinfo: %s\n", err)
		}

	case cfg.OLSRFile != "":
		if _, err := os.Stat(cfg.OLSRFile); err != nil {
			fmt.Printf("not reading network information: OLSR file %q does not exist\n", cfg.OLSRFile)
		}
		hostData, err = olsr.ReadFromFile(cfg.OLSRFile)
		if err != nil {
			fmt.Printf("error reading OLSR data from file %q: %s", cfg.OLSRFile, err)
		}
	}

	rec = mergePhonebookWithRouting(rec, hostData, cfg)

	records.Mu.Lock()
	defer records.Mu.Unlock()
	records.Entries = rec
	records.Updated = time.Now()

	return updatedFrom, nil
}

func exportOnce(cfg *configuration.Config) error {
	records.Mu.RLock()
	defer records.Mu.RUnlock()
	sort.Sort(data.ByName(records.Entries))

	for _, outTgt := range cfg.Targets {
		if cfg.Debug {
			fmt.Printf("Exporting for target %q\n", outTgt)
		}
		outTgt := strings.ToLower(strings.TrimSpace(outTgt))
		exp, ok := exporters[outTgt]
		if !ok {
			return fmt.Errorf("unknown target %q", outTgt)
		}

		ext, ok := extensions[outTgt]
		if !ok {
			ext = defaultExtension
		}

		for _, outFmt := range cfg.Formats {
			if cfg.Debug {
				fmt.Printf("Exporting for format %q\n", outFmt)
			}
			switch strings.ToLower(strings.TrimSpace(outFmt)) {
			case "d", "direct": // Direct calling phonebook.
				body, err := exp.Export(records.Entries, exporter.FormatDirect, cfg.ActivePfx, cfg.Resolve, cfg.IndicateActive, cfg.FilterInactive, cfg.Debug)
				if err != nil {
					return err
				}
				outpath := filepath.Join(cfg.Path, fmt.Sprintf("phonebook_%s_direct%s", outTgt, ext))
				os.WriteFile(outpath, body, 0644)
			case "p", "pbx": // PBX calling phonebook.
				body, err := exp.Export(records.Entries, exporter.FormatPBX, cfg.ActivePfx, cfg.Resolve, cfg.IndicateActive, cfg.FilterInactive, cfg.Debug)
				if err != nil {
					return err
				}
				outpath := filepath.Join(cfg.Path, fmt.Sprintf("phonebook_%s_pbx%s", outTgt, ext))
				os.WriteFile(outpath, body, 0644)
			case "c", "combined":
				body, err := exp.Export(records.Entries, exporter.FormatCombined, cfg.ActivePfx, cfg.Resolve, cfg.IndicateActive, cfg.FilterInactive, cfg.Debug)
				if err != nil {
					return err
				}
				outpath := filepath.Join(cfg.Path, fmt.Sprintf("phonebook_%s_combined%s", outTgt, ext))
				os.WriteFile(outpath, body, 0644)
			default:
				return fmt.Errorf("unknown format: %q", outFmt)
			}
		}
	}

	return nil
}

func ignoreIdentityPfx(id string) bool {
	for _, pfx := range ignoredIdentityPfxs {
		if strings.HasPrefix(id, pfx) {
			return true
		}
	}
	return false
}

func getLocalIdentities() (map[string]bool, error) {
	identities := map[string]bool{
		data.AREDNLocalNode: true,
	}

	if hn, err := os.Hostname(); err != nil {
		return nil, fmt.Errorf("unable to look up hostname: %s", err)
	} else {
		hn = strings.ToLower(hn)
		hn = strings.Trim(hn, ".")
		if !ignoreIdentityPfx(hn) {
			identities[hn] = true
			if !strings.HasSuffix(hn, data.AREDNDomain) {
				identities[fmt.Sprintf("%s.%s", hn, data.AREDNDomain)] = true
			}
		}
	}

	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("unable to look up interfaces: %s", err)
	}
	for _, i := range ifaces {
		addrs, err := i.Addrs()
		if err != nil {
			return nil, fmt.Errorf("unable to look up addresses for interface %s: %s", i.Name, err)
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if !ignoreIdentityPfx(ip.String()) {
				identities[ip.String()] = true
			}
		}
	}

	return identities, nil
}

func runServer(ctx context.Context, cfg *configuration.Config, cfgPath string, client *http.Client, ver *data.Version) error {
	if cfg.LDAPServer {
		ldapSrv := &ldap.Server{
			Config:  cfg,
			Records: records,
		}
		s := ldapserver.NewServer()
		s.Bind = ldapSrv.Bind
		s.Search = ldapSrv.Search

		go func() {
			if err := s.ListenAndServe(fmt.Sprintf(":%d", cfg.LDAPPort)); err != nil {
				fmt.Printf("LDAP server failed: %s\n", err)
			}
		}()
	}

	var sipSrv *sip.Server
	if cfg.SIPServer {
		identities, err := getLocalIdentities()
		if err != nil {
			identities = nil
			if cfg.Debug {
				fmt.Printf("unable to look up local identities, using empty set: %s\n", err)
			}
		} else if cfg.Debug {
			fmt.Println("using local SIP identities:")
			for k := range identities {
				fmt.Printf("  - %s\n", k)
			}
		}
		sipSrv = &sip.Server{
			Config:          cfg,
			Records:         records,
			RegisterCache:   data.NewTTL[string, *data.SIPClient](),
			LocalIdentities: identities,
		}

		go func() {
			if cfg.Debug {
				fmt.Println("Starting SIP Listener")
			}
			if err := sipSrv.ListenAndServe(ctx, "udp", fmt.Sprintf(":%d", cfg.SIPPort)); err != nil {
				fmt.Printf("SIP server failed: %s\n", err)
			}
		}()
	}

	if cfg.SysInfoURL != "" {
		go func() {
			for {
				if err := refreshSysinfo(cfg, client); err != nil {
					fmt.Printf("error refreshing sysinfo: %s\n", err)
				}
				time.Sleep(sysInfoReload)
			}
		}()
	}

	go func() {
		for {
			if updatedFrom, err := refreshRecordsAndExport(cfg, client); err == nil {
				fmt.Printf("Updated phonebook records from %q\n", updatedFrom)
			} else {
				fmt.Printf("error refreshing and exporting phone records: %s\n", err)
			}
			time.Sleep(cfg.Reload)
		}
	}()

	updates = &data.Updates{
		Mu: &sync.RWMutex{},
	}
	if len(cfg.UpdateURLs) > 0 {
		go func() {
			for {
				if err := refreshUpdates(cfg, client); err != nil {
					fmt.Printf("error refreshing updates: %s\n", err)
				}
				time.Sleep(updateInfoReload)
			}
		}()
	}

	if cfg.WebServer {
		resFS, err := fs.Sub(webFS, "resources")
		if err != nil {
			return err
		}
		tmpls := template.Must(template.ParseFS(webFS, "templates/*.html"))
		srv := server.NewServer(cfg, cfgPath, ver, records, runtimeInfo, exporters, updates, refreshRecordsAndExport, sipSrv.SendSIPMessage, sipSrv.RegisterCache, tmpls, client)
		http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(resFS))))
		http.HandleFunc("/", srv.Index)
		http.HandleFunc("/index.html", srv.Index)
		http.HandleFunc("/info", srv.Info)
		http.HandleFunc("/phonebook", srv.ServePhonebook)
		http.HandleFunc("/showconfig", srv.ShowConfig)
		http.HandleFunc("/reload", srv.ReloadPhonebook)
		if cfg.WebUser != "" && cfg.WebPwd != "" {
			if cfg.Debug {
				fmt.Println("protecting most web endpoints with configured basicAuth user/pwd")
			}
			http.HandleFunc("/message", srv.BasicAuth(srv.SendMessage))
			http.HandleFunc("/updateconfig", srv.BasicAuth(srv.UpdateConfig))
		} else {
			if cfg.Debug {
				fmt.Println("not protecting any of the web endpoints with basicAuth as not both user/pwd were set")
			}
			http.HandleFunc("/message", srv.SendMessage)
			http.HandleFunc("/updateconfig", srv.UpdateConfig)
		}
		listener, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.Port))
		if err != nil {
			return err
		}

		return http.Serve(listener, nil)
	}

	for {
		fmt.Println("phonebook running")
		time.Sleep(time.Hour)
	}
}

func runLocal(cfg *configuration.Config, client *http.Client) error {
	if err := refreshSysinfo(cfg, client); err != nil {
		return err
	}
	if updatedFrom, err := refreshRecords(cfg, client); err == nil {
		fmt.Printf("Updated phonebook records from %q\n", updatedFrom)
	} else {
		return err
	}
	if err := exportOnce(cfg); err != nil {
		return err
	}

	return nil
}

func main() {
	ctx := context.Background()
	// Parse flags globally.
	flag.Parse()
	fmt.Printf("phonebook starting %q\n", Version)

	records = &data.Records{
		Mu: &sync.RWMutex{},
	}
	runtimeInfo = &data.RuntimeInfo{
		Mu: &sync.RWMutex{},
	}
	exporters = map[string]exporter.Exporter{
		"generic":     &exporter.Generic{},
		"cisco":       &exporter.Cisco{},
		"yealink":     &exporter.Yealink{},
		"snom":        &exporter.Snom{},
		"grandstream": &exporter.Grandstream{},
		"vcard":       &exporter.VCard{},
	}

	var cfg *configuration.Config
	if *conf != "" {
		if c, err := configuration.ReadFromJSON(*conf); err != nil {
			fmt.Printf("unable to read config: %s\n", err)
			os.Exit(1)
		} else {
			c.Reload = time.Duration(c.ReloadSeconds) * time.Second
			cfg = c
		}
	} else {
		cfg = &configuration.Config{
			Sources:                     strings.Split(*sources, ","),
			OLSRFile:                    *olsrFile,
			SysInfoURL:                  *sysInfoURL,
			Server:                      *daemonize,
			LDAPServer:                  *ldapServer,
			SIPServer:                   *sipServer,
			WebServer:                   *webServer,
			Debug:                       *debug,
			AllowRuntimeConfigChanges:   *allowRtCfgChg,
			AllowPermanentConfigChanges: *allowPermCfgChg,
			UpdateURLs:                  strings.Split(*updateURLs, ","),
			Path:                        *path,
			Formats:                     strings.Split(*formats, ","),
			Targets:                     strings.Split(*targets, ","),
			Resolve:                     *resolve,
			IndicateActive:              *indicateActive,
			FilterInactive:              *filterInactive,
			ActivePfx:                   *activePfx,
			IncludeRoutable:             *includeRoutable,
			CountryPrefix:               *countryPfx,
			Port:                        *port,
			Cache:                       *cache,
			Reload:                      *reload,
			WebUser:                     *webUser,
			WebPwd:                      *webPwd,
			LDAPPort:                    *ldapPort,
			LDAPUser:                    *ldapUser,
			LDAPPwd:                     *ldapPwd,
			SIPPort:                     *sipPort,
		}
	}
	// Detect when flag is set to run as a server even when reading config.
	if *daemonize {
		cfg.Server = *daemonize
	}

	if err := cfg.IsValid(); err != nil {
		fmt.Println("config/flag validation failed:", err)
		os.Exit(1)
	}

	httpClient := &http.Client{
		Timeout: httpTimeout,
	}
	if cfg.Server {
		if *debug {
			fmt.Println("Running phonebook in server mode")
		}
		if err := runServer(ctx, cfg, *conf, httpClient, &data.Version{
			Version:   Version,
			CommitSHA: CommitSHA,
		}); err != nil {
			fmt.Printf("unable to run server: %s\n", err)
			os.Exit(1)
		}
	} else {
		if *debug {
			fmt.Println("Running phonebook as a one-time export")
		}
		if err := runLocal(cfg, httpClient); err != nil {
			fmt.Printf("unable to run: %s\n", err)
			os.Exit(1)
		}
	}
}
