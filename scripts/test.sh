#!/usr/bin/env bash
# Test runner for tmctl
set -euo pipefail

readonly SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly PROJECT_ROOT="$(dirname "${SCRIPT_DIR}")"

export PATH="${HOME}/.local/bin:${HOME}/.local/go/bin:${PATH}"

echo "Running Go unit test suite for tmctl..."
go -C "${PROJECT_ROOT}" test -v ./...
echo "All tests passed successfully!"
