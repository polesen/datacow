# Fix: Export Goroutine is Uncancellable

The export goroutine in `rowbrowser.go` uses `context.Background()`, which means it cannot be cancelled. If the user quits datacow mid-export, the goroutine runs to completion regardless, holding the DB connection and writing to disk after the process should have stopped.

## Current code (`rowbrowser.go:617–631`)

```go
go func() {
    err := ex.Export(context.Background(), m.ds, opts, format, path, func(n int) {
        ...
    })
    ...
}()
```

## Fix

Wire a cancellable context through the export lifecycle:

1. When `startExport` is called, create a `context.WithCancel` and store the cancel function in `RowBrowserModel`.
2. Pass the derived context to `ex.Export(ctx, ...)` instead of `context.Background()`.
3. Call cancel when:
   - The export completes normally or with error (already in the goroutine — defer cancel).
   - The user presses `Esc` while `modeExporting` is active.
   - The App is shutting down (handle `tea.QuitMsg` or the existing quit path in `app.go`).

### `RowBrowserModel` struct change

Add a cancel field:
```go
exportCancel context.CancelFunc // non-nil only while modeExporting
```

### `startExport` change

```go
ctx, cancel := context.WithCancel(context.Background())
m.exportCancel = cancel

go func() {
    defer cancel()
    err := ex.Export(ctx, m.ds, opts, format, path, func(n int) { ... })
    ...
}()
```

### Cancel on Esc during export

In `handleNormalKey` (or `handleKey`), in the `modeExporting` branch, when `Esc` is pressed:
```go
if m.exportCancel != nil {
    m.exportCancel()
    m.exportCancel = nil
}
m.mode = modeNormal
m.statusMsg = "Export cancelled"
```

### Cancel on quit

In `app.go`, in the `tea.QuitMsg` / quit-key handler, call `a.rowBrowser.CancelExport()` if the row browser is ready. Add a method:
```go
func (m RowBrowserModel) CancelExport() {
    if m.exportCancel != nil {
        m.exportCancel()
    }
}
```

Note: `export.Exporter` already passes `ctx` through to the underlying `db.Client.Query` calls, so cancellation propagates to the database without additional work.

## Files to modify

| File | Change |
|---|---|
| `internal/tui/views/rowbrowser.go` | Add `exportCancel` field; update `startExport`; cancel on Esc; add `CancelExport()` |
| `internal/tui/app.go` | Call `CancelExport()` on quit |

## Tests

No new tests needed — cancellation of the DB context is already exercised by the executor's existing context tests. Manual verification:

1. Start a large export, press `Esc` → export stops, status shows "Export cancelled".
2. Start a large export, press `q` → process exits cleanly without hanging.

## Definition of Done

```bash
make preflight
go build ./...
gotestsum --format testdox ./...
staticcheck ./...
make lint
```
