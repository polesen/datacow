package views_test

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/polesen/datacow/internal/core/dataset"
	"github.com/polesen/datacow/internal/core/db"
	"github.com/polesen/datacow/internal/core/schema"
	"github.com/polesen/datacow/internal/tui/keys"
	"github.com/polesen/datacow/internal/tui/views"
)

func TestTableListModel_StartsLoading(t *testing.T) {
	m := views.NewTableListModel(keys.Default(), nil, nil, nil, nil)
	if !m.IsLoading() {
		t.Error("expected loading state initially")
	}
	if m.DatasetCount() != 0 {
		t.Errorf("expected 0 datasets initially, got %d", m.DatasetCount())
	}
}

func TestTableListModel_TablesLoaded(t *testing.T) {
	m := views.NewTableListModel(keys.Default(), nil, nil, nil, nil)
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
	m := views.NewTableListModel(keys.Default(), nil, nil, nil, nil)
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
	m := views.NewTableListModel(keys.Default(), nil, nil, nil, nil)
	// Still loading — navigation should have no effect
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.Cursor() != 0 {
		t.Errorf("cursor should stay at 0 while loading, got %d", m.Cursor())
	}
}

func TestTableListModel_Error(t *testing.T) {
	m := views.NewTableListModel(keys.Default(), nil, nil, nil, nil)
	m, _ = m.Update(views.ErrMsg{Err: errors.New("connection refused")})
	if m.IsLoading() {
		t.Error("should not be loading after error")
	}
	if m.Err() == nil {
		t.Error("expected error to be set")
	}
}

func TestTableListModel_SelectedDataset(t *testing.T) {
	m := views.NewTableListModel(keys.Default(), nil, nil, nil, nil)
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
	m := views.NewTableListModel(keys.Default(), nil, nil, nil, nil)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 20})
	m, _ = m.Update(views.TablesLoadedMsg([]dataset.Dataset{
		{Name: "users", Table: "users"},
	}))
	v := m.View()
	if v == "" {
		t.Error("expected non-empty view")
	}
}


// --- Schema tree tests ---------------------------------------------------

func TestTableListModel_KindBadgesInView(t *testing.T) {
	m := views.NewTableListModel(keys.Default(), nil, nil, nil, nil)
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
	m := views.NewTableListModel(keys.Default(), nil, nil, nil, nil)
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
	m := views.NewTableListModel(keys.Default(), nil, nil, nil, nil)
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
	m := views.NewTableListModel(keys.Default(), nil, nil, nil, nil)
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
	m := views.NewTableListModel(keys.Default(), nil, nil, nil, nil)
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
	m := views.NewTableListModel(keys.Default(), nil, nil, nil, nil)
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

func TestTableListModel_SelectByName_Found(t *testing.T) {
	m := views.NewTableListModel(keys.Default(), nil, nil, nil, nil)
	m, _ = m.Update(views.TablesLoadedMsg([]dataset.Dataset{
		{Name: "users", Table: "users"},
		{Name: "orders", Table: "orders"},
		{Name: "products", Table: "products"},
	}))

	m, found := m.SelectByName("orders")
	if !found {
		t.Error("SelectByName('orders') should return true")
	}
	if m.Cursor() != 1 {
		t.Errorf("expected cursor 1 after SelectByName('orders'), got %d", m.Cursor())
	}
}

func TestTableListModel_SelectByName_NotFound(t *testing.T) {
	m := views.NewTableListModel(keys.Default(), nil, nil, nil, nil)
	m, _ = m.Update(views.TablesLoadedMsg([]dataset.Dataset{
		{Name: "users", Table: "users"},
	}))

	_, found := m.SelectByName("nonexistent")
	if found {
		t.Error("SelectByName('nonexistent') should return false")
	}
	if m.Cursor() != 0 {
		t.Errorf("cursor should remain 0 after failed SelectByName, got %d", m.Cursor())
	}
}

// ---- Filter tests --------------------------------------------------------

// loadedTableListModel sets up a model with tables loaded and a given size.
func loadedTableListModel(t *testing.T, cache *schema.Cache, datasets []dataset.Dataset) views.TableListModel {
	t.Helper()
	m := views.NewTableListModel(keys.Default(), nil, nil, nil, cache)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 20})
	m, _ = m.Update(views.TablesLoadedMsg(datasets))
	return m
}

func pressSlash(m views.TableListModel) (views.TableListModel, tea.Cmd) {
	return m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
}

func typeText(m views.TableListModel, text string) (views.TableListModel, tea.Cmd) {
	var cmd tea.Cmd
	for _, r := range text {
		m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	return m, cmd
}

func pressEsc(m views.TableListModel) (views.TableListModel, tea.Cmd) {
	return m.Update(tea.KeyMsg{Type: tea.KeyEsc})
}

func pressEnter(m views.TableListModel) (views.TableListModel, tea.Cmd) {
	return m.Update(tea.KeyMsg{Type: tea.KeyEnter})
}

func TestTableListFilter_IdleListUnchanged(t *testing.T) {
	// Without opening the filter, the view should show all tables unmodified.
	m := loadedTableListModel(t, nil, []dataset.Dataset{
		{Name: "users", Table: "users", Kind: dataset.KindTable},
		{Name: "orders", Table: "orders", Kind: dataset.KindTable},
	})
	v := m.View()
	if !strings.Contains(v, "users") || !strings.Contains(v, "orders") {
		t.Errorf("idle view should show all tables, got:\n%s", v)
	}
	if m.FilterActive() {
		t.Error("filter should not be active in idle state")
	}
	if m.FilterInputActive() {
		t.Error("filter input should not be active in idle state")
	}
}

func TestTableListFilter_SlashOpensInput(t *testing.T) {
	m := loadedTableListModel(t, nil, []dataset.Dataset{
		{Name: "users", Table: "users", Kind: dataset.KindTable},
	})
	m, _ = pressSlash(m)
	if !m.FilterInputActive() {
		t.Error("/ should open the filter input")
	}
	v := m.View()
	// The filter bar (with "/" prompt) should be visible.
	if !strings.Contains(v, "/") {
		t.Errorf("view should contain filter prompt after /, got:\n%s", v)
	}
}

func TestTableListFilter_TypingNarrowsList(t *testing.T) {
	m := loadedTableListModel(t, nil, []dataset.Dataset{
		{Name: "users", Table: "users", Kind: dataset.KindTable},
		{Name: "orders", Table: "orders", Kind: dataset.KindTable},
		{Name: "products", Table: "products", Kind: dataset.KindTable},
	})
	m, _ = pressSlash(m)
	m, _ = typeText(m, "ord")
	v := m.View()
	if strings.Contains(v, "users") {
		t.Errorf("users should be hidden by filter, got:\n%s", v)
	}
	if strings.Contains(v, "products") {
		t.Errorf("products should be hidden by filter, got:\n%s", v)
	}
	if !strings.Contains(v, "orders") {
		t.Errorf("orders should be visible after filter 'ord', got:\n%s", v)
	}
}

func TestTableListFilter_CaseInsensitive(t *testing.T) {
	m := loadedTableListModel(t, nil, []dataset.Dataset{
		{Name: "Users", Table: "Users", Kind: dataset.KindTable},
		{Name: "orders", Table: "orders", Kind: dataset.KindTable},
	})
	m, _ = pressSlash(m)
	m, _ = typeText(m, "users")
	v := m.View()
	if !strings.Contains(v, "Users") {
		t.Errorf("filter should be case-insensitive; Users should match 'users', got:\n%s", v)
	}
	if strings.Contains(v, "orders") {
		t.Errorf("orders should not match 'users', got:\n%s", v)
	}
}

func TestTableListFilter_NoMatchPlaceholder(t *testing.T) {
	m := loadedTableListModel(t, nil, []dataset.Dataset{
		{Name: "users", Table: "users", Kind: dataset.KindTable},
	})
	m, _ = pressSlash(m)
	m, _ = typeText(m, "zzzzz")
	v := m.View()
	if !strings.Contains(v, `No tables match`) {
		t.Errorf("should show no-match placeholder, got:\n%s", v)
	}
	if strings.Contains(v, "users") {
		t.Errorf("hidden tables should not appear, got:\n%s", v)
	}
}

func TestTableListFilter_EscClearsFilterAndRestoresCursor(t *testing.T) {
	m := loadedTableListModel(t, nil, []dataset.Dataset{
		{Name: "users", Table: "users", Kind: dataset.KindTable},
		{Name: "orders", Table: "orders", Kind: dataset.KindTable},
		{Name: "products", Table: "products", Kind: dataset.KindTable},
	})
	// Move to orders (index 1).
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.Cursor() != 1 {
		t.Fatalf("expected cursor at 1, got %d", m.Cursor())
	}
	// Open filter and type something.
	m, _ = pressSlash(m)
	m, _ = typeText(m, "pro") // only products matches
	// Esc should clear and restore cursor to orders.
	m, _ = pressEsc(m)
	if m.FilterActive() {
		t.Error("filter should be cleared after Esc")
	}
	if m.FilterInputActive() {
		t.Error("filter input should be closed after Esc")
	}
	v := m.View()
	if !strings.Contains(v, "users") || !strings.Contains(v, "orders") || !strings.Contains(v, "products") {
		t.Errorf("all tables should be visible after Esc, got:\n%s", v)
	}
	if m.Cursor() != 1 {
		t.Errorf("cursor should be restored to orders (1) after Esc, got %d", m.Cursor())
	}
}

func TestTableListFilter_EnterKeepsFilter(t *testing.T) {
	m := loadedTableListModel(t, nil, []dataset.Dataset{
		{Name: "users", Table: "users", Kind: dataset.KindTable},
		{Name: "orders", Table: "orders", Kind: dataset.KindTable},
	})
	m, _ = pressSlash(m)
	m, _ = typeText(m, "ord")
	m, _ = pressEnter(m)
	if !m.FilterActive() {
		t.Error("filter should remain held after Enter")
	}
	if m.FilterInputActive() {
		t.Error("filter input should be closed after Enter")
	}
	v := m.View()
	if strings.Contains(v, "users") {
		t.Errorf("users should still be hidden after Enter, got:\n%s", v)
	}
	if !strings.Contains(v, "orders") {
		t.Errorf("orders should still be visible after Enter, got:\n%s", v)
	}
}

func TestTableListFilter_FilterStatus(t *testing.T) {
	m := loadedTableListModel(t, nil, []dataset.Dataset{
		{Name: "users", Table: "users", Kind: dataset.KindTable},
		{Name: "orders", Table: "orders", Kind: dataset.KindTable},
		{Name: "products", Table: "products", Kind: dataset.KindTable},
	})
	if m.FilterStatus() != "" {
		t.Error("FilterStatus should be empty when no filter active")
	}
	m, _ = pressSlash(m)
	m, _ = typeText(m, "ord")
	status := m.FilterStatus()
	if !strings.Contains(status, "ord") {
		t.Errorf("FilterStatus should contain query 'ord', got: %q", status)
	}
	if !strings.Contains(status, "1/3") {
		t.Errorf("FilterStatus should contain match count '1/3', got: %q", status)
	}
}

func TestTableListFilter_CursorSnapsWhenCurrentRowDropsOut(t *testing.T) {
	m := loadedTableListModel(t, nil, []dataset.Dataset{
		{Name: "alpha", Table: "alpha", Kind: dataset.KindTable},
		{Name: "beta", Table: "beta", Kind: dataset.KindTable},
		{Name: "gamma", Table: "gamma", Kind: dataset.KindTable},
	})
	// Cursor at gamma (index 2).
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.Cursor() != 2 {
		t.Fatalf("expected cursor at 2, got %d", m.Cursor())
	}
	// Filter to only alpha — gamma is no longer visible.
	m, _ = pressSlash(m)
	m, _ = typeText(m, "alpha")
	if m.Cursor() != 0 {
		t.Errorf("cursor should snap to alpha (0), got %d", m.Cursor())
	}
}

func TestTableListFilter_CacheNotReadyHint(t *testing.T) {
	// When schemaCache is nil (not ready), the filter input footer shows the hint.
	m := loadedTableListModel(t, nil, []dataset.Dataset{
		{Name: "users", Table: "users", Kind: dataset.KindTable},
	})
	m, _ = pressSlash(m)
	v := m.View()
	if !strings.Contains(v, "schema loading") {
		t.Errorf("view should show 'schema loading' hint when cache not ready, got:\n%s", v)
	}
}

func TestTableListFilter_CacheReadyNoHint(t *testing.T) {
	// When cache is ready, no hint should appear.
	tables := []schema.Table{
		{Name: "users", Kind: db.KindTable, Columns: []db.Column{{Name: "id"}}},
	}
	datasets := []dataset.Dataset{
		{Name: "users", Table: "users", Kind: dataset.KindTable},
	}
	cache := schema.NewCacheWithData(tables, datasets)
	m := loadedTableListModel(t, cache, []dataset.Dataset{
		{Name: "users", Table: "users", Kind: dataset.KindTable},
	})
	m, _ = pressSlash(m)
	v := m.View()
	if strings.Contains(v, "schema loading") {
		t.Errorf("view should NOT show 'schema loading' hint when cache is ready, got:\n%s", v)
	}
}

func TestTableListFilter_SubMatchVisible(t *testing.T) {
	// A dataset that matches only via a column name should be visible in the filtered list
	// but must remain collapsed (the user expands manually to see which sub-item matched).
	tables := []schema.Table{
		{
			Name: "users",
			Kind: db.KindTable,
			Columns: []db.Column{
				{Name: "email_address"},
				{Name: "id"},
			},
		},
		{
			Name: "orders",
			Kind: db.KindTable,
			Columns: []db.Column{
				{Name: "total"},
			},
		},
	}
	datasets := []dataset.Dataset{
		{Name: "users", Table: "users", Kind: dataset.KindTable},
		{Name: "orders", Table: "orders", Kind: dataset.KindTable},
	}
	cache := schema.NewCacheWithData(tables, datasets)
	m := loadedTableListModel(t, cache, datasets)

	m, _ = pressSlash(m)
	m, _ = typeText(m, "email")

	v := m.View()
	// orders has no 'email' column — must be hidden.
	if strings.Contains(v, "orders") {
		t.Errorf("orders should be hidden (no column 'email'), got:\n%s", v)
	}
	// users matches via column — must be visible.
	if !strings.Contains(v, "users") {
		t.Errorf("users should be visible (column email_address matches), got:\n%s", v)
	}
	// Must remain collapsed — "Columns" header only appears when expanded.
	if strings.Contains(v, "Columns") {
		t.Errorf("sub-matched dataset should remain collapsed, got:\n%s", v)
	}
}

func TestTableListFilter_NameMatchStaysCollapsed(t *testing.T) {
	// A dataset that matches by name should remain collapsed (not auto-expanded).
	tables := []schema.Table{
		{Name: "users", Kind: db.KindTable, Columns: []db.Column{{Name: "id"}}},
	}
	datasets := []dataset.Dataset{
		{Name: "users", Table: "users", Kind: dataset.KindTable},
	}
	cache := schema.NewCacheWithData(tables, datasets)
	m := loadedTableListModel(t, cache, datasets)

	m, _ = pressSlash(m)
	m, _ = typeText(m, "users")

	v := m.View()
	if !strings.Contains(v, "users") {
		t.Errorf("users should be visible (name match), got:\n%s", v)
	}
	// Should NOT show expanded tree (Columns, Indexes, Foreign Keys).
	if strings.Contains(v, "Columns") {
		t.Errorf("name-match dataset should remain collapsed, got:\n%s", v)
	}
}

func TestTableListFilter_NavigationInFilteredList(t *testing.T) {
	// "apple" and "apricot" both contain "p"; "cherry" does not.
	m := loadedTableListModel(t, nil, []dataset.Dataset{
		{Name: "apple", Table: "apple", Kind: dataset.KindTable},
		{Name: "cherry", Table: "cherry", Kind: dataset.KindTable},
		{Name: "apricot", Table: "apricot", Kind: dataset.KindTable},
	})
	m, _ = pressSlash(m)
	m, _ = typeText(m, "p") // matches apple (0) and apricot (2), skips cherry (1)
	m, _ = pressEnter(m)    // hold filter, close input

	// Cursor should be on first visible (apple at index 0).
	if m.Cursor() != 0 {
		t.Errorf("cursor should be at apple (0), got %d", m.Cursor())
	}
	// Down should move to apricot (index 2), skipping cherry (index 1, hidden).
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.Cursor() != 2 {
		t.Errorf("Down should skip to apricot (2), got %d", m.Cursor())
	}
	// Down again should not move past apricot.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.Cursor() != 2 {
		t.Errorf("cursor should stay at apricot (2) at end, got %d", m.Cursor())
	}
	// Up should go back to apple (0).
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if m.Cursor() != 0 {
		t.Errorf("Up should move to apple (0), got %d", m.Cursor())
	}
}

func TestTableListFilter_ReopenPrefilled(t *testing.T) {
	// After Enter (held filter), pressing / again should pre-fill the input.
	m := loadedTableListModel(t, nil, []dataset.Dataset{
		{Name: "users", Table: "users", Kind: dataset.KindTable},
		{Name: "orders", Table: "orders", Kind: dataset.KindTable},
	})
	m, _ = pressSlash(m)
	m, _ = typeText(m, "ord")
	m, _ = pressEnter(m) // hold filter, close input

	// Re-open: should be pre-filled.
	m, _ = pressSlash(m)
	if !m.FilterInputActive() {
		t.Error("second / should re-open filter input")
	}
	v := m.View()
	if !strings.Contains(v, "ord") {
		t.Errorf("re-opened filter input should be pre-filled with 'ord', got:\n%s", v)
	}
}

func TestTableListFilter_SubMatchDoesNotAutoExpand(t *testing.T) {
	// Datasets visible only because a column name matched should NOT be auto-expanded.
	// Typing a short query that hits many column names must not burst open every tree.
	tables := []schema.Table{
		{Name: "users", Kind: db.KindTable, Columns: []db.Column{{Name: "email_address"}, {Name: "id"}}},
		{Name: "orders", Kind: db.KindTable, Columns: []db.Column{{Name: "customer_email"}}},
	}
	datasets := []dataset.Dataset{
		{Name: "users", Table: "users", Kind: dataset.KindTable},
		{Name: "orders", Table: "orders", Kind: dataset.KindTable},
	}
	cache := schema.NewCacheWithData(tables, datasets)
	m := loadedTableListModel(t, cache, datasets)

	m, _ = pressSlash(m)
	m, _ = typeText(m, "email") // matches via column sub-match only, not dataset name

	v := m.View()
	if !strings.Contains(v, "users") {
		t.Errorf("users should be visible (column sub-match), got:\n%s", v)
	}
	if !strings.Contains(v, "orders") {
		t.Errorf("orders should be visible (column sub-match), got:\n%s", v)
	}
	// Tree must stay collapsed — "Columns" header only appears when expanded.
	if strings.Contains(v, "Columns") {
		t.Errorf("sub-matched datasets should NOT be auto-expanded, got:\n%s", v)
	}
}

// ---- Filter bar visibility tests -------------------------------------------

func TestTableListFilter_HeldFilterFooterVisible(t *testing.T) {
	// After Enter (held filter), the pane should show a footer with the query.
	m := loadedTableListModel(t, nil, []dataset.Dataset{
		{Name: "users", Table: "users", Kind: dataset.KindTable},
		{Name: "orders", Table: "orders", Kind: dataset.KindTable},
	})
	m, _ = pressSlash(m)
	m, _ = typeText(m, "ord")
	m, _ = pressEnter(m)

	if m.FilterInputActive() {
		t.Fatal("filter input should be closed after Enter")
	}
	if !m.FilterActive() {
		t.Fatal("filter should be held after Enter")
	}

	v := m.View()
	// Footer must contain the query and the match/total count.
	if !strings.Contains(v, `"ord"`) {
		t.Errorf("held-filter footer must show query, got:\n%s", v)
	}
	if !strings.Contains(v, "1/2") {
		t.Errorf("held-filter footer must show match count, got:\n%s", v)
	}
	// Footer must contain edit and clear hints.
	if !strings.Contains(v, "edit") {
		t.Errorf("held-filter footer must contain 'edit' hint, got:\n%s", v)
	}
	if !strings.Contains(v, "clear") {
		t.Errorf("held-filter footer must contain 'clear' hint, got:\n%s", v)
	}
}

func TestTableListFilter_FooterAbsentWhenNoFilter(t *testing.T) {
	// When no filter is active the footer line must not be rendered.
	m := loadedTableListModel(t, nil, []dataset.Dataset{
		{Name: "users", Table: "users", Kind: dataset.KindTable},
	})
	v := m.View()
	// The held-filter marker strings must not appear.
	if strings.Contains(v, "esc clear") {
		t.Errorf("footer should be absent when no filter active, got:\n%s", v)
	}
}

func TestTableListFilter_InputOpenFooterVisible(t *testing.T) {
	// When the filter input is open, the footer must contain the text input prompt.
	m := loadedTableListModel(t, nil, []dataset.Dataset{
		{Name: "users", Table: "users", Kind: dataset.KindTable},
	})
	m, _ = pressSlash(m)
	v := m.View()
	if !strings.Contains(v, "/") {
		t.Errorf("filter input footer should be visible when input is open, got:\n%s", v)
	}
	// The held-filter hints should not appear while the input is open.
	if strings.Contains(v, "esc clear") {
		t.Errorf("held-filter hints should not appear while input is open, got:\n%s", v)
	}
}

func TestTableListFilter_FlashToggle(t *testing.T) {
	// Pressing Enter sets filterFlashing; receiving filterFlashExpiredMsg clears it.
	m := loadedTableListModel(t, nil, []dataset.Dataset{
		{Name: "users", Table: "users", Kind: dataset.KindTable},
	})
	m, _ = pressSlash(m)
	m, _ = typeText(m, "us")
	m, _ = pressEnter(m)

	// After Enter the flash should be active — the view returns regardless,
	// but we verify that the flash expires correctly.
	m, _ = m.Update(views.FilterFlashExpiredMsgForTest())
	v := m.View()
	// After expiry the footer is still present (held filter) but not flashing.
	if !strings.Contains(v, `"us"`) {
		t.Errorf("held-filter footer should still be visible after flash expires, got:\n%s", v)
	}
}

func TestTableListFilter_OnFocusGainedWithFilter(t *testing.T) {
	// OnFocusGained with an active filter should set flashing and return a cmd.
	m := loadedTableListModel(t, nil, []dataset.Dataset{
		{Name: "users", Table: "users", Kind: dataset.KindTable},
	})
	m, _ = pressSlash(m)
	m, _ = typeText(m, "us")
	m, _ = pressEnter(m)

	m2, cmd := m.OnFocusGained()
	if cmd == nil {
		t.Error("OnFocusGained with active filter should return a tick cmd")
	}
	// Sending the flash-expired message should clear the flash.
	m2, _ = m2.Update(views.FilterFlashExpiredMsgForTest())
	v := m2.View()
	if !strings.Contains(v, `"us"`) {
		t.Errorf("footer should still be visible after flash expires, got:\n%s", v)
	}
}

func TestTableListFilter_OnFocusGainedNoFilter(t *testing.T) {
	// OnFocusGained with no active filter should return nil cmd.
	m := loadedTableListModel(t, nil, []dataset.Dataset{
		{Name: "users", Table: "users", Kind: dataset.KindTable},
	})
	_, cmd := m.OnFocusGained()
	if cmd != nil {
		t.Error("OnFocusGained without active filter should return nil cmd")
	}
}

func TestTableListFilter_PersistsAfterOnFocusGained(t *testing.T) {
	// OnFocusGained is what app.go calls when returning focus to the tables pane.
	// It must NOT clear the filter — only Esc should do that.
	m := loadedTableListModel(t, nil, []dataset.Dataset{
		{Name: "orders", Table: "orders", Kind: dataset.KindTable},
		{Name: "users", Table: "users", Kind: dataset.KindTable},
	})
	m, _ = pressSlash(m)
	m, _ = typeText(m, "ord")
	m, _ = pressEnter(m) // hold filter

	m, _ = m.OnFocusGained()

	if !m.FilterActive() {
		t.Error("filter must persist after OnFocusGained — ClearFilter must not be called on focus switch")
	}
	v := m.View()
	if !strings.Contains(v, `"ord"`) {
		t.Errorf("held-filter bar must still be visible after OnFocusGained, got:\n%s", v)
	}
}

func TestTableListFilter_NavigationWhileInputOpen(t *testing.T) {
	// Up/Down arrow keys should navigate the filtered list even while the filter
	// input is still open, without requiring the user to press Enter first.
	m := loadedTableListModel(t, nil, []dataset.Dataset{
		{Name: "apple", Table: "apple", Kind: dataset.KindTable},
		{Name: "cherry", Table: "cherry", Kind: dataset.KindTable},
		{Name: "apricot", Table: "apricot", Kind: dataset.KindTable},
	})
	m, _ = pressSlash(m)
	m, _ = typeText(m, "p") // matches apple (0) and apricot (2), not cherry (1)

	if !m.FilterInputActive() {
		t.Fatal("filter input should still be open")
	}
	if m.Cursor() != 0 {
		t.Fatalf("cursor should be at apple (0) after filter, got %d", m.Cursor())
	}

	// Down while input open should jump to apricot (2), skipping hidden cherry (1).
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.Cursor() != 2 {
		t.Errorf("Down while filter open should move to apricot (2), got %d", m.Cursor())
	}
	if !m.FilterInputActive() {
		t.Error("filter input should remain open after Down navigation")
	}

	// Up while input open should return to apple (0).
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if m.Cursor() != 0 {
		t.Errorf("Up while filter open should return to apple (0), got %d", m.Cursor())
	}
	if !m.FilterInputActive() {
		t.Error("filter input should remain open after Up navigation")
	}
}

func TestTableListFilter_YAMLDatasetNameOnly(t *testing.T) {
	// YAML SQL datasets (KindDataset) match on name only, not schema.
	cache := schema.NewCacheWithData(nil, []dataset.Dataset{
		{Name: "monthly_report", SQL: "SELECT 1", Kind: dataset.KindDataset},
	})
	m := loadedTableListModel(t, cache, []dataset.Dataset{
		{Name: "monthly_report", SQL: "SELECT 1", Kind: dataset.KindDataset},
		{Name: "users", Table: "users", Kind: dataset.KindTable},
	})
	m, _ = pressSlash(m)
	m, _ = typeText(m, "monthly")
	v := m.View()
	if !strings.Contains(v, "monthly_report") {
		t.Errorf("YAML dataset should match on name, got:\n%s", v)
	}
	if strings.Contains(v, "users") {
		t.Errorf("users should be hidden, got:\n%s", v)
	}
}
