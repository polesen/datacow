package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

var _ Client = (*mysqlClient)(nil)
var _ StatsProvider = (*mysqlClient)(nil)

type mysqlClient struct {
	db *sql.DB
}

func newMySQLClient(dsn string) (Client, error) {
	dsn = ensureParseTime(dsn)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("open mysql: %w", err)
	}
	return &mysqlClient{db: db}, nil
}

// ensureParseTime appends parseTime=true to the DSN if not already present.
// Required so that datetime columns from information_schema scan into time.Time.
func ensureParseTime(dsn string) string {
	if strings.Contains(dsn, "parseTime") {
		return dsn
	}
	if strings.Contains(dsn, "?") {
		return dsn + "&parseTime=true"
	}
	return dsn + "?parseTime=true"
}

func (c *mysqlClient) Ping(ctx context.Context) error {
	return c.db.PingContext(ctx)
}

func (c *mysqlClient) ListTables(ctx context.Context) ([]TableEntry, error) {
	rows, err := c.db.QueryContext(ctx, `
		SELECT TABLE_NAME, TABLE_TYPE
		FROM information_schema.TABLES
		WHERE TABLE_SCHEMA = DATABASE()
		  AND TABLE_TYPE IN ('BASE TABLE', 'VIEW')
		ORDER BY TABLE_NAME
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

func (c *mysqlClient) Indexes(ctx context.Context, table string) ([]Index, error) {
	rows, err := c.db.QueryContext(ctx, `
		SELECT INDEX_NAME, NON_UNIQUE, COLUMN_NAME
		FROM information_schema.STATISTICS
		WHERE TABLE_SCHEMA = DATABASE()
		  AND TABLE_NAME   = ?
		ORDER BY INDEX_NAME, SEQ_IN_INDEX
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
		var nonUnique int
		if err := rows.Scan(&name, &nonUnique, &col); err != nil {
			return nil, err
		}
		p, ok := byName[name]
		if !ok {
			p = &partial{unique: nonUnique == 0}
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

func (c *mysqlClient) Describe(ctx context.Context, table string) ([]Column, error) {
	rows, err := c.db.QueryContext(ctx, `
		SELECT COLUMN_NAME, DATA_TYPE, IS_NULLABLE
		FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA = DATABASE()
		  AND TABLE_NAME = ?
		ORDER BY ORDINAL_POSITION
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

func (c *mysqlClient) ForeignKeys(ctx context.Context, table string) ([]ForeignKey, error) {
	rows, err := c.db.QueryContext(ctx, `
		SELECT
			kcu.COLUMN_NAME,
			kcu.REFERENCED_TABLE_NAME,
			kcu.REFERENCED_COLUMN_NAME
		FROM information_schema.KEY_COLUMN_USAGE kcu
		JOIN information_schema.TABLE_CONSTRAINTS tc
			ON tc.CONSTRAINT_NAME = kcu.CONSTRAINT_NAME
			AND tc.TABLE_SCHEMA   = kcu.TABLE_SCHEMA
			AND tc.TABLE_NAME     = kcu.TABLE_NAME
		WHERE tc.CONSTRAINT_TYPE = 'FOREIGN KEY'
		  AND kcu.TABLE_SCHEMA   = DATABASE()
		  AND kcu.TABLE_NAME     = ?
		ORDER BY kcu.COLUMN_NAME
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

func (c *mysqlClient) Query(ctx context.Context, query string, args ...any) ([]map[string]any, error) {
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

func (c *mysqlClient) Placeholder(_ int) string {
	return "?"
}

func (c *mysqlClient) Close() error {
	return c.db.Close()
}

func (c *mysqlClient) TableStats(ctx context.Context, table string) (TableStats, error) {
	row := c.db.QueryRowContext(ctx, `
		SELECT
			TABLE_ROWS,
			DATA_LENGTH,
			INDEX_LENGTH,
			DATA_FREE,
			TABLE_COMMENT,
			CREATE_TIME,
			UPDATE_TIME,
			ENGINE,
			AUTO_INCREMENT
		FROM information_schema.TABLES
		WHERE TABLE_SCHEMA = DATABASE()
		  AND TABLE_NAME   = ?
	`, table)

	var tableRows, dataLen, indexLen, dataFree sql.NullInt64
	var comment sql.NullString
	var createTime, updateTime sql.NullTime
	var engine sql.NullString
	var autoIncr sql.NullInt64

	if err := row.Scan(&tableRows, &dataLen, &indexLen, &dataFree,
		&comment, &createTime, &updateTime, &engine, &autoIncr); err != nil {
		if err == sql.ErrNoRows {
			return TableStats{}, fmt.Errorf("table not found: %s", table)
		}
		return TableStats{}, fmt.Errorf("table stats %s: %w", table, err)
	}

	var stats TableStats

	if tableRows.Valid {
		v := tableRows.Int64
		stats.RowEstimate = &v
	}
	if dataLen.Valid && indexLen.Valid {
		total := dataLen.Int64 + indexLen.Int64
		stats.TotalBytes = &total
		stats.TableBytes = &dataLen.Int64
		stats.IndexBytes = &indexLen.Int64
	} else if dataLen.Valid {
		stats.TableBytes = &dataLen.Int64
	} else if indexLen.Valid {
		stats.IndexBytes = &indexLen.Int64
	}
	if dataFree.Valid {
		stats.FreeBytes = &dataFree.Int64
	}
	if comment.Valid {
		stats.Description = comment.String
	}
	if createTime.Valid {
		t := createTime.Time.In(time.UTC)
		stats.CreatedAt = &t
	}
	if updateTime.Valid {
		t := updateTime.Time.In(time.UTC)
		stats.LastAnalyzed = &t
	}
	if engine.Valid {
		stats.Engine = engine.String
	}
	if autoIncr.Valid {
		stats.NextAutoIncr = &autoIncr.Int64
	}
	return stats, nil
}
