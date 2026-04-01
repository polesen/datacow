# M5 — TUI Table Browser (MVP)

## Goal
This is the MVP milestone. Connect to Postgres or MySQL, list all tables, navigate into one, and browse its rows with pagination. This is what "this is Datacow" looks like.

## Depends On
M2 (DB core), M3 (dataset layer), M4 (TUI shell)

## Acceptance Criteria
- [ ] On launch, TUI connects to the DB and shows a list of all tables
- [ ] Arrow keys navigate the list, Enter opens a table
- [ ] Row browser: paged table view, 50 rows per page, columns sized to content
- [ ] Page navigation: `]` / `[` or PgDn/PgUp for next/previous page
- [ ] Current page and total rows shown in status bar
- [ ] `esc` or `backspace` goes back to the table list
- [ ] Long column values truncated with ellipsis
- [ ] NULL values shown distinctly (e.g., greyed out `null`)
- [ ] Connection errors shown as a readable message, not a panic
- [ ] Looks good with tables that have 2 columns and tables that have 30+ columns

## UX Details
- Table list: show table name + row count (lazy-loaded, shown as `...` until fetched)
- Row browser: fixed header row with column names, scrollable body
- Wide tables: horizontal scroll with `←` / `→`
- Status bar shows: `table_name  page 1/24  1,193 rows  q quit  esc back`

## Notes
- Wire up `core.Client` → `DatasetResolver` → `DatasetExecutor` from M2/M3
- Spinner while loading (use `charmbracelet/bubbles/spinner`)
- This milestone = the MVP. When this works, Datacow exists.

## Verify
```bash
go run ./cmd --connection-string="postgres://datacow:datacow@localhost:5432/datacow_test?sslmode=disable"
# See table list → navigate into a table → see rows → paginate → go back
```
