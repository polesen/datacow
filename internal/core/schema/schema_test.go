package schema_test

import (
	"context"
	"os"
	"testing"

	"github.com/polesen/datacow/internal/core/db"
	"github.com/polesen/datacow/internal/core/schema"
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
		"CREATE INDEX sc_categories_label_idx ON sc_categories(label)",
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
		client.Query(ctx, "DROP TABLE IF EXISTS sc_items")      //nolint:errcheck
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

	cats, ok := found["sc_categories"]
	if !ok {
		t.Error("sc_categories missing from schema")
	} else {
		// Indexes should be populated; the explicit label index must be present.
		foundLabelIdx := false
		for _, ix := range cats.Indexes {
			if ix.Name == "sc_categories_label_idx" {
				foundLabelIdx = true
				break
			}
		}
		if !foundLabelIdx {
			t.Errorf("sc_categories_label_idx not found in indexes: %v", cats.Indexes)
		}
	}

	items, ok := found["sc_items"]
	if !ok {
		t.Fatal("sc_items missing from schema")
	}
	if len(items.ForeignKeys) != 1 {
		t.Errorf("sc_items FK count: got %d, want 1", len(items.ForeignKeys))
	}
}
