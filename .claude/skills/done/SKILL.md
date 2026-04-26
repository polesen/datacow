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
6. Architecture — run the `/arch-review` skill. Resolve any layer violations before continuing.
7. SQL injection — spawn the `sql-security-reviewer` subagent (Agent tool, subagent_type: `sql-security-reviewer`). Pass it the list of changed Go files from `git diff --name-only HEAD` filtered to `*.go`. Review its findings and fix any injection vector before continuing.

Only report the task as done when all seven steps pass with zero findings.
