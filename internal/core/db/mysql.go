package db

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/go-sql-driver/mysql"
)

type mysqlClient struct {
	db *sql.DB
}

func newMySQLClient(dsn string) (Client, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("open mysql: %w", err)
	}
	return &mysqlClient{db: db}, nil
}

func (c *mysqlClient) Ping(ctx context.Context) error {
	return c.db.PingContext(ctx)
}

func (c *mysqlClient) ListTables(ctx context.Context) ([]string, error) {
	rows, err := c.db.QueryContext(ctx, `
		SELECT TABLE_NAME
		FROM information_schema.TABLES
		WHERE TABLE_SCHEMA = DATABASE()
		  AND TABLE_TYPE IN ('BASE TABLE', 'VIEW')
		ORDER BY TABLE_NAME
	`)
	if err != nil {
		return nil, fmt.Errorf("list tables: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tables = append(tables, name)
	}
	return tables, rows.Err()
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
