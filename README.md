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