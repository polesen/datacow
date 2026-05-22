# Datacow — Task Milestones

Each file is a self-contained task Claude can work on autonomously.
See `CONTEXT.md` for how tasks are written and structured.
Read `CLAUDE.md` in the repo root before starting any task.

## Ready

Tasks in `ready/` are scoped and ready to run. Pick any of them:

```bash
./tasks/run-task.sh ready/<task>.md
```

## Done

- [000001 Project Scaffold](done/000001-project-scaffold.md)
- [000002 Database Core Layer](done/000002-db-core.md)
- [000003 Dataset Layer](done/000003-dataset-layer.md)
- [000004 TUI Shell](done/000004-tui-shell.md)
- [000005 TUI Split Layout](done/000005-tui-split-layout.md)
- [000006 TUI Table Browser](done/000006-tui-table-browser.md) ⭐ MVP
- [000007 Filter, Sort, Export](done/000007-filter-sort-export.md)
- [000008 FK Drill-Down](done/000008-fk-drilldown.md)
- [000009 TUI Query Log](done/000009-tui-query-log.md)
- [000010 YAML Config + Custom Datasets](done/000010-yaml-config-datasets.md)
- [000011 Multi-Datasource TUI](done/000011-multi-datasource.md)
- [000012 Schema Explorer Tree](done/000012-schema-explorer.md)
- [000013 Cell Viewer + Save to File](done/000013-cell-viewer.md)
- [000014 Maximize Pane](done/000014-maximize-pane.md)
- [000015 Fuzzy Goto](done/000015-fuzzy-goto.md)
- [000016 Fix: Mixed Receivers](done/000016-fix-mixed-receivers.md)
- [000017 Fix: Export Cancellation](done/000017-fix-export-cancellation.md)
- [000018 Fix: Context Param Order](done/000018-fix-context-param-order.md)
- [000019 Help Overlay](done/000019-help-overlay.md)
- [000020 Query Log Improvements](done/000020-query-log-improvements.md)
- [000021 Query Log Filter System](done/000021-query-log-filter-system.md)
- [000022 Table Info Overlay](done/000022-table-info-overlay.md)
- [000023 Filter / Search Redesign](done/000023-filter-search-redesign.md)
- [000024 Fix: Shift+Tab Reverse Focus](done/000024-fix-shift-tab-reverse-focus.md)
- [000025 Table List Filter Search](done/000025-tablelist-filter-search.md)
- [000026 Row Browser: Per-Dataset Page Size, No Default COUNT(\*)](done/000026-page-size-and-no-count.md)
- [000027 Reverse FK Drill-Down](done/000027-reverse-fk-drilldown.md)
- [000028 Table List Filter Bar Visibility](done/000028-tablelist-filter-bar-visibility.md)
- [000029 Column Picker — Select and Reorder Visible Columns](done/000029-column-picker.md)
- [000030 Save Table View as Perspective](done/000030-save-table-perspective.md)
- [000031 Multi-Column Sort](done/000031-multi-column-sort.md)
- [000032 SQL Dataset Editor with Completions](done/000032-sql-dataset-editor.md)
- [000033 g/G Goto First/Last — Tables Pane and Row Browser](done/000033-goto-first-last.md)

## Future Work

Not planned for the current version. Kept in `future/` for reference.

- [HTTP API Server](future/http-api.md) — enables the web app; defer until web UI is prioritised
- [Web App](future/web-app.md) — depends on HTTP API; full web UI for non-terminal users
