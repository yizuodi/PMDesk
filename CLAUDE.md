# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Development commands

The module declares Go 1.26.5 in `go.mod`; use that toolchain. The router relies on Go 1.22 method-aware `net/http` patterns. SQLite is pure Go (`modernc.org/sqlite`), so keep builds CGO-free.

```bash
# Download modules
go mod download

# Run locally without opening a browser; use explicit paths to keep runtime data local
go run . -addr 127.0.0.1:8765 -no-browser -db ./pmjira.db -config ./jira_config.json

# Run all tests
go test ./...

# Run one package or one test
go test ./internal/web
go test ./internal/web -run '^TestMarkdownHTML$'

# Format changed Go files, then list any remaining formatting drift
gofmt -w path/to/changed.go
gofmt -l .

# Standard static checks
go vet ./...

# Build native console plus Windows console/GUI binaries
./build.sh

# Quick native build only
CGO_ENABLED=0 go build .
```

There is no separate frontend build, asset generation, or migration command. Templates, CSS, JavaScript, and SQL migrations are embedded and take effect after rebuilding/restarting. Tests cover store migrations/CRUD/order behavior and web Markdown, auth, route, and attachment paths.

`build.sh` produces `pmjira`, `pmjira.exe`, and `pmjira-gui.exe`. The GUI build sets `main.guiBuild=true`; that mode redirects logs to `pmjira.log` beside the database and is stopped through the web UI.

## Architecture

PM Jira is a local, single-process server-rendered application rather than a SPA or split frontend/backend:

1. `main.go` resolves database/config paths, opens the store, constructs the web server, binds the listener, optionally launches a browser, and handles signal or `/shutdown` termination.
2. `internal/web/server.go` owns the `http.ServeMux`, applies recovery/logging/auth middleware, and registers all method-aware routes.
3. Feature handlers in `internal/web/handlers_*.go` parse forms and path values, call the store, then render a page/HTMX fragment or redirect.
4. `internal/store/*.go` contains feature-specific SQL over one shared SQLite connection.
5. `internal/web/templates.go` renders server-side `html/template` pages and fragments from embedded assets. HTMX and `static/app.js` provide progressive enhancement for comments, board moves, previews, and attachment updates.

### Persistence and runtime files

- `internal/store/store.go` opens SQLite in WAL mode with foreign keys and a busy timeout. `SetMaxOpenConns(1)` deliberately serializes writes for this personal/local tool.
- Schema changes are numbered SQL files in `internal/store/migrations/` and are embedded/applied in filename order at startup. Add a new migration; do not rewrite an already-applied migration. Each migration and its schema-version record run in one transaction on a pinned connection; foreign keys are temporarily disabled only for controlled table rebuilds and checked before commit.
- The database stores attachment metadata, while file bytes live under `attachments/` next to the database. Changes to issue deletion or storage paths must preserve both DB cascade behavior and filesystem cleanup.
- Default runtime files live beside the executable (`pmjira.db`, `jira_config.json`, `attachments/`, and GUI `pmjira.log`). During `go run`, pass explicit `-db` and `-config` paths when location matters because defaults are based on the temporary executable.

### Rendering and feature boundaries

- Shared layout and partials are parsed once, then cloned per page. Every page template must define `page` and be registered in the `pageFiles` map in `internal/web/templates.go`; otherwise it cannot be rendered.
- `RenderPage` returns full layout-wrapped HTML. `RenderFragment` returns named partials for HTMX requests.
- Markdown is stored as source, rendered server-side with Goldmark, and sanitized with Bluemonday. Keep saved-content rendering and `/markdown/preview` on the shared renderer so previews match persisted output.
- Dashboard aggregation belongs in `internal/store/dashboard.go`; issues/board, versions, comments/activity, attachments, navigation, and people each have corresponding store and handler modules.
- Version names are parsed into structured R/C/B/SPC/CP/KEY fields in the store layer. Versions are maintained independently from personal issues; their pages focus on dates, status, structured names, and Markdown说明.

### Authentication boundary

`jira_config.json` controls an optional administrator password. With no password, all requests are effectively admin. With a password, pages are generally anonymously readable, non-GET requests and edit/settings pages require admin, and the people module is login-only. Login state is a stateless HMAC cookie derived from the current password, so changing the password invalidates existing sessions. This is a personal local tool with anonymous-read/admin-write access only; it is not designed for public exposure or multi-user authorization.
