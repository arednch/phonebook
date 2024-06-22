# Phonebook

Phonebook conversion from CSV to a number of output formats intended to be used for AREDN.

For release notes, see the [release page](https://github.com/arednch/packages/releases) or
[this document](https://docs.google.com/document/d/18D14Ch3GjUZmSRQALEKslvtEJ0O76pZkV3VNJ6vsB14/edit).

## Flags

Generally applicable:

- `source`: Path or URL to fetch the phonebook CSV from. Default: ""
- `olsr`: Path to the OLSR hosts file. Default: `/tmp/run/hosts_olsr`
- `sysinfo`: URL from which to fetch AREDN sysinfo. Usually: `http://localnode.local.mesh/cgi-bin/sysinfo.json?hosts=1`
- `server`: Phonebook acts as a server when set to true. Default: false
- `ldap_server`: When the phonebook is running as a server, it also exposes an LDAP v3 server when set to true. Default: false
- `sip_server`: When the phonebook is running as a server, it also runs a _very_ simple SIP server when set to true. Default: false
- `debug`: Print verbose debug messages on stdout when set to true. Default: false

Only relevant when running in **non-server / ad-hoc mode**:

- `path`: Folder to write the phonebooks to locally. Default: ""
- `formats`: Comma separated list of formats to export.

		- Supported: pbx,direct,combined
		- Default: "combined"

- `targets`: Comma separated list of targets to export.

		- Supported: generic,yealink,cisco,snom,grandstream,vcard
		- Default: ""

- `resolve`: Resolve hostnames to IPs when set to true using OLSR data. Default: `false`
- `indicate_active`: Prefixes active participants in the phonebook with `[A]`. Default: `false`
- `filter_inactive`: Filters inactive participants to not show in the phonebook. Default: `false`

Only relevant when running in **server mode**:

- `port`: Port to listen on (when running as a server). Default: `8080`
- `reload`: Duration after which to try to reload the phonebook source. Default: `1h`
- `conf`: Config file to read settings from instead of parsing flags. Default: ""

Only relevant when running in **server mode** AND **LDAP server** is active:

- `ldap_port`: Port to listen on for the LDAP server (when running as a server AND LDAP server is on as well). Default: `3890`
- `ldap_user`: Username to provide to connect to the LDAP server. Default: `aredn`
- `ldap_pwd`: Password to provide to connect to the LDAP server. Default: `aredn`

Only relevant when running in **server mode** AND **SIP server** is active:

- `sip_port`: Port to listen on for the SIP server (when running as a server AND SIP server is on as well). Default: `5060`

## Examples

Read CSV from a local file and write the XML files in the `/www` folder for Yealink phones:

```
go run phonebook.go -source='AREDN_Phonebook.csv' -targets='yealink' -formats='direct' -path=/www/
```

Read the CSV from a URL and write the XML files in the `/tmp` folder for Yealink and Cisco phones:

```
go run phonebook.go -source='http://aredn-node.local.mesh:8080/phonebook.csv' -targets='yealink,cisco' -formats='direct,pbx' -path=/tmp/
```

## OpenWRT / AREDN

In order to run this on an AREDN node, the `Makefile` in `openwrt` needs to be built into a package.
The following pointers provide the necessary starting points:

- https://openwrt.org/docs/guide-developer/toolchain/use-buildsystem
- https://openwrt.org/docs/guide-developer/toolchain/single.package
- https://openwrt.org/docs/guide-developer/packages

See https://github.com/arednch/packages/tree/main/phonebook for the definitions we use.

## Service

In order to run it as a service, set it up and run it as such:

`/etc/systemd/system/phonebook.service`
```
[Unit]
Description=Phonebook for AREDN.

[Service]
User=root
WorkingDirectory=/tmp/
ExecStart=/usr/bin/phonebook --server=true --port=8080 --source="<insert CSV source>" --olsr="/tmp/run/hosts_olsr" --sysinfo="http://localnode.local.mesh/cgi-bin/sysinfo.json?hosts=1"
Restart=always

[Install]
WantedBy=multi-user.target
```

The following reloads the services, starts it, enables it to run after reboots and gets its status:
```
sudo systemctl daemon-reload
sudo systemctl start phonebook.service
sudo systemctl enable phonebook.service
systemctl status phonebook.service
```

You could also simplify later re-deployments a bit:

```
#!/bin/sh

cd /tmp/
rm -rf phonebook
git clone https://github.com/arednch/phonebook.git phonebook
cd phonebook
go build .

cp phonebook /usr/bin/
systemctl restart phonebook.service
systemctl status phonebook.service
```

### Configuration

Optionally, instead of passing flags, the config values can be read from a JSON config file too:

```
go run . -conf="/etc/phonebook.conf" -server
```

A typical file would look like this:

```
{
	"source": "/www/arednstack/phonebook.csv",
	"olsr_file": "/tmp/run/hosts_olsr",
	"sysinfo_url": "http://localnode.local.mesh/cgi-bin/sysinfo.json?hosts=1",
	"ldap_server": true,
	"debug": false,
	"path": "/www/arednstack",
	"formats": [
		"combined",
		"direct",
		"pbx"
	],
	"targets": [
		"generic"
	],
	"resolve": false,
	"indicate_active": true,
	"filter_inactive": false,
	"active_pfx": "*",
	"port": 8081,
	"reload_seconds": 3600,
	"ldap_port": 3890,
	"ldap_user": "aredn",
	"ldap_pwd": "aredn"
}
```

The config allows to set the same paramaters as the flags (modulo the `conf` flag).

### Queries

The server can then be queried as follows (replace "server" and "port" accordingly):

http://server:port/phonebook?target=generic&format=combined

The same formats and targets as if run from the commandline are supported.

Additionally, the following parameters are supported:

- `resolve`: Set to `true` in order to attempt to resolve hostnames to IPs for phones based on OLSR data (this assumes that the data is available.)
- `ia`: Set to `true` in order to indicate active phones (i.e. there's a route) in the directory.
- `fi`: Set to `true` in order to filter the directory to just the active phones.

## Supported Devices

Note: The following list is not complete. It will work with many more devices. This is the list of "confirmed tested" devices at some point in time (not with every release!). If you have other devices that work, please let us know.

- Yealink T41P: XML (`generic`, `yealink`), LDAP
- Yealink T48G: XML (`generic`, `yealink`), LDAP
- Linphone (iOS, iPadOS, Android, Chromebook, Ubuntu): LDAP
