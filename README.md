<p align="center">
  <img src="./frontend/public/img/logo.svg" width="120" height="120" alt="File Browser Next Logo"/>
</p>

<h1 align="center">File Browser Next</h1>

<p align="center">
  <b>A modern, secure, next-generation web-based file manager.</b>
</p>

<p align="center">
  <a href="https://filebrowsernext.github.io/filebrowserNEXT/"><img src="https://img.shields.io/badge/Documentation-GitHub_Pages-blue" alt="Documentation"/></a>
  <a href="#security-resolutions"><img src="https://img.shields.io/badge/Security-Hardened-success" alt="Security Hardened"/></a>
  <a href="#modern-ui--experience"><img src="https://img.shields.io/badge/UI-Modernized-indigo" alt="UI Modernized"/></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-Apache_2.0-blue.svg" alt="License"/></a>
</p>

---

**File Browser Next** is an actively maintained, modernized continuation of the original File Browser project. It provides a sleek file managing interface within a specified directory, allowing you to upload, delete, preview, and edit your files from any browser on desktop or mobile.

**Official Documentation & Live Showcase:** [filebrowsernext.github.io/filebrowserNEXT](https://filebrowsernext.github.io/filebrowserNEXT/)

---

## Security Resolutions

File Browser Next specifically resolves the critical security architecture issues previously documented in upstream File Browser:

### 1. Session & JWT Revocation ([#5216](https://github.com/filebrowser/filebrowser/issues/5216) — Resolved)
- **Persistent Server-Side Revocation Store**: Implemented a dedicated BoltDB-backed revocation store with in-memory fast-lookup caching and automatic expired token cleanup.
- **Dedicated Logout Endpoint (`POST /api/logout`)**: Calling logout instantly revokes the JWT server-side via its cryptographic `jti` and clears client authentication cookies.
- **Instant Invalidation on Password or Account Updates**: Every user account tracks an `UpdatedAt` timestamp. Any password change or permission change immediately invalidates all existing JWTs issued prior to that timestamp.
- **Single-Use Token Renewal**: When renewing an active session token, the previous token (`jti`) is immediately revoked, preventing token replay attacks.
- **Strict Error Handling**: Tokens referencing deleted users or invalid states now properly return `401 Unauthorized` instead of internal server errors.

### 2. Command Runner & Execution Hardening ([#5199](https://github.com/filebrowser/filebrowser/issues/5199) — Resolved)
- **Strict Working Directory Confinement**: Execution working directories are strictly confined within the user's isolated scope using canonical path traversal checks (`filepath.Rel`). Commands can never escape their designated filesystem boundary.
- **Shell Metacharacter Sanitization**: Non-admin executions strictly prohibit shell injection primitives (`;`, `&`, `|`, `` ` ``, `$`, `\n`, `>`, `<`) via `runner.HasDangerousShellMetachars()`.
- **Safe Direct Binary Execution**: Non-admin commands execute binaries directly without passing strings through an unconstrained shell interpreter.

---

## Modern UI & Experience

File Browser Next features a complete visual redesign:
- **Modern Design System**: Refreshed color tokens featuring an elegant indigo/slate theme, soft multi-layer box shadows, and 8px/12px/16px rounded borders.
- **Glassmorphic Elements**: Modern blur backdrops (`backdrop-filter: blur(16px)`) on top navigation bars, action bars, modals, and file selection docks.
- **Redesigned Login Page**: Card layout with ambient mesh gradients, improved form controls, and responsive styling.
- **New Logo & Identity**: A sleek vector emblem with vibrant indigo-to-cyan gradients.
- **Responsive File Views**: Polished grid cards, clean list rows, smooth hover elevations, and modern multi-selection pill docks.

---

## Getting Started

### Quick Start with Prebuilt Binary / Go

```bash
# Build the backend binary
go build -o filebrowser .

# Start the server with a quick setup
./filebrowser -r /path/to/your/files
```

Access the interface in your browser at `http://127.0.0.1:8080` (default credentials: `admin` / `admin`).

### Building from Source

**Requirements:**
- Go 1.23+
- Node.js 20+ & pnpm

```bash
# 1. Build the Frontend
cd frontend
pnpm install
pnpm run build
cd ..

# 2. Build the Go Binary (embeds frontend)
go build -o filebrowser .
```

---

## Documentation

- General documentation on installation and configuration can be found in the [`docs`](docs) directory.
- For development guidelines, see [CONTRIBUTING.md](CONTRIBUTING.md).

---

## License

[Apache License 2.0](LICENSE) © File Browser Next Contributors & File Browser Authors.
