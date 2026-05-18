package dataset

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/polesen/datacow/internal/core/db"
)

// identRe matches safe SQL identifiers: letters, digits, underscore, starting with letter/underscore.
var identRe = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

// validOperators is the closed set of allowed filter operators.
var validOperators = map[string]string{
	"=":    "=",
	"like": "LIKE",
	">":    ">",
	"<":    "<",
	">=":   ">=",
	"<=":   "<=",
}

// Executor runs parameterized queries against a dataset.
type Executor struct {
	client db.Client
}

// NewExecutor returns an Executor backed by the given client.
func NewExecutor(client db.Client) *Executor {
	return &Executor{client: client}
}

// Query executes the dataset with the given options and returns a paginated result.
// Column names in filters and sort are validated against the dataset schema to prevent injection.
// Filter values are always passed as query parameters — never interpolated.
func (e *Executor) Query(ctx context.Context, ds Dataset, opts QueryOptions) (*QueryResult, error) {
	if opts.SkipCount && opts.OnlyCount {
		return nil, fmt.Errorf("SkipCount and OnlyCount are mutually exclusive")
	}

	cols, err := e.columns(ctx, ds)
	if err != nil {
		return nil, fmt.Errorf("resolve columns: %w", err)
	}

	colSet := make(map[string]bool, len(cols))
	for _, c := range cols {
		colSet[c.Name] = true
	}

	if err := e.validateOptions(opts, colSet); err != nil {
		return nil, err
	}

	page := opts.Page
	if page < 1 {
		page = 1
	}
	pageSize := opts.PageSize
	if pageSize < 1 {
		pageSize = 50
	}

	from := e.fromClause(ds)
	where, args := e.buildWhere(opts.Filters, 1)

	order := ""
	if opts.Sort != nil {
		dir := "ASC"
		if opts.Sort.Desc {
			dir = "DESC"
		}
		order = " ORDER BY " + opts.Sort.Column + " " + dir
	}

	offset := (page - 1) * pageSize
	countSQL := "SELECT COUNT(*) AS _dc_count FROM " + from + where

	if opts.OnlyCount {
		countRows, err := e.client.Query(ctx, countSQL, args...)
		if err != nil {
			return nil, fmt.Errorf("count: %w", err)
		}
		total := extractCount(countRows)
		totalPages := int((total + int64(pageSize) - 1) / int64(pageSize))
		if totalPages < 1 {
			totalPages = 1
		}
		return &QueryResult{
			Columns:    cols,
			Rows:       nil,
			Page:       page,
			PageSize:   pageSize,
			TotalRows:  &total,
			TotalPages: &totalPages,
		}, nil
	}

	n := len(args) + 1
	limitSQL := " LIMIT " + e.client.Placeholder(n) + " OFFSET " + e.client.Placeholder(n+1)
	dataSQL := "SELECT * FROM " + from + where + order + limitSQL

	if opts.SkipCount {
		// Fetch one extra row to detect whether another page exists.
		dataArgs := append(args, pageSize+1, offset) //nolint:gocritic
		rows, err := e.client.Query(ctx, dataSQL, dataArgs...)
		if err != nil {
			return nil, fmt.Errorf("query: %w", err)
		}
		hasMore := len(rows) > pageSize
		if hasMore {
			rows = rows[:pageSize]
		}
		return &QueryResult{
			Columns:  cols,
			Rows:     rows,
			Page:     page,
			PageSize: pageSize,
			HasMore:  hasMore,
			// TotalRows, TotalPages: nil
		}, nil
	}

	// Default: run COUNT and data queries concurrently — they are independent.
	type countResult struct {
		total int64
		err   error
	}
	countCh := make(chan countResult, 1)
	go func() {
		countRows, err := e.client.Query(ctx, countSQL, args...)
		if err != nil {
			countCh <- countResult{err: fmt.Errorf("count: %w", err)}
			return
		}
		countCh <- countResult{total: extractCount(countRows)}
	}()

	dataArgs := append(args, pageSize, offset) //nolint:gocritic
	rows, err := e.client.Query(ctx, dataSQL, dataArgs...)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}

	cr := <-countCh
	if cr.err != nil {
		return nil, cr.err
	}

	total := cr.total
	totalPages := int((total + int64(pageSize) - 1) / int64(pageSize))
	if totalPages < 1 {
		totalPages = 1
	}
	hasMore := page < totalPages

	return &QueryResult{
		Columns:    cols,
		Rows:       rows,
		Page:       page,
		PageSize:   pageSize,
		TotalRows:  &total,
		TotalPages: &totalPages,
		HasMore:    hasMore,
	}, nil
}

// columns returns column metadata for the dataset.
// For table datasets it uses Describe; for SQL datasets it fetches one row to read map keys.
func (e *Executor) columns(ctx context.Context, ds Dataset) ([]db.Column, error) {
	if ds.Table != "" {
		if !identRe.MatchString(ds.Table) {
			return nil, fmt.Errorf("invalid table name %q", ds.Table)
		}
		return e.client.Describe(ctx, ds.Table)
	}
	// SQL dataset: Query returns []map[string]any — column names come from map keys.
	// LIMIT 1 is the minimum needed; LIMIT 0 always returns empty with no key metadata.
	rows, err := e.client.Query(ctx, "SELECT * FROM ("+ds.SQL+") AS _dc_schema LIMIT 1")
	if err != nil {
		return nil, fmt.Errorf("schema probe: %w", err)
	}
	if len(rows) == 0 {
		return nil, nil
	}
	cols := make([]db.Column, 0, len(rows[0]))
	for name := range rows[0] {
		cols = append(cols, db.Column{Name: name})
	}
	return cols, nil
}

// fromClause returns the SQL FROM target for a dataset.
// Table name safety is guaranteed by the identRe check in columns().
func (e *Executor) fromClause(ds Dataset) string {
	if ds.Table != "" {
		return ds.Table
	}
	return "(" + ds.SQL + ") AS _dc_dataset"
}

// buildWhere builds a parameterized WHERE clause.
// startN is the 1-based index of the first placeholder.
func (e *Executor) buildWhere(filters []Filter, startN int) (string, []any) {
	if len(filters) == 0 {
		return "", nil
	}
	var parts []string
	var args []any
	n := startN
	for _, f := range filters {
		op := validOperators[strings.ToLower(f.Operator)]
		parts = append(parts, f.Column+" "+op+" "+e.client.Placeholder(n))
		args = append(args, f.Value)
		n++
	}
	return " WHERE " + strings.Join(parts, " AND "), args
}

// validateOptions checks that all filter/sort column names are safe identifiers
// and exist in the dataset schema.
func (e *Executor) validateOptions(opts QueryOptions, colSet map[string]bool) error {
	for _, f := range opts.Filters {
		if !identRe.MatchString(f.Column) {
			return fmt.Errorf("invalid filter column name %q", f.Column)
		}
		if !colSet[f.Column] {
			return fmt.Errorf("unknown filter column %q", f.Column)
		}
		if _, ok := validOperators[strings.ToLower(f.Operator)]; !ok {
			return fmt.Errorf("unsupported filter operator %q", f.Operator)
		}
	}
	if opts.Sort != nil {
		if !identRe.MatchString(opts.Sort.Column) {
			return fmt.Errorf("invalid sort column name %q", opts.Sort.Column)
		}
		if !colSet[opts.Sort.Column] {
			return fmt.Errorf("unknown sort column %q", opts.Sort.Column)
		}
	}
	return nil
}

// ForeignKeys returns FK relationships for the given table.
func (e *Executor) ForeignKeys(ctx context.Context, table string) ([]db.ForeignKey, error) {
	return e.client.ForeignKeys(ctx, table)
}

// PrimaryKeyColumns returns the ordered list of primary-key column names for a table.
// It identifies the PK by looking for an index named "PRIMARY" (MySQL) or ending in "_pkey"
// (Postgres convention). Falls back to the first unique index if neither is found.
func (e *Executor) PrimaryKeyColumns(ctx context.Context, table string) ([]string, error) {
	indexes, err := e.client.Indexes(ctx, table)
	if err != nil {
		return nil, err
	}
	for _, idx := range indexes {
		if idx.Name == "PRIMARY" || strings.HasSuffix(idx.Name, "_pkey") {
			return idx.Columns, nil
		}
	}
	for _, idx := range indexes {
		if idx.Unique {
			return idx.Columns, nil
		}
	}
	return nil, nil
}

// extractCount pulls the _dc_count value from a COUNT(*) result row.
func extractCount(rows []map[string]any) int64 {
	if len(rows) == 0 {
		return 0
	}
	v := rows[0]["_dc_count"]
	switch n := v.(type) {
	case int64:
		return n
	case int32:
		return int64(n)
	case float64:
		return int64(n)
	case []byte:
		var i int64
		_, _ = fmt.Sscan(string(n), &i)
		return i
	default:
		return 0
	}
}
