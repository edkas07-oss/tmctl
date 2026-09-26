# TM-ADR-0032 — Consolidation of Fragmented Observability Landscape into tmctl Super Operator, Pure Pull-Based GitOps Engine, and Host Telemetry Agent Absorption

| Property | Value |
| --- | --- |
| **ADR ID** | TM-ADR-0032 |
| **Title** | Consolidation of Fragmented Observability Landscape into tmctl Super Operator, Pure Pull-Based GitOps Engine, and Host Telemetry Agent Absorption |
| **Project** | Tomcat Monitoring |
| **Section** | Observability Architecture, Tooling Consolidation, GitOps Automation, and Host Lifecycle Governance |
| **Status** | Accepted |
| **Date** | 2026-09-26 |

---

## 🔍 Overview

Dokumen keputusan arsitektur (*Architecture Decision Record* — ADR) ini secara formal menetapkan:
1. **Konsolidasi Seluruh Lanskap Tomcat Monitoring ke dalam `tmctl` sebagai *Super Operator* Tunggal**: Menghilangkan fragmentasi operasional yang sebelumnya terpecah di 5 repositori terpisah, dan mengangkat `tmctl` menjadi operator mandiri (*standalone single-binary*) dengan tingkat kapabilitas dan filosofi rancangan yang setara (*1-to-1 feature parity*) dengan `tcctl`.
2. **Penyerapan Penuh Host Telemetry Agent (`tm-agent`) & Pensiunnya Skrip Bash (`tomcat-diagnostic-event-collector`)**: Mengintegrasikan mesin *streaming* socket event kontainer (`died`, `oom`, `restart`), penulisan bukti crash secara atomik (`0600` `.tmp` $\rightarrow$ `.json`), serta manajemen retensi spool mandiri langsung ke dalam subperintah `tmctl agent`, sekaligus memensiunkan skrip Bash warisan `collector.sh`.
3. **Penerapan Mekanisme DevOps *Pure Pull-Based GitOps* (`tmctl gitops`)**: Menggantikan pendekatan *push-based* orkestrasi Ansible controller dengan mesin rekonsiliasi GitOps otonom berbasis manifest deklaratif `monitoring-spec.yaml`, polling commit via REST API Gitea/GitHub (tanpa dependensi Git CLI pada Windows), pendeteksian penyimpangan (*drift detection*), serta penjadwalan native OS (*systemd user timer* di Linux dan *Windows Task Scheduler* di Windows Server).
4. **Integrasi Mesin Triage Diagnostik & Pengujian Alert (`tmctl diagnostic`)**: Menyediakan evaluasi heuristik deterministik langsung terhadap kegagalan runtime (Cgroup OOM Killer 137, SIGABRT 134, SIGTERM 143) serta pengiriman *synthetic alert probe* untuk memverifikasi jalur Alertmanager dan Postfix relay dari hulu ke hilir.
5. **Penyediaan Daemon API Ringan (`tmctl serve`)**: Membuka antarmuka HTTP REST API lokal terlindungi API Key untuk integrasi dashboard terpusat dan *Self-Service Portal*.

---

## 🌍 Context & Analisis Temuan (Root Cause Analysis)

Hasil tinjauan arsitektur komprehensif terhadap ekosistem Tomcat Monitoring mengidentifikasi friksi operasional dan *technical debt* yang substansial:

### 1. Fragmentasi Berlebih (*Over-Engineering across 5 Repositories*)
Ekosistem pemantauan sebelumnya terpecah ke dalam lima repositori berbeda:
* `tomcat-monitoring`: Repositori orkestrasi berat berbasis Ansible (playbook, role, inventory).
* `tomcat-diagnostic-event-collector`: Daemon Bash berbasis skrip `collector.sh`.
* `tm-agent`: Daemon berbasis Go yang bertugas memantau event socket.
* `tomcat-diagnostic-service`: Microservice Node.js analitik diagnostik AI.
* `tmctl`: CLI operator Go yang awalnya hanya menangani subset deployment kontainer.

Kondisi multi-repo ini memicu *cognitive overload*, perlunya memelihara 5 pipeline CI/CD Jenkins yang rentan mengalami desinkronisasi versi dan skema kontrak (*schema desynchronization*).

### 2. Redundansi Murni antara Bash Collector dan Go Agent
Keberadaan `tomcat-diagnostic-event-collector` (Bash) dan `tm-agent` (Go) secara bersamaan merupakan pemborosan pemeliharaan. Keduanya melakukan hal yang sama: memantau event socket kontainer dan menulis rekaman JSON ke `/opt/tm-home/spool`. Terlebih lagi, `tm-agent` menduplikasi 80% modul internal yang sudah ada di `tmctl` (`internal/engine`, `internal/validator`, `internal/config`).

### 3. Hambatan Operasional Pendekatan Push-Based Ansible
Penggunaan Ansible controller untuk mengelola pemantauan harian memerlukan mesin kontroler terpisah, pembukaan port dan akun SSH/WinRM di setiap target host, dependensi Python, serta risiko kegagalan pipeline jika koneksi jaringan terputus saat eksekusi playbook.

### 4. Kontradiksi terhadap Filosofi Unggul `tcctl`
Pada domain beban kerja aplikasi Tomcat, kakas `tcctl` telah membuktikan keberhasilan konsep *Super Operator*: satu biner statis Go mandiri yang mencakup deployment blue-green, audit CIS hardening, sertifikat SSL, pemindaian kerentanan (VA), REST API daemon, hingga mesin GitOps mandiri. Sangat janggal apabila domain pemantauan (*monitoring*) justru mempertahankan arsitektur lama yang terfragmentasi.

---

## 💡 Keputusan Arsitektur (*Architectural Decisions*)

```text
                TRANSISI ARSITEKTUR OBSERVABILITAS TOMCAT MONITORING
  
  [ARSITEKTUR LAMA: TERFRAGMENTASI & PUSH-BASED]
  Ansible Controller (SSH/WinRM Push)
     ├── tomcat-monitoring (Playbooks, Roles, Inventory)
     ├── tomcat-diagnostic-event-collector (Skrip Bash Spooler)
     ├── tm-agent (Go Daemon Terpisah)
     ├── tmctl (CLI Parsial)
     └── tomcat-diagnostic-service (Container Image)
  
                             ⬇ KONSOLIDASI RADIKAL ⬇
  
  [ARSITEKTUR BARU: UNIFIED SUPER OPERATOR & PULL-BASED GITOPS]
  Target Host (Linux / Windows Server)
     └── tmctl (The Single Super Operator Binary)
           ├── tmctl stack       ──► Orkestrasi Kontainer (Socket REST API)
           ├── tmctl agent       ──► Host Telemetry & Spool (Absorpsi tm-agent)
           ├── tmctl gitops      ──► Pull-Based Reconciler (monitoring-spec.yaml)
           ├── tmctl diagnostic  ──► Crash Triage & Synthetic Alert Probe
           ├── tmctl rules       ──► AI Diagnostic Rulepack Ingestion
           └── tmctl serve       ──► Embedded REST API Daemon
```

### 1. Konsolidasi Peran ke dalam `tmctl`
Menetapkan `tmctl` sebagai *Single Source of Truth* dan satu-satunya biner yang didistribusikan ke server target. Menghapus kebutuhan distribusi biner terpisah untuk agen pengumpul telemetry.

### 2. Penghapusan Skrip Bash Warisan
Secara formal mengarsipkan repositori `tomcat-diagnostic-event-collector`. Seluruh fungsi pengumpulan event dan pengelolaan spool kini ditangani secara lintas platform (Linux & Windows) oleh paket `internal/agent` di dalam `tmctl`.

### 3. Pengadopsian Pure Pull-Based GitOps (`internal/gitops`)
Mengadopsi pola rekonsiliasi yang identik dengan `tcctl gitops`:
* **Manifest Deklaratif:** Didefinisikan dalam berkas `monitoring-spec.yaml`.
* **Zero External Git CLI Dependency:** Polling commit SHA dan pengunduhan manifest dilakukan via HTTP REST API standar terhadap Git server (Gitea / GitHub).
* **Native OS Schedulers:** Rekonsiliasi dijalankan secara periodik via `systemd --user timer` pada Linux atau *Windows Task Scheduler* (`tmctl-gitops-reconciler`) pada Windows Server.
* **Autonomous Self-Healing:** Jika kontainer monitoring mati atau konfigurasi menyimpang dari Git, `tmctl gitops sync` akan mengembalikan kondisi sesuai spesifikasi tanpa campur tangan manual.

### 4. Spesifikasi Kontrak Deklaratif (`monitoring-spec.yaml`)
Format manifest terstandarisasi untuk mengontrol seluruh lapisan pemantauan:

```yaml
version: "1.0"
metadata:
  environment: "production"
  tier: "monitoring"
  topology: "all-in-one"

spec:
  engine: "auto" # podman / docker
  agent:
    enabled: true
    targetContainer: "tomcat-jmx-exporter"
    spoolDir: "/opt/tm-home/spool"
    retentionHours: 24
    maxFiles: 1000

  stack:
    diagnosticService:
      image: "localhost:5000/tomcat-diagnostic-service:1.0.0"
      port: 3000
      memoryLimit: "512M"
      enabled: true
    prometheus:
      image: "prom/prometheus:v2.45.0"
      port: 9090
      memoryLimit: "1G"
      enabled: true
    alertmanager:
      image: "prom/alertmanager:v0.25.0"
      port: 9093
      memoryLimit: "256M"
      enabled: true
    postfixRelay:
      image: "localhost:5000/postfix-relay:1.0.0"
      port: 587
      enabled: true

  rules:
    autoIngest: true
    rulepackPath: "rules/enterprise-rulepack.json"
```

---

## ⚡ Konsekuensi (*Consequences*)

### Positif:
1. **Penyederhanaan Operasional Drastis:** Operator hanya perlu mengelola 1 file biner `tmctl` dan 1 file manifest deklaratif `monitoring-spec.yaml`.
2. **Eliminasi Kebutuhan Ansible Controller:** Menghapus kebutuhan server kontroler Ansible terdedikasi, tidak ada lagi isu timeout SSH atau manajemen kredensial push-based.
3. **Penyelarasan Mental Model Tim (*Cognitive Alignment*):** Pola kerja antara manajemen aplikasi (`tcctl`) dan pemantauan (`tmctl`) menjadi 100% seragam.
4. **Self-Healing Otomatis:** Perbaikan otomatis terhadap kontainer yang mati atau konfigurasi yang melenceng langsung dari host lokal.
5. **Keamanan Jaringan Maksimal:** Node host tidak perlu membuka port inbound SSH/WinRM untuk keperluan otomasi rutin karena node menarik perubahan secara *outbound pull*.

### Pertimbangan / Dampak Transisi:
1. Ukuran biner statis `tmctl` bertambah moderat dari ~5.7 MB menjadi ~6.9 MB (Linux) dan ~7.2 MB (Windows), yang masih sangat ringan untuk ukuran static binary lengkap dengan TLS, GitOps client, dan engine parser.
2. Repositori `tomcat-monitoring` beralih fungsi dari runner Ansible aktif menjadi repositori deklaratif manifest GitOps (`tomcat-monitoring-gitops`).
