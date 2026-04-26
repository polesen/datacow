# M11 — Multi-Datasource TUI

## Goal

Extend the TUI to support multiple datasources configured in `datasources.yaml`. When more
than one datasource is available, the user is presented with a datasource picker before
reaching the table/dataset list. The connection is established lazily on selection.

## Depends On

M10 (YAML Config + Custom Datasets)

## Background

M10 adds config loading and custom datasets for a single active connection. This milestone
adds the multi-datasource UX: a new pane for selecting which database to connect to, lazy
connection establishment, and the ability to switch between datasources within a session.

## Acceptance Criteria

- [ ] New `DatasourceListModel` pane in TUI shown when multiple datasources are configured
- [ ] Datasource list shows: name, connection string (truncated), connection status (disconnected / connecting / connected / error)
- [ ] Selecting a datasource connects to it (lazy — no connection until selected)
- [ ] Connection happens asynchronously; status updates shown while connecting
- [ ] Connection error shown inline in the datasource list (not a hard crash)
- [ ] Once connected, transition to the existing table/dataset list (M10 scoped datasets apply)
- [ ] `Esc` or back key returns to datasource list from table list
- [ ] If only one datasource configured (or `--connection-string` used), datasource list is skipped
- [ ] Switching datasources within a session is supported (go back, select another)
- [ ] Previously connected datasources show status (connected) — re-selecting reuses the connection
- [ ] Connections are closed cleanly when the TUI exits
- [ ] Tests for datasource list model state machine (connecting, connected, error, switch)

## TUI Layout

When in datasource selection mode, the 3-pane layout collapses to a centered/left-pane
datasource picker. The right panes are empty until a connection is established.

```
┌─────────────────────────────────────────────────────┐
│  Datasources             │                           │
│ ──────────────────────── │                           │
│  production   connected  │  Select a datasource      │
│  staging      —          │  to browse its data.      │
│  local-dev    —          │                           │
│                          │                           │
│  [Enter] connect         │                           │
│  [q] quit                │                           │
└─────────────────────────────────────────────────────┘
```

## State Machine

```
AppState:
  DatasourceSelection   ← shown when >1 datasource configured
  TableList             ← existing state (single datasource or post-selection)
  RowBrowser            ← existing state
```

Transition: `DatasourceSelection → TableList` on successful connection.
Transition: `TableList → DatasourceSelection` on Esc (when multi-datasource mode).

## Types to Add

```go
// internal/tui/views/datasourcelist.go

type DatasourceListModel struct {
    datasources []config.DatasourceConfig
    statuses    map[string]ConnStatus
    cursor      int
    keys        *keys.Bindings
}

type ConnStatus int
const (
    StatusDisconnected ConnStatus = iota
    StatusConnecting
    StatusConnected
    StatusError
)

type DatasourceConnectedMsg struct {
    Name   string
    Client db.Client
}

type DatasourceErrorMsg struct {
    Name string
    Err  error
}
```

## Connection Management

The `App` maintains a map of open connections keyed by datasource name. When the user
selects a datasource:

1. If a connection for that name already exists → reuse it, transition immediately.
2. If not → fire async connect command, show "connecting..." status, await `DatasourceConnectedMsg`.

On exit, `App.Close()` iterates the connection map and calls `client.Close()` on each.

## Notes

- The `--connection-string` CLI flag still works as before. It implies single-datasource mode
  and the TUI starts directly at the table list.
- If `datasources.yaml` has exactly one entry, the TUI connects automatically (no picker shown).
- This milestone does NOT add per-datasource auth (API keys, users.yaml) — that comes later.
- Do not try to detect which config datasets are "valid" for a given datasource before connecting —
  discover that at runtime when the user selects a dataset.

## Verify

```bash
go test ./internal/tui/...
go test ./internal/core/config/...
go build ./...
make lint
staticcheck ./...
```
