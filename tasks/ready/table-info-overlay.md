# Table Info Overlay + Remove Expensive COUNT(*)

Two connected changes: remove the `COUNT(*)` query storm at connect, and replace it with a lazy, on-demand table info overlay that surfaces cheap DB-native statistics.

## Problem

On connect, `tablelist.go` fires a rolling queue of `COUNT(*)` queries — one per table, 5 concurrent — to populate the row count column. On a database with 100+ large tables this can run for minutes and saturates the connection pool. The user did not ask for it.

## What to build

### 1. Remove the COUNT(*) at connect

In `internal/tui/views/tablelist.go`:
- Delete `loadRowCountCmd`, the `counts` map, `nextCountIdx`, and the `RowCountMsg` handler.
- Remove the count column from the table list render. The right-hand column is gone entirely — table names get the full width back.
- Delete `RowCountMsg` from `internal/tui/views/tablelist.go` and any references in `app.go`.

No count is shown in the table list. Stats are available on demand via the info overlay (below).

### 2. New abstraction: `db.StatsProvider`

Add to `internal/core/db/client.go` — a **separate optional interface**, not a method on `Client`. Drivers that support it advertise it; those that don't are silently skipped.

```go
// StatsProvider is an optional interface drivers can implement to return
// cheap, catalog-sourced statistics for a single table. Callers must
// type-assert the Client to StatsProvider before calling.
type StatsProvider interface {
    TableStats(ctx context.Context, table string) (TableStats, error)
}

type TableStats struct {
    RowEstimate  *int64     // planner/autovacuum estimate; nil = unknown
    TotalBytes   *int64     // table + indexes + TOAST/overflow; nil = unknown
    TableBytes   *int64     // data pages only; nil = unknown
    IndexBytes   *int64     // all indexes; nil = unknown
    FreeBytes    *int64     // unused/fragmented space; nil = unknown
    Description  string     // table comment or description; "" = none
    LastAnalyzed *time.Time // nil = unknown
    LastVacuumed *time.Time // nil = unknown (PostgreSQL only)
    CreatedAt    *time.Time // nil = unknown (MySQL only)
    Engine       string     // storage engine; "" = unknown (MySQL: InnoDB/MyISAM/…)
    NextAutoIncr *int64     // next AUTO_INCREMENT value; nil = unknown (MySQL only)
}
```

Use `*int64` for all nullable numeric fields — `nil` means absent, a non-nil pointer to `0` means "confirmed empty". This is consistent with how `LastAnalyzed`, `LastVacuumed`, and `NextAutoIncr` are already typed, and is idiomatic Go. Never use `-1` as a sentinel.

### 3. PostgreSQL implementation

Add `TableStats(ctx, table string) (TableStats, error)` to `postgresClient` in `internal/core/db/postgres.go`.

Single query — no sequential round-trips:

```sql
SELECT
    c.reltuples::bigint                        AS row_estimate,
    pg_total_relation_size(c.oid)              AS total_bytes,
    pg_relation_size(c.oid)                    AS table_bytes,
    pg_indexes_size(c.oid)                     AS index_bytes,
    obj_description(c.oid, 'pg_class')         AS description,
    s.last_autovacuum,
    s.last_autoanalyze
FROM pg_class c
JOIN pg_namespace n ON n.oid = c.relnamespace
LEFT JOIN pg_stat_user_tables s ON s.relid = c.oid
WHERE n.nspname = 'public'
  AND c.relname = $1
  AND c.relkind = 'r'
```

Fields not available in PostgreSQL: `FreeBytes`, `CreatedAt`, `Engine`, `NextAutoIncr` — leave as `nil` / `""`.

Note: `reltuples` is `-1` in the catalog for a brand-new table that has never been analyzed. Map `reltuples < 0` → `RowEstimate = nil`.

### 4. MySQL implementation

Add `TableStats(ctx, table string) (TableStats, error)` to `mysqlClient` in `internal/core/db/mysql.go`.

Single query:

```sql
SELECT
    TABLE_ROWS,
    DATA_LENGTH,
    INDEX_LENGTH,
    DATA_FREE,
    TABLE_COMMENT,
    CREATE_TIME,
    UPDATE_TIME,
    ENGINE,
    AUTO_INCREMENT
FROM information_schema.TABLES
WHERE TABLE_SCHEMA = DATABASE()
  AND TABLE_NAME   = ?
```

Mapping:
- `TABLE_ROWS` → `RowEstimate` (InnoDB: approximate; MyISAM: exact)
- `DATA_LENGTH` → `TableBytes`
- `INDEX_LENGTH` → `IndexBytes`
- `DATA_LENGTH + INDEX_LENGTH` → `TotalBytes`
- `DATA_FREE` → `FreeBytes`
- `CREATE_TIME` → `CreatedAt`
- `UPDATE_TIME` → `LastAnalyzed` (best available; MySQL has no separate analyze timestamp)
- `ENGINE` → `Engine`
- `AUTO_INCREMENT` → `NextAutoIncr` (NULL for tables without one)

Fields not available in MySQL: `LastVacuumed`, `Description` uses `TABLE_COMMENT`.

### 5. `LoggingClient` passthrough

`internal/core/db/logging.go` — `LoggingClient` wraps `db.Client`. It must also implement `StatsProvider` when the inner client does, so the type assertion works through the wrapper:

```go
func (c *LoggingClient) TableStats(ctx context.Context, table string) (TableStats, error) {
    if sp, ok := c.inner.(StatsProvider); ok {
        return sp.TableStats(ctx, table)
    }
    return TableStats{}, fmt.Errorf("not supported")
}
```

Log this call via the query log with label `"info <table>"` and kind `QueryKindSystem`.

### 6. TUI: Table Info Overlay

New file: `internal/tui/views/tableinfo.go`

```go
type TableInfoModel struct {
    tableName string
    stats     *db.TableStats // nil = not yet loaded
    err       error
    loading   bool
    width     int
    height    int
    spinChar  string
}

type tableStatsLoadedMsg struct {
    stats db.TableStats
    err   error
}

func NewTableInfoModel() TableInfoModel
func (m *TableInfoModel) SetSize(w, h int)
func (m *TableInfoModel) SetSpinChar(s string)
func (m TableInfoModel) Load(client db.Client, table string) tea.Cmd  // returns tableStatsLoadedMsg
func (m TableInfoModel) Update(msg tea.Msg) (TableInfoModel, tea.Cmd)
func (m TableInfoModel) View() string
```

`Load` returns a `tea.Cmd` that type-asserts `client` to `StatsProvider`. If not supported, it immediately returns a msg with a descriptive error ("statistics not available for this database").

**Display layout** (example with PostgreSQL):

```
 Table Info — orders

 Rows (estimate)    ~1,234,567
 Total size         234 MB
   Table data       198 MB
   Indexes           36 MB

 Description        Customer orders, partitioned by month

 Last analyzed      2026-05-01 14:32
 Last vacuumed      2026-05-01 14:30

 i or esc   close
```

MySQL adds (when available):

```
 Engine             InnoDB
 Free space         12 MB
 Created            2024-11-03 09:15
 Auto-increment     9,999,124
```

If a field is unknown (`-1` / `nil` / `""`), omit that row entirely rather than showing a dash.

If `StatsProvider` is not implemented, show:
```
 Statistics not available for this database type.
```

**Size formatting** — `formatBytes(n int64) string` in `tableinfo.go`:

| Range | Format | Example |
|---|---|---|
| < 1 024 | `N B` | `512 B` |
| < 1 048 576 | `N KB` | `128 KB` |
| < 1 073 741 824 | `N.N MB` | `42.3 MB` |
| < 1 099 511 627 776 | `N.N GB` | `3.2 GB` |
| ≥ 1 099 511 627 776 | `N.N TB` | `1.8 TB` |

**Row estimate formatting** — `formatEstimate(n int64) string` (called only when `RowEstimate != nil`):
- `0` → `"0"` (confirmed empty table)
- < 1 000 → exact number, no prefix
- ≥ 1 000 → `"~N"` with K/M/G suffix: `~42K`, `~1.2M`, `~3.4B`

`formatBytes` is called only when the field is non-nil. Nil fields are omitted from the display entirely.

The `~` signals it is an estimate, not a precise count.

### 7. Wire into app.go

- Add `screenTableInfo` to the `screen` const.
- Add `tableInfoModel views.TableInfoModel` field to `App`.
- Add `i` key binding to `keys.Map` with help `"i  table info"`.
- In `Update()`: handle `i` key when `a.screen == screenSplit` AND the focused panel is the table list AND the selected dataset has `Kind == KindTable` (not a view, not a custom SQL dataset). Open: `a.screenBeforeOverlay = a.screen; a.screen = screenTableInfo`, then fire `tableInfoModel.Load(a.client, tableName)`.
- `i` on a view or custom SQL dataset: ignore (no action, no error message).
- Close on `i` or `Esc` from `screenTableInfo`.
- Propagate `WindowSizeMsg` and `spinner.TickMsg` to `tableInfoModel`.
- Render in `View()` when `a.screen == screenTableInfo`.

Each time the overlay opens, reset and reload — do not cache between opens. Schema changes between visits.

## Files to create / modify

| File | Change |
|---|---|
| `internal/core/db/client.go` | Add `StatsProvider` interface and `TableStats` struct |
| `internal/core/db/postgres.go` | Implement `TableStats` |
| `internal/core/db/mysql.go` | Implement `TableStats` |
| `internal/core/db/logging.go` | Passthrough `StatsProvider` with query log entry |
| `internal/tui/views/tablelist.go` | Remove COUNT(*) logic: `loadRowCountCmd`, `counts`, `nextCountIdx`, `RowCountMsg`, count column in render |
| `internal/tui/views/tableinfo.go` | New — `TableInfoModel`, `formatBytes`, `formatEstimate` |
| `internal/tui/app.go` | Add `screenTableInfo`, `tableInfoModel`, wire `i` key |
| `internal/tui/keys/keys.go` | Add `TableInfo key.Binding` (`i`, help `"i  table info"`) |

## Tests

`internal/core/db/postgres_test.go` — add `TestTableStats`:
- Create a test table, insert a row, run ANALYZE, call `TableStats`.
- Assert `RowEstimate >= 0`, `TotalBytes > 0`, `TableBytes > 0`, `IndexBytes >= 0`.
- Assert `Description` is empty (no comment set).

`internal/core/db/mysql_test.go` — same pattern with MySQL equivalents.

`internal/tui/views/tableinfo_test.go`:
- `TestFormatBytes` — table-driven: each size tier, boundary values (1023, 1024, etc.), 0.
- `TestFormatEstimate` — table-driven: 0, 999, 1000, 999999, 1000000.

## Definition of Done

```bash
make preflight
go build ./...
gotestsum --format testdox ./...
staticcheck ./...
make lint
```

Manual:
1. Connect to a database with many tables → no COUNT(*) queries appear in the query log at startup.
2. Table list no longer shows a row count column.
3. Navigate to a table in the table list, press `i` → info overlay opens showing a spinner briefly, then stats.
4. Row estimate shows with `~` prefix and K/M/G suffix where appropriate.
5. Sizes scale correctly: a 2 GB table shows GB, not MB or bytes.
6. Press `i` on a view → nothing happens.
7. Press `i` on a custom SQL dataset → nothing happens.
8. Press `i` again or `Esc` → returns to previous screen.
9. Open info, wait, open again → stats reload (no stale cache).
10. Verify query log shows a single `info <table>` system entry per open — not a COUNT(*).
