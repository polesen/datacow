# Datacow — Task Milestones

Each file is a self-contained task Claude can work on autonomously.
See `CONTEXT.md` for how tasks are written and structured.
Read `CLAUDE.md` in the repo root before starting any task.

## Ready

Tasks in `ready/` are scoped and ready to run. Pick any of them:

```bash
./tasks/run-task.sh ready/<task>.md
```

- [Filter Bar Visibility — Tables Pane](ready/tablelist-filter-bar-visibility.md)
- [Column Picker — Select and Reorder Visible Columns](ready/column-picker.md)

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
- [Filter / Search Redesign](done/filter-search-redesign.md)
- [Fix: Shift+Tab Reverse Focus](done/fix-shift-tab-reverse-focus.md)
- [Reverse FK Drill-Down](done/reverse-fk-drilldown.md)
- [Row Browser: Per-Dataset Page Size, No Default COUNT(\*)](done/page-size-and-no-count.md)
- [Table List Filter Search](done/tablelist-filter-search.md)

## Future Work

Not planned for the current version. Kept in `future/` for reference.

- [HTTP API Server](future/http-api.md) — enables the web app; defer until web UI is prioritised
- [Web App](future/web-app.md) — depends on HTTP API; full web UI for non-terminal users
