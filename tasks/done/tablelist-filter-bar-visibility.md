# TUI: Filter Bar Visibility — Tables Pane

Make it impossible to miss that the tables pane is showing a filtered view.
Today the filter bar disappears once you press Enter (filter held, input
closed), leaving only a muted line in the shared app status bar that is easy
to overlook. This task fixes that with three targeted changes: always-visible
filter bar inside the pane, a brief attention-grabbing flash at the two
moments the user needs it, and right-aligned shortcuts in the status bar that
no longer shift as filter text grows.

## Background

The filter feature itself (`/` in the tables pane) already works correctly.
The problem is purely visual: once the filter is committed the pane gives no
strong indication that you are looking at a subset. A user who tabs away and
comes back may not notice the filter is still active.

The row browser handles local search better — `StatusLine()` stays in the
status bar on the left and the bar uses a split layout so shortcuts are always
right-anchored. The tables pane should converge on the same principles.

## Behaviour

### 1. Filter bar always visible inside the pane

`TableListModel.View()` currently renders a filter bar footer only when
`filterInputOpen`. Extend this so the footer is rendered whenever a filter is
active (`filterInputOpen || filterQuery != ""`), using two distinct render
modes:

- **Input open** (`filterInputOpen == true`): render the text input as today.
- **Filter held** (`filterInputOpen == false && filterQuery != ""`): render a
  compact read-only status line in the same footer slot:

  ```
  / "customers"  4/23  ·  / edit  ·  esc clear
  ```

  Style this with a new `FilterBarHeld` style (see Files). The style must
  visually pop — a warm amber/orange background (`#E0AF68` dark / `#B8670A`
  light, dark foreground text both modes) so the bar reads as an alert, not
  decoration.

- `listHeight` must be reduced by 1 whenever either mode is active (as it
  already is for `filterInputOpen`) so the bar never overlaps list content.

### 2. Flash on commit and focus gain

Add a `filterFlashing bool` field to `TableListModel`. When `filterFlashing`
is true, the held-filter bar (mode 2 above) renders with `FilterBarFlash`
style instead of `FilterBarHeld`. `FilterBarFlash` is a brighter, more
saturated version of the same amber — e.g. `#FF9E64` background, bold text.

Trigger the flash in two situations:

**A. Filter committed** — when the user presses Enter while the filter input
is open (transitioning `filterInputOpen` → false). At that point:
  1. Set `filterFlashing = true`.
  2. Return `tea.Tick(400*time.Millisecond, func(time.Time) tea.Msg { return filterFlashExpiredMsg{} })`.

**B. Pane gains focus with a held filter** — add a method:
  ```go
  func (m TableListModel) OnFocusGained() (TableListModel, tea.Cmd)
  ```
  If `filterQuery != ""`, set `filterFlashing = true` and return the same
  400ms tick. If no filter is active, return `m, nil`.

  Call `m.tableList.OnFocusGained()` in `app.go` wherever focus is
  assigned to `focusTables` (Tab, Shift+Tab, key `1`, and the initial
  panel layout).

Handle `filterFlashExpiredMsg` in `TableListModel.Update()`: set
`filterFlashing = false`.

### 3. Status bar: right-aligned shortcuts, no filter status in parts

In `app.go renderStatusBar()`:

- **Remove** the `FilterStatus()` block (lines 958–961). The filter status is
  now shown inside the pane — it no longer belongs in the shared status bar.

- **Change layout** for the tablelist branch (`a.focus == focusTables`): use
  the same left+gap+right split as `renderRowBrowserStatusBar` instead of
  `strings.Join(parts, "  ")`. The left side carries `runningPart` (spinner /
  schema-loading indicator) or is empty; the right side carries the
  `TableListHelp()` bindings, right-anchored. This matches the row browser and
  prevents any future text on the left from pushing shortcuts right.

## Files

| File | Change |
|---|---|
| `internal/tui/style/style.go` | Add `FilterBarHeld` (amber background, dark text) and `FilterBarFlash` (brighter amber, bold). |
| `internal/tui/views/tablelist.go` | Add `filterFlashing bool`; add `filterFlashExpiredMsg` type; extend `View()` to always render footer when filter active; implement flash render path; add `OnFocusGained()`; handle `filterFlashExpiredMsg` in `Update()`. |
| `internal/tui/views/tablelist_test.go` | Add view tests: held-filter footer present, input-open footer present, flash style toggled, footer absent when no filter. |
| `internal/tui/app.go` | Remove `FilterStatus()` from status bar parts; switch tablelist status bar branch to left+gap+right layout; call `OnFocusGained()` at all focus-assignment sites. |
| `internal/tui/app_test.go` | Add smoke test: after committing a filter and tabbing away and back, the tablelist view contains the held-filter bar text. |

## Acceptance Criteria

- [ ] Pressing `/`, typing a query, and pressing Enter leaves a visible footer bar at the bottom of the tables pane showing `/ "<query>"  M/N` in a warm amber background.
- [ ] The footer bar is present regardless of whether the tables pane is focused.
- [ ] When the filter input is open (`/` pressed), the footer shows the editable text input as before.
- [ ] When no filter is active, no footer line is rendered (list uses full height).
- [ ] Pressing Enter to commit the filter causes the footer bar to flash to a brighter style for ~400ms, then settle to the calm amber.
- [ ] Tabbing away from the tables pane and back while a filter is held triggers the same 400ms flash.
- [ ] The app status bar no longer shows `filter: "X"  M/N` text — it is not duplicated between pane and status bar.
- [ ] The app status bar shortcut keys for the tables pane are right-aligned and do not shift when typing into the filter input.
- [ ] All existing filter behaviour (typing narrows list, Esc clears, re-open pre-fills, cursor restore) is unchanged.
- [ ] `make preflight` and `/done` pass.

## What NOT to Change

- **Filter logic** — `computeFilter`, `applyFilter`, `clearFilter`, `visibleDatasetIndices`, cursor restore. Do not touch match behaviour.
- **Row browser local search** — `localsearch.go`, `rowbrowser.go`. Separate feature, different component.
- **`FilterStatus()` method** — it may still be called from tests; leave the method, just stop calling it from the status bar.
- **`TableListHelp()` bindings** — do not add, remove, or reorder entries. Layout change only.
- **Schema cache, schema explorer, cell viewer, query log, export, FK drill-down** — unrelated.
- **API / web layer** — TUI-only task.

## Definition of Done

See [definition-of-done.md](../definition-of-done.md). All gates must pass.
