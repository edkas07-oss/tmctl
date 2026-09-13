#!/usr/bin/env bash
# Validation script for tmctl repository
set -euo pipefail

readonly SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly PROJECT_ROOT="$(dirname "${SCRIPT_DIR}")"

readonly REQUIRED_FILES=(
    "PROJECT"
    "VERSION"
    "CONFIG"
    "CONFIG.example"
    ".gitignore"
    "README.md"
    "AGENTS.md"
    "Makefile"
    "Jenkinsfile"
    "go.mod"
    "cmd/tmctl/main.go"
    "scripts/build.sh"
    "scripts/validate.sh"
    "scripts/test.sh"
)

fail() {
    printf 'VALIDATION FAILED: %s\n' "$1" >&2
    exit 1
}

validate_required_files() {
    for rel_path in "${REQUIRED_FILES[@]}"; do
        [[ -f "${PROJECT_ROOT}/${rel_path}" ]] || fail "Required file not found: ${rel_path}"
    done
}

validate_shell_syntax() {
    bash -n "${SCRIPT_DIR}"/*.sh
    bash -n "${PROJECT_ROOT}/CONFIG"
    bash -n "${PROJECT_ROOT}/CONFIG.example"
}

validate_sensitive_files() {
    local sensitive_file
    while IFS= read -r sensitive_file; do
        fail "Forbidden sensitive file found: ${sensitive_file#"${PROJECT_ROOT}/"}"
    done < <(
        find "${PROJECT_ROOT}" \
            -path "${PROJECT_ROOT}/.git" -prune -o \
            -type f \( \
                -name '*.pem' -o -name '*.key' -o -name '*.p12' -o \
                -name '*.pfx' -o -name '*.jks' -o -name '*.keystore' -o \
                -name '.env' -o -name '*.env' \
            \) -print
    )
}

validate_go_vet() {
    export PATH="${HOME}/.local/bin:${HOME}/.local/go/bin:${PATH}"
    go -C "${PROJECT_ROOT}" vet ./...
}

main() {
    validate_required_files
    validate_shell_syntax
    validate_sensitive_files
    validate_go_vet
    echo "tmctl validation passed: all layout, syntax, and static assertions valid."
}

main "$@"
