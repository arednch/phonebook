# Phonebook

Phonebook conversion from CSV to XML intended to be used for AREDN.

## Flags

Generally applicable:

- `source`: Path or URL to fetch the phonebook CSV from. Default: ""
- `olsr`: Path to the OLSR hosts file. Default: `/tmp/run/hosts_olsr.stable`
- `server`: Phonebook acts as a server when set to true. Default: false

Only relevant when running in **non-server / ad-hoc mode**:

- `path`: Folder to write the phonebooks to locally. Default: ""
- `formats`: Comma separated list of formats to export.

		- Supported: combined
		- Default: "pbx,direct,combined"

- `targets`: Comma separated list of targets to export.

		- Supported: generic,yealink,cisco,snom
		- Default: ""

- `resolve`: Resolve hostnames to IPs when set to true using OSLR data. Default: `false`
- `indicate_active`: Prefixes active participants in the phonebook with `[A]`. Default: `false`
- `filter_inactive`: Filters inactive participants to not show in the phonebook. Default: `false`

Only relevant when running in **server mode**:

- `port`: Port to listen on (when running as a server). Default: `8080`
- `reload`: Duration after which to try to reload the phonebook source. Default: `1h`
- `conf`: Config file to read settings from instead of parsing flags. Default: ""

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

See https://github.com/finfinack/aredn-packages/tree/main/phonebook for the definitions we use.

## Service

In order to run it as a service, set it up and run it as such:

`/etc/systemd/system/phonebook.service`
```
[Unit]
Description=Phonebook for AREDN.

[Service]
User=root
WorkingDirectory=/tmp/
ExecStart=/usr/bin/phonebook --server=true --port=8080 --source="<insert CSV source>" --olsr="/tmp/run/hosts_olsr.stable"
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
git clone https://github.com/finfinack/phonebook.git phonebook
cd phonebook
go build .

cp phonebook /usr/bin/
systemctl restart phonebook.service
systemctl status phonebook.service
```

### Configuration

Optionally, instead of passing flags, the config values can be read from a config file too:

```
go run . -conf="config"
```

A typical file would look like this:

```
{
	"source": "http://aredn-node.local.mesh:8080/phonebook.csv",
	"olsr_file": "/tmp/run/hosts_olsr.stable",
	"server": false,
  "path": "/www",
	"formats": [
		"direct",
		"pbx",
		"combined"
	],
	"targets": [
		"yealink"
	],
	"resolve": false,
	"indicate_active": true,
	"filter_inactive": false,
	"port": 8080,
	"reload_seconds": 3600
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