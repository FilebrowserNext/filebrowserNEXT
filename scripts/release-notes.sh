#!/usr/bin/env bash
# Prints the GitHub release notes for a tag: the matching CHANGELOG.md
# section, a comparison link with the previous release, and the download
# table. Usage: scripts/release-notes.sh v3.0.2-next [CHANGELOG.md]
set -euo pipefail

tag="${1:?usage: release-notes.sh <tag> [changelog]}"
changelog="${2:-CHANGELOG.md}"
version="${tag#v}"
repository="${GITHUB_REPOSITORY:-FilebrowserNext/filebrowserNEXT}"

section="$(awk -v heading="## File Browser Next ${version} " '
  index($0, heading) == 1 { found = 1; next }
  found && /^## / { exit }
  found { print }
' "${changelog}")"

if [[ -z "${section//[[:space:]]/}" ]]; then
  echo "release-notes: no section for ${version} in ${changelog}" >&2
  exit 1
fi

# Previous release: the tag sorted right below this one.
previous="$(git tag --list 'v*' --sort=-version:refname |
  awk -v current="${tag}" 'found { print; exit } $0 == current { found = 1 }')"

printf '%s\n' "${section}" | sed -e '/./,$!d'
printf '\n'
if [[ -n "${previous}" ]]; then
  printf '**Full Changelog**: https://github.com/%s/compare/%s...%s\n' "${repository}" "${previous}" "${tag}"
fi
cat <<'EOF'

---

### Downloads & Checksums

| OS / Architecture | Archive |
|---|---|
| Linux x86_64 | `filebrowser-linux-amd64.tar.gz` |
| Linux ARM64 / Raspberry Pi | `filebrowser-linux-arm64.tar.gz` |
| macOS Intel | `filebrowser-darwin-amd64.tar.gz` |
| macOS Apple Silicon (M1/M2/M3/M4) | `filebrowser-darwin-arm64.tar.gz` |
| Windows 64-bit | `filebrowser-windows-amd64.zip` |

Verify archive integrity using `checksums.txt` (SHA256).

Access the web UI at `http://127.0.0.1:8080` (or `http://<your-server-ip>:8080`) — default credentials: `admin` / `admin`.
EOF
