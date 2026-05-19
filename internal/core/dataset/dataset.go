package dataset

import "github.com/polesen/datacow/internal/core/db"

// Kind classifies a Dataset by its underlying object type.
type Kind string

const (
	KindTable   Kind = "table"   // auto-discovered or YAML-referenced base table
	KindView    Kind = "view"    // auto-discovered database view
	KindDataset Kind = "dataset" // YAML-defined custom SQL query
)

// Dataset represents a named view of data — either a plain table or a saved SQL query.
type Dataset struct {
	Name  string
	Table string // set for auto-discovered or named table datasets
	SQL   string // set for custom SQL query datasets
	Kind  Kind
}

// QueryOptions controls pagination, filtering, and sorting for a dataset query.
type QueryOptions struct {
	Page     int
	PageSize int
	Filters  []Filter
	Sort     *Sort

	// Columns is the ordered list of column names to SELECT.
	// nil or empty means SELECT * (all columns, schema order).
	Columns []string

	// SkipCount disables the COUNT(*) query and uses PageSize+1 row probing
	// to populate HasMore. TotalRows and TotalPages on the result are nil.
	SkipCount bool

	// OnlyCount runs only the COUNT(*) query — the data SELECT is skipped.
	// The returned QueryResult has Columns and TotalRows/TotalPages set,
	// Rows is empty, HasMore is unused. Used by goto-last.
	// Mutually exclusive with SkipCount.
	OnlyCount bool
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
	Columns  []db.Column
	Rows     []map[string]any
	Page     int
	PageSize int

	// TotalRows is nil when the total was not computed (SkipCount path
	// without end-of-data discovery). Set by the executor only in the
	// default-count path and the OnlyCount path.
	TotalRows *int64

	// TotalPages mirrors TotalRows — nil when unknown.
	TotalPages *int

	// HasMore is true when the executor detected that another page exists
	// beyond the returned rows. Always populated; in the default-count
	// path it is derived from TotalPages and Page.
	HasMore bool
}
