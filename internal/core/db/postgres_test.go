package db_test

import (
	"context"
	"os"
	"testing"

	"github.com/polesen/datacow/internal/core/db"
)

func postgresClient(t *testing.T) db.Client {
	t.Helper()
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set")
	}
	client, err := db.Connect(dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	if err := client.Ping(context.Background()); err != nil {
		_ = client.Close()
		t.Fatalf("postgres not reachable: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func TestPostgres_Ping(t *testing.T) {
	client := postgresClient(t)
	if err := client.Ping(context.Background()); err != nil {
		t.Fatalf("Ping: %v", err)
	}
}

func TestPostgres_SchemaIntrospection(t *testing.T) {
	client := postgresClient(t)
	ctx := context.Background()

	for _, stmt := range []string{
		"DROP TABLE IF EXISTS dc_orders",
		"DROP TABLE IF EXISTS dc_customers",
		`CREATE TABLE dc_customers (
			id   SERIAL PRIMARY KEY,
			name TEXT NOT NULL,
			email TEXT
		)`,
		`CREATE TABLE dc_orders (
			id          SERIAL PRIMARY KEY,
			customer_id INT NOT NULL REFERENCES dc_customers(id),
			amount      NUMERIC NOT NULL
		)`,
	} {
		if _, err := queryExec(ctx, client, stmt); err != nil {
			t.Fatalf("setup %q: %v", stmt[:20], err)
		}
	}
	t.Cleanup(func() {
		queryExec(ctx, client, "DROP TABLE IF EXISTS dc_orders")    //nolint:errcheck
		queryExec(ctx, client, "DROP TABLE IF EXISTS dc_customers") //nolint:errcheck
	})

	t.Run("ListTables", func(t *testing.T) {
		entries, err := client.ListTables(ctx)
		if err != nil {
			t.Fatalf("ListTables: %v", err)
		}
		names := tableEntryNames(entries)
		assertContains(t, names, "dc_customers")
		assertContains(t, names, "dc_orders")
		got := findTableEntry(entries, "dc_customers")
		if got == nil || got.Kind != db.KindTable {
			t.Errorf("dc_customers: got %+v, want KindTable", got)
		}
	})

	t.Run("ListTables_View", func(t *testing.T) {
		if _, err := queryExec(ctx, client, "DROP VIEW IF EXISTS dc_active_customers"); err != nil {
			t.Fatalf("drop view: %v", err)
		}
		if _, err := queryExec(ctx, client, "CREATE VIEW dc_active_customers AS SELECT * FROM dc_customers"); err != nil {
			t.Fatalf("create view: %v", err)
		}
		t.Cleanup(func() {
			queryExec(ctx, client, "DROP VIEW IF EXISTS dc_active_customers") //nolint:errcheck
		})
		entries, err := client.ListTables(ctx)
		if err != nil {
			t.Fatalf("ListTables: %v", err)
		}
		got := findTableEntry(entries, "dc_active_customers")
		if got == nil || got.Kind != db.KindView {
			t.Errorf("dc_active_customers: got %+v, want KindView", got)
		}
	})

	t.Run("Indexes", func(t *testing.T) {
		if _, err := queryExec(ctx, client, "CREATE UNIQUE INDEX dc_customers_email_idx ON dc_customers(email)"); err != nil {
			t.Fatalf("create index: %v", err)
		}
		t.Cleanup(func() {
			queryExec(ctx, client, "DROP INDEX IF EXISTS dc_customers_email_idx") //nolint:errcheck
		})
		idx, err := client.Indexes(ctx, "dc_customers")
		if err != nil {
			t.Fatalf("Indexes: %v", err)
		}
		var found *db.Index
		for i := range idx {
			if idx[i].Name == "dc_customers_email_idx" {
				found = &idx[i]
			}
		}
		if found == nil {
			t.Fatalf("dc_customers_email_idx not in %+v", idx)
		}
		if !found.Unique {
			t.Error("expected unique index")
		}
		if len(found.Columns) != 1 || found.Columns[0] != "email" {
			t.Errorf("columns: got %v, want [email]", found.Columns)
		}
	})

	t.Run("Describe", func(t *testing.T) {
		cols, err := client.Describe(ctx, "dc_customers")
		if err != nil {
			t.Fatalf("Describe: %v", err)
		}
		if len(cols) != 3 {
			t.Fatalf("expected 3 columns, got %d", len(cols))
		}
		assertColumn(t, cols[0], "id", false)
		assertColumn(t, cols[1], "name", false)
		assertColumn(t, cols[2], "email", true)
	})

	t.Run("ForeignKeys", func(t *testing.T) {
		fks, err := client.ForeignKeys(ctx, "dc_orders")
		if err != nil {
			t.Fatalf("ForeignKeys: %v", err)
		}
		if len(fks) != 1 {
			t.Fatalf("expected 1 FK, got %d", len(fks))
		}
		if fks[0].Column != "customer_id" {
			t.Errorf("FK column: got %q, want %q", fks[0].Column, "customer_id")
		}
		if fks[0].ReferencedTable != "dc_customers" {
			t.Errorf("FK referenced table: got %q, want %q", fks[0].ReferencedTable, "dc_customers")
		}
		if fks[0].ReferencedColumn != "id" {
			t.Errorf("FK referenced column: got %q, want %q", fks[0].ReferencedColumn, "id")
		}
	})

	t.Run("Query", func(t *testing.T) {
		if _, err := queryExec(ctx, client, "INSERT INTO dc_customers (name, email) VALUES ('Alice', 'alice@example.com')"); err != nil {
			t.Fatalf("insert: %v", err)
		}
		rows, err := client.Query(ctx, "SELECT name, email FROM dc_customers WHERE name = $1", "Alice")
		if err != nil {
			t.Fatalf("Query: %v", err)
		}
		if len(rows) != 1 {
			t.Fatalf("expected 1 row, got %d", len(rows))
		}
		if rows[0]["name"] != "Alice" {
			t.Errorf("name: got %v, want Alice", rows[0]["name"])
		}
	})
}
