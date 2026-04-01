# M4 — TUI Shell

## Goal
A working Bubble Tea application that starts, renders a basic layout, and quits cleanly. No real data yet — just the shell that future TUI milestones will build on.

## Depends On
M1 (project scaffold)

## Acceptance Criteria
- [ ] `datacow --connection-string=...` launches the TUI
- [ ] TUI renders: a header bar (app name + version), a main content area, a status/help bar at the bottom
- [ ] `q` or `ctrl+c` quits cleanly
- [ ] Keybindings registry: all keys go through a central `keys.go` (not hardcoded in views)
- [ ] Lip Gloss theme file: colours, borders, styles defined in one place (`internal/tui/style/`)
- [ ] Connection string is parsed and passed to the TUI as initial state (doesn't connect yet)
- [ ] Looks good in both light and dark terminals

## Layout Sketch
```
┌─────────────────────────────────────────┐
│ datacow v0.1          mydb@localhost    │  ← header
├─────────────────────────────────────────┤
│                                         │
│         (content area)                  │
│                                         │
├─────────────────────────────────────────┤
│ q quit  ?  help  /  filter              │  ← status bar
└─────────────────────────────────────────┘
```

## Notes
- Use `charmbracelet/bubbletea` and `charmbracelet/lipgloss`
- Add `charmbracelet/bubbles` for reusable components (list, table, textinput, spinner)
- The TUI app struct should accept a `core.Client` (or nil for now) — wired up in `cmd/main.go`
- No test for TUI render output — just verify it starts and exits cleanly via manual run

## Verify
```bash
go run ./cmd --connection-string="postgres://datacow:datacow@localhost:5432/datacow_test?sslmode=disable"
# TUI opens, press q, exits with code 0
```
