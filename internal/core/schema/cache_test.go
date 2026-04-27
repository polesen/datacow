package schema_test

import (
	"context"
	"os"
	"sync"
	"testing"

	"github.com/polesen/datacow/internal/core/dataset"
	"github.com/polesen/datacow/internal/core/db"
	"github.com/polesen/datacow/internal/core/schema"
)

func openPostgres(t *testing.T) db.Client {
	t.Helper()
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set")
	}
	client, err := db.Connect(dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	if err := client.Ping(context.Background()); err != nil {
		t.Skipf("postgres not reachable: %v", err)
	}
	return client
}

func TestCache_NotReadyBeforeLoad(t *testing.T) {
	c := schema.NewCache()
	if c.Ready() {
		t.Error("cache should not be ready before Load")
	}
}

func TestCache_ReadyAfterLoad(t *testing.T) {
	client := openPostgres(t)
	ctx := context.Background()
	setupCacheTables(t, client, ctx)

	resolver := dataset.NewResolver(client, nil, "")
	c := schema.NewCache()
	if err := c.Load(ctx, client, resolver); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !c.Ready() {
		t.Error("cache should be ready after Load")
	}
}

func TestCache_LoadPopulatesEntries(t *testing.T) {
	client := openPostgres(t)
	ctx := context.Background()
	setupCacheTables(t, client, ctx)

	resolver := dataset.NewResolver(client, nil, "")
	c := schema.NewCache()
	if err := c.Load(ctx, client, resolver); err != nil {
		t.Fatalf("Load: %v", err)
	}

	results := c.Search("")
	// Find our test tables in the results
	found := map[string]schema.EntryKind{}
	for _, r := range results {
		found[r.Entry.Name] = r.Entry.Kind
	}
	if found["cache_users"] != schema.EntryKindTable {
		t.Errorf("expected cache_users table entry, got %q", found["cache_users"])
	}
	if found["cache_orders"] != schema.EntryKindTable {
		t.Errorf("expected cache_orders table entry, got %q", found["cache_orders"])
	}

	// Column entries
	if found["cache_users.id"] != schema.EntryKindColumn {
		t.Errorf("expected cache_users.id column entry, got %q", found["cache_users.id"])
	}
	if found["cache_orders.user_id"] != schema.EntryKindColumn {
		t.Errorf("expected cache_orders.user_id column entry, got %q", found["cache_orders.user_id"])
	}
}

func TestCache_SearchEmptyQuery_DefaultOrder(t *testing.T) {
	tables := []schema.Table{
		{Name: "orders", Kind: db.KindTable, Columns: []db.Column{{Name: "id"}}},
	}
	datasets := []dataset.Dataset{
		{Name: "orders", Table: "orders", Kind: dataset.KindTable},
		{Name: "order_stats", Table: "order_stats", Kind: dataset.KindView},
		{Name: "monthly", SQL: "SELECT 1", Kind: dataset.KindDataset},
	}
	c := schema.NewCacheWithData(tables, datasets)
	results := c.Search("")

	// Expect: table, view, dataset, then column
	kindSeq := make([]schema.EntryKind, 0, len(results))
	for _, r := range results {
		kindSeq = append(kindSeq, r.Entry.Kind)
	}

	// Find transition points: tables before views before datasets before columns
	seenView := false
	seenDataset := false
	seenColumn := false
	for _, k := range kindSeq {
		switch k {
		case schema.EntryKindView:
			seenView = true
		case schema.EntryKindDataset:
			if !seenView {
				t.Error("dataset appeared before view in empty-query results")
			}
			seenDataset = true
		case schema.EntryKindColumn:
			if !seenView || !seenDataset {
				t.Error("column appeared before dataset in empty-query results")
			}
			seenColumn = true
		case schema.EntryKindTable:
			if seenView || seenDataset || seenColumn {
				t.Error("table appeared after view/dataset/column in empty-query results")
			}
		}
	}
}

func TestCache_SearchWithQuery(t *testing.T) {
	datasets := []dataset.Dataset{
		{Name: "orders", Table: "orders", Kind: dataset.KindTable},
		{Name: "products", Table: "products", Kind: dataset.KindTable},
		{Name: "order_items", Table: "order_items", Kind: dataset.KindTable},
	}
	c := schema.NewCacheWithData(nil, datasets)

	results := c.Search("ord")
	names := make([]string, len(results))
	for i, r := range results {
		names[i] = r.Entry.Name
	}

	// "orders" and "order_items" should match; "products" should not
	foundOrders := false
	foundOrderItems := false
	for _, n := range names {
		switch n {
		case "orders":
			foundOrders = true
		case "order_items":
			foundOrderItems = true
		case "products":
			t.Error("products should not match query 'ord'")
		}
	}
	if !foundOrders {
		t.Error("orders should match query 'ord'")
	}
	if !foundOrderItems {
		t.Error("order_items should match query 'ord'")
	}
}

func TestCache_SearchBestMatchFirst(t *testing.T) {
	datasets := []dataset.Dataset{
		{Name: "something_orders_distant", Table: "x", Kind: dataset.KindTable},
		{Name: "orders", Table: "orders", Kind: dataset.KindTable},
	}
	c := schema.NewCacheWithData(nil, datasets)

	results := c.Search("orders")
	if len(results) < 2 {
		t.Fatalf("expected at least 2 results, got %d", len(results))
	}
	if results[0].Entry.Name != "orders" {
		t.Errorf("best match should be 'orders', got %q", results[0].Entry.Name)
	}
}

func TestCache_SearchColumnEntry(t *testing.T) {
	tables := []schema.Table{
		{
			Name: "users",
			Kind: db.KindTable,
			Columns: []db.Column{
				{Name: "email"},
				{Name: "id"},
			},
		},
	}
	datasets := []dataset.Dataset{
		{Name: "users", Table: "users", Kind: dataset.KindTable},
	}
	c := schema.NewCacheWithData(tables, datasets)

	for _, query := range []string{"email", "users.em"} {
		results := c.Search(query)
		found := false
		for _, r := range results {
			if r.Entry.Name == "users.email" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("query %q: users.email not found in results", query)
		}
	}
}

func TestCache_Refresh_UpdatesEntries(t *testing.T) {
	client := openPostgres(t)
	ctx := context.Background()

	// Start with first table only
	if _, err := client.Query(ctx, "DROP TABLE IF EXISTS cache_refresh_b"); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if _, err := client.Query(ctx, "DROP TABLE IF EXISTS cache_refresh_a"); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if _, err := client.Query(ctx, "CREATE TABLE cache_refresh_a (id SERIAL PRIMARY KEY)"); err != nil {
		t.Fatalf("setup: %v", err)
	}
	t.Cleanup(func() {
		client.Query(ctx, "DROP TABLE IF EXISTS cache_refresh_b") //nolint:errcheck
		client.Query(ctx, "DROP TABLE IF EXISTS cache_refresh_a") //nolint:errcheck
	})

	resolver := dataset.NewResolver(client, nil, "")
	c := schema.NewCache()
	if err := c.Load(ctx, client, resolver); err != nil {
		t.Fatalf("Load: %v", err)
	}

	// Verify first table exists, second does not
	results := c.Search("")
	found := map[string]bool{}
	for _, r := range results {
		found[r.Entry.Name] = true
	}
	if !found["cache_refresh_a"] {
		t.Error("cache_refresh_a should be in initial results")
	}
	if found["cache_refresh_b"] {
		t.Error("cache_refresh_b should not be in initial results")
	}

	// Add second table and refresh
	if _, err := client.Query(ctx, "CREATE TABLE cache_refresh_b (id SERIAL PRIMARY KEY)"); err != nil {
		t.Fatalf("add table: %v", err)
	}

	if err := c.Refresh(ctx, client, resolver); err != nil {
		t.Fatalf("Refresh: %v", err)
	}

	results = c.Search("")
	found = map[string]bool{}
	for _, r := range results {
		found[r.Entry.Name] = true
	}
	if !found["cache_refresh_b"] {
		t.Error("cache_refresh_b should appear after Refresh")
	}
}

func TestCache_ThreadSafety(t *testing.T) {
	client := openPostgres(t)
	ctx := context.Background()
	setupCacheTables(t, client, ctx)

	resolver := dataset.NewResolver(client, nil, "")
	c := schema.NewCache()
	if err := c.Load(ctx, client, resolver); err != nil {
		t.Fatalf("Load: %v", err)
	}

	var wg sync.WaitGroup
	const readers = 10
	const refreshes = 3

	// Concurrent readers
	for range readers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 20 {
				c.Search("cache")
			}
		}()
	}

	// Concurrent refreshes
	for range refreshes {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = c.Refresh(ctx, client, resolver)
		}()
	}

	wg.Wait()
}

// setupCacheTables creates the shared test tables used across multiple tests.
func setupCacheTables(t *testing.T, client db.Client, ctx context.Context) {
	t.Helper()
	for _, stmt := range []string{
		"DROP TABLE IF EXISTS cache_orders",
		"DROP TABLE IF EXISTS cache_users",
		"CREATE TABLE cache_users (id SERIAL PRIMARY KEY, email TEXT NOT NULL)",
		`CREATE TABLE cache_orders (
			id      SERIAL PRIMARY KEY,
			user_id INT REFERENCES cache_users(id),
			total   NUMERIC NOT NULL
		)`,
	} {
		if _, err := client.Query(ctx, stmt); err != nil {
			t.Fatalf("setup: %v", err)
		}
	}
	t.Cleanup(func() {
		client.Query(ctx, "DROP TABLE IF EXISTS cache_orders") //nolint:errcheck
		client.Query(ctx, "DROP TABLE IF EXISTS cache_users")  //nolint:errcheck
	})
}
