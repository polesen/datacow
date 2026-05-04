# Fix: Mixed Pointer/Value Receivers on TUI Models

Four TUI model types have an outlier pointer-receiver method among otherwise all-value-receiver methods. This violates the Go idiom (never mix receiver types on the same type) and the Bubble Tea convention (all-value, since Update returns a new copy).

## Why this matters

Mixing receivers is subtle but dangerous: a pointer method called on a local value copy mutates a temporary, and only works correctly by accident when the caller happens to return that local copy. A future refactor could silently break the mutation without a compile error.

## Changes required

### 1. `RowBrowserModel` — `rowbrowser.go:580`

**Current:**
```go
func (m *RowBrowserModel) cycleSort() {
    // mutates m.sort in place
}
```
Called at `rowbrowser.go:389`: `m.cycleSort()`

**Fix:** Convert to value receiver returning the modified model.
```go
func (m RowBrowserModel) cycleSort() RowBrowserModel {
    // same logic, mutate local m
    return m
}
```
Update call site at line 389:
```go
m = m.cycleSort()
```

### 2. `QueryLogView` — `querylog.go:56`

**Current:**
```go
func (v *QueryLogView) SetSpinChar(s string) {
    v.spinChar = s
}
```
Called at `app.go:344`: `a.queryLogView.SetSpinChar(a.appSpinner.View())`

`queryLogView` is stored by value in `App` — the pointer method call works because struct fields are addressable. But it's inconsistent with the rest of `QueryLogView`.

**Fix:** Value receiver returning updated view.
```go
func (v QueryLogView) SetSpinChar(s string) QueryLogView {
    v.spinChar = s
    return v
}
```
Update call site in `app.go:344`:
```go
a.queryLogView = a.queryLogView.SetSpinChar(a.appSpinner.View())
```

### 3. `GotoModel` — `goto.go`

Three pointer-receiver methods among otherwise all-value methods:
- `Focus() tea.Cmd` at line 54
- `refreshResults()` at line 275
- `ensureCursorVisible()` at line 330

`refreshResults` and `ensureCursorVisible` are private helpers that mutate `m.results`/`m.cursor`/`m.scrollOffset`. `Focus` is the public entry point that calls both.

**Fix:** Convert all three to value receivers returning the modified model.

```go
func (m GotoModel) refreshResults() GotoModel {
    // same logic, mutate local m
    return m
}

func (m GotoModel) ensureCursorVisible() GotoModel {
    // same logic, mutate local m
    return m
}

func (m GotoModel) Focus() (GotoModel, tea.Cmd) {
    m.input.SetValue("")
    m.cursor = 0
    m.scrollOffset = 0
    m = m.refreshResults()
    cmd := m.input.Focus()
    return m, cmd
}
```

Update call site in `app.go:368`:
```go
a.gotoModel, cmd = a.gotoModel.Focus()
return a, cmd
```

Also update the internal calls to `refreshResults` and `ensureCursorVisible` inside `Update` and other value-receiver methods to assign the returned model:
```go
m = m.refreshResults()
m = m.ensureCursorVisible()
```

### 4. `TableListModel` — `tablelist.go`

Three pointer-receiver methods:
- `anyLoading() bool` at line 326 — reads only, no mutation. Pointer receiver is entirely unnecessary.
- `SelectByName(name string) bool` at line 350 — mutates `m.cursor`, calls `ensureCursorVisible`.
- `ensureCursorVisible()` at line 444 — mutates `m.scrollOffset`.

**Fix:**

`anyLoading` — trivial, just change receiver to value (no return value change needed since it doesn't mutate):
```go
func (m TableListModel) anyLoading() bool { ... }
```

`ensureCursorVisible` — value receiver returning model:
```go
func (m TableListModel) ensureCursorVisible() TableListModel {
    // same logic
    return m
}
```

`SelectByName` — value receiver returning `(TableListModel, bool)`:
```go
func (m TableListModel) SelectByName(name string) (TableListModel, bool) {
    for i, ds := range m.datasets {
        if ds.Name == name {
            m.cursor = i
            m = m.ensureCursorVisible()
            return m, true
        }
    }
    return m, false
}
```

Update call site in `app.go:544`:
```go
a.tableList, _ = a.tableList.SelectByName(msg.Dataset.Name)
```

Update internal calls to `ensureCursorVisible` inside `TableListModel` methods to assign the return value.

## Files to modify

| File | Change |
|---|---|
| `internal/tui/views/rowbrowser.go` | `cycleSort` — value receiver, return model |
| `internal/tui/views/querylog.go` | `SetSpinChar` — value receiver, return view |
| `internal/tui/views/goto.go` | `Focus`, `refreshResults`, `ensureCursorVisible` — value receivers, return model |
| `internal/tui/views/tablelist.go` | `anyLoading`, `SelectByName`, `ensureCursorVisible` — value receivers; `SelectByName` returns `(TableListModel, bool)` |
| `internal/tui/app.go` | Update all call sites for the changed signatures |

## Tests

No new tests. Run the existing suite — if the receiver conversions are correct the tests pass unchanged.

## Definition of Done

```bash
make preflight
go build ./...
gotestsum --format testdox ./...
staticcheck ./...
make lint
```
