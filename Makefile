# Makefile for vncgate 远程桌面
BIN := bin
NAME := vncgate
ARCHS := amd64 arm64 arm
BUILDTIME := $(shell date "+%Y-%m-%dT%H:%M:%S")
UPX := $(shell command -v upx 2>/dev/null)

# 版本号 (可按需通过命令行覆盖: make arm64 VERSION=x.x.x)
VERSION ?= 1.7.0
PKG_VERSION ?= $(shell date "+%Y%m%d%H%M%S")

.PHONY: all $(ARCHS) upx clean

all: $(ARCHS)

arm: GOARM=7

$(ARCHS):
	@mkdir -p $(BIN)
	@echo "Building $(NAME)($(VERSION)-$(PKG_VERSION)) (linux/$@)"
	@GOOS=linux GOARCH=$@ GOARM=$(GOARM) go build -trimpath -ldflags="-s -w \
		-X main.VERSION=$(VERSION) \
		-X main.BUILDTIME=$(BUILDTIME) \
		-X main.PKG_VERSION=$(PKG_VERSION)" \
		-o $(BIN)/$(NAME)-linux-$@ .

upx: all
	@command -v upx >/dev/null 2>&1 || { echo "upx 未安装, 请先安装"; exit 1; }
	@for a in $(ARCHS); do \
		echo "UPX compressing $(NAME)-linux-$$a"; \
		upx --best --lzma $(BIN)/$(NAME)-linux-$$a; \
	done

clean:
	rm -rf $(BIN)