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

Once all quality gates pass, close out the task:

8. Move task file — `git mv tasks/ready/<task>.md tasks/done/<task>.md` (or from `drafts/`)
9. Commit — stage all changes including the task file move, then commit with a message explaining why the change was made
10. Push — `git push -u origin <branch>`
11. Open PR — `gh pr create` with a short descriptive title (not the task filename) and a body describing what was built and why, followed by:
    ```
    ## Review checklist
    - [ ] Code does what the task describes
    - [ ] Run `/simplify` if reviewer flags unnecessary complexity
    ```

Only report the task as done when all eleven steps are complete.
