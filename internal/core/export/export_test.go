package export_test

import (
	"context"
	"encoding/csv"
	"os"
	"testing"

	"github.com/beetio/datacow/internal/core/dataset"
	"github.com/beetio/datacow/internal/core/db"
	"github.com/beetio/datacow/internal/core/export"
)

func pgClient(t *testing.T) db.Client {
	t.Helper()
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set")
	}
	c, err := db.Connect(dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	if err := c.Ping(context.Background()); err != nil {
		_ = c.Close()
		t.Fatalf("postgres not reachable: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func setupTable(t *testing.T, client db.Client) {
	t.Helper()
	ctx := context.Background()
	for _, stmt := range []string{
		"DROP TABLE IF EXISTS exp_items",
		`CREATE TABLE exp_items (
			id   SERIAL PRIMARY KEY,
			name TEXT NOT NULL,
			qty  INT NOT NULL
		)`,
		"INSERT INTO exp_items (name, qty) VALUES ('alpha', 10), ('beta', 20), ('gamma', 30)",
	} {
		if _, err := client.Query(ctx, stmt); err != nil {
			t.Fatalf("setup: %v", err)
		}
	}
	t.Cleanup(func() {
		_, _ = client.Query(context.Background(), "DROP TABLE IF EXISTS exp_items")
	})
}

func tempFile(t *testing.T, pattern string) string {
	t.Helper()
	f, err := os.CreateTemp("", pattern)
	if err != nil {
		t.Fatal(err)
	}
	name := f.Name()
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(name) })
	return name
}

func TestExporter_ExportCSV(t *testing.T) {
	client := pgClient(t)
	setupTable(t, client)

	ex := dataset.NewExecutor(client)
	exp := export.NewExporter(ex)
	ds := dataset.Dataset{Name: "exp_items", Table: "exp_items"}

	path := tempFile(t, "datacow_test_*.csv")

	var progress []int
	err := exp.Export(context.Background(), ds, dataset.QueryOptions{}, export.FormatCSV, path, func(n int) {
		progress = append(progress, n)
	})
	if err != nil {
		t.Fatalf("Export: %v", err)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()

	records, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatalf("read csv: %v", err)
	}

	if len(records) != 4 { // 1 header + 3 data rows
		t.Errorf("expected 4 records, got %d", len(records))
	}

	found := false
	for _, cell := range records[0] {
		if cell == "name" {
			found = true
		}
	}
	if !found {
		t.Errorf("header row missing 'name' column: %v", records[0])
	}

	if len(progress) == 0 {
		t.Error("progress callback never called")
	}
	if progress[len(progress)-1] != 3 {
		t.Errorf("expected final progress 3, got %d", progress[len(progress)-1])
	}
}

func TestExporter_ExportCSV_WithFilter(t *testing.T) {
	client := pgClient(t)
	setupTable(t, client)

	ex := dataset.NewExecutor(client)
	exp := export.NewExporter(ex)
	ds := dataset.Dataset{Name: "exp_items", Table: "exp_items"}

	path := tempFile(t, "datacow_test_*.csv")

	opts := dataset.QueryOptions{
		Filters: []dataset.Filter{{Column: "name", Operator: "=", Value: "alpha"}},
	}
	if err := exp.Export(context.Background(), ds, opts, export.FormatCSV, path, nil); err != nil {
		t.Fatalf("Export with filter: %v", err)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()

	records, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatalf("read csv: %v", err)
	}
	if len(records) != 2 { // 1 header + 1 matching row
		t.Errorf("expected 2 records (header+1 row), got %d", len(records))
	}
}

func TestExporter_ExportExcel(t *testing.T) {
	client := pgClient(t)
	setupTable(t, client)

	ex := dataset.NewExecutor(client)
	exp := export.NewExporter(ex)
	ds := dataset.Dataset{Name: "exp_items", Table: "exp_items"}

	path := tempFile(t, "datacow_test_*.xlsx")

	var finalProgress int
	if err := exp.Export(context.Background(), ds, dataset.QueryOptions{}, export.FormatExcel, path, func(n int) {
		finalProgress = n
	}); err != nil {
		t.Fatalf("Export Excel: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Size() == 0 {
		t.Error("excel file is empty")
	}
	if finalProgress != 3 {
		t.Errorf("expected progress 3, got %d", finalProgress)
	}
}
