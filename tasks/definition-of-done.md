# Definition of Done

Before marking any task complete, invoke the `/done` skill. It runs all checks in sequence and stops on the first failure.

## Quality Gates

1. **Build** — `go build ./...` must compile with zero errors
2. **Databases ready** — `make wait-for-db` — confirm Postgres and MySQL are reachable
3. **Tests pass** — `gotestsum --format testdox ./...` — all tests green, zero failures
4. **Lint** — `make lint` — zero golangci-lint warnings
5. **Static analysis** — `staticcheck ./...` — zero findings
6. **Architecture** — run `/arch-review`; resolve any layer violations before continuing
7. **SQL injection** — run the `sql-security-reviewer` subagent against all changed Go files; fix any findings before proceeding

## Task Closure

When all quality gates pass:

- Move the task file: `git mv tasks/ready/<task>.md tasks/done/<task>.md`
- Commit all changes including the task file move
- Push: `git push -u origin <branch>`
- Open a PR with `gh pr create`:
  - **Title**: short descriptive feature name (not the task filename)
  - **Body**: what was built and why, followed by:
    ```
    ## Review checklist
    - [ ] Code does what the task describes
    - [ ] Run `/simplify` if reviewer flags unnecessary complexity
    ```

## Notes

- A task is not done until all seven quality gates pass with zero findings
- Do not skip the SQL injection scan — it catches patterns that `staticcheck` and lint miss
- If `make wait-for-db` fails, stop immediately — the devcontainer needs attention, not the code
