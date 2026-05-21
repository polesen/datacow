package db

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

var _ Client = (*postgresClient)(nil)
var _ StatsProvider = (*postgresClient)(nil)

type postgresClient struct {
	db *sql.DB
}

func newPostgresClient(dsn string) (Client, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	return &postgresClient{db: db}, nil
}

func (c *postgresClient) Ping(ctx context.Context) error {
	return c.db.PingContext(ctx)
}

func (c *postgresClient) ListTables(ctx context.Context) ([]TableEntry, error) {
	rows, err := c.db.QueryContext(ctx, `
		SELECT table_name, table_type
		FROM information_schema.tables
		WHERE table_schema = 'public'
		  AND table_type IN ('BASE TABLE', 'VIEW')
		ORDER BY table_name
	`)
	if err != nil {
		return nil, fmt.Errorf("list tables: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var entries []TableEntry
	for rows.Next() {
		var name, tableType string
		if err := rows.Scan(&name, &tableType); err != nil {
			return nil, err
		}
		kind := KindTable
		if tableType == "VIEW" {
			kind = KindView
		}
		entries = append(entries, TableEntry{Name: name, Kind: kind})
	}
	return entries, rows.Err()
}

func (c *postgresClient) Indexes(ctx context.Context, table string) ([]Index, error) {
	rows, err := c.db.QueryContext(ctx, `
		SELECT
			i.relname                       AS index_name,
			ix.indisunique                  AS is_unique,
			a.attname                       AS column_name,
			array_position(ix.indkey, a.attnum) AS ord
		FROM pg_class t
		JOIN pg_namespace n  ON n.oid = t.relnamespace
		JOIN pg_index ix     ON ix.indrelid = t.oid
		JOIN pg_class i      ON i.oid = ix.indexrelid
		JOIN pg_attribute a  ON a.attrelid = t.oid AND a.attnum = ANY(ix.indkey)
		WHERE n.nspname = 'public'
		  AND t.relname = $1
		ORDER BY i.relname, ord
	`, table)
	if err != nil {
		return nil, fmt.Errorf("indexes %s: %w", table, err)
	}
	defer func() { _ = rows.Close() }()

	type partial struct {
		unique bool
		cols   []string
	}
	byName := map[string]*partial{}
	var order []string
	for rows.Next() {
		var name, col string
		var unique bool
		var ord int
		if err := rows.Scan(&name, &unique, &col, &ord); err != nil {
			return nil, err
		}
		p, ok := byName[name]
		if !ok {
			p = &partial{unique: unique}
			byName[name] = p
			order = append(order, name)
		}
		p.cols = append(p.cols, col)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]Index, 0, len(order))
	for _, name := range order {
		p := byName[name]
		out = append(out, Index{Name: name, Columns: p.cols, Unique: p.unique})
	}
	return out, nil
}

func (c *postgresClient) Describe(ctx context.Context, table string) ([]Column, error) {
	rows, err := c.db.QueryContext(ctx, `
		SELECT column_name, data_type, is_nullable
		FROM information_schema.columns
		WHERE table_schema = 'public'
		  AND table_name = $1
		ORDER BY ordinal_position
	`, table)
	if err != nil {
		return nil, fmt.Errorf("describe %s: %w", table, err)
	}
	defer func() { _ = rows.Close() }()

	var columns []Column
	for rows.Next() {
		var col Column
		var isNullable string
		if err := rows.Scan(&col.Name, &col.Type, &isNullable); err != nil {
			return nil, err
		}
		col.Nullable = isNullable == "YES"
		columns = append(columns, col)
	}
	return columns, rows.Err()
}

func (c *postgresClient) ForeignKeys(ctx context.Context, table string) ([]ForeignKey, error) {
	rows, err := c.db.QueryContext(ctx, `
		SELECT
			kcu.column_name,
			ccu.table_name  AS referenced_table,
			ccu.column_name AS referenced_column
		FROM information_schema.table_constraints AS tc
		JOIN information_schema.key_column_usage AS kcu
			ON tc.constraint_name = kcu.constraint_name
			AND tc.table_schema   = kcu.table_schema
		JOIN information_schema.constraint_column_usage AS ccu
			ON ccu.constraint_name = tc.constraint_name
			AND ccu.table_schema   = tc.table_schema
		WHERE tc.constraint_type = 'FOREIGN KEY'
		  AND tc.table_schema    = 'public'
		  AND tc.table_name      = $1
		ORDER BY kcu.column_name
	`, table)
	if err != nil {
		return nil, fmt.Errorf("foreign keys %s: %w", table, err)
	}
	defer func() { _ = rows.Close() }()

	var fks []ForeignKey
	for rows.Next() {
		var fk ForeignKey
		if err := rows.Scan(&fk.Column, &fk.ReferencedTable, &fk.ReferencedColumn); err != nil {
			return nil, err
		}
		fks = append(fks, fk)
	}
	return fks, rows.Err()
}

func (c *postgresClient) Query(ctx context.Context, query string, args ...any) ([]map[string]any, error) {
	rows, err := c.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer func() { _ = rows.Close() }()

	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var result []map[string]any
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		row := make(map[string]any, len(cols))
		for i, col := range cols {
			row[col] = vals[i]
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func (c *postgresClient) Placeholder(n int) string {
	return "$" + strconv.Itoa(n)
}

func (c *postgresClient) Dialect() Dialect { return DialectPostgres }

func (c *postgresClient) Close() error {
	return c.db.Close()
}

func (c *postgresClient) TableStats(ctx context.Context, table string) (TableStats, error) {
	row := c.db.QueryRowContext(ctx, `
		SELECT
			c.reltuples::bigint                    AS row_estimate,
			pg_total_relation_size(c.oid)          AS total_bytes,
			pg_relation_size(c.oid)                AS table_bytes,
			pg_indexes_size(c.oid)                 AS index_bytes,
			obj_description(c.oid, 'pg_class')     AS description,
			s.last_autovacuum,
			s.last_autoanalyze
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		LEFT JOIN pg_stat_user_tables s ON s.relid = c.oid
		WHERE n.nspname = 'public'
		  AND c.relname = $1
		  AND c.relkind = 'r'
	`, table)

	var rowEst int64
	var totalBytes, tableBytes, indexBytes int64
	var description sql.NullString
	var lastVacuumed, lastAnalyzed sql.NullTime

	if err := row.Scan(&rowEst, &totalBytes, &tableBytes, &indexBytes,
		&description, &lastVacuumed, &lastAnalyzed); err != nil {
		if err == sql.ErrNoRows {
			return TableStats{}, fmt.Errorf("table not found: %s", table)
		}
		return TableStats{}, fmt.Errorf("table stats %s: %w", table, err)
	}

	var stats TableStats

	if rowEst >= 0 {
		stats.RowEstimate = &rowEst
	}
	stats.TotalBytes = &totalBytes
	stats.TableBytes = &tableBytes
	stats.IndexBytes = &indexBytes
	if description.Valid {
		stats.Description = description.String
	}
	if lastVacuumed.Valid {
		t := lastVacuumed.Time.In(time.UTC)
		stats.LastVacuumed = &t
	}
	if lastAnalyzed.Valid {
		t := lastAnalyzed.Time.In(time.UTC)
		stats.LastAnalyzed = &t
	}
	return stats, nil
}
