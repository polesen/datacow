package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/polesen/datacow/internal/core/config"
)

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

func TestLoad_MissingFile(t *testing.T) {
	cfg, err := config.Load("/nonexistent/path/datacow.yaml")
	if err != nil {
		t.Fatalf("expected nil error for missing file, got %v", err)
	}
	if len(cfg.Datasources) != 0 || len(cfg.Datasets) != 0 {
		t.Error("expected empty config for missing file")
	}
}

func TestLoad_ValidFile(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "datacow.yaml", `
datasources:
  - name: production
    connection_string: postgres://user:pass@prod/mydb
  - name: staging
    connection_string: postgres://user:pass@staging/mydb

datasets:
  - name: active_orders
    datasource: production
    sql: SELECT id FROM orders WHERE status = 'active'
  - name: recent_signups
    sql: SELECT id, email FROM users LIMIT 100
  - name: Products
    table: products
`)
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cfg.Datasources) != 2 {
		t.Fatalf("expected 2 datasources, got %d", len(cfg.Datasources))
	}
	if cfg.Datasources[0].Name != "production" {
		t.Errorf("expected datasource[0].name=production, got %q", cfg.Datasources[0].Name)
	}
	if cfg.Datasources[0].ConnectionString != "postgres://user:pass@prod/mydb" {
		t.Errorf("unexpected connection_string: %q", cfg.Datasources[0].ConnectionString)
	}

	if len(cfg.Datasets) != 3 {
		t.Fatalf("expected 3 datasets, got %d", len(cfg.Datasets))
	}

	activeOrders := cfg.Datasets[0]
	if activeOrders.Name != "active_orders" {
		t.Errorf("expected dataset[0].name=active_orders, got %q", activeOrders.Name)
	}
	if activeOrders.Datasource != "production" {
		t.Errorf("expected dataset[0].datasource=production, got %q", activeOrders.Datasource)
	}
	if activeOrders.SQL == "" {
		t.Error("expected dataset[0].sql to be set")
	}

	recentSignups := cfg.Datasets[1]
	if recentSignups.Datasource != "" {
		t.Errorf("expected unscoped dataset, got datasource=%q", recentSignups.Datasource)
	}

	products := cfg.Datasets[2]
	if products.Table != "products" {
		t.Errorf("expected dataset[2].table=products, got %q", products.Table)
	}
}

func TestLoad_MalformedYAML(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "bad.yaml", `{this is: [not valid yaml`)
	_, err := config.Load(path)
	if err == nil {
		t.Error("expected error for malformed YAML")
	}
}

func TestLoad_MissingName(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "datacow.yaml", `
datasets:
  - table: users
`)
	_, err := config.Load(path)
	if err == nil {
		t.Error("expected error when dataset name is missing")
	}
}

func TestLoad_MissingTableAndSQL(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "datacow.yaml", `
datasets:
  - name: broken
`)
	_, err := config.Load(path)
	if err == nil {
		t.Error("expected error when neither table nor sql is set")
	}
}

func TestLoad_BothTableAndSQL(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "datacow.yaml", `
datasets:
  - name: broken
    table: users
    sql: SELECT * FROM users
`)
	_, err := config.Load(path)
	if err == nil {
		t.Error("expected error when both table and sql are set")
	}
}

func TestLoad_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "empty.yaml", "")
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("unexpected error for empty file: %v", err)
	}
	if len(cfg.Datasources) != 0 || len(cfg.Datasets) != 0 {
		t.Error("expected empty config for empty file")
	}
}

func TestDefaultPaths(t *testing.T) {
	paths := config.DefaultPaths()
	if len(paths) < 2 {
		t.Fatalf("expected at least 2 default paths, got %d", len(paths))
	}
	found := false
	for _, p := range paths {
		if filepath.Base(p) == "datacow.yaml" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected a path ending in datacow.yaml in DefaultPaths")
	}
}

// --- CF01–CF06: perspective config acceptance criteria ---

// CF01: Load() parses perspectives — name, columns, filters, and sort array are all populated.
func TestAC_CF01_LoadParsesPerspectives(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "datacow.yaml", `
datasets:
  - name: api_logs
    table: api_logs
    perspectives:
      - name: Failed Calls
        columns: [id, timestamp, result]
        filters:
          - { column: result, operator: "!=", value: 200 }
        sort:
          - { column: timestamp, desc: true }
`)
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Datasets) != 1 {
		t.Fatalf("expected 1 dataset, got %d", len(cfg.Datasets))
	}
	ds := cfg.Datasets[0]
	if len(ds.Perspectives) != 1 {
		t.Fatalf("expected 1 perspective, got %d", len(ds.Perspectives))
	}
	p := ds.Perspectives[0]
	if p.Name != "Failed Calls" {
		t.Errorf("expected name 'Failed Calls', got %q", p.Name)
	}
	if len(p.Columns) != 3 || p.Columns[0] != "id" || p.Columns[2] != "result" {
		t.Errorf("unexpected columns: %v", p.Columns)
	}
	if len(p.Filters) != 1 {
		t.Fatalf("expected 1 filter, got %d", len(p.Filters))
	}
	if p.Filters[0].Column != "result" || p.Filters[0].Operator != "!=" {
		t.Errorf("unexpected filter: %+v", p.Filters[0])
	}
	if len(p.Sort) != 1 {
		t.Fatalf("expected 1 sort entry, got %d", len(p.Sort))
	}
	if p.Sort[0].Column != "timestamp" || !p.Sort[0].Desc {
		t.Errorf("unexpected sort: %+v", p.Sort[0])
	}
}

// CF02: Load() returns an error when an sql-type dataset has perspectives.
func TestAC_CF02_LoadRejectsSQLDatasetWithPerspectives(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "datacow.yaml", `
datasets:
  - name: custom_query
    sql: SELECT id FROM orders
    perspectives:
      - name: Bad Perspective
`)
	_, err := config.Load(path)
	if err == nil {
		t.Error("expected error for sql dataset with perspectives")
	}
}

// CF03: AppendPerspective on a non-existent file creates the file with the perspective.
func TestAC_CF03_AppendPerspectiveCreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "datacow.yaml")

	p := config.PerspectiveConfig{
		Name:    "Failed Calls",
		Columns: []string{"id", "result"},
		Filters: []config.FilterConfig{{Column: "result", Operator: "!=", Value: 200}},
	}
	if err := config.AppendPerspective(path, "production", "api_logs", p); err != nil {
		t.Fatalf("AppendPerspective: %v", err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load after AppendPerspective: %v", err)
	}
	if len(cfg.Datasets) != 1 {
		t.Fatalf("expected 1 dataset, got %d", len(cfg.Datasets))
	}
	ds := cfg.Datasets[0]
	if ds.Datasource != "production" {
		t.Errorf("expected datasource 'production', got %q", ds.Datasource)
	}
	if ds.Table != "api_logs" {
		t.Errorf("expected table 'api_logs', got %q", ds.Table)
	}
	if len(ds.Perspectives) != 1 || ds.Perspectives[0].Name != "Failed Calls" {
		t.Errorf("expected perspective 'Failed Calls', got %v", ds.Perspectives)
	}
}

// CF04: AppendPerspective on existing file with no matching table appends a new dataset entry.
func TestAC_CF04_AppendPerspectiveAddsNewEntry(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "datacow.yaml", `
datasets:
  - name: users
    table: users
`)

	p := config.PerspectiveConfig{Name: "Active"}
	if err := config.AppendPerspective(path, "", "orders", p); err != nil {
		t.Fatalf("AppendPerspective: %v", err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.Datasets) != 2 {
		t.Fatalf("expected 2 datasets after append, got %d", len(cfg.Datasets))
	}
	var found bool
	for _, ds := range cfg.Datasets {
		if ds.Table == "orders" {
			found = true
			if len(ds.Perspectives) != 1 || ds.Perspectives[0].Name != "Active" {
				t.Errorf("unexpected perspectives on orders: %v", ds.Perspectives)
			}
		}
	}
	if !found {
		t.Error("no dataset with table=orders found after append")
	}
}

// CF05: AppendPerspective upsert — same name replaces; different name appends.
// After upsert, Load() returns exactly the expected perspectives list.
func TestAC_CF05_AppendPerspectiveUpsert(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "datacow.yaml")

	first := config.PerspectiveConfig{Name: "Failed Calls", Columns: []string{"id"}}
	if err := config.AppendPerspective(path, "", "api_logs", first); err != nil {
		t.Fatalf("first append: %v", err)
	}

	// Replace "Failed Calls" with updated columns.
	updated := config.PerspectiveConfig{Name: "Failed Calls", Columns: []string{"id", "result"}}
	if err := config.AppendPerspective(path, "", "api_logs", updated); err != nil {
		t.Fatalf("replace append: %v", err)
	}

	// Add a second distinct perspective.
	second := config.PerspectiveConfig{Name: "Recent Errors"}
	if err := config.AppendPerspective(path, "", "api_logs", second); err != nil {
		t.Fatalf("second append: %v", err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.Datasets) != 1 {
		t.Fatalf("expected 1 dataset, got %d", len(cfg.Datasets))
	}
	perspectives := cfg.Datasets[0].Perspectives
	if len(perspectives) != 2 {
		t.Fatalf("expected 2 perspectives after upsert+append, got %d: %v", len(perspectives), perspectives)
	}
	// "Failed Calls" should have updated columns.
	if perspectives[0].Name != "Failed Calls" {
		t.Errorf("expected first perspective 'Failed Calls', got %q", perspectives[0].Name)
	}
	if len(perspectives[0].Columns) != 2 || perspectives[0].Columns[1] != "result" {
		t.Errorf("expected updated columns [id result], got %v", perspectives[0].Columns)
	}
	// Second perspective was appended.
	if perspectives[1].Name != "Recent Errors" {
		t.Errorf("expected second perspective 'Recent Errors', got %q", perspectives[1].Name)
	}
}

// CF06: Save writes atomically — no .tmp file remains; contents parseable by Load().
func TestAC_CF06_SaveIsAtomic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "datacow.yaml")

	cfg := &config.Config{
		Datasets: []config.DatasetConfig{
			{
				Name:  "api_logs",
				Table: "api_logs",
				Perspectives: []config.PerspectiveConfig{
					{Name: "Failed Calls"},
				},
			},
		},
	}
	if err := config.Save(path, cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// No .tmp file should remain.
	if _, err := os.Stat(path + ".tmp"); err == nil {
		t.Error("expected .tmp file to be gone after Save, but it exists")
	}

	// The written file should be parseable.
	loaded, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load after Save: %v", err)
	}
	if len(loaded.Datasets) != 1 || len(loaded.Datasets[0].Perspectives) != 1 {
		t.Errorf("unexpected parsed content: %+v", loaded)
	}
}
