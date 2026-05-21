package db

import (
	"context"
	"fmt"
	"strings"
)

var _ Client = (*LoggingClient)(nil)

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

func (c *LoggingClient) ListTables(ctx context.Context) ([]TableEntry, error) {
	id := c.log.begin("list tables", "", QueryKindSystem)
	entries, err := c.inner.ListTables(ctx)
	c.log.end(id, int64(len(entries)), err)
	return entries, err
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

func (c *LoggingClient) Indexes(ctx context.Context, table string) ([]Index, error) {
	id := c.log.begin("indexes "+table, "", QueryKindSystem)
	idx, err := c.inner.Indexes(ctx, table)
	c.log.end(id, int64(len(idx)), err)
	return idx, err
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

func (c *LoggingClient) Dialect() Dialect {
	return c.inner.Dialect()
}

func (c *LoggingClient) Close() error {
	return c.inner.Close()
}

func (c *LoggingClient) TableStats(ctx context.Context, table string) (TableStats, error) {
	sp, ok := c.inner.(StatsProvider)
	if !ok {
		return TableStats{}, fmt.Errorf("statistics not available for this database")
	}
	id := c.log.begin("info "+table, "", QueryKindSystem)
	stats, err := sp.TableStats(ctx, table)
	c.log.end(id, 0, err)
	return stats, err
}

func kindFromSQL(sql string) QueryKind {
	up := strings.ToUpper(sql)
	if strings.Contains(up, "INFORMATION_SCHEMA") ||
		strings.Contains(up, "PG_CATALOG") ||
		strings.Contains(up, "_DC_SCHEMA") ||
		strings.Contains(up, "_DC_COUNT") {
		return QueryKindSystem
	}
	return QueryKindUser
}
