# SQL Dataset Editor with Completions

Users can open a multi-line SQL editor for any `KindDataset` dataset, edit the query with schema-aware completions, and save the result back to the config file without leaving the TUI. No syntax highlighting — the text is plain.

## Background

`KindDataset` datasets are defined by hand in `datacow.yaml` as raw SQL strings. Once written, there is no way to adjust the query from inside the TUI — the user must open a text editor, edit the YAML, and restart. This task adds an in-TUI SQL editor with:

- A multi-line textarea for editing the SQL
- Tab-triggered schema-aware completions (table names, column names, SQL keywords)
- Save back to the config file on confirm

The completion engine lives in `internal/core/completions/` so a future web editor can reuse it. It is vendor-neutral by design: schema info already comes from the schema cache in memory; keyword lists are per-vendor static maps keyed on a new `db.Dialect` type. Adding a new DB vendor (SQLite, Oracle, MSSQL) requires only implementing `db.Client` and returning the correct `Dialect()` constant — no changes to the completion engine itself.

Because `KindDataset` entries can only exist when a YAML config file is present, `tui.Config.ConfigPath` is always non-empty when the editor can open. No zero-config edge case to handle.

## Architecture

### `db.Dialect` — vendor discriminator

Add to `internal/core/db/client.go`:

```go
type Dialect string

const (
    DialectPostgres Dialect = "postgres"
    DialectMySQL    Dialect = "mysql"
    DialectSQLite   Dialect = "sqlite"
    DialectMSSQL    Dialect = "mssql"
    DialectOracle   Dialect = "oracle"
)
```

Add `Dialect() Dialect` to the `Client` interface. The two existing clients return their constant:

```go
func (c *postgresClient) Dialect() db.Dialect { return db.DialectPostgres }
func (c *mysqlClient)    Dialect() db.Dialect { return db.DialectMySQL }
```

`Dialect()` also drives identifier quoting when a completion is inserted: Postgres / SQLite / Oracle use `"`, MySQL uses `` ` ``, MSSQL uses `[` / `]`.

### `internal/core/completions/` — new package

```go
type Completer struct{ /* unexported */ }

type Suggestion struct {
    Text   string // text to insert
    Kind   Kind
    Detail string // e.g. "varchar(255)" for columns; empty for keywords
}

type Kind int

const (
    KindTable   Kind = iota
    KindColumn
    KindKeyword
)

// New builds a Completer from the schema tables already in memory.
// dialect selects the per-vendor keyword extensions and quote character.
func New(tables []schema.Table, dialect db.Dialect) *Completer

// Complete returns ranked suggestions for the current cursor position.
// sql is the full editor content; cursorPos is a byte offset.
func (c *Completer) Complete(sql string, cursorPos int) []Suggestion
```

**Context detection — left-scan heuristic:**

Scan left from `cursorPos`, collecting the current partial word (the "prefix") and the last SQL keyword encountered before it.

| Last keyword | Prefix shape | Suggestions |
|---|---|---|
| `FROM` `JOIN` `UPDATE` `INTO` | any | table names matching prefix |
| `SELECT` `WHERE` `ON` `SET` `BY` | `table.prefix` | columns of that table matching suffix |
| `SELECT` `WHERE` `ON` `SET` `BY` | bare word | columns from all tables + keywords matching prefix |
| any / ambiguous | bare word | keywords matching prefix |

Suggestions are ranked: exact-prefix matches first, then prefix-case-insensitive matches. Table and column suggestions rank above keywords when context is unambiguous. Returns an empty (non-nil) slice when nothing matches.

**Keyword lists:**

`commonKeywords` covers ANSI SQL: `SELECT`, `FROM`, `WHERE`, `JOIN`, `LEFT`, `INNER`, `RIGHT`, `FULL`, `OUTER`, `CROSS`, `GROUP BY`, `ORDER BY`, `HAVING`, `LIMIT`, `OFFSET`, `INSERT`, `UPDATE`, `DELETE`, `WITH`, `AS`, `ON`, `AND`, `OR`, `NOT`, `NULL`, `IS`, `IN`, `LIKE`, `BETWEEN`, `CASE`, `WHEN`, `THEN`, `ELSE`, `END`, `DISTINCT`, `EXISTS`, `UNION`, `ALL`, `COUNT`, `SUM`, `AVG`, `MIN`, `MAX`, `COALESCE`, `CAST`, `ASC`, `DESC`.

`dialectKeywords map[db.Dialect][]string` adds vendor-specific extensions:

- Postgres: `RETURNING`, `ILIKE`, `ON CONFLICT`, `JSONB`, `ARRAY`, `LATERAL`, `WINDOW`, `OVER`, `PARTITION BY`
- MySQL: `STRAIGHT_JOIN`, `SQL_NO_CACHE`, `REPLACE INTO`, `CALC_FOUND_ROWS`, `ON DUPLICATE KEY`
- SQLite: `ROWID`, `WITHOUT ROWID`, `STRICT`, `UNIXEPOCH`, `GLOB`
- MSSQL: `TOP`, `WITH (NOLOCK)`, `CROSS APPLY`, `OUTER APPLY`, `IDENTITY`, `NOCOUNT`, `IIF`
- Oracle: `ROWNUM`, `CONNECT BY`, `START WITH`, `PRIOR`, `DUAL`, `MINUS`, `NVL`, `DECODE`

Adding a new vendor = add a `[]string` entry to the map.

**Identifier quoting:**

When a suggestion is a table or column name that matches a reserved word or contains special characters, `Completer` wraps it with the dialect's quote chars on insertion. A reasonable reserved-word check (lowercase comparison against a small set) is sufficient; quote proactively when uncertain.

### TUI — SQL editor overlay (`internal/tui/views/sqleditor.go`)

```go
type SQLEditorModel struct {
    textarea    textarea.Model
    completer   *completions.Completer
    popup       []completions.Suggestion // nil = popup closed
    popupCursor int
    popupPrefix string // text replaced when a suggestion is accepted
    original    string // SQL at open time, for Esc revert
    datasetName string // shown in the title bar
    configPath  string // target config file for save
    cancelled   bool   // set on Esc with popup closed
    saved       bool   // set on Ctrl+S success
    err         string // inline error message
    width       int
    height      int
}
```

**Rendering — editor (popup closed):**

```
┌─ Edit SQL — api_logs_summary ───────────────────────────┐
│                                                          │
│  SELECT source, COUNT(*) AS total                       │
│  FROM api_logs                                          │
│  WHERE created_at > NOW() - INTERVAL '7 days'           │
│  GROUP BY sou█                                          │
│                                                          │
│  Tab completions · Ctrl+S save · Esc cancel             │
└──────────────────────────────────────────────────────────┘
```

**Rendering — with popup open:**

The popup renders as a block between the textarea and the hint footer. Up to 8 entries are shown; the selected entry is highlighted with a `>` marker. `SetSize` reserves 12 rows of chrome (1 title + 8 popup + 1 hint + 2 border) so opening the popup never pushes the title or first line of the textarea off-screen.

```
│  GROUP BY sou█                                          │
│  …                                                       │
│  > source        integer                                 │
│    source_ip     varchar(45)                             │
│                                                          │
│  Tab completions · Ctrl+S save · Esc cancel             │
```

A future task may replace this with a true floating overlay anchored at the cursor.

**Keys:**

| Key | Context | Action |
|---|---|---|
| `Tab` | popup closed | open popup; suggestions for current word |
| `Tab` | popup open | move cursor down (wraps to top) |
| `Shift+Tab` | popup open | move cursor up (wraps to bottom) |
| `Enter` | popup open | insert selected suggestion at cursor; close popup |
| `Esc` | popup open | close popup; editor stays open; SQL unchanged |
| `Ctrl+S` | popup closed | confirm: validate, save, close |
| `Esc` | popup closed | cancel: close editor; SQL reverts to `original` |
| any printable key | popup open | forward to textarea; close popup |
| any printable key | popup closed | forward to textarea |

`Enter` with popup closed inserts a newline (textarea default).

**On confirm (`Ctrl+S`):**
1. Reject empty SQL — render inline `"SQL cannot be empty"`, keep overlay open.
2. Reject empty `ConfigPath` — render `"no config file path — cannot save"`, keep overlay open.
3. Call `config.UpdateDatasetSQL(configPath, datasetName, editorSQL)` (helper added in `internal/core/config/config.go`; loads YAML, finds the SQL-bearing dataset by name, replaces its `SQL` field, atomically saves).
4. On IO / not-found error: render the OS error inline, keep overlay open.
5. On success: set `saved = true`; emit `DatasetSQLSavedMsg{DatasetName: ..., SQL: ..., Path: ...}`.

**App handles `DatasetSQLSavedMsg`:**
- Close the editor overlay.
- Show status line: `"Saved to <path>"`.
- Reload the dataset list (same `TablesLoadedMsg` flow as task 30's perspective reload).
- If the saved dataset is currently open in the row browser, trigger a re-fetch.

**On cancel (`Esc` with popup closed):** set `cancelled = true`; app closes overlay with no state change.

### TUI — keybindings and entry points

Add `EditSQL key.Binding` (default: `E`, uppercase) to `keys.Map`. Lowercase `e` is already bound to `Export`; using `E` lets both shortcuts coexist on the same row.

Two entry points, both gated on `KindDataset`:

- **Schema explorer**: `E` when the cursor row is a `KindDataset` entry. Opens editor pre-populated with `dataset.SQL` from the focused dataset.
- **Row browser**: `E` when `currentDataset.Kind == KindDataset`. Opens editor pre-populated with `currentDataset.SQL`.

`E` on `KindTable`, `KindView`, or `KindPerspective` is a no-op (editor does not open). `e` (lowercase) continues to behave exactly as before — Export menu on the row browser, no-op in the schema explorer.

In `helpoverlay.go`: rename the existing "Row Browser" section to "Dataset" and add `EditSQL` to it alongside `DrillFwd` / `DrillReverse` (these are all dataset-context operations). The entry is shown unconditionally — `HelpOverlayView` does not currently carry per-context state, and adding it would touch every overlay caller. Conditional rendering is tracked by `tasks/drafts/context-sensitive-help.md`.

**Building the `Completer`:** when opening the editor, the app constructs `completions.New(schemaCache.Tables(), client.Dialect())`. Both are already held in app state — no new DB queries at editor-open time.

## Acceptance Criteria

Tests follow the `TestAC_<SECTION><NN>_<description>` pattern. The acceptance test file must open with a **coverage map** comment mapping every criterion to its test(s).

### CP — Completion engine (unit tests in `completions/completer_test.go`)

- CP01: `Complete("SELECT ", 7)` with a schema containing table `users` and dialect Postgres returns at least one `KindKeyword` suggestion (`SELECT` itself or similar); result is non-nil.
- CP02: `Complete("SELECT * FROM ", 14)` with tables `users` and `orders` returns both table names as `KindTable` suggestions.
- CP03: `Complete("SELECT * FROM us", 16)` returns `users` but not `orders` (prefix filter applied).
- CP04: `Complete("SELECT u.", 9)` with `users` having columns `id` (int) and `email` (varchar) returns two `KindColumn` suggestions; `Detail` of the `id` entry contains `"int"` (or the column type string).
- CP05: `Complete("SELECT em", 9)` (bare prefix, no table qualifier) returns column names from all tables whose names begin with `"em"` plus any matching keywords.
- CP06: `New(tables, db.DialectMySQL).Complete("SELECT ", 7)` returns at least one MySQL-specific keyword (e.g. `STRAIGHT_JOIN`) not present in the equivalent Postgres result.
- CP07: `New(tables, db.DialectPostgres).Complete("SELECT ", 7)` returns at least one PG-specific keyword (e.g. `RETURNING`) not present in the MySQL result.
- CP08: `Complete("", 0)` returns a non-nil, non-empty slice (keywords at minimum).
- CP09: `Complete("SELECT xyz", 10)` with no table named `xyz` and no matching keywords returns an empty (non-nil) slice.
- CP10: `Complete("SELECT * FROM orders o WHERE o.", 30)` returns column names for `orders` (dot-qualified context); no table names in the result.

### DI — Dialect interface (compile-time and unit tests in `db/`)

- DI01: `postgresClient.Dialect()` returns `db.DialectPostgres`.
- DI02: `mysqlClient.Dialect()` returns `db.DialectMySQL`.
- DI03: Both `var _ db.Client = (*postgresClient)(nil)` and `var _ db.Client = (*mysqlClient)(nil)` continue to compile after adding `Dialect()` to the interface.

### ED — Editor view (view unit tests in `sqleditor_test.go`)

- ED01: `View()` of a freshly opened editor contains the dataset name, the pre-populated SQL string, and `"Ctrl+S save"` and `"Esc cancel"`.
- ED02: Sending `Ctrl+S` with an empty textarea does not close the overlay — `View()` contains `"cannot be empty"` (or equivalent) and the editor is still visible.
- ED03: `Tab` with completions available opens the popup — `View()` contains at least one suggestion text string.
- ED04: With popup open, sending `Enter` inserts the first suggestion into the editor text; `View()` no longer shows the popup.
- ED05: With popup open, sending `Esc` closes the popup but leaves the editor open; `View()` still contains `"Edit SQL"`.
- ED06: With popup closed, sending `Esc` sets `cancelled = true` on the model.
- ED07: `e` key is present in `keys.Map`, wired in both schema explorer and row browser `Update()`, and appears in `helpoverlay.go` in a Dataset-context section.
- ED08: Sending `e` while the cursor is on a `KindTable` row in the schema explorer does not cause the editor to open (no `SQLEditorModel` returned from `Update()`).

### AC — App integration tests (in `app_test.go` or `sqleditor_acceptance_test.go`)

- AC01 (open from schema explorer): Load a config with at least one `KindDataset`. Navigate to it in the schema explorer; press `e`. Assert: `View()` contains `"Edit SQL"` and the dataset's SQL string.
- AC02 (open from row browser): With a `KindDataset` open in the row browser, press `e`. Assert: same as AC01.
- AC03 (completions in context): In the editor, with a schema containing table `orders`, position the textarea so the content ends with `"FROM ord"`, press `Tab`. Assert: `View()` contains `"orders"` in the popup area.
- AC04 (insert completion): From AC03, press `Enter`. Assert: editor textarea content contains `"orders"` and the popup is no longer visible.
- AC05 (save and reload): Edit the SQL, press `Ctrl+S`. Assert: overlay closes; status line contains `"Saved to"`; config file on disk reflects the new SQL when read back via `config.Load()`.
- AC06 (row browser re-fetches after save): With a `KindDataset` open in the row browser, open the editor, change the SQL to a different valid query, confirm. Assert: row browser re-fetches; its `View()` reflects results from the new query (or a loading state).
- AC07 (Esc reverts): Open the editor, modify the SQL, press `Esc` (popup closed). Assert: overlay closes; config file is unchanged; the dataset SQL in memory equals the original.
- AC08 (Esc closes popup not editor): Open editor, press `Tab` to open popup, press `Esc`. Assert: popup is gone; `"Edit SQL"` is still visible; SQL text is unchanged.

## What NOT to Change

- Syntax highlighting — explicitly out of scope for this task.
- Creating new SQL datasets from within the TUI — `e` only opens on existing `KindDataset` entries; defer new-dataset creation to a future task.
- `KindTable`, `KindView`, `KindPerspective` — `e` must be a no-op on these.
- `config.Load()` and `config.AppendPerspective()` — do not change signatures or behaviour.
- `PageSizeRegistry`, `ColumnRegistry`, filter state, sort state — unaffected.
- The `COUNT(*)` subquery — unaffected.
- Export — unaffected.

## Implementation Notes

Decisions taken during implementation that diverge from the original draft above (each was reconciled in this spec):

1. **Key binding `E` (uppercase) instead of `e`.** Lowercase `e` was already bound to Export. Rather than break Export on `KindDataset` rows, the editor uses `Shift+E`. Both shortcuts now coexist on the same row browser; `e` still opens Export everywhere it did before, `E` opens the editor only when the row/dataset is `KindDataset`.
2. **Help-overlay entry is unconditional.** Spec originally said "shown only when focused context is `KindDataset`". `HelpOverlayView` carries no per-context state today; threading it through would touch every overlay caller. The entry now lives in a renamed "Dataset" section alongside `DrillFwd`/`DrillReverse`. Conditional rendering is tracked by the existing draft `tasks/drafts/context-sensitive-help.md`.
3. **Popup is a block, not a floating overlay.** Spec originally described the popup as "float[ing] immediately below the cursor". The implementation renders it as a block between the textarea and hint footer; `SetSize` reserves 12 chrome rows so the box never overflows a 40-row terminal. Functionally identical for completion acceptance; visually slightly different.
4. **`config.UpdateDatasetSQL` helper added.** Spec walked through the load → find → mutate → save loop inline in the editor's `confirm()`. That is business logic, which CLAUDE.md forbids in the TUI layer. Extracted into `config.UpdateDatasetSQL(path, name, newSQL)` so the editor stays purely presentational.
5. **`SQLEditorModel` extra fields.** Added `popupPrefix` (records the text to delete-and-replace on Enter) and `configPath` (target file for save). Both are required by the implementation; the draft missed them.

## Definition of Done

See `tasks/definition-of-done.md`. Invoke `/done` after all acceptance criteria are met.
