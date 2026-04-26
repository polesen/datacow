# TUI: Maximize Pane

Press `z` to zoom the currently focused pane to full screen. Press `z` again — or `esc` — to return to the split layout.

Modelled on tmux `prefix z` and vim `ctrl+w z`: instant, reversible, zero modals.

## UX

```
Normal split:
╭─ 1 Tables ──────╮╭─ 2 orders ───────────────────────────╮
│ orders      12k  ││ id  │ user_id │ total                 │
│ order_items  67k ││  1  │  42     │ 99.99                 │
│ users        1k  ││  2  │  17     │ 14.50                 │
╰──────────────────╯╰───────────────────────────────────────╯
╭─ 3 SQL ──────────────────────────────────────────────────╮
│ SELECT * FROM orders LIMIT 50 OFFSET 0              12ms  │
╰──────────────────────────────────────────────────────────╯

Press z on pane 2:
╭─ 2 orders ───────────────────────────────────────────────╮
│ id  │ user_id │ created_at          │ total  │ status     │
├─────┼─────────┼─────────────────────┼────────┼────────────┤
│  1  │  42     │ 2024-01-15 09:23:11 │  99.99 │ shipped    │
│  2  │  17     │ 2024-01-15 11:47:02 │  14.50 │ pending    │
╰──────────────────────────────────────────────────────────╯
z restore  esc restore  q quit  [ prev  ] next  / filter  ...
```

- `z` in split view zooms the focused pane to full terminal width × full content height
- `z` again restores the split
- `esc` also restores the split when maximized — but only when the pane itself has no
  use for `esc` (e.g. no FK drill to collapse, no filter active)
- Pane number keys `1` / `2` / `3` while maximized switch _which_ pane is maximized
  (do not exit maximized mode first)
- When focus is on pane 3 (SQL), `z` opens the existing full-screen query log view —
  same as pressing `L` — because the query log view is already the right fullscreen
  treatment for that pane
- Header and status bar remain visible at all times (only content area changes)
- Border and panel title render at full terminal width
- Status bar shows `z restore` while any pane is maximized

## Architecture

Only two files change. No new files.

### `internal/tui/keys/keys.go`

Add one binding to `Map`:
```go
Maximize key.Binding  // z — maximize/restore focused pane
```

Default:
```go
Maximize: key.NewBinding(
    key.WithKeys("z"),
    key.WithHelp("z", "maximize"),
),
```

Add `Maximize` to `ShortHelp()` only when it would be contextually useful — the
status bar handles this via the new maximized rendering branch described below.

### `internal/tui/app.go`

**New field on `App`:**
```go
maximized bool
```

**New size helpers** (add alongside the existing `maximizedH`/`modelH` family):
```go
func (a *App) maximizedPanelH() int {
    h := a.contentHeight() - 2  // subtract panel borders
    if h < 1 { h = 1 }
    return h
}

func (a *App) maximizedPanelInnerW() int {
    w := a.width - 2  // subtract panel borders
    if w < 1 { w = 1 }
    return w
}
```

**New helpers `pushMaximizedSizes()` and `pushNormalSizes()`:**

Extract the repeated WindowSizeMsg fan-out from the existing `WindowSizeMsg` handler
into `pushNormalSizes()`, then call that from both the resize handler and the
`maximized = false` paths:

```go
func (a *App) pushNormalSizes() {
    a.tableList, _ = a.tableList.Update(
        tea.WindowSizeMsg{Width: a.leftInnerW(), Height: a.modelH()})
    if a.rowBrowserReady {
        a.rowBrowser, _ = a.rowBrowser.Update(
            tea.WindowSizeMsg{Width: a.rightInnerW(), Height: a.modelH()})
    }
    a.sqlPane, _ = a.sqlPane.Update(
        tea.WindowSizeMsg{Width: a.sqlInnerW(), Height: sqlPaneContentH})
}

func (a *App) pushMaximizedSizes() {
    switch a.focus {
    case focusTables:
        a.tableList, _ = a.tableList.Update(
            tea.WindowSizeMsg{Width: a.maximizedPanelInnerW(), Height: a.maximizedPanelH()})
    case focusRowBrowser:
        if a.rowBrowserReady {
            a.rowBrowser, _ = a.rowBrowser.Update(
                tea.WindowSizeMsg{Width: a.maximizedPanelInnerW(), Height: a.maximizedPanelH()})
        }
    }
    // focusSQL never reaches pushMaximizedSizes — it goes to screenQueryLog instead.
}
```

**`Update()` — key handling in the `screenSplit` block:**

Add the `z` handler early in the `screenSplit` key block, before the per-focus `esc` handlers. Guard it with `!inFilterInput` (same as the `1`/`2`/`3` guards):

```go
if !inFilterInput && key.Matches(msg, a.keys.Maximize) {
    if a.focus == focusSQL {
        // Pane 3: reuse the full-screen query log view.
        a.screenBeforeOverlay = a.screen
        a.screen = screenQueryLog
        a.queryLogView, _ = a.queryLogView.Update(
            tea.WindowSizeMsg{Width: a.width - 1, Height: a.contentHeight()})
        return a, nil
    }
    a.maximized = !a.maximized
    if a.maximized {
        a.pushMaximizedSizes()
    } else {
        a.pushNormalSizes()
    }
    return a, nil
}
```

**Pane number keys while maximized:** when `a.maximized` is true and `1`/`2` is
pressed, switch focus AND call `pushMaximizedSizes()` so the newly focused model gets
the full-width size:

```go
if !inFilterInput {
    switch msg.String() {
    case "1":
        a.focus = focusTables
        if a.maximized { a.pushMaximizedSizes() }
        return a, nil
    case "2":
        a.focus = focusRowBrowser
        if a.maximized { a.pushMaximizedSizes() }
        return a, nil
    case "3":
        if a.maximized {
            // Exit maximized, then show query log (same as pressing L).
            a.maximized = false
            a.pushNormalSizes()
        }
        a.focus = focusSQL
        return a, nil
    }
}
```

**`esc` handlers — modify to respect maximized:**

Modify the row browser back handler (no drill to collapse) to exit maximized instead
of switching focus:

```go
if a.focus == focusRowBrowser && key.Matches(msg, a.keys.Back) &&
    a.rowBrowserReady && !a.rowBrowser.NeedsBackKey() {
    if a.maximized {
        a.maximized = false
        a.pushNormalSizes()
    } else {
        a.focus = focusTables
    }
    return a, nil
}
```

Add a maximized-esc handler for the table list (before the existing multi-datasource check):

```go
if a.focus == focusTables && key.Matches(msg, a.keys.Back) && a.maximized {
    a.maximized = false
    a.pushNormalSizes()
    return a, nil
}
```

**`WindowSizeMsg` handler:** when a resize arrives while `a.maximized` is true, push
maximized sizes to the focused pane instead of (or in addition to) normal split sizes.
The non-focused models still need their normal sizes in case the user restores the split:

```go
case tea.WindowSizeMsg:
    a.width = msg.Width
    a.height = msg.Height
    a.datasourcePicker, _ = a.datasourcePicker.Update(
        tea.WindowSizeMsg{Width: a.leftInnerW(), Height: a.modelH()})
    if a.maximized {
        a.pushMaximizedSizes()
        // Also keep non-focused models at their normal sizes for when the split is restored.
        // (pushMaximizedSizes only updates the focused model; call pushNormalSizes for the rest
        // by temporarily toggling maximized — or just do it inline.)
    } else {
        a.pushNormalSizes()
    }
    a.sqlPane, _ = a.sqlPane.Update(tea.WindowSizeMsg{Width: a.sqlInnerW(), Height: sqlPaneContentH})
    // ... existing query log / cell viewer size pushes unchanged ...
```

Simplest correct approach: when `maximized`, call `pushNormalSizes()` first (all models
get their split sizes), then call `pushMaximizedSizes()` (the focused model gets the
override). This ensures non-focused models are always at a sane size for split restore.

**`renderContent()` — new maximized branch:**

```go
case screenSplit:
    if a.maximized {
        return a.renderMaximizedContent()
    }
    return a.renderSplitContent()
```

**New `renderMaximizedContent()`:**

```go
func (a *App) renderMaximizedContent() string {
    switch a.focus {
    case focusTables:
        return a.renderPanel("1 Tables", true, a.width, a.contentHeight(), a.tableList.View())
    case focusRowBrowser:
        title := "2 Row Browser"
        if a.rowBrowserReady {
            title = "2 " + a.rowBrowser.DatasetName()
        }
        return a.renderPanel(title, true, a.width, a.contentHeight(), a.renderRightPane())
    default:
        return a.renderSplitContent()
    }
}
```

**`renderStatusBar()` — maximized hint:**

When `a.screen == screenSplit && a.maximized`, prepend `a.keys.Maximize` (shown as
`z restore`) to whatever binding list would normally be shown:

```go
if a.screen == screenSplit && a.maximized {
    // Show z-restore as the first hint, followed by the normal set.
    restoreBinding := key.NewBinding(key.WithKeys("z"), key.WithHelp("z", "restore"))
    // ... prepend restoreBinding to parts ...
}
```

## Tests

### `internal/tui/app_test.go` — extend existing test file

- **Toggle on** — in split view with focus on pane 1, pressing `z` sets `app.maximized = true`
- **Toggle off** — pressing `z` again sets `app.maximized = false`
- **Pane 2 zoom** — pressing `z` with focus on pane 2 sets `maximized = true`
- **Pane 3 zoom** — pressing `z` with focus on pane 3 transitions to `screenQueryLog`
  (does NOT set `maximized = true`)
- **Esc exits maximized (tables)** — when `maximized = true` and focus is pane 1, `esc`
  sets `maximized = false` and returns to split layout
- **Esc exits maximized (row browser, no drill)** — when `maximized = true`, focus on
  pane 2, `NeedsBackKey() == false`: `esc` clears maximized (does not shift focus to pane 1)
- **Esc collapses drill first** — when `maximized = true`, focus on pane 2,
  `NeedsBackKey() == true`: `esc` message goes to row browser (not to the maximized
  toggle path); `maximized` remains true
- **Resize while maximized** — `WindowSizeMsg{Width: 200, Height: 50}` while
  `maximized = true` and focus on pane 2: row browser receives `Width = 198, Height = 46`
  (maximized dimensions), table list receives its normal split width
- **Pane key while maximized** — pressing `2` while maximized on pane 1 keeps
  `maximized = true` and pushes maximized size to row browser
- **Split not broken after restore** — press `z`, then `z` again; `renderSplitContent`
  renders without panic and produces output containing both pane borders

## Files changed

| File | Change |
|---|---|
| `internal/tui/keys/keys.go` | Add `Maximize` binding (`z`) |
| `internal/tui/app.go` | Add `maximized` field; `pushMaximizedSizes`, `pushNormalSizes`, `renderMaximizedContent`; key handling; resize handling; status bar hint |

## Definition of Done

```bash
make preflight
go build ./...
gotestsum --format testdox ./...
make lint
staticcheck ./...
```

Manual verification:
1. Press `z` on table list (pane 1) → table list fills the full terminal; all columns at
   full width; border and "1 Tables" title visible; status bar shows `z restore`
2. Press `z` again → split view restored exactly; all three panes visible
3. Press `z` on row browser (pane 2) → row browser fills full width; more columns
   visible for wide tables; horizontal scroll still works with `←`/`→`
4. Press `esc` while maximized (no drill active) → split restored
5. Drill into an FK while maximized (pane 2) → press `esc` → drill collapses; maximized
   stays true; press `esc` again → split restored
6. Focus pane 3 (`3`), press `z` → query log full-screen opens; `esc` returns to split
7. Resize terminal while maximized → layout fills new dimensions without clipping or blank areas
8. Press `1` while maximized on pane 2 → table list becomes maximized (no flash to split)
9. Press `2` while maximized on pane 1 → row browser becomes maximized
10. Multi-datasource mode: `z` works the same when table list is focused
