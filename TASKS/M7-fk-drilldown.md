# M7 — Foreign Key Drill-Down Navigation

## Goal
The key differentiator: clicking a foreign key cell navigates to the related records, appending them below the current view. The parent stays visible. Multiple levels deep.

## Depends On
M5 (TUI table browser), M2 (FK detection in schema layer)

## Acceptance Criteria
- [ ] FK columns visually distinguished (e.g., underlined or different colour)
- [ ] Enter on a FK cell fetches related rows from the referenced table, filtered by the FK value
- [ ] Child dataset **appends below** the current view (does not replace it)
- [ ] A visual separator between parent and child shows the relationship: `orders → customer_id → customers`
- [ ] Scroll up reveals the parent
- [ ] `esc` collapses the most recent child level
- [ ] Multiple levels supported (navigate from orders → customers → addresses)
- [ ] Works with composite FKs (less common, but don't crash)
- [ ] FK detection uses schema introspection from M2 (auto-detected, no config)

## UX Sketch
```
┌─ orders (3 of 1,193 rows) ─────────────────┐
│ id   customer_id   amount   status          │
│ 42   [1001]        $50.00   paid            │  ← FK cell selected
│ 43   1002          $30.00   pending         │
├─ → customer_id = 1001 → customers ─────────┤
│ id     name          email                  │
│ 1001   Jane Smith    jane@example.com       │
└─────────────────────────────────────────────┘
```

## Notes
- The "append below" model is the core UX innovation — implement it carefully
- Each level in the drill-down stack is a `DatasetView` with its own filters, sort, pagination
- Consider a `DrillStack []DatasetView` in app state

## Verify
```bash
# Open a table with FK columns → navigate to FK cell → press Enter → see child rows appended
# Press Enter on another FK in child → three levels visible → esc collapses one level at a time
```
