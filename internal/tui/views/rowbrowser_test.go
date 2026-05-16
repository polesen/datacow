package views_test

import (
	"strings"
	"testing"

	"github.com/polesen/datacow/internal/core/dataset"
	"github.com/polesen/datacow/internal/core/db"
	"github.com/polesen/datacow/internal/tui/keys"
	"github.com/polesen/datacow/internal/tui/views"
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
	m := views.NewRowBrowserModel(keys.Default(), nil, nil, ds)
	if !m.IsLoading() {
		t.Error("expected loading state initially")
	}
}

func TestRowBrowserModel_RowsLoaded(t *testing.T) {
	ds := dataset.Dataset{Name: "users", Table: "users"}
	m := views.NewRowBrowserModel(keys.Default(), nil, nil, ds)

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
	m := views.NewRowBrowserModel(keys.Default(), nil, nil, ds)
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
	m := views.NewRowBrowserModel(keys.Default(), nil, nil, ds)

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
	m := views.NewRowBrowserModel(keys.Default(), nil, nil, ds)

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
	m := views.NewRowBrowserModel(keys.Default(), nil, nil, ds)

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
	cols := []db.Column{{Name: "id"}, {Name: "name"}, {Name: "email"}}
	rows := []map[string]any{{"id": int64(1), "name": "Alice", "email": "a@b.com"}}
	result := makeResult(1, 1, 2, cols, rows)

	// Narrow window (width=0): only 1 column fits, so cursor and scroll move together.
	t.Run("narrow", func(t *testing.T) {
		m := views.NewRowBrowserModel(keys.Default(), nil, nil, ds)
		m, _ = m.Update(views.RowsLoadedMsg(result))

		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
		if m.ColCursor() != 1 || m.ColOffset() != 1 {
			t.Errorf("after right: cursor=%d offset=%d, want cursor=1 offset=1", m.ColCursor(), m.ColOffset())
		}

		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
		if m.ColCursor() != 0 || m.ColOffset() != 0 {
			t.Errorf("after left: cursor=%d offset=%d, want cursor=0 offset=0", m.ColCursor(), m.ColOffset())
		}

		// Cannot go left past 0.
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
		if m.ColCursor() != 0 || m.ColOffset() != 0 {
			t.Errorf("past left boundary: cursor=%d offset=%d, want cursor=0 offset=0", m.ColCursor(), m.ColOffset())
		}
	})

	// Wide window (all columns visible): cursor moves without scrolling colOffset.
	t.Run("wide", func(t *testing.T) {
		m := views.NewRowBrowserModel(keys.Default(), nil, nil, ds)
		m, _ = m.Update(tea.WindowSizeMsg{Width: 200, Height: 30})
		m, _ = m.Update(views.RowsLoadedMsg(result))

		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
		if m.ColCursor() != 1 || m.ColOffset() != 0 {
			t.Errorf("right in wide: cursor=%d offset=%d, want cursor=1 offset=0", m.ColCursor(), m.ColOffset())
		}

		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
		if m.ColCursor() != 2 || m.ColOffset() != 0 {
			t.Errorf("right again in wide: cursor=%d offset=%d, want cursor=2 offset=0", m.ColCursor(), m.ColOffset())
		}

		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
		if m.ColCursor() != 1 || m.ColOffset() != 0 {
			t.Errorf("left in wide: cursor=%d offset=%d, want cursor=1 offset=0", m.ColCursor(), m.ColOffset())
		}
	})
}

func TestRowBrowserModel_StatusLine(t *testing.T) {
	ds := dataset.Dataset{Name: "users", Table: "users"}
	m := views.NewRowBrowserModel(keys.Default(), nil, nil, ds)

	result := makeResult(2, 5, 230,
		[]db.Column{{Name: "id"}},
		nil,
	)
	m, _ = m.Update(views.RowsLoadedMsg(result))

	sl := m.StatusLine()
	for _, want := range []string{"users", "2", "5", "230"} {
		if !strings.Contains(sl, want) {
			t.Errorf("StatusLine %q missing %q", sl, want)
		}
	}
}

func TestRowBrowserModel_View(t *testing.T) {
	ds := dataset.Dataset{Name: "users", Table: "users"}
	m := views.NewRowBrowserModel(keys.Default(), nil, nil, ds)
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
		if !strings.Contains(v, col) {
			t.Errorf("view missing column header %q", col)
		}
	}
	// Should show row data
	if !strings.Contains(v, "Alice") {
		t.Error("view missing row data 'Alice'")
	}
	// NULL value should appear
	if !strings.Contains(v, "null") {
		t.Error("view missing null indicator")
	}
}

// --- Filter modal tests ---

func TestRowBrowserModel_FilterModal_Open(t *testing.T) {
	ds := dataset.Dataset{Name: "users", Table: "users"}
	m := views.NewRowBrowserModel(keys.Default(), nil, nil, ds)
	result := makeResult(1, 1, 1, []db.Column{{Name: "id"}}, nil)
	m, _ = m.Update(views.RowsLoadedMsg(result))

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if !m.IsFilterModalOpen() {
		t.Error("expected filter modal open after 'q'")
	}
}

func TestRowBrowserModel_FilterModal_Cancel(t *testing.T) {
	ds := dataset.Dataset{Name: "users", Table: "users"}
	m := views.NewRowBrowserModel(keys.Default(), nil, nil, ds)
	result := makeResult(1, 1, 1, []db.Column{{Name: "id"}}, nil)
	m, _ = m.Update(views.RowsLoadedMsg(result))

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.IsFilterModalOpen() {
		t.Error("expected filter modal closed after esc")
	}
}

func TestRowBrowserModel_FilterModal_Apply(t *testing.T) {
	ds := dataset.Dataset{Name: "users", Table: "users"}
	m := views.NewRowBrowserModel(keys.Default(), nil, nil, ds)
	result := makeResult(1, 1, 1, []db.Column{{Name: "id"}}, nil)
	m, _ = m.Update(views.RowsLoadedMsg(result))

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	// Apply with Ctrl+J closes modal
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlJ})
	if m.IsFilterModalOpen() {
		t.Error("expected filter modal closed after Ctrl+J")
	}
}

// --- Local search tests ---

func TestRowBrowserModel_LocalSearch_Open(t *testing.T) {
	ds := dataset.Dataset{Name: "users", Table: "users"}
	m := views.NewRowBrowserModel(keys.Default(), nil, nil, ds)
	result := makeResult(1, 1, 1, []db.Column{{Name: "id"}}, nil)
	m, _ = m.Update(views.RowsLoadedMsg(result))

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	if !m.IsLocalSearchInputActive() {
		t.Error("expected local search input active after '/'")
	}
}

func TestRowBrowserModel_LocalSearch_CloseWithEsc(t *testing.T) {
	ds := dataset.Dataset{Name: "users", Table: "users"}
	m := views.NewRowBrowserModel(keys.Default(), nil, nil, ds)
	result := makeResult(1, 1, 1, []db.Column{{Name: "id"}}, nil)
	m, _ = m.Update(views.RowsLoadedMsg(result))

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.IsLocalSearchInputActive() {
		t.Error("expected local search input closed after esc")
	}
}

func TestRowBrowserModel_LocalSearch_NextPrev(t *testing.T) {
	ds := dataset.Dataset{Name: "users", Table: "users"}
	m := views.NewRowBrowserModel(keys.Default(), nil, nil, ds)
	result := makeResult(1, 1, 3,
		[]db.Column{{Name: "name"}},
		[]map[string]any{
			{"name": "Alice"},
			{"name": "Bob"},
			{"name": "Alice2"},
		},
	)
	m, _ = m.Update(views.RowsLoadedMsg(result))

	// Open search and type "alice"
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	for _, r := range "alice" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// n advances
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	// N goes back
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'N'}})
	// No crash — search stays active
	if m.IsLocalSearchInputActive() {
		t.Error("search input should be closed after Enter")
	}
}

func TestRowBrowserModel_QuickFilter_Open(t *testing.T) {
	ds := dataset.Dataset{Name: "users", Table: "users"}
	m := views.NewRowBrowserModel(keys.Default(), nil, nil, ds)
	result := makeResult(1, 1, 1,
		[]db.Column{{Name: "id"}, {Name: "name"}},
		[]map[string]any{{"id": int64(1), "name": "Alice"}},
	)
	m, _ = m.Update(views.RowsLoadedMsg(result))

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'='}})
	if !m.IsFilterModalOpen() {
		t.Error("expected filter modal open after '='")
	}
}

// --- Sort tests ---

func TestRowBrowserModel_Sort_Cycle(t *testing.T) {
	ds := dataset.Dataset{Name: "users", Table: "users"}
	m := views.NewRowBrowserModel(keys.Default(), nil, nil, ds)
	result := makeResult(1, 1, 2,
		[]db.Column{{Name: "id"}, {Name: "name"}},
		[]map[string]any{{"id": int64(1), "name": "Alice"}},
	)
	m, _ = m.Update(views.RowsLoadedMsg(result))

	// colCursor=0 → sort on "id"
	// First s: ASC
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	s := m.ActiveSort()
	if s == nil || s.Column != "id" || s.Desc {
		t.Errorf("expected sort id ASC, got %v", s)
	}

	// Second s: DESC
	m, _ = m.Update(views.RowsLoadedMsg(result)) // loaded first to unblock
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	s = m.ActiveSort()
	if s == nil || s.Column != "id" || !s.Desc {
		t.Errorf("expected sort id DESC, got %v", s)
	}

	// Third s: clear sort
	m, _ = m.Update(views.RowsLoadedMsg(result))
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	s = m.ActiveSort()
	if s != nil {
		t.Errorf("expected no sort, got %v", s)
	}
}

func TestRowBrowserModel_Sort_ViewIndicator(t *testing.T) {
	ds := dataset.Dataset{Name: "users", Table: "users"}
	m := views.NewRowBrowserModel(keys.Default(), nil, nil, ds)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	result := makeResult(1, 1, 1,
		[]db.Column{{Name: "id"}},
		[]map[string]any{{"id": int64(1)}},
	)
	m, _ = m.Update(views.RowsLoadedMsg(result))

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	v := m.View()
	if !strings.Contains(v, "↑") {
		t.Error("expected ↑ indicator in view after sort ASC")
	}
}

func TestRowBrowserModel_Sort_StatusLine(t *testing.T) {
	ds := dataset.Dataset{Name: "users", Table: "users"}
	m := views.NewRowBrowserModel(keys.Default(), nil, nil, ds)
	result := makeResult(1, 1, 1,
		[]db.Column{{Name: "price"}},
		[]map[string]any{{"price": 9.99}},
	)
	m, _ = m.Update(views.RowsLoadedMsg(result))
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})

	sl := m.StatusLine()
	if !strings.Contains(sl, "price") {
		t.Errorf("StatusLine %q missing sort column name", sl)
	}
}

// --- Export menu test ---

func TestRowBrowserModel_ExportMenu_Open(t *testing.T) {
	ds := dataset.Dataset{Name: "users", Table: "users"}
	m := views.NewRowBrowserModel(keys.Default(), nil, nil, ds)
	result := makeResult(1, 1, 1, []db.Column{{Name: "id"}}, nil)
	m, _ = m.Update(views.RowsLoadedMsg(result))

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	if !m.ExportMenuActive() {
		t.Error("expected export menu active after 'e'")
	}

	// Esc closes it
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.ExportMenuActive() {
		t.Error("expected export menu closed after esc")
	}
}

func TestRowBrowserModel_NeedsBackKey(t *testing.T) {
	ds := dataset.Dataset{Name: "users", Table: "users"}
	m := views.NewRowBrowserModel(keys.Default(), nil, nil, ds)
	result := makeResult(1, 1, 1, []db.Column{{Name: "id"}}, nil)
	m, _ = m.Update(views.RowsLoadedMsg(result))

	if m.NeedsBackKey() {
		t.Error("NeedsBackKey should be false in normal mode")
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if !m.NeedsBackKey() {
		t.Error("NeedsBackKey should be true when filter modal is open")
	}
}

// --- FK drill-down tests ---

func TestRowBrowserModel_RowCursor_Down(t *testing.T) {
	ds := dataset.Dataset{Name: "orders", Table: "orders"}
	m := views.NewRowBrowserModel(keys.Default(), nil, nil, ds)
	result := makeResult(1, 1, 2,
		[]db.Column{{Name: "id"}, {Name: "customer_id"}},
		[]map[string]any{
			{"id": int64(1), "customer_id": int64(100)},
			{"id": int64(2), "customer_id": int64(101)},
		},
	)
	m, _ = m.Update(views.RowsLoadedMsg(result))

	if m.RowCursor() != 0 {
		t.Errorf("expected row cursor 0 initially, got %d", m.RowCursor())
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.RowCursor() != 1 {
		t.Errorf("expected row cursor 1 after down, got %d", m.RowCursor())
	}

	// Cannot go past last row
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.RowCursor() != 1 {
		t.Errorf("expected row cursor still 1 at boundary, got %d", m.RowCursor())
	}
}

func TestRowBrowserModel_RowCursor_Up(t *testing.T) {
	ds := dataset.Dataset{Name: "orders", Table: "orders"}
	m := views.NewRowBrowserModel(keys.Default(), nil, nil, ds)
	result := makeResult(1, 1, 2,
		[]db.Column{{Name: "id"}},
		[]map[string]any{{"id": int64(1)}, {"id": int64(2)}},
	)
	m, _ = m.Update(views.RowsLoadedMsg(result))
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if m.RowCursor() != 0 {
		t.Errorf("expected row cursor 0 after up, got %d", m.RowCursor())
	}

	// Cannot go above 0
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if m.RowCursor() != 0 {
		t.Errorf("expected row cursor still 0 at boundary, got %d", m.RowCursor())
	}
}

func TestRowBrowserModel_FKsLoaded(t *testing.T) {
	ds := dataset.Dataset{Name: "orders", Table: "orders"}
	m := views.NewRowBrowserModel(keys.Default(), nil, nil, ds)

	fks := []db.ForeignKey{
		{Column: "customer_id", ReferencedTable: "customers", ReferencedColumn: "id"},
	}
	m, _ = m.Update(views.FKsLoadedMsg(fks))

	if len(m.ForeignKeys()) != 1 {
		t.Errorf("expected 1 FK, got %d", len(m.ForeignKeys()))
	}
	if m.ForeignKeys()[0].Column != "customer_id" {
		t.Errorf("unexpected FK column: %s", m.ForeignKeys()[0].Column)
	}
}

func TestRowBrowserModel_DrillDown_OnFKCell(t *testing.T) {
	ds := dataset.Dataset{Name: "orders", Table: "orders"}
	m := views.NewRowBrowserModel(keys.Default(), nil, nil, ds)
	result := makeResult(1, 1, 1,
		[]db.Column{{Name: "id"}, {Name: "customer_id"}},
		[]map[string]any{{"id": int64(42), "customer_id": int64(1001)}},
	)
	m, _ = m.Update(views.RowsLoadedMsg(result))
	m, _ = m.Update(views.FKsLoadedMsg([]db.ForeignKey{
		{Column: "customer_id", ReferencedTable: "customers", ReferencedColumn: "id"},
	}))

	// Navigate to customer_id column
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})

	// Press Enter to drill down
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if !m.IsLoading() {
		t.Error("expected loading state after drill-down")
	}
	if m.DrillDepth() != 1 {
		t.Errorf("expected drill depth 1, got %d", m.DrillDepth())
	}
}

func TestRowBrowserModel_DrillDown_OnNonFKCell(t *testing.T) {
	ds := dataset.Dataset{Name: "orders", Table: "orders"}
	m := views.NewRowBrowserModel(keys.Default(), nil, nil, ds)
	result := makeResult(1, 1, 1,
		[]db.Column{{Name: "id"}, {Name: "customer_id"}},
		[]map[string]any{{"id": int64(42), "customer_id": int64(1001)}},
	)
	m, _ = m.Update(views.RowsLoadedMsg(result))
	m, _ = m.Update(views.FKsLoadedMsg([]db.ForeignKey{
		{Column: "customer_id", ReferencedTable: "customers", ReferencedColumn: "id"},
	}))

	// colCursor=0 (id column) — not an FK
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if m.DrillDepth() != 0 {
		t.Errorf("expected drill depth 0, got %d", m.DrillDepth())
	}
	if m.IsLoading() {
		t.Error("should not be loading after Enter on non-FK cell")
	}
}

func TestRowBrowserModel_DrillDown_NullFKCell(t *testing.T) {
	ds := dataset.Dataset{Name: "orders", Table: "orders"}
	m := views.NewRowBrowserModel(keys.Default(), nil, nil, ds)
	result := makeResult(1, 1, 1,
		[]db.Column{{Name: "id"}, {Name: "customer_id"}},
		[]map[string]any{{"id": int64(42), "customer_id": nil}},
	)
	m, _ = m.Update(views.RowsLoadedMsg(result))
	m, _ = m.Update(views.FKsLoadedMsg([]db.ForeignKey{
		{Column: "customer_id", ReferencedTable: "customers", ReferencedColumn: "id"},
	}))

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if m.DrillDepth() != 0 {
		t.Error("should not drill into null FK cell")
	}
}

func TestRowBrowserModel_PopDrillStack(t *testing.T) {
	ds := dataset.Dataset{Name: "orders", Table: "orders"}
	m := views.NewRowBrowserModel(keys.Default(), nil, nil, ds)
	result := makeResult(1, 1, 1,
		[]db.Column{{Name: "id"}, {Name: "customer_id"}},
		[]map[string]any{{"id": int64(42), "customer_id": int64(1001)}},
	)
	m, _ = m.Update(views.RowsLoadedMsg(result))
	m, _ = m.Update(views.FKsLoadedMsg([]db.ForeignKey{
		{Column: "customer_id", ReferencedTable: "customers", ReferencedColumn: "id"},
	}))

	// Drill down
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.DrillDepth() != 1 {
		t.Fatalf("setup: expected drill depth 1, got %d", m.DrillDepth())
	}

	// Press Esc to pop
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})

	if m.DrillDepth() != 0 {
		t.Errorf("expected drill depth 0 after esc, got %d", m.DrillDepth())
	}
	if m.IsLoading() {
		t.Error("should not be loading after pop")
	}
}

func TestRowBrowserModel_NeedsBackKey_WithDrillStack(t *testing.T) {
	ds := dataset.Dataset{Name: "orders", Table: "orders"}
	m := views.NewRowBrowserModel(keys.Default(), nil, nil, ds)
	result := makeResult(1, 1, 1,
		[]db.Column{{Name: "id"}, {Name: "customer_id"}},
		[]map[string]any{{"id": int64(42), "customer_id": int64(1001)}},
	)
	m, _ = m.Update(views.RowsLoadedMsg(result))
	m, _ = m.Update(views.FKsLoadedMsg([]db.ForeignKey{
		{Column: "customer_id", ReferencedTable: "customers", ReferencedColumn: "id"},
	}))

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if !m.NeedsBackKey() {
		t.Error("NeedsBackKey should be true when drill stack is non-empty")
	}
}

func TestRowBrowserModel_DrillStack_Rendering(t *testing.T) {
	ds := dataset.Dataset{Name: "orders", Table: "orders"}
	m := views.NewRowBrowserModel(keys.Default(), nil, nil, ds)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	result := makeResult(1, 1, 1,
		[]db.Column{{Name: "id"}, {Name: "customer_id"}},
		[]map[string]any{{"id": int64(42), "customer_id": int64(1001)}},
	)
	m, _ = m.Update(views.RowsLoadedMsg(result))
	m, _ = m.Update(views.FKsLoadedMsg([]db.ForeignKey{
		{Column: "customer_id", ReferencedTable: "customers", ReferencedColumn: "id"},
	}))

	// Drill down
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// Load child rows
	childResult := makeResult(1, 1, 1,
		[]db.Column{{Name: "id"}, {Name: "name"}},
		[]map[string]any{{"id": int64(1001), "name": "Jane Smith"}},
	)
	m, _ = m.Update(views.RowsLoadedMsg(childResult))

	v := m.View()
	if !strings.Contains(v, "orders") {
		t.Error("view should contain parent table name 'orders'")
	}
	if !strings.Contains(v, "customer_id") {
		t.Error("view should contain FK breadcrumb with column name 'customer_id'")
	}
	if !strings.Contains(v, "Jane Smith") {
		t.Error("view should contain child data 'Jane Smith'")
	}
}

func TestRowBrowserModel_DrillDown_MultiLevel(t *testing.T) {
	ds := dataset.Dataset{Name: "orders", Table: "orders"}
	m := views.NewRowBrowserModel(keys.Default(), nil, nil, ds)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 60})

	result := makeResult(1, 1, 1,
		[]db.Column{{Name: "id"}, {Name: "customer_id"}},
		[]map[string]any{{"id": int64(42), "customer_id": int64(1001)}},
	)
	m, _ = m.Update(views.RowsLoadedMsg(result))
	m, _ = m.Update(views.FKsLoadedMsg([]db.ForeignKey{
		{Column: "customer_id", ReferencedTable: "customers", ReferencedColumn: "id"},
	}))

	// First drill
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	customerResult := makeResult(1, 1, 1,
		[]db.Column{{Name: "id"}, {Name: "address_id"}},
		[]map[string]any{{"id": int64(1001), "address_id": int64(5)}},
	)
	m, _ = m.Update(views.RowsLoadedMsg(customerResult))
	m, _ = m.Update(views.FKsLoadedMsg([]db.ForeignKey{
		{Column: "address_id", ReferencedTable: "addresses", ReferencedColumn: "id"},
	}))

	if m.DrillDepth() != 1 {
		t.Fatalf("expected drill depth 1, got %d", m.DrillDepth())
	}

	// Second drill
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if m.DrillDepth() != 2 {
		t.Errorf("expected drill depth 2, got %d", m.DrillDepth())
	}

	// Pop twice
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.DrillDepth() != 1 {
		t.Errorf("after first pop, expected depth 1, got %d", m.DrillDepth())
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.DrillDepth() != 0 {
		t.Errorf("after second pop, expected depth 0, got %d", m.DrillDepth())
	}
}

func TestRowBrowserModel_DrillDown_CompositeFKGraceful(t *testing.T) {
	// Two FK columns on the same table — should drill on the selected one without crashing.
	ds := dataset.Dataset{Name: "order_items", Table: "order_items"}
	m := views.NewRowBrowserModel(keys.Default(), nil, nil, ds)
	result := makeResult(1, 1, 1,
		[]db.Column{{Name: "order_id"}, {Name: "product_id"}},
		[]map[string]any{{"order_id": int64(1), "product_id": int64(2)}},
	)
	m, _ = m.Update(views.RowsLoadedMsg(result))
	m, _ = m.Update(views.FKsLoadedMsg([]db.ForeignKey{
		{Column: "order_id", ReferencedTable: "orders", ReferencedColumn: "id"},
		{Column: "product_id", ReferencedTable: "products", ReferencedColumn: "id"},
	}))

	// Navigate to product_id column
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if m.DrillDepth() != 1 {
		t.Errorf("expected drill depth 1, got %d", m.DrillDepth())
	}
	if !m.IsLoading() {
		t.Error("expected loading after drill-down into product_id")
	}
}
