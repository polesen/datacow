# Multi-Column Sort

Replace the single-column sort cycle with a sort manager overlay that lets users build an ordered list of sort levels. Sort is applied as a multi-column `ORDER BY` clause.

## Background

The current `s`-key gesture cycles a single column through ASC → DESC → off. It works well for one-column sort but has no path to secondary sort levels. This task replaces the gesture with a sort manager overlay (similar to the column picker) and changes the underlying `QueryOptions.Sort` from `*Sort` to `[]Sort`.

The `save-table-perspective` task already uses `[]SortConfig` in `PerspectiveConfig.Sort` in anticipation of this change. The core model needs to align.

## Architecture

### Core layer — `QueryOptions.Sort []Sort`

Change `QueryOptions` in `internal/core/dataset/dataset.go`:

```go
// Sort is the ordered list of sort levels applied as ORDER BY.
// nil or empty means no sort (natural table order).
Sort []Sort
```

The `Sort` struct itself is unchanged: `{Column string; Desc bool}`.

**SQL builder**: when `len(Sort) > 0`, emit:
```sql
ORDER BY col1 ASC, col2 DESC, col3 ASC
```
Column names go through the same whitelist validation (`identRe` + `colSet` check) already applied to single-sort today.

**Empty/nil `Sort`**: no `ORDER BY` clause — existing behaviour.

All callers that read `QueryOptions.Sort` need updating. The executor, export, and any test that constructs `QueryOptions` with a non-nil `Sort` field are affected.

### Core layer — `QueryOptionsPreset.Sort` (perspectives alignment)

In `internal/core/dataset/dataset.go`, `QueryOptionsPreset.Sort` changes from `*Sort` to `[]Sort` to match. The `save-table-perspective` task spec uses `[]SortConfig` in `PerspectiveConfig` — the in-memory representation should match that shape.

### TUI — `SortManagerModel`

New component: `internal/tui/views/sortmanager.go`.

The model holds:
- `active []dataset.Sort` — the current sort levels, in priority order
- `available []string` — column names not yet in `active`, in schema order
- `cursor int` — position across both sections (0 = first active entry, `len(active)` = first available entry, etc.)

**Rendering:**
```
┌─ Sort ──────────────────────────────────────────┐
│  J/K reorder · Space dir · Del remove           │
│  Enter confirm · Esc cancel                     │
│                                                  │
│  Active                                          │
│  1. name          ↑                              │
│> 2. created_at    ↓                              │
│  ─────────────────────────────────────────────── │
│  Available                                       │
│  id                                              │
│  status                                          │
│  email                                           │
└─────────────────────────────────────────────────┘
```

When no active sorts exist, skip the `Active` section header and divider, show only `Available`. The hint line adjusts: `Space add · Enter confirm · Esc cancel`.

**Keys:**

| Key | Context | Action |
|-----|---------|--------|
| `↑` / `↓` | anywhere | move cursor |
| `Space` | active entry | toggle direction (ASC ↔ DESC) |
| `Space` | available entry | add column as the next sort level; cursor moves to it in the active section |
| `J` | active entry | move entry down (lower priority); no-op on last active |
| `K` | active entry | move entry up (higher priority); no-op on first active |
| `Del` or `d` | active entry | remove entry; cursor stays at same position or moves up |
| `Enter` | anywhere | confirm, close, trigger re-fetch |
| `Esc` | anywhere | cancel, revert to previous sort state without re-fetching |

**Opening the overlay:** The overlay is opened progressively — it only appears when it's actually needed. The `s` key behaviour depends on context:

| Condition | Behaviour |
|-----------|-----------|
| No current sort | `cycleSort()` as today: no sort → ASC → DESC → off. No overlay. |
| Sort active on the **same** column as cursor | `cycleSort()` as today: cycle direction or remove. No overlay. |
| Sort active on a **different** column than cursor | Open the sort manager, pre-populated with the existing sort level(s) **and the cursor column already added** as the next level. |
| `S` (uppercase) | Always open the sort manager, regardless of current state. Use this to reorder, remove, or inspect sort levels without pressing `s` on a new column first. |

When the overlay opens due to a second column (`s` on a different column), the new column is already in the `Active` section so the user can immediately adjust its direction or reorder — they do not need to add it manually.

**On confirm:** emit a `SortConfirmedMsg{Sort: []dataset.Sort}` that the row browser handles — updates `m.sort`, closes the overlay, triggers a re-fetch.

**On cancel (`Esc`):** close overlay, no state change, no re-fetch.

### TUI — row browser changes

`cycleSort()` is kept but extended: it is only called when the conditions above do not trigger the overlay.

`m.sort` changes type from `*dataset.Sort` to `[]dataset.Sort`.

`buildQueryOptions()` passes the slice directly to `QueryOptions.Sort`.

`m.sort` changes type from `*dataset.Sort` to `[]dataset.Sort`.

`buildQueryOptions()` passes the slice directly to `QueryOptions.Sort`.

### TUI — column headers

`buildHeader()` currently marks the sorted column with `↑` or `↓`. Change to accept `[]dataset.Sort` and annotate each sorted column with a priority superscript:

| Sort level | ASC marker | DESC marker |
|-----------|-----------|------------|
| 1st | `↑¹` | `↓¹` |
| 2nd | `↑²` | `↓²` |
| 3rd | `↑³` | `↓³` |
| 4th+ | `↑⁴` etc. | `↓⁴` etc. |

Unicode superscripts: `¹²³⁴⁵⁶⁷⁸⁹`. Unsorted columns are unmarked.

### TUI — pill bar

`renderActivePills()` currently renders one sort pill. Change to render a single pill containing all sort levels joined by ` · `:

```
[ name ↑ · created_at ↓ ]
```

If the full string would overflow the available width, truncate with `…`:
```
[ name ↑ · created_at ↓ · … ]
```

No pill when `len(sort) == 0` (existing behaviour).

## Acceptance Criteria

Tests follow the `TestAC_<SECTION><NN>_<description>` pattern. The acceptance test file must open with a **coverage map** comment mapping every criterion to its test(s).

### CL — Core / SQL layer (tests in `dataset_test.go`)

- CL01: `QueryOptions{Sort: []dataset.Sort{{Column:"name", Desc:false}, {Column:"id", Desc:true}}}` produces `ORDER BY name ASC, id DESC` in the emitted SQL (both table and SQL-dataset variants).
- CL02: `QueryOptions{Sort: nil}` and `QueryOptions{Sort: []dataset.Sort{}}` both produce no `ORDER BY` clause.
- CL03: A `Sort` entry with an unknown column name is rejected with an error (same whitelist check as today).
- CL04: Single-element `[]Sort` produces the same SQL as the old single-sort path (regression guard).

### SM — Sort manager (view unit tests in `sortmanager_test.go`)

- SM01: `View()` of a freshly opened overlay with no active sorts contains `"Available"`, the column names, and `"Enter confirm"` / `"Esc cancel"`. The `"Active"` section header and divider are absent.
- SM02: `View()` with two active sorts shows `"Active"`, `"1."` and `"2."` with the correct column names and direction arrows (`↑`/`↓`), plus the `"Available"` section below the divider.
- SM03: `Space` on an available column adds it to the active section — the view now contains `"1. <colname>"` and the column name moves out of the `Available` section.
- SM04: `Space` on an active `↑` entry changes it to `↓` (and vice versa).
- SM05: `Del` on an active entry removes it — the view no longer contains that entry's number, and the remaining entries are renumbered.
- SM06: `J` on active entry 1 swaps it with entry 2 — the view shows them in reversed order.
- SM07: `K` on active entry 2 swaps it with entry 1 — same net result as SM06.
- SM08: `Enter` emits `SortConfirmedMsg` with the correct `[]dataset.Sort` slice in priority order.
- SM09: `Esc` does not emit `SortConfirmedMsg`; the model reports no changes applied.
- SM10: `s` key and `S` key are both present in `keys.Map`, wired in the row browser's `Update()`, and `S` appears in `helpoverlay.go`.
- SM11: `S` (uppercase) always opens the sort manager — even with no active sort, the overlay appears (showing only `Available`).

### HD — Column header display (view unit tests in `rowbrowser_test.go`)

- HD01: Row browser with `sort = [{name, ASC}, {id, DESC}]` — `View()` contains `↑¹` adjacent to `"name"` and `↓²` adjacent to `"id"`.
- HD02: Row browser with `sort = nil` — `View()` contains neither `↑¹` nor `↓¹` nor any superscript marker.
- HD03: Unsorted columns in a multi-sort result have no sort marker in the header.

### PL — Pill bar display (view unit tests in `rowbrowser_test.go`)

- PL01: `sort = [{name, ASC}, {created_at, DESC}]` — `View()` contains a pill with both `"name ↑"` and `"created_at ↓"` and a separator between them.
- PL02: `sort = nil` — `View()` does not contain a sort pill.
- PL03: Single-element `sort` — `View()` contains `"name ↑"` (or `↓`) without a separator.

### AC — App integration tests (in `app_test.go` or `sortmanager_acceptance_test.go`)

- AC01 (single sort — no overlay): Load a table with columns A, B, C. Cursor on column A. Press `s`. Assert: overlay does NOT appear; pill bar shows `"A ↑"`; column header shows `↑¹`.
- AC02 (cycle on same column — no overlay): With `A ↑` active, cursor on A, press `s`. Assert: overlay does NOT appear; pill shows `"A ↓"`. Press `s` again. Assert: pill is absent (sort cleared).
- AC03 (second column — overlay opens): With `A ↑` active, move cursor to column B, press `s`. Assert: overlay appears; overlay `View()` shows `"1. A ↑"` in Active and B already appears in Active as `"2. B ↑"` (pre-added), not in Available.
- AC04 (confirm multi-sort from AC03): In the overlay from AC03, press `Enter`. Assert: pill shows `"A ↑ · B ↑"`; column headers show `↑¹` on A and `↑²` on B.
- AC05 (S always opens overlay): With no sort active, press `S`. Assert: overlay appears showing only `Available` section with column names. Press `Esc`. Assert: no sort pill.
- AC06 (S opens overlay with existing sort): With `A ↑` active, press `S`. Assert: overlay appears with `"1. A ↑"` in Active; no re-fetch triggered yet.
- AC07 (toggle direction in overlay): Open overlay (via `S`), add column A, press `Space` on A in Active. Assert: entry shows `↓`. Confirm. Assert: pill shows `"A ↓"` and header shows `↓¹`.
- AC08 (reorder): Open overlay with `[A ASC, B ASC]` already active. Navigate to A (priority 1), press `J`. Assert: overlay shows B as `1.` and A as `2.`. Confirm. Assert: pill shows `"B ↑ · A ↑"`.
- AC09 (remove): Open overlay with `[A ASC, B ASC]`. Navigate to A, press `Del`. Assert: overlay shows only `"1. B"`. Confirm. Assert: pill shows only `"B ↑"`.
- AC10 (Esc reverts): Open overlay (via `S`), add a column, press `Esc`. Assert: pill bar is unchanged from before opening the overlay.

## What NOT to Change

- Filter logic — filters are validated against full schema, not the sort selection.
- Column projection (`QueryOptions.Columns`) — unchanged.
- `PageSizeRegistry`, `ColumnRegistry` — no changes.
- Export — already passes `QueryOptions` to the executor; multi-sort flows through automatically once the slice is set.
- The `COUNT(*)` subquery — unaffected.

## Definition of Done

See `tasks/definition-of-done.md`. Invoke `/done` after all acceptance criteria are met.
