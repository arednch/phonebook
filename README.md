# Phonebook

Phonebook conversion from CSV to XML intended to be used for AREDN.

## Examples

Read CSV from a local file and write the XML files in the default `/www` folder for Yealink phones:

```
go run phonebook.go -source='AREDN_Phonebook.csv' -formats='yealink'
```

Read the CSV from a URL and write the XML files in the default folder for Yealink and Cisco phones:

```
go run phonebook.go -source='http://aredn-node.local.mesh:8080/phonebook.csv' -formats='yealink,cisco'
```

## OpenWRT / AREDN

In order to run this on an AREDN node, the `Makefile` in `openwrt` needs to be built into a package.
The following pointers provide the necessary starting points:

- https://openwrt.org/docs/guide-developer/toolchain/use-buildsystem
- https://openwrt.org/docs/guide-developer/toolchain/single.package
- https://openwrt.org/docs/guide-developer/packages

## Service

In order to run it as a service, set it up and run it as such:

`/etc/systemd/system/phonebook.service`
```
[Unit]
Description=Phonebook for AREDN.

[Service]
User=root
WorkingDirectory=/tmp/
ExecStart=/usr/bin/phonebook --server=true --port=8080 --source="<insert CSV source>"
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