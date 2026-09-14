#!/usr/bin/env bash
# Build script for tmctl (Unified Operator CLI)
set -euo pipefail

readonly SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly PROJECT_ROOT="$(dirname "${SCRIPT_DIR}")"

VERSION="$(cat "${PROJECT_ROOT}/VERSION" 2>/dev/null || echo "0.1.0")"
GIT_COMMIT="$(git -C "${PROJECT_ROOT}" rev-parse --short HEAD 2>/dev/null || echo "unknown")"
BUILD_DATE="$(date -u +'%Y-%m-%dT%H:%M:%SZ')"

LDFLAGS="-s -w -X 'github.com/eddywiyatno/tmctl/internal/buildinfo.Version=${VERSION}' \
           -X 'github.com/eddywiyatno/tmctl/internal/buildinfo.GitCommit=${GIT_COMMIT}' \
           -X 'github.com/eddywiyatno/tmctl/internal/buildinfo.BuildDate=${BUILD_DATE}'"

export PATH="${HOME}/.local/bin:${HOME}/.local/go/bin:${PATH}"

echo "Building tmctl binaries..."
mkdir -p "${PROJECT_ROOT}/bin/linux_amd64" "${PROJECT_ROOT}/bin/linux_arm64" "${PROJECT_ROOT}/bin/windows_amd64"

# Linux amd64
echo "1. Building Linux amd64 static binary..."
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="${LDFLAGS}" -o "${PROJECT_ROOT}/bin/linux_amd64/tmctl" "${PROJECT_ROOT}/cmd/tmctl"
cp "${PROJECT_ROOT}/bin/linux_amd64/tmctl" "${PROJECT_ROOT}/bin/tmctl"

# Linux arm64
echo "2. Building Linux arm64 static binary..."
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -ldflags="${LDFLAGS}" -o "${PROJECT_ROOT}/bin/linux_arm64/tmctl" "${PROJECT_ROOT}/cmd/tmctl"

# Windows amd64
echo "3. Building Windows amd64 binary (tmctl.exe)..."
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="${LDFLAGS}" -o "${PROJECT_ROOT}/bin/windows_amd64/tmctl.exe" "${PROJECT_ROOT}/cmd/tmctl"

echo "Build complete. Artifacts generated in bin/:"
ls -lh "${PROJECT_ROOT}/bin/linux_amd64/tmctl" "${PROJECT_ROOT}/bin/linux_arm64/tmctl" "${PROJECT_ROOT}/bin/windows_amd64/tmctl.exe"
