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
3. **Single-Click MP3 Audio Player**:
   - Inline playback button and sticky bottom player bar.
   - Native HTTP byte-range request streaming (`206 Partial Content`, `Accept-Ranges: bytes`) allows instant seek, scrub, and playback in all modern browsers.
4. **Hierarchical Folders / Collections (Up to 4 Levels Deep)**:
   - Full nested directory organization: `Root (Level 1) ➔ Folder (Level 2) ➔ Sub-folder (Level 3) ➔ Deepest (Level 4)`.
   - Strict server-side depth limit enforcement (5th level insertion is rejected).
   - Breadcrumb navigation paths (e.g. `Work / 2026 / Audio / Podcast`).
5. **Text Snippets (Up to 10,000 Runes)**:
   - Fast plain-text / code paste with syntax highlighting indicators.
   - Live character counter.
   - One-click copy to clipboard.
6. **Tags & Fuzzy Search**:
   - Tagging on all files and snippets with aggregated tag pills.
   - Multi-stage fuzzy search algorithm incorporating exact match, prefix matching, word boundary heuristics, subsequence matching, and Levenshtein typo tolerance.
7. **Shareable Public Links**:
   - 32-byte cryptographically secure random tokens (`base64url`).
   - Configurable expiration (1 hour, 24 hours, 7 days, 30 days, or permanent).
   - Instant owner revocation.
   - Public view, download, and audio playback without authentication.
   - Automatic access view counter.
8. **Security Hardening**:
   - Double-submit CSRF cookie protection on all state-changing endpoints.
   - Context-aware XSS escaping with Go's `html/template`.
   - Strict HTTP security headers (`Content-Security-Policy`, `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, `Referrer-Policy`).
9. **Zero External Runtime Dependencies**:
   - HTML templates, CSS, and JS are embedded directly into the binary via `embed.FS`.
   - Pure Go SQLite database engine (`modernc.org/sqlite`) requires no C compiler (CGO-free).
10. **Footer Versioning**:
    - Displays version and update timestamp (`v1.0.0 (updated 2026-08-24 01:45)`).

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

## 🔍 Self-Critique & Scaling Beyond 10 Users

### Assumptions & Current Limitations
1. **Single Node Storage**: Disk files are stored on the local filesystem. Suitable for a single server or mounted volume (EFS / NFS).
2. **SQLite Concurrency**: SQLite with WAL mode supports unlimited concurrent readers and 1 serialized writer. Perfectly suited for ~10–100 active users.
3. **In-Memory Fuzzy Ranking**: Scoring is performed after fetching candidate items for the user, which takes <2ms for hundreds of items per user.

### Three Key Architectural Improvements for Scaling to 10,000+ Users:
1. **Object Storage (S3 / GCS)**: Replace the local disk storage interface with an S3-compatible blob store (`minio-go` / AWS SDK) and generate pre-signed upload/download URLs to offload file traffic from application servers.
2. **Dedicated Database (PostgreSQL / SQLite with Litestream / Turso)**: Migrate metadata and sessions to PostgreSQL with connection pooling (e.g. `pgx`) or replicate SQLite across nodes with Litestream.
3. **Full-Text & Vector Search Engine (Meilisearch / SQLite FTS5)**: Implement SQLite FTS5 (Full-Text Search) with BM25 ranking or a dedicated Meilisearch/Elasticsearch instance for sub-millisecond search across millions of documents.
