# Datacow — Task Milestones

Each file is a self-contained task Claude can work on autonomously.
Read `CLAUDE.md` in the repo root before starting any task.

## Milestone Order

| Milestone | Task | Depends On | Status |
|---|---|---|---|
| M1 | [Project Scaffold](M1-project-scaffold.md) | — | done |
| M2 | [Database Core Layer](M2-db-core.md) | M1 | done |
| M3 | [Dataset Layer](M3-dataset-layer.md) | M2 | done |
| M4 | [TUI Shell](M4-tui-shell.md) | M1 | done |
| M5 | [TUI Table Browser](M5-tui-table-browser.md) ⭐ MVP | M2, M3, M4 | done |
| M6 | [Filter, Sort, Export](M6-filter-sort-export.md) | M5 | done |
| M7 | [FK Drill-Down](M7-fk-drilldown.md) | M5 | done |
| — | [TUI Query Log](tui-query-log.md) | M7 | done |
| M8 | [HTTP API Server](M8-http-api.md) | M3 | todo |
| M9 | [Web App](M9-web-app.md) | M8 | todo |
| M10 | [YAML Config + Custom Datasets](M10-yaml-config-datasets.md) | M3, M5 | todo |
| M11 | [Multi-Datasource TUI](M11-multi-datasource.md) | M10 | todo |

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
