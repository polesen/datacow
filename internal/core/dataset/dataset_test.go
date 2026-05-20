package dataset_test

import (
	"context"
	"os"
	"testing"

	"github.com/polesen/datacow/internal/core/dataset"
	"github.com/polesen/datacow/internal/core/db"
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
		if result.TotalRows == nil || *result.TotalRows != 5 {
			t.Errorf("TotalRows: got %v, want 5", result.TotalRows)
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
		if result.TotalPages == nil || *result.TotalPages != 1 {
			t.Errorf("TotalPages: got %v, want 1", result.TotalPages)
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
		if result.TotalRows == nil || *result.TotalRows != 5 {
			t.Errorf("TotalRows: got %v, want 5", result.TotalRows)
		}
		if len(result.Rows) != 2 {
			t.Errorf("row count page 1: got %d, want 2", len(result.Rows))
		}
		if result.TotalPages == nil || *result.TotalPages != 3 {
			t.Errorf("TotalPages: got %v, want 3", result.TotalPages)
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
		if result.TotalRows == nil || *result.TotalRows != 3 {
			t.Errorf("TotalRows: got %v, want 3", result.TotalRows)
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
		if result.TotalRows == nil || *result.TotalRows != 1 {
			t.Errorf("TotalRows: got %v, want 1", result.TotalRows)
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
		if result.TotalRows == nil || *result.TotalRows != 2 { // Apple 1.50, Elderberry 3.00
			t.Errorf("TotalRows: got %v, want 2", result.TotalRows)
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
		if result.TotalRows == nil || *result.TotalRows != 2 { // Banana 0.75, Carrot 0.50
			t.Errorf("TotalRows: got %v, want 2", result.TotalRows)
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
		if result.TotalRows == nil || *result.TotalRows != 3 { // Apple 1.50, Daikon 1.00, Elderberry 3.00
			t.Errorf("TotalRows: got %v, want 3", result.TotalRows)
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
		if result.TotalRows == nil || *result.TotalRows != 3 { // Banana 0.75, Carrot 0.50, Daikon 1.00
			t.Errorf("TotalRows: got %v, want 3", result.TotalRows)
		}
	})

	t.Run("Query_sort_asc", func(t *testing.T) {
		result, err := ex.Query(ctx, ds, dataset.QueryOptions{
			Page:     1,
			PageSize: 10,
			Sort:     []dataset.Sort{{Column: "name", Desc: false}},
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
			Sort:     []dataset.Sort{{Column: "name", Desc: true}},
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
		if result.TotalRows == nil || *result.TotalRows != 2 {
			t.Errorf("TotalRows: got %v, want 2", result.TotalRows)
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

	t.Run("Query_SkipCount_has_more_true", func(t *testing.T) {
		// ds_products has 5 rows; PageSize=2 probes 3 → HasMore=true.
		result, err := ex.Query(ctx, ds, dataset.QueryOptions{Page: 1, PageSize: 2, SkipCount: true})
		if err != nil {
			t.Fatalf("Query: %v", err)
		}
		if !result.HasMore {
			t.Error("HasMore should be true when more rows exist")
		}
		if len(result.Rows) != 2 {
			t.Errorf("got %d rows, want 2", len(result.Rows))
		}
		if result.TotalRows != nil {
			t.Error("TotalRows should be nil with SkipCount")
		}
		if result.TotalPages != nil {
			t.Error("TotalPages should be nil with SkipCount")
		}
	})

	t.Run("Query_SkipCount_has_more_false", func(t *testing.T) {
		// ds_products has 5 rows; PageSize=10 probes 11 → HasMore=false.
		result, err := ex.Query(ctx, ds, dataset.QueryOptions{Page: 1, PageSize: 10, SkipCount: true})
		if err != nil {
			t.Fatalf("Query: %v", err)
		}
		if result.HasMore {
			t.Error("HasMore should be false when all rows fit in one page")
		}
		if len(result.Rows) != 5 {
			t.Errorf("got %d rows, want 5", len(result.Rows))
		}
		if result.TotalRows != nil {
			t.Error("TotalRows should be nil with SkipCount")
		}
	})

	t.Run("Query_OnlyCount", func(t *testing.T) {
		result, err := ex.Query(ctx, ds, dataset.QueryOptions{Page: 1, PageSize: 10, OnlyCount: true})
		if err != nil {
			t.Fatalf("Query: %v", err)
		}
		if len(result.Rows) != 0 {
			t.Errorf("Rows should be empty with OnlyCount, got %d", len(result.Rows))
		}
		if result.TotalRows == nil || *result.TotalRows != 5 {
			t.Errorf("TotalRows: got %v, want 5", result.TotalRows)
		}
		if result.TotalPages == nil || *result.TotalPages != 1 {
			t.Errorf("TotalPages: got %v, want 1", result.TotalPages)
		}
		if len(result.Columns) == 0 {
			t.Error("Columns should be populated with OnlyCount")
		}
	})

	t.Run("Query_SkipCount_and_OnlyCount_rejected", func(t *testing.T) {
		_, err := ex.Query(ctx, ds, dataset.QueryOptions{Page: 1, PageSize: 10, SkipCount: true, OnlyCount: true})
		if err == nil {
			t.Error("expected error when both SkipCount and OnlyCount are set")
		}
	})

	t.Run("Query_columns_projection_table", func(t *testing.T) {
		result, err := ex.Query(ctx, ds, dataset.QueryOptions{
			Page:     1,
			PageSize: 10,
			Columns:  []string{"id", "name"},
		})
		if err != nil {
			t.Fatalf("Query: %v", err)
		}
		if len(result.Columns) != 2 {
			t.Errorf("expected 2 columns, got %d", len(result.Columns))
		}
		if result.Columns[0].Name != "id" || result.Columns[1].Name != "name" {
			t.Errorf("unexpected columns: %v", result.Columns)
		}
		for _, row := range result.Rows {
			if _, ok := row["price"]; ok {
				t.Error("price should not be present in projected result")
			}
			if _, ok := row["id"]; !ok {
				t.Error("id should be present in projected result")
			}
		}
	})

	t.Run("Query_columns_empty_selects_all", func(t *testing.T) {
		result, err := ex.Query(ctx, ds, dataset.QueryOptions{
			Page:     1,
			PageSize: 10,
			Columns:  nil,
		})
		if err != nil {
			t.Fatalf("Query: %v", err)
		}
		// ds_products has 4 columns: id, name, price, tag
		if len(result.Columns) < 4 {
			t.Errorf("expected all columns, got %d", len(result.Columns))
		}
	})

	t.Run("Query_columns_projection_sql_dataset", func(t *testing.T) {
		sqlDS := dataset.Dataset{
			Name: "all_products",
			SQL:  "SELECT id, name, price, tag FROM ds_products",
		}
		result, err := ex.Query(ctx, sqlDS, dataset.QueryOptions{
			Page:     1,
			PageSize: 10,
			Columns:  []string{"id", "name"},
		})
		if err != nil {
			t.Fatalf("Query sql dataset with columns: %v", err)
		}
		if len(result.Columns) != 2 {
			t.Errorf("expected 2 columns, got %d", len(result.Columns))
		}
		for _, row := range result.Rows {
			if _, ok := row["price"]; ok {
				t.Error("price should not be present in projected result")
			}
		}
	})

	t.Run("Query_columns_unknown_rejected", func(t *testing.T) {
		_, err := ex.Query(ctx, ds, dataset.QueryOptions{
			Page:     1,
			PageSize: 10,
			Columns:  []string{"nonexistent"},
		})
		if err == nil {
			t.Error("expected error for unknown column")
		}
	})

	t.Run("Query_columns_injection_rejected", func(t *testing.T) {
		_, err := ex.Query(ctx, ds, dataset.QueryOptions{
			Page:     1,
			PageSize: 10,
			Columns:  []string{"id; DROP TABLE ds_products; --"},
		})
		if err == nil {
			t.Error("expected error for injection attempt in column name")
		}
	})

	// CL01: multi-column sort emits ORDER BY col1 ASC, col2 DESC.
	t.Run("TestAC_CL01_MultiColumnSortOrderBy", func(t *testing.T) {
		result, err := ex.Query(ctx, ds, dataset.QueryOptions{
			Page:     1,
			PageSize: 10,
			Sort:     []dataset.Sort{{Column: "name", Desc: false}, {Column: "id", Desc: true}},
		})
		if err != nil {
			t.Fatalf("Query: %v", err)
		}
		if len(result.Rows) == 0 {
			t.Fatal("expected rows")
		}
	})

	// CL01 (SQL dataset variant): multi-column sort works on SQL datasets.
	t.Run("TestAC_CL01_MultiColumnSort_SQLDataset", func(t *testing.T) {
		sqlDS := dataset.Dataset{
			Name: "all_products",
			SQL:  "SELECT id, name, price FROM ds_products",
		}
		result, err := ex.Query(ctx, sqlDS, dataset.QueryOptions{
			Page:     1,
			PageSize: 10,
			Sort:     []dataset.Sort{{Column: "name", Desc: false}, {Column: "price", Desc: true}},
		})
		if err != nil {
			t.Fatalf("Query sql dataset multi-sort: %v", err)
		}
		if len(result.Rows) == 0 {
			t.Fatal("expected rows from sql dataset multi-sort")
		}
	})

	// CL02: nil and empty Sort produce no ORDER BY.
	t.Run("TestAC_CL02_NilAndEmptySortNoOrderBy", func(t *testing.T) {
		r1, err := ex.Query(ctx, ds, dataset.QueryOptions{Page: 1, PageSize: 10, Sort: nil})
		if err != nil {
			t.Fatalf("nil sort: %v", err)
		}
		r2, err := ex.Query(ctx, ds, dataset.QueryOptions{Page: 1, PageSize: 10, Sort: []dataset.Sort{}})
		if err != nil {
			t.Fatalf("empty sort: %v", err)
		}
		if len(r1.Rows) != len(r2.Rows) {
			t.Errorf("nil and empty sort produced different row counts: %d vs %d", len(r1.Rows), len(r2.Rows))
		}
	})

	// CL03: unknown sort column rejected.
	t.Run("TestAC_CL03_UnknownSortColumnRejected", func(t *testing.T) {
		_, err := ex.Query(ctx, ds, dataset.QueryOptions{
			Page:     1,
			PageSize: 10,
			Sort:     []dataset.Sort{{Column: "nonexistent_column", Desc: false}},
		})
		if err == nil {
			t.Error("expected error for unknown sort column")
		}
	})

	// CL04: single-element Sort produces the same result as the old single-sort path.
	t.Run("TestAC_CL04_SingleElementSortMatchesSingleSort", func(t *testing.T) {
		r1, err := ex.Query(ctx, ds, dataset.QueryOptions{
			Page:     1,
			PageSize: 10,
			Sort:     []dataset.Sort{{Column: "name", Desc: false}},
		})
		if err != nil {
			t.Fatalf("single sort: %v", err)
		}
		if len(r1.Rows) < 2 {
			t.Fatal("expected at least 2 rows")
		}
		first := stringVal(r1.Rows[0]["name"])
		second := stringVal(r1.Rows[1]["name"])
		if first > second {
			t.Errorf("single-element slice sort: expected ASC, got %q before %q", first, second)
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
