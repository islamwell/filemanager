# Production-Ready Go File Manager

A secure, high-performance, self-contained file and snippet manager web application built in idiomatic Go (1.22+). Designed for small teams (~10 users) with zero-dependency binary distribution, hierarchical folder organization up to 4 levels deep, tagging, fuzzy search, single-click MP3 audio streaming, and secure shareable links with optional expiration.

---

## 🌟 Key Features

1. **Authentication & Session Management**:
   - Secure user registration and login.
   - Passwords hashed with `bcrypt` (cost 12).
   - Session cookies hardened with `HttpOnly`, `SameSite=Lax`, and configurable `Secure` HTTPS flags.
2. **File Storage & Streaming**:
   - Configurable upload limit (default 100 MB).
   - Direct-to-disk streaming (`io.Copy` with 32 KB chunk buffers) prevents RAM exhaustion.
   - Sharded UUID disk storage paths isolate files and prevent filesystem directory limits.
   - Strict filename sanitization and RFC 5987 UTF-8 `Content-Disposition` headers eliminate path traversal risks.
   - SHA-256 integrity checksums computed on the fly.
3. **In-Browser Media Previews & Streaming**:
   - Single-click MP3 & audio playback with sticky bottom player bar.
   - In-browser image lightbox modal for instant full-resolution preview (`PNG`, `JPEG`, `GIF`, `WebP`, `SVG`).
   - Native HTML5 `<video>` player supporting HTTP byte-range request streaming (`206 Partial Content`, `Accept-Ranges: bytes`) for smooth seeking.
   - In-browser PDF preview frame without forcing file downloads.
4. **Hierarchical Folders & Item Organization**:
   - Full nested directory organization up to 4 levels deep: `Root (Level 1) ➔ Folder (Level 2) ➔ Sub-folder (Level 3) ➔ Deepest (Level 4)`.
   - Reassign and move any file or text snippet between folders or back to Root with 1 click.
   - Strict server-side depth limit enforcement (5th level insertion is rejected).
   - Breadcrumb navigation paths (e.g. `Work / 2026 / Audio / Podcast`).
5. **Text Snippets & Markdown Rendering**:
   - Create and edit existing plain-text / code snippets (up to 10,000 runes).
   - Live character counter.
   - Client-side Markdown preview toggle (renders headings, bold, code blocks, quotes, lists).
   - One-click copy to clipboard.
6. **Tags & Fuzzy Search**:
   - Tagging on all files and snippets with aggregated tag pills.
   - Multi-stage fuzzy search algorithm incorporating exact match, prefix matching, word boundary heuristics, subsequence matching, and Levenshtein typo tolerance.
7. **Shareable Public Links & Custom Vanity URLs**:
   - 32-byte cryptographically secure random tokens (`base64url`) or **custom vanity aliases** (e.g. `/share/q3-presentation`, `/share/team-audio`).
   - Configurable expiration (1 hour, 24 hours, 7 days, 30 days, or permanent).
   - Instant owner revocation.
   - Public view, download, audio streaming, and rich media previews without authentication.
   - Automatic access view counter.
   - Drag and drop files from desktop directly onto the browser window with visual drop target overlay.
   - Press <kbd>Ctrl+V</kbd> / <kbd>Cmd+V</kbd> anywhere to instantly upload pasted screenshots or images from the clipboard.
   - Paste plain text directly to create new text snippets.
9. **Dark / Light Mode System**:
   - Zero-FOUC modern theme switching adhering to system preferences and persisted in localStorage.
   - Fully styled dark palette with CSS variables and responsive navigation toggle.
10. **Bulk Multi-Selection & Batch Actions**:
    - Multi-select checkboxes for files and snippets with floating action bar.
    - On-the-fly streaming ZIP download (`archive/zip`) with zero disk/RAM bloat.
    - Batch deletion and batch relocation across folder hierarchies.
11. **Advanced Share Security**:
    - Optional bcrypt password protection on public share links with secure session unlock.
    - Self-destruct view limits (`max_views`) expiring links automatically after reached access counts.
12. **Inline Renaming & Storage Quota Meter**:
    - Quick inline modal renaming for files and snippets without page reloading.
    - Real-time disk storage analytics breaking down bytes by image, audio, video, documents, and other.
13. **Progressive Web App (PWA) & Offline Mode**:
    - Full offline capability with Service Worker (`/sw.js`) and Web App Manifest (`/manifest.json`).
    - Dedicated offline fallback page (`/offline`) allowing users to view and copy cached snippets offline.
    - Automatic snippet caching in browser localStorage upon viewing.
    - Real-time network status indicator banner.
14. **Admin Approval & User Access Management**:
    - First registered user automatically becomes the primary administrator and is immediately approved.
    - Subsequent registrations are queued with pending approval and cannot log in until approved.
    - Admin dashboard (`/admin/users`) to approve, revoke, promote/demote, and manage user accounts.
    - Instant session revocation when user approval is revoked.
15. **Security Hardening**:
    - In-memory sliding window IP rate limiting on `/login` and `/register` to stop brute-force attacks and prevent bcrypt CPU exhaustion.
    - Double-submit CSRF cookie protection on all state-changing endpoints.
    - Context-aware XSS escaping with Go's `html/template`.
    - Strict HTTP security headers (`Content-Security-Policy`, `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, `Referrer-Policy`).
16. **Operations & Monitoring**:
    - Unauthenticated `/healthz` endpoint returning JSON health status, version, and server timestamp.
    - Zero external runtime dependencies (HTML, CSS, JS embedded via `embed.FS`; pure Go SQLite).
17. **Footer Versioning**:
    - Displays version and update timestamp (`v1.0.16 (updated 2026-10-14 10:54)`).

---

## 🛠️ Architecture Overview

```
cmd/server/main.go          # Server entry point, CLI flags, graceful shutdown
internal/
├── auth/                   # Bcrypt password hashing & crypto session tokens
├── config/                 # Typed config loader (flags & env vars)
├── handlers/               # HTTP handlers (auth, files, snippets, categories, share, search)
├── middleware/             # Logging, security headers, sessions, CSRF, max body limit
├── models/                 # Domain models (User, FileItem, Snippet, Category, ShareLink, Tag)
├── services/               # Fuzzy search scoring, versioning constants
├── static/                 # Embedded CSS & client JavaScript
├── store/                  # SQLite storage repository (WAL mode, foreign keys, indexes)
└── templates/              # Embedded HTML5 templates (auto-escaped)
tests/                      # Automated unit and integration test suite
```

---

## 🚀 Quick Start

### Prerequisites
- Go 1.22 or newer.

### Clone and Run
```bash
# 1. Run the server directly
go run cmd/server/main.go

# 2. Or build a standalone binary
go build -o filemanager cmd/server/main.go
./filemanager
```

The web application will start at **`http://localhost:8080`**.

---

## ⚙️ Configuration

You can configure the file manager using CLI flags or environment variables:

| Parameter | Flag | Environment Variable | Default | Description |
|---|---|---|---|---|
| **Port** | `-port` | `PORT` | `8080` | HTTP listening port |
| **Host** | `-host` | `HOST` | `0.0.0.0` | HTTP listening host address |
| **Storage Dir** | `-storage` | `STORAGE_DIR` | `./data/storage` | Directory for physical file storage |
| **Database Path** | `-db` | `DATABASE_PATH` | `./data/filemanager.db` | Path to SQLite database file |
| **Max File Size** | `-max-file-size` | `MAX_FILE_SIZE` | `104857600` (100MB) | Maximum upload file size in bytes |
| **Secure Cookie** | `-secure-cookie` | `SECURE_COOKIE` | `false` | Require HTTPS for session cookies |

### Example: Running in Production with Custom Config
```bash
./filemanager \
  -port 9000 \
  -storage /var/data/files \
  -db /var/data/app.db \
  -max-file-size 209715200 \
  -secure-cookie=true
```

---

## 🔒 Security Design Details

### 1. Password Hashing
Uses `golang.org/x/crypto/bcrypt` with a cost factor of `12`. Passwords are authenticated in constant time.

### 2. CSRF Defense (Double-Submit Pattern)
All `POST`, `PUT`, and `DELETE` actions require a valid CSRF token matching the secure cookie. Every form includes `<input type="hidden" name="_csrf" value="{{.CSRFToken}}">`.

### 3. Path Traversal Mitigation
Untrusted client filenames (e.g. `../../etc/passwd`) are cleaned using `filepath.Base` and sanitized with strict regex character stripping. Physical storage uses UUID-named files in sharded subdirectories (`data/storage/ab/abcd-1234.dat`), completely decoupling client filenames from the server filesystem.

### 4. Streaming Without Memory Exhaustion
File uploads do not buffer 100MB in RAM. The request body is limited by `http.MaxBytesReader` and streamed directly to disk in 32KB chunks via `io.MultiWriter`, computing the SHA-256 checksum simultaneously.

### 5. Why Random Tokens Over Signed URLs for Sharing
- **Revocation**: Deleting a database record revokes the link instantly. Signed URLs cannot be revoked without maintaining a revocation blacklist.
- **Privacy**: Random tokens are opaque (256-bit entropy) and leak zero item IDs, user IDs, or metadata in query parameters.
- **Analytics**: Enables tracking view counts and enforcing view limits.

---

## 🧪 Running Automated Tests

Run the complete test suite with Go's race detector:

```bash
go test -v -race ./...
```

### Covered Test Suites:
- `tests/auth_test.go`: User registration, login verification, duplicate checks, session token validation and expiration.
- `tests/category_test.go`: 1-4 level category tree creation, breadcrumb path computation, and rejection of 5th level nesting.
- `tests/file_test.go`: Multipart upload streaming, SHA-256 calculation, MP3 detection, full download, HTTP range streaming (`206 Partial Content`), and disk deletion.
- `tests/snippet_test.go`: Snippet creation, 10,000 rune validation boundary checks, viewing, and deletion.
- `tests/share_test.go`: 32-byte token generation, unauthenticated public view/download, expiration validation, and owner revocation.
- `tests/fuzzy_test.go`: Subsequence, prefix, Levenshtein typo tolerance, tag score bonuses, and store ranking.
- `tests/csrf_test.go`: CSRF cookie issuance, valid token pass-through, and 403 Forbidden rejection on missing/mismatched tokens.

---

## 🐧 Linux Installation & Production Deployment Guide

Because the application is written with pure-Go SQLite (`modernc.org/sqlite`) and embeds all templates/static assets via `embed.FS`, the entire application compiles into a single, self-contained binary with **zero external runtime dependencies**.

### ⚡ Method 1: Self-Contained Node.js Installer & Menu (`install.js`)

A zero-npm-dependency Node.js installer and management suite is included at the root of the repository (`install.js`). It provides a menu-driven terminal interface, step-by-step wizard, express presets, Plesk reverse-proxy configuration, pre-flight diagnostics, live backups, and service management using purely native Node.js core modules:

```bash
# 1. Launch Interactive Menu & Wizard
sudo node install.js

# 2. Or run directly with flags:
sudo node install.js --wizard       # Step-by-step guided questionnaire
sudo node install.js --express      # 1-Click install with standard production defaults
sudo node install.js --plesk        # 1-Click install configured for Plesk (127.0.0.1:8080 HTTPS)
node install.js --diagnostics       # Pre-flight check (OS, RAM, disk space, ports, Go, systemd)
sudo node install.js --status       # View live systemd status & journalctl logs
sudo node install.js --backup       # Take compressed live archive of DB and storage
sudo node install.js --uninstall    # Clean uninstallation

# 3. Custom parameters & dry-run:
sudo node install.js --port 9000 --host 127.0.0.1 --max-size-mb 200 --secure-cookie true
node install.js --dry-run --express # Preview installation steps without modifying system
```

---

### 🐧 Method 2: Shell Installer & Menu (`install.sh`)

Alternatively, an all-in-one Bash script is provided:

```bash
# Launch Interactive Menu & Wizard
sudo ./install.sh

# Presets
sudo ./install.sh --wizard     # Guided questionnaire
sudo ./install.sh --express    # 1-Click standard install
sudo ./install.sh --plesk      # Plesk reverse-proxy preset
sudo ./install.sh --backup     # Backup DB and files
sudo ./install.sh --uninstall  # Clean uninstallation
```

---

### 🛠️ Method 3: Manual Step-by-Step Installation

#### Step 1: Building the Binary for Linux

#### Option A: Build on Linux Directly
```bash
# Ubuntu / Debian
sudo apt update && sudo apt install -y golang git

# Clone and build
git clone <your-repo-url> /tmp/filemanager-src
cd /tmp/filemanager-src
CGO_ENABLED=0 go build -ldflags="-s -w" -o filemanager cmd/server/main.go
```

#### Option B: Cross-Compile from macOS / Windows
You can cross-compile on your development machine without installing any Linux toolchains:

```bash
# Build for 64-bit Linux (x86_64 / amd64)
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o filemanager-linux-amd64 cmd/server/main.go

# Or build for ARM64 Linux (e.g. Raspberry Pi, AWS Graviton)
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o filemanager-linux-arm64 cmd/server/main.go

# Transfer binary to your Linux server
scp filemanager-linux-amd64 user@your-server-ip:/tmp/filemanager
```

---

### Step 2: System Setup & Directory Structure

Run these commands on your Linux server to create a dedicated system user and storage locations:

```bash
# 1. Create a dedicated non-login system user
sudo useradd -r -s /usr/sbin/nologin -d /var/lib/filemanager filemanager

# 2. Place binary in /usr/local/bin
sudo mv /tmp/filemanager /usr/local/bin/filemanager
sudo chmod +x /usr/local/bin/filemanager

# 3. Create persistent directories for SQLite database and uploaded files
sudo mkdir -p /var/lib/filemanager/data
sudo mkdir -p /var/lib/filemanager/storage
sudo mkdir -p /etc/filemanager

# 4. Set ownership and secure permissions (0750)
sudo chown -R filemanager:filemanager /var/lib/filemanager
sudo chmod -R 750 /var/lib/filemanager
```

---

### Step 3: Configure Environment Variables

Create `/etc/filemanager/filemanager.env`:

```bash
sudo tee /etc/filemanager/filemanager.env > /dev/null << 'EOF'
PORT=8080
HOST=127.0.0.1
DATABASE_PATH=/var/lib/filemanager/data/filemanager.db
STORAGE_DIR=/var/lib/filemanager/storage
MAX_FILE_SIZE=104857600
SECURE_COOKIE=true
EOF

sudo chmod 600 /etc/filemanager/filemanager.env
sudo chown filemanager:filemanager /etc/filemanager/filemanager.env
```

---

### Step 4: Create systemd Service

Create `/etc/systemd/system/filemanager.service`:

```bash
sudo tee /etc/systemd/system/filemanager.service > /dev/null << 'EOF'
[Unit]
Description=Go File Manager Web Service
After=network.target

[Service]
Type=simple
User=filemanager
Group=filemanager
WorkingDirectory=/var/lib/filemanager
EnvironmentFile=/etc/filemanager/filemanager.env
ExecStart=/usr/local/bin/filemanager
Restart=always
RestartSec=5s

# Security Hardening
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/filemanager

[Install]
WantedBy=multi-user.target
EOF
```

Enable and start the service:

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now filemanager

# Check status and logs
sudo systemctl status filemanager
sudo journalctl -u filemanager -f
```

---

### Step 5: Configure Reverse Proxy (HTTPS / SSL)

#### Option A: Caddy (Easiest - Automatic SSL)
Install Caddy and add to `/etc/caddy/Caddyfile`:

```caddy
files.yourdomain.com {
    reverse_proxy 127.0.0.1:8080
}
```

Reload Caddy:
```bash
sudo systemctl reload caddy
```

#### Option B: Nginx + Certbot
Create `/etc/nginx/sites-available/filemanager`:

```nginx
server {
    server_name files.yourdomain.com;

    # Allow large file uploads (100MB)
    client_max_body_size 100M;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # Stream uploads directly to backend without disk buffering in Nginx
        proxy_request_buffering off;
        proxy_buffering off;
    }
}
```

Enable site and acquire free SSL certificate:
```bash
sudo ln -s /etc/nginx/sites-available/filemanager /etc/nginx/sites-enabled/
sudo nginx -t && sudo systemctl reload nginx
sudo certbot --nginx -d files.yourdomain.com
```

#### Option C: Plesk Control Panel (Hosting Servers)

If **Plesk** is already installed on your server:

1. **Domain Creation**: In Plesk, add a Subdomain (e.g. `files.yourdomain.com`) or Domain.
2. **SSL / TLS**: Issue a free Let's Encrypt certificate via the Plesk **SSL/TLS Certificates** icon.
3. **Configure Reverse Proxy in Plesk**:
   - Go to **Domains** > `files.yourdomain.com` > **Apache & nginx Settings**.
   - Scroll to **Additional nginx directives** and paste:
     ```nginx
     location / {
         proxy_pass http://127.0.0.1:8080;
         proxy_http_version 1.1;
         proxy_set_header Host $host;
         proxy_set_header X-Real-IP $remote_addr;
         proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
         proxy_set_header X-Forwarded-Proto $scheme;

         proxy_request_buffering off;
         proxy_buffering off;
         client_max_body_size 100M;
     }
     ```
   - Uncheck **Proxy mode** (or set smart static file processing to off if you want all requests routed to Go).
   - Click **Apply**.
4. **Web Application Firewall (WAF / ModSecurity)**:
   - If users upload code snippets with SQL or script tags, ModSecurity may flag false-positives.
   - Go to **Web Application Firewall (ModSecurity)** for `files.yourdomain.com` and set it to **Detection only** or **Off** if 403 errors occur.

---

### Step 6: Firewall Setup

```bash
# Ubuntu / Debian (UFW)
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
sudo ufw enable
```

---

## 🔍 Self-Critique & Scaling Beyond 10 Users

### Assumptions & Current Limitations
1. **Single Node Storage**: Disk files are stored on the local filesystem. Suitable for a single server or mounted volume (EFS / NFS).
2. **SQLite Concurrency**: SQLite with WAL mode supports unlimited concurrent readers and 1 serialized writer. Perfectly suited for ~10–100 active users.
3. **In-Memory Fuzzy Ranking**: Scoring is performed after fetching candidate items for the user, which takes <2ms for hundreds of items per user.

### Three Key Architectural Improvements for Scaling to 10,000+ Users:
1. **Object Storage (S3 / GCS)**: Replace the local disk storage interface with an S3-compatible blob store (`minio-go` / AWS SDK) and generate pre-signed upload/download URLs to offload file traffic from application servers.
2. **Dedicated Database (PostgreSQL / SQLite with Litestream / Turso)**: Migrate metadata and sessions to PostgreSQL with connection pooling (e.g. `pgx`) or replicate SQLite across nodes with Litestream.
3. **Full-Text & Vector Search Engine (Meilisearch / SQLite FTS5)**: Implement SQLite FTS5 (Full-Text Search) with BM25 ranking or a dedicated Meilisearch/Elasticsearch instance for sub-millisecond search across millions of documents.
