# Datacow — Claude Context

Read this before doing any work. It contains all architectural decisions made so far.

## Global Preferences

@/etc/claude/CLAUDE.md

## Local Context

If `CONTEXT.local.md` exists in the repo root, read it. It contains competitive positioning and business model context that is not committed to git.

## What This Is

Datacow is a zero-config database explorer — "like k9s or lazygit, but for databases."

Connect with a single connection string and immediately navigate tables, drill through
foreign key relationships, filter, sort, and export — no SQL knowledge required.

Two interfaces, one binary:
- **TUI** — terminal app, works standalone, keyboard-driven, scriptable
- **Web app** — served via `datacow serve`, talks to embedded HTTP API

## Product Reference

Full product definition is in `PRODUCT.md`. Read it. Key points:

- `datasource` = a database connection (connection string)
- `dataset` = a named view — either a plain table or a saved SQL query
- Zero-config mode: `datacow --connection-string=...` auto-discovers all tables as datasets
- YAML config (`datasources.yaml`, `datasets.yaml`, `users.yaml`) is the optional persistent layer
- Read-only for now — no INSERT/UPDATE/DELETE

## Tech Stack

| Concern | Choice | Reason |
|---|---|---|
| Language | Go | Single binary, no runtime, cross-platform |
| TUI | Bubble Tea (charmbracelet/bubbletea) | Industry standard, used by k9s-style tools |
| TUI styling | Lip Gloss (charmbracelet/lipgloss) | Pairs with Bubble Tea |
| CLI/commands | Cobra | Standard Go CLI framework, completions support |
| DB abstraction | Go `database/sql` + drivers | Standard interface, pluggable drivers |
| HTTP server | Go `net/http` (stdlib) or Chi router | Lightweight, no heavy frameworks |
| Web UI | TBD — decide before building | See Web UI section below |

## Architecture

Three layers, one binary — **core library used by both TUI and HTTP server**:

```
TUI app          HTTP server (datacow serve)
    \               /
     \             /
      [Core library]       ← ALL business logic lives here
            |
       DB drivers
```

**The TUI does NOT go through the HTTP API.** It uses the core library directly.
This keeps the TUI fast, standalone, and zero-dependency.

The HTTP server uses the same core library and also serves the embedded web app.

## Project Structure

Follow the lazygit/k9s pattern — strict 3-layer separation:

```
/cmd
  └─ main.go                  # Minimal entry point, delegates immediately

/internal
  ├─ /core                    # ALL business logic — no TUI dependencies allowed here
  │   ├─ /db                  # DB connection, driver abstraction
  │   │   ├─ client.go        # Interface definition
  │   │   ├─ postgres.go
  │   │   └─ mysql.go
  │   ├─ /schema              # Schema introspection (tables, columns, FKs)
  │   ├─ /dataset             # Dataset resolution, query execution, pagination
  │   ├─ /export              # CSV, Excel export
  │   └─ /config              # YAML config loading (datasources, datasets, users)
  │
  ├─ /tui                     # TUI layer — depends on core, NOT on http
  │   ├─ app.go               # Bubble Tea application root
  │   ├─ /context             # Panel contexts (table list, row browser, etc.)
  │   ├─ /views               # View components
  │   └─ /keys                # Keybinding definitions
  │
  ├─ /api                     # HTTP server — depends on core, NOT on tui
  │   ├─ server.go
  │   └─ /handlers
  │
  └─ /web                     # Embedded web app assets (go:embed)
      └─ dist/                # Built web app output (gitignored, built at compile time)
```

## Web UI

Not decided yet. Requirements:
- Fast iteration cycle: edit → save → see change in browser (hot reload)
- Embedded in Go binary via `go:embed` for production
- During development, served from disk (env var or build tag to switch)

Candidates: Svelte/SvelteKit (lightweight, fast HMR), React + Vite.
**Decide and document here before building the web UI milestone.**

## Database Drivers

Must support (v1): PostgreSQL, MySQL
Should support: SQLite, MSSQL, Oracle

All drivers implement the same `core/db.Client` interface. Adding a new DB = new file in `/internal/core/db/`.

## Development Setup

```bash
# Run TUI against a local database
go run ./cmd --connection-string="postgres://..."

# Run HTTP server + web app
go run ./cmd serve --connection-string="postgres://..."

# Run tests
go test ./...

# Build
go build -o datacow ./cmd
```

Test databases are available via Docker Compose (see `docker-compose.dev.yml`).

## AI Integration (planned, not v1)

Pluggable LLM provider (Anthropic, OpenAI, Ollama). Provider configured via YAML or env var.

Priority features:
1. Natural language filtering → SQL WHERE clauses
2. Natural language → dataset (generates SQL, developer reviews)
3. Data Q&A — ask questions about visible data

LLM abstraction layer lives in `/internal/core/ai/`. Not built yet — defer until core data layer is solid.

## Definition of Done

Before considering any task complete, always run these checks and ensure they all pass:

```bash
go build ./...        # must compile cleanly
go test ./...         # all tests must pass
make lint             # zero lint issues
```

Do not mark a task done or stop working if any of these fail.

## Conventions

- **No business logic in TUI or API layers.** If you're writing a DB query in a view, move it to core.
- **Interfaces first.** Define `db.Client` interface before implementing postgres/mysql.
- **Table-driven tests** for core logic. No mocks for DB — use test containers or real DBs.
- **No global state.** Pass dependencies explicitly (factory pattern, like gh CLI).
- **Keybindings configurable** — don't hardcode key handlers, route through a keybindings registry.

## Current Status

Early stage. No Go code exists yet. Tasks are in `TASKS/`.

Start with: `TASKS/M1-project-scaffold.md`
