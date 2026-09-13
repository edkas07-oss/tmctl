# AGENTS.md — Developer and AI Agent Guidelines for `tmctl`

## 🎯 Repository Purpose
`tmctl` (*Tomcat Monitoring Control CLI*) adalah kakas baris perintah tunggal (*Single Static Binary*) berbasis bahasa Go yang berkomunikasi langsung dengan **Container Engine Socket API** (Podman / Docker) untuk orkestrasi kontainer dan manajemen platform secara seragam lintas OS (*Linux & Windows*).

## 🏛️ Architecture Rules & Non-Negotiables
1. **Zero External Runtime Dependency:** `tmctl` wajib dikompilasi dengan `CGO_ENABLED=0` tanpa ketergantungan interpreter eksternal (Python, Node.js, atau Bash) di mesin operator.
2. **Direct Socket API Communication:** Berkomunikasi langsung dengan Container Engine Socket (Unix Domain Socket di Linux, Named Pipe di Windows `\\.\pipe\docker_engine`, atau TCP mTLS).
3. **Cross-Compilation Matrix:** Mendukung kompilasi silang ke Linux (`amd64`, `arm64`) dan Windows (`amd64` menghasilkan `tmctl.exe`).
4. **Zero Secret Leakage:** Tidak boleh menyimpan kredensial atau sertifikat sensitif di repositori Git.
5. **Deterministic Exit Codes:** Mengembalikan kode keluar 0 untuk operasi sukses, dan non-nol untuk kegagalan.

## 🛠️ Build & Validation Commands
- Build Native: `make build` atau `./scripts/build.sh`
- Cross-Compile Matrix: `make build-all`
- Unit Testing: `make test` atau `./scripts/test.sh`
- Validation Suite: `make validate` atau `./scripts/validate.sh`
- Install to PATH: `make install` (memasang ke `~/.local/bin/tmctl`)
