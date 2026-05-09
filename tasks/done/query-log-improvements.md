# TUI: Query Log Improvements

Make the query log easier to read at a glance: show when each query ran, how long it took (already shown, but in a cleaner layout), whether it failed, and make the ordering unambiguous.

## Current problems

- **Ordering is ambiguous.** The full-screen log (`L`) shows newest at the top, but the compact SQL pane (bottom strip) shows newest at the bottom. Neither says so. Users can't tell which end is "latest."
- **No timestamp.** `QueryEntry.StartedAt` is stored but never rendered — you cannot tell when a query ran.
- **No error display.** `QueryEntry.Error` is stored but never rendered — a failed query looks identical to a successful one.
- **SQL pane and full-screen log are ordered opposite to each other** — one is newest-first, the other oldest-first — with no labels to explain the difference.

## What to build

### Full-screen query log (`internal/tui/views/querylog.go`)

**Column layout (history rows):**

```
History  (newest first)
  HH:MM:SS   42ms   orders data        1,234 rows   user
  HH:MM:SS    8ms   describe orders    12 rows       system
  HH:MM:SS   15ms   list tables        8 rows        system
  HH:MM:SS   ERR    user_sessions      login failed  user
```

Changes:
1. Change header from `"History"` → `"History  (newest first)"` — one line, no new section.
2. Add a `HH:MM:SS` timestamp column at the start of each history row, taken from `entry.StartedAt`. Use `entry.StartedAt.Format("15:04:05")`.
3. When `entry.Error != nil`: replace the row count field with an `ERR` badge (styled like `style.StatusKey` but distinct — use `lipgloss.NewStyle().Foreground(lipgloss.Color("9"))` for red if no existing error style exists, otherwise add one to `internal/tui/style/`). Show a short truncation of `entry.Error.Error()` in place of the label or append it in the SQL preview section.
4. SQL preview at the bottom: also show `"At: HH:MM:SS  Duration: 42ms"` on the line before the SQL. If the entry has an error, show `"Error: <message>"` instead of (or after) SQL.

Running rows already show elapsed time via the spinner — no change needed there.

### Compact SQL pane (`internal/tui/views/sqlpane.go`)

The SQL pane currently shows oldest-at-top / newest-at-bottom (streaming style). Keep that order — it makes sense for a live feed. But:

1. Change the iteration so it's explicit and documented in a comment.
2. Add `HH:MM:SS` prefix to each history row (same `entry.StartedAt.Format("15:04:05")`). Adjust `sqlW` calculation to account for the 9-char timestamp prefix.
3. When `entry.Error != nil`: render the row with a dim red style instead of `style.StatusDesc`.

### Style additions (`internal/tui/style/style.go` or wherever styles live)

Add `QueryError lipgloss.Style` if a suitable red/error style does not already exist. Check first — don't add if there is already one.

## Files to modify

| File | Change |
|---|---|
| `internal/tui/views/querylog.go` | Add timestamp column, error display, ordering label, SQL preview enhancement |
| `internal/tui/views/sqlpane.go` | Add timestamp prefix, error row styling, column width adjustment |
| `internal/tui/style/` | Add error style if missing |

No changes to core, app.go, or keys — all data is already in `QueryEntry`.

## Implementation notes

- `entry.StartedAt` is a `time.Time` — format with `.Format("15:04:05")` for a 8-char fixed-width column.
- History is `[]QueryEntry` newest-first (index 0 = newest). The full-screen view iterates forward (newest first). Document this.
- SQL pane iterates `for i := len(history)-1; i >= 0; i--` which appends oldest first → newest at bottom. Keep this but add a comment.
- The `formatDuration` helper already exists in `querylog.go` — call it from `sqlpane.go` (they're in the same package).

## Tests

No new unit tests needed — this is pure rendering logic with no branching state. Manual verification is sufficient.

## Definition of Done

```bash
make preflight
go build ./...
gotestsum --format testdox ./...
staticcheck ./...
make lint
```

Manual:
1. Open any table → press `L` → History header reads `"History  (newest first)"`.
2. Each history row shows a `HH:MM:SS` timestamp matching when the query was issued.
3. Force a query error (e.g., break a dataset SQL) → the failing entry shows `ERR` badge and the error message in the SQL preview.
4. Compact SQL pane (bottom strip) shows timestamps on each row.
5. Failed query rows in the SQL pane render with a red/dim style distinguishable from normal rows.
