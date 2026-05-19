---
name: done
description: Run the Datacow Definition of Done checklist — build, tests, race detector, lint, staticcheck, acceptance tests, SQL injection scan
---

Run each step below in order. Stop and report clearly if any step fails — do not continue past a failure.

1. `go build ./...` — must compile with zero errors
2. `make wait-for-db` — ensure Postgres and MySQL are ready
3. `gotestsum --format testdox ./...` — all tests must pass
4. `gotestsum --format testdox -- -race ./...` — no data races and no timing-sensitive failures; the race detector slows goroutine scheduling and catches conditions that only surface on slower hardware
5. `make lint` — zero lint warnings
6. `staticcheck ./...` — zero static analysis findings
7. Architecture — run the `/arch-review` skill. Resolve any layer violations before continuing.
8. SQL injection — spawn the `sql-security-reviewer` subagent (Agent tool, subagent_type: `sql-security-reviewer`). Pass it the list of changed Go files from `git diff --name-only HEAD` filtered to `*.go`. Review its findings and fix any injection vector before continuing.
9. Acceptance tests — `gotestsum --format testdox -- -run TestAC ./...` must pass. If any `TestAC_*` test fails, fix the implementation. If the spec deliberately changed (as a considered decision, not a workaround), update both the spec AND the test together — never delete a failing acceptance test without a matching spec update and explanation in the task file's Implementation Notes section.
10. Acceptance coverage — verify there is at least one `TestAC_*` test per acceptance criterion listed in the task file's `## Acceptance Criteria` section. Count the bulleted criteria and the `TestAC_*` functions. They must match, or each uncovered criterion must have a comment in the acceptance test file (or in app_test.go) explaining which existing named test covers it and why a duplicate is unnecessary.

Once all quality gates pass, close out the task:

11. Task file accuracy — compare the task file (behaviour, files table, acceptance criteria, implementation notes) against what was actually built. If anything diverges — added behaviour, removed features, changed UX details, extra files touched — draft the proposed updates and **present them to the user for approval before making any edits**. Only proceed after explicit confirmation. If nothing diverged, state that clearly and continue.
12. Move task file — `git mv tasks/ready/<task>.md tasks/done/<task>.md` (or from `drafts/`)
13. Commit — stage all changes including the task file move, then commit with a message explaining why the change was made
14. Push — `git push -u origin <branch>`
15. Open PR — `gh pr create` with a short descriptive title (not the task filename) and a body describing what was built and why, followed by:
    ```
    ## Review checklist
    - [ ] Code does what the task describes
    - [ ] Run `/simplify` if reviewer flags unnecessary complexity
    ```

Only report the task as done when all fifteen steps are complete.
