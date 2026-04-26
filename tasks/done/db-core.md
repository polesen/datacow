# M2 — Database Core Layer

## Goal
Implement the core database abstraction: a `Client` interface with working Postgres and MySQL drivers. Schema introspection (tables, columns, foreign keys) must work against real databases.

## Depends On
M1 (project scaffold)

## Acceptance Criteria
- [ ] `core/db.Client` interface defined with methods below
- [ ] Postgres driver implemented and tested against real DB
- [ ] MySQL driver implemented and tested against real DB
- [ ] `core/schema` package: list tables, list columns (name, type, nullable), detect FK relationships
- [ ] `core/db.Connect(dsn string) (Client, error)` auto-detects driver from DSN prefix (`postgres://`, `mysql://`)
- [ ] All tests pass against real DBs (use `TEST_POSTGRES_DSN` and `TEST_MYSQL_DSN` env vars — set in devcontainer)

## Interface to Implement

```go
// internal/core/db/client.go
type Client interface {
    // Ping verifies the connection is alive
    Ping(ctx context.Context) error

    // ListTables returns all table and view names in the default schema
    ListTables(ctx context.Context) ([]string, error)

    // Describe returns column metadata for a table
    Describe(ctx context.Context, table string) ([]Column, error)

    // ForeignKeys returns FK relationships for a table
    ForeignKeys(ctx context.Context, table string) ([]ForeignKey, error)

    // Query executes a SELECT and returns rows as generic maps
    Query(ctx context.Context, sql string, args ...any) ([]map[string]any, error)

    // Close releases the connection
    Close() error
}

type Column struct {
    Name     string
    Type     string
    Nullable bool
}

type ForeignKey struct {
    Column           string
    ReferencedTable  string
    ReferencedColumn string
}
```

## Notes
- Use `github.com/lib/pq` or `github.com/jackc/pgx/v5` for Postgres (prefer pgx)
- Use `github.com/go-sql-driver/mysql` for MySQL
- Schema introspection queries differ per DB — each driver implements its own `ListTables`, `Describe`, `ForeignKeys`
- Tests should create a temp schema, insert known data, and assert introspection results

## Verify
```bash
TEST_POSTGRES_DSN="postgres://datacow:datacow@localhost:5432/datacow_test?sslmode=disable" \
TEST_MYSQL_DSN="datacow:datacow@tcp(localhost:3306)/datacow_test" \
go test ./internal/core/...
```
