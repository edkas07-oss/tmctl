# Makefile for tmctl (Unified Cross-Platform Operator CLI)

SHELL := /bin/bash
PROJECT := tmctl
BINARY_NAME := tmctl
VERSION := $(shell cat VERSION 2>/dev/null || echo "1.0.0")
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE := $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')
LDFLAGS := -s -w \
	-X 'github.com/eddywiyatno/tmctl/internal/buildinfo.Version=$(VERSION)' \
	-X 'github.com/eddywiyatno/tmctl/internal/buildinfo.GitCommit=$(GIT_COMMIT)' \
	-X 'github.com/eddywiyatno/tmctl/internal/buildinfo.BuildDate=$(BUILD_DATE)'

.PHONY: all build build-linux build-windows build-all test validate clean install help

all: test build

help:
	@echo "Targets:"
	@echo "  build         - Build native binary for Linux (amd64 ELF)"
	@echo "  build-linux   - Compile for Linux host platform (amd64 ELF)"
	@echo "  build-windows - Cross-compile for Windows (amd64 PE exe)"
	@echo "  build-all     - Compile for both Linux and Windows simultaneously"
	@echo "  test          - Run unit tests"
	@echo "  validate      - Validate contracts and syntax"
	@echo "  clean         - Clean build artifacts"
	@echo "  install       - Install binary to ~/.local/bin"

build: build-linux

build-linux:
	@echo "Building $(BINARY_NAME) for Linux (amd64)..."
	@mkdir -p bin
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o bin/$(BINARY_NAME) ./cmd/tmctl

build-windows:
	@echo "Cross-compiling $(BINARY_NAME) for Windows (amd64)..."
	@mkdir -p bin
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o bin/$(BINARY_NAME).exe ./cmd/tmctl

build-all: build-linux build-windows
	@echo "Multi-platform binaries ready in bin/:"
	@ls -la bin/

test:
	go test -v -race=false ./...

validate:
	./scripts/validate.sh

install: build-linux
	@mkdir -p $(HOME)/.local/bin
	cp bin/$(BINARY_NAME) $(HOME)/.local/bin/$(BINARY_NAME)
	@echo "Installed $(BINARY_NAME) to $(HOME)/.local/bin/$(BINARY_NAME)"

clean:
	rm -rf bin/ *.out *.tmp

