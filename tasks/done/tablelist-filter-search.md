# TUI: Filter the Tables List with `/`

## Implementation Notes

The following deliberate deviations from the original spec were made during implementation:

1. **B04 — Sub-matched datasets stay collapsed, not auto-expanded.** The spec said datasets visible only via a column/FK/index match should be auto-expanded. The implementation shows them collapsed instead. Reason: short queries (e.g. "id", "email") match many column names across many tables; bursting every matching tree open on every keystroke is visually jarring and makes the list unreadable. The user can expand manually.

2. **B08 — Navigation works while the input is still open.** The spec only described navigation working after Enter (filter held, input closed). The implementation also routes ↑/↓ to the filtered list while the input is still open, so the user does not have to press Enter before navigating. This is strictly additive — the spec-described behaviour (navigation after Enter) still works.

Add a k9s-style live filter to the table list view (left pane). Press `/`,
type a query, and the list narrows in place as you type. The match is deep:
it hits the dataset name as well as the column, FK, and index names that
appear in the expandable tree under each row, so the user can find a table
by something they remember about its schema, not just by its name. `Esc`
clears the filter and closes the input.

## Background

Today the only way to find a specific table among many is `↑/↓` scrolling
or the global `Ctrl+P` Goto overlay. Goto is a fuzzy modal across
*everything* (tables, columns, datasources); it floats over the layout and
takes the user away from where they were. A `/`-style inline filter is the
right tool when the user is already in the tables pane and just wants to
narrow what they see — same affordance k9s, lazygit, and the row browser
already use.

`/` is unused in the table list context today. The row browser's `/`
binding is unaffected (and is independently being remapped by
[filter-search-redesign](filter-search-redesign.md); these two tasks can
land in either order).

## UX

### State machine

```
        ┌──────────────┐   /    ┌────────────────────┐
        │  list (idle) │───────▶│ list + filter input│
        └──────────────┘        └────────┬───────────┘
              ▲                       ↑/↓ (works while input open)
              │ Esc                      │ Enter
              │                          ▼
              │                 ┌────────────────────┐
              └─────────────────│ list (filter held) │
                       Esc      └────────┬───────────┘
                                         │ /
                                         ▼
                              (re-open input pre-filled)
```

- **`/`** — open a single-line input docked at the bottom of the tables
  pane. If a filter is already held, pre-fill the input with it and place
  the cursor at the end.
- **typing** — re-run the match on every keystroke; the list re-renders
  immediately. Matching substrings in the dataset name and in any
  matching child row (column / FK / index) are highlighted.
- **`↑/↓` while input is open** — navigate the filtered list without
  needing to press Enter first. The input stays open and focused.
- **`Enter`** — blur the input, keep the filter held. Focus returns to the
  list; `↑/↓/Enter` work as usual against the filtered set. Status bar
  shows `filter: "X"  M/N`.
- **`Esc`** — clear the filter completely and close the input. Full list
  restored.
- **switching pane / leaving the tables view** — clear the filter. The
  filter is scoped to the current visit of the tables pane.

### Match rules

Case-insensitive substring match. A dataset is included in the filtered
view when *any* of the following contain the query string:

| Source | Provided by | Always available? |
|---|---|---|
| Dataset name (`ds.Name`) | `tableListModel.datasets` | Yes |
| Column names on the underlying table | `schema.Cache` | Yes once cache is ready |
| Foreign-key target table names | `schema.Cache` | Yes once cache is ready |
| Index names on the underlying table | `schema.Cache` (extended — see Files) | Yes once cache is ready |

Substring — not fuzzy. Live filtering needs to be predictable; fuzzy
matching is appropriate for the Goto modal but feels jittery as you type
in an inline filter.

If the schema cache is not yet loaded when `/` is pressed, fall back to
matching dataset names only and show a subtle hint in the input footer:
`(schema loading — name match only)`. Once the cache becomes ready, the
filter re-evaluates automatically.

### Filtered-view rendering

- Non-matching datasets are **hidden**, not dimmed. The list stays tight.
- A dataset that matches only via a sub-item (column / FK / index) is
  **shown collapsed** — the user expands manually to see the matching
  sub-row. _(Deviation from original spec which said "auto-expanded":
  short queries like "id" or "email" can hit many column names; bursting
  every matched tree open on each keystroke is jarring. See Implementation
  Notes at the top of this file.)_
- A dataset whose name matches directly is shown collapsed (default
  state), unchanged from today. The user can expand manually if they
  want.
- Matching substrings in any visible row (header or sub-row) are rendered
  in a highlight style (reuse the search-highlight style from the goto
  overlay if one exists; otherwise add one in `internal/tui/style`).

### Cursor behavior

- Opening the input does not move the cursor.
- Whenever the filter changes (including the open-with-empty-query
  moment), if the current cursor's dataset is no longer in the filtered
  list, snap the cursor to the first visible dataset header. If the
  filtered list is empty, the cursor sits past the end and `Enter` is a
  no-op.
- Closing the input via `Esc` restores the previous cursor position only
  if that dataset still exists (which it does — `Esc` clears the filter,
  full list returns); preserve cursor identity by name, not by index.

### Empty result

When the filter matches nothing, render a single line in the list area:
`No tables match "<query>"`. Sub-rows / tree drawing are suppressed.

## Keybindings

Add one new binding. Do not touch any other keys.

| Key | Where | Action |
|---|---|---|
| `/` | Table list (idle or filter-held) | Open filter input (pre-filled if held) |
| `Esc` | Filter input open OR filter held | Clear filter, close input |
| `Enter` | Filter input open | Blur input, keep filter held, move focus into list |

In `keys.Map`, add `TableListFilter` with key `/` and help `"/ filter"`.
Do not reuse `Filter` (that one is row-browser-only and is being
reshaped by [filter-search-redesign](filter-search-redesign.md)).

The status bar / help overlay must list `/` for "filter tables" in the
table-list context. Update `keys.TableListHelp()` and `keys.FullHelp()`.

## Files

| File | Change |
|---|---|
| `internal/core/schema/schema.go` | Add `Indexes []db.Index` to `Table`; populate in `Load` via `client.Indexes`. Skip indexes for views (consistent with the lazy path in `tablelist.go`). |
| `internal/core/schema/schema_test.go` | Extend `Load` test to assert indexes are populated. |
| `internal/core/schema/cache_test.go` | Adjust if any test asserts the shape of `Table`. |
| `internal/tui/views/tablelist.go` | Plumb a `*schema.Cache`; new filter state (input model, query, held flag); filter computation + match-source aggregation; auto-expand-on-sub-match; cursor preservation; render highlights; render filter input footer; route `/`, `Esc`, `Enter` only when input is open. |
| `internal/tui/views/tablelist_test.go` | Direct `View()` tests for: idle list unchanged, `/` opens input, typing narrows list, sub-match auto-expands, no-match placeholder, `Esc` restores, cache-not-ready hint shown, cursor snaps when current row drops out. |
| `internal/tui/app.go` | Pass `schemaCache` into `NewTableListModel`. Wire focus-leave-clears-filter when transitioning from the table list pane. |
| `internal/tui/keys/keys.go` | Add `TableListFilter` binding (`/`). Add to `TableListHelp()` and the appropriate group in `FullHelp()`. |
| `internal/tui/views/helpoverlay.go` | Add entry for the new binding. |
| `internal/tui/views/helpoverlay_test.go` | Assert the new entry is rendered. |
| `internal/tui/style/style.go` (or wherever match highlight lives) | Add `SearchHighlight` style if not already present; reuse from goto if it exists. |

The `TableListModel` constructor signature changes; update every call
site (app.go, tests) accordingly.

## Acceptance Criteria

### Behaviour

- [x] `/` in the table list opens a single-line input docked at the bottom of the tables pane. The input is empty on first open, or pre-filled with the held filter on re-open.
- [x] Typing into the input filters the visible datasets live (every keystroke). Match is case-insensitive substring against dataset names, column names, FK target table names, and index names sourced from the schema cache.
- [x] When the cache is not yet ready, the filter matches against dataset names only and the input footer shows `(schema loading — name match only)`. Once the cache becomes ready, the filter re-evaluates without user action.
- [x] Datasets that match only via a sub-item (column / FK / index) are **shown collapsed** (not auto-expanded — see Implementation Notes). Matching substrings in visible rows are highlighted.
- [x] Datasets whose name matches directly are shown in their current expand state (default: collapsed), with the matching substring in the name highlighted.
- [x] Non-matching datasets are hidden, not dimmed.
- [x] When no dataset matches, the list area renders a single `No tables match "<query>"` line.
- [x] `Enter` blurs the input, keeps the filter held, and moves focus back to the list. `↑/↓` navigate the filtered set both while the input is open and after Enter (additive to spec).
- [x] `Esc` clears the filter, closes the input, and restores the full list. The previous cursor row (by name) is reselected if it still exists; otherwise the cursor lands on the first dataset.
- [x] Switching focus to another pane, or leaving the tables view, clears the filter.
- [x] The status bar shows `filter: "X"  M/N` while a filter is held (input open or closed). `M` is the count of matching datasets; `N` is the total.

### Schema cache

- [x] `schema.Table` gains an `Indexes []db.Index` field, populated by `schema.Load`. Views skip the index lookup (consistent with `tablelist.go`'s existing branch).
- [x] No call sites of `schema.Load` regress.

### Keys & help

- [x] `keys.Map` has a `TableListFilter` binding bound to `/`.
- [x] `keys.TableListHelp()` includes the new binding.
- [x] The full help overlay lists `/ filter tables` in the table-list group.
- [x] No other key behaviour is changed. In particular, `Ctrl+P` (Goto), `i` (table info), `Ctrl+R` (refresh schema), and the row-browser `/` are untouched. While the filter input is open, global keys are consumed by the filter input, not the app.

### Tests

- [x] `tablelist_acceptance_test.go` covers every acceptance criterion via `TestAC_*` tests using direct `View()` calls — opening the input, narrowing on type, sub-match visible-collapsed, no-match placeholder, cache-not-ready hint, cursor snap, cursor restore on `Esc`, navigation while input open, navigation after Enter, filter status, key binding, TableListHelp.
- [x] `tablelist_test.go` covers additional rendering invariants and edge cases.
- [x] `schema_test.go` covers `Indexes` populated by `Load`.
- [x] `app_test.go` teatest smoke tests cover: column sub-match end-to-end (`TestApp_TableListFilter_ColumnSubMatch`), global key blocking (`TestApp_TableListFilter_BlocksGlobalKeys` / `TestAC_KH04`), leaving-pane clears filter (`TestAC_B10_LeavingPaneClearsFilter`).

### Definition of done

- [ ] `make preflight` passes.
- [ ] `/done` checks all pass.

## What NOT to Change

- **Goto modal (`Ctrl+P`)** — unrelated; remains the global fuzzy navigator. Do not merge or unify with this filter.
- **Row browser `/`** — the row browser's filter/search is being reshaped by [filter-search-redesign](filter-search-redesign.md). Do not touch it here, and do not reuse its key bindings (`Filter`, `LocalSearch`, etc.) — add a new `TableListFilter` binding instead so the two tasks remain independent.
- **Schema explorer pane** — its tree and behaviour are not part of this task.
- **Match algorithm** — substring only; do not introduce fuzzy matching, regex, or boolean operators.
- **Schema cache loading model** — do not change *when* the cache loads (still background after datasource activation). Only extend *what* it stores (add indexes).
- **`db.Client` interface** — no new methods. `Indexes` already exists.
- **YAML SQL datasets (`KindDataset`)** — they have no underlying table; match them on `Name` only. Do not attempt to introspect their columns.
- **`tablelist.go` lazy expansion path** — the existing `loadExpansionCmd` / `loadIndexesCmd` flow stays. Filtering reads from the cache; manual expansion still triggers the lazy path for any not-yet-cached datasets (e.g. while the initial cache load is in flight).
- **API / web layer** — TUI-only task.

## Definition of Done

See [definition-of-done.md](../definition-of-done.md). All gates must pass.
