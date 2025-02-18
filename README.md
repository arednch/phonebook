# Phonebook

Phonebook conversion from CSV to a number of output formats intended to be used for AREDN.

For release notes, see the [release page](https://github.com/arednch/packages/releases) or
[this document](https://docs.google.com/document/d/18D14Ch3GjUZmSRQALEKslvtEJ0O76pZkV3VNJ6vsB14/edit).

## Flags

Generally applicable:

- `conf`: Config file to read settings from instead of parsing flags. Default: ""
- `sources`: Comma separated list of paths and/or URLs to fetch the phonebook CSV from. Default: ""
- `olsr`: Path to the OLSR hosts file. Default: `/tmp/run/hosts_olsr`
- `sysinfo`: URL from which to fetch AREDN sysinfo. Usually: `http://localnode.local.mesh/cgi-bin/sysinfo.json?hosts=1`
- `server`: Phonebook acts as a server when set to true. Default: false
- `ldap_server`: When the phonebook is running as a server, it also exposes an LDAP v3 server when set to true. Default: false
- `sip_server`: When the phonebook is running as a server, it also runs a _very_ simple SIP server when set to true. Default: false
- `debug`: Print verbose debug messages on stdout when set to true. Default: false
- `allow_runtime_config_changes`: Allows runtime config changes via web server when set to true.
- `allow_permanent_config_changes`: Allows permanent config changes via web server when set to true.
- `include_routable`: Adds other routable phone numbers even when not in the phonebook. Default: `false`
- `country_prefix`: Mandatory three digit country prefix. Default: None.
- `active_pfx`: Prefix to add when -indicate_active is set. Default: `*`
- `indicate_active`: Prefixes active participants in the phonebook with `active_pfx`. Default: `false`

Primarily relevant when running in **non-server / ad-hoc mode**:

Note: These settings can also be used in server mode which means the output files will be produced as well.

- `path`: Folder to write the phonebooks to locally. Default: ""
- `formats`: Comma separated list of formats to export.

		- Supported: pbx,direct,combined
		- Default: "combined"

- `targets`: Comma separated list of targets to export.

		- Supported: generic,yealink,cisco,snom,grandstream,vcard
		- Default: ""

- `resolve`: Resolve hostnames to IPs when set to true using OLSR data. Default: `false`
- `filter_inactive`: Filters inactive participants to not show in the phonebook. Default: `false`

Only relevant when running in **server mode**:

- `port`: Port to listen on (when running as a server). Default: `8081`
- `cache`: Local folder to cache the downloaded phonebook CSV in (for reliability when the network goes down). Default: `/www/phonebook.csv`
- `reload`: Duration after which to try to reload the phonebook source. Default: `1h`
- `update_urls`: Comma separated list of URLs to fetch information from (used to send optional messages to users). Default: None.
- `web_user`: Username to protect many of the web endpoints with (BasicAuth). Default: None
- `web_pwd`: Password to protect many of the web endpoints with (BasicAuth). Default: None

	Note: Both `web_user` AND `web_pwd` need to be set in order to protect the endpoints.

	Note: See [web service](#web-service) section for a documentation of which endpoints are protected when this is turned on.

Only relevant when running in **server mode** AND **LDAP server** is active:

- `ldap_port`: Port to listen on for the LDAP server (when running as a server AND LDAP server is on as well). Default: `3890`
- `ldap_user`: Username to provide to connect to the LDAP server. Default: `aredn`
- `ldap_pwd`: Password to provide to connect to the LDAP server. Default: `aredn`

Only relevant when running in **server mode** AND **SIP server** is active:

- `sip_port`: Port to listen on for the SIP server (when running as a server AND SIP server is on as well). Default: `5060`

## Examples

Read CSV from a local file and write the XML files in the `/www` folder for Yealink phones:

```bash
go run phonebook.go -source='AREDN_Phonebook.csv' -targets='yealink' -formats='direct' -path=/www/
```

Read the CSV from a URL and write the XML files in the `/tmp` folder for Yealink and Cisco phones:

```bash
go run phonebook.go -source='http://aredn-node.local.mesh:8081/phonebook.csv' -targets='yealink,cisco' -formats='direct,pbx' -path=/tmp/
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
ExecStart=/usr/bin/phonebook --server=true --port=8081 --source="<insert CSV source>" --olsr="/tmp/run/hosts_olsr" --sysinfo="http://localnode.local.mesh/cgi-bin/sysinfo.json?hosts=1"
Restart=always

[Install]
WantedBy=multi-user.target
```

The following reloads the services, starts it, enables it to run after reboots and gets its status:

```bash
sudo systemctl daemon-reload
sudo systemctl start phonebook.service
sudo systemctl enable phonebook.service
systemctl status phonebook.service
```

You could also simplify later re-deployments a bit:

```bash
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

```bash
go run . -conf="/etc/phonebook.conf" -server
```

A typical file would look like this:

```json
{
  "sources": [
		"http://aredn-node-1.local.mesh/phonebook.csv",
		"http://aredn-node-2.local.mesh/phonebook.csv"
  ],
  "update_urls": [
		"http://aredn-node-1.local.mesh/updates.json",
		"http://aredn-node-2.local.mesh/updates.json"
  ],
	"olsr_file": "/tmp/run/hosts_olsr",
	"sysinfo_url": "http://localnode.local.mesh/cgi-bin/sysinfo.json?hosts=1",
  "ldap_server": true,
  "sip_server": true,
  "debug": true,
	"allow_runtime_config_changes": true,
	"allow_permanent_config_changes": false,
  "country_prefix": "041",
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
  "include_routable": true,
  "port": 8081,
  "cache": "/www/arednstack/phonebook.csv",
  "reload_seconds": 3600,
	"web_user": "aredn",
	"web_pwd": "notasecret",
  "ldap_port": 3890,
  "ldap_user": "aredn",
  "ldap_pwd": "aredn",
  "sip_port": 5060
}
```

The config allows to set the same paramaters as the flags (modulo the `conf` flag).

### Web Service

The phonebook exposes a web interface when run as a service. This chapter elaborates on
the exposed end points.

Note: We assume the standard AREDN setup and thus use "localnode.local.mesh" as the
host and "8081" as the port. Adjust it as needed if you are using it in another configuration.

Note: Each endpoint has a `BasicAuth protection` definition. This is a simple statement whether that endpoint needs BasicAuth authentication when `web_user` and `web_pwd` is set (see [flags](#flags) for more details) as well.

#### / and /index.html

This is the entry page and should provide access to the other endpoints in an understandable way.

Example: http://localnode.local.mesh:8081/index.html

BasicAuth protection: No.

Required parameters:

- n/a

Optional parameters:

- n/a

#### /info

This endpoint is meant for informational and debugging purposes as it exposes some information about the node and phonebook in a machine readable way.

Example: http://localnode.local.mesh:8081/info

BasicAuth protection: No.

Required parameters:

- n/a

Optional parameters:

- n/a

#### /phonebook

This endpoint is the primary one and generates a phonebook ad-hoc in the requested format.
It's primarily intended to be used directly by the phone as the URL to load the phonebook from.

Example: http://localnode.local.mesh:8081/phonebook?target=generic&format=combined&ia=true

BasicAuth protection: No.

Required parameters:

- `format`: Single value specifying the format (e.g. "combined"). See [flags](#flags) for more details.

- `target`: Single value specifying the target (e.g. "generic"). See [flags](#flags) for more details.

Optional parameters:

- `resolve`: Set to `true` in order to attempt to resolve hostnames to IPs for phones based on OLSR data (this assumes that the data is available.)

- `ia`: Set to `true` in order to indicate active phones (i.e. there's a route) in the directory.

- `fi`: Set to `true` in order to filter the directory to just the active phones.

#### /reload

This endpoint forces the phonebook server to attempt to reload the upstream phonebook (CSV) from whatever source is configured (usually a local file on disk updated by a cron job).

Example: http://localnode.local.mesh:8081/reload

BasicAuth protection: Yes.

Required parameters:

- n/a

Optional parameters:

- n/a

#### /message

This endpoint allows sending a SIP message to another participant.

Example: http://localnode.local.mesh:8081/message?to=800030&msg=test%20message

BasicAuth protection: Yes.

Required parameters:

- `from`: Phone number of the sender.
- `to`: Phone number of a recipient.
- `msg`: Message to send.

Optional parameters:

- n/a

#### /showconfig

This endpoint returns the currently loaded phonebook configuration in JSON format.
It's primarily intended for (local or remote) debugging purposes. Some fields may be censored (e.g. passwords).

Example: http://localnode.local.mesh:8081/showconfig?type=r

BasicAuth protection: Yes.

Required parameters:

- `type`: The type of the config needs to be set and can be either one of the following:

		- `disk` (alias `d`): The config from disk is loaded and displayed.
		- `runtime` (alias `r`): The runtime config is displayed.
		- `diff`: The config from disk is loaded and compared to the runtime config and a human readable diff between the two is displayed.

Optional parameters:

- n/a

#### /updateconfig

This endpoint allows to update a limited set of configuration settings.

Example: http://localnode.local.mesh:8081/updateconfig?sources=http://hb9edi-vm-gw.local.mesh/filerepo/Phonebook/AREDN_Phonebook.csv

BasicAuth protection: Yes.

Required parameters:

- n/a

Optional parameters:

- `perm`: When set to `true`, instructs the phonebook server to write the config to disk as well. If not set, changes are only made to the running service. When the service restarts (e.g. when the Node reboots), the config is read from disk again.

- `sources`: Defines the source paths or URLs to load the upstream phonebook from (CSV). See [flags](#flags) for more details.

- `reload`: Defines the amount of time to wait between reloading the phonebook data from the specified source. See [flags](#flags) for more details.

	Important: Be careful with updating this as it may overload upstream servers (depending on what the "source" is set to). The default value has been chosen specifically with that in mind.

- `debug`: Defines the debug output flag (set to "true" or "false"). See [flags](#flags) for more details.

- `routable`: Defines if routable phone numbers should be included even if not in the phonebook. See [flags](#flags) for more details.

- `webuser`: Defines the user required to authenticate via basicAuth for most web endpoints. See [flags](#flags) for more details.

- `webpwd`: Defines the password required to authenticate via basicAuth for most web endpoints. See [flags](#flags) for more details.

- `apfx`: Active prefix (what contacts get prefixed with when they're online). See [flags](#flags) for more details.

- `cpfx`: Three digit country prefix. See [flags](#flags) for more details.

## Supported Devices

**Notes**:

- The above list is not complete. It will work with many more devices. This is just the list of "confirmed tested" devices at some point in time.
- The list is NOT retested with each release so some changes are expected over time.
- If you have other devices that work or you're happy to test for us, please let us know.

**Legend**:

- 🟢: This feature works.
- 🟡: This feature only works partially with this device.
- 🔴: This feature does not work with this device.
- n/a: This hasn't been tested with that device.

### Nodes

|           | MikroTik hAP AC Lite | MikroTik hAP AC3  | Ubiquiti NanoBeam 5AC Gen 2 |
|:----------|:--------------------:|:-----------------:|:---------------------------:|
| SKU       | RB952Ui-5ac2nD       | RBD53iG-5HacD2HnD | n/a                         |
| Target    | ath79                | ipq40xx           | ath79                       |
| Arch      | MIPS                 | ARM Cortex        | MIPS                        |
| RAM       | 64MB                 | 256MB             | 128MB                       |
| Phonebook | 🟡                   | 🟢                | 🟢                           |

Note: Some devices do not have a lot of space (e.g. hAP Lite). If you run into problems:

  - Make sure you use the latest version (specifically, at least [v1.8.1](https://github.com/arednch/packages/releases/tag/v1.8.1) which provides a packed/smaller binary).
	- If the above didn't help: Make sure to uninstall other packages.
	- If the above didn't help: Uninstall previous versions of the phonebook before firmware upgrades or installing the latest version.
	- If the above didn't help: Reinstall the firmware from scratch (without keeping the configuration).

**Known Limitations**:

Note: This list is dynamic and we update it as well as possible but it's likely incomplete and not always up to date with the latest developments. If you have updates, please let us know.

### Phones

| **Feature**            | Yealink T48G | Yealink T41P | Grandstream GXP1620 | Snom D120 | Linphone ([iOS](https://apps.apple.com/app/linphone/id360065638))    | Acrobits Softphone ([iOS](https://apps.apple.com/app/acrobits-softphone/id314192799)) |
|:-----------------------|:------------:|:------------:|:-------------------:|:---------:|:-----------------:|:------------------------:|
| **Server**             |              |              |                     |           |                   |                          |
| Phonebook: XML         | 🟢           | 🟢           | n/a                  | n/a       | 🔴               | [🔴 (no support)](#generic-no-support) |
| Phonebook: vCard       | n/a          | n/a          | n/a                 | n/a       | 🔴                | [🔴 (no support)](#generic-no-support) |
| **LDAP**               |              |              |                     |           |                   |                          |
| Basic: Fetch contacts  | 🟢           | 🟢           | 🟢                   | n/a       | 🟢               | [🔴 (no support)](#generic-no-support) |
| Search                 | 🟢           | n/a          | n/a                 | n/a       | 🟢                | [🔴 (no support)](#generic-no-support) |
| **SIP**                |              |              |                     |           |                   |                          |
| Basic: Register        | 🟢           | 🟢           | 🟢                  | 🟢        | 🟢               | 🟢                       |
| Redirect calls (AREDN) | 🟢           | n/a          | 🟢                 | n/a       | [🟡 (only incoming)](#linphone-sip-redirect) | 🟢             |
| Redirect calls (local) | 🟢           | n/a          | n/a                 | n/a       | [🟡 (only incoming)](#linphone-sip-redirect) | 🟢             |
| Callback from History  | 🟢           | 🟢           | 🟢                  | n/a       | [n/a](#linphone-sip-redirect)                | 🟢             |

**Known Limitations**:

Note: This list is dynamic and we update it as well as possible but it's likely incomplete and not always up to date with the latest developments. If you have updates, please let us know.

- <a id="generic-no-support">No support</a>: The app or device does not support this feature, i.e. nothing we can change.

- <a id="linphone-sip-redirect">Linphone (SIP Call Redirects)</a>: Linphone does not correctly parse call redirection responses from SIP Servers and instead of calling the redirect contact on the new host, keep calling the original SIP server instead.
