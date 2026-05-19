# Reverse FK Drill-Down

Today the row browser drills *forward*: pressing `Enter` on a foreign-key cell
opens the referenced parent row. This task adds the *reverse* direction —
"who points at this row?" — from the same cell, plus a static "Referenced
By" view in the table list. Same mental model, same drill stack, opposite
arrow.

## Background

The forward path was shipped by [M7](../done/fk-drilldown.md):
`internal/tui/views/rowbrowser.go:461 handleDrillDown` pushes a level onto
`m.drillStack` and re-queries the referenced table filtered by the FK
column. Inbound FKs are not surfaced anywhere today, even though the
schema layer already loads outgoing FKs for every table — the inverse is
a derived view of the same data.

Two complementary entry points, both worth having:

1. **From a cell** — when standing on a row, "show me everything that
   references this row." The natural symmetric pair to forward drill.
2. **From the table list** — when browsing tables, "what tables point at
   this one?" — a piece of schema literacy currently hidden.

## Keys

`Enter` is preserved as forward drill. Add two new bindings:

| Key | Where | Action |
|---|---|---|
| `>` | Row browser, normal mode | Forward drill (alias of `Enter`) |
| `<` | Row browser, normal mode | Reverse drill — open the "referenced by" picker for the cell's column |
| `Esc` | Row browser | Pop drill stack (unchanged — works for both forward and reverse levels) |

Rationale: `>` and `<` are visually symmetric and read naturally as FK
direction. `Enter` keeps working so muscle memory is intact. `R` was
considered but `<` / `>` carry the direction in the glyph itself.

Add to `keys.Map`:

```go
DrillFwd     key.Binding   // ">"   help: "> drill forward"
DrillReverse key.Binding   // "<"   help: "< referenced by"
```

`Enter` continues to invoke `handleDrillDown` (forward). `>` is an alias —
routed to the same handler. `<` invokes a new `handleReverseDrillDown`.

Update `keys.ShortHelp()` and `keys.FullHelp()` so the row-browser group
lists both `>` and `<`. Status bar must show `<` in row-browser context.

## Cell eligibility

A cell is eligible for reverse drill **only if its column is referenced
by at least one other table's FK** (i.e. appears as `ReferencedColumn` in
some inbound FK). For a typical schema this means PK columns and
occasionally unique columns; for tables that aren't pointed at by
anything, no cell is eligible.

When the user presses `<` on an ineligible cell, show a transient status
message: `no tables reference this column`. Do not open the picker.

When pressed on an eligible cell whose value is `NULL`, treat it like the
forward path treats NULL FK cells today (no-op, no message).

### Visual marker on the header

Column headers whose column is referenced by an inbound FK get a subtle
marker so the user can see at a glance which cells will respond to `<`.
Mirror the existing forward treatment in `internal/tui/style/style.go`
(`FKColHeader` / `FKColHeaderActive`) — add `RefByColHeader` /
`RefByColHeaderActive` with a different decoration (a trailing `↩` glyph
in the header text is the simplest; a dotted underline is fine too). A
column that is *both* an outgoing FK and a referenced column (rare —
e.g. a PK that is also a child-table FK) shows both markers; do not
collapse them.

The cell body style does not change — only the header.

## Picker

When `<` is pressed on an eligible cell:

- **0 referencing tables** — shouldn't reach here (header marker prevents
  it), but defensively show the same status message.
- **1 referencing table** — skip the picker and drill immediately. Single
  unambiguous result; an overlay would be friction.
- **2+ referencing tables** — open a floating overlay picker, centered,
  reusing the Goto overlay pattern in `internal/tui/views/goto.go`.

Picker items render as `referencing_table.column` (e.g. `orders.customer_id`),
one per line, in alphabetical order by `referencing_table`, secondary by
`column`. The header reads:
`Referenced by — <table>.<column> = <value>`.

Keys inside the picker:

| Key | Action |
|---|---|
| `↑/↓` / `k/j` | Move cursor |
| `Enter` | Select — drill into the highlighted referencing table |
| `Esc` | Close picker, no navigation |

Typing while the picker is open filters the list (case-insensitive
substring on the rendered `table.column` string), matching the Goto UX.
If the filtered list is empty, render a single `No matches` line.

## Drill behavior

On selection — whether after the picker or via single-match shortcut —
push a level onto `m.drillStack` exactly as the forward path does, but
with the reverse semantics:

- `ds` becomes the **referencing** table (the one whose FK points back
  at us).
- The filter is `referencing_column = <cell value>`.
- The breadcrumb reads `← <referencing_table>.<referencing_column> = <value> ← <current_table>`
  (left arrow to match `<` and distinguish from forward levels which
  use `→`).
- Sort, page, column cursor reset to defaults — same as forward.

`popDrillStack` already handles arbitrary levels and does not need
changes. Pressing `Esc` on a reverse level pops back to the parent the
same way it does for forward levels. Mixed stacks (forward, reverse,
forward, …) must work.

## Table list — "Referenced By" sub-tree

In the tables pane, when a dataset row is expanded, the tree today shows
`Columns`, `Indexes`, `Foreign Keys` (`internal/tui/views/tablelist.go:342 subLines`).
Add a fourth section:

```
└─ Referenced By
   ← orders.customer_id
   ← invoices.customer_id
   ← subscriptions.customer_id
```

Format each row as `← <from_table>.<from_column>` to mirror the existing
`formatFK` style (`→ column → ref_table.ref_column`) and to use the same
arrow convention as the reverse drill breadcrumb.

Section is always present (consistent with `Foreign Keys`). When the
referencing list is empty, render `(none)`. When the schema cache is not
yet ready, render `loading…` with the spinner — same affordance as the
existing async sections.

Change the last-section glyph: today `Foreign Keys` is `└─`; with this
new section, `Foreign Keys` becomes `├─` and `Referenced By` becomes
`└─`. Update the box-drawing prefixes for sub-rows accordingly
(`│   ` for non-last, `    ` for last).

This section is YAML-dataset-aware: SQL-defined datasets (`KindDataset`)
have no underlying table, so they have no inbound FKs — render the
section absent (not "(none)") for those rows. They already skip column
introspection; follow that pattern.

## Where the inbound FK data lives

Inbound FKs are a global derived view: to answer "what points at table
X?" you must read *every* table's outgoing FKs. This means it cannot
live on a single `schema.Table` populated by a per-table client call.
Two places to plug it in:

1. **`schema.Table.ReferencedBy []InboundFK`** — populated by
   `schema.Load` in a second pass after all outgoing FKs are collected.
   Mirrors the existing `ForeignKeys` field; callers read it the same
   way.
2. **Cache lookup** — a derived map computed once when the cache
   becomes ready: `cache.ReferencedBy(table string) []InboundFK`.

Both are reasonable. Prefer **(1)** — it's symmetric with the existing
field, no new method, and the schema package is already the home for
this kind of derived structural data. Define:

```go
// InboundFK describes an FK that points at this table from elsewhere.
type InboundFK struct {
    FromTable  string // referencing table
    FromColumn string // column in the referencing table that holds the FK
    ToColumn   string // the column in *this* table that is being referenced
}
```

Populate `Table.ReferencedBy` in `schema.Load` after the outgoing-FK
loop. The cache exposes it transparently because it stores `[]Table`.

The row browser needs access to inbound FKs *for the active table* when
the user presses `<`. Plumb the schema cache into `RowBrowserModel` (it
already receives an executor; add `*schema.Cache` alongside), and look
up `cache.Tables()` filtered to the active table name. Falling back
behavior: if the cache is not ready, show `schema loading — try again`
as the status message and do nothing. Do not block the keypress.

## Files

| File | Change |
|---|---|
| `internal/core/db/client.go` | No change — `ForeignKey` is already sufficient as outgoing data. |
| `internal/core/schema/schema.go` | Add `InboundFK` type; add `ReferencedBy []InboundFK` to `Table`; populate in `Load` via a second pass over all collected outgoing FKs. |
| `internal/core/schema/schema_test.go` | Extend `Load` test: with `sc_items` referencing `sc_orders`, assert `sc_orders.ReferencedBy` contains the inbound FK from `sc_items`. |
| `internal/core/schema/cache_test.go` | If any test asserts the shape of `Table`, adjust. |
| `internal/tui/keys/keys.go` | Add `DrillFwd` (`>`) and `DrillReverse` (`<`) bindings. Wire `>` as alias of `Enter` for forward drill. Add to `ShortHelp`, `FullHelp`, and the row-browser group. |
| `internal/tui/views/rowbrowser.go` | Plumb `*schema.Cache` into the model. Add `handleReverseDrillDown` that: validates eligibility via the cache, opens the picker (or skips to single match), pushes a reverse level onto `m.drillStack`. Route `>` to `handleDrillDown`; route `<` to the new handler. Compute "referenced column" set for header marker styling (analogous to `fkColSet`). |
| `internal/tui/views/rowbrowser_test.go` | Direct `View()` and unit tests: `<` on ineligible cell yields status message and no nav; `<` with one referencing table drills directly; `<` with multiple opens picker; reverse drill pushes a level with `← from.col = val ← to` breadcrumb; Esc pops cleanly across mixed forward/reverse stacks; header marker rendered on referenced columns; marker active style on selected column. |
| `internal/tui/views/refbypicker.go` (new) | Floating overlay listing `referencing_table.column` entries with substring filter — model after `views/goto.go`. Emits a selection message that the row browser turns into a drill. |
| `internal/tui/views/refbypicker_test.go` (new) | Render-state tests: header reflects source column and value; list shows alpha-sorted entries; typing filters live; `Enter` emits the selection message; `Esc` emits cancel; empty filter result shows `No matches`. |
| `internal/tui/style/style.go` | Add `RefByColHeader` and `RefByColHeaderActive` mirroring `FKColHeader` / `FKColHeaderActive` (different decoration, e.g. trailing `↩` glyph). |
| `internal/tui/views/tablelist.go` | Read inbound FKs from the schema cache (`cache.Tables()` lookup by name). Render new `└─ Referenced By` sub-section; demote `Foreign Keys` to `├─`. Skip section entirely for `KindDataset` rows. Show `loading…` when cache not yet ready. |
| `internal/tui/views/tablelist_test.go` | Add tests: section rendered for a table with inbound FKs; `(none)` when none; absent for `KindDataset` rows; box-drawing prefixes correct after promotion of `Foreign Keys` to non-last. |
| `internal/tui/app.go` | Pass `*schema.Cache` into `RowBrowserModel` constructor (it already exists for the table list per the [filter-search-redesign](filter-search-redesign.md) family of tasks; if not yet plumbed in this code path, plumb it). |
| `internal/tui/views/helpoverlay.go` | Add help entries for `>` (drill forward) and `<` (referenced by). |
| `internal/tui/views/helpoverlay_test.go` | Assert the new entries render. |

Constructor signatures change for `RowBrowserModel`; update every call
site (`app.go`, tests, teatest setup).

## Acceptance Criteria

### Schema layer

- [ ] `schema.InboundFK` exists with `FromTable`, `FromColumn`, `ToColumn`.
- [ ] `schema.Table.ReferencedBy []InboundFK` is populated by `schema.Load` for every table that is referenced by at least one outgoing FK in the loaded set.
- [ ] Tables not referenced anywhere have an empty (or nil) `ReferencedBy`.
- [ ] `schema_test.go` covers both presence and absence.

### Row browser — reverse drill from a cell

- [ ] `>` is bound and behaves identically to `Enter` (forward drill).
- [ ] `<` on a cell whose column is not referenced by any inbound FK shows transient status `no tables reference this column` and does not navigate.
- [ ] `<` on an eligible cell with `NULL` value is a no-op (parity with forward drill on NULL FK).
- [ ] `<` on an eligible cell with exactly one referencing table drills immediately, pushing a reverse level onto the drill stack. The new view is filtered `referencing_column = cell_value`.
- [ ] `<` on an eligible cell with 2+ referencing tables opens the picker overlay.
- [ ] The picker lists entries in alphabetical order by `referencing_table`, then by `column`.
- [ ] Picker header reads `Referenced by — <table>.<column> = <value>`.
- [ ] Picker accepts substring filter input; `No matches` line when filter yields nothing.
- [ ] Picker `Enter` selects and drills; `Esc` closes without nav.
- [ ] Reverse drill breadcrumb reads `← <referencing_table>.<referencing_column> = <value> ← <current_table>`.
- [ ] `Esc` pops a reverse level the same way it pops a forward level.
- [ ] Mixed stacks (forward → reverse → forward) work; `Esc` unwinds in LIFO order.

### Visual marker

- [ ] Column headers for columns referenced by at least one inbound FK render with the reverse-FK marker style.
- [ ] When the cursor is on such a column, the active variant is used.
- [ ] A column that is *both* an outgoing FK and a referenced column shows both decorations; neither is suppressed.

### Table list — Referenced By section

- [ ] Expanded table rows show four sections in order: `Columns`, `Indexes`, `Foreign Keys`, `Referenced By` — with `Referenced By` as the last (`└─`) section.
- [ ] Each entry is `← <from_table>.<from_column>`.
- [ ] Empty case renders `(none)`.
- [ ] The section is absent for `KindDataset` rows (SQL-defined datasets).
- [ ] When the schema cache is not ready yet, the section renders `loading…` with the spinner; once ready, the row re-renders without further user action.

### Keys & help

- [ ] `keys.Map.DrillFwd` is `>`, `keys.Map.DrillReverse` is `<`.
- [ ] Full help overlay lists both bindings under the row-browser group.
- [ ] Status bar shows `<` in the row-browser context.
- [ ] No other key behaviour changes.

### Tests

- [ ] `rowbrowser_test.go` covers the eligibility, single-match shortcut, picker open, breadcrumb, and stack-pop behaviour above via direct `View()` and `Update` tests with a `stubClient` schema.
- [ ] `refbypicker_test.go` covers picker render, filter, select, cancel, and empty-state.
- [ ] `tablelist_test.go` covers the new sub-section in all three states (populated, empty, loading) and the absent case for `KindDataset`.
- [ ] At least one `teatest` smoke test exercises end-to-end: open a table → place cursor on a referenced column → press `<` → assert picker overlay rendered (when applicable) → select → assert the drill stack contains a reverse level with the expected breadcrumb.

### Definition of done

- [ ] `make preflight` passes.
- [ ] `/done` checks all pass.

## What NOT to Change

- **Forward FK drill-down** — `handleDrillDown` stays exactly as it is. `Enter` continues to call it; `>` is wired as a key alias, not as a re-implementation.
- **`db.Client` interface** — `ForeignKey` is sufficient as raw data; the reverse view is derived in `schema.Load`. No new driver methods.
- **`db.ForeignKey` struct** — do not rename or restructure. The new `InboundFK` is a separate type with the opposite directionality.
- **Drill stack mechanism** — `m.drillStack`, `popDrillStack`, and `savedLevel` are unchanged. A reverse level is just another `savedLevel` with a different breadcrumb prefix.
- **Goto overlay (`Ctrl+P`)** — independent. Do not merge with the picker; the picker is scoped to one source cell and uses substring match.
- **Table list filter (`/`)** — being added by [tablelist-filter-search](tablelist-filter-search.md). Do not touch it here. The Referenced By section content is *not* in the filter match set in this task — leave that as a future extension.
- **Row browser `/` filter** — being reshaped by [filter-search-redesign](filter-search-redesign.md). Do not touch.
- **Schema cache loading model** — same lazy-background load. Only its shape changes (new `ReferencedBy` field flows through naturally).
- **YAML SQL datasets (`KindDataset`)** — they have no underlying table; never appear as `referencing` or `referenced` in the new flows. Both row-browser reverse drill and the table-list section ignore them.
- **API / web layer** — TUI-only task.

## Definition of Done

See [definition-of-done.md](../definition-of-done.md). All gates must pass.
