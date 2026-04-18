# TUI: Schema Explorer Tree

Expand any table or view in the left pane to reveal its structure inline — columns,
indexes, and foreign keys — without leaving the keyboard or opening a separate pane.

## UX

```
  orders                    1,234
▶ users                       892    ← press → to expand
  products                    541

  orders                    1,234
▼ users                       892    ← expanded
  ├─ Columns
  │   ├─ id            bigint  NN
  │   ├─ email         text    NN
  │   └─ created_at    timestamptz
  ├─ Indexes
  │   ├─ users_pkey    (id)  UNIQUE
  │   └─ users_email   (email)  UNIQUE
  └─ Foreign Keys
      └─ (none)
  products                    541
```

Object-type badges appear on every row in the list:

```
  orders           [table]   1,234
  user_stats       [view]      —
  active_users     [dataset]   —
```

- `→` / `l` expands the focused item; `←` / `h` collapses it
- `Enter` still opens the row browser (unchanged)
- `d` while focused on any table/view opens its DDL in the SQL pane
- Tree items (columns, indexes, FKs) are not selectable — they are display-only
- Scroll accounts for expanded rows so the focused item stays visible
- Indexes are loaded lazily the first time a node is expanded; a spinner shows while loading
- If indexes fail to load, show `(error)` under the Indexes group — don't crash

## Architecture

### Core layer

**1. Distinguish tables from views in `ListTables`**

Change `db.Client.ListTables` to return typed entries:

```go
type TableKind string

const (
    KindTable TableKind = "table"
    KindView  TableKind = "view"
)

type TableEntry struct {
    Name string
    Kind TableKind
}

// ListTables returns all tables and views in the default schema.
ListTables(ctx context.Context) ([]TableEntry, error)
```

Update both postgres and mysql drivers. The Postgres query should read from
`information_schema.tables` and include `table_type`. MySQL similarly.

**2. Add `Indexes` to `db.Client`**

```go
type Index struct {
    Name    string
    Columns []string
    Unique  bool
}

Indexes(ctx context.Context, table string) ([]Index, error)
```

Postgres: query `pg_indexes` or `information_schema` + `pg_index`.
MySQL: query `information_schema.statistics`.

**3. Add `DDL` to `db.Client`**

```go
DDL(ctx context.Context, table string) (string, error)
```

Postgres: `pg_get_tabledef` or reconstruct from catalog. Simplest approach: use `pg_dump`-style
query from `information_schema` + `pg_get_indexdef`. MySQL: `SHOW CREATE TABLE`.

**4. Add `Kind` to `dataset.Dataset`**

```go
type DatasetKind string

const (
    DatasetKindTable   DatasetKind = "table"
    DatasetKindView    DatasetKind = "view"
    DatasetKindDataset DatasetKind = "dataset"  // custom SQL from YAML
)

type Dataset struct {
    Name string
    SQL  string
    Kind DatasetKind
}
```

Resolver must propagate `Kind` from the `TableEntry` for auto-discovered items.
Custom SQL datasets from YAML always get `KindDataset`.

**5. `schema.Table` gains `Kind`**

```go
type Table struct {
    Name        string
    Kind        db.TableKind
    Columns     []db.Column
    ForeignKeys []db.ForeignKey
}
```

### TUI layer

**`TableListModel` becomes a tree model**

Add per-item expand state and lazy index cache:

```go
type treeItem struct {
    dataset    dataset.Dataset
    expanded   bool
    indexes    []db.Index
    indexState indexState  // idle | loading | loaded | error
}

type indexState int
const (
    indexIdle indexState = iota
    indexLoading
    indexLoaded
    indexError
)
```

Add a `loadIndexesCmd(table string) tea.Cmd` that calls `db.Client.Indexes` and returns
an `IndexesLoadedMsg` or `IndexesErrMsg`.

Add a `loadDDLCmd(table string) tea.Cmd` that calls `db.Client.DDL` and returns a
`DDLLoadedMsg` — the SQL pane receives it and displays the DDL string.

**Key bindings**

```go
ExpandTree  key.Binding  // → l
CollapseTree key.Binding // ← h
ShowDDL     key.Binding  // d
```

**View rendering**

When computing visible rows, expand tree items inline. Track a `visibleItems` slice
mapping render-line index to `(itemIdx, subRow)` so cursor and scroll work correctly.

**Badge rendering**

Replace the `(query)` label logic with a general kind badge:

| Kind      | Badge       |
|-----------|-------------|
| table     | (hidden — default, no badge) |
| view      | `[view]`    |
| dataset   | `[dataset]` |

Tables get no badge to keep the common case clean.

## Files to create / modify

| File | Change |
|---|---|
| `internal/core/db/client.go` | Add `TableEntry`, `TableKind`, `Index`, change `ListTables` signature, add `Indexes`, `DDL` |
| `internal/core/db/postgres.go` | Implement new methods |
| `internal/core/db/mysql.go` | Implement new methods |
| `internal/core/schema/schema.go` | Add `Kind` to `Table`, update `Load` |
| `internal/core/dataset/dataset.go` | Add `DatasetKind` to `Dataset` |
| `internal/core/dataset/resolver.go` | Propagate `Kind` from `TableEntry` |
| `internal/tui/views/tablelist.go` | Tree expand/collapse, lazy index load, kind badges, DDL key |
| `internal/tui/views/tablelist_test.go` | Tree rendering, kind badge, expand/collapse tests |
| `internal/tui/keys/keys.go` | `ExpandTree`, `CollapseTree`, `ShowDDL` |
| `internal/tui/app.go` | Handle `DDLLoadedMsg`, push DDL string to SQL pane |

## Tests

- `db.ListTables` returns correct `Kind` for both tables and views (integration, real DB)
- `db.Indexes` returns correct index list with unique flags (integration, real DB)
- `db.DDL` returns non-empty string for a known table (integration, real DB)
- `dataset.Resolver` sets `Kind` correctly for auto-discovered vs YAML datasets (unit)
- `TableListModel` renders `[view]` and `[dataset]` badges correctly (unit, no DB)
- Expand/collapse key handling toggles `expanded` on the focused item (unit)
- View height accounting includes expanded sub-rows (unit)

## Definition of Done

```bash
make preflight
go build ./...
gotestsum --format testdox ./...
staticcheck ./...
make lint
```

Manual:
1. Connect to test DB — tables show no badge, views show `[view]`, YAML datasets show `[dataset]`
2. Press `→` on a table → expands with Columns, Indexes (spinner then list), Foreign Keys
3. Press `←` → collapses back to single row
4. Expand a view → Columns visible, Indexes empty or populated, no crash
5. Press `d` on any table → DDL appears in SQL pane
6. Scroll with expanded items — focused item stays visible, scroll offset correct
7. Index load failure → `(error)` shown under Indexes group, rest of app unaffected
