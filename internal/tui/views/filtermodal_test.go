package views_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/polesen/datacow/internal/core/dataset"
	"github.com/polesen/datacow/internal/core/db"
	"github.com/polesen/datacow/internal/tui/views"
)

func testCols() []db.Column {
	return []db.Column{
		{Name: "id", Type: "integer", Nullable: false},
		{Name: "name", Type: "text", Nullable: true},
		{Name: "status", Type: "text", Nullable: false},
		{Name: "created_at", Type: "timestamp", Nullable: true},
		{Name: "price", Type: "numeric", Nullable: true},
		{Name: "active", Type: "boolean", Nullable: false},
	}
}

func testDS() dataset.Dataset {
	return dataset.Dataset{Name: "orders", Table: "orders"}
}

func newModal(filters []dataset.Filter) views.FilterModalModel {
	return views.NewFilterModal(testDS(), testCols(), filters)
}

// --- Tests ---

func TestFilterModal_OpenWithExistingFilters(t *testing.T) {
	existing := []dataset.Filter{
		{Column: "status", Operator: "=", Value: "'active'"},
		{Column: "id", Operator: ">", Value: "100"},
	}
	m := newModal(existing)

	filters := m.Filters()
	if len(filters) != 2 {
		t.Fatalf("expected 2 filters, got %d", len(filters))
	}
	if filters[0].Column != "status" {
		t.Errorf("filter[0].Column = %q, want 'status'", filters[0].Column)
	}
}

func TestFilterModal_ApplyViaCtrlJ(t *testing.T) {
	m := newModal(nil)
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlJ})
	if !m2.IsApplied() {
		t.Error("expected IsApplied() after Ctrl+J")
	}
	if m2.IsCancelled() {
		t.Error("should not be cancelled")
	}
}

func TestFilterModal_ApplyViaEnterAfterAddingFilter(t *testing.T) {
	// Simulate the natural flow: add a filter, then press Enter to apply.
	m := newModal(nil)

	// Type column "id", Tab to Op, Tab to Value, type "42", Enter to add filter.
	for _, r := range "id" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	for _, r := range "42" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // adds filter, returns to column field

	if len(m.Filters()) != 1 {
		t.Fatalf("expected 1 filter after entry, got %d", len(m.Filters()))
	}
	if m.IsApplied() {
		t.Error("should not be applied yet — filter was just added")
	}

	// Second Enter in the empty column field should apply the modal.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !m.IsApplied() {
		t.Error("expected IsApplied() after Enter with empty column and no filter selected")
	}
}

func TestFilterModal_CancelViaEsc(t *testing.T) {
	m := newModal(nil)
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if !m2.IsCancelled() {
		t.Error("expected IsCancelled() after Esc")
	}
	if m2.IsApplied() {
		t.Error("should not be applied")
	}
}

func TestFilterModal_AddFilter_ValidColumn(t *testing.T) {
	m := newModal(nil)

	// Type column name "id"
	for _, r := range "id" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	// Tab to Op
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	// Tab to Value
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	// Type "42"
	for _, r := range "42" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	// Enter to submit
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	filters := m.Filters()
	if len(filters) != 1 {
		t.Fatalf("expected 1 filter, got %d", len(filters))
	}
	if filters[0].Column != "id" {
		t.Errorf("filter.Column = %q, want 'id'", filters[0].Column)
	}
	if filters[0].Operator != "=" {
		t.Errorf("filter.Operator = %q, want '='", filters[0].Operator)
	}
}

func TestFilterModal_AddFilter_UnknownColumn_ShowsError(t *testing.T) {
	m := newModal(nil)

	for _, r := range "nonexistent_col" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	for _, r := range "foo" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if len(m.Filters()) != 0 {
		t.Errorf("expected 0 filters after invalid column, got %d", len(m.Filters()))
	}

	m.SetWidth(80)
	v := m.View()
	if !strings.Contains(v, "unknown") {
		t.Errorf("expected validation error in view, got: %s", v)
	}
}

func TestFilterModal_DeleteFilter(t *testing.T) {
	existing := []dataset.Filter{
		{Column: "status", Operator: "=", Value: "'active'"},
	}
	m := newModal(existing)
	// Column input is empty and focused, 'd' deletes the selected filter
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})

	if len(m.Filters()) != 0 {
		t.Errorf("expected 0 filters after delete, got %d", len(m.Filters()))
	}
}

func TestFilterModal_EditFilter_LoadsIntoForm(t *testing.T) {
	existing := []dataset.Filter{
		{Column: "id", Operator: "=", Value: "1"},
	}
	m := newModal(existing)

	// Enter with listCursor=0 and empty column field → load filter
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// Advance to Value and replace
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab}) // Column → Op
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab}) // Op → Value
	// Clear and type new value
	for i := 0; i < 5; i++ {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	}
	for _, r := range "999" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	filters := m.Filters()
	if len(filters) != 1 {
		t.Fatalf("expected 1 filter (replaced), got %d", len(filters))
	}
	if filters[0].Value != "999" {
		t.Errorf("expected replaced value '999', got %v", filters[0].Value)
	}
}

func TestFilterModal_OpCyclesForTextType(t *testing.T) {
	m := newModal(nil)

	// "name" is a text column → ops: = like
	for _, r := range "name" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab}) // to Op

	// Right cycles op
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab}) // to Value
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	filters := m.Filters()
	if len(filters) == 1 && filters[0].Operator != "like" {
		t.Errorf("expected operator 'like', got %q", filters[0].Operator)
	}
}

func TestFilterModal_IntegerRejectsLetters(t *testing.T) {
	m := newModal(nil)

	for _, r := range "id" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})

	// Letters should be rejected (including 'd', which previously bypassed the guard)
	for _, r := range "ad" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	// Type a digit — should be accepted
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'5'}})

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	filters := m.Filters()
	if len(filters) == 1 {
		val := filters[0].Value.(string)
		if strings.ContainsAny(val, "ad") {
			t.Errorf("integer field accepted letters, got value: %v", val)
		}
	}
}

func TestFilterModal_QuickFilter_Prefilled(t *testing.T) {
	m := views.NewFilterModalQuickFilter(testDS(), testCols(), nil, "status", "'active'")

	m.SetWidth(80)
	v := m.View()
	if !strings.Contains(v, "status") {
		t.Error("view should contain column name 'status'")
	}
	if !strings.Contains(v, "active") {
		t.Error("view should contain pre-filled value")
	}
}

func TestFilterModal_View_Sections(t *testing.T) {
	m := newModal(nil)
	m.SetWidth(80)
	v := m.View()

	for _, want := range []string{"Query Filter", "Active filters", "Edit / add filter"} {
		if !strings.Contains(v, want) {
			t.Errorf("View() missing section %q", want)
		}
	}
}

func TestFilterModal_Filters_IndependentCopy(t *testing.T) {
	existing := []dataset.Filter{{Column: "id", Operator: "=", Value: "1"}}
	m := newModal(existing)
	filters := m.Filters()
	filters[0].Column = "mutated"
	// Modal's internal state should be unaffected
	if m.Filters()[0].Column != "id" {
		t.Error("Filters() should return an independent copy")
	}
}
