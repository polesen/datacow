# TUI: Running Tasks + Query Log

Two visibility features for the TUI:
1. **Running tasks** — spinner + count + label in the status bar while queries are in-flight
2. **Query log** — full-screen panel (`L`) showing all queries with full SQL, timing, and system/user tags

## Architecture

```
LoggingClient (db package)          ← wraps db.Client; intercepts every call
    ↓ passed to executor + resolver
QueryLog (db package)               ← thread-safe ring buffer of entries
    ↓ held by App
TUI App reads RunningCount()/Snapshot() on render
    ↓
status bar:  ⠸ 2 running: orders data
L key:       screenQueryLog — running section + history section
```

All queries are intercepted at the `db.Client` boundary — no changes to executor or resolver.

## Query kinds

| Kind | Examples |
|---|---|
| `system` | `Tables()`, `Describe()`, `ForeignKeys()`, SQL containing `information_schema` / `pg_catalog` / `_dc_schema` |
| `user` | `Query()` calls for data browsing, filtering, row counts |

## Files to create / modify

| File | Change |
|---|---|
| `internal/core/db/querylog.go` | New — `QueryLog`, `QueryEntry`, `QueryKind` |
| `internal/core/db/logging.go` | New — `LoggingClient` wrapping `db.Client` |
| `internal/tui/views/querylog.go` | New — query log TUI panel |
| `internal/tui/app.go` | Wire `LoggingClient`, add `appSpinner`, `queryLog`, `prevScreen`, `screenQueryLog` routing, `L` key |
| `internal/tui/keys/keys.go` | Add `QueryLog key.Binding` (`L`, help `"L  query log"`) |

No changes to `executor.go`, `resolver.go`, or driver files.

## Implementation

Start the golang LSP and use it. gopls already installed.

### `internal/core/db/querylog.go`

```go
type QueryKind int
const (
    QueryKindUser   QueryKind = iota  // triggered by user navigation
    QueryKindSystem                   // schema introspection / datacow-internal
)

type QueryEntry struct {
    ID        int
    Label     string        // e.g. "query", "describe orders", "list tables"
    SQL       string        // full SQL; empty for non-SQL methods (Tables, Describe, ForeignKeys)
    Kind      QueryKind
    StartedAt time.Time
    Duration  time.Duration // 0 = still running
    RowCount  int64
    Error     error
}

type QueryLog struct {
    mu      sync.RWMutex
    nextID  int
    running map[int]*runningEntry  // in-flight
    history []QueryEntry           // completed, newest-first, cap 200
}

func NewQueryLog() *QueryLog
func (l *QueryLog) begin(label, sql string, kind QueryKind) int
func (l *QueryLog) end(id int, rowCount int64, err error)
func (l *QueryLog) RunningCount() int
func (l *QueryLog) CurrentLabel() string
func (l *QueryLog) Snapshot() (running []QueryEntry, history []QueryEntry)
```

### `internal/core/db/logging.go`

Wraps any `db.Client`. All calls log to `*QueryLog`.

- `Tables(ctx)` → label `"list tables"`, kind `system`, SQL `""`
- `Describe(ctx, table)` → label `"describe <table>"`, kind `system`, SQL `""`
- `ForeignKeys(ctx, table)` → label `"FK <table>"`, kind `system`, SQL `""`
- `Query(ctx, sql, args...)` → label `"query"`, SQL = full sql arg, kind from heuristic:

```go
func kindFromSQL(sql string) QueryKind {
    up := strings.ToUpper(sql)
    if strings.Contains(up, "INFORMATION_SCHEMA") ||
        strings.Contains(up, "PG_CATALOG") ||
        strings.Contains(up, "_DC_SCHEMA") {
        return QueryKindSystem
    }
    return QueryKindUser
}
```

Row counts: `len(rows)` / `len(tables)` / `len(cols)` / `len(fks)`.

### `internal/tui/keys/keys.go`

Add `QueryLog key.Binding` with `key.WithKeys("L")` and help `"L  query log"`.

### `internal/tui/app.go`

New fields on `App`:
```go
appSpinner   spinner.Model
queryLog     *db.QueryLog
prevScreen   screen
queryLogView views.QueryLogView
```

Add `screenQueryLog` to the `screen` const block.

`New()` wires the logging client:
```go
queryLog := db.NewQueryLog()
lc := db.NewLoggingClient(client, queryLog)
resolver := dataset.NewResolver(lc)
executor := dataset.NewExecutor(lc)
a.queryLog = queryLog
a.queryLogView = views.NewQueryLogView(queryLog)
```

`Init()` adds `a.appSpinner.Tick` to the batch (always ticking for status bar animation).

`Update()` changes:
- Handle `spinner.TickMsg` at top: update `a.appSpinner`, then forward to sub-view
- Handle `L` key: toggle `screenQueryLog` (save/restore `prevScreen`)
- Handle `Esc` in `screenQueryLog`: go back to `prevScreen`
- Route msgs to `queryLogView` when `a.screen == screenQueryLog`
- `WindowSizeMsg` propagates to `queryLogView`

`renderStatusBar()` when `a.queryLog.RunningCount() > 0`, show on left:
```
⠸ 2 running: orders data
```

### `internal/tui/views/querylog.go`

```go
type QueryLogView struct {
    queryLog    *db.QueryLog
    cursor      int          // selected history entry
    width, height int
}

func NewQueryLogView(ql *db.QueryLog) QueryLogView
func (v QueryLogView) Update(msg tea.Msg) (QueryLogView, tea.Cmd)
func (v QueryLogView) View() string
```

Layout:
```
Running (2)
  ⠸ orders data        0.4s…    user
  ⠸ describe orders    0.1s…    system

History
  42ms   orders data        1,234 rows   user
   8ms   describe orders    12 cols      system
  15ms   list tables        8 tables     system

SQL: SELECT * FROM orders WHERE status = $1 ORDER BY created_at DESC LIMIT 50 OFFSET 0
```

- Running: spinner + label + elapsed + kind badge
- History: duration + label + row count + kind badge
- Selected history entry shows full SQL at bottom
- Up/Down moves cursor through history
- Esc / L handled by App (not view)

## Tests

- `internal/core/db/querylog_test.go` — thread safety, RunningCount, Snapshot, history cap
- `internal/core/db/logging_test.go` — entries created for Query/Tables/Describe/ForeignKeys calls

## Definition of Done

```bash
make preflight
go build ./...
gotestsum --format testdox ./...
staticcheck ./...
make lint
```

Manual:
1. Table list loads → status bar shows spinner + "N running: list tables" → completes
2. Open a table → status bar shows running data + count queries
3. Press `L` → query log opens, running at top, history below with full SQL
4. Cursor through history → full SQL shown at bottom
5. Schema queries labeled `system`, data queries labeled `user`
6. Press `L` or `Esc` → returns to previous screen
