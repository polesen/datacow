# M10 — YAML Config + Custom Datasets in TUI

## Goal

Implement config file loading for datasources and datasets, and surface custom SQL datasets
alongside auto-discovered tables in the TUI. This is the first step toward a configured,
persistent setup on top of the zero-config foundation.

## Depends On

M3 (Dataset Layer), M5 (TUI Table Browser)

## Background

The dataset executor already fully supports custom SQL (`Dataset{SQL: "..."}`), but there is
currently no way to define custom datasets other than via code. The `internal/core/config/`
package is empty. This milestone wires that up end-to-end.

## Datasource Scoping (design decision)

Datasets use a **soft-link model**:

- `datasource` field is **optional**.
- If set, the dataset is only shown when that named datasource is the active connection.
- If omitted, the dataset is shown against *any* active connection.

This means simple setups (one connection string on the CLI) work with no extra config. Staging/prod
mirrors can share a `datasets.yaml` with no `datasource` field. Complex setups can pin datasets to
specific connections by name.

## Acceptance Criteria

- [ ] `internal/core/config` package: loads `datasources.yaml` and `datasets.yaml`
- [ ] `Config`, `DatasourceConfig`, `DatasetConfig` types defined
- [ ] CLI: `--config` flag (default: `~/.datacow/config.yaml` or `./datacow.yaml`)
- [ ] Config file supports both datasources and datasets in one file (see format below)
- [ ] Custom datasets from config merged with auto-discovered tables in TUI table list
- [ ] Custom table-reference datasets (`table:`) shown same as auto-discovered
- [ ] Custom SQL datasets (`sql:`) shown with a visual marker in the list (e.g., dim color or "(query)" label)
- [ ] Dataset scoping: datasets with `datasource: <name>` only appear when that datasource is active
- [ ] Dataset scoping: datasets without `datasource` appear for any active connection
- [ ] Zero-config mode (`--connection-string` only, no config file) continues to work unchanged
- [ ] Tests for config loading: valid file, missing file, malformed YAML, missing required fields
- [ ] Tests for dataset merging: auto-discovered + config, scoped + unscoped

## Config File Format

```yaml
# datacow.yaml

datasources:
  - name: production
    connection_string: postgres://user:pass@prod-host/mydb
  - name: staging
    connection_string: postgres://user:pass@staging-host/mydb

datasets:
  # Custom SQL dataset pinned to a specific datasource
  - name: active_orders
    datasource: production
    sql: |
      SELECT o.id, o.created_at, c.name AS customer, o.total
      FROM orders o
      JOIN customers c ON c.id = o.customer_id
      WHERE o.status = 'active'

  # Custom SQL dataset that works with any connected DB (same schema)
  - name: recent_signups
    sql: SELECT id, email, created_at FROM users ORDER BY created_at DESC LIMIT 100

  # Table reference — same as auto-discovered but explicitly named
  - name: Products
    table: products
```

## Types to Implement

```go
// internal/core/config/config.go

type Config struct {
    Datasources []DatasourceConfig `yaml:"datasources"`
    Datasets    []DatasetConfig    `yaml:"datasets"`
}

type DatasourceConfig struct {
    Name             string `yaml:"name"`
    ConnectionString string `yaml:"connection_string"`
}

type DatasetConfig struct {
    Name       string `yaml:"name"`
    Datasource string `yaml:"datasource"` // optional; empty = any connection
    Table      string `yaml:"table"`      // mutually exclusive with SQL
    SQL        string `yaml:"sql"`        // mutually exclusive with Table
}

// Load reads and parses a config file. Returns empty Config if file does not exist.
func Load(path string) (*Config, error)

// DefaultPaths returns the search order for config files:
// 1. ./datacow.yaml
// 2. ~/.datacow/config.yaml
func DefaultPaths() []string
```

## TUI Integration

The `Resolver` (currently only auto-discovers tables) needs to be extended to also include
config-defined datasets. Two options:

**Option A** — Extend `Resolver` to accept config datasets:
```go
func NewResolver(client db.Client, configDatasets []DatasetConfig, activeDatasourceName string) *Resolver
```

**Option B** — Separate `ConfigResolver` that merges after auto-discovery:
```go
type ConfigResolver struct {
    config             *config.Config
    activeDatasource   string
}
func (r *ConfigResolver) Filter(datasets []dataset.Dataset) []dataset.Dataset
```

Prefer Option A for simplicity — the resolver already owns the "what datasets are available" concern.
The active datasource name comes from whichever connection is active (CLI flag name or config name).

When no `--connection-string` is given but a config file is present with datasources, the TUI
should prompt to select a datasource (deferred to M11 — for now, require `--connection-string`
or exactly one datasource in config).

## Notes

- Use `gopkg.in/yaml.v3` for YAML parsing (already a common Go choice; check go.mod first).
- Config loading must not fail hard if the file is missing — zero-config mode must still work.
- `DatasetConfig` validation: `name` required; exactly one of `table` or `sql` required.
- Column/table name injection is already handled in the executor — no new surface here.
- The `DatasourceConfig.Name` for the active connection when using `--connection-string` alone
  should be treated as `""` (empty string), so only unscoped datasets are shown.

## Verify

```bash
go test ./internal/core/config/...
go test ./internal/core/dataset/...   # resolver tests with config datasets
go build ./...
make lint
staticcheck ./...
```
