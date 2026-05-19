package views_test

import (
	"strings"
	"testing"

	"github.com/polesen/datacow/internal/tui/views"
)

// makeSelections builds a ColumnSelection slice from names (all visible).
func makeSelections(names ...string) []views.ColumnSelection {
	sel := make([]views.ColumnSelection, len(names))
	for i, n := range names {
		sel[i] = views.ColumnSelection{Name: n, Visible: true}
	}
	return sel
}

// makePicker builds a ColumnPickerModel sized to 80×30.
func makePicker(names ...string) views.ColumnPickerModel {
	sel := makeSelections(names...)
	return views.NewColumnPickerModel(sel, sel, 80, 30)
}

// TestColumnPicker_RenderChecked verifies that visible columns show [✓].
func TestColumnPicker_RenderChecked(t *testing.T) {
	p := makePicker("id", "name", "email")
	out := p.View()
	if !strings.Contains(out, "[✓]") {
		t.Error("expected [✓] for visible columns")
	}
	if !strings.Contains(out, "id") || !strings.Contains(out, "name") || !strings.Contains(out, "email") {
		t.Error("expected all column names in view")
	}
}

// TestColumnPicker_RenderUnchecked verifies that hidden columns show [ ].
func TestColumnPicker_RenderUnchecked(t *testing.T) {
	sel := []views.ColumnSelection{
		{Name: "id", Visible: true},
		{Name: "payload", Visible: false},
	}
	p := views.NewColumnPickerModel(sel, sel, 80, 30)
	out := p.View()
	if !strings.Contains(out, "[ ]") {
		t.Error("expected [ ] for hidden column")
	}
}

// TestColumnPicker_ZeroColumnError verifies the error message when no columns are selected.
func TestColumnPicker_ZeroColumnError(t *testing.T) {
	sel := []views.ColumnSelection{
		{Name: "id", Visible: false},
		{Name: "name", Visible: false},
	}
	p := views.NewColumnPickerModel(sel, sel, 80, 30)
	p = p.TryConfirm()
	if p.IsConfirmed() {
		t.Error("should not confirm with zero visible columns")
	}
	out := p.View()
	if !strings.Contains(out, "at least one column required") {
		t.Errorf("expected error message in view, got:\n%s", out)
	}
}

// TestColumnPicker_ToggleVisibility verifies that Space toggles the focused column.
func TestColumnPicker_ToggleVisibility(t *testing.T) {
	p := makePicker("id", "name")
	// Initially cursor is at "id" which is visible — toggle off.
	p = p.HandleKey(" ")
	sel := p.Selection()
	if sel[0].Visible {
		t.Error("id should be hidden after toggle")
	}
	// Toggle back on.
	p = p.HandleKey(" ")
	sel = p.Selection()
	if !sel[0].Visible {
		t.Error("id should be visible after second toggle")
	}
}

// TestColumnPicker_ReorderJ verifies that J moves the focused column down.
func TestColumnPicker_ReorderJ(t *testing.T) {
	p := makePicker("id", "name", "email")
	// cursor at 0 ("id"), J moves it down to position 1.
	p = p.HandleKey("J")
	sel := p.Selection()
	if sel[0].Name != "name" || sel[1].Name != "id" {
		t.Errorf("J should swap id and name, got: %v %v", sel[0].Name, sel[1].Name)
	}
}

// TestColumnPicker_ReorderK verifies that K moves the focused column up.
func TestColumnPicker_ReorderK(t *testing.T) {
	p := makePicker("id", "name", "email")
	// Move cursor to position 1 ("name"), then K moves it up.
	p = p.HandleKey("down")
	p = p.HandleKey("K")
	sel := p.Selection()
	if sel[0].Name != "name" || sel[1].Name != "id" {
		t.Errorf("K should swap name and id, got: %v %v", sel[0].Name, sel[1].Name)
	}
}

// TestColumnPicker_SelectAll verifies that 'a' makes all columns visible.
func TestColumnPicker_SelectAll(t *testing.T) {
	sel := []views.ColumnSelection{
		{Name: "id", Visible: false},
		{Name: "name", Visible: false},
	}
	p := views.NewColumnPickerModel(sel, sel, 80, 30)
	p = p.HandleKey("a")
	for _, s := range p.Selection() {
		if !s.Visible {
			t.Errorf("%s should be visible after 'a'", s.Name)
		}
	}
}

// TestColumnPicker_Reset verifies that 'r' restores the original schema order.
func TestColumnPicker_Reset(t *testing.T) {
	orig := makeSelections("id", "name", "email")
	// Start with a reordered / partially hidden selection.
	cur := []views.ColumnSelection{
		{Name: "email", Visible: true},
		{Name: "id", Visible: false},
		{Name: "name", Visible: true},
	}
	p := views.NewColumnPickerModel(orig, cur, 80, 30)
	p = p.HandleKey("r")
	sel := p.Selection()
	if len(sel) != 3 || sel[0].Name != "id" || sel[1].Name != "name" || sel[2].Name != "email" {
		t.Errorf("r should restore schema order, got: %v", sel)
	}
	for _, s := range sel {
		if !s.Visible {
			t.Errorf("%s should be visible after reset", s.Name)
		}
	}
}

// TestColumnPicker_ConfirmWithVisible verifies that Enter confirms when at least one column is visible.
func TestColumnPicker_ConfirmWithVisible(t *testing.T) {
	p := makePicker("id", "name")
	p = p.TryConfirm()
	if !p.IsConfirmed() {
		t.Error("expected IsConfirmed=true when columns are visible")
	}
}

// TestColumnPicker_Cancel verifies that cancel sets IsCancelled.
func TestColumnPicker_Cancel(t *testing.T) {
	p := makePicker("id", "name")
	p = p.Cancel()
	if !p.IsCancelled() {
		t.Error("expected IsCancelled=true after cancel")
	}
}

// TestColumnRegistry_Seed verifies that Seed initializes all columns visible.
func TestColumnRegistry_Seed(t *testing.T) {
	r := views.NewColumnRegistry()
	cols := []views.ColumnSelection{
		{Name: "id", Visible: true},
		{Name: "name", Visible: true},
	}
	r.SeedFromSelections("ds1", cols)
	sel := r.Get("ds1")
	if len(sel) != 2 || !sel[0].Visible || !sel[1].Visible {
		t.Errorf("expected all columns visible, got %v", sel)
	}
}

// TestColumnRegistry_VisibleColumns_Default verifies nil returned when all visible in schema order.
func TestColumnRegistry_VisibleColumns_Default(t *testing.T) {
	r := views.NewColumnRegistry()
	r.SeedFromSelections("ds1", makeSelections("id", "name"))
	cols := r.VisibleColumns("ds1")
	if cols != nil {
		t.Errorf("expected nil (SELECT *) for default selection, got %v", cols)
	}
}

// TestColumnRegistry_VisibleColumns_Projected verifies non-nil returned when columns are hidden.
func TestColumnRegistry_VisibleColumns_Projected(t *testing.T) {
	r := views.NewColumnRegistry()
	r.SeedFromSelections("ds1", makeSelections("id", "name", "payload"))
	r.Set("ds1", []views.ColumnSelection{
		{Name: "id", Visible: true},
		{Name: "name", Visible: true},
		{Name: "payload", Visible: false},
	})
	cols := r.VisibleColumns("ds1")
	if len(cols) != 2 || cols[0] != "id" || cols[1] != "name" {
		t.Errorf("expected [id name], got %v", cols)
	}
}

// TestColumnRegistry_IsDefault verifies IsDefault returns false when projection differs.
func TestColumnRegistry_IsDefault(t *testing.T) {
	r := views.NewColumnRegistry()
	r.SeedFromSelections("ds1", makeSelections("id", "name"))
	if !r.IsDefault("ds1") {
		t.Error("expected IsDefault=true for fresh seed")
	}
	r.Set("ds1", []views.ColumnSelection{
		{Name: "id", Visible: false},
		{Name: "name", Visible: true},
	})
	if r.IsDefault("ds1") {
		t.Error("expected IsDefault=false after hiding a column")
	}
}
