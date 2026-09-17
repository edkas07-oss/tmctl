# AGENTS.md — Developer & AI Agent Guidelines for `tmctl`

## 🎯 Repository Purpose
`tmctl` (*Tomcat Monitoring Control CLI*) is a unified, single static Go binary designed to interface directly with **Container Engine Socket APIs** (Podman and Docker). It orchestrates containers, manages diagnostic rules, authenticates registries, and validates platform compliance uniformly across operating systems (*Linux & Windows*), eliminating reliance on legacy, brittle Bash scripts.

## 🏛️ Architecture Rules & Non-Negotiables
1. **Zero External Runtime Dependency:** `tmctl` MUST be compiled with `CGO_ENABLED=0` without external runtime interpreter dependencies (Python, Node.js, or Bash) on the operator host.
2. **Direct Socket API Communication:** Communicates directly with the Container Engine Socket (Unix Domain Socket on Linux, Named Pipe on Windows `\\.\pipe\docker_engine`, or TCP mTLS).
3. **Cross-Compilation Matrix:** Supports native cross-compilation for Linux (`amd64`, `arm64`) and Windows (`amd64` producing `tmctl.exe`).
4. **Zero Secret Leakage:** Never store plaintext passwords, bearer tokens, or private TLS keys in the Git repository.
5. **Deterministic Exit Codes:** Return exit code 0 for successful operations and non-zero for failures.
6. **Two-Tier Storage Standard:** Align configuration and secret paths with `/opt/tm-home` (Linux) and `C:\tm-home` (Windows).

## 🛠️ Build & Validation Commands
- **Build Native:** `make build` or `./scripts/build.sh`
- **Cross-Compile Matrix:** `make build-all`
- **Unit Testing:** `make test` or `./scripts/test.sh`
- **Validation Suite:** `make validate` or `./scripts/validate.sh`
- **Install to PATH:** `make install` (installs to `~/.local/bin/tmctl`)
