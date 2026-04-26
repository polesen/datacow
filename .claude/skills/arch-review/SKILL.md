---
name: arch-review
description: Scan changed Go files for layer violations — business logic in TUI/API, DB access outside core, missing interfaces, global state
---

Review all Go files changed in this branch (`git diff --name-only main -- '*.go'`) for the violations below. Flag each one with file:line, the violation type, and a one-line fix suggestion. If nothing is found, say so.

## Violations to flag

**1. DB access outside `internal/core/`**
Any import or use of `database/sql`, `pgx`, `github.com/jackc/pgx`, or raw SQL strings (`SELECT`, `INSERT`, `UPDATE`, `DELETE`, `CREATE`, `DROP`) in files under `internal/tui/` or `internal/api/`. All database access must go through `core/db.Client`.

**2. Business logic in TUI or API layers**
Non-trivial logic (data transformation, filtering, sorting, pagination calculations, query construction) in files under `internal/tui/` or `internal/api/`. Presentation concerns (rendering, key handling, HTTP routing) are fine; anything that computes or decides belongs in `core/`.

**3. Concrete type used where an interface should exist**
A new struct that is constructed and passed across package boundaries without a corresponding interface. The pattern is: if callers import the concrete type's package just to instantiate it, an interface is missing.

**4. Package-level mutable state**
`var` declarations at package scope that are not constants (`const`) or read-only maps/slices initialised once. Pass state explicitly via struct fields or function parameters.

## What NOT to flag

- Test files (`_test.go`) — they are allowed to import concrete types directly
- `internal/core/db/` files — DB access is expected there
- `cmd/` package — it is the wiring layer and may construct concrete types
- Unexported package-level vars that are clearly initialised once and never mutated (e.g. `var defaultStyles = lipgloss.NewStyle()`)

## Output format

For each finding:

```
[TYPE] internal/tui/context/table.go:42
  Found: sql.Query() called directly in TUI layer
  Fix: move query to a core function and call it from here
```

End with a summary line: `N violation(s) found` or `No violations found`.
