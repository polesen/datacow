# Save Table View as Perspective

Let users name and persist the current row browser state — active columns, filters, and sort — as a **perspective** nested under its parent table dataset in `datacow.yaml`. A perspective is a reusable, named lens over a table, not a SQL VIEW.

## Background

After the column picker (task 29), users can project columns, filter rows, and sort — but all of that state is lost when they navigate away or restart. The natural next step is to say "this is how I want to see this table most of the time" and persist it. The `api_logs` table might have two useful perspectives: **Failed Calls** (result ≠ 200) and **Recent Errors** (status = 500, last 24h) — both reachable from the schema explorer without rebuilding filters each time.

## Architecture

### Config layer — `PerspectiveConfig` and `DatasetConfig.Perspectives`

Extend `DatasetConfig` in `internal/core/config/config.go`:

```go
type DatasetConfig struct {
    Name         string              `yaml:"name"`
    Datasource   string              `yaml:"datasource,omitempty"`
    Table        string              `yaml:"table,omitempty"`
    SQL          string              `yaml:"sql,omitempty"`
    Perspectives []PerspectiveConfig `yaml:"perspectives,omitempty"`
}

type PerspectiveConfig struct {
    Name    string         `yaml:"name"`
    Columns []string       `yaml:"columns,omitempty"`
    Filters []FilterConfig `yaml:"filters,omitempty"`
    Sort    []SortConfig   `yaml:"sort,omitempty"` // array for future multi-column sort; only first element used today
}

type FilterConfig struct {
    Column   string `yaml:"column"`
    Operator string `yaml:"operator"` // "=", "like", ">", "<", ">=", "<="
    Value    any    `yaml:"value"`
}

type SortConfig struct {
    Column string `yaml:"column"`
    Desc   bool   `yaml:"desc,omitempty"`
}
```

Sort is a **`[]SortConfig` array** even though only `Sort[0]` is applied today — this avoids a schema migration when multi-column sort is added later.

`Load()` validation:
- Perspective names must be non-empty.
- A perspective on an `sql`-type dataset is a config error: `"dataset %q: perspectives are only supported on table datasets"`.

### Config layer — `Save` and `AppendPerspective`

Add `Save(path string, cfg *Config) error`. Writes atomically: marshal to YAML, write to `path+".tmp"`, rename. Creates parent directories with `os.MkdirAll`.

Add `AppendPerspective(path string, datasource string, tableName string, p PerspectiveConfig) error`:
1. Load current file (empty config if not found).
2. Find a `DatasetConfig` matching both `datasource` and `table` (blank `datasource` matches any).
3. If no match, create a minimal entry: `DatasetConfig{Name: tableName, Datasource: datasource, Table: tableName}`.
4. Upsert the perspective by name (replace if a perspective with the same name already exists; append otherwise).
5. Call `Save`.

The caller is responsible for choosing the path — see zero-config handling below.

### Core layer — `KindPerspective`

Add `KindPerspective Kind = "perspective"` to `internal/core/dataset/dataset.go`.

Extend `Dataset`:

```go
type Dataset struct {
    Name        string
    Table       string
    SQL         string
    Kind        Kind
    ParentTable string              // set for KindPerspective
    Preset      *QueryOptionsPreset // set for KindPerspective; nil otherwise
}

type QueryOptionsPreset struct {
    Columns []string
    Filters []Filter
    Sort    *Sort // nil if no sort configured
}
```

`dataset.Resolver` already converts `DatasetConfig` entries into `Dataset` values. Extend it to also resolve `Perspectives` into additional `Dataset` entries of kind `KindPerspective`, ordered immediately after their parent table in the flat list.

### TUI — `tui.Config`

Add `ConfigPath string` to `internal/tui/app.go`'s `Config` struct. Empty string = zero-config mode (no file loaded yet). `cmd/main.go` passes the path it actually loaded from, or `""`.

### TUI — save-perspective overlay

A new minimal overlay in `internal/tui/views/saveperspective.go`, similar to the page-size picker. It contains a single `textinput.Model` and an optional error message line.

**Keybinding:** `P` (uppercase). Available in the row browser when `result != nil` **and** the current dataset is `KindTable`, `KindView`, or `KindPerspective` (not `KindDataset`).

**Rendering:**
```
┌─ Save perspective ──────────────────┐
│  Name: █                            │
│                                     │
│  Enter confirm · Esc cancel         │
└─────────────────────────────────────┘
```

Error state (empty name submitted):
```
│  Name: █                            │
│  name is required                   │
│  Enter confirm · Esc cancel         │
```

**On confirm (`Enter`):**
1. Reject empty name — render `"name is required"` inline, keep overlay open.
2. Collect active state: `ColumnRegistry` visible columns (nil if all columns visible in schema order), filters from filter state, sort.
3. Determine the config path:
   - If `tui.Config.ConfigPath != ""`, use it.
   - Else try `~/.datacow/config.yaml` (create dir with `os.MkdirAll` if needed).
   - Else try `./datacow.yaml`.
   - If both fail, render the OS error inline, keep overlay open.
4. Call `config.AppendPerspective(path, activeDatasource, tableName, p)`.
5. On success: close overlay; emit a `PerspectiveSavedMsg{Path: path}` which the app handles to: show a brief status line `"Saved to <path>"`; update `tui.Config.ConfigPath` if it was previously empty; reload the dataset list so the new perspective appears in the schema explorer immediately.
6. On IO error: render the error inline, keep overlay open.

**`Esc`** closes without writing anything.

### TUI — schema explorer (`TableListModel`)

`KindPerspective` datasets need tree support:

- `FocusedExpandable()` must exclude `KindPerspective` (perspectives are leaf nodes, never expandable).
- A table node is expandable if it has perspectives **or** has schema columns/FKs (existing logic).
- `treeNode` gets a `perspectives []dataset.Dataset` field, populated from the full dataset list when the schema cache is ready.
- Perspective sub-lines render **above** the column/FK sub-lines. Each sub-line:
  ```
    ⊙ Failed Calls                    [P]
  ```
  The `⊙` glyph (or similar distinctive character) and `[P]` badge distinguish perspectives from columns.
- `datasetKindBadge(KindPerspective)` returns `"[P]"` styled distinctly (e.g. cyan).
- Navigating to a perspective sub-line and pressing `Enter` opens it in the row browser with `Preset` applied.

**Searchability:** perspective names participate in the existing filter query. A table whose any perspective name matches the query is treated as a name match. The matching perspective sub-lines are shown even if the parent is currently collapsed.

### TUI — row browser: opening a perspective

When a `KindPerspective` dataset is opened:
1. Pre-seed `ColumnRegistry` from `Preset.Columns` (nil = all visible in schema order, no pre-seed).
2. Pre-seed filter state from `Preset.Filters`.
3. Pre-seed sort from `Preset.Sort` (nil = no sort).
4. The pill bar shows filter/sort/cols pills as normal — they reflect the active preset state.
5. `P` key is **available** when the current dataset is `KindPerspective`. The overlay is pre-filled with the current perspective name so the user can confirm (overwrite) or rename (creates a sibling perspective). `AppendPerspective` is an upsert-by-name, so confirming with the same name updates the existing perspective.

## UX Summary

```
Schema explorer (table pane)         Row browser
─────────────────────────────        ─────────────────────────
▶ api_logs          [T]              # Opening "Failed Calls":
  ⊙ Failed Calls   [P]  ←─ Enter ─► cols: id,timestamp,result
  ⊙ Recent Errors  [P]              filter: result != 200
  ▸ Columns                         sort: timestamp ↓
    id
    ...
```

## YAML example

```yaml
datasets:
  - name: api_logs
    table: api_logs
    perspectives:
      - name: Failed Calls
        columns: [id, timestamp, method, result, error]
        filters:
          - { column: result, operator: "!=", value: 200 }
        sort:
          - { column: timestamp, desc: true }
      - name: Recent Errors
        filters:
          - { column: status, operator: "=", value: 500 }
```

## Acceptance Criteria

Tests follow the `TestAC_<SECTION><NN>_<description>` pattern (same convention as `columnpicker_acceptance_test.go`). The acceptance test file must open with a **coverage map** comment that maps every criterion below to the test(s) covering it.

### CF — Config (core tests in `config_test.go` / `dataset_test.go`)

- CF01: `Load()` parses `perspectives:` correctly — name, columns, filter fields, and the `sort` array are all populated on the returned `PerspectiveConfig`.
- CF02: `Load()` returns an error when an `sql`-type dataset has perspectives.
- CF03: `AppendPerspective` on a non-existent file creates the file; the written YAML contains the datasource entry (when non-empty) and the perspective nested under the table dataset.
- CF04: `AppendPerspective` on an existing file with no matching table appends a new table dataset entry containing the perspective.
- CF05: `AppendPerspective` upsert — same name replaces existing; different name appends alongside. After upsert, reading the file back via `Load()` returns exactly the expected perspectives list.
- CF06: `Save` writes atomically — no `.tmp` file remains after a successful call; file contents are valid YAML parseable by `Load()`.
- CF07: `Resolver` with a table dataset that has two perspectives emits them as `KindPerspective` entries immediately after the parent in the flat list; `Preset.Columns`, `Preset.Filters`, and `Preset.Sort` match the config values.

### SP — Save-perspective overlay (view unit tests in `saveperspective_test.go`)

- SP01: `View()` of a freshly opened overlay contains `"Save perspective"`, a cursor/input area, and both `"Enter confirm"` and `"Esc cancel"`.
- SP02: Sending `Enter` with an empty name input renders `"name is required"` in the view and does **not** emit a close/save message — the overlay stays open.
- SP03: `P` key is present in `keys.Map` with value `"P"`, wired in the row browser's `Update()`, and present in `helpoverlay.go`. The help bar always shows `"P"` regardless of dataset kind.
- SP04: `P` key press on a `KindPerspective` dataset **does** open the save overlay, pre-filled with the perspective name — the rendered view must contain `"Save perspective"` and the perspective name.

### TL — Table list perspectives (view unit tests in `tablelist_test.go` or `tablelist_perspectives_test.go`)

- TL01: A `TableListModel` loaded with a table dataset that has one perspective renders an expand indicator on the table row (same character used for tables with columns).
- TL02: After expanding the table, the rendered view contains the perspective name and `"[P]"`. Perspective sub-lines appear before column sub-lines.
- TL03: Filter query `"failed"` applied to a model where `"api_logs"` has a perspective named `"Failed Calls"` — the table row is visible in the filtered output and the rendered view contains `"Failed Calls"`.
- TL04: When the cursor is on a perspective sub-line, `FocusedExpandable()` returns false.
- TL05: `datasetKindBadge(dataset.KindPerspective)` returns a non-empty string containing `"P"`.

### RB — Row browser pre-seeding (view unit tests in `rowbrowser_test.go`)

- RB01: Opening a `KindPerspective` dataset with `Preset.Columns = ["id", "name"]` and a 3-column result → view contains `"cols 2/3"` pill; `"extra"` column header is absent from the rendered table.
- RB02: Opening a `KindPerspective` dataset with `Preset.Filters = [{Column:"result", Operator:"!=", Value:200}]` → view contains a filter pill with `"result"` and `"200"` (or `"!="`) visible.
- RB03: Opening a `KindPerspective` dataset with `Preset.Sort = &Sort{Column:"timestamp", Desc:true}` → view contains the sort pill with `"timestamp"` and a descending indicator (`"↓"`).
- RB04: Sending `P` key while viewing a `KindPerspective` dataset → `View()` contains `"Save perspective"` and the perspective name (overlay is pre-filled).
- RB05: Switching from a `KindPerspective` dataset to a plain `KindTable` dataset (via `TablesLoadedMsg` or dataset swap) → sending `P` opens the overlay (view contains `"Save perspective"`).

### AC — App integration tests (in `app_test.go` or `saveperspective_acceptance_test.go`)

- AC01 (end-to-end save): Load a table into the row browser with a result. Press `P`, type a name, press `Enter`. Assert: `View()` no longer contains `"Save perspective"`; `View()` contains `"Saved to"` followed by a non-empty path string.
- AC02 (schema explorer refresh after save): After AC01, the table list `View()` contains an expand indicator for the parent table and, after sending the expand key, contains the perspective name.
- AC03 (navigate to perspective): After AC02, navigate cursor to the perspective sub-line and press `Enter`. Assert: row browser `View()` contains the filter pill value that was active when saving (proving pre-seeding works end-to-end).
- AC04 (P pre-filled on perspective): With a perspective open in the row browser, send `P`. Assert: `View()` contains `"Save perspective"` and the perspective name. Confirm with Enter; assert overlay closes and `"Saved to"` appears (perspective is updated).
- AC05 (zero-config file creation): Start the TUI with `ConfigPath = ""` and a temp dir as CWD. Save a perspective. Assert: `tui.Config.ConfigPath` is non-empty after the save; the file at that path exists and `config.Load()` on it returns the expected `PerspectiveConfig`.

## What NOT to Change

- Editing or deleting a perspective from within the TUI — defer to a future task; users hand-edit the YAML.
- SQL datasets (`KindDataset`) — perspectives are table-origin only.
- `PageSizeRegistry` — no changes.
- Filter/sort validation in `QueryOptions` — still validated against full schema, not the perspective's column projection.
- The `COUNT(*)` subquery — unaffected.

## Definition of Done

See `tasks/definition-of-done.md`. Invoke `/done` after all acceptance criteria are met.
