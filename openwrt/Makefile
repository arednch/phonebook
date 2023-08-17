include $(TOPDIR)/rules.mk

PKG_NAME:=phonebook
PKG_VERSION:=v0.1
PKG_RELEASE:=$(AUTORELEASE)

PKG_LICENSE:=Apache-2.0
PKG_LICENSE_FILES:=LICENSE
PKG_MAINTAINER:=Martin Suess

PKG_SOURCE:=phonebook-$(PKG_VERSION).tar.gz
PKG_SOURCE_URL:=https://codeload.github.com/finfinack/phonebook/tar.gz/${PKG_VERSION}?
PKG_HASH:=skip
PKG_BUILD_DEPENDS:=golang/glog
PKG_BUILD_PARALLEL:=1

GO_PKG:=github.com/finfinack/phonebook

include $(INCLUDE_DIR)/package.mk
include ../../lang/golang/golang-package.mk

define Package/phonebook
  SECTION:=net
  CATEGORY:=Network
  SUBMENU:=Telephony
  TITLE:=A phonebook fetching and conversion tool intended for AREDN.
  URL:=https://github.com/finfinack/phonebook
  DEPENDS:=$(GO_ARCH_DEPENDS) +ca-bundle
endef

$(eval $(call GoBinPackage,phonebook))
$(eval $(call BuildPackage,phonebook))