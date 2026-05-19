# TUI: Filter / Search Redesign

Split today's single "filter" feature into two distinct concepts with clearer
keybindings, and replace the inline-prompt + pill-editing flow with a proper
modal that uses schema knowledge to help the user build correct WHERE clauses.

Today there is one "filter" feature on `/`: a single-line prompt parses
`column=value`, filters become pills, `Tab` enters a pill-edit mode, `x`
removes a pill. The prompt is unforgiving, has no column completion, no
type-awareness on values, and the pill-edit mode is awkward and buggy.

The redesign:

- **Query Filter** (was `/`, now `q`) — builds the SQL `WHERE` clause. Server-side. Opens a modal with schema-aware column completion, type-aware value help, a live SQL preview, and clean edit/remove of existing filters.
- **Local Search** (now `/`) — k9s-style. Substring match across all visible cells in the currently-loaded page. No round-trip to the DB.
- **Quick-filter from cell** (new, `=`) — opens the Query Filter modal pre-filled with `<column> = <selected cell value>`.

The internal SQL surface does not grow: `dataset.Filter`, `dataset.QueryOptions`,
and the existing six operators (`=`, `like`, `>`, `<`, `>=`, `<=`) stay
exactly as they are. This is a UX-layer task.

## Keybinding Changes

| Key | Before | After |
|---|---|---|
| `/` | Open inline filter prompt | Open local search (page-only, k9s-style) |
| `q` | Quit (global) | Open Query Filter modal (row browser only) |
| `Q` | — | Quit (global, replaces lowercase `q`) |
| `=` | — | Quick-filter from selected cell |
| `n` / `N` | — | Next / previous match while local search is active |
| `Tab` | Enter pill-edit mode | (Removed — pill mode is gone) |
| `x` | Remove focused pill | (Removed — pill mode is gone) |
| `s` | Cycle server-side sort | Unchanged (local sort split is a separate task) |
| `Esc` | Exit filter mode | Close modal / clear local search (depending on context) |

`Ctrl+C` continues to quit globally. `q` outside the row browser remains "quit"
for this task — only the row browser context rebinds `q` to the Query Filter
modal. The help overlay must reflect both bindings.

## Query Filter Modal

Opens on `q` (or `=` from a cell). Centered modal over the row browser; the
table dims behind it. Existing filters are loaded into the modal on open.

```
┌─ Query Filter · public.orders ──────────────────────────────────────┐
│                                                                     │
│ Active filters:                                                     │
│   1 ▸ status        =      'active'                                 │
│   2   created_at    >      '2024-01-01'                             │
│                                                                     │
│ Edit / add filter:                                                  │
│   Column  [status_____________▾]                                    │
│   Op      [=  ▾]                                                    │
│   Value   ['active'_]                                               │
│                                                                     │
│ ─ Help ────────────────────────────────────────────────────────────│
│ status: text NOT NULL · ops: = like                                 │
│ Tip: text values use single quotes. like accepts % wildcards.       │
│                                                                     │
│ ─ SQL preview ─────────────────────────────────────────────────────│
│   SELECT * FROM public.orders                                       │
│   WHERE status = $1 AND created_at > $2                             │
│   ORDER BY id ASC                                                   │
│   LIMIT 50 OFFSET 0                                                 │
│                                                                     │
│ ↑↓ select filter · Enter edit · d delete · Tab next field           │
│ Ctrl+Enter apply · Esc cancel                                       │
└─────────────────────────────────────────────────────────────────────┘
```

### Layout regions

- **Active filters** — numbered list of filters in the order they will be
  AND-joined. The selected row is highlighted (cursor `▸`). Empty when there
  are no filters; show `(none)` placeholder.
- **Edit / add filter** — three fields: Column (text input with completion),
  Op (cycle/select), Value (text input). Tab cycles fields; Shift+Tab cycles
  backward. The form is the only way to add or change a filter.
- **Help** — single-paragraph contextual hint, recomputed whenever the focused
  field or selected column changes. Shows: column type, allowed operators for
  that type, and one type-specific tip (see "Type-aware help" below).
- **SQL preview** — live re-render whenever the active-filter list changes
  (not on every keystroke in the edit form — only after the user commits an
  add/edit). Show the final query that will be sent: parameterized
  placeholders (`$1`, `?` etc., matching the active driver's placeholder
  style), all active filters AND-joined, current sort, current page's
  `LIMIT` / `OFFSET`.

### Active-filter list interactions

- `↑` / `↓` (or `j` / `k`) move the cursor through the active-filter list.
- `Enter` on a selected filter loads it into the edit form (column, op,
  value all populated). Submitting the form (`Enter` from the Value field
  when no filter is being added) **replaces** filter #N rather than
  appending.
- `d` (or `Delete`/`Backspace`) on a selected filter removes it from the
  list. No confirmation — `Esc` discards all unsubmitted changes anyway.
- The user can mix add and edit operations freely before applying; all
  changes are batched until `Ctrl+Enter`.

### Add / edit form interactions

- **Column field** — typing filters a dropdown of column names from
  `m.result.Columns`. Down/Up navigate the dropdown; `Tab` or `Enter`
  accepts the highlighted entry. Unknown column names are rejected on
  submit (validation error in Help region) — no SQL is sent for invalid
  columns.
- **Op field** — cycles through the operators **valid for the selected
  column's type** (see mapping below). Left/Right (or `Space`) cycle;
  `Tab` moves to Value.
- **Value field** — type-aware (see "Type-aware help" below). Free-text
  for text/date/timestamp; numeric-only filter for integer/numeric types
  (silently drops disallowed keystrokes); pre-fills `''` with cursor
  inside for text types.
- `Enter` from the Value field submits the form: either appends a new
  filter or replaces the filter currently selected in the active-filter
  list, then clears the form back to a blank "add" state.

### Apply / cancel

- `Ctrl+Enter` (anywhere in the modal) applies all pending changes:
  closes the modal, replaces `rowBrowserModel.filters` with the modal's
  active-filter list, and triggers `loadPageCmd(1)`.
- `Esc` (anywhere in the modal) discards all pending changes and closes
  the modal. `rowBrowserModel.filters` is untouched.

### Quick-filter entry (`=` from a cell)

Pressing `=` while a cell is focused in the row browser opens the modal
with:

- The active-filter list showing all currently-applied filters (unchanged).
- The add/edit form pre-populated with:
  - Column = the focused column name
  - Op = `=`
  - Value = the cell's raw value, properly quoted/formatted for the
    column's type (text → `'value'` with cursor positioned just before
    the closing quote; numeric → bare number; null cells → ignored, do
    not open the modal at all and show a brief status message
    `"= cannot filter on NULL"`).

The user can hit `Enter` immediately to add the filter, then `Ctrl+Enter`
to apply — two keystrokes from cell to filtered result.

## Type-aware help

Use `db.Column.Type` (already in `m.result.Columns`) to drive the help and
the value-field behavior. Map driver type strings into broad categories;
unknown types fall through to "text" behavior.

| Category | Matches (case-insensitive contains) | Allowed ops | Value field behavior | Tip |
|---|---|---|---|---|
| text | `text`, `char`, `varchar`, `string`, `uuid`, `json`, `jsonb` | `=`, `like` | Pre-fill `''`, cursor inside; accept any character | "text values use single quotes. like accepts `%` wildcards." |
| integer | `int`, `serial`, `bigint`, `smallint` | `=`, `>`, `<`, `>=`, `<=` | Allow only digits and leading `-` | "integer column. type a whole number." |
| numeric | `numeric`, `decimal`, `float`, `double`, `real` | `=`, `>`, `<`, `>=`, `<=` | Allow digits, `-`, single `.` | "numeric column. decimals allowed." |
| date/time | `date`, `time`, `timestamp` | `=`, `>`, `<`, `>=`, `<=` | Free text; suggest ISO format | "use ISO format, e.g. `'2024-01-15'` or `'2024-01-15 10:00:00'`." |
| boolean | `bool` | `=` | Cycle between `true` / `false` on any keystroke | "boolean column. value is true or false." |

The mapping lives in `internal/tui/views/filtermodal_typehints.go` and is
fully unit-tested — both the category resolver and the per-category allowed
ops / tip. The category set is intentionally coarse; new drivers can extend
it later without rewriting the modal.

This task does **not** add new SQL operators. If the column type would
benefit from operators we don't support yet (e.g. `ilike`, `is null`),
that's a follow-up task.

## Local Search (`/`)

Press `/` in the row browser → a single-line input docks at the bottom of
the table. As the user types, all cells in the currently-loaded page are
matched case-insensitively against the substring; rows containing at
least one matching cell are marked as "matches". Non-matching rows are
dimmed (not removed — context matters in a tabular view). Matching
substrings within matching rows are highlighted.

- `n` jumps the cursor to the next matching row (wraps).
- `N` jumps to the previous matching row (wraps).
- `Enter` exits the search input but keeps the highlight + `n`/`N`
  navigation active.
- `Esc` clears the search and removes all highlighting/dimming.
- Status bar shows `search: "foo"  3/12 matches` while active.
- Changing page (`[` / `]`) clears the search — it's scoped to the
  loaded page.
- Applying a new query filter clears the search.

Local search is implemented purely in `rowbrowser.go` rendering — no DB
calls, no changes to `dataset` or `core`.

## Files

| File | Change |
|---|---|
| `internal/tui/views/filtermodal.go` | New — `FilterModalModel`, `View()`, `Update()`, list navigation, form, dropdown |
| `internal/tui/views/filtermodal_typehints.go` | New — type-category resolver, per-category ops/tips, value-field validators |
| `internal/tui/views/filtermodal_test.go` | New — modal interactions: open with existing filters, add/edit/delete, validation, quick-filter prefill, apply/cancel, SQL preview correctness |
| `internal/tui/views/filtermodal_typehints_test.go` | New — table-driven tests for category resolution and allowed ops per category |
| `internal/tui/views/localsearch.go` | New — local-search state, match computation, highlight rendering helpers |
| `internal/tui/views/localsearch_test.go` | New — substring match across mixed types, `n`/`N` wrap-around, clearing on page change |
| `internal/tui/views/rowbrowser.go` | Wire `q` → open modal, `/` → local search, `=` → quick filter, `n`/`N` while search active, remove `Tab`/`x` pill paths, integrate modal apply into `loadPageCmd` |
| `internal/tui/views/rowbrowser_test.go` | Update — remove pill-mode tests, add tests for new keybindings and local-search flow |
| `internal/tui/views/filter.go`, `filter_test.go` | Delete — replaced entirely |
| `internal/tui/keys/keys.go` | Replace `Filter`/`RemoveFilter`/`FilterPills`; add `QueryFilter` (`q`), `LocalSearch` (`/`), `QuickFilterCell` (`=`), `NextMatch` (`n`), `PrevMatch` (`N`); rebind global `Quit` to `Q`/`Ctrl+C` |
| `internal/tui/views/helpoverlay.go` | Update — split filter help into "Query Filter (`q`)" and "Local Search (`/`)" sections, document `=`, `n`, `N`, `Q` |
| `internal/tui/app.go` | Add modal screen state, route quick-filter cell context into modal |

## Acceptance Criteria

### Keybindings & help

- [ ] `q` in the row browser opens the Query Filter modal.
- [ ] `q` is no longer the global Quit. `Q` and `Ctrl+C` quit the app.
- [ ] `/` in the row browser opens local search, not the old filter prompt.
- [ ] `=` on a focused non-null cell opens the modal pre-filled with `column = <value>`.
- [ ] `=` on a NULL cell shows a transient status message and does not open the modal.
- [ ] `Tab` and `x` no longer have filter-related behaviour in the row browser.
- [ ] Help overlay lists `q`, `/`, `=`, `n`, `N`, `Q` with descriptions matching their actions.

### Query Filter modal

- [ ] Opening the modal shows the row browser's currently-applied filters in the active-filter list, in order.
- [ ] Column field offers completion from `m.result.Columns`; submitting an unknown column shows a validation message in the Help region and does not add the filter.
- [ ] Op field only offers operators valid for the selected column's type category. Switching column resets Op to the first valid operator for the new type if the previous Op is no longer valid.
- [ ] Value field enforces per-type input rules (integer rejects letters, numeric allows one `.`, text accepts any character, boolean cycles `true`/`false`).
- [ ] Text columns pre-fill `''` with cursor inside the quotes.
- [ ] `Enter` on a selected active-filter loads it into the form; submitting replaces that filter rather than appending.
- [ ] `d` on a selected active-filter removes it from the list.
- [ ] SQL preview reflects the active-filter list, current sort, and current page's `LIMIT`/`OFFSET`, using the active driver's placeholder style.
- [ ] `Ctrl+Enter` applies all changes and triggers a single `loadPageCmd(1)`.
- [ ] `Esc` discards pending changes and leaves the row browser's filters unchanged.

### Local search

- [ ] `/` docks an input at the bottom; typing live-highlights matching rows and dims non-matches; status bar shows `search: "X"  M/N matches`.
- [ ] `n` / `N` move the cursor to next/previous matching row, wrapping at the ends.
- [ ] Match is case-insensitive substring across the rendered string form of every cell in the loaded page.
- [ ] `Enter` exits the input and keeps highlights + navigation active.
- [ ] `Esc`, paging, and applying a query filter all clear the search state.

### Tests

- [ ] New `filtermodal_test.go`, `filtermodal_typehints_test.go`, `localsearch_test.go` cover the rules above (rendering invariants via direct `View()` calls; teatest for the open-modal-from-`q`, open-from-`=`, and apply round-trip flows).
- [ ] `rowbrowser_test.go` updated; no tests of the removed pill-mode or inline-prompt remain.
- [ ] Help overlay tests assert presence of the new entries.

### Definition of done

- [ ] `make preflight` passes.
- [ ] `/done` checks all pass.

## What NOT to Change

- **`dataset.Filter`, `dataset.QueryOptions`, `executor.go`** — the SQL surface is untouched. No new operators, no `IS NULL`, no `ilike`, no multi-column sort, no AND/OR composition.
- **Server-side sort (`s`)** — same key, same behavior. Local sort is a separate task.
- **Schema cache / `db.Column` / `Describe`** — consume what's already there; do not add new fields.
- **Export, FK drill-down, schema explorer, cell viewer, query log, multi-datasource picker** — unrelated, leave alone.
- **"Save as dataset" / editing named datasets** — explicitly out of scope. The modal is designed to be extensible toward this later, but no UI hooks for save/load in this task.
- **API/web layer** — TUI-only task.
- **Global `q` outside the row browser** — other screens may continue to treat `q` as quit if they do today; only the row browser remaps it. The global Quit binding (`Q` / `Ctrl+C`) is the canonical way to quit everywhere.

## Definition of Done

See [definition-of-done.md](../definition-of-done.md). All gates must pass.

## As Implemented

Decisions and divergences from the spec above.

**SQL preview removed.** The spec included a live SQL preview section in the modal. Removed during implementation: the preview was a separate code path from `Executor.Query()`, so it could silently produce different SQL from what actually executes. The query log already shows the real SQL that ran. There is no value in a preview that might lie.

**Form fields rendered on one line.** The spec showed Column, Op, and Value as three stacked rows. Implemented as a single inline row — `> [column…  ] [= ▾] [value…]` — which is more compact and reads left-to-right like a WHERE clause.

**Text values are not pre-filled with `''`.** The spec said text/datetime fields should pre-fill `''` with the cursor inside the quotes. Removed: filter values are SQL parameters, not interpolated strings. Wrapping them in quotes means the quotes become part of the value and nothing matches. The type tip explains this.

**JSON type is a separate category.** The spec grouped `json`/`jsonb` under text (ops: `=`, `like`). `like` on a JSON column produces a DB error. Added `typeCatJSON` → ops: `=` only.

**Local search filters rows, not dims them.** The spec said non-matching rows are dimmed. Implemented as filtering: only matching rows are shown. Dimming keeps context but makes it hard to scan; hiding rows is how k9s-style search works.

**Quick-filter value is not single-quoted.** The spec said text cells should pre-fill as `'value'`. Same reason as above — values are parameters, not SQL literals. The cell value is placed directly in the value field.
