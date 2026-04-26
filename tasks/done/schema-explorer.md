# TUI: Schema Explorer Tree

Expand any table or view in the left pane to reveal its structure inline — columns,
indexes, and foreign keys — without leaving the keyboard.

> **Scope note.** A proposed `d` key to dump DDL to the SQL pane was deferred from this
> task — it required a new display mode in the SQL pane and a non-trivial Postgres
> CREATE TABLE reconstruction, both of which belong in a standalone task. The items
> below describe only what was actually shipped.

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
  orders                      1,234
  user_stats       [view]         —
  active_users     [dataset]      —
```

- `→` / `l` expands the focused item; `←` / `h` collapses it
- `Enter` (or `→` on an already-expanded row) opens the row browser (unchanged drill UX)
- Tree items (columns, indexes, FKs) are not selectable — they are display-only
- YAML SQL datasets (`Kind=dataset`) are not expandable — pressing `→` is a no-op
- Scroll accounts for expanded rows so the focused item stays visible
- Columns, FKs, and indexes are loaded lazily the first time a node is expanded; a spinner shows while loading
- If indexes fail to load, `(error)` is shown under the Indexes group — the rest of the app is unaffected
- Views render the Indexes group with `(n/a for views)` since indexes on views aren't meaningful here

## Architecture

### Core layer

**1. Typed `ListTables` in `db.Client`**

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

ListTables(ctx context.Context) ([]TableEntry, error)
```

Both drivers already filtered `WHERE table_type IN ('BASE TABLE','VIEW')` but discarded
the `table_type`; the new query returns it alongside the name and maps it to `TableKind`.

**2. `Indexes` on `db.Client`**

```go
type Index struct {
    Name    string
    Columns []string
    Unique  bool
}

Indexes(ctx context.Context, table string) ([]Index, error)
```

- Postgres: join `pg_class` / `pg_index` / `pg_attribute` scoped to `nspname='public'`, aggregate columns by index name ordered by position in `indkey`.
- MySQL: `information_schema.STATISTICS` filtered by `TABLE_SCHEMA = DATABASE()` and `TABLE_NAME = ?`, aggregate columns by `INDEX_NAME` ordered by `SEQ_IN_INDEX`. `NON_UNIQUE=0` → unique.
- Both take `table` as a bound parameter — no string interpolation.

**3. `schema.Table` gains `Kind`**

```go
type Table struct {
    Name        string
    Kind        db.TableKind
    Columns     []db.Column
    ForeignKeys []db.ForeignKey
}
```

`schema.Load` propagates `Kind` from each `TableEntry`.

**4. `dataset.Dataset` gains `Kind`**

```go
type Kind string

const (
    KindTable   Kind = "table"    // auto-discovered or YAML table-ref
    KindView    Kind = "view"     // auto-discovered view
    KindDataset Kind = "dataset"  // YAML custom SQL
)

type Dataset struct {
    Name  string
    Table string
    SQL   string
    Kind  Kind
}
```

`dataset.Resolver`:
- auto-discovered entries inherit `Kind` from `TableEntry.Kind`
- YAML datasets with `SQL != ""` → `KindDataset`; `table:`-refs → `KindTable`

### TUI layer

**`TableListModel` tree state**

A `tree []treeNode` slice runs parallel to `datasets`, holding per-row expansion state
and lazily-loaded introspection data:

```go
type treeNode struct {
    expanded   bool
    expState   expansionLoadState // idle | loading | loaded | error
    cols       []db.Column
    fks        []db.ForeignKey
    indexState indexLoadState     // idle | loading | loaded | error
    indexes    []db.Index
}
```

`TableListModel` now takes a `db.Client` in addition to `resolver` + `executor`, used
for schema introspection when expanding.

**Commands & messages**

- `loadExpansionCmd(idx, ds)` — calls `client.Describe` + `client.ForeignKeys`, returns `ExpansionLoadedMsg`
- `loadIndexesCmd(idx, ds)` — calls `client.Indexes`, returns `IndexesLoadedMsg`
- Views short-circuit to `indexLoaded` with an empty list — no index query is fired

**Key routing**

The existing `Right` binding both expands and drills, disambiguated by state:

- `Right` on a *collapsed* expandable row → expand (handled inside `TableListModel`)
- `Right` on an *expanded* or non-expandable row → drill into row browser (same as today)
- `Enter` → always drill (unchanged)
- `Left` on an *expanded* row → collapse
- `Left` on a collapsed row → fall through to existing behavior

`app.go` peeks at `tableList.FocusedExpandable()` / `FocusedExpanded()` to decide
whether to consume `Right`/`Left` before the drill logic runs.

**Visible-line flattening**

`buildLines()` flattens `datasets + tree` into a `[]visibleLine` where each entry is
either a header row for a dataset or a sub-row of an expanded block. Scroll operates
in visible-line space so `ensureCursorVisible()` can keep the focused header on screen
even when earlier rows have expanded and pushed content down.

**Badge rendering**

`datasetKindBadge(Kind)` returns:

| Kind      | Badge       |
|-----------|-------------|
| table     | (hidden — default, no badge) |
| view      | `[view]`    |
| dataset   | `[dataset]` |

## Files changed

| File | Change |
|---|---|
| `internal/core/db/client.go` | Added `TableKind`, `TableEntry`, `Index`; typed `ListTables` signature; added `Indexes` to interface |
| `internal/core/db/postgres.go` | `ListTables` returns `table_type`; new `Indexes` via `pg_index`/`pg_attribute` |
| `internal/core/db/mysql.go` | `ListTables` returns `TABLE_TYPE`; new `Indexes` via `information_schema.STATISTICS` |
| `internal/core/db/logging.go` | Updated `ListTables` wrapper; added `Indexes` wrapper |
| `internal/core/db/logging_test.go` | Stub updated to new signature + `Indexes` method |
| `internal/core/db/postgres_test.go` | Integration tests for `ListTables_View` (view kind) and `Indexes` (unique flag, columns) |
| `internal/core/db/mysql_test.go` | Same |
| `internal/core/db/helpers_test.go` | `tableEntryNames`, `findTableEntry` helpers |
| `internal/core/schema/schema.go` | Added `Kind` to `Table`; `Load` propagates from `TableEntry` |
| `internal/core/dataset/dataset.go` | Added `Kind` type + field |
| `internal/core/dataset/resolver.go` | Propagates `Kind` from `TableEntry`; YAML SQL → `KindDataset`, table-refs → `KindTable` |
| `internal/core/dataset/resolver_test.go` | Stub updated; new `TestResolver_YAMLKinds` |
| `internal/tui/views/tablelist.go` | Tree state, expand/collapse, visible-line flattening, kind badges, lazy load commands |
| `internal/tui/views/tablelist_test.go` | Badge rendering, expand/collapse, expansion + indexes loading, error handling, sub-row rendering |
| `internal/tui/app.go` | Passes `db.Client` to `NewTableListModel`; routes `Right`/`Left` through the tree before drill logic |

## Tests

- `db.ListTables` returns `KindTable` for base tables and `KindView` for views (integration, real DB)
- `db.Indexes` returns the index list with unique flag and ordered columns (integration, real DB)
- `dataset.Resolver` sets `Kind` correctly for auto-discovered vs. YAML datasets (unit)
- `TableListModel` renders `[view]` / `[dataset]` badges; bare tables have no badge (unit)
- `FocusedExpandable`/`FocusedExpanded` gating (unit)
- Right/Left expand/collapse round-trip (unit)
- `ExpansionLoadedMsg` + `IndexesLoadedMsg` populate the tree and render columns / FKs / indexes (unit)
- Index load error shows `(error)` marker without crashing (unit)

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
4. Press `→` on an already-expanded row → drills into row browser (existing behavior preserved)
5. Expand a view → Columns visible, Indexes section shows `(n/a for views)`, no crash
6. Scroll with expanded items — focused item stays visible, scroll offset correct
7. Index load failure → `(error)` shown under Indexes group, rest of app unaffected
