package db

import (
	"context"
	"fmt"
	"testing"
)

// stubClient is a minimal db.Client for testing LoggingClient.
type stubClient struct {
	tables  []TableEntry
	cols    []Column
	fks     []ForeignKey
	indexes []Index
	rows    []map[string]any
}

func (s *stubClient) Ping(_ context.Context) error { return nil }
func (s *stubClient) ListTables(_ context.Context) ([]TableEntry, error) {
	return s.tables, nil
}

func (s *stubClient) Describe(_ context.Context, _ string) ([]Column, error) {
	return s.cols, nil
}

func (s *stubClient) ForeignKeys(_ context.Context, _ string) ([]ForeignKey, error) {
	return s.fks, nil
}

func (s *stubClient) Indexes(_ context.Context, _ string) ([]Index, error) {
	return s.indexes, nil
}

func (s *stubClient) Query(_ context.Context, _ string, _ ...any) ([]map[string]any, error) {
	return s.rows, nil
}
func (s *stubClient) Placeholder(n int) string { return fmt.Sprintf("$%d", n) }
func (s *stubClient) Close() error             { return nil }

func TestLoggingClient_ListTables(t *testing.T) {
	stub := &stubClient{tables: []TableEntry{
		{Name: "orders", Kind: KindTable},
		{Name: "users", Kind: KindTable},
		{Name: "products", Kind: KindView},
	}}
	ql := NewQueryLog()
	lc := NewLoggingClient(stub, ql)

	tables, err := lc.ListTables(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tables) != 3 {
		t.Fatalf("want 3 tables, got %d", len(tables))
	}

	_, history := ql.Snapshot()
	if len(history) != 1 {
		t.Fatalf("want 1 history entry, got %d", len(history))
	}
	e := history[0]
	if e.Label != "list tables" {
		t.Errorf("want label 'list tables', got %q", e.Label)
	}
	if e.Kind != QueryKindSystem {
		t.Errorf("want QueryKindSystem, got %v", e.Kind)
	}
	if e.SQL != "" {
		t.Errorf("want empty SQL, got %q", e.SQL)
	}
	if e.RowCount != 3 {
		t.Errorf("want rowCount=3, got %d", e.RowCount)
	}
}

func TestLoggingClient_Describe(t *testing.T) {
	stub := &stubClient{cols: []Column{{Name: "id"}, {Name: "name"}}}
	ql := NewQueryLog()
	lc := NewLoggingClient(stub, ql)

	_, err := lc.Describe(context.Background(), "orders")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, history := ql.Snapshot()
	if len(history) != 1 {
		t.Fatalf("want 1 history entry, got %d", len(history))
	}
	e := history[0]
	if e.Label != "describe orders" {
		t.Errorf("want label 'describe orders', got %q", e.Label)
	}
	if e.Kind != QueryKindSystem {
		t.Errorf("want QueryKindSystem")
	}
	if e.RowCount != 2 {
		t.Errorf("want rowCount=2, got %d", e.RowCount)
	}
}

func TestLoggingClient_ForeignKeys(t *testing.T) {
	stub := &stubClient{fks: []ForeignKey{{Column: "user_id", ReferencedTable: "users", ReferencedColumn: "id"}}}
	ql := NewQueryLog()
	lc := NewLoggingClient(stub, ql)

	_, err := lc.ForeignKeys(context.Background(), "orders")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, history := ql.Snapshot()
	if len(history) != 1 {
		t.Fatalf("want 1 history entry, got %d", len(history))
	}
	e := history[0]
	if e.Label != "FK orders" {
		t.Errorf("want label 'FK orders', got %q", e.Label)
	}
	if e.Kind != QueryKindSystem {
		t.Errorf("want QueryKindSystem")
	}
}

func TestLoggingClient_Query_User(t *testing.T) {
	stub := &stubClient{rows: []map[string]any{{"id": 1}, {"id": 2}}}
	ql := NewQueryLog()
	lc := NewLoggingClient(stub, ql)

	sql := "SELECT * FROM orders LIMIT 50"
	_, err := lc.Query(context.Background(), sql)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, history := ql.Snapshot()
	if len(history) != 1 {
		t.Fatalf("want 1 history entry, got %d", len(history))
	}
	e := history[0]
	if e.Label != "query" {
		t.Errorf("want label 'query', got %q", e.Label)
	}
	if e.SQL != sql {
		t.Errorf("want SQL=%q, got %q", sql, e.SQL)
	}
	if e.Kind != QueryKindUser {
		t.Errorf("want QueryKindUser")
	}
	if e.RowCount != 2 {
		t.Errorf("want rowCount=2, got %d", e.RowCount)
	}
}

func TestLoggingClient_Query_System(t *testing.T) {
	stub := &stubClient{rows: []map[string]any{{"col": "id"}}}
	ql := NewQueryLog()
	lc := NewLoggingClient(stub, ql)

	sql := "SELECT * FROM (SELECT id FROM orders) AS _dc_schema LIMIT 1"
	_, err := lc.Query(context.Background(), sql)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, history := ql.Snapshot()
	e := history[0]
	if e.Kind != QueryKindSystem {
		t.Errorf("want QueryKindSystem for _DC_SCHEMA query, got %v", e.Kind)
	}
}

func TestKindFromSQL(t *testing.T) {
	tests := []struct {
		sql  string
		want QueryKind
	}{
		{"SELECT * FROM orders", QueryKindUser},
		{"SELECT * FROM information_schema.columns", QueryKindSystem},
		{"SELECT * FROM INFORMATION_SCHEMA.tables", QueryKindSystem},
		{"SELECT nspname FROM pg_catalog.pg_namespace", QueryKindSystem},
		{"SELECT * FROM (SELECT 1) AS _dc_schema LIMIT 1", QueryKindSystem},
		{"SELECT count(*) FROM orders WHERE status = $1", QueryKindUser},
		{"SELECT COUNT(*) AS _dc_count FROM alerts", QueryKindSystem},
		{"SELECT COUNT(*) AS _dc_count FROM orders WHERE active = $1", QueryKindSystem},
	}
	for _, tc := range tests {
		got := kindFromSQL(tc.sql)
		if got != tc.want {
			t.Errorf("kindFromSQL(%q) = %v, want %v", tc.sql, got, tc.want)
		}
	}
}

func TestLoggingClient_PassThrough(t *testing.T) {
	stub := &stubClient{}
	ql := NewQueryLog()
	lc := NewLoggingClient(stub, ql)

	if lc.Placeholder(1) != "$1" {
		t.Error("Placeholder should pass through to inner client")
	}
	if err := lc.Ping(context.Background()); err != nil {
		t.Error("Ping should pass through")
	}
}
