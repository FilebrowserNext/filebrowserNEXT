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
- **Command Confinement & Sanitization**: Strict filesystem directory confinement and detection/filtering of dangerous shell metacharacters.

## Reporting a Vulnerability

If you discover a security vulnerability in File Browser Next, please report it privately via GitHub Security Advisories or by contacting the maintainers directly.

Please include:
- Description of the issue
- A plaintext proof of concept (no compiled binaries)
- Steps to reproduce
- Recommended remediation, if any

