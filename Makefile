# Makefile for tmctl (Unified Cross-Platform Operator CLI)

BINARY_NAME=tmctl
VERSION=$(shell cat VERSION 2>/dev/null || echo "0.1.0")
GIT_COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE=$(shell date -u +'%Y-%m-%dT%H:%M:%SZ')
LDFLAGS=-s -w -X 'github.com/eddywiyatno/tmctl/internal/buildinfo.Version=$(VERSION)' \
           -X 'github.com/eddywiyatno/tmctl/internal/buildinfo.GitCommit=$(GIT_COMMIT)' \
           -X 'github.com/eddywiyatno/tmctl/internal/buildinfo.BuildDate=$(BUILD_DATE)'

.PHONY: all build build-all test lint clean install validate help

all: test build

help:
	@echo "Targets:"
	@echo "  build       - Build native binary for current OS/Arch"
	@echo "  build-all   - Cross-compile for Linux (amd64, arm64) and Windows (amd64)"
	@echo "  test        - Run unit tests"
	@echo "  validate    - Validate contracts and syntax"
	@echo "  clean       - Clean build artifacts"
	@echo "  install     - Install binary to ~/.local/bin"

build:
	@mkdir -p bin
	CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o bin/$(BINARY_NAME) ./cmd/tmctl

build-all:
	@mkdir -p bin/linux_amd64 bin/linux_arm64 bin/windows_amd64
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o bin/linux_amd64/$(BINARY_NAME) ./cmd/tmctl
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o bin/linux_arm64/$(BINARY_NAME) ./cmd/tmctl
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o bin/windows_amd64/$(BINARY_NAME).exe ./cmd/tmctl
	cp bin/linux_amd64/$(BINARY_NAME) bin/$(BINARY_NAME)
	@(cd bin && sha256sum linux_amd64/$(BINARY_NAME) linux_arm64/$(BINARY_NAME) windows_amd64/$(BINARY_NAME).exe $(BINARY_NAME) > checksums.txt)


test:
	go test -v -race=false ./...

validate:
	./scripts/validate.sh

install: build
	@mkdir -p $(HOME)/.local/bin
	cp bin/$(BINARY_NAME) $(HOME)/.local/bin/$(BINARY_NAME)
	@echo "Installed tmctl to $(HOME)/.local/bin/$(BINARY_NAME)"

clean:
	rm -rf bin/ *.out *.tmp
