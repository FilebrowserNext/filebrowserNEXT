# Contributing to File Browser Next

File Browser Next is an actively maintained project. Pull requests, bug reports, and feature proposals are welcome.

## Project Structure

The backend is written in [Go](https://golang.org/) and the frontend (in the `frontend/` subdirectory) is written in [Vue.js](https://vuejs.org/). Some features are tightly coupled between the two layers, so a basic understanding of both is recommended.

- Learn Go: [https://github.com/golang/go/wiki/Learn](https://github.com/golang/go/wiki/Learn)
- Learn Vue.js: [https://vuejs.org/guide/introduction.html](https://vuejs.org/guide/introduction.html)

Clone the repository:

```bash
git clone https://github.com/FilebrowserNext/filebrowserNEXT
```

## Build

Build the complete project (frontend + backend) in two steps:

```bash
# 1. Build the frontend assets
cd frontend
pnpm install
pnpm run build
cd ..

# 2. Compile the Go binary (embeds the built frontend)
go build -o filebrowser .
```

## Development

### Frontend

Requirements: Node.js >= 24.0.0, pnpm >= 10.0.0.

```bash
cd frontend

# Install dependencies
pnpm install

# Watch mode — serves the UI with hot reload
pnpm run dev
```

When using `pnpm run dev`, access the interface through the Vite development server URL, not through the Go binary directly.

To produce a static build of the frontend (required before building the Go binary):

```bash
pnpm run build
```

### Backend

```bash
# Download Go module dependencies
go mod download

# Build
go build -o filebrowser .

# Run directly
go run . -r /path/to/your/files
```

## Documentation

Documentation lives in [`docs/`](../docs/) and is built into a static site with [MkDocs](https://www.mkdocs.org/) and published to GitHub Pages.

To preview the documentation locally:

```bash
pip install mkdocs-material
mkdocs serve
```

The CLI reference pages in [`docs/cli/`](cli/) are generated from the commands themselves. After changing any CLI command, regenerate them:

```bash
go run . cmds generate-docs docs/cli/
```

## Translations

Locale files live in [`frontend/src/i18n/`](https://github.com/FilebrowserNext/filebrowserNEXT/tree/main/frontend/src/i18n) and can be edited directly. To add a new language, copy an existing locale file and translate the strings.

## Release

Releases are created via GitHub Actions. To trigger a release, push a Git tag:

```bash
git tag v3.x.x
git push origin v3.x.x
```

The CI workflow builds binaries for all supported platforms (Linux amd64/arm64, macOS amd64/arm64, Windows amd64), packages them as archives, and publishes them to the GitHub release.

## Authentication Provider

To build a custom authentication provider, implement the `Auther` interface defined in [`auth/auth.go`](https://github.com/FilebrowserNext/filebrowserNEXT/blob/main/auth/auth.go):

```go
// Auther is the authentication interface.
type Auther interface {
    // Auth is called to authenticate a request.
    Auth(r *http.Request, usr users.Store, stg *settings.Settings, srv *settings.Server) (*users.User, error)
    // LoginPage indicates if this auther needs a login page.
    LoginPage() bool
}
```

After implementing the interface:

1. Add it to the [`auth/`](https://github.com/FilebrowserNext/filebrowserNEXT/blob/main/auth) directory.
2. Register it in the [configuration parser](https://github.com/FilebrowserNext/filebrowserNEXT/blob/main/cmd/config.go) (`addConfigFlags`).
3. Register it in the auth storage backend.

## Code of Conduct

By participating in this project you agree to abide by the [Code of Conduct](https://github.com/FilebrowserNext/filebrowserNEXT/blob/main/CODE_OF_CONDUCT.md).

