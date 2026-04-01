package db

import "context"

// Client is the interface all database drivers must implement.
// Each driver (postgres, mysql, sqlite, ...) lives in its own file in this package.
type Client interface {
	// Ping verifies the connection is alive.
	Ping(ctx context.Context) error

	// ListTables returns all table and view names in the default schema.
	ListTables(ctx context.Context) ([]string, error)

	// Describe returns column metadata for a table.
	Describe(ctx context.Context, table string) ([]Column, error)

	// ForeignKeys returns FK relationships originating from a table.
	ForeignKeys(ctx context.Context, table string) ([]ForeignKey, error)

	// Query executes a SQL SELECT and returns rows as generic maps.
	Query(ctx context.Context, sql string, args ...any) ([]map[string]any, error)

	// Close releases the connection.
	Close() error
}

// Column describes a single column in a table.
type Column struct {
	Name     string
	Type     string
	Nullable bool
}

// ForeignKey describes a FK relationship from a column in one table to another.
type ForeignKey struct {
	Column           string
	ReferencedTable  string
	ReferencedColumn string
}
