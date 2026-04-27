package views_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/polesen/datacow/internal/core/config"
	"github.com/polesen/datacow/internal/core/dataset"
	"github.com/polesen/datacow/internal/core/db"
	"github.com/polesen/datacow/internal/core/schema"
	"github.com/polesen/datacow/internal/tui/views"
)

// makeTestCache builds a cache with a few tables and columns.
func makeTestCache() *schema.Cache {
	tables := []schema.Table{
		{
			Name: "users",
			Kind: db.KindTable,
			Columns: []db.Column{
				{Name: "id"},
				{Name: "email"},
			},
		},
		{
			Name: "orders",
			Kind: db.KindTable,
			Columns: []db.Column{
				{Name: "id"},
				{Name: "user_id"},
				{Name: "total"},
			},
		},
	}
	datasets := []dataset.Dataset{
		{Name: "users", Table: "users", Kind: dataset.KindTable},
		{Name: "orders", Table: "orders", Kind: dataset.KindTable},
		{Name: "user_stats", Table: "user_stats", Kind: dataset.KindView},
		{Name: "monthly_orders", SQL: "SELECT 1", Kind: dataset.KindDataset},
	}
	return schema.NewCacheWithData(tables, datasets)
}

func TestGotoModel_NotReady_ShowsLoading(t *testing.T) {
	cache := schema.NewCache() // not loaded
	m := views.NewGotoModel(cache, nil)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	v := m.View()
	if !strings.Contains(v, "Loading schema") {
		t.Errorf("expected 'Loading schema' in view when cache not ready, got:\n%s", v)
	}
}

func TestGotoModel_NotReady_NoPanic(t *testing.T) {
	m := views.NewGotoModel(nil, nil)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	_ = m.View() // should not panic
}

func TestGotoModel_Ready_ShowsAllEntries(t *testing.T) {
	cache := makeTestCache()
	m := views.NewGotoModel(cache, nil)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 40})
	m.Focus() //nolint:errcheck

	v := m.View()
	for _, name := range []string{"users", "orders", "user_stats", "monthly_orders"} {
		if !strings.Contains(v, name) {
			t.Errorf("expected %q in goto view, got:\n%s", name, v)
		}
	}
}

func TestGotoModel_Filtering(t *testing.T) {
	cache := makeTestCache()
	m := views.NewGotoModel(cache, nil)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 40})
	m.Focus() //nolint:errcheck

	// Type "user" — should show users and user_stats but not orders or monthly_orders.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("user")})
	v := m.View()
	if !strings.Contains(v, "users") {
		t.Errorf("expected 'users' after typing 'user', got:\n%s", v)
	}
	if !strings.Contains(v, "user_stats") {
		t.Errorf("expected 'user_stats' after typing 'user', got:\n%s", v)
	}
	if strings.Contains(v, "monthly_orders") {
		t.Errorf("expected 'monthly_orders' to be filtered out, got:\n%s", v)
	}
}

func TestGotoModel_Ranking_ExactPrefixFirst(t *testing.T) {
	datasets := []dataset.Dataset{
		{Name: "something_with_orders_distant", Table: "x", Kind: dataset.KindTable},
		{Name: "orders", Table: "orders", Kind: dataset.KindTable},
	}
	cache := schema.NewCacheWithData(nil, datasets)
	m := views.NewGotoModel(cache, nil)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 40})
	m.Focus() //nolint:errcheck
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("orders")})

	// First result should be "orders" (exact match), not the distant one.
	// Trigger Enter on first result.
	m.Focus() //nolint:errcheck
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("orders")})
	// cursor is at 0 (first result)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected command from Enter")
	}
	msg := cmd()
	sel, ok := msg.(views.GotoSelectedMsg)
	if !ok {
		t.Fatalf("expected GotoSelectedMsg, got %T", msg)
	}
	if sel.Dataset == nil || sel.Dataset.Name != "orders" {
		t.Errorf("expected orders dataset, got %+v", sel.Dataset)
	}
}

func TestGotoModel_CursorMovement(t *testing.T) {
	cache := makeTestCache()
	m := views.NewGotoModel(cache, nil)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 40})
	m.Focus() //nolint:errcheck
	m.Focus() //nolint:errcheck // second focus resets results

	// Move down twice.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})

	// Move up once.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})

	// Move up past the top — should stay at 0.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})

	// vim keys: j = down, k = up
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})

	// The test verifies no panic and that the above key sequences are handled.
}

func TestGotoModel_Enter_EmitsGotoSelectedMsg(t *testing.T) {
	cache := makeTestCache()
	m := views.NewGotoModel(cache, nil)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 40})
	m.Focus() //nolint:errcheck

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected command from Enter")
	}
	msg := cmd()
	if _, ok := msg.(views.GotoSelectedMsg); !ok {
		t.Fatalf("expected GotoSelectedMsg, got %T", msg)
	}
}

func TestGotoModel_Esc_EmitsNoMsg(t *testing.T) {
	// Esc is handled by the App, not the GotoModel.
	// The GotoModel should not emit GotoSelectedMsg on Esc.
	cache := makeTestCache()
	m := views.NewGotoModel(cache, nil)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 40})
	m.Focus() //nolint:errcheck

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd != nil {
		msg := cmd()
		if _, ok := msg.(views.GotoSelectedMsg); ok {
			t.Error("GotoModel should not emit GotoSelectedMsg on Esc")
		}
	}
}

func TestGotoModel_Scroll(t *testing.T) {
	// Build 15 datasets to exceed the 12-visible limit.
	datasets := make([]dataset.Dataset, 15)
	for i := range datasets {
		datasets[i] = dataset.Dataset{
			Name:  strings.Repeat("t", i+1), // t, tt, ttt, …
			Table: strings.Repeat("t", i+1),
			Kind:  dataset.KindTable,
		}
	}
	cache := schema.NewCacheWithData(nil, datasets)
	m := views.NewGotoModel(cache, nil)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 50})
	m.Focus() //nolint:errcheck

	// Navigate down 14 times to reach the last entry.
	for range 14 {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	}
	// Navigate back up to verify scrolling doesn't panic.
	for range 14 {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	}

	v := m.View()
	if v == "" {
		t.Error("expected non-empty view after scroll test")
	}
}

func TestGotoModel_ColumnEntry_NavigatesToParentTable(t *testing.T) {
	cache := makeTestCache()
	m := views.NewGotoModel(cache, nil)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 40})
	m.Focus() //nolint:errcheck

	// Search for "email" to find users.email column entry.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("email")})

	// The first result should be users.email; select it.
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected command from Enter")
	}
	msg := cmd()
	sel, ok := msg.(views.GotoSelectedMsg)
	if !ok {
		t.Fatalf("expected GotoSelectedMsg, got %T", msg)
	}
	if sel.Dataset == nil {
		t.Fatal("expected non-nil Dataset for column entry navigation")
	}
	// Should navigate to the parent table "users".
	if sel.Dataset.Name != "users" {
		t.Errorf("expected users dataset for column entry, got %q", sel.Dataset.Name)
	}
}

func TestGotoModel_DatasourceEntries(t *testing.T) {
	cache := makeTestCache()
	datasources := []config.DatasourceConfig{
		{Name: "production", ConnectionString: "postgres://prod"},
		{Name: "staging", ConnectionString: "postgres://staging"},
	}
	m := views.NewGotoModel(cache, datasources)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 40})
	m.Focus() //nolint:errcheck

	// Datasource entries should appear when query is empty.
	v := m.View()
	if !strings.Contains(v, "production") {
		t.Errorf("expected 'production' datasource in view, got:\n%s", v)
	}
	if !strings.Contains(v, "[datasource]") {
		t.Errorf("expected [datasource] badge in view, got:\n%s", v)
	}
}

func TestGotoModel_DatasourceSelect_EmitsCorrectMsg(t *testing.T) {
	cache := makeTestCache()
	datasources := []config.DatasourceConfig{
		{Name: "production", ConnectionString: "postgres://prod"},
	}
	m := views.NewGotoModel(cache, datasources)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 40})
	m.Focus() //nolint:errcheck

	// First result should be the "production" datasource.
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected command from Enter")
	}
	msg := cmd()
	sel, ok := msg.(views.GotoSelectedMsg)
	if !ok {
		t.Fatalf("expected GotoSelectedMsg, got %T", msg)
	}
	if sel.Datasource != "production" {
		t.Errorf("expected Datasource='production', got %q", sel.Datasource)
	}
	if sel.Dataset != nil {
		t.Error("expected nil Dataset for datasource selection")
	}
}
