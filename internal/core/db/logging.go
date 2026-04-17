package db

import (
	"context"
	"strings"
)

// LoggingClient wraps a db.Client and logs all calls to a QueryLog.
type LoggingClient struct {
	inner Client
	log   *QueryLog
}

// NewLoggingClient returns a Client that records every call in the QueryLog.
func NewLoggingClient(inner Client, log *QueryLog) *LoggingClient {
	return &LoggingClient{inner: inner, log: log}
}

func (c *LoggingClient) Ping(ctx context.Context) error {
	return c.inner.Ping(ctx)
}

func (c *LoggingClient) ListTables(ctx context.Context) ([]string, error) {
	id := c.log.begin("list tables", "", QueryKindSystem)
	tables, err := c.inner.ListTables(ctx)
	c.log.end(id, int64(len(tables)), err)
	return tables, err
}

func (c *LoggingClient) Describe(ctx context.Context, table string) ([]Column, error) {
	id := c.log.begin("describe "+table, "", QueryKindSystem)
	cols, err := c.inner.Describe(ctx, table)
	c.log.end(id, int64(len(cols)), err)
	return cols, err
}

func (c *LoggingClient) ForeignKeys(ctx context.Context, table string) ([]ForeignKey, error) {
	id := c.log.begin("FK "+table, "", QueryKindSystem)
	fks, err := c.inner.ForeignKeys(ctx, table)
	c.log.end(id, int64(len(fks)), err)
	return fks, err
}

func (c *LoggingClient) Query(ctx context.Context, sql string, args ...any) ([]map[string]any, error) {
	id := c.log.begin("query", sql, kindFromSQL(sql))
	rows, err := c.inner.Query(ctx, sql, args...)
	c.log.end(id, int64(len(rows)), err)
	return rows, err
}

func (c *LoggingClient) Placeholder(n int) string {
	return c.inner.Placeholder(n)
}

func (c *LoggingClient) Close() error {
	return c.inner.Close()
}

func kindFromSQL(sql string) QueryKind {
	up := strings.ToUpper(sql)
	if strings.Contains(up, "INFORMATION_SCHEMA") ||
		strings.Contains(up, "PG_CATALOG") ||
		strings.Contains(up, "_DC_SCHEMA") {
		return QueryKindSystem
	}
	return QueryKindUser
}
