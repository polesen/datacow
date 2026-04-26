# Definition of Done

Before marking any task complete, invoke the `/done` skill. It runs all checks in sequence and stops on the first failure.

## Quality Gates

1. **Build** — `go build ./...` must compile with zero errors
2. **Databases ready** — `make wait-for-db` — confirm Postgres and MySQL are reachable
3. **Tests pass** — `gotestsum --format testdox ./...` — all tests green, zero failures
4. **Lint** — `make lint` — zero golangci-lint warnings
5. **Static analysis** — `staticcheck ./...` — zero findings
6. **SQL injection** — run the `sql-security-reviewer` subagent against all changed Go files; fix any findings before proceeding

## Task Closure

When all quality gates pass:

- Move the task file from `ready/` (or `drafts/`) to `done/` via `git mv`
- Commit: include the task closure in the same commit as the final code changes, or as a follow-up commit on the same branch

## Notes

- A task is not done until all six quality gates pass with zero findings
- Do not skip the SQL injection scan — it catches patterns that `staticcheck` and lint miss
- If `make wait-for-db` fails, stop immediately — the devcontainer needs attention, not the code
