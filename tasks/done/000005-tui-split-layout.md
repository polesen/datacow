# Plan: Lazygit-Style Split Layout

## Context

The TUI currently switches between full-screen views (table list → row browser → query log). The user wants a persistent split layout like lazygit: always-visible panels, number-key focus switching, no more full-screen takeovers. Three panes:

```
┌─ 1 Tables ──┐┌─ 2 Row Browser ──────────────────────┐
│ users       ││ id  name       email                  │
│ orders      ││  1  Alice      alice@example.com       │
│ products    ││  2  Bob        bob@example.com         │
└─────────────┘└──────────────────────────────────────┘
┌─ 3 SQL ──────────────────────────────────────────────┐
│ SELECT * FROM users LIMIT 100 OFFSET 0               │
└──────────────────────────────────────────────────────┘
```

## Critical Files

- `/workspace/internal/tui/app.go` — root model, layout, routing
- `/workspace/internal/tui/views/tablelist.go` — left panel
- `/workspace/internal/tui/views/rowbrowser.go` — right panel
- `/workspace/internal/tui/views/querylog.go` — basis for SQL pane
- `/workspace/internal/tui/style/style.go` — border styles
- `/workspace/internal/tui/keys/keys.go` — keybindings

## Implementation Steps

### 1. `keys/keys.go` — new bindings

Add to `Map` struct:
```go
SwitchFocus key.Binding   // tab — cycle through panes
Pane1       key.Binding   // "1"
Pane2       key.Binding   // "2"
Pane3       key.Binding   // "3"
```

Remove `"tab"` from `FilterPills` — it conflicts. `FilterPills` keeps working via explicit pill-mode entry but Tab is now global focus-switch. Add `NeedsTabKey()` to `RowBrowserModel` (see step 3) to gate this.

Update `TableListHelp()` and `FullHelp()` to include the new bindings.

### 2. `style/style.go` — panel border styles

```go
var PanelActive = lipgloss.NewStyle().
    Border(lipgloss.RoundedBorder()).
    BorderForeground(colorPrimary)   // #7DCFFF

var PanelInactive = lipgloss.NewStyle().
    Border(lipgloss.RoundedBorder()).
    BorderForeground(colorBorder)    // #3B4261
```

Borders consume 2 cells width + 2 lines height — account for this in all size helpers.

Panel titles ("1 Tables" etc.) rendered as first line inside the panel content, styled with `colorKey` when active, `colorMuted` when inactive. This avoids needing lipgloss border-title support.

### 3. `views/rowbrowser.go` — new helper methods

```go
// NeedsTabKey: true when row browser is consuming Tab for filter pill navigation
func (m RowBrowserModel) NeedsTabKey() bool {
    return (m.mode == modeNormal && len(m.filters) > 0) || m.mode == modeFilterPills
}
```

Also add `focused bool` field + `SetFocused(bool) RowBrowserModel` method (used to alter border color in App.View).

### 4. `views/tablelist.go`

Add `focused bool` field + `SetFocused(bool) TableListModel` method.

### 5. New `views/sqlpane.go` — SQL pane model

Lightweight model backed by `*db.QueryLog`. Renders compact query history.

```go
type SQLPaneModel struct {
    queryLog    *db.QueryLog
    cursor      int
    focused     bool
    width, height int
}
```

- **Unfocused**: shows last 1-2 queries (most recent at top), truncated to fit height
- **Focused**: scrollable list, cursor navigation with ↑/↓, same data as existing QueryLogView but in a smaller panel
- Uses existing `db.QueryLog` — no new data fetching

This reuses the query log data already collected. No new goroutines or messages needed.

### 6. `app.go` — restructure

**New `focus` type:**
```go
type focus int
const (
    focusTables     focus = iota  // pane 1
    focusRowBrowser               // pane 2
    focusSQL                      // pane 3
)
```

**Updated `screen` enum** — collapse `screenTableList` and `screenRowBrowser` into one:
```go
const (
    screenSplit    screen = iota  // normal 3-pane view
    screenQueryLog                // existing full-screen overlay (L key)
    screenError
)
```

**New/changed `App` fields:**
```go
focus           focus
rowBrowserReady bool          // true once a table has been loaded
sqlPane         views.SQLPaneModel
// remove: prevScreen (replace with screenBeforeOverlay)
```

**New sizing helpers:**
```go
// Left panel: 28% of width, minus 2 for border
func (a *App) leftW() int  { return max(1, a.width*28/100 - 2) }
// Right panel: rest of width, minus 2 for border
func (a *App) rightW() int { return max(1, a.width - a.width*28/100 - 2) }
// SQL pane inner height (fixed 4 lines)
const sqlPaneInnerH = 4
// Top panels inner height
func (a *App) topH() int   { return max(1, a.contentHeight() - sqlPaneInnerH - 2) }
```

**`View()` — new split render:**
```go
func (a *App) renderSplitContent() string {
    // Top row: tables (left) + row browser (right)
    leftBox  := panelBorder(a.focus == focusTables).Width(a.leftW()).Height(a.topH()).Render(a.tableList.View())
    rightBox := panelBorder(a.focus == focusRowBrowser).Width(a.rightW()).Height(a.topH()).Render(a.renderRightPanel())
    topRow   := lipgloss.JoinHorizontal(lipgloss.Top, leftBox, rightBox)

    // Bottom: SQL pane (full width)
    sqlInnerW := a.width - 2
    sqlBox := panelBorder(a.focus == focusSQL).Width(sqlInnerW).Height(sqlPaneInnerH).Render(a.sqlPane.View())

    return lipgloss.JoinVertical(lipgloss.Left, topRow, sqlBox)
}

func (a *App) renderRightPanel() string {
    if !a.rowBrowserReady {
        return style.Muted.Render("Press ↵ or → on a table to open it")
    }
    return a.rowBrowser.View()
}
```

**`Update()` — key routing changes:**

1. **Number keys** — always intercepted at App level before panel routing:
   ```go
   case "1": a.focus = focusTables
   case "2": a.focus = focusRowBrowser
   case "3": a.focus = focusSQL
   ```

2. **Tab** — intercepted unless `rowBrowser.NeedsTabKey()`:
   ```go
   if key.Matches(msg, a.keys.SwitchFocus) && !a.rowBrowser.NeedsTabKey() {
       a.focus = (a.focus + 1) % 3
       return a, nil
   }
   ```

3. **Enter/Right on table list** — loads table in row browser AND shifts focus to pane 2:
   ```go
   if a.focus == focusTables && (enter || right) {
       if ds := a.tableList.SelectedDataset(); ds != nil {
           a.rowBrowser = views.NewRowBrowserModel(...)
           a.rowBrowserReady = true
           a.focus = focusRowBrowser
           return a, a.rowBrowser.Init()
       }
   }
   ```

4. **Left from row browser at col 0, no drill** — shifts focus back to pane 1 instead of going to screenTableList.

5. **WindowSizeMsg** — always dispatched to ALL panels (not just focused), so resizes work correctly even when unfocused.

6. **RowCountMsg** — always routed to tableList regardless of focus (preserve existing behavior).

7. **Spinner ticks** — dispatched to tableList, rowBrowser, and sqlPane.

**Status bar** — show keybindings for the focused pane:
- `focusTables`: quit, ↑↓, ↵ select, tab/1/2/3
- `focusRowBrowser`: existing row browser bar
- `focusSQL`: ↑↓ scroll, 1/2 switch pane

### 7. `app.go` — query log overlay handling

The `L` key still works — it overlays full-screen on top of the split. Save `a.screen` + `a.focus` before entering, restore on exit. `screenQueryLog` is unchanged.

### 8. Test changes

**`app_test.go`** (teatest headless renders):
- Both panel names ("Tables", "Row Browser") appear in initial render
- SQL pane border appears in initial render
- Press `1`/`2`/`3` — assert focus shifts (status bar text changes)
- Press Tab — assert focus cycles
- Press Enter on table — right panel loads data, focus shifts to pane 2
- At pane 2 col 0 no drill, press Left — focus returns to pane 1

**`views/sqlpane_test.go`**:
- Unfocused render shows recent SQL truncated to width
- Focused render shows scrollable list with cursor

## Verification

```bash
make preflight
go build ./...
gotestsum --format testdox ./...
make lint
staticcheck ./...
```

Visual check — run and inspect at two widths:
```bash
go run ./cmd --connection-string="$TEST_POSTGRES_DSN"
# then resize terminal to 80 cols and 200 cols to verify borders don't break
```
