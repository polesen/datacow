package views_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/polesen/datacow/internal/core/dataset"
	"github.com/polesen/datacow/internal/core/db"
	"github.com/polesen/datacow/internal/tui/keys"
	"github.com/polesen/datacow/internal/tui/views"
	tea "github.com/charmbracelet/bubbletea"
)

func TestTableListModel_StartsLoading(t *testing.T) {
	m := views.NewTableListModel(keys.Default(), nil, nil, nil)
	if !m.IsLoading() {
		t.Error("expected loading state initially")
	}
	if m.DatasetCount() != 0 {
		t.Errorf("expected 0 datasets initially, got %d", m.DatasetCount())
	}
}

func TestTableListModel_TablesLoaded(t *testing.T) {
	m := views.NewTableListModel(keys.Default(), nil, nil, nil)
	m, _ = m.Update(views.TablesLoadedMsg([]dataset.Dataset{
		{Name: "users", Table: "users"},
		{Name: "posts", Table: "posts"},
	}))
	if m.IsLoading() {
		t.Error("should not be loading after tables loaded")
	}
	if m.DatasetCount() != 2 {
		t.Errorf("expected 2 datasets, got %d", m.DatasetCount())
	}
	if m.Err() != nil {
		t.Errorf("unexpected error: %v", m.Err())
	}
}

func TestTableListModel_Navigation(t *testing.T) {
	m := views.NewTableListModel(keys.Default(), nil, nil, nil)
	m, _ = m.Update(views.TablesLoadedMsg([]dataset.Dataset{
		{Name: "users", Table: "users"},
		{Name: "posts", Table: "posts"},
		{Name: "comments", Table: "comments"},
	}))

	if m.Cursor() != 0 {
		t.Errorf("expected cursor 0, got %d", m.Cursor())
	}

	// Move down twice
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.Cursor() != 1 {
		t.Errorf("expected cursor 1 after down, got %d", m.Cursor())
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.Cursor() != 2 {
		t.Errorf("expected cursor 2 after down, got %d", m.Cursor())
	}

	// Cannot go past last item
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.Cursor() != 2 {
		t.Errorf("cursor should stay at 2 at end, got %d", m.Cursor())
	}

	// Move up
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if m.Cursor() != 1 {
		t.Errorf("expected cursor 1 after up, got %d", m.Cursor())
	}

	// Cannot go before first item
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if m.Cursor() != 0 {
		t.Errorf("cursor should stay at 0, got %d", m.Cursor())
	}
}

func TestTableListModel_NavigationBlockedWhileLoading(t *testing.T) {
	m := views.NewTableListModel(keys.Default(), nil, nil, nil)
	// Still loading — navigation should have no effect
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.Cursor() != 0 {
		t.Errorf("cursor should stay at 0 while loading, got %d", m.Cursor())
	}
}

func TestTableListModel_Error(t *testing.T) {
	m := views.NewTableListModel(keys.Default(), nil, nil, nil)
	m, _ = m.Update(views.ErrMsg{Err: errors.New("connection refused")})
	if m.IsLoading() {
		t.Error("should not be loading after error")
	}
	if m.Err() == nil {
		t.Error("expected error to be set")
	}
}

func TestTableListModel_SelectedDataset(t *testing.T) {
	m := views.NewTableListModel(keys.Default(), nil, nil, nil)
	if m.SelectedDataset() != nil {
		t.Error("expected nil SelectedDataset when no tables loaded")
	}

	m, _ = m.Update(views.TablesLoadedMsg([]dataset.Dataset{
		{Name: "users", Table: "users"},
		{Name: "posts", Table: "posts"},
	}))

	ds := m.SelectedDataset()
	if ds == nil {
		t.Fatal("expected non-nil SelectedDataset after load")
	}
	if ds.Name != "users" {
		t.Errorf("expected selected dataset 'users', got %q", ds.Name)
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	ds = m.SelectedDataset()
	if ds.Name != "posts" {
		t.Errorf("expected selected dataset 'posts', got %q", ds.Name)
	}
}

func TestTableListModel_View(t *testing.T) {
	m := views.NewTableListModel(keys.Default(), nil, nil, nil)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 20})
	m, _ = m.Update(views.TablesLoadedMsg([]dataset.Dataset{
		{Name: "users", Table: "users"},
	}))
	v := m.View()
	if v == "" {
		t.Error("expected non-empty view")
	}
}

func TestTableListModel_RowCountReceived(t *testing.T) {
	m := views.NewTableListModel(keys.Default(), nil, nil, nil)
	m, _ = m.Update(views.TablesLoadedMsg([]dataset.Dataset{
		{Name: "users", Table: "users"},
	}))
	_, _ = m.Update(views.RowCountMsg{Name: "users", Count: 1234})
	// No crash, count stored
}

// --- Schema tree tests ---------------------------------------------------

func TestTableListModel_KindBadgesInView(t *testing.T) {
	m := views.NewTableListModel(keys.Default(), nil, nil, nil)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 10})
	m, _ = m.Update(views.TablesLoadedMsg([]dataset.Dataset{
		{Name: "users", Table: "users", Kind: dataset.KindTable},
		{Name: "user_stats", Table: "user_stats", Kind: dataset.KindView},
		{Name: "recent", SQL: "SELECT 1", Kind: dataset.KindDataset},
	}))
	v := m.View()
	if strings.Contains(v, "[view]") == false {
		t.Errorf("view should contain [view] badge, got:\n%s", v)
	}
	if strings.Contains(v, "[dataset]") == false {
		t.Errorf("view should contain [dataset] badge, got:\n%s", v)
	}
	// A bare table should not get a badge.
	for ln := range strings.SplitSeq(v, "\n") {
		if strings.Contains(ln, "users") && !strings.Contains(ln, "user_stats") {
			if strings.Contains(ln, "[") {
				t.Errorf("plain table row should have no badge, got: %q", ln)
			}
		}
	}
}

func TestTableListModel_ExpandCollapseTable(t *testing.T) {
	m := views.NewTableListModel(keys.Default(), nil, nil, nil)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 10})
	m, _ = m.Update(views.TablesLoadedMsg([]dataset.Dataset{
		{Name: "users", Table: "users", Kind: dataset.KindTable},
	}))

	if m.FocusedExpanded() {
		t.Fatal("row should not be expanded initially")
	}
	if !m.FocusedExpandable() {
		t.Fatal("table should be expandable")
	}

	// Right expands — client is nil so commands return nil, but state flips.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	if !m.FocusedExpanded() {
		t.Fatal("row should be expanded after right key")
	}

	// Left collapses.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	if m.FocusedExpanded() {
		t.Error("row should be collapsed after left key")
	}
}

func TestTableListModel_YAMLDatasetNotExpandable(t *testing.T) {
	m := views.NewTableListModel(keys.Default(), nil, nil, nil)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 10})
	m, _ = m.Update(views.TablesLoadedMsg([]dataset.Dataset{
		{Name: "recent", SQL: "SELECT 1", Kind: dataset.KindDataset},
	}))
	if m.FocusedExpandable() {
		t.Error("YAML SQL dataset should not be expandable")
	}
	// Right is a no-op.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	if m.FocusedExpanded() {
		t.Error("YAML SQL dataset should remain unexpanded after right")
	}
}

func TestTableListModel_ExpansionLoadedPopulatesTree(t *testing.T) {
	m := views.NewTableListModel(keys.Default(), nil, nil, nil)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 20})
	m, _ = m.Update(views.TablesLoadedMsg([]dataset.Dataset{
		{Name: "users", Table: "users", Kind: dataset.KindTable},
	}))
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})

	m, _ = m.Update(views.ExpansionLoadedMsg{
		Idx:  0,
		Cols: []db.Column{{Name: "id", Type: "bigint", Nullable: false}},
		FKs:  nil,
	})
	m, _ = m.Update(views.IndexesLoadedMsg{
		Idx:     0,
		Indexes: []db.Index{{Name: "users_pkey", Columns: []string{"id"}, Unique: true}},
	})

	v := m.View()
	if !strings.Contains(v, "id") || !strings.Contains(v, "bigint") {
		t.Errorf("expanded view should include column info, got:\n%s", v)
	}
	if !strings.Contains(v, "users_pkey") {
		t.Errorf("expanded view should include index info, got:\n%s", v)
	}
	if !strings.Contains(v, "UNIQUE") {
		t.Errorf("unique flag should render, got:\n%s", v)
	}
}

func TestTableListModel_IndexErrorDoesNotCrash(t *testing.T) {
	m := views.NewTableListModel(keys.Default(), nil, nil, nil)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 20})
	m, _ = m.Update(views.TablesLoadedMsg([]dataset.Dataset{
		{Name: "users", Table: "users", Kind: dataset.KindTable},
	}))
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m, _ = m.Update(views.IndexesLoadedMsg{Idx: 0, Err: errors.New("boom")})
	v := m.View()
	if !strings.Contains(v, "(error)") {
		t.Errorf("expected (error) marker in view, got:\n%s", v)
	}
}

func TestTableListModel_ExpandedRendersSubRows(t *testing.T) {
	m := views.NewTableListModel(keys.Default(), nil, nil, nil)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 30})
	m, _ = m.Update(views.TablesLoadedMsg([]dataset.Dataset{
		{Name: "users", Table: "users", Kind: dataset.KindTable},
		{Name: "orders", Table: "orders", Kind: dataset.KindTable},
	}))
	collapsed := m.View()
	if strings.Contains(collapsed, "Columns") {
		t.Fatalf("collapsed view unexpectedly contains expansion markers: %s", collapsed)
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m, _ = m.Update(views.ExpansionLoadedMsg{
		Idx:  0,
		Cols: []db.Column{{Name: "id", Type: "int"}},
	})
	m, _ = m.Update(views.IndexesLoadedMsg{Idx: 0})
	expanded := m.View()
	for _, marker := range []string{"Columns", "Indexes", "Foreign Keys"} {
		if !strings.Contains(expanded, marker) {
			t.Errorf("expanded view missing %q marker, got:\n%s", marker, expanded)
		}
	}
}
