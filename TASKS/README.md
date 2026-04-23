# Datacow — Task Milestones

Each file is a self-contained task Claude can work on autonomously.
Read `CLAUDE.md` in the repo root before starting any task.

## Up Next

- [Fuzzy Goto](ready/fuzzy-goto.md) — `ctrl+p` global fuzzy search over tables, views, datasets, columns, datasources; schema cache in core; `ctrl+r` refresh
- [Maximize Pane](ready/maximize-pane.md) — `z` zooms the focused pane to full screen; `z` or `esc` restores the split

## Done

- [Project Scaffold](done/project-scaffold.md)
- [Database Core Layer](done/db-core.md)
- [Dataset Layer](done/dataset-layer.md)
- [TUI Shell](done/tui-shell.md)
- [TUI Split Layout](done/tui-split-layout.md)
- [TUI Table Browser](done/tui-table-browser.md) ⭐ MVP
- [Filter, Sort, Export](done/filter-sort-export.md)
- [FK Drill-Down](done/fk-drilldown.md)
- [TUI Query Log](done/tui-query-log.md)
- [YAML Config + Custom Datasets](done/yaml-config-datasets.md)
- [Multi-Datasource TUI](done/multi-datasource.md)
- [Schema Explorer Tree](done/schema-explorer.md)
- [Cell Viewer + Save to File](done/cell-viewer.md)

## Future Work

Not planned for the current version. Kept in `future/` for reference.

- [HTTP API Server](future/http-api.md) — enables the web app; defer until web UI is prioritised
- [Web App](future/web-app.md) — depends on HTTP API; full web UI for non-terminal users

## Running Autonomously

To run Claude on a task with full permissions inside the dev container:

```bash
# 1. Start the dev container (from repo root)
npx @devcontainers/cli up --workspace-folder .

# 2. Exec Claude inside it
npx @devcontainers/cli exec --workspace-folder . \
  claude --dangerously-skip-permissions \
  "Read CLAUDE.md, then complete the task described in TASKS/cell-viewer.md. Verify all acceptance criteria are met before finishing."
```

Replace the task filename with whichever milestone you want to run.

## Completing a Milestone

When a task is done, move its file to `done/` and move the entry from **Up Next** to **Done**.
