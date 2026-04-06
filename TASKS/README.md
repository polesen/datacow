# Datacow — Task Milestones

Each file is a self-contained task Claude can work on autonomously.
Read `CLAUDE.md` in the repo root before starting any task.

## Milestone Order

| Milestone | Task | Depends On | Status |
|---|---|---|---|
| M1 | [Project Scaffold](M1-project-scaffold.md) | — | done |
| M2 | [Database Core Layer](M2-db-core.md) | M1 | todo |
| M3 | [Dataset Layer](M3-dataset-layer.md) | M2 | todo |
| M4 | [TUI Shell](M4-tui-shell.md) | M1 | todo |
| M5 | [TUI Table Browser](M5-tui-table-browser.md) ⭐ MVP | M2, M3, M4 | todo |
| M6 | [Filter, Sort, Export](M6-filter-sort-export.md) | M5 | todo |
| M7 | [FK Drill-Down](M7-fk-drilldown.md) | M5 | todo |
| M8 | [HTTP API Server](M8-http-api.md) | M3 | todo |
| M9 | [Web App](M9-web-app.md) | M8 | todo |

## Running Autonomously

To run Claude on a task with full permissions inside the dev container:

```bash
# Start dev container
docker compose -f .devcontainer/docker-compose.yml up -d

# Run Claude on a specific task
claude --dangerouslySkipPermissions "Read CLAUDE.md, then complete the task described in $@. Verify all acceptance criteria are met before finishing."
```

## Updating Task Status

When a milestone is complete, update the Status column in this table to `done`.
