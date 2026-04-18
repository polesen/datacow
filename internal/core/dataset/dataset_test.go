package dataset_test

import (
	"context"
	"os"
	"testing"

	"github.com/beetio/datacow/internal/core/dataset"
	"github.com/beetio/datacow/internal/core/db"
)

// helper: connect to Postgres or skip
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

// helper: connect to MySQL or skip
func myClient(t *testing.T) db.Client {
	t.Helper()
	dsn := os.Getenv("TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("TEST_MYSQL_DSN not set")
	}
	c, err := db.Connect(dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	if err := c.Ping(context.Background()); err != nil {
		_ = c.Close()
		t.Fatalf("mysql not reachable: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

// setupPostgres creates a small test table and returns cleanup func.
func setupPostgres(t *testing.T, client db.Client) {
	t.Helper()
	ctx := context.Background()
	for _, stmt := range []string{
		"DROP TABLE IF EXISTS ds_products",
		`CREATE TABLE ds_products (
			id    SERIAL PRIMARY KEY,
			name  TEXT NOT NULL,
			price NUMERIC NOT NULL,
			tag   TEXT
		)`,
		"INSERT INTO ds_products (name, price, tag) VALUES ('Apple', 1.50, 'fruit')",
		"INSERT INTO ds_products (name, price, tag) VALUES ('Banana', 0.75, 'fruit')",
		"INSERT INTO ds_products (name, price, tag) VALUES ('Carrot', 0.50, 'veggie')",
		"INSERT INTO ds_products (name, price, tag) VALUES ('Daikon', 1.00, 'veggie')",
		"INSERT INTO ds_products (name, price, tag) VALUES ('Elderberry', 3.00, 'fruit')",
	} {
		if _, err := client.Query(ctx, stmt); err != nil {
			t.Fatalf("setup %q: %v", stmt[:20], err)
		}
	}
	t.Cleanup(func() {
		client.Query(ctx, "DROP TABLE IF EXISTS ds_products") //nolint:errcheck
	})
}

// setupMySQL creates the same test table for MySQL.
func setupMySQL(t *testing.T, client db.Client) {
	t.Helper()
	ctx := context.Background()
	for _, stmt := range []string{
		"DROP TABLE IF EXISTS ds_products",
		`CREATE TABLE ds_products (
			id    INT AUTO_INCREMENT PRIMARY KEY,
			name  VARCHAR(255) NOT NULL,
			price DECIMAL(10,2) NOT NULL,
			tag   VARCHAR(255)
		)`,
		"INSERT INTO ds_products (name, price, tag) VALUES ('Apple', 1.50, 'fruit')",
		"INSERT INTO ds_products (name, price, tag) VALUES ('Banana', 0.75, 'fruit')",
		"INSERT INTO ds_products (name, price, tag) VALUES ('Carrot', 0.50, 'veggie')",
		"INSERT INTO ds_products (name, price, tag) VALUES ('Daikon', 1.00, 'veggie')",
		"INSERT INTO ds_products (name, price, tag) VALUES ('Elderberry', 3.00, 'fruit')",
	} {
		if _, err := client.Query(ctx, stmt); err != nil {
			t.Fatalf("setup %q: %v", stmt[:20], err)
		}
	}
	t.Cleanup(func() {
		client.Query(ctx, "DROP TABLE IF EXISTS ds_products") //nolint:errcheck
	})
}

// runResolverTests contains the resolver sub-tests shared between Postgres and MySQL.
func runResolverTests(t *testing.T, client db.Client) {
	t.Helper()
	ctx := context.Background()
	resolver := dataset.NewResolver(client, nil, "")

	t.Run("Resolve_includes_table", func(t *testing.T) {
		datasets, err := resolver.Resolve(ctx)
		if err != nil {
			t.Fatalf("Resolve: %v", err)
		}
		found := false
		for _, d := range datasets {
			if d.Table == "ds_products" {
				found = true
				// auto-discovered dataset name equals table name
				if d.Name != "ds_products" {
					t.Errorf("Name: got %q, want %q", d.Name, "ds_products")
				}
			}
		}
		if !found {
			t.Error("ds_products not found in resolved datasets")
		}
	})
}

// runExecutorTests contains executor sub-tests shared between Postgres and MySQL.
func runExecutorTests(t *testing.T, client db.Client) {
	t.Helper()
	ctx := context.Background()
	ex := dataset.NewExecutor(client)
	ds := dataset.Dataset{Name: "ds_products", Table: "ds_products"}

	t.Run("Query_no_opts", func(t *testing.T) {
		result, err := ex.Query(ctx, ds, dataset.QueryOptions{Page: 1, PageSize: 10})
		if err != nil {
			t.Fatalf("Query: %v", err)
		}
		if result.TotalRows != 5 {
			t.Errorf("TotalRows: got %d, want 5", result.TotalRows)
		}
		if len(result.Rows) != 5 {
			t.Errorf("row count: got %d, want 5", len(result.Rows))
		}
		if result.Page != 1 {
			t.Errorf("Page: got %d, want 1", result.Page)
		}
		if result.PageSize != 10 {
			t.Errorf("PageSize: got %d, want 10", result.PageSize)
		}
		if result.TotalPages != 1 {
			t.Errorf("TotalPages: got %d, want 1", result.TotalPages)
		}
		if len(result.Columns) == 0 {
			t.Error("Columns should not be empty")
		}
	})

	t.Run("Query_pagination", func(t *testing.T) {
		result, err := ex.Query(ctx, ds, dataset.QueryOptions{Page: 1, PageSize: 2})
		if err != nil {
			t.Fatalf("Query page 1: %v", err)
		}
		if result.TotalRows != 5 {
			t.Errorf("TotalRows: got %d, want 5", result.TotalRows)
		}
		if len(result.Rows) != 2 {
			t.Errorf("row count page 1: got %d, want 2", len(result.Rows))
		}
		if result.TotalPages != 3 {
			t.Errorf("TotalPages: got %d, want 3", result.TotalPages)
		}

		result2, err := ex.Query(ctx, ds, dataset.QueryOptions{Page: 3, PageSize: 2})
		if err != nil {
			t.Fatalf("Query page 3: %v", err)
		}
		if len(result2.Rows) != 1 {
			t.Errorf("row count page 3: got %d, want 1", len(result2.Rows))
		}
	})

	t.Run("Query_filter_eq", func(t *testing.T) {
		result, err := ex.Query(ctx, ds, dataset.QueryOptions{
			Page:     1,
			PageSize: 10,
			Filters: []dataset.Filter{
				{Column: "tag", Operator: "=", Value: "fruit"},
			},
		})
		if err != nil {
			t.Fatalf("Query: %v", err)
		}
		if result.TotalRows != 3 {
			t.Errorf("TotalRows: got %d, want 3", result.TotalRows)
		}
	})

	t.Run("Query_filter_like", func(t *testing.T) {
		result, err := ex.Query(ctx, ds, dataset.QueryOptions{
			Page:     1,
			PageSize: 10,
			Filters: []dataset.Filter{
				{Column: "name", Operator: "like", Value: "B%"},
			},
		})
		if err != nil {
			t.Fatalf("Query: %v", err)
		}
		if result.TotalRows != 1 {
			t.Errorf("TotalRows: got %d, want 1", result.TotalRows)
		}
	})

	t.Run("Query_filter_gt", func(t *testing.T) {
		result, err := ex.Query(ctx, ds, dataset.QueryOptions{
			Page:     1,
			PageSize: 10,
			Filters: []dataset.Filter{
				{Column: "price", Operator: ">", Value: 1.0},
			},
		})
		if err != nil {
			t.Fatalf("Query: %v", err)
		}
		if result.TotalRows != 2 { // Apple 1.50, Elderberry 3.00
			t.Errorf("TotalRows: got %d, want 2", result.TotalRows)
		}
	})

	t.Run("Query_filter_lt", func(t *testing.T) {
		result, err := ex.Query(ctx, ds, dataset.QueryOptions{
			Page:     1,
			PageSize: 10,
			Filters: []dataset.Filter{
				{Column: "price", Operator: "<", Value: 1.0},
			},
		})
		if err != nil {
			t.Fatalf("Query: %v", err)
		}
		if result.TotalRows != 2 { // Banana 0.75, Carrot 0.50
			t.Errorf("TotalRows: got %d, want 2", result.TotalRows)
		}
	})

	t.Run("Query_filter_gte", func(t *testing.T) {
		result, err := ex.Query(ctx, ds, dataset.QueryOptions{
			Page:     1,
			PageSize: 10,
			Filters: []dataset.Filter{
				{Column: "price", Operator: ">=", Value: 1.0},
			},
		})
		if err != nil {
			t.Fatalf("Query: %v", err)
		}
		if result.TotalRows != 3 { // Apple 1.50, Daikon 1.00, Elderberry 3.00
			t.Errorf("TotalRows: got %d, want 3", result.TotalRows)
		}
	})

	t.Run("Query_filter_lte", func(t *testing.T) {
		result, err := ex.Query(ctx, ds, dataset.QueryOptions{
			Page:     1,
			PageSize: 10,
			Filters: []dataset.Filter{
				{Column: "price", Operator: "<=", Value: 1.0},
			},
		})
		if err != nil {
			t.Fatalf("Query: %v", err)
		}
		if result.TotalRows != 3 { // Banana 0.75, Carrot 0.50, Daikon 1.00
			t.Errorf("TotalRows: got %d, want 3", result.TotalRows)
		}
	})

	t.Run("Query_sort_asc", func(t *testing.T) {
		result, err := ex.Query(ctx, ds, dataset.QueryOptions{
			Page:     1,
			PageSize: 10,
			Sort:     &dataset.Sort{Column: "name", Desc: false},
		})
		if err != nil {
			t.Fatalf("Query: %v", err)
		}
		if len(result.Rows) < 2 {
			t.Fatal("expected at least 2 rows")
		}
		first := stringVal(result.Rows[0]["name"])
		second := stringVal(result.Rows[1]["name"])
		if first > second {
			t.Errorf("expected ASC order, got %q before %q", first, second)
		}
	})

	t.Run("Query_sort_desc", func(t *testing.T) {
		result, err := ex.Query(ctx, ds, dataset.QueryOptions{
			Page:     1,
			PageSize: 10,
			Sort:     &dataset.Sort{Column: "name", Desc: true},
		})
		if err != nil {
			t.Fatalf("Query: %v", err)
		}
		if len(result.Rows) < 2 {
			t.Fatal("expected at least 2 rows")
		}
		first := stringVal(result.Rows[0]["name"])
		second := stringVal(result.Rows[1]["name"])
		if first < second {
			t.Errorf("expected DESC order, got %q before %q", first, second)
		}
	})

	t.Run("Query_sql_dataset", func(t *testing.T) {
		sqlDS := dataset.Dataset{
			Name: "cheap_products",
			SQL:  "SELECT id, name, price FROM ds_products WHERE price < 1.0",
		}
		result, err := ex.Query(ctx, sqlDS, dataset.QueryOptions{Page: 1, PageSize: 10})
		if err != nil {
			t.Fatalf("Query sql dataset: %v", err)
		}
		if result.TotalRows != 2 {
			t.Errorf("TotalRows: got %d, want 2", result.TotalRows)
		}
	})

	t.Run("Query_invalid_filter_column", func(t *testing.T) {
		_, err := ex.Query(ctx, ds, dataset.QueryOptions{
			Page:     1,
			PageSize: 10,
			Filters: []dataset.Filter{
				{Column: "nonexistent", Operator: "=", Value: "x"},
			},
		})
		if err == nil {
			t.Error("expected error for unknown filter column")
		}
	})

	t.Run("Query_injection_attempt", func(t *testing.T) {
		_, err := ex.Query(ctx, ds, dataset.QueryOptions{
			Page:     1,
			PageSize: 10,
			Filters: []dataset.Filter{
				{Column: "name'; DROP TABLE ds_products; --", Operator: "=", Value: "x"},
			},
		})
		if err == nil {
			t.Error("expected error for injection attempt in column name")
		}
	})
}

// stringVal coerces a row value to string for comparison.
func stringVal(v any) string {
	switch s := v.(type) {
	case string:
		return s
	case []byte:
		return string(s)
	default:
		return ""
	}
}

func TestPostgres_Resolver(t *testing.T) {
	client := pgClient(t)
	setupPostgres(t, client)
	runResolverTests(t, client)
}

func TestPostgres_Executor(t *testing.T) {
	client := pgClient(t)
	setupPostgres(t, client)
	runExecutorTests(t, client)
}

func TestMySQL_Resolver(t *testing.T) {
	client := myClient(t)
	setupMySQL(t, client)
	runResolverTests(t, client)
}

func TestMySQL_Executor(t *testing.T) {
	client := myClient(t)
	setupMySQL(t, client)
	runExecutorTests(t, client)
}
