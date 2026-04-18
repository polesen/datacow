# TUI: Cell Viewer + Save to File

Inspect the full contents of any cell in the row browser, and optionally save it to disk.

Primarily useful for large/structured column types (JSON, TEXT, BLOB) but available on all cells.

## UX

Press `Enter` (or `v`) on a selected cell in the row browser → full-screen overlay opens.

```
┌─ users · id=42 · bio (text) ────────────────────────────────────┐
│                                                                  │
│  Passionate software engineer with 10 years of experience in... │
│  (scrollable)                                                    │
│                                                                  │
│                                                                  │
│  s  save to file    c  copy to clipboard    Esc  close          │
└──────────────────────────────────────────────────────────────────┘
```

Header shows: table name, PK column=value (or compound), column name, column type.

For JSON and XML: auto-detect and pretty-print with indentation.

### Save flow

Press `s` → editable filename input appears, pre-filled with suggested name → `Enter` confirms, saves to `$PWD`.

Suggested filename format: `{table}-{pk-values}-{column}.{ext}`

Examples:
- `users-42-bio.txt`
- `documents-7-body.json`
- `orders-5-12-notes.txt` (compound PK: values only, dash-separated)

Sanitization rules for all filename segments:
- Spaces → `-`
- Strip anything not in `[a-zA-Z0-9_\-.]`
- Truncate each segment to 40 chars to avoid absurd paths

Extension inference (in priority order):
1. JSON detection: attempt `json.Unmarshal` → `.json` (pretty-print on save)
2. XML detection: trim and check `<` prefix → `.xml`
3. BLOB magic bytes:
   - `\x89PNG` → `.png`
   - `\xFF\xD8\xFF` → `.jpg`
   - `GIF8` → `.gif`
   - `RIFF....WEBP` → `.webp`
   - `%PDF` → `.pdf`
   - `PK\x03\x04` → `.zip`
   - fallback → `.bin`
4. Text columns → `.txt`

### Copy to clipboard

Press `c` → copy raw value (not pretty-printed) to system clipboard. No filename prompt.
Use `golang.design/x/clipboard` or `atotto/clipboard`.

## Architecture

```
CellViewerModel (tui/views)
    ↑ opened by screenRowBrowser
    receives: table name, column name, column type, PK map, raw cell value ([]byte or string)
    owns: scroll offset, save/copy state
```

No new core logic needed — the cell value is already in memory from the last query result.
MIME detection is a pure function in `tui/views/cellviewer.go` (no DB access).

## Files to create / modify

| File | Change |
|---|---|
| `internal/tui/views/cellviewer.go` | New — `CellViewerModel`, `View()`, `Update()`, MIME detection, filename suggestion |
| `internal/tui/views/cellviewer_test.go` | New — filename suggestion and MIME detection tests |
| `internal/tui/app.go` | Add `screenCellViewer`, wire open/close, pass cell data |
| `internal/tui/keys/keys.go` | Add `ViewCell` binding (`v` / `Enter`, help `"v  view cell"`) |

## Implementation notes

### `CellViewerModel`

```go
type CellViewerModel struct {
    tableName  string
    pkValues   []string  // ordered PK col=val pairs, values only for filename
    columnName string
    columnType string
    raw        []byte

    scrollOffset int
    width, height int

    saveInput   textinput.Model  // visible only when saving
    saveState   saveState        // idle | prompting | saved | error
    saveMsg     string           // "Saved to users-42-bio.json" or error text
}
```

### Filename suggestion

```go
func suggestFilename(table string, pkValues []string, column string, raw []byte) string {
    ext := inferExtension(raw)
    parts := []string{sanitize(table)}
    for _, v := range pkValues {
        parts = append(parts, sanitize(v))
    }
    parts = append(parts, sanitize(column))
    return strings.Join(parts, "-") + ext
}
```

### MIME detection

Pure function, no dependencies beyond stdlib:

```go
func inferExtension(data []byte) string {
    // JSON, XML, then magic bytes, then .txt / .bin
}
```

### Opening the viewer

In `screenRowBrowser`, when the user presses `v` or `Enter` on a cell:
- Extract the cell value from the current result row
- Build the PK map from the row's PK columns
- Construct `CellViewerModel` and transition to `screenCellViewer`

## Tests

`internal/tui/views/cellviewer_test.go`:
- `suggestFilename` — table/column names with special chars, compound PKs, truncation
- `inferExtension` — JSON string, XML string, each magic byte sequence, plain text, empty bytes

## Definition of Done

```bash
make preflight
go build ./...
gotestsum --format testdox ./...
staticcheck ./...
make lint
```

Manual:
1. Open any table, navigate to a cell with long text → press `v` → viewer opens with full content
2. Navigate to a JSON column → viewer shows pretty-printed JSON
3. Press `s` → filename prompt pre-filled with suggested name → edit name → `Enter` → file appears in `$PWD`
4. Press `c` → value copied to clipboard
5. Press `Esc` → returns to row browser, scroll position preserved
6. Compound-PK table → filename includes both PK values, no `=` or `,` in name
