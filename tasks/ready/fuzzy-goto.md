# TUI: Schema Cache + Fuzzy Goto

Global fuzzy-search over all schema objects — tables, views, datasets, columns, and
datasources. Triggered with `ctrl+p` from anywhere. Backed by a full schema cache in
core that loads eagerly on startup and can be refreshed on demand.

Modelled on Neovim Telescope / lazygit command palette: floating dialog, live-filtered
as you type, scored ranking, match highlighting, keyboard navigation.

## UX

```
╭─ Goto ──────────────────────────────────────────────╮
│ > ord                                               │
├─────────────────────────────────────────────────────┤
│  orders                                    [table]  │  ← cursor (highlighted)
│  order_items                               [table]  │
│  orders.id                                [column]  │
│  orders.user_id                           [column]  │
│  order_summary                             [view]   │
│  monthly_orders                          [dataset]  │
╰─────────────────────────────────────────────────────╯
               esc close  ↵ goto  ↑↓ select
```

- `ctrl+p` opens the dialog from any screen with an active connection (not from the
  datasource picker before a connection is open)
- Text input is always focused; `↑/k` and `↓/j` move the cursor while typing
- Matched characters are highlighted in a distinct colour
- Empty query: all entries shown, sorted datasources → tables → views → datasets → columns
- Non-empty query: results ranked by fuzzy score (best match first)
- `Enter` navigates to the selected entry and closes the dialog
- `Esc` closes and returns to the previous screen without navigating
- Visible results: up to 12; list scrolls when there are more

**What's searchable:**

| Kind | Display | Example | Navigates to |
|---|---|---|---|
| `table` | bare name | `orders` | open in row browser |
| `view` | `[view]` badge | `user_stats [view]` | open in row browser |
| `dataset` | `[dataset]` badge | `monthly_orders [dataset]` | open in row browser |
| `column` | `table.column [column]` | `orders.user_id [column]` | open parent table in row browser |
| `datasource` | `[datasource]` badge | `production [datasource]` | switch connection |

Datasource entries only appear in multi-datasource mode.  
Columns come from real DB tables and views — not from YAML SQL datasets (no fixed schema
to introspect there without executing the query).

**Refresh:**

- `ctrl+r` triggers a background cache reload; spinner shows in the status bar while loading
- When complete, the goto index is silently updated — no modal, no interruption

## Architecture

### Core layer — `internal/core/schema/cache.go`

The cache lives in core so the HTTP API (future work) can expose a `/schema` endpoint
without depending on the TUI.

```go
type EntryKind string

const (
    EntryKindTable      EntryKind = "table"
    EntryKindView       EntryKind = "view"
    EntryKindDataset    EntryKind = "dataset"
    EntryKindColumn     EntryKind = "column"
    EntryKindDatasource EntryKind = "datasource"
)

// SearchEntry is one item in the goto index.
type SearchEntry struct {
    Kind      EntryKind
    Name      string           // display and fuzzy-match target; columns: "table.column"
    TableName string           // parent table (columns) or same as Name (others)
    Dataset   *dataset.Dataset // navigation target; nil for datasource entries
    DSName    string           // non-empty for datasource entries
}

// MatchResult is one fuzzy search hit.
type MatchResult struct {
    Entry          SearchEntry
    MatchedIndexes []int // positions in Name that matched, for highlighting
}

// Cache holds the full schema snapshot for one datasource.
type Cache struct {
    mu       sync.RWMutex
    ready    bool
    tables   []Table            // raw: all tables + views with columns + FKs
    datasets []dataset.Dataset  // raw: all datasets (auto-discovered + YAML)
    entries  []SearchEntry      // pre-built search index
}

func NewCache() *Cache
func (c *Cache) Ready() bool

// Tables and Datasets expose the raw data for future use (e.g. HTTP API /schema).
func (c *Cache) Tables() []Table
func (c *Cache) Datasets() []dataset.Dataset

// Search runs a fuzzy match over the entry index.
// Empty query returns all entries in default sort order.
func (c *Cache) Search(query string) []MatchResult

// load is the shared internal implementation used by both Load and Refresh.
func (c *Cache) load(ctx context.Context, client db.Client, resolver *dataset.Resolver) error
```

`Load` and `Refresh` are identical except Refresh does a full reload under the write lock
and swaps atomically:

```go
func (c *Cache) Load(ctx context.Context, client db.Client, resolver *dataset.Resolver) error
func (c *Cache) Refresh(ctx context.Context, client db.Client, resolver *dataset.Resolver) error
```

**`load` internals:**

1. Call `schema.Load(ctx, client)` — fetches all tables + views with their columns and FKs
2. Call `resolver.Resolve(ctx)` — fetches all datasets (tables, views, YAML SQL)
3. Build a `map[string]*dataset.Dataset` from table name → dataset pointer (for column entries)
4. Construct `entries []SearchEntry`:
   - One entry per dataset (kind from `ds.Kind`)
   - One entry per column for every table/view from schema.Load (kind `EntryKindColumn`,
     `Name = table + "." + col.Name`, `TableName = table`, `Dataset` = map lookup)
   - YAML SQL datasets are excluded from column entries (no fixed schema)
5. Acquire write lock, replace `tables`, `datasets`, `entries`, set `ready = true`

**Thread safety:** reads (Search, Tables, Datasets, Ready) hold the read lock; the swap
in load holds the write lock for the assignment only (building is done outside the lock).

**Fuzzy matching:** use `github.com/sahilm/fuzzy`. Implement `fuzzy.Source` over
`[]SearchEntry` returning `entry.Name` from `String(i int)`. Empty query bypasses fuzzy
and returns all entries in default order.

**Indexes:** not included in the cache or search results — they are not navigatable
objects and are already handled lazily by the schema explorer tree. Deferred to a future
task.

### TUI layer

**`internal/tui/views/goto.go` — new file**

```go
type GotoModel struct {
    cache       *schema.Cache
    datasources []config.DatasourceConfig
    input       textinput.Model
    results     []schema.MatchResult
    cursor      int
    width       int
    height      int
}

// GotoSelectedMsg is emitted when the user confirms a selection.
type GotoSelectedMsg struct {
    Dataset    *dataset.Dataset // nil when navigating to a datasource
    Datasource string           // non-empty when switching datasource
}
```

`NewGotoModel(cache *schema.Cache, datasources []config.DatasourceConfig)` — the model
holds a pointer to the cache; `Search()` is called on every keystroke. If `!cache.Ready()`,
the result list shows "Loading schema…".

`GotoModel.View()` renders a centered floating panel using `lipgloss.Place()`:

- Panel width: `min(termWidth-4, 72)`
- Panel height: input line + divider + up-to-12 result rows + footer hint
- The panel is placed in the centre of the available screen area

Match highlighting: for each `MatchResult`, walk `MatchedIndexes` to split `entry.Name`
into matched/unmatched rune runs, render matched runs with `style.GotoMatch`.

**`internal/tui/views/tablelist.go` — one addition**

Add `SelectByName(name string) bool` — sets `m.cursor` to the first dataset whose
`Name` matches; returns true if found. Used by the app after a goto selection to keep
the table list cursor in sync.

**`internal/tui/app.go` — changes**

New screen constant:
```go
screenGoto screen = iota
```

New fields on `App`:
```go
schemaCache    *schema.Cache
gotoModel      views.GotoModel
cacheLoading   bool
```

`activateConnection` creates a fresh `schema.Cache` and `GotoModel`, then returns a
`cacheLoadCmd` alongside the existing `tableList.Init()`.

`cacheLoadCmd` wraps `cache.Load(ctx, client, resolver)` and returns
`SchemaCacheReadyMsg{}` on success or `SchemaCacheErrMsg{Err}` on failure.

`cacheRefreshCmd` wraps `cache.Refresh(...)` and returns `SchemaCacheRefreshedMsg{}`.

**`Update()` additions:**

```go
// ctrl+p — open goto (top-level, before per-screen routing)
if key.Matches(msg, a.keys.Goto) && a.screen != screenDatasourcePicker {
    a.screenBeforeOverlay = a.screen
    a.screen = screenGoto
    a.gotoModel.Focus()
    return a, nil
}

// ctrl+r — trigger refresh (top-level, when connection is active)
if key.Matches(msg, a.keys.Refresh) && a.schemaCache != nil && !a.cacheLoading {
    a.cacheLoading = true
    return a, a.cacheRefreshCmd()
}
```

Message handlers:
- `SchemaCacheReadyMsg` / `SchemaCacheRefreshedMsg`: set `cacheLoading = false`; the
  `GotoModel` already holds the `*Cache` pointer and sees the updated state automatically
- `SchemaCacheErrMsg`: set `cacheLoading = false`; log silently (goto shows stale data)
- `GotoSelectedMsg` with `Dataset != nil`: call `tableList.SelectByName(ds.Name)`,
  create `RowBrowserModel` for the dataset, push size, set `screenSplit`, `focusRowBrowser`
- `GotoSelectedMsg` with `Datasource != ""`: emit `DatasourceSelectMsg` (existing path)

Route `screenGoto` messages to `gotoModel` in the per-screen switch.

`renderContent()` adds a `screenGoto` case that renders the current underlying screen
content as background and overlays the goto dialog using `lipgloss.Place()`. Because
lipgloss renders each component as a string (not a live canvas), this is done by
rendering the split or picker content first and then placing the dialog string on top:

```go
case screenGoto:
    return a.gotoModel.View() // GotoModel.View() uses lipgloss.Place over full w×h
```

`renderStatusBar()` shows a "schema loading…" indicator (spinner + label) alongside the
running-query indicator when `cacheLoading` is true.

### Styles

Add to `internal/tui/style/style.go`:
```go
GotoMatch lipgloss.Style  // bold accent colour for matched characters
```

### Keys

Add to `keys.Map`:
```go
Goto    key.Binding  // ctrl+p
Refresh key.Binding  // ctrl+r
```

## Tests

### `internal/core/schema/cache_test.go` — new

- **`Ready()`** — false before Load, true after
- **`Load()`** — populated entries include one per dataset and one per column of each table
- **`Search` empty query** — returns all entries in default order (datasources omitted from
  core; tables before views before datasets before columns)
- **`Search` with query** — matched entries present, unmatched absent; best match first
- **`Search` column** — `"users.email"` matches query `"email"` and `"users.em"`
- **`Refresh`** — after schema changes (add a table), Refresh updates entries; old entries gone
- **Thread safety** — concurrent Search calls during Refresh do not race (run with `-race`)

Use real DBs via `TEST_POSTGRES_DSN` / `TEST_MYSQL_DSN`.

### `internal/tui/views/goto_test.go` — new

- **Not ready** — `View()` shows "Loading schema…"; no panic
- **Ready** — after `SetCache`, all entries visible with empty query
- **Filtering** — typing partial name reduces result list correctly
- **Ranking** — exact prefix ranks above distant match
- **Cursor movement** — `↓`/`↑`/`j`/`k` move selection; wraps at boundaries
- **Enter** emits `GotoSelectedMsg` with correct Dataset / Datasource
- **Esc** emits no `GotoSelectedMsg`
- **Scroll** — 15 entries, 12 visible: scrolling keeps cursor in view
- **Column entry navigation** — selecting `users.email` [column] emits `GotoSelectedMsg`
  with the `users` dataset

### `internal/tui/views/tablelist_test.go`

- **`SelectByName`** — known name selects and returns true; unknown name returns false

## Files changed

| File | Change |
|---|---|
| `internal/core/schema/cache.go` | New — `Cache`, `EntryKind`, `SearchEntry`, `MatchResult` |
| `internal/core/schema/cache_test.go` | New — integration tests against real DB |
| `internal/tui/keys/keys.go` | Add `Goto` (`ctrl+p`) and `Refresh` (`ctrl+r`) |
| `internal/tui/style/style.go` | Add `GotoMatch` style |
| `internal/tui/views/goto.go` | New — `GotoModel`, `GotoSelectedMsg` |
| `internal/tui/views/goto_test.go` | New — unit tests |
| `internal/tui/views/tablelist.go` | Add `SelectByName` |
| `internal/tui/views/tablelist_test.go` | Test `SelectByName` |
| `internal/tui/app.go` | `screenGoto`, `schemaCache`, cache load/refresh cmds, key routing, message handlers |
| `go.mod` / `go.sum` | Add `github.com/sahilm/fuzzy` |

## Definition of Done

```bash
make preflight
go build ./...
gotestsum --format testdox ./...
staticcheck ./...
make lint
```

Manual:
1. Press `ctrl+p` from the table list → dialog opens with all tables/views/datasets
2. Press `ctrl+p` from the row browser → dialog opens (works from any screen)
3. Type a partial name → results filter live; matched characters visually distinct
4. Type a column name (e.g. `email`) → column entries appear as `table.email [column]`
5. Select a column entry → row browser opens on the parent table
6. Press `↓`/`↑` to move selection, `Enter` to navigate, `Esc` to close without action
7. Close dialog → previous screen and focus fully restored
8. In multi-datasource mode: datasource entries appear first; selecting one switches connection
9. Press `ctrl+r` → spinner shows in status bar; goto index updates silently when done
10. Open dialog before initial load completes → "Loading schema…" shown; no crash
11. Open dialog with 15+ tables → result list scrolls correctly
