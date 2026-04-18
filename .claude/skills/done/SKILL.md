---
name: done
description: Run the Datacow Definition of Done checklist — build, tests, lint, staticcheck, SQL injection scan
---

Run each step below in order. Stop and report clearly if any step fails — do not continue past a failure.

1. `go build ./...` — must compile with zero errors
2. `make wait-for-db` — ensure Postgres and MySQL are ready
3. `gotestsum --format testdox ./...` — all tests must pass
4. `make lint` — zero lint warnings
5. `staticcheck ./...` — zero static analysis findings
6. SQL injection scan — run: `grep -rn "fmt\.Sprintf\|+.*SELECT\|+.*WHERE\|+.*FROM\|+.*INSERT\|+.*UPDATE\|+.*DELETE" --include="*.go" .`
   - Review every match: confirm it does NOT interpolate user input, column names, or table names
   - If a match is a safe string literal with no external input, note it as safe
   - If any match is unsafe, fix it before continuing

Only report the task as done when all six steps pass and no injection vector is found.
