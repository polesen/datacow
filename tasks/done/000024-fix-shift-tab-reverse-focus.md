# Fix: Shift+Tab Does Not Cycle Focus Backward

`Tab` cycles focus forward through the three panes (table list → row browser → query log → table list). The reverse direction is missing — `Shift+Tab` does nothing today. Add `Shift+Tab` as the reverse cycle so users can move back without wrapping all the way around.

## Current behaviour

- `Tab` advances focus: `focusTables` → `focusRowBrowser` → `focusSQL` → `focusTables`.
- `Shift+Tab` is unbound; pressing it has no effect.

The cycle lives in `internal/tui/app.go:496–501`:

```go
if key.Matches(msg, a.keys.SwitchFocus) {
    if !a.rowBrowserReady || a.focus != focusRowBrowser || !a.rowBrowser.NeedsTabKey() {
        a.focus = focus((int(a.focus) + 1) % 3)
        return a, nil
    }
}
```

The `SwitchFocus` binding is defined in `internal/tui/keys/keys.go:74–77` with key `tab`.

## Fix

Add a `SwitchFocusBack` binding for `shift+tab` and wire it as the inverse of the existing forward cycle. Use the same `NeedsTabKey()` gate as the forward direction so the behaviour stays symmetric — if the row browser ever consumes `shift+tab` later (e.g. for reverse-pill navigation), the gate is already in place.

### 1. `internal/tui/keys/keys.go`

Add the field to `Map`:

```go
SwitchFocus     key.Binding
SwitchFocusBack key.Binding
```

Add the default binding next to `SwitchFocus` in `Default()`:

```go
SwitchFocusBack: key.NewBinding(
    key.WithKeys("shift+tab"),
    key.WithHelp("shift+tab", "prev pane"),
),
```

Include `SwitchFocusBack` in `FullHelp()` next to `SwitchFocus`:

```go
{m.SwitchFocus, m.SwitchFocusBack, m.Pane1, m.Pane2, m.Pane3, m.Maximize, m.Goto},
```

### 2. `internal/tui/app.go`

Just below the existing `SwitchFocus` block (around line 496), add the reverse branch. Use modular arithmetic that handles wrap-around correctly (Go's `%` can return a negative result for negative operands, so add the cycle length before taking the modulus):

```go
if key.Matches(msg, a.keys.SwitchFocusBack) {
    if !a.rowBrowserReady || a.focus != focusRowBrowser || !a.rowBrowser.NeedsTabKey() {
        a.focus = focus((int(a.focus) + 2) % 3)
        return a, nil
    }
}
```

(`+2 mod 3` is equivalent to `-1 mod 3` for a three-state cycle and avoids the negative-modulus pitfall.)

### 3. Help overlay

`internal/tui/views/helpoverlay.go` renders the bindings from `keys.Map.FullHelp()`. Adding `SwitchFocusBack` to that slice is sufficient — verify the overlay actually shows the new entry. If the overlay has any hand-written section for pane navigation, update it there too.

## Acceptance Criteria

- [ ] `Shift+Tab` in the split view cycles focus backward: row browser → table list, query log → row browser, table list → query log.
- [ ] `Tab` behaviour is unchanged.
- [ ] When the row browser is consuming `Tab` (filter input or filter-pill mode), `Shift+Tab` is also passed through to the row browser rather than switching panes, matching the existing `Tab` gate.
- [ ] Help overlay shows `shift+tab` with the description `prev pane` alongside the existing `tab` entry.
- [ ] Existing tests pass unchanged. Add at least one teatest case that focuses the row browser, presses `Shift+Tab`, and asserts focus has moved to the table list.

## Files to modify

| File | Change |
|---|---|
| `internal/tui/keys/keys.go` | Add `SwitchFocusBack` field, default binding, include in `FullHelp()` |
| `internal/tui/app.go` | Handle `SwitchFocusBack` next to the existing `SwitchFocus` block |
| `internal/tui/views/helpoverlay.go` | Verify new entry renders (no change expected if it reads `FullHelp()` directly) |
| `internal/tui/app_test.go` (or nearest equivalent) | New teatest case for reverse focus cycle |

## What NOT to Change

- The forward `Tab` cycle — same key, same behaviour, same gate.
- Filter-pill navigation, filter input, or any other row-browser key handling. `NeedsTabKey()` is consulted, not modified.
- The pane order (`focusTables` → `focusRowBrowser` → `focusSQL`). Only the direction changes.
- Anything outside the split view (modals, overlays, single-pane screens) — they don't have multiple panes to cycle.

## Definition of Done

See [definition-of-done.md](../definition-of-done.md). All gates must pass.
