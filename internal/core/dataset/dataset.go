package dataset

import "github.com/beetio/datacow/internal/core/db"

// Dataset represents a named view of data — either a plain table or a saved SQL query.
type Dataset struct {
	Name  string
	Table string // set for auto-discovered or named table datasets
	SQL   string // set for custom SQL query datasets
}

// QueryOptions controls pagination, filtering, and sorting for a dataset query.
type QueryOptions struct {
	Page     int
	PageSize int
	Filters  []Filter
	Sort     *Sort
}

// Filter describes a single WHERE condition.
type Filter struct {
	Column   string
	Operator string // "=", "like", ">", "<", ">=", "<="
	Value    any
}

// Sort describes an ORDER BY clause.
type Sort struct {
	Column string
	Desc   bool
}

// QueryResult holds the rows returned by a dataset query plus pagination metadata.
type QueryResult struct {
	Columns    []db.Column
	Rows       []map[string]any
	TotalRows  int64
	Page       int
	PageSize   int
	TotalPages int
}
