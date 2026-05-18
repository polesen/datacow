# Column Picker — Select and Reorder Visible Columns

Add a column picker overlay to the row browser that lets users choose which columns are visible and in what order. The selection is pushed into `QueryOptions` so the executor issues a projected `SELECT` — large JSON or blob columns that are hidden are never fetched from the database. Export respects the same projection.

## Background

Wide tables are common: 20–50 columns, many of which the user doesn't care about. Today there is no way to hide noise columns or promote important ones to the left. Crucially, some tables carry large JSON/blob columns that are expensive to transfer and render. Column projection solves both problems at the query level.

## Architecture

### Core layer — `QueryOptions.Columns`

Add an ordered `Columns []string` field to `QueryOptions`:

```go
// Columns is the ordered list of column names to SELECT.
// nil or empty means SELECT * (all columns, schema order).
Columns []string
```

The executor already validates column names against the schema for filters and sort. Apply the same whitelist check to `Columns`. Column names cannot be parameterized in SQL — they must go through the identifier validator (`identRe`) and be confirmed present in `colSet` before interpolation.

For table datasets, build `SELECT col1, col2, ... FROM table_name ...`.
For SQL datasets, wrap: `SELECT col1, col2, ... FROM (user_sql) AS _dc_dataset ...`.

The `COUNT(*)` query is unaffected — it always counts all rows regardless of projection.

`QueryResult.Columns` will reflect only the projected columns, in the requested order.

### TUI — `ColumnRegistry`

Add a `ColumnRegistry` in `internal/tui/views/` that maps `ds.Name → []ColumnSelection`:

```go
type ColumnSelection struct {
    Name    string
    Visible bool
}
```

`RowBrowserModel` holds a `*ColumnRegistry` (shared across navigation levels, like `PageSizeRegistry`). When a new result arrives for a dataset with no existing registry entry, seed it with all columns visible in schema order. On subsequent fetches, pass the visible columns (in their picker order) as `QueryOptions.Columns`.

The same `ColumnRegistry` entry is used when the user triggers an export — the exporter receives `QueryOptions` with `Columns` already set to the active projection.

### TUI — `ColumnPickerModel`

A new Bubble Tea component in `internal/tui/views/columnpicker.go`. It receives the full column list (from the last result or schema probe) and the current `[]ColumnSelection`. It returns the updated selection on confirm, which triggers a re-fetch.

## UX

### Opening the picker

`C` (uppercase) opens the column picker overlay. Available whenever `result != nil`.

### Overlay layout

```
┌─ Columns ────────────────────────────────┐
│  Space toggle · J/K reorder              │
│  a select all · r reset · Enter confirm  │
│                                           │
│  [✓] id                                   │
│  [✓] name                                 │
│  [ ] payload          (large JSON)        │
│  [✓] status                               │
│  [ ] internal_notes                       │
│  ...                                      │
└───────────────────────────────────────────┘
```

- `↑`/`↓` moves cursor.
- `Space` toggles visibility of the focused column.
- `J`/`K` moves the focused column down/up (reorder).
- `a` selects all columns.
- `r` resets to all-visible, original schema order.
- `Enter` applies, closes, and triggers a re-fetch with the new `QueryOptions.Columns`.
- `Esc` cancels without re-fetching.

At least one column must remain visible. Confirming with zero visible columns shows an inline error `"at least one column required"` and keeps the overlay open.

### Status bar

When a non-default projection is active, the status bar shows `cols: N/M` (visible / total). Not shown when all columns are selected in schema order.

### Persistence

Session-only — not persisted to disk. Each dataset (`ds.Name`) has its own independent column selection, preserved for the lifetime of the TUI session. Navigating away (drill-down, back) does not clear the selection.

## Acceptance Criteria

- [ ] `QueryOptions.Columns` added; executor builds a projected SELECT when non-empty; column names validated against schema before interpolation.
- [ ] Empty/nil `Columns` produces `SELECT *` (no behaviour change for existing callers).
- [ ] Works for both table datasets (direct SELECT) and SQL datasets (wrapping subquery).
- [ ] `C` opens the column picker; `Esc` cancels without re-fetching.
- [ ] `Space` toggles visibility; `J`/`K` reorder; `a` selects all; `r` resets.
- [ ] `Enter` applies and triggers a re-fetch; the row browser reflects the new column set.
- [ ] Confirming with zero visible columns shows `"at least one column required"` and keeps the overlay open.
- [ ] Status bar shows `cols: N/M` when a non-default projection is active.
- [ ] Column selection is preserved when navigating within a session; each dataset is independent.
- [ ] Export (`e`) uses the active column projection — only selected columns appear in the CSV/Excel output.
- [ ] `C` key added to `keys.Map`, wired in `Update()`, listed in `helpoverlay.go`.
- [ ] Core tests: `QueryOptions.Columns` produces correct SQL for table and SQL datasets; validated against schema; empty columns falls back to `SELECT *`.
- [ ] `ColumnPickerModel` view unit tests: checked/unchecked rendering, zero-column error.
- [ ] App integration test: open picker with `C`, hide a column, confirm, verify column header is absent from rendered output and the re-fetch SQL contains only the projected columns (via query log).

## What NOT to Change

- Filter and sort logic — these operate on the full column set; `QueryOptions.Filters` and `QueryOptions.Sort` are validated against the full schema, not the projection.
- `PageSizeRegistry` — no structural changes; `ColumnRegistry` follows the same pattern independently.
- The `COUNT(*)` subquery — always counts all rows, unaffected by projection.

## Definition of Done

See `tasks/definition-of-done.md`. Invoke `/done` after all acceptance criteria are met.
