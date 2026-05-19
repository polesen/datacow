# Task Context — How Tasks Work

## Lifecycle

```
drafts/   →   ready/   →   done/
```

- **`drafts/`** — ideas under consideration; not yet scoped or approved for implementation
- **`ready/`** — fully scoped, ready to hand to Claude; `run-task.sh` picks tasks from here
- **`done/`** — merged; file moved here after PR is merged, renamed to `NNNNNN-<original-name>.md` where `NNNNNN` is the next zero-padded 6-digit sequence number (e.g. `000029-my-feature.md`)

## Writing a Task

Tasks are markdown files. Use this template:

```markdown
# <Short title>

One-paragraph description of the goal. What problem does this solve, and what does the result look like?

## Background (optional)

Context that helps Claude understand why, not just what. Omit if the goal is self-evident.

## [Behaviour / UX / Architecture sections]

Whatever sections are needed to specify the work precisely. For TUI tasks, include ASCII diagrams.
For core/API tasks, describe interfaces and contracts.

## Acceptance Criteria

Bullet list of observable outcomes that must be true when the task is complete.
Be specific — "the `z` key zooms the focused pane" not "zoom works".

## What NOT to Change

Explicit list of things Claude must leave alone. Prevents scope creep.
Example: "Do not touch the schema cache — that is a separate task."

## Definition of Done

See [definition-of-done.md](definition-of-done.md). All gates must pass.
```

## Scoping Rules

- One task = one feature branch = one PR
- A task should be completable in a single autonomous Claude session (a few hours of work)
- If a task requires understanding another task's output, it goes into `drafts/` until that dependency is merged
- "What NOT to Change" is mandatory — it forces explicit scope decisions before Claude starts

## Running a Task

```bash
./run-task.sh ready/my-task.md
```

This creates the feature branch, starts the devcontainer, runs preflight, runs Claude, then pushes and opens a PR.

See `README.md` for the full workflow.
