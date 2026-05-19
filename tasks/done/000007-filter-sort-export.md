# M6 — Filter, Sort, Export

## Goal
Add filtering, sorting, and CSV/Excel export to the row browser.

## Depends On
M5 (TUI table browser)

## Acceptance Criteria

### Filtering
- [ ] `/` opens a filter input bar at the bottom of the row browser
- [ ] Typing `column_name=value` adds a filter, re-fetches rows
- [ ] Multiple filters supported (shown as pills/tags above the table)
- [ ] `x` on a filter tag removes it
- [ ] Supports operators: `=`, `like`, `>`, `<`
- [ ] Filter state shown in status bar

### Sorting
- [ ] Press `s` on a column header to sort by that column ASC
- [ ] Press `s` again to sort DESC, again to remove sort
- [ ] Sort indicator shown in column header (↑ / ↓)
- [ ] Only one sort column at a time (v1)

### Export
- [ ] `e` opens export menu: CSV or Excel
- [ ] Export fetches ALL rows (not just current page), streams to file
- [ ] File saved as `<table_name>_<timestamp>.csv` / `.xlsx` in current directory
- [ ] Progress shown during export (row count)
- [ ] Success message with file path shown in status bar

## Notes
- Filtering and sorting go through `QueryOptions` from M3 — no new DB logic needed
- Excel export: use `github.com/xuri/excelize/v2`
- Export should respect active filters and sort

## Verify
```bash
# Filter: open table, press /, type "status=active", see filtered rows
# Sort: press s on a column, see sort indicator, rows reorder
# Export: press e, select CSV, file appears in current directory
```
