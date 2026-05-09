# TUI: Query Log System Filter

The query log currently shows all queries — both user-initiated data queries and
datacow-internal system queries (schema introspection, COUNT pagination, FK lookups).
Most users only care about the queries that ran against their data. System queries
are noise for everyday use but useful when debugging datacow itself.

## What to build

Add a toggle to the full-screen query log (`L`) that filters between:
- **User only** (default) — hide system queries; show only `QueryKindUser` entries
- **All** — show everything, labelled with `user` / `system` badges as today

### Behaviour

- Default state on open: user-only (system queries hidden).
- Press `s` to toggle. The filter state persists while the view is open; resets to
  user-only each time the view is opened.
- The history header line must reflect the current filter:
  - Default: `History  (newest first)  [user only — s: show all]`
  - Toggled: `History  (newest first)  [all queries — s: user only]`
- Cursor and scroll offset reset to 0 when the filter is toggled (the list length
  changes, keeping the old cursor position would be confusing).
- The SQL preview section is unaffected — it always shows the selected entry's full
  details regardless of its kind.

### Empty state

If the filtered list is empty (e.g. user-only mode but no user queries yet), show
the existing `(none)` placeholder.

## Files to modify

| File | Change |
|---|---|
| `internal/tui/views/querylog.go` | Add `showSystem bool` field; filter history slice before rendering; update header; handle `s` key |
| `internal/tui/views/querylog_test.go` | Tests (see below) |

No changes to keys, app.go, help overlay, or style — `s` is local to the query log
view and needs no global keybinding registration.

## Implementation notes

- Filter the history slice at the top of `View()` (or in `Update()` on toggle),
  not in the data model. `QueryLog.History()` always returns everything; the view
  decides what to render.
- Resetting cursor/scroll on toggle: set `v.cursor = 0` and recalculate scroll
  offset in the next `View()` call (the scroll offset is already derived from
  cursor in `View()`).
- The compact SQL pane (`sqlpane.go`) is **not** affected — it always shows all
  entries (it is a live feed, not an analysis view).

## Tests

Add to `internal/tui/views/querylog_test.go`:

1. **`TestQueryLogView_DefaultHidesSystemQueries`** — add one user query and one
   system query; in a fresh view the system query must not appear in the output.
2. **`TestQueryLogView_ToggleShowsSystemQueries`** — press `s`; now both entries
   must appear in the output.
3. **`TestQueryLogView_ToggleBackHidesSystem`** — press `s` twice; system entry
   must be hidden again.
4. **`TestQueryLogView_HeaderReflectsFilterState`** — verify header contains
   `"user only"` by default and `"all queries"` after pressing `s`.
5. **`TestQueryLogView_EmptyAfterFilter`** — add only system queries; user-only
   mode must show `(none)` placeholder.

To add a system entry via `stubClient`/`LoggingClient`, call `addQuery` with the
`_dc_count` SQL pattern (e.g. `"SELECT COUNT(*) AS _dc_count FROM t"`) — that path
is already classified as `QueryKindSystem` by `kindFromSQL`.

## Definition of Done

```bash
make preflight
go build ./...
gotestsum --format testdox ./...
staticcheck ./...
make lint
```

Manual:
1. Open any table → press `L` → header shows `[user only — s: show all]`, system
   queries (describe, list tables, FK lookups, count) are absent from the list.
2. Press `s` → header changes to `[all queries — s: user only]`, system entries
   appear with `system` badge.
3. Press `s` again → back to user-only.
4. Navigate to a fresh datasource with no user queries yet → user-only mode shows
   `(none)`.
