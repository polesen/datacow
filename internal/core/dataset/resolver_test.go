package dataset_test

import (
	"context"
	"testing"

	"github.com/beetio/datacow/internal/core/config"
	"github.com/beetio/datacow/internal/core/dataset"
	"github.com/beetio/datacow/internal/core/db"
)

// stubClient is a minimal db.Client stub for resolver unit tests.
type stubClient struct {
	tables []db.TableEntry
}

func (s *stubClient) Ping(_ context.Context) error                              { return nil }
func (s *stubClient) Close() error                                              { return nil }
func (s *stubClient) ListTables(_ context.Context) ([]db.TableEntry, error)     { return s.tables, nil }
func (s *stubClient) Describe(_ context.Context, _ string) ([]db.Column, error) { return nil, nil }
func (s *stubClient) ForeignKeys(_ context.Context, _ string) ([]db.ForeignKey, error) {
	return nil, nil
}

func (s *stubClient) Indexes(_ context.Context, _ string) ([]db.Index, error) {
	return nil, nil
}

func (s *stubClient) Query(_ context.Context, _ string, _ ...any) ([]map[string]any, error) {
	return nil, nil
}
func (s *stubClient) Placeholder(_ int) string { return "$1" }

// TestResolver_NoConfig verifies auto-discovery with no config datasets.
func TestResolver_NoConfig(t *testing.T) {
	sc := &stubClient{tables: []db.TableEntry{
		{Name: "users", Kind: db.KindTable},
		{Name: "orders", Kind: db.KindView},
	}}
	r := dataset.NewResolver(sc, nil, "")
	datasets, err := r.Resolve(context.Background())
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(datasets) != 2 {
		t.Fatalf("expected 2 datasets, got %d", len(datasets))
	}
	for _, d := range datasets {
		if d.Table == "" {
			t.Errorf("auto-discovered dataset %q should have Table set", d.Name)
		}
	}
	if datasets[0].Kind != dataset.KindTable {
		t.Errorf("users should be KindTable, got %q", datasets[0].Kind)
	}
	if datasets[1].Kind != dataset.KindView {
		t.Errorf("orders should be KindView, got %q", datasets[1].Kind)
	}
}

// TestResolver_YAMLKinds verifies config-defined datasets get the right Kind.
func TestResolver_YAMLKinds(t *testing.T) {
	sc := &stubClient{}
	cfgDatasets := []config.DatasetConfig{
		{Name: "custom", SQL: "SELECT 1"},
		{Name: "ref", Table: "users"},
	}
	r := dataset.NewResolver(sc, cfgDatasets, "")
	datasets, err := r.Resolve(context.Background())
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if datasets[0].Kind != dataset.KindDataset {
		t.Errorf("SQL dataset should be KindDataset, got %q", datasets[0].Kind)
	}
	if datasets[1].Kind != dataset.KindTable {
		t.Errorf("table-ref dataset should be KindTable, got %q", datasets[1].Kind)
	}
}

// TestResolver_ConfigMerged verifies config datasets are appended after auto-discovered.
func TestResolver_ConfigMerged(t *testing.T) {
	sc := &stubClient{tables: []db.TableEntry{{Name: "users", Kind: db.KindTable}}}
	cfgDatasets := []config.DatasetConfig{
		{Name: "recent_signups", SQL: "SELECT id FROM users LIMIT 10"},
		{Name: "Products", Table: "products"},
	}
	r := dataset.NewResolver(sc, cfgDatasets, "")
	datasets, err := r.Resolve(context.Background())
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	// 1 auto + 2 config (both unscoped)
	if len(datasets) != 3 {
		t.Fatalf("expected 3 datasets, got %d", len(datasets))
	}
	if datasets[0].Name != "users" || datasets[0].Table != "users" {
		t.Errorf("first dataset: got %+v", datasets[0])
	}
	if datasets[1].Name != "recent_signups" || datasets[1].SQL == "" {
		t.Errorf("sql dataset: got %+v", datasets[1])
	}
	if datasets[2].Name != "Products" || datasets[2].Table != "products" {
		t.Errorf("table ref dataset: got %+v", datasets[2])
	}
}

// TestResolver_ScopedFiltered verifies scoped datasets only appear for matching datasource.
func TestResolver_ScopedFiltered(t *testing.T) {
	sc := &stubClient{tables: []db.TableEntry{{Name: "orders", Kind: db.KindTable}}}
	cfgDatasets := []config.DatasetConfig{
		{Name: "active_orders", Datasource: "production", SQL: "SELECT id FROM orders"},
		{Name: "all_orders", SQL: "SELECT id FROM orders"},
	}

	t.Run("matching datasource sees scoped and unscoped", func(t *testing.T) {
		r := dataset.NewResolver(sc, cfgDatasets, "production")
		datasets, err := r.Resolve(context.Background())
		if err != nil {
			t.Fatalf("Resolve: %v", err)
		}
		if len(datasets) != 3 {
			t.Fatalf("expected 3 datasets (1 auto + 2 config), got %d", len(datasets))
		}
	})

	t.Run("different datasource only sees unscoped", func(t *testing.T) {
		r := dataset.NewResolver(sc, cfgDatasets, "staging")
		datasets, err := r.Resolve(context.Background())
		if err != nil {
			t.Fatalf("Resolve: %v", err)
		}
		if len(datasets) != 2 {
			t.Fatalf("expected 2 datasets (1 auto + 1 unscoped), got %d", len(datasets))
		}
		for _, d := range datasets {
			if d.Name == "active_orders" {
				t.Error("production-scoped dataset should not appear for staging")
			}
		}
	})

	t.Run("empty datasource name only sees unscoped", func(t *testing.T) {
		r := dataset.NewResolver(sc, cfgDatasets, "")
		datasets, err := r.Resolve(context.Background())
		if err != nil {
			t.Fatalf("Resolve: %v", err)
		}
		if len(datasets) != 2 {
			t.Fatalf("expected 2 datasets (1 auto + 1 unscoped), got %d", len(datasets))
		}
		for _, d := range datasets {
			if d.Name == "active_orders" {
				t.Error("scoped dataset should not appear when no active datasource")
			}
		}
	})
}
