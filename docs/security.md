# Security Policy

## Supported Versions

File Browser Next is actively maintained and receives regular security updates.

| Version | Supported |
| ------- | --------- |
| Next (main) | Yes       |
| Legacy 2.x  | No (upstream archived) |

## Security Hardening in File Browser Next

File Browser Next incorporates native defenses against the vulnerabilities identified in legacy versions:
- **Server-Side Token Revocation**: Dedicated token revocation backend ensuring immediate invalidation on logout, password changes, and renewal replay attacks.
- **Targeted Session Invalidation**: Only security-sensitive updates (password, permissions, username, scope, rules) invalidate active sessions. Preference-only updates (language, theme, view mode) no longer silently log users out.
- **Case-Insensitive Authentication**: Username lookup at login tolerates any casing, preventing frustrating 403 errors from trivial capitalization differences.
- **Brute Force Protection**: `POST /api/login` and share password endpoints are rate-limited to 10 attempts per IP per 5-minute window, returning `HTTP 429` when exceeded. Normal users are never affected.
- **Reverse-Proxy-Aware IP Resolution**: Proxy forwarding headers (`CF-Connecting-IP`, `X-Real-IP`, `X-Forwarded-For`) are trusted only when the direct TCP connection comes from a private or loopback address — i.e. from a local reverse proxy such as Cloudflare, Caddy, Nginx, or Pangolin. When the app is accessed directly from the internet, these headers are ignored entirely so that a public attacker cannot forge a fake IP address to circumvent rate limiting.
- **Hardened HTTP Headers**: Every response includes `X-Frame-Options: DENY`, `X-Content-Type-Options: nosniff`, `Referrer-Policy: strict-origin-when-cross-origin`, and `Permissions-Policy` disabling camera, microphone, and geolocation.
- **Command Confinement & Sanitization**: Strict filesystem directory confinement and detection/filtering of dangerous shell metacharacters.

## Reporting a Vulnerability

If you discover a security vulnerability in File Browser Next, please report it privately via GitHub Security Advisories or by contacting the maintainers directly.

Please include:
- Description of the issue
- A plaintext proof of concept (no compiled binaries)
- Steps to reproduce
- Recommended remediation, if any

