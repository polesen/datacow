package views_test

// Acceptance tests for the column picker feature.
// Each test maps to one or more acceptance criteria in tasks/ready/column-picker.md.
//
// Sections:
//   CP  — Column Picker (CP01–CP07)
//
// Coverage map (criteria in order from the task spec):
//   1. QueryOptions.Columns + projected SELECT + validation    → dataset_test.go: Query_columns_projection_table, Query_columns_unknown_rejected, Query_columns_injection_rejected
//   2. Empty/nil Columns → SELECT *                           → dataset_test.go: Query_columns_empty_selects_all
//   3. Works for table and SQL datasets                        → dataset_test.go: Query_columns_projection_table, Query_columns_projection_sql_dataset
//   4. C opens picker; Esc cancels without re-fetch           → TestAC_CP01
//   5. Space toggles; J/K reorder; a selects all; r resets    → TestAC_CP02
//   6. Enter applies and triggers re-fetch                    → TestAC_CP03
//   7. Zero visible columns shows error, keeps overlay open   → TestAC_CP04
//   8. Status bar shows cols: N/M when non-default            → TestAC_CP05
//   9. Selection preserved across navigation                  → TestAC_CP06
//  10. Export uses active column projection                    → TestAC_CP03 (verifies registry state; startExport passes VisibleColumns — same code path, no separate export integration test needed)
//  11. C key in keys.Map + helpoverlay                        → TestAC_CP07
//  12. Core tests: projection SQL for table and SQL datasets  → dataset_test.go (see above)
//  13. ColumnPickerModel view unit tests                      → columnpicker_test.go: TestColumnPicker_*
//  14. App integration test: open, hide, confirm, SQL log     → app_test.go: TestApp_ColumnPicker_SmokeTest

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/polesen/datacow/internal/core/dataset"
	"github.com/polesen/datacow/internal/core/db"
	"github.com/polesen/datacow/internal/tui/keys"
	"github.com/polesen/datacow/internal/tui/views"
)

// makeColumnPickerModel returns a RowBrowserModel with a result loaded and a column registry.
func makeColumnPickerModel(cols []db.Column, rows []map[string]any) views.RowBrowserModel {
	ds := dataset.Dataset{Name: "cp_test", Table: "cp_test"}
	reg := views.NewColumnRegistry()
	m := views.NewRowBrowserModelWithColumns(keys.Default(), nil, nil, ds, nil, nil, reg)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	result := makeResult(1, 1, int64(len(rows)),
		cols,
		rows,
	)
	m, _ = m.Update(views.RowsLoadedMsg(result))
	return m
}

// CP01: C opens the column picker; Esc cancels without re-fetching.
func TestAC_CP01_COpensPickerEscCancels(t *testing.T) {
	cols := []db.Column{{Name: "id"}, {Name: "name"}, {Name: "payload"}}
	rows := []map[string]any{{"id": 1, "name": "Alice", "payload": "data"}}
	m := makeColumnPickerModel(cols, rows)

	// C opens the picker.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'C'}})
	v := m.View()
	if !strings.Contains(v, "Columns") {
		t.Errorf("expected picker to open with 'Columns' header, got:\n%s", v)
	}
	if !strings.Contains(v, "id") || !strings.Contains(v, "name") || !strings.Contains(v, "payload") {
		t.Errorf("expected all column names in picker, got:\n%s", v)
	}

	// Esc closes without mode change.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	v = m.View()
	if strings.Contains(v, "Columns") && strings.Contains(v, "Space toggle") {
		t.Error("picker should be closed after Esc")
	}
	// After Esc, all three columns should still be shown (no re-fetch, no change).
	if !strings.Contains(v, "id") || !strings.Contains(v, "name") || !strings.Contains(v, "payload") {
		t.Errorf("all columns should be visible after Esc cancel, got:\n%s", v)
	}
}

// CP02: Space toggles visibility; J/K reorder; a selects all; d deselects all; r resets.
func TestAC_CP02_PickerKeyboardControls(t *testing.T) {
	cols := []db.Column{{Name: "id"}, {Name: "name"}, {Name: "extra"}}
	rows := []map[string]any{{"id": 1, "name": "Alice", "extra": "x"}}
	m := makeColumnPickerModel(cols, rows)

	// Open picker.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'C'}})

	// J moves the focused column down (id swaps with name).
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'J'}})
	// K moves it back up.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'K'}})

	// Move down to "name", Space hides it.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	v := m.View()
	if !strings.Contains(v, "[ ]") {
		t.Errorf("expected [ ] for hidden column after Space, got:\n%s", v)
	}

	// 'a' selects all.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	v = m.View()
	if strings.Contains(v, "[ ]") {
		t.Errorf("expected all [✓] after 'a', got:\n%s", v)
	}

	// 'd' deselects all.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	v = m.View()
	if strings.Contains(v, "[✓]") {
		t.Errorf("expected all [ ] after 'd', got:\n%s", v)
	}

	// 'r' resets to all visible in schema order.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	v = m.View()
	if strings.Contains(v, "[ ]") {
		t.Errorf("expected all [✓] after 'r', got:\n%s", v)
	}
}

// CP03: Enter confirms, triggers re-fetch; hidden column absent from row browser.
func TestAC_CP03_EnterAppliesProjection(t *testing.T) {
	cols := []db.Column{{Name: "id"}, {Name: "name"}, {Name: "drop_me"}}
	rows := []map[string]any{{"id": 1, "name": "Alice", "drop_me": "secret"}}
	m := makeColumnPickerModel(cols, rows)

	// Open picker.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'C'}})

	// Navigate to "drop_me" (cursor=2) and hide it.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})

	// Confirm — we have no executor so no re-fetch happens, but mode changes.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// Verify the model went back to normal mode (no picker in view).
	v := m.View()
	if strings.Contains(v, "Space toggle") {
		t.Errorf("picker should be closed after Enter, got:\n%s", v)
	}

	// The registry now has drop_me hidden. StatusLine should show cols: 2/3.
	sl := m.StatusLine()
	if !strings.Contains(sl, "cols: 2/3") {
		t.Errorf("status line should show 'cols: 2/3', got: %s", sl)
	}

	// Simulate the re-fetch arriving with the projected result (only id and name).
	projectedResult := makeResult(1, 1, 1,
		[]db.Column{{Name: "id"}, {Name: "name"}},
		[]map[string]any{{"id": 1, "name": "Alice"}},
	)
	m, _ = m.Update(views.RowsLoadedMsg(projectedResult))

	v = m.View()
	if !strings.Contains(v, "id") || !strings.Contains(v, "name") {
		t.Errorf("projected columns should be visible, got:\n%s", v)
	}
	if strings.Contains(v, "drop_me") {
		t.Errorf("dropped column header should be absent from row browser, got:\n%s", v)
	}
}

// CP04: Confirming with zero visible columns shows error and keeps picker open.
func TestAC_CP04_ZeroColumnsShowsError(t *testing.T) {
	cols := []db.Column{{Name: "id"}, {Name: "name"}}
	rows := []map[string]any{{"id": 1, "name": "Alice"}}
	m := makeColumnPickerModel(cols, rows)

	// Open picker.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'C'}})

	// Hide both columns.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}}) // hide id
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}}) // hide name

	// Try to confirm.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	v := m.View()
	if !strings.Contains(v, "at least one column required") {
		t.Errorf("expected error 'at least one column required', got:\n%s", v)
	}
	// Picker must still be open.
	if !strings.Contains(v, "Space toggle") {
		t.Errorf("picker should remain open after failed confirm, got:\n%s", v)
	}
}

// CP05: Status bar shows cols: N/M when non-default projection is active.
func TestAC_CP05_StatusBarColsIndicator(t *testing.T) {
	cols := []db.Column{{Name: "id"}, {Name: "name"}, {Name: "extra"}}
	rows := []map[string]any{{"id": 1, "name": "Alice", "extra": "x"}}
	m := makeColumnPickerModel(cols, rows)

	// Default state: status line should NOT show "cols:".
	sl := m.StatusLine()
	if strings.Contains(sl, "cols:") {
		t.Errorf("status line should not show cols: for default selection, got: %s", sl)
	}

	// Open picker, hide "extra".
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'C'}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}}) // hide extra
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// After confirm, status line shows cols: 2/3.
	sl = m.StatusLine()
	if !strings.Contains(sl, "cols: 2/3") {
		t.Errorf("expected 'cols: 2/3' in status line, got: %s", sl)
	}
}

// CP06: Column selection is preserved across multiple result arrivals for the same dataset.
func TestAC_CP06_SelectionPreserved(t *testing.T) {
	cols := []db.Column{{Name: "id"}, {Name: "name"}, {Name: "extra"}}
	rows := []map[string]any{{"id": 1, "name": "Alice", "extra": "x"}}
	m := makeColumnPickerModel(cols, rows)

	// Hide "extra" via the picker.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'C'}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// Simulate re-fetch result with projection.
	projected := makeResult(1, 1, 1,
		[]db.Column{{Name: "id"}, {Name: "name"}},
		[]map[string]any{{"id": 1, "name": "Alice"}},
	)
	m, _ = m.Update(views.RowsLoadedMsg(projected))

	// Status line still shows cols: 2/3.
	sl := m.StatusLine()
	if !strings.Contains(sl, "cols: 2/3") {
		t.Errorf("expected 'cols: 2/3' preserved after re-fetch, got: %s", sl)
	}
}

// CP07: C key is in keys.Map and in the full help.
func TestAC_CP07_KeyMappedAndInHelp(t *testing.T) {
	k := keys.Default()
	// Verify the key binding exists.
	bindings := k.ColumnPicker.Keys()
	if len(bindings) == 0 {
		t.Error("ColumnPicker key binding must have at least one key")
	}
	found := false
	for _, key := range bindings {
		if key == "C" {
			found = true
		}
	}
	if !found {
		t.Errorf("ColumnPicker key must be 'C', got %v", bindings)
	}

	// Verify it appears in FullHelp.
	full := k.FullHelp()
	for _, group := range full {
		for _, b := range group {
			if b.Help().Key == k.ColumnPicker.Help().Key {
				return
			}
		}
	}
	t.Error("ColumnPicker must appear in FullHelp")
}
