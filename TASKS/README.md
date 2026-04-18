# Datacow — Task Milestones

Each file is a self-contained task Claude can work on autonomously.
Read `CLAUDE.md` in the repo root before starting any task.

## Milestones

| Task | Status |
|---|---|
| [Project Scaffold](project-scaffold.md) | done |
| [Database Core Layer](db-core.md) | done |
| [Dataset Layer](dataset-layer.md) | done |
| [TUI Shell](tui-shell.md) | done |
| [TUI Table Browser](tui-table-browser.md) ⭐ MVP | done |
| [Filter, Sort, Export](filter-sort-export.md) | done |
| [FK Drill-Down](fk-drilldown.md) | done |
| [TUI Query Log](tui-query-log.md) | done |
| [YAML Config + Custom Datasets](yaml-config-datasets.md) | done |
| [Multi-Datasource TUI](multi-datasource.md) | done |
| [Cell Viewer + Save to File](cell-viewer.md) | todo |

## Future Work

Not planned for the current version. Kept in `future/` for reference.

| Task | Notes |
|---|---|
| [HTTP API Server](future/http-api.md) | Enables the web app; defer until web UI is prioritised |
| [Web App](future/web-app.md) | Depends on HTTP API; full web UI for non-terminal users |

## Running Autonomously

To run Claude on a task with full permissions inside the dev container:

```bash
# 1. Start the dev container (from repo root)
npx @devcontainers/cli up --workspace-folder .

# 2. Exec Claude inside it
npx @devcontainers/cli exec --workspace-folder . \
  claude --dangerously-skip-permissions \
  "Read CLAUDE.md, then complete the task described in TASKS/yaml-config-datasets.md. Verify all acceptance criteria are met before finishing."
```

Replace the task filename with whichever milestone you want to run.

## Updating Task Status

When a milestone is complete, update the Status column in this table to `done`.
