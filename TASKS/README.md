# Datacow — Task Milestones

Each file is a self-contained task Claude can work on autonomously.
Read `CLAUDE.md` in the repo root before starting any task.

## Milestone Order

| Milestone | Task | Status |
|---|---|---|
| M1 | [Project Scaffold](M1-project-scaffold.md) | done |
| M2 | [Database Core Layer](M2-db-core.md) | done |
| M3 | [Dataset Layer](M3-dataset-layer.md) | done |
| M4 | [TUI Shell](M4-tui-shell.md) | done |
| M5 | [TUI Table Browser](M5-tui-table-browser.md) ⭐ MVP | done |
| M6 | [Filter, Sort, Export](M6-filter-sort-export.md) | done |
| M7 | [FK Drill-Down](M7-fk-drilldown.md) | done |
| — | [TUI Query Log](tui-query-log.md) | done |
| M8 | [YAML Config + Custom Datasets](M8-yaml-config-datasets.md) | todo |
| M9 | [Multi-Datasource TUI](M9-multi-datasource.md) | todo |

## Future Work

Not planned for the current version. Kept in `future/` for reference.

| Task | Notes |
|---|---|
| [HTTP API Server](future/M8-http-api.md) | Enables the web app; defer until web UI is prioritised |
| [Web App](future/M9-web-app.md) | Depends on HTTP API; full web UI for non-terminal users |

## Running Autonomously

To run Claude on a task with full permissions inside the dev container:

```bash
# 1. Start the dev container (from repo root)
npx @devcontainers/cli up --workspace-folder .

# 2. Exec Claude inside it
npx @devcontainers/cli exec --workspace-folder . \
  claude --dangerously-skip-permissions \
  "Read CLAUDE.md, then complete the task described in TASKS/M2-db-core.md. Verify all acceptance criteria are met before finishing."
```

Replace `TASKS/M2-db-core.md` with whichever milestone you want to run.

## Updating Task Status

When a milestone is complete, update the Status column in this table to `done`.
