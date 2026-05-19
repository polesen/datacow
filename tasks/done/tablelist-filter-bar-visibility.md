# TUI: Filter Bar Visibility — Tables Pane and Row Browser

Make it impossible to miss that a filter is active, in both the tables pane and the
row browser. Today the filter bars disappear once the input is closed, leaving only a
muted line in the shared app status bar that is easy to overlook. This task fixes that
with always-visible pane-level bars, brief attention-grabbing flashes at the two moments
the user needs them, right-aligned shortcuts in the status bar, and filter state that
persists until the user explicitly clears it.

## Background

The filter feature itself (`/` in the tables pane, `/` in the row browser) already works
correctly. The problem is purely visual: once a filter is committed the pane gives no
strong indication that you are looking at a subset. A user who tabs away and comes back
may not notice the filter is still active — made worse by the old design, which also
cleared the filter silently on any focus change.

## Behaviour

### 1. Tables pane: filter bar always visible inside the pane

`TableListModel.View()` renders a filter bar footer only when `filterInputOpen`.
Extended so the footer is rendered whenever a filter is active
(`filterInputOpen || filterQuery != ""`), using two distinct render modes:

- **Input open** (`filterInputOpen == true`): render the text input as before.
- **Filter held** (`filterInputOpen == false && filterQuery != ""`): render a
  compact read-only status line in the same footer slot:

  ```
  / "customers"  4/23  ·  / edit  ·  esc clear
  ```

  Styled with `FilterBarHeld` (warm amber/orange background, dark foreground text
  both modes) so the bar reads as an alert, not decoration.

- `listHeight` reduced by 1 whenever either mode is active so the bar never overlaps
  list content.

### 2. Row browser: local search bar always visible when held

`RowBrowserModel.View()` previously showed a `FilterBar` at the bottom only while the
search input was active. Extended so a held search (input closed, query set) also renders
a full-width `FilterBarHeld` bar at the bottom:

```
search: "alice"  2/3 matches  ·  n next  ·  N prev  ·  esc clear
```

`StatusLine()` is updated to only return search status while the input is actively open
(not when held), so the status bar shows regular page info once the search is committed.
Both `visibleRowCount()` and the per-render `bottomBarLines` count reserve space for the
held bar identically to the open input.

### 3. Flash on commit and focus gain — both panes

Both `TableListModel` and `RowBrowserModel` have a `filterFlashing` / `localSearchFlashing`
bool. When true the held bar renders with `FilterBarFlash` instead of `FilterBarHeld`.
`FilterBarFlash` is a brighter, more saturated amber — `#FF9E64` background, bold text.

**Trigger: filter committed** — Enter while the filter/search input is open:
1. Set `*Flashing = true`.
2. Return `tea.Tick(400ms, func(time.Time) tea.Msg { return *FlashExpiredMsg{} })`.

**Trigger: pane gains focus with a held filter/search** — both models have `OnFocusGained()`:
```go
func (m TableListModel) OnFocusGained() (TableListModel, tea.Cmd)
func (m RowBrowserModel) OnFocusGained() (RowBrowserModel, tea.Cmd)
```
If a filter/search is held, sets flashing and returns the 400ms tick. If nothing is
held, returns `m, nil`.

`app.go` calls `OnFocusGained()` wherever focus is assigned to `focusTables` or
`focusRowBrowser` (Tab, Shift+Tab, keys `1`/`2`, Left/Esc back from row browser).

Handle `*FlashExpiredMsg` in each model's `Update()`: reset flashing to false.

### 4. Filter persists across focus changes

Previously, any navigation away from the tables pane (Tab, Shift+Tab, keys `2`/`3`,
Enter/Right into a table, `DatasetSelectedMsg`) called `ClearFilter()`. All six call
sites have been removed. The filter now persists in `TableListModel` state until the user
explicitly presses Esc.

### 5. Status bar: right-aligned shortcuts, no filter status in parts

In `app.go renderStatusBar()`:

- **Removed** the `FilterStatus()` block. The filter status is now shown inside the
  pane — it no longer belongs in the shared status bar.

- **Changed layout** for the tablelist branch (`a.focus == focusTables`): uses the same
  left+gap+right split as `renderRowBrowserStatusBar` instead of `strings.Join(parts, "  ")`.
  The left side carries `runningPart` (spinner / schema-loading indicator) or is empty;
  the right side carries the `TableListHelp()` bindings, right-anchored.

## Files

| File | Change |
|---|---|
| `internal/tui/style/style.go` | Add `FilterBarHeld` (amber background, dark text) and `FilterBarFlash` (brighter amber, bold). |
| `internal/tui/views/tablelist.go` | Add `filterFlashing bool`; add `filterFlashExpiredMsg` type; extend `View()` to always render footer when filter active; implement flash render path; add `OnFocusGained()`; handle `filterFlashExpiredMsg` in `Update()`. |
| `internal/tui/views/rowbrowser.go` | Add `localSearchFlashing bool`; add `localSearchFlashExpiredMsg` type; extend `View()` to render held-search amber bar; implement flash render path; add `OnFocusGained()`; handle `localSearchFlashExpiredMsg` in `Update()`; update `StatusLine()` and `visibleRowCount()`. |
| `internal/tui/views/tablelist_test.go` | Add view tests: held-filter footer present, input-open footer present, flash style toggled, footer absent when no filter, OnFocusGained with/without filter, filter persists after OnFocusGained. |
| `internal/tui/views/rowbrowser_test.go` | Add tests: held search bar visible, flash on commit, OnFocusGained flashes, OnFocusGained no-op when no search, StatusLine shows page info (not search) when held. |
| `internal/tui/views/export_test.go` | Export `FilterFlashExpiredMsgForTest`, `LocalSearchFlashExpiredMsgForTest`, `IsLocalSearchFlashing()` for external test packages. |
| `internal/tui/app.go` | Remove all six `ClearFilter()` calls from focus-switch paths; call `OnFocusGained()` at all `focusTables` and `focusRowBrowser` assignment sites; switch tablelist status bar branch to left+gap+right layout; remove `FilterStatus()` from status bar parts. |
| `internal/tui/app_test.go` | Replace `TestAC_B10_LeavingPaneClearsFilter` with `TestAC_B10_FilterPersistsAcrossFocusChanges`; update `TestApp_TableListFilter_HeldBarVisibleAfterTabAway` to test persistence instead of clear. |
| `internal/tui/views/tablelist_acceptance_test.go` | Update B10 comment (now documents persistence, not clearing); update B11 comment. |

## Acceptance Criteria

- [ ] Pressing `/`, typing a query, and pressing Enter leaves a visible footer bar at the bottom of the tables pane showing `/ "<query>"  M/N` in a warm amber background.
- [ ] The tables pane footer bar is present regardless of whether the tables pane is focused.
- [ ] When the tables filter input is open (`/` pressed), the footer shows the editable text input as before.
- [ ] When no filter is active in the tables pane, no footer line is rendered (list uses full height).
- [ ] Pressing Enter to commit the tables filter causes the footer bar to flash to a brighter style for ~400ms, then settle to the calm amber.
- [ ] Tabbing away from the tables pane and back while a filter is held triggers the same 400ms flash.
- [ ] The tables pane filter persists across all focus changes (Tab, Shift+Tab, keys `2`/`3`, Enter into a table, back from row browser) until Esc is explicitly pressed.
- [ ] The row browser local search (`/`) shows a held amber bar at the bottom after Enter, displaying the query, match count, and `n next · N prev · esc clear` hints.
- [ ] Committing the row browser search (Enter) causes the same 400ms flash on the held bar.
- [ ] Switching focus to the row browser while a search is held triggers the same 400ms flash.
- [ ] The app status bar no longer shows `filter: "X"  M/N` text — it is not duplicated between pane and status bar.
- [ ] The app status bar shortcut keys for the tables pane are right-aligned and do not shift when typing into the filter input.
- [ ] All existing filter/search behaviour (typing narrows list, Esc clears, re-open pre-fills, cursor restore) is unchanged.
- [ ] `make preflight` and `/done` pass.

## What NOT to Change

- **Filter logic** — `computeFilter`, `applyFilter`, `clearFilter`, `visibleDatasetIndices`, cursor restore. Do not touch match behaviour.
- **`FilterStatus()` method** — it may still be called from tests; leave the method, just stop calling it from the status bar.
- **`TableListHelp()` bindings** — do not add, remove, or reorder entries. Layout change only.
- **Schema cache, schema explorer, cell viewer, query log, export, FK drill-down** — unrelated.
- **API / web layer** — TUI-only task.

## Implementation Notes

- `StatusLine()` in `RowBrowserModel` was changed to only return search status when
  the input is actively open (`InputActive()`), not when the search is held. This lets
  the status bar show regular page info once the search is committed — the pane-level
  bar is the canonical filter indicator.
- The six `ClearFilter()` call sites removed from `app.go` were originally added in
  the tablelist-filter-search task. The original design treated the filter as transient;
  the revised design treats it as durable state owned by the model.
- teatest's accumulated-bytes model makes it impossible to prove filter ABSENCE
  conclusively. `TestAC_B10_FilterPersistsAcrossFocusChanges` uses the reverse of the
  old "xfresh" trick: after Tab → "1" → "/", typing a suffix "Z" and committing gives
  `"filter_b10cZ"` if the filter persisted (pre-filled), or `"Z"` if cleared. WaitFor
  `"filter_b10cZ"` is the positive assertion.

## Definition of Done

See [definition-of-done.md](../definition-of-done.md). All gates must pass.
