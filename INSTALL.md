# 📦 Installation & Build Guide — `tmctl`

This document provides complete instructions for compiling, installing, and deploying the **`tmctl`** (*Unified Cross-Platform Operator CLI*) across Linux and Windows Server environments.

---

## 📑 Table of Contents

- [1. Prerequisites](#1-prerequisites)
- [2. Building from Source](#2-building-from-source)
- [3. Local Installation (`~/.local/bin`)](#3-local-installation-localbin)
- [4. Cross-Compilation Matrix](#4-cross-compilation-matrix)
- [5. Environment & Socket Configuration](#5-environment--socket-configuration)
- [6. Verification & Health Check](#6-verification--health-check)

---

## 1. Prerequisites

- **Go:** Version 1.23+ installed on build workstation.
- **Container Engine:** Podman 4.0+ or Docker Engine 24.0+ (running rootless or system socket).
- **Target OS:** Linux (`amd64` / `arm64`) or Windows Server 2019/2022/2025 (`amd64`).

---

## 2. Building from Source

To compile the native static binary for your current operating system and architecture:

```bash
# Clone repository
git clone git@github.com:edkas07-oss/tmctl.git
cd tmctl

# Build native binary into bin/
make build
```

This compiles a pure static binary (`CGO_ENABLED=0`) into `bin/tmctl` (or `bin/tmctl.exe` on Windows).

---

## 3. Local Installation (`~/.local/bin`)

To install `tmctl` into your user's `$PATH`:

```bash
make install
```

This copies the binary to `~/.local/bin/tmctl`. Ensure `~/.local/bin` is in your `$PATH`:
```bash
export PATH="${HOME}/.local/bin:${PATH}"
tmctl version
```

---

## 4. Cross-Compilation Matrix

To build the full matrix of production binaries for all supported platforms:

```bash
make build-all
```

Outputs generated in `bin/`:
- `bin/linux_amd64/tmctl` (Linux x86_64)
- `bin/linux_arm64/tmctl` (Linux ARM64 / AWS Graviton)
- `bin/windows_amd64/tmctl.exe` (Windows x86_64)
- `bin/checksums.txt` (SHA-256 integrity checksums)

---

## 5. Environment & Socket Configuration

`tmctl` automatically discovers the active container engine socket. You can optionally configure overrides in the `CONFIG` file or via environment variables:

| Variable | Default | Description |
| :--- | :--- | :--- |
| `CONTAINER_ENGINE` | *(Auto-detect)* | Override engine (`podman` or `docker`) |
| `SOCKET_PATH` | *(Auto-detect)* | Custom Unix socket path or Named Pipe |
| `PLATFORM_NAME` | `tomcat-monitoring` | Target platform namespace |
| `NETWORK_NAME` | `devops-lab` | OCI bridge network |

---

## 6. Verification & Health Check

Verify connection to container engine socket:

```bash
# Inspect container fleet status
tmctl stack status

# Validate repository layout and contracts
tmctl validate
```
