# Future: Schema Cache + Search / Goto

## Motivation

The schema explorer (see `schema-explorer.md`) loads indexes lazily per table on demand.
That is fine for browsing one table at a time. But the following features require the
full schema to be in memory for all tables at once:

- **Schema search**: fuzzy-find any table, view, column, or index by name — like `Ctrl+P` in an editor
- **Goto**: jump directly to a table or column from anywhere in the TUI
- **Completion**: suggest column names when typing a filter or SQL query

These features need a **schema cache** — a background-loaded, full snapshot of all schema
objects — plus a **refresh mechanism** so the cache stays consistent after DDL changes.

## Scope (when this is built)

**Schema cache in core:**

- `schema.Cache` holds the full schema for a datasource: all tables, views, columns, indexes, FKs
- Loaded eagerly in a background goroutine on startup (or on first access)
- Exposes a `Refresh(ctx)` method that reloads all metadata
- Sends a `CacheReadyMsg` / `CacheRefreshedMsg` to the TUI via a channel or tea.Cmd
- Thread-safe reads (schema is immutable once loaded; swap atomically on refresh)

**Refresh UX:**

- Keybinding (e.g. `R`) triggers refresh — spinner in status bar while loading
- After DDL changes (detected or user-triggered), cache is invalidated and reloaded

**Search / Goto UX:**

- `Ctrl+P` or `/` opens a fuzzy-search overlay across all schema object names
- Results show type icon: table / view / column (table.column) / index
- `Enter` navigates to the selected object: opens the table in the row browser,
  or expands the tree to the selected column

## Dependencies

- Requires schema-explorer.md to be complete first (kind distinctions, index metadata)
- Requires a fuzzy-match library or simple prefix/substring matching in core

## Notes

- The cache should live in `internal/core/schema/cache.go`, not in the TUI
- The TUI holds a reference to the cache and subscribes to updates
- Consider whether the HTTP API (future/http-api.md) should expose the schema cache
  as a `/schema` endpoint — probably yes
