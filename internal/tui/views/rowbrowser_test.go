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

// makeResult builds a QueryResult for tests. HasMore is derived from page < totalPages.
// TotalRows and TotalPages are nil (SkipCount semantics). totalRows parameter is unused
// but kept for readability at call sites that describe the logical dataset size.
func makeResult(page, totalPages int, _ int64, cols []db.Column, rows []map[string]any) *dataset.QueryResult {
	hasMore := page < totalPages
	return &dataset.QueryResult{
		Columns:  cols,
		Rows:     rows,
		Page:     page,
		PageSize: 50,
		HasMore:  hasMore,
	}
}


func newModel(ds dataset.Dataset) views.RowBrowserModel {
	return views.NewRowBrowserModel(keys.Default(), nil, nil, ds, nil, nil)
}

func TestRowBrowserModel_StartsLoading(t *testing.T) {
	ds := dataset.Dataset{Name: "users", Table: "users"}
	m := newModel(ds)
	if !m.IsLoading() {
		t.Error("expected loading state initially")
	}
}

func TestRowBrowserModel_RowsLoaded(t *testing.T) {
	ds := dataset.Dataset{Name: "users", Table: "users"}
	m := newModel(ds)

	// HasMore=true on page 1 of 3 — totals are not yet known.
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
	// Totals are unknown until end-of-data is discovered.
	if _, ok := m.TotalPages(); ok {
		t.Error("TotalPages should not be known when HasMore=true")
	}
	if _, ok := m.TotalRows(); ok {
		t.Error("TotalRows should not be known when HasMore=true")
	}
}

func TestRowBrowserModel_RowsLoaded_LastPage_DiscoversTotal(t *testing.T) {
	ds := dataset.Dataset{Name: "users", Table: "users"}
	m := newModel(ds)

	// HasMore=false on page 3 — totals are inferred.
	rows := []map[string]any{{"id": int64(1)}, {"id": int64(2)}}
	result := makeResult(3, 3, 0,
		[]db.Column{{Name: "id"}},
		rows,
	)
	m, _ = m.Update(views.RowsLoadedMsg(result))

	if tp, ok := m.TotalPages(); !ok || tp != 3 {
		t.Errorf("expected TotalPages=3, got %d (ok=%v)", tp, ok)
	}
	// Inferred: (3-1)*50 + 2 = 102
	if tr, ok := m.TotalRows(); !ok || tr != 102 {
		t.Errorf("expected TotalRows=102, got %d (ok=%v)", tr, ok)
	}
}

func TestRowBrowserModel_NextPage(t *testing.T) {
	ds := dataset.Dataset{Name: "users", Table: "users"}
	m := newModel(ds)
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
	m := newModel(ds)

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
	m := newModel(ds)

	// HasMore=false means this is the last page.
	result := makeResult(3, 3, 150, []db.Column{{Name: "id"}}, nil)
	m, _ = m.Update(views.RowsLoadedMsg(result))

	// Press ] on last page — should not trigger loading
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{']'}})
	if m.IsLoading() {
		t.Error("should not load when already on last page (HasMore=false)")
	}
}

func TestRowBrowserModel_PrevPageAtFirstPage(t *testing.T) {
	ds := dataset.Dataset{Name: "users", Table: "users"}
	m := newModel(ds)

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
		m := newModel(ds)
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
		m := newModel(ds)
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

// --- Status bar tests ---

func TestRowBrowserModel_StatusLine_TotalUnknown(t *testing.T) {
	ds := dataset.Dataset{Name: "users", Table: "users"}
	m := newModel(ds)

	// Page 2 with more pages ahead — total unknown.
	result := makeResult(2, 5, 0, []db.Column{{Name: "id"}}, nil)
	m, _ = m.Update(views.RowsLoadedMsg(result))

	sl := m.StatusLine()
	if !strings.Contains(sl, "users") {
		t.Errorf("StatusLine %q missing dataset name", sl)
	}
	if !strings.Contains(sl, "page 2") {
		t.Errorf("StatusLine %q missing page number", sl)
	}
	// Total must NOT appear.
	if strings.Contains(sl, "/") {
		t.Errorf("StatusLine %q must not show total pages when unknown", sl)
	}
}

func TestRowBrowserModel_StatusLine_TotalInferred(t *testing.T) {
	ds := dataset.Dataset{Name: "users", Table: "users"}
	m := newModel(ds)

	// HasMore=false on page 3 — totals are inferred (tilde).
	rows := []map[string]any{{"id": 1}, {"id": 2}}
	result := makeResult(3, 3, 0, []db.Column{{Name: "id"}}, rows)
	m, _ = m.Update(views.RowsLoadedMsg(result))

	sl := m.StatusLine()
	if !strings.Contains(sl, "page 3/3") {
		t.Errorf("StatusLine %q missing page 3/3", sl)
	}
	if !strings.Contains(sl, "~") {
		t.Errorf("StatusLine %q missing tilde for inferred total", sl)
	}
	// Inferred: (3-1)*50 + 2 = 102
	if !strings.Contains(sl, "102") {
		t.Errorf("StatusLine %q missing inferred row count 102", sl)
	}
}

func TestRowBrowserModel_StatusLine_TotalExact(t *testing.T) {
	ds := dataset.Dataset{Name: "users", Table: "users"}
	m := newModel(ds)

	// Inject an exact-count result (simulating countLoadedMsg arriving).
	tp := 5
	tr := int64(247)
	countResult := &dataset.QueryResult{
		Page:       1,
		PageSize:   50,
		TotalRows:  &tr,
		TotalPages: &tp,
	}
	m, _ = m.Update(views.CountLoadedMsgForTest(countResult))
	// After count, model loads the last page. Inject that page result.
	pageResult := makeResult(5, 5, 0, []db.Column{{Name: "id"}}, []map[string]any{{"id": 1}})
	m, _ = m.Update(views.RowsLoadedMsg(pageResult))

	sl := m.StatusLine()
	if !strings.Contains(sl, "page 5/5") {
		t.Errorf("StatusLine %q missing page 5/5", sl)
	}
	if strings.Contains(sl, "~") {
		t.Errorf("StatusLine %q must not have tilde for exact total", sl)
	}
	if !strings.Contains(sl, "247") {
		t.Errorf("StatusLine %q missing exact row count 247", sl)
	}
}

func TestRowBrowserModel_StatusLine_FindingLastPage(t *testing.T) {
	ds := dataset.Dataset{Name: "users", Table: "users"}
	m := newModel(ds)
	result := makeResult(1, 3, 0, []db.Column{{Name: "id"}}, nil)
	m, _ = m.Update(views.RowsLoadedMsg(result))

	// Press G — sets "Finding last page..." status.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	sl := m.StatusLine()
	if !strings.Contains(sl, "Finding last page") {
		t.Errorf("StatusLine %q should show 'Finding last page...' after G", sl)
	}
}

func TestRowBrowserModel_View(t *testing.T) {
	ds := dataset.Dataset{Name: "users", Table: "users"}
	m := newModel(ds)
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

// --- Page size input tests ---

func TestRowBrowserModel_PageSizeInput_Open(t *testing.T) {
	ds := dataset.Dataset{Name: "users", Table: "users"}
	m := newModel(ds)
	result := makeResult(1, 1, 1, []db.Column{{Name: "id"}}, nil)
	m, _ = m.Update(views.RowsLoadedMsg(result))

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'P'}})
	if !m.IsPageSizeInputOpen() {
		t.Error("expected page size input open after 'P'")
	}
}

func TestRowBrowserModel_PageSizeInput_EscCloses(t *testing.T) {
	ds := dataset.Dataset{Name: "users", Table: "users"}
	m := newModel(ds)
	result := makeResult(1, 1, 1, []db.Column{{Name: "id"}}, nil)
	m, _ = m.Update(views.RowsLoadedMsg(result))

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'P'}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.IsPageSizeInputOpen() {
		t.Error("expected page size input closed after Esc")
	}
}

func TestRowBrowserModel_PageSizeInput_ValidValue_TriggersLoad(t *testing.T) {
	ds := dataset.Dataset{Name: "users", Table: "users"}
	reg := views.NewPageSizeRegistry(50)
	m := views.NewRowBrowserModel(keys.Default(), nil, nil, ds, reg, nil)
	result := makeResult(1, 3, 0, []db.Column{{Name: "id"}}, nil)
	m, _ = m.Update(views.RowsLoadedMsg(result))

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'P'}})
	// Clear pre-filled value ("50") then type new value.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	for _, r := range "25" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if m.IsPageSizeInputOpen() {
		t.Error("page size input should be closed after valid Enter")
	}
	if reg.Get("users") != 25 {
		t.Errorf("registry should have page size 25 for 'users', got %d", reg.Get("users"))
	}
}

func TestRowBrowserModel_PageSizeInput_PreservesPosition(t *testing.T) {
	// Page 3, cursor at row 10, page size 50:
	//   absolute row = (3-1)*50 + 10 = 110
	// Shrink to 25: new page = 110/25+1 = 5, cursor in page = 110%25 = 10.
	ds := dataset.Dataset{Name: "users", Table: "users"}
	reg := views.NewPageSizeRegistry(50)
	m := views.NewRowBrowserModel(keys.Default(), nil, nil, ds, reg, nil)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 160, Height: 40})

	// Page 3 with 50 rows, HasMore=true.
	rows := make([]map[string]any, 50)
	for i := range rows {
		rows[i] = map[string]any{"id": i + 1}
	}
	result := makeResult(3, 10, 0, []db.Column{{Name: "id"}}, rows)
	m, _ = m.Update(views.RowsLoadedMsg(result))

	// Move cursor to row 10.
	for range 10 {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	}
	if m.RowCursor() != 10 {
		t.Fatalf("pre-condition: cursor should be at row 10, got %d", m.RowCursor())
	}

	// Open page-size input, clear "50", type "25", Enter.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'P'}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	for _, r := range "25" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// Simulate the new load arriving on page 5 (with 25 rows).
	newRows := make([]map[string]any, 25)
	for i := range newRows {
		newRows[i] = map[string]any{"id": i + 1}
	}
	newResult := makeResult(5, 10, 0, []db.Column{{Name: "id"}}, newRows)
	m, _ = m.Update(views.RowsLoadedMsg(newResult))

	if m.Page() != 5 {
		t.Errorf("expected page 5, got %d", m.Page())
	}
	if m.RowCursor() != 10 {
		t.Errorf("expected cursor at row 10, got %d", m.RowCursor())
	}
	// Cursor must be visible: rowOffset ≤ cursor < rowOffset + visibleRows.
	vis := m.VisibleRowCount()
	off := m.RowOffset()
	if m.RowCursor() < off || (vis > 0 && m.RowCursor() >= off+vis) {
		t.Errorf("cursor %d not visible: offset=%d visible=%d", m.RowCursor(), off, vis)
	}
}

func TestRowBrowserModel_PageSizeInput_InvalidValue_ShowsError(t *testing.T) {
	ds := dataset.Dataset{Name: "users", Table: "users"}
	m := newModel(ds)
	result := makeResult(1, 1, 1, []db.Column{{Name: "id"}}, nil)
	m, _ = m.Update(views.RowsLoadedMsg(result))

	m, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'P'}})
	// Clear the pre-filled value ("50") and type "0" (invalid).
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'0'}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if !m.IsPageSizeInputOpen() {
		t.Error("page size input should remain open after invalid value")
	}
	v := m.View()
	if !strings.Contains(v, "must be between") {
		t.Errorf("view should show error message, got: %q", v)
	}
}

func TestRowBrowserModel_PageSizeInput_NonDigitDropped(t *testing.T) {
	ds := dataset.Dataset{Name: "users", Table: "users"}
	reg := views.NewPageSizeRegistry(50)
	m := views.NewRowBrowserModel(keys.Default(), nil, nil, ds, reg, nil)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	result := makeResult(1, 1, 1, []db.Column{{Name: "id"}}, nil)
	m, _ = m.Update(views.RowsLoadedMsg(result))

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'P'}})
	// Type 'a' — should be silently dropped.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if !m.IsPageSizeInputOpen() {
		t.Error("page size input must remain open after non-digit")
	}
}

func TestRowBrowserModel_PageSizeInput_ViewRendered(t *testing.T) {
	ds := dataset.Dataset{Name: "users", Table: "users"}
	m := newModel(ds)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	result := makeResult(1, 1, 1, []db.Column{{Name: "id"}}, nil)
	m, _ = m.Update(views.RowsLoadedMsg(result))

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'P'}})
	v := m.View()
	if !strings.Contains(v, "Page size:") {
		t.Errorf("view should show 'Page size:' bar when P pressed, got:\n%s", v)
	}
}

// --- First/last page ---

func TestRowBrowserModel_FirstPage_g(t *testing.T) {
	ds := dataset.Dataset{Name: "users", Table: "users"}
	m := newModel(ds)

	result := makeResult(3, 5, 0, []db.Column{{Name: "id"}}, nil)
	m, _ = m.Update(views.RowsLoadedMsg(result))

	// Press g — should load page 1.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	if !m.IsLoading() {
		t.Error("expected loading after 'g' (first page)")
	}
}

func TestRowBrowserModel_FirstPage_Home(t *testing.T) {
	ds := dataset.Dataset{Name: "users", Table: "users"}
	m := newModel(ds)

	result := makeResult(3, 5, 0, []db.Column{{Name: "id"}}, nil)
	m, _ = m.Update(views.RowsLoadedMsg(result))

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyHome})
	if !m.IsLoading() {
		t.Error("expected loading after Home key (first page)")
	}
}

func TestRowBrowserModel_LastPage_G_SetsStatus(t *testing.T) {
	ds := dataset.Dataset{Name: "users", Table: "users"}
	m := newModel(ds)

	result := makeResult(1, 5, 0, []db.Column{{Name: "id"}}, nil)
	m, _ = m.Update(views.RowsLoadedMsg(result))

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	sl := m.StatusLine()
	if !strings.Contains(sl, "Finding last page") {
		t.Errorf("after G, status should show 'Finding last page...', got: %q", sl)
	}
}

// --- Filter modal tests ---

func TestRowBrowserModel_FilterModal_Open(t *testing.T) {
	ds := dataset.Dataset{Name: "users", Table: "users"}
	m := newModel(ds)
	result := makeResult(1, 1, 1, []db.Column{{Name: "id"}}, nil)
	m, _ = m.Update(views.RowsLoadedMsg(result))

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if !m.IsFilterModalOpen() {
		t.Error("expected filter modal open after 'q'")
	}
}

func TestRowBrowserModel_FilterModal_Cancel(t *testing.T) {
	ds := dataset.Dataset{Name: "users", Table: "users"}
	m := newModel(ds)
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
	m := newModel(ds)
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
	m := newModel(ds)
	result := makeResult(1, 1, 1, []db.Column{{Name: "id"}}, nil)
	m, _ = m.Update(views.RowsLoadedMsg(result))

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	if !m.IsLocalSearchInputActive() {
		t.Error("expected local search input active after '/'")
	}
}

func TestRowBrowserModel_LocalSearch_CloseWithEsc(t *testing.T) {
	ds := dataset.Dataset{Name: "users", Table: "users"}
	m := newModel(ds)
	result := makeResult(1, 1, 1, []db.Column{{Name: "id"}}, nil)
	m, _ = m.Update(views.RowsLoadedMsg(result))

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.IsLocalSearchInputActive() {
		t.Error("expected local search input closed after esc")
	}
}

func TestRowBrowserModel_LocalSearch_FilteredView(t *testing.T) {
	ds := dataset.Dataset{Name: "users", Table: "users"}
	m := newModel(ds)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
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

	v := m.View()
	// Matching rows visible
	if !strings.Contains(v, "Alice") {
		t.Error("filtered view should show Alice")
	}
	if !strings.Contains(v, "Alice2") {
		t.Error("filtered view should show Alice2")
	}
	// Non-matching row must NOT appear
	if strings.Contains(v, "Bob") {
		t.Error("filtered view must not show Bob")
	}

	// arrow keys navigate matches — no crash and input stays closed
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})

	if m.IsLocalSearchInputActive() {
		t.Error("search input should be closed after Enter")
	}
}

func TestRowBrowserModel_LocalSearch_FlashOnCommit(t *testing.T) {
	ds := dataset.Dataset{Name: "users", Table: "users"}
	m := newModel(ds)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	result := makeResult(1, 1, 2,
		[]db.Column{{Name: "name"}},
		[]map[string]any{{"name": "Alice"}, {"name": "Bob"}},
	)
	m, _ = m.Update(views.RowsLoadedMsg(result))

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	for _, r := range "alice" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if cmd == nil {
		t.Fatal("Enter with active query must return a tick cmd for the flash")
	}
	if !m.IsLocalSearchFlashing() {
		t.Error("localSearchFlashing must be true immediately after Enter")
	}

	// Simulate flash expiry.
	m, _ = m.Update(views.LocalSearchFlashExpiredMsgForTest())
	if m.IsLocalSearchFlashing() {
		t.Error("localSearchFlashing must be false after expiry message")
	}
}

func TestRowBrowserModel_LocalSearch_OnFocusGained_Flashes(t *testing.T) {
	ds := dataset.Dataset{Name: "users", Table: "users"}
	m := newModel(ds)
	result := makeResult(1, 1, 1, []db.Column{{Name: "id"}}, nil)
	m, _ = m.Update(views.RowsLoadedMsg(result))

	// Open, type, commit (hold the search).
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	for _, r := range "foo" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	// Expire the commit flash so state is calm.
	m, _ = m.Update(views.LocalSearchFlashExpiredMsgForTest())

	m2, cmd := m.OnFocusGained()
	if cmd == nil {
		t.Error("OnFocusGained with held search must return a tick cmd")
	}
	if !m2.IsLocalSearchFlashing() {
		t.Error("OnFocusGained must set localSearchFlashing=true when search is held")
	}
}

func TestRowBrowserModel_LocalSearch_OnFocusGained_NoSearch(t *testing.T) {
	ds := dataset.Dataset{Name: "users", Table: "users"}
	m := newModel(ds)
	_, cmd := m.OnFocusGained()
	if cmd != nil {
		t.Error("OnFocusGained with no search must return nil cmd")
	}
}

func TestRowBrowserModel_LocalSearch_HeldBarVisible(t *testing.T) {
	ds := dataset.Dataset{Name: "users", Table: "users"}
	m := newModel(ds)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	result := makeResult(1, 1, 2,
		[]db.Column{{Name: "name"}},
		[]map[string]any{{"name": "Alice"}, {"name": "Bob"}},
	)
	m, _ = m.Update(views.RowsLoadedMsg(result))

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	for _, r := range "alice" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if m.IsLocalSearchInputActive() {
		t.Fatal("expected search input closed after Enter")
	}
	v := m.View()
	if !strings.Contains(v, `"alice"`) {
		t.Errorf("held search bar must show quoted query, got:\n%s", v)
	}
	if !strings.Contains(v, "esc clear") {
		t.Errorf("held search bar must show 'esc clear' hint, got:\n%s", v)
	}
	if !strings.Contains(v, "↑/↓ navigate") {
		t.Errorf("held search bar must show '↑/↓ navigate' hint, got:\n%s", v)
	}
}

func TestRowBrowserModel_LocalSearch_ArrowNavigation(t *testing.T) {
	ds := dataset.Dataset{Name: "users", Table: "users"}
	m := newModel(ds)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	result := makeResult(1, 1, 3,
		[]db.Column{{Name: "name"}},
		[]map[string]any{
			{"name": "Alice"},
			{"name": "Bob"},
			{"name": "Alice2"},
		},
	)
	m, _ = m.Update(views.RowsLoadedMsg(result))

	// commit a search that matches two rows
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	for _, r := range "alice" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// initial cursor is at match 0 — Alice is selected, Alice2 also visible
	v := m.View()
	if !strings.Contains(v, "Alice") {
		t.Error("first match (Alice) must be visible initially")
	}

	// Down should advance to match 1
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	v = m.View()
	if !strings.Contains(v, "Alice2") {
		t.Error("second match (Alice2) must be visible after Down")
	}

	// Up should go back to match 0 — no crash
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if m.IsLocalSearchInputActive() {
		t.Error("search input must stay closed during arrow navigation")
	}
}

func TestRowBrowserModel_StatusLine_LocalSearchHeld(t *testing.T) {
	ds := dataset.Dataset{Name: "users", Table: "users"}
	m := newModel(ds)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	result := makeResult(1, 1, 1, []db.Column{{Name: "id"}}, nil)
	m, _ = m.Update(views.RowsLoadedMsg(result))

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	for _, r := range "alice" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	sl := m.StatusLine()
	if strings.Contains(sl, "search:") {
		t.Errorf("StatusLine must not include search text when held (bar is in the view): %q", sl)
	}
	if !strings.Contains(sl, "users") {
		t.Errorf("StatusLine must show dataset name when search is held, got: %q", sl)
	}
}

func TestRowBrowserModel_QuickFilter_Open(t *testing.T) {
	ds := dataset.Dataset{Name: "users", Table: "users"}
	m := newModel(ds)
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
	m := newModel(ds)
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
	m := newModel(ds)
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
	m := newModel(ds)
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

func TestRowBrowserModel_Sort_ClearsTotalOnChange(t *testing.T) {
	ds := dataset.Dataset{Name: "users", Table: "users"}
	m := newModel(ds)

	// Load last page (infers total).
	result := makeResult(3, 3, 0, []db.Column{{Name: "id"}}, []map[string]any{{"id": 1}})
	m, _ = m.Update(views.RowsLoadedMsg(result))

	if _, ok := m.TotalPages(); !ok {
		t.Fatal("expected TotalPages to be known after last page")
	}

	// Sort change must clear the discovered total.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	if _, ok := m.TotalPages(); ok {
		t.Error("TotalPages should be cleared after sort change")
	}
}

// --- Export menu test ---

func TestRowBrowserModel_ExportMenu_Open(t *testing.T) {
	ds := dataset.Dataset{Name: "users", Table: "users"}
	m := newModel(ds)
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
	m := newModel(ds)
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

func TestRowBrowserModel_NeedsBackKey_PageSizeInput(t *testing.T) {
	ds := dataset.Dataset{Name: "users", Table: "users"}
	m := newModel(ds)
	result := makeResult(1, 1, 1, []db.Column{{Name: "id"}}, nil)
	m, _ = m.Update(views.RowsLoadedMsg(result))

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'P'}})
	if !m.NeedsBackKey() {
		t.Error("NeedsBackKey should be true when page size input is open")
	}
}

// --- FK drill-down tests ---

func TestRowBrowserModel_RowCursor_Down(t *testing.T) {
	ds := dataset.Dataset{Name: "orders", Table: "orders"}
	m := newModel(ds)
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
	m := newModel(ds)
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
	m := newModel(ds)

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
	m := newModel(ds)
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
	m := newModel(ds)
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
	m := newModel(ds)
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
	m := newModel(ds)
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
	m := newModel(ds)
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
	m := newModel(ds)
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
	m := newModel(ds)
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
	m := newModel(ds)
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
