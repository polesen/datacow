package db

import "context"

// Client is the interface all database drivers must implement.
// Each driver (postgres, mysql, sqlite, ...) lives in its own file in this package.
type Client interface {
	// Ping verifies the connection is alive.
	Ping(ctx context.Context) error

	// ListTables returns all tables and views in the default schema.
	ListTables(ctx context.Context) ([]TableEntry, error)

	// Describe returns column metadata for a table.
	Describe(ctx context.Context, table string) ([]Column, error)

	// ForeignKeys returns FK relationships originating from a table.
	ForeignKeys(ctx context.Context, table string) ([]ForeignKey, error)

	// Indexes returns all indexes on a table.
	Indexes(ctx context.Context, table string) ([]Index, error)

	// Query executes a SQL SELECT and returns rows as generic maps.
	Query(ctx context.Context, sql string, args ...any) ([]map[string]any, error)

	// Placeholder returns the SQL parameter placeholder for argument position n (1-based).
	// PostgreSQL uses $1, $2, … — MySQL uses ? for every position.
	Placeholder(n int) string

	// Close releases the connection.
	Close() error
}

// TableKind classifies an object returned by ListTables.
type TableKind string

const (
	KindTable TableKind = "table"
	KindView  TableKind = "view"
)

// TableEntry is a single row returned by ListTables.
type TableEntry struct {
	Name string
	Kind TableKind
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

// Index describes a single index on a table.
type Index struct {
	Name    string
	Columns []string
	Unique  bool
}
