# TUI: Help Overlay

Full-screen keybinding reference, triggered by `?`. Fills the gap between the compact status bar hint and knowing nothing — without the complexity of per-context tracking.

## Design decisions

**Not context-sensitive.** `?` is only meaningful from `screenSplit` (the main view). Overlays (query log, cell viewer, goto) are self-contained and transient — showing a binding list inside them adds noise. Disable `?` in all non-split screens.

**Grouped, not flat.** Bindings are organised by usage group within a single overlay. This gives the readability benefit of context-sensitivity without needing to track focus state.

**Full-screen, not a popup.** Consistent with query log and cell viewer. No border math needed.

**Toggle.** `?` opens; `?` or `Esc` closes. Same pattern as `L` for the query log.

## UX

Press `?` from the main view:

```
 Keybindings

 Navigation
   ↑ / k    up               ↓ / j    down
   ← / h    back             → / l    select / drill down
   ↵        select           esc      back / close
   [        prev page        ]        next page

 Data
   /        filter           x        remove filter
   tab      cycle filters    s        sort
   e        export           v        view cell

 Layout
   tab      next pane        1        tables pane
   2        browser pane     3        sql pane
   z        maximize pane    ctrl+p   goto table

 System
   L        query log        ctrl+r   refresh schema
   ?        this help        q        quit


 ? or esc  close
```

## Implementation

### `internal/tui/app.go`

Add `screenHelp` to the `screen` const block (alongside `screenQueryLog`, `screenCellViewer`).

Wire `?` key in `Update()`:
- Only from `screenSplit` — same guard as the `L` / query log handler
- On open: `a.screenBeforeOverlay = a.screen; a.screen = screenHelp`
- On close (`?` or `Esc` while `screenHelp`): `a.screen = a.screenBeforeOverlay`

Add `helpView views.HelpOverlayView` field to `App`. Propagate `WindowSizeMsg` to it. Render it in `View()` when `a.screen == screenHelp`.

### `internal/tui/views/helpoverlay.go`

```go
type HelpOverlayView struct {
    keys        keys.Map
    width, height int
}

func NewHelpOverlayView(k keys.Map) HelpOverlayView
func (v *HelpOverlayView) SetSize(w, h int)
func (v HelpOverlayView) View() string
```

`View()` renders the groups below, using `style.ColHeader` for group titles and alternating `style.StatusKey` / `style.StatusDesc` for key / description pairs. Two-column layout within each group (left and right halves of the terminal width).

**Groups and bindings** (define these directly in the view — do not call `keys.FullHelp()`, which is an incomplete stub):

| Group | Bindings |
|---|---|
| Navigation | Up, Down, Left, Right, Enter, Back, NextPage, PrevPage |
| Data | Filter, FilterPills, RemoveFilter, Sort, Export, ViewCell |
| Layout | SwitchFocus, Pane1, Pane2, Pane3, Maximize, Goto |
| System | QueryLog, Refresh, Help, Quit |

Footer line at the bottom: `style.Muted.Render("  ? or esc   close")`.

### `internal/tui/keys/keys.go`

Update `FullHelp()` to include the complete grouped list (currently it is missing `ViewCell`, `Goto`, `Refresh`, `FilterPills`, `RemoveFilter`, `NextPage`, `PrevPage`). The help overlay view renders its own layout, but `FullHelp()` should be accurate for any future use by `bubbles/help`.

## Files to create / modify

| File | Change |
|---|---|
| `internal/tui/views/helpoverlay.go` | New — `HelpOverlayView` |
| `internal/tui/app.go` | Add `screenHelp`, wire `?` key, add `helpView` field, render screen |
| `internal/tui/keys/keys.go` | Complete `FullHelp()` with all bindings |

No core or style changes required — existing styles are sufficient.

## Tests

No unit tests — pure rendering with no branching state. Manual verification is sufficient.

## Definition of Done

```bash
make preflight
go build ./...
gotestsum --format testdox ./...
staticcheck ./...
make lint
```

Manual:
1. Press `?` from the main split view → help overlay opens full-screen with all four groups.
2. All bindings from `keys.Map` appear in the overlay, correctly labelled.
3. Press `?` again → returns to previous screen.
4. Press `Esc` → returns to previous screen.
5. Press `?` while in query log or cell viewer → nothing happens (key ignored).
6. Resize the terminal while the overlay is open → layout adapts cleanly.
