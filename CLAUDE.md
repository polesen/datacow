# Datacow — Claude Context

Read this before doing any work. It contains all architectural decisions made so far.

## Global Preferences

@/etc/claude/CLAUDE.md

## Local Context

If `CONTEXT.local.md` exists in the repo root, read it. It contains context that is not committed to git.

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

## MCP Servers

Two MCP servers are available inside the devcontainer. Use them proactively — don't wait to be asked.

### postgres
Direct access to the test Postgres database. Use it to:
- Verify schema state before writing introspection code
- Check that queries return the expected rows
- Inspect actual data when a test fails
- Confirm migrations or table setup worked correctly

Use this whenever you are writing or debugging anything that touches the database.

### context7
Fetches live, version-accurate documentation for any library. Use it to:
- Look up current Bubble Tea / Lip Gloss APIs before writing TUI code
- Check pgx v5 query patterns before writing DB code
- Verify Cobra flag/completion APIs

Use this whenever you are about to use a third-party library — always prefer current docs over training data.

## Go Tooling

The following tools are installed and should be used actively:

| Tool | When to use |
|---|---|
| `gopls` | Available as LSP — provides go-to-definition, type info, inline errors. Used automatically. |
| `staticcheck` | Run after lint for deeper static analysis: `staticcheck ./...` |
| `dlv` | Debug unexpected behaviour instead of adding print statements: `dlv test ./path/to/pkg` |
| `gotestsum` | Always use instead of `go test`: `gotestsum --format testdox ./...` |
| `gofumpt` | Runs automatically via hook after every Go file edit. |
| `gomodifytags` | Add or edit struct tags: `gomodifytags -file foo.go -struct Foo -add-tags json` |
| `psql` | Connect to Postgres directly: `psql $TEST_POSTGRES_DSN` |
| `mysql` | Connect to MySQL directly: `mysql -h mysql -u datacow -pdatacow datacow_test` |
| `pg_isready` | Check Postgres connectivity: `pg_isready -h postgres -U datacow` |
| `mysqladmin` | Check MySQL connectivity: `mysqladmin ping -h mysql -u datacow -pdatacow` |

## SQL Security

SQL injection is the most critical security concern in this codebase. Apply these rules everywhere, without exception:

- **Always use parameterized queries or prepared statements** — never interpolate user input, column names, table names, filter values, or AI-generated content directly into SQL strings
- This applies to:
  - Internal datacow queries (schema introspection, dataset execution, pagination)
  - User-supplied filters and sort parameters passed to `QueryOptions`
  - AI-generated SQL queries used as dataset definitions
  - Any dynamic SQL constructed at runtime
- Column names and table names cannot be parameterized in SQL — if they must be dynamic, validate them strictly against a whitelist of known schema names before use
- Prefer `database/sql` parameterized queries (`$1`, `?`) over any form of string building
- When reviewing or generating code that touches SQL, actively look for injection vectors and flag or fix them before finishing

## Before Starting Any Task

Run the preflight check first. If it fails, stop immediately — do not proceed:

```bash
make preflight
```

This verifies all required tools and database connections are available. A failed preflight means the devcontainer needs rebuilding, not that the code is broken.

## TDD

Follow TDD for all new functionality across all layers — core, TUI, and API/web.

The cycle:
1. **Write the test first** — it should fail (red). A compile error because types don't exist yet is fine; stub out just enough to compile, then confirm the test fails before writing implementation.
2. **Write the minimum implementation** to make it pass (green).
3. **Refactor** if needed, keeping tests green.

Never write implementation before a failing test exists.

**Testing by layer:**
- **`core/`** — table-driven tests with `gotestsum`, real databases via `TEST_POSTGRES_DSN` / `TEST_MYSQL_DSN`
- **`tui/`** — use `github.com/charmbracelet/x/exp/teatest` for headless TUI rendering tests
- **`api/`** — use `net/http/httptest` for handler tests against real core logic
- **`web/`** — API integration tests via HTTP; frontend component tests if complexity warrants it

## Branching

Each task runs on its own feature branch (`task/M3-dataset-layer` etc.), created by `run-task.sh` before Claude starts. Do not create or switch branches. Commit your work to whatever branch is current when you start.

Never commit directly to `main` or push to `main`. If the current branch is `main`, create a feature branch before committing.

## Definition of Done

See `TASKS/definition-of-done.md`. Invoke the `/done` skill — it runs all checks in sequence and stops on the first failure.

## Persistent Memory

The auto-memory system (`~/.claude/projects/...`) is Mac-local. It is not accessible from devcontainers or the GitHub app. Do not use it for project-level knowledge.

When you learn something that should persist across sessions and environments:
- **Project conventions, lessons, architectural decisions** → update this file (`CLAUDE.md`) directly and commit it
- **Local-only config or secrets** → `CONTEXT.local.md` (gitignored, never commit)

## Conventions

- **No business logic in TUI or API layers.** If you're writing a DB query in a view, move it to core.
- **Interfaces first.** Define `db.Client` interface before implementing postgres/mysql.
- **Table-driven tests** for core logic. No mocks for DB — use test containers or real DBs.
- **No global state.** Pass dependencies explicitly (factory pattern, like gh CLI).
- **Keybindings configurable** — don't hardcode key handlers, route through a keybindings registry.
- **New TUI features get a keybinding and a help entry.** When adding any user-visible action: add a `key.Binding` to `keys.Map`, wire it through `Update()`, and add it to the relevant group in `views/helpoverlay.go`. Do not add actions that are only reachable by reading the source code.
- **Tests own their fixtures.** Create tables in test setup via `client.Query(ctx, "CREATE TABLE IF NOT EXISTS ...")` and drop them in `t.Cleanup`. Never seed via CI workflow steps or rely on tables left by other test packages. See `internal/core/db/postgres_test.go` for the pattern.

## Current Status

@tasks/README.md

Tasks are in `tasks/`. See `tasks/CONTEXT.md` for the task lifecycle.

