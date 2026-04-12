package schema_test

import (
	"context"
	"os"
	"testing"

	"github.com/beetio/datacow/internal/core/db"
	"github.com/beetio/datacow/internal/core/schema"
)

func TestLoad_Postgres(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set")
	}
	client, err := db.Connect(dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = client.Close() }()
	if err := client.Ping(context.Background()); err != nil {
		t.Skipf("postgres not reachable: %v", err)
	}

	ctx := context.Background()
	for _, stmt := range []string{
		"DROP TABLE IF EXISTS sc_items",
		"DROP TABLE IF EXISTS sc_categories",
		"CREATE TABLE sc_categories (id SERIAL PRIMARY KEY, label TEXT NOT NULL)",
		`CREATE TABLE sc_items (
			id          SERIAL PRIMARY KEY,
			category_id INT REFERENCES sc_categories(id),
			name        TEXT NOT NULL
		)`,
	} {
		if _, err := client.Query(ctx, stmt); err != nil {
			t.Fatalf("setup: %v", err)
		}
	}
	defer func() {
		client.Query(ctx, "DROP TABLE IF EXISTS sc_items")       //nolint:errcheck
		client.Query(ctx, "DROP TABLE IF EXISTS sc_categories") //nolint:errcheck
	}()

	tables, err := schema.Load(ctx, client)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	found := map[string]*schema.Table{}
	for i := range tables {
		found[tables[i].Name] = &tables[i]
	}

	if _, ok := found["sc_categories"]; !ok {
		t.Error("sc_categories missing from schema")
	}
	items, ok := found["sc_items"]
	if !ok {
		t.Fatal("sc_items missing from schema")
	}
	if len(items.ForeignKeys) != 1 {
		t.Errorf("sc_items FK count: got %d, want 1", len(items.ForeignKeys))
	}
}
