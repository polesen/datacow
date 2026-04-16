package views_test

import (
	"testing"

	"github.com/beetio/datacow/internal/core/dataset"
	"github.com/beetio/datacow/internal/core/db"
	"github.com/beetio/datacow/internal/tui/keys"
	"github.com/beetio/datacow/internal/tui/views"
	tea "github.com/charmbracelet/bubbletea"
)

func makeResult(page, totalPages int, totalRows int64, cols []db.Column, rows []map[string]any) *dataset.QueryResult {
	return &dataset.QueryResult{
		Columns:    cols,
		Rows:       rows,
		TotalRows:  totalRows,
		Page:       page,
		PageSize:   50,
		TotalPages: totalPages,
	}
}

func TestRowBrowserModel_StartsLoading(t *testing.T) {
	ds := dataset.Dataset{Name: "users", Table: "users"}
	m := views.NewRowBrowserModel(keys.Default(), nil, ds)
	if !m.IsLoading() {
		t.Error("expected loading state initially")
	}
}

func TestRowBrowserModel_RowsLoaded(t *testing.T) {
	ds := dataset.Dataset{Name: "users", Table: "users"}
	m := views.NewRowBrowserModel(keys.Default(), nil, ds)

	result := makeResult(1, 3, 150,
		[]db.Column{{Name: "id"}, {Name: "name"}},
		[]map[string]any{
			{"id": int64(1), "name": "Alice"},
			{"id": int64(2), "name": "Bob"},
		},
	)
	m, _ = m.Update(views.RowsLoadedMsg(result))

	if m.IsLoading() {
		t.Error("should not be loading after rows loaded")
	}
	if m.Page() != 1 {
		t.Errorf("expected page 1, got %d", m.Page())
	}
	if m.TotalPages() != 3 {
		t.Errorf("expected 3 total pages, got %d", m.TotalPages())
	}
	if m.TotalRows() != 150 {
		t.Errorf("expected 150 total rows, got %d", m.TotalRows())
	}
}

func TestRowBrowserModel_NextPage(t *testing.T) {
	ds := dataset.Dataset{Name: "users", Table: "users"}
	m := views.NewRowBrowserModel(keys.Default(), nil, ds)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})

	result := makeResult(1, 3, 150,
		[]db.Column{{Name: "id"}},
		[]map[string]any{{"id": int64(1)}},
	)
	m, _ = m.Update(views.RowsLoadedMsg(result))

	// Press ]
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{']'}})
	if !m.IsLoading() {
		t.Error("expected loading state after next page key")
	}

	// Simulate page 2 arriving
	result2 := makeResult(2, 3, 150,
		[]db.Column{{Name: "id"}},
		[]map[string]any{{"id": int64(51)}},
	)
	m, _ = m.Update(views.RowsLoadedMsg(result2))
	if m.Page() != 2 {
		t.Errorf("expected page 2, got %d", m.Page())
	}
}

func TestRowBrowserModel_PrevPage(t *testing.T) {
	ds := dataset.Dataset{Name: "users", Table: "users"}
	m := views.NewRowBrowserModel(keys.Default(), nil, ds)

	result := makeResult(2, 3, 150,
		[]db.Column{{Name: "id"}},
		[]map[string]any{{"id": int64(51)}},
	)
	m, _ = m.Update(views.RowsLoadedMsg(result))

	// Press [
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'['}})
	if !m.IsLoading() {
		t.Error("expected loading state after prev page key")
	}
}

func TestRowBrowserModel_NextPageAtLastPage(t *testing.T) {
	ds := dataset.Dataset{Name: "users", Table: "users"}
	m := views.NewRowBrowserModel(keys.Default(), nil, ds)

	result := makeResult(3, 3, 150, []db.Column{{Name: "id"}}, nil)
	m, _ = m.Update(views.RowsLoadedMsg(result))

	// Press ] on last page — should not trigger loading
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{']'}})
	if m.IsLoading() {
		t.Error("should not load when already on last page")
	}
}

func TestRowBrowserModel_PrevPageAtFirstPage(t *testing.T) {
	ds := dataset.Dataset{Name: "users", Table: "users"}
	m := views.NewRowBrowserModel(keys.Default(), nil, ds)

	result := makeResult(1, 3, 150, []db.Column{{Name: "id"}}, nil)
	m, _ = m.Update(views.RowsLoadedMsg(result))

	// Press [ on first page — should not trigger loading
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'['}})
	if m.IsLoading() {
		t.Error("should not load when already on first page")
	}
}

func TestRowBrowserModel_HorizontalScroll(t *testing.T) {
	ds := dataset.Dataset{Name: "users", Table: "users"}
	m := views.NewRowBrowserModel(keys.Default(), nil, ds)

	result := makeResult(1, 1, 2,
		[]db.Column{{Name: "id"}, {Name: "name"}, {Name: "email"}},
		[]map[string]any{{"id": int64(1), "name": "Alice", "email": "a@b.com"}},
	)
	m, _ = m.Update(views.RowsLoadedMsg(result))

	if m.ColOffset() != 0 {
		t.Errorf("expected colOffset 0, got %d", m.ColOffset())
	}

	// Right key scrolls right
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	if m.ColOffset() != 1 {
		t.Errorf("expected colOffset 1 after right, got %d", m.ColOffset())
	}

	// Left key scrolls back
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	if m.ColOffset() != 0 {
		t.Errorf("expected colOffset 0 after left, got %d", m.ColOffset())
	}

	// Cannot scroll left past 0
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	if m.ColOffset() != 0 {
		t.Errorf("colOffset should stay at 0, got %d", m.ColOffset())
	}
}

func TestRowBrowserModel_StatusLine(t *testing.T) {
	ds := dataset.Dataset{Name: "users", Table: "users"}
	m := views.NewRowBrowserModel(keys.Default(), nil, ds)

	result := makeResult(2, 5, 230,
		[]db.Column{{Name: "id"}},
		nil,
	)
	m, _ = m.Update(views.RowsLoadedMsg(result))

	sl := m.StatusLine()
	for _, want := range []string{"users", "2", "5", "230"} {
		found := false
		for i := range sl {
			if i+len(want) <= len(sl) && sl[i:i+len(want)] == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("StatusLine %q missing %q", sl, want)
		}
	}
}

func TestRowBrowserModel_View(t *testing.T) {
	ds := dataset.Dataset{Name: "users", Table: "users"}
	m := views.NewRowBrowserModel(keys.Default(), nil, ds)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})

	result := makeResult(1, 1, 2,
		[]db.Column{{Name: "id"}, {Name: "name"}},
		[]map[string]any{
			{"id": int64(1), "name": "Alice"},
			{"id": int64(2), "name": nil},
		},
	)
	m, _ = m.Update(views.RowsLoadedMsg(result))

	v := m.View()
	if v == "" {
		t.Error("expected non-empty view")
	}
	// Should contain column headers
	for _, col := range []string{"id", "name"} {
		if !containsStr(v, col) {
			t.Errorf("view missing column header %q", col)
		}
	}
	// Should show row data
	if !containsStr(v, "Alice") {
		t.Error("view missing row data 'Alice'")
	}
	// NULL value should appear
	if !containsStr(v, "null") {
		t.Error("view missing null indicator")
	}
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && func() bool {
		for i := 0; i <= len(s)-len(sub); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	}()
}
