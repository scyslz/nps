PKG_NAME = npc
PKG_VERS = 0.26.0
PKG_EXT = tar.gz
PKG_DIST_SITE = https://github.com/djylb/nps/archive
PKG_DIST_NAME = v$(PKG_VERS).$(PKG_EXT)
PKG_DIR =  $(PKG_NAME)-$(PKG_VERS)
PKG_DIST_FILE = $(PKG_NAME)-$(PKG_VERS).$(PKG_EXT)

HOMEPAGE = https://github.com/djylb/nps
COMMENT  = nps是一款轻量级、高性能、功能强大的内网穿透代理服务器。
LICENSE  = ISC

DEPENDS = native/go

CONFIGURE_TARGET = nope
COMPILE_TARGET = npc_compile
INSTALL_TARGET = npc_install

include ../../mk/spksrc.cross-cc.mk

UNSUPPORTED_ARCHS = $(PPC_ARCHES)

ifeq ($(findstring $(ARCH),$(ARM5_ARCHES)),$(ARCH))
GO_ARCH = arm
ENV += GOARM=5
endif
ifeq ($(findstring $(ARCH),$(ARM7_ARCHES)),$(ARCH))
GO_ARCH = arm
ENV += GOARM=7
endif
ifeq ($(findstring $(ARCH),$(ARM8_ARCHES)),$(ARCH))
GO_ARCH = arm64
endif
ifeq ($(findstring $(ARCH),$(x86_ARCHES)),$(ARCH))
GO_ARCH = 386
endif
ifeq ($(findstring $(ARCH),$(x64_ARCHES)),$(ARCH))
GO_ARCH = amd64
endif
# Go has no ppc32 support for qoriq https://github.com/golang/go/issues/22885
#ifeq ($(findstring $(ARCH),$(PPC_ARCHES)),$(ARCH))
#GO_ARCH = ppc32
#endif

NPC_DIR = $(WORK_DIR)/src/github.com/$(PKG_NAME)/nps-$(PKG_VERS)
EXTRACT_PATH = $(WORK_DIR)/src/github.com/$(PKG_NAME)
NPC_BIN = $(WORK_DIR)/src/github.com/$(PKG_NAME)/nps-$(PKG_VERS)/$(PKG_NAME)

CONF_ARGS = -ldflags="-s -w"
ENV += GOPATH="$(WORK_DIR)" \
    GOOS=linux \
    GOARCH="$(GO_ARCH)" \
    CGO_ENABLED=0 \
    GOPROXY=direct \
    PATH=$(WORK_DIR)/../../../native/go/work-native/go/bin/:$$PATH \

.PHONY: npc_compile
npc_compile:
    mkdir -p $(GOPATH)/bin
    cd $(NPC_DIR) && cp -rf /spksrc/nps/* ./ && env $(ENV) go build $(CONF_ARGS) cmd/npc/npc.go

.PHONY: npc_install
npc_install:
    mkdir -p $(STAGING_INSTALL_PREFIX)/bin
    install -m 755 $(NPC_BIN) $(STAGING_INSTALL_PREFIX)/bin
