# Row Browser: Per-Dataset Page Size, No Default COUNT(\*), First/Last Page

Today every row-browser query runs a `COUNT(*)` alongside the data query so the
status bar can show `page N/M  T rows`. The page size is hard-coded to 50 with
no way to change it. There are no bindings to jump to the first or last page.

This task replaces all three:

- **Page size is configurable per dataset, in memory.** Press `P`, type a
  number, Enter to apply. Each dataset (table or saved query) keeps its own
  page size for the life of the process; restart resets everything to the
  default (`50`).
- **No `COUNT(*)` on the default page-load path.** The status bar shows just
  `page N` until the user discovers the end of the dataset. The total is
  discovered (and then displayed as `page N/M  ~T rows`) in only two
  situations: (a) paging forward into a page that returns fewer rows than the
  page size, or (b) the user explicitly pressing `G` / `End` to jump to the
  last page, which runs a one-shot `COUNT(*)`.
- **First-page / last-page bindings.** `g` and `Home` jump to page 1. `G` and
  `End` jump to the last page (triggering the one-shot count).

## Background

`COUNT(*)` on every page load is expensive on large tables — on a 50M-row
Postgres table without an accurate `pg_class.reltuples`, a full scan can take
seconds per page. The previous "rolling COUNT on connect" was removed for the
same reason (commit `342c147`, *"remove COUNT(\*) query storm"*). The
table-info overlay (`i`) already provides a cheap row estimate via
catalog statistics when the user actually wants one — the row browser does
not need to compete with it.

Hard-coded page size of 50 is fine for small tables and bad for big ones:
50 rows of a wide column-heavy table is wasteful when the user wants a quick
overview, and tiny when paging through a million-row audit log.

Per-dataset (not per-datasource, not per-session) is the right granularity
because page size is a property of how the user wants to read *this view*
of *this data*. The choice survives drill-downs (popping back to the
parent dataset restores its own size) and tab-pane switches.

## End-of-data detection without `COUNT(*)`

When `SkipCount: true` (the new opt-in flag — see Core changes), the executor
fetches `PageSize + 1` rows from the underlying driver, returns the first
`PageSize`, and sets `HasMore = true` iff the driver returned the full
`PageSize + 1`. The extra row is paid for once per page and is the cheapest
correct way to answer "is there another page?" without scanning the rest of
the table.

`HasMore == false` means the user has just loaded the last page. At that
moment the row browser knows: `TotalPages = m.result.Page`, and a tight
estimate of the row count as `(Page - 1) * PageSize + len(Rows)`. Both are
stored as the discovered total and displayed.

Any change that invalidates the total — adding/removing a filter, changing
sort, changing page size — clears the discovered total. Drilling down into
an FK invalidates because the new dataset has its own total.

## UX

### Status bar

The row browser status line drives off whether the total is known:

```
# Default (total unknown)
public.orders  page 3                   [2 filter(s)]  sort: id DESC

# After hitting end via ] paging
public.orders  page 5/5  ~247 rows      [2 filter(s)]

# After pressing G (one-shot COUNT)
public.orders  page 5/5  247 rows       [2 filter(s)]
```

- Tilde (`~`) prefixes the row count when it was inferred from
  `(Page-1)*PageSize + len(Rows)`. No tilde when the count came from an
  explicit `COUNT(*)`.
- When the total is unknown, omit `/M` and the rows component entirely.
  Don't render `page N/?` — empty is cleaner than guessing.
- `[N filter(s)]` and `sort: ...` are unchanged in placement and behaviour.

### Per-dataset page size — `P`

Pressing `P` (Shift+P) in the row browser docks a single-line input at the
bottom, identical in style to the existing filter input:

```
Page size: 50_
```

- Pre-filled with the current effective page size for the active dataset.
- Cursor sits at the end of the existing value.
- Accepts digits only (silently drop non-digit keystrokes — same pattern the
  type-aware filter modal will use for integer columns).
- `Enter` validates the value, stores it in the page-size registry under the
  active dataset's name, and triggers `loadPageCmd(1)`. The discovered total
  (if any) is cleared because page numbers no longer map to the previous
  view.
- `Esc` discards the input; nothing changes.
- Validation: empty input or `0` is rejected and the bar stays open with a
  one-shot error message: `Page size: 50  (must be between 1 and 10000)`.
  Values are clamped to `[1, 10000]`; anything outside is rejected the same
  way.

While the page-size input is open, no other key bindings fire in the row
browser, exactly as with the filter input today.

The page-size value is held in a small registry shared across all
`RowBrowserModel` instances:

```go
// internal/tui/views/pagesize.go
type PageSizeRegistry struct {
    sizes map[string]int // keyed by dataset.Name
    def   int            // default for unknown keys
}

func NewPageSizeRegistry(defaultSize int) *PageSizeRegistry
func (r *PageSizeRegistry) Get(name string) int
func (r *PageSizeRegistry) Set(name string, size int)
```

Owned by `App`, constructed once with `NewPageSizeRegistry(50)`, and passed
into `NewRowBrowserModel`. Tests can construct their own. No file
persistence — restart resets. No global state.

### First / last page — `g` / `G` / `Home` / `End`

| Key                | Action                                                  |
| ------------------ | ------------------------------------------------------- |
| `g`, `Home`        | Jump to page 1. Same load path as `loadPageCmd(1)` today. |
| `G`, `End`         | Run a one-shot `COUNT(*)`; compute last page; load it.    |

`G` / `End` flow:

1. Status bar shows `Finding last page...` while the count is in flight.
2. Executor is invoked with `QueryOptions.OnlyCount = true` — see Core
   changes. The executor returns a `QueryResult` populated with `TotalRows`
   and `TotalPages` set, and no `Rows`. (`OnlyCount: true` and
   `SkipCount: true` are mutually exclusive; the executor errors if both
   are set.)
3. On result, the row browser stores `TotalRows` and `TotalPages` (the
   *non-inferred*, COUNT-derived values — no `~` prefix in the status bar)
   and calls `loadPageCmd(TotalPages)` to fetch the page itself. The second
   load runs with `SkipCount: true` like every other page load.

If the count fails (timeout, permission), the row browser stays on the
current page and shows a transient status message: `goto last failed: <err>`.
The discovered total, if any, is preserved.

`g` / `Home` simply call `loadPageCmd(1)`. No count.

Bindings come in *pairs* — `g` is the primary entry shown in the help overlay,
with `Home` listed alongside as `(home)`. Same pattern for `G` / `End`.

## Core changes

### `QueryOptions`

Add two flags. They are independent of each other except `SkipCount` and
`OnlyCount` are mutually exclusive (validated at the top of `Query`).

```go
type QueryOptions struct {
    Page     int
    PageSize int
    Filters  []Filter
    Sort     *Sort

    // SkipCount disables the COUNT(*) query and uses PageSize+1 row probing
    // to populate HasMore. TotalRows and TotalPages on the result are nil.
    SkipCount bool

    // OnlyCount runs only the COUNT(*) query — the data SELECT is skipped.
    // The returned QueryResult has Columns and TotalRows/TotalPages set,
    // Rows is empty, HasMore is unused. Used by goto-last.
    OnlyCount bool
}
```

### `QueryResult`

Change the totals to optional (pointer) and add `HasMore`. Per the project
Go idioms, "no sentinel integers" — `*int64` / `*int` with `nil = unknown`
is the correct shape.

```go
type QueryResult struct {
    Columns    []db.Column
    Rows       []map[string]any
    Page       int
    PageSize   int

    // TotalRows is nil when the total was not computed (SkipCount path
    // without end-of-data discovery). Set by the executor only in the
    // default-count path and the OnlyCount path.
    TotalRows  *int64

    // TotalPages mirrors TotalRows — nil when unknown.
    TotalPages *int

    // HasMore is true when the executor detected that another page exists
    // beyond the returned rows. Always populated; in the default-count
    // path it is derived from TotalPages and Page.
    HasMore    bool
}
```

`TotalRows` and `TotalPages` are pointer-typed. Every existing caller that
reads them is updated:

| File                                      | Change                                                                                       |
| ----------------------------------------- | -------------------------------------------------------------------------------------------- |
| `internal/core/dataset/executor.go`        | Two paths: default (with COUNT), `SkipCount` (PageSize+1 probe, no COUNT), `OnlyCount` (no data SELECT). Set pointer fields appropriately. |
| `internal/core/dataset/dataset_test.go`    | Update to dereference `*TotalRows` / `*TotalPages`. Add new tests for `SkipCount` (HasMore=true / false, totals nil) and `OnlyCount` (Rows empty, totals set). |
| `internal/core/export/exporter.go`         | Exporter keeps the default code path (no SkipCount). Update `page >= result.TotalPages` to dereference. Exporter remains untouched UX-wise — it walks every page and benefits from the count. |
| `internal/tui/views/rowbrowser.go`         | Pass `SkipCount: true` on default loads; on `G` press, send `OnlyCount: true` first and then load the discovered page. Use `HasMore` and the discovered-total fields to drive the status bar. Remove `m.result.Page < m.result.TotalPages` gate — replace with `m.result.HasMore`. |
| `internal/tui/views/rowbrowser.go` accessors | `TotalPages()` and `TotalRows()` return `(value, ok)` instead of bare ints. Update callers. |
| `internal/tui/app.go`                       | Constructs and owns the `PageSizeRegistry`; passes it to `NewRowBrowserModel`. |

Executor implementation detail (`SkipCount` path):

```go
// Inside Executor.Query when opts.SkipCount:
dataArgs := append(args, pageSize+1, offset) // request one extra row
rows, err := e.client.Query(ctx, dataSQL, dataArgs...)
// ...
hasMore := len(rows) > pageSize
if hasMore {
    rows = rows[:pageSize]
}
return &QueryResult{
    Columns:  cols,
    Rows:     rows,
    Page:     page,
    PageSize: pageSize,
    HasMore:  hasMore,
    // TotalRows, TotalPages: nil
}, nil
```

`OnlyCount` runs just the existing count goroutine inline (no goroutine
needed because there's nothing else to wait on) and returns a result with
`Columns` populated (so callers can still render the table header during
the brief moment between the count and the follow-up data fetch),
`TotalRows`/`TotalPages` set, and `Rows` empty.

## Row browser bookkeeping

Add three fields:

```go
type RowBrowserModel struct {
    // ...existing fields...

    pageSizes        *PageSizeRegistry // shared across drills, owned by App
    knownTotalPages  *int              // discovered total; nil until end reached
    knownTotalRows   *int64            // ditto; tilde shown when inferred
    knownTotalExact  bool              // true iff discovered via COUNT(*) (G key)
    pageSizeInput    textinput.Model   // docked input for the P key
    mode             uiMode            // new variant: modePageSizeInput
}
```

State transitions that **clear** `knownTotalPages` / `knownTotalRows`:

- Filter added or removed.
- Sort changed.
- Page size changed.
- FK drill-down (new dataset → new totals).
- Drill-stack pop (parent's totals were saved alongside its other state in
  `savedLevel`; restore them too).

State transitions that **set** them:

- Default page-load returns with `HasMore == false`:
  `knownTotalPages = &m.result.Page`,
  `knownTotalRows = &((Page-1)*PageSize + int64(len(Rows)))`,
  `knownTotalExact = false` (tilde shown).
- `OnlyCount` round-trip completes:
  `knownTotalPages = result.TotalPages`,
  `knownTotalRows = result.TotalRows`,
  `knownTotalExact = true` (no tilde).

The accessor `TotalPages() (int, bool)` returns `(*m.knownTotalPages, true)`
or `(0, false)`. Renderers that previously read the raw `m.result.TotalPages`
go through the accessor.

`savedLevel` (the drill-stack entry) grows fields for `knownTotalPages`,
`knownTotalRows`, `knownTotalExact`, and the dataset's page size (snapshot
in case the user changes it mid-drill on a child level — when we pop back,
we restore the page size that was active for the parent).

## Keys

Add to `keys.Map`:

```go
FirstPage  key.Binding // g, home — first page
LastPage   key.Binding // G, end  — last page (runs one-shot COUNT)
PageSize   key.Binding // P       — open page-size input
```

Default bindings:

```go
FirstPage: key.NewBinding(
    key.WithKeys("g", "home"),
    key.WithHelp("g/home", "first page"),
),
LastPage: key.NewBinding(
    key.WithKeys("G", "end"),
    key.WithHelp("G/end", "last page"),
),
PageSize: key.NewBinding(
    key.WithKeys("P"),
    key.WithHelp("P", "page size"),
),
```

`FullHelp()` adds them to the navigation group:

```go
{m.Up, m.Down, m.Left, m.Right, m.Enter, m.Back, m.NextPage, m.PrevPage, m.FirstPage, m.LastPage, m.PageSize},
```

Help-overlay text lives in `views/helpoverlay.go` and must list all three
new entries.

## Acceptance Criteria

### Page size

- [ ] `P` in the row browser docks a single-line input at the bottom labelled `Page size:`, pre-filled with the current effective page size for the active dataset.
- [ ] Only digits are accepted; non-digit keystrokes are silently dropped while the input is open.
- [ ] `Enter` with a valid value (`1..10000`) updates the dataset's page size in the registry, clears the discovered total, and reloads page 1.
- [ ] `Enter` with `0`, empty, or a value outside `[1, 10000]` keeps the input open and shows the inline error `(must be between 1 and 10000)`.
- [ ] `Esc` closes the input with no change.
- [ ] Switching to a different dataset and back restores the page size that was last set for the original dataset.
- [ ] Drill-down into an FK uses the child dataset's own page size; popping back restores the parent's page size.
- [ ] Restarting the binary resets every dataset back to the default (`50`).

### No default COUNT(\*)

- [ ] On default page loads, the executor does **not** issue a `COUNT(*)` query. The query log shows only the data SELECT (with `LIMIT PageSize+1`).
- [ ] Status bar shows `page N` with no `/M` and no rows component while the total is unknown.
- [ ] Paging forward past the last page is impossible: when `HasMore` is false, `]` is a no-op.
- [ ] When `]` lands on the last page (`HasMore == false`), the row browser records the discovered total and the status bar updates to `page N/N  ~T rows` (tilde present).

### First / last page

- [ ] `g` and `Home` in the row browser jump to page 1. No COUNT is issued.
- [ ] `G` and `End` in the row browser run a one-shot `COUNT(*)` (visible in the query log as the only count query) and then load the computed last page.
- [ ] While the count is in flight, the status bar shows `Finding last page...`.
- [ ] Once the count completes, the status bar shows `page N/N  T rows` (no tilde — exact).
- [ ] If the count fails, the status bar shows `goto last failed: <err>` transiently and the current page does not change.
- [ ] The exact total persists across subsequent `[` / `]` paging until a filter, sort, or page size change clears it.

### Core changes

- [ ] `QueryResult.TotalRows` is `*int64`; `QueryResult.TotalPages` is `*int`. Both are `nil` when not computed.
- [ ] `QueryResult.HasMore` is set on every result.
- [ ] `QueryOptions.SkipCount` skips the COUNT goroutine entirely; the executor fetches `PageSize+1` rows and sets `HasMore` accordingly.
- [ ] `QueryOptions.OnlyCount` skips the data SELECT; the result has totals set and an empty `Rows` slice.
- [ ] Setting both `SkipCount` and `OnlyCount` returns an error from `Executor.Query` before any DB calls.
- [ ] All existing dataset/executor tests pass with the pointer-typed totals.

### Keys & help

- [ ] `keys.Map` has `FirstPage`, `LastPage`, `PageSize` bindings.
- [ ] `FullHelp()` groups them with the pagination keys.
- [ ] Help overlay renders `g/home first page`, `G/end last page`, `P page size`.

### Tests

- [ ] `internal/core/dataset/dataset_test.go` — new cases for `SkipCount` (HasMore=true with full page, HasMore=false on partial page, totals nil) and `OnlyCount` (Rows empty, totals set, Columns populated).
- [ ] `internal/core/dataset/dataset_test.go` — `SkipCount` + `OnlyCount` rejected with a clear error.
- [ ] `internal/tui/views/rowbrowser_test.go` — direct `View()` tests for the three status-bar states (unknown / inferred-with-tilde / exact-no-tilde), the page-size input bar rendering, and the `Finding last page...` state.
- [ ] `internal/tui/views/pagesize_test.go` — new file; unit tests for the `PageSizeRegistry` (defaults, set/get, isolation between names).
- [ ] At least one `teatest` smoke test that: opens the row browser → presses `P`, types `25`, Enter → asserts the next data query used `LIMIT 26 OFFSET 0` (via `QueryLog`).
- [ ] At least one `teatest` smoke test that: opens the row browser → presses `G` → asserts a `COUNT(*)` query was logged and the status bar shows `page N/N  T rows` without a tilde.
- [ ] At least one `teatest` smoke test that: opens the row browser → presses `]` until `HasMore == false` → asserts the status bar shows `~T rows` (tilde present).

### Definition of done

- [ ] `make preflight` passes.
- [ ] `/done` checks all pass.

## Files

| File                                       | Change                                                                                                                             |
| ------------------------------------------ | ---------------------------------------------------------------------------------------------------------------------------------- |
| `internal/core/dataset/dataset.go`         | `QueryOptions` gains `SkipCount` and `OnlyCount`. `QueryResult.TotalRows` → `*int64`, `TotalPages` → `*int`. New `HasMore bool`.    |
| `internal/core/dataset/executor.go`        | Branch on `SkipCount` / `OnlyCount`; reject the both-set combination; PageSize+1 probe for `SkipCount`; count-only path for `OnlyCount`. |
| `internal/core/dataset/dataset_test.go`    | Update existing tests for pointer totals; new cases for the two flags.                                                              |
| `internal/core/export/exporter.go`         | Dereference `*TotalPages`. No behaviour change (exporter keeps the count path).                                                     |
| `internal/tui/views/pagesize.go`           | New — `PageSizeRegistry`.                                                                                                          |
| `internal/tui/views/pagesize_test.go`      | New — registry unit tests.                                                                                                         |
| `internal/tui/views/rowbrowser.go`         | New `modePageSizeInput`, new fields for the discovered total, wire `SkipCount` / `OnlyCount`, `P` / `g` / `G` handlers, save and restore totals + page size in `savedLevel`. |
| `internal/tui/views/rowbrowser_test.go`    | Status-bar rendering states; page-size input rendering; total invalidation on filter/sort/page-size change.                         |
| `internal/tui/app.go`                      | Construct and own a `PageSizeRegistry` in `New`; pass it to every `NewRowBrowserModel` call site.                                   |
| `internal/tui/keys/keys.go`                | Add `FirstPage`, `LastPage`, `PageSize` bindings; add to `FullHelp()`.                                                              |
| `internal/tui/views/helpoverlay.go`        | Document the three new bindings.                                                                                                   |
| `internal/tui/views/helpoverlay_test.go`   | Assert the new entries render.                                                                                                     |

## What NOT to Change

- **Default page size value.** Stays `50`. Changing the default is out of scope.
- **Exporter.** Keeps the count path (no `SkipCount`). The exporter walks every page intentionally and benefits from a known total. Do not change its UX or the way it iterates.
- **Filter / sort surface.** No changes to `dataset.Filter`, the six operators, or the filter UX. This task is orthogonal to [filter-search-redesign](filter-search-redesign.md); the two can land in either order.
- **Table-info overlay (`i`).** Already provides a cheap row-estimate path via catalog statistics. Do not merge with this task — different code path, different purpose.
- **Persistence.** Page-size choices are in-memory only. No YAML, no SQLite, no dotfile. Persistent page sizes are a separate, larger task that would touch the config layer.
- **Page-size per datasource or per session.** The grain is per-dataset by name. Two different datasets in the same datasource have independent page sizes.
- **Query-log entries.** The query log records whatever the executor sends — `LIMIT PageSize+1` will show up as-is in `SkipCount` mode. No special formatting.
- **API / web layer.** TUI-only task.
- **`m.result.TotalPages` direct field reads outside the executor.** Every read goes through the new pointer-aware accessor or the `HasMore` field. Don't leave `result.TotalPages == 0` checks lying around — they will compile after the type change but mean nothing useful.

## Definition of Done

See [definition-of-done.md](../definition-of-done.md). All gates must pass.
