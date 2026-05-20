package views_test

// Acceptance tests for the Multi-Column Sort feature.
// Each test maps to one or more acceptance criteria in tasks/ready/multi-column-sort.md.
//
// Coverage map:
//   CL01: multi-column ORDER BY emitted                    → dataset_test.go: TestAC_CL01_MultiColumnSortOrderBy
//   CL02: nil/empty sort → no ORDER BY                    → dataset_test.go: TestAC_CL02_NilAndEmptySortNoOrderBy
//   CL03: unknown column rejected                          → dataset_test.go: TestAC_CL03_UnknownSortColumnRejected
//   CL04: single-element matches old single-sort           → dataset_test.go: TestAC_CL04_SingleElementSortMatchesSingleSort
//   SM01-SM11: sort manager model unit tests               → sortmanager_test.go
//   HD01: ↑¹/↓² in header for multi-sort                 → rowbrowser_test.go: TestAC_HD01_MultiSortHeaderSuperscripts
//   HD02: nil sort → no superscripts                      → rowbrowser_test.go: TestAC_HD02_NoSortNoSuperscripts
//   HD03: unsorted columns have no marker                  → rowbrowser_test.go: TestAC_HD03_UnsortedColumnsNoMarker
//   PL01: multi-sort pill shows both columns + separator  → rowbrowser_test.go: TestAC_PL01_MultiSortPillBothColumns
//   PL02: nil sort → no pill                              → rowbrowser_test.go: TestAC_PL02_NoSortNoPill
//   PL03: single sort pill, no separator                  → rowbrowser_test.go: TestAC_PL03_SingleSortPillNoSeparator
//   AC01: single sort — s cycles, no overlay              → TestAC_AC01_SingleSortNoOverlay
//   AC02: cycle on same column — no overlay               → TestAC_AC02_CycleOnSameColumnNoOverlay
//   AC03: second column → overlay with both pre-added     → TestAC_AC03_SecondColumnOpensOverlay
//   AC04: confirm multi-sort from AC03                    → TestAC_AC04_ConfirmMultiSort
//   AC05: S opens overlay even with no sort               → TestAC_AC05_SAlwaysOpensOverlay
//   AC06: S opens overlay with existing sort in active    → TestAC_AC06_SOpensWithExistingSort
//   AC07: toggle direction in overlay                     → TestAC_AC07_ToggleDirectionInOverlay
//   AC08: reorder via J                                   → TestAC_AC08_ReorderViaJ
//   AC09: remove via Del                                  → TestAC_AC09_RemoveViaDel
//   AC10: Esc reverts                                     → TestAC_AC10_EscReverts

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/polesen/datacow/internal/core/dataset"
	"github.com/polesen/datacow/internal/core/db"
	"github.com/polesen/datacow/internal/tui/keys"
	"github.com/polesen/datacow/internal/tui/views"
)

// makeSortModel creates a RowBrowserModel loaded with a result.
func makeSortModel(cols []db.Column, rows []map[string]any) views.RowBrowserModel {
	ds := dataset.Dataset{Name: "sort_test", Table: "sort_test"}
	m := views.NewRowBrowserModel(keys.Default(), nil, nil, ds, nil, nil)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	result := makeResult(1, 1, int64(len(rows)), cols, rows)
	m, _ = m.Update(views.RowsLoadedMsg(result))
	return m
}

// AC01: single sort — s on a column, no sort active → cycle, no overlay.
func TestAC_AC01_SingleSortNoOverlay(t *testing.T) {
	cols := []db.Column{{Name: "A"}, {Name: "B"}, {Name: "C"}}
	rows := []map[string]any{{"A": "a", "B": "b", "C": "c"}}
	m := makeSortModel(cols, rows)

	// Cursor on A (default=0), press s.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})

	v := m.View()
	// Overlay must NOT appear.
	if strings.Contains(v, "Sort ") && strings.Contains(v, "Available") && strings.Contains(v, "Enter confirm") {
		t.Errorf("expected no sort manager overlay on first s, got:\n%s", v)
	}

	// Re-inject result so the model is unblocked.
	result := makeResult(1, 1, 1, cols, rows)
	m, _ = m.Update(views.RowsLoadedMsg(result))

	v = m.View()
	if !strings.Contains(v, "A") {
		t.Errorf("expected 'A' in sort pill, got:\n%s", v)
	}
	if !strings.Contains(v, "↑¹") {
		t.Errorf("expected ↑¹ on column A in header, got:\n%s", v)
	}
}

// AC02: cycle on same column — pressing s again cycles ASC → DESC → off, no overlay.
func TestAC_AC02_CycleOnSameColumnNoOverlay(t *testing.T) {
	cols := []db.Column{{Name: "A"}, {Name: "B"}}
	rows := []map[string]any{{"A": "a", "B": "b"}}
	m := makeSortModel(cols, rows)
	result := makeResult(1, 1, 1, cols, rows)

	// First s → A ASC
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	m, _ = m.Update(views.RowsLoadedMsg(result))

	// Second s on A → A DESC
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	// Overlay must not appear.
	v := m.View()
	if strings.Contains(v, "Enter confirm") && strings.Contains(v, "Available") {
		t.Errorf("expected no overlay on second s same column, got:\n%s", v)
	}

	m, _ = m.Update(views.RowsLoadedMsg(result))
	v = m.View()
	if !strings.Contains(v, "↓¹") && !strings.Contains(v, "↓") {
		t.Errorf("expected ↓ after second s, got:\n%s", v)
	}

	// Third s on A → no sort
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	m, _ = m.Update(views.RowsLoadedMsg(result))
	sort := m.ActiveSort()
	if len(sort) != 0 {
		t.Errorf("expected no sort after third s, got %v", sort)
	}
}

// AC03: with A ↑ active, move to B, press s → overlay appears; both A and B in Active.
func TestAC_AC03_SecondColumnOpensOverlay(t *testing.T) {
	cols := []db.Column{{Name: "A"}, {Name: "B"}}
	rows := []map[string]any{{"A": "a", "B": "b"}}
	m := makeSortModel(cols, rows)
	result := makeResult(1, 1, 1, cols, rows)

	// Sort A ASC.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	m, _ = m.Update(views.RowsLoadedMsg(result))

	// Move cursor to B.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})

	// Press s → overlay should open.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})

	v := m.View()
	if !strings.Contains(v, "Active") {
		t.Errorf("expected overlay with Active section, got:\n%s", v)
	}
	if !strings.Contains(v, "1. ") {
		t.Errorf("expected '1. ' for A in Active, got:\n%s", v)
	}
	if !strings.Contains(v, "2. ") {
		t.Errorf("expected '2. ' for B in Active (pre-added), got:\n%s", v)
	}
	// B should be in Active (pre-added), not in Available.
	// Available should be empty or not contain B.
}

// AC04: confirm from AC03 → pill shows "A ↑ · B ↑"; headers show ↑¹ and ↑².
func TestAC_AC04_ConfirmMultiSort(t *testing.T) {
	cols := []db.Column{{Name: "A"}, {Name: "B"}}
	rows := []map[string]any{{"A": "a", "B": "b"}}
	m := makeSortModel(cols, rows)
	result := makeResult(1, 1, 1, cols, rows)

	// Sort A ASC.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	m, _ = m.Update(views.RowsLoadedMsg(result))

	// Move to B, press s to open overlay.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})

	// Confirm.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = m.Update(views.RowsLoadedMsg(result))

	v := m.View()
	if !strings.Contains(v, "↑¹") {
		t.Errorf("expected ↑¹ on A, got:\n%s", v)
	}
	if !strings.Contains(v, "↑²") {
		t.Errorf("expected ↑² on B, got:\n%s", v)
	}
	if !strings.Contains(v, "·") {
		t.Errorf("expected separator in multi-sort pill, got:\n%s", v)
	}
}

// AC05: S with no sort active → overlay shows only Available section.
func TestAC_AC05_SAlwaysOpensOverlay(t *testing.T) {
	cols := []db.Column{{Name: "A"}, {Name: "B"}}
	rows := []map[string]any{{"A": "a", "B": "b"}}
	m := makeSortModel(cols, rows)

	// Press S with no sort.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'S'}})

	v := m.View()
	if !strings.Contains(v, "Available") {
		t.Errorf("expected overlay with Available section, got:\n%s", v)
	}
	if strings.Contains(v, "Active") {
		t.Errorf("expected no Active section when no sort, got:\n%s", v)
	}

	// Press Esc.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	sort := m.ActiveSort()
	if len(sort) != 0 {
		t.Errorf("expected no sort after Esc, got %v", sort)
	}
}

// AC06: S with existing sort → overlay shows Active section with existing sort.
func TestAC_AC06_SOpensWithExistingSort(t *testing.T) {
	cols := []db.Column{{Name: "A"}, {Name: "B"}}
	rows := []map[string]any{{"A": "a", "B": "b"}}
	m := makeSortModel(cols, rows)
	result := makeResult(1, 1, 1, cols, rows)

	// Sort A ASC.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	m, _ = m.Update(views.RowsLoadedMsg(result))

	// Press S.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'S'}})

	v := m.View()
	if !strings.Contains(v, "Active") {
		t.Errorf("expected Active section with existing sort, got:\n%s", v)
	}
	if !strings.Contains(v, "1. ") {
		t.Errorf("expected '1. A' in Active section, got:\n%s", v)
	}
}

// AC07: open overlay, add A, toggle direction with Space → shows ↓. Confirm → pill shows A ↓.
func TestAC_AC07_ToggleDirectionInOverlay(t *testing.T) {
	cols := []db.Column{{Name: "A"}, {Name: "B"}}
	rows := []map[string]any{{"A": "a", "B": "b"}}
	m := makeSortModel(cols, rows)
	result := makeResult(1, 1, 1, cols, rows)

	// Open overlay with S.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'S'}})

	// Space adds A (first available).
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})

	// cursor moved to A in Active; Space again → toggle to ↓.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})

	v := m.View()
	if !strings.Contains(v, "↓") {
		t.Errorf("expected ↓ after toggle in overlay, got:\n%s", v)
	}

	// Confirm.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = m.Update(views.RowsLoadedMsg(result))

	v = m.View()
	if !strings.Contains(v, "↓¹") && !strings.Contains(v, "↓") {
		t.Errorf("expected ↓ on A after confirm, got:\n%s", v)
	}
}

// AC08: reorder — open overlay with [A, B], move A down with J → B becomes 1., A becomes 2.
func TestAC_AC08_ReorderViaJ(t *testing.T) {
	cols := []db.Column{{Name: "A"}, {Name: "B"}}
	rows := []map[string]any{{"A": "a", "B": "b"}}
	m := makeSortModel(cols, rows)
	result := makeResult(1, 1, 1, cols, rows)

	// Build A ASC + B ASC via sort manager.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'S'}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}}) // add A
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})                       // cursor to Available
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}}) // add B
	// Now go back to A (index 0).
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})

	// cursor=0 (A), press J → A moves to position 1, B to position 0.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'J'}})

	v := m.View()
	if !strings.Contains(v, "1. ") || !strings.Contains(v, "2. ") {
		t.Fatalf("expected two numbered entries, got:\n%s", v)
	}

	// Confirm.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = m.Update(views.RowsLoadedMsg(result))

	sort := m.ActiveSort()
	if len(sort) != 2 {
		t.Fatalf("expected 2 sort entries, got %d", len(sort))
	}
	if sort[0].Column != "B" || sort[1].Column != "A" {
		t.Errorf("expected [B, A] after J reorder, got %v", sort)
	}

	v = m.View()
	if !strings.Contains(v, "·") {
		t.Errorf("expected pill with separator after reorder confirm, got:\n%s", v)
	}
}

// AC09: remove — open overlay with [A, B], remove A → only B remains.
func TestAC_AC09_RemoveViaDel(t *testing.T) {
	cols := []db.Column{{Name: "A"}, {Name: "B"}}
	rows := []map[string]any{{"A": "a", "B": "b"}}
	m := makeSortModel(cols, rows)
	result := makeResult(1, 1, 1, cols, rows)

	// Build A + B in active.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'S'}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}}) // add A
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})                       // cursor to B in Available
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}}) // add B
	// cursor is now at B (index 1 in active). Go to A (index 0).
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp}) // now at A (index 0)

	// Delete A.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDelete})

	v := m.View()
	if !strings.Contains(v, "1. ") {
		t.Errorf("expected '1. B' after removing A, got:\n%s", v)
	}
	if strings.Contains(v, "2. ") {
		t.Errorf("expected no '2.' after removing A, got:\n%s", v)
	}

	// Confirm.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = m.Update(views.RowsLoadedMsg(result))

	sort := m.ActiveSort()
	if len(sort) != 1 || sort[0].Column != "B" {
		t.Errorf("expected only B after removing A, got %v", sort)
	}
}

// AC10: Esc reverts — add a column in overlay then Esc → pill unchanged.
func TestAC_AC10_EscReverts(t *testing.T) {
	cols := []db.Column{{Name: "A"}, {Name: "B"}}
	rows := []map[string]any{{"A": "a", "B": "b"}}
	m := makeSortModel(cols, rows)
	result := makeResult(1, 1, 1, cols, rows)

	// Sort A ASC first.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	m, _ = m.Update(views.RowsLoadedMsg(result))

	sortBefore := m.ActiveSort()

	// Open overlay.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'S'}})

	// Navigate to available B and add it.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})

	// Esc → cancel.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})

	sortAfter := m.ActiveSort()
	if len(sortAfter) != len(sortBefore) {
		t.Errorf("expected sort unchanged after Esc: before=%v after=%v", sortBefore, sortAfter)
	}
	if len(sortAfter) > 0 && sortAfter[0].Column != sortBefore[0].Column {
		t.Errorf("expected sort unchanged after Esc: before=%v after=%v", sortBefore, sortAfter)
	}
}
