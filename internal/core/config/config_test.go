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
