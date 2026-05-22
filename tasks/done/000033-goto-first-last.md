# `g` / `G` Goto First / Last — Tables Pane and Row Browser

Extend `g` / `G` so the same two keys mean "jump to first / last item" in both
the tables pane and the row browser. The tables pane currently has no binding;
add one that selects the first / last dataset in the visible list. The row
browser already binds `g` / `G` to first / last *page*, but the cursor still
lands on row 0 after the page load — change `G` so the cursor lands on the
*last row* of the last page, and tighten `g` so it always snaps the cursor to
row 0 even when already on page 1.

## Background

The product mental model is "g/G means jump to the first/last thing in this
list". In the row browser today, pressing `G` correctly loads the last page —
but the cursor lands on row 0 of that page, so the user has to press `j`
repeatedly to actually see the last row. In the tables pane there is no
g/G binding at all; the user has to hold `j` to scroll to the bottom of a
long schema. Both gaps break the muscle-memory promise of g/G.

## Behaviour

### 1. Tables pane: `g` / `G` select first / last visible dataset

`TableListModel.Update()` (normal mode, input *not* open) gains two new branches
parallel to the existing `Up` / `Down` cases:

- `FirstPage` (`g`/`home`): set `m.cursor` to the first index in
  `m.visibleDatasetIndices()`, then `m = m.ensureCursorVisible()`.
- `LastPage` (`G`/`end`): set `m.cursor` to the *last* index in
  `m.visibleDatasetIndices()`, then `m = m.ensureCursorVisible()`.

If `visibleDatasetIndices()` is empty (e.g. a filter with zero matches), both
keys are no-ops.

The same two branches are added inside the `m.filterInputOpen` block, mirroring
how `Up` / `Down` are routed there today — pressing `g` / `G` while the
filter input is open should still type the character into the text input, so
**only the `home` / `end` aliases** trigger first/last selection in that mode;
the bare `g` / `G` keys must fall through to the textinput. (Implementation
note: split the key match on `msg.String() == "home"` / `"end"` inside the
input-open block, or check `key.Matches(msg, m.keys.FirstPage) && msg.Type ==
tea.KeyHome` style — choose whichever keeps the existing typing behaviour
intact and is covered by tests.)

### 2. Row browser: `g` snaps cursor to first row even on page 1

Today `FirstPage` early-returns when `m.result.Page == 1`. Change it to always
snap `m.rowCursor = 0` and `m.rowOffset = 0` before the early return (or in
addition to the page load):

```go
case key.Matches(msg, m.keys.FirstPage):
    m.rowCursor = 0
    m.rowOffset = 0
    if m.result.Page != 1 {
        m = m.clearLocalSearch()
        m.loading = true
        return m, tea.Batch(m.spinner.Tick, m.loadPageCmd(1))
    }
```

When a page load happens, the existing `applyLoadedPage` path already resets
`rowCursor = 0` / `rowOffset = 0` (rowbrowser.go:363–364), so no further
change is needed for the load branch.

### 3. Row browser: `G` lands cursor on last row of last page

`LastPage` currently issues `loadCountCmd()`, and the `countLoadedMsg` handler
calls `loadPageCmd(lastPage)`. After the page arrives, the cursor sits on row 0.

Change the `G` flow so the cursor lands on the *last* row of the loaded page.
Two options — pick whichever lands cleaner in review:

- **Option A (preferred):** Add a `pendingLastRow bool` field to
  `RowBrowserModel`. Set it to `true` in the `countLoadedMsg` handler
  immediately before dispatching `loadPageCmd(lastPage)`. In the
  `applyLoadedPage` path (rowbrowser.go ~360), after the existing
  `pendingRowCursor` block, add:

  ```go
  if m.pendingLastRow {
      m.pendingLastRow = false
      m.rowCursor = max(0, len(r.Rows)-1)
      if vis := m.visibleRowCount(); vis > 0 {
          m.rowOffset = max(0, m.rowCursor-vis+1)
      }
  }
  ```

- **Option B:** Set `m.pendingRowCursor = &sentinel` where `sentinel` is
  `math.MaxInt` (or `len(rows)-1` if we already know page size from the COUNT
  result). The existing clamp at rowbrowser.go:368–370 already trims to
  `len(r.Rows)-1`. Adjust the offset calculation in the same block to use
  `max(0, m.rowCursor-vis+1)` instead of `max(0, m.rowCursor-vis/2)` so the
  last row sits at the bottom of the viewport (consistent with `j` reaching
  the end).

  Option B is smaller code but conflates two flows (page-size restore +
  goto-last) onto one field. Prefer Option A unless review suggests otherwise.

For both options, also handle the edge case where the user is **already on the
last page** when `G` is pressed: `loadCountCmd` will still run, but if
`*m.knownTotalPages == m.result.Page` we can short-circuit and just snap the
cursor to the last row of the current page without a reload. Implement the
short-circuit inside the `LastPage` key handler, before calling
`loadCountCmd()`:

```go
case key.Matches(msg, m.keys.LastPage):
    if m.knownTotalExact && m.knownTotalPages != nil && *m.knownTotalPages == m.result.Page {
        m.rowCursor = max(0, len(m.result.Rows)-1)
        if vis := m.visibleRowCount(); vis > 0 {
            m.rowOffset = max(0, m.rowCursor-vis+1)
        }
        return m, nil
    }
    m.statusMsg = "Finding last page..."
    return m, m.loadCountCmd()
```

### 4. Help text

Update the `FirstPage` / `LastPage` `key.WithHelp` strings in
`internal/tui/keys/keys.go` to reflect the new dual meaning:

- `FirstPage`: `"g/home"`, `"first row"` (was `"first page"`)
- `LastPage`: `"G/end"`, `"last row"` (was `"last page"`)

Add `FirstPage` and `LastPage` to `TableListHelp()` so they appear in the
tables-pane status bar:

```go
func (m Map) TableListHelp() []key.Binding {
    return []key.Binding{m.Quit, m.Up, m.Down, m.FirstPage, m.LastPage, m.Enter, m.TableListFilter, m.TableInfo, m.SwitchFocus}
}
```

The help overlay (`internal/tui/views/helpoverlay.go`) already lists
`FirstPage` / `LastPage` under the Navigation group — no change there beyond
the inherited help-text update.

## Files

| File | Change |
|---|---|
| `internal/tui/views/tablelist.go` | Add `FirstPage` / `LastPage` key handlers in both normal mode and `filterInputOpen` mode (home/end only when input is open). |
| `internal/tui/views/rowbrowser.go` | `FirstPage`: always snap `rowCursor`/`rowOffset` to 0. `LastPage`: short-circuit when already on the last known page; otherwise set `pendingLastRow = true` before the COUNT-driven load. `applyLoadedPage`: consume `pendingLastRow` and place cursor at `len(rows)-1` with viewport scrolled to keep it visible. Add `pendingLastRow bool` field. |
| `internal/tui/keys/keys.go` | Update `FirstPage` / `LastPage` help text to "first row" / "last row". Add both bindings to `TableListHelp()`. |
| `internal/tui/views/tablelist_test.go` | New tests: `g` selects first visible dataset; `G` selects last visible dataset; both respect an active filter; both are no-ops on empty list; `g` / `G` inside filter input still type the character (only `home` / `end` trigger jump there). |
| `internal/tui/views/rowbrowser_test.go` | New tests: `g` on page 1 snaps cursor from a mid-page position to row 0; `G` short-circuit on last page snaps cursor to last row without reload; `G` from earlier page lands on last row of last page after load. |
| `internal/tui/app_test.go` or `tablelist_acceptance_test.go` | One teatest smoke covering `g` / `G` in the tables pane end-to-end. |

## Acceptance Criteria

- [ ] In the tables pane with focus, pressing `g` moves the selection to the topmost visible dataset in the list.
- [ ] In the tables pane with focus, pressing `G` moves the selection to the bottommost visible dataset in the list.
- [ ] When a table-list filter is held, `g` / `G` jump to the first / last *visible* (filtered) match, not the first / last underlying dataset.
- [ ] When the table-list filter input is open and the user types `g` or `G`, the letter is inserted into the input (filter narrows accordingly); only `home` / `end` jump to first / last while the input is open.
- [ ] In the row browser, pressing `g` always places the row cursor on the first row of page 1 — including when the browser is already showing page 1 with the cursor on a different row.
- [ ] In the row browser, pressing `G` places the row cursor on the *last* row of the last page (not row 0 of the last page), with the viewport scrolled so that row is visible at the bottom.
- [ ] If the row browser is already on the last page (`knownTotalPages == result.Page` and total is exact), `G` snaps the cursor to the last row of the current page without issuing a new query.
- [ ] The status bar help text under `g` / `G` reads "first row" / "last row" (not "first page" / "last page").
- [ ] The tables-pane status bar shows `g/home` and `G/end` as available shortcuts.
- [ ] `make preflight` and `/done` pass.

## What NOT to Change

- **Page-load mechanics** — `loadPageCmd`, `loadCountCmd`, `applyLoadedPage`, `countLoadedMsg` plumbing. Reuse them; do not rewrite pagination.
- **Local search navigation** — when `localSearch.IsActive()`, `↓` / `↑` still walk match-to-match. `g` / `G` do not need to interact with local search; if a search is held, `g` / `G` clear it via the existing `clearLocalSearch()` only on the page-load branch (same as today's `FirstPage` path) and otherwise leave it alone.
- **`pendingRowCursor`** — leave the existing field and its single caller (`applyPageSizeInput`) intact. Add a separate `pendingLastRow` field rather than overloading it (Option A above).
- **Row browser `↓` / `↑` / `[` / `]` semantics** — unchanged.
- **Schema explorer, query log, cell viewer, sort manager, column picker, save perspective, SQL editor** — unrelated.
- **API / web layer** — TUI-only task.

## Definition of Done

See [definition-of-done.md](../definition-of-done.md). All gates must pass.
