# Context-Sensitive Help Overlay

The help overlay currently hardcodes its four binding groups directly in `helpoverlay.go`. Every time a new keybinding is added to a view, two files must be updated: the view/keys file and the overlay. This is how `i  table info` was missed — it was added to `keys.Map` and `FullHelp()` but not to the overlay's hardcoded System group.

The fix: each view declares its own help groups via an interface. The overlay aggregates whatever the active context provides.

## Design

Define a `HelpProvider` interface in `internal/tui/views`:

```go
type HelpGroup struct {
    Title    string
    Bindings []key.Binding
}

type HelpProvider interface {
    HelpGroups() []HelpGroup
}
```

Each model that owns keybindings implements `HelpGroups()`:

- `TableListModel` — Navigation group (up/down/enter/expand) + a Tables group (i = table info)
- `RowBrowserModel` — Data group (filter/sort/export/view cell/drill) + Navigation group (page/row/col)
- `SQLPaneModel` — minimal (up/down/switch focus)
- `App` itself — Layout group (pane 1/2/3, maximize, goto, refresh) + System group (query log, help, quit)

`HelpOverlayView` is simplified to accept `[]HelpGroup` at render time instead of holding a `keys.Map`:

```go
func (v HelpOverlayView) View(groups []HelpGroup) string
```

`App.renderContent()` assembles the groups from the currently active/focused models before rendering:

```go
case screenHelp:
    groups := a.activeHelpGroups()
    return a.helpView.View(groups)
```

`activeHelpGroups()` collects from the focused pane first, then appends the app-level System group. This makes the overlay context-sensitive: focusing the table list shows table-specific bindings; focusing the row browser shows browser-specific bindings.

## Behaviour changes

- Help overlay shows bindings relevant to the currently focused pane, not a static all-bindings dump.
- New bindings are added only in the model that owns them — `helpoverlay.go` requires no changes for new features.
- The app-level System group (query log, refresh, help, quit) always appears regardless of focus.

## What NOT to change

- The `?` / `Esc` toggle wiring in `app.go` — only the rendering path changes.
- `keys.Map` — bindings stay there. `HelpGroups()` implementations reference `m.keys.X` as before.
- `FullHelp()` on `keys.Map` — leave it for now; it's a separate concern.

## Files to modify

| File | Change |
|---|---|
| `internal/tui/views/helpoverlay.go` | Remove `keys.Map` field; `View` accepts `[]HelpGroup` |
| `internal/tui/views/tablelist.go` | Add `HelpGroups() []HelpGroup` |
| `internal/tui/views/rowbrowser.go` | Add `HelpGroups() []HelpGroup` |
| `internal/tui/views/sqlpane.go` | Add `HelpGroups() []HelpGroup` |
| `internal/tui/app.go` | Add `activeHelpGroups()`, update render call |

## Tests

- Update `helpoverlay_test.go`: construct groups directly and assert they appear in `View()` output.
- Existing app integration tests that open the help overlay (`App help overlay opens and closes`) should continue to pass without changes — the toggle behaviour is unchanged.

## Definition of Done

See `definition-of-done.md`. All gates must pass.

Manual:
1. Press `?` while table list is focused → Navigation and Tables groups visible, including `i  table info`.
2. Press `?` while row browser is focused → Data and Navigation groups visible with browser bindings.
3. System group (query log, refresh, quit) always appears in both cases.
4. Adding a new binding to `TableListModel.HelpGroups()` makes it appear in the overlay with no other changes.
5. All existing tests pass.
