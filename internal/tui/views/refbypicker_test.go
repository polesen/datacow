package views_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/polesen/datacow/internal/core/schema"
	"github.com/polesen/datacow/internal/tui/views"
)

func makePickerEntries() []schema.InboundFK {
	return []schema.InboundFK{
		{FromTable: "orders", FromColumn: "customer_id", ToColumn: "id"},
		{FromTable: "invoices", FromColumn: "customer_id", ToColumn: "id"},
		{FromTable: "subscriptions", FromColumn: "customer_id", ToColumn: "id"},
	}
}

func sizedPicker(entries []schema.InboundFK, srcTable, srcCol, cellValue string) views.RefByPickerModel {
	m := views.NewRefByPickerModel(entries, srcTable, srcCol, cellValue)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m, _ = m.Focus()
	return m
}

func TestRefByPickerModel_RenderHeader(t *testing.T) {
	m := sizedPicker(makePickerEntries(), "customers", "id", "42")
	v := m.View()
	if !strings.Contains(v, "Referenced by") {
		t.Errorf("view must contain 'Referenced by', got:\n%s", v)
	}
	if !strings.Contains(v, "customers.id") {
		t.Errorf("view must show source table.column 'customers.id', got:\n%s", v)
	}
	if !strings.Contains(v, "42") {
		t.Errorf("view must show cell value '42', got:\n%s", v)
	}
}

func TestRefByPickerModel_ListAlphabetical(t *testing.T) {
	m := sizedPicker(makePickerEntries(), "customers", "id", "1")
	v := m.View()
	// All three entries should appear.
	if !strings.Contains(v, "invoices.customer_id") {
		t.Errorf("view must list invoices.customer_id, got:\n%s", v)
	}
	if !strings.Contains(v, "orders.customer_id") {
		t.Errorf("view must list orders.customer_id, got:\n%s", v)
	}
	if !strings.Contains(v, "subscriptions.customer_id") {
		t.Errorf("view must list subscriptions.customer_id, got:\n%s", v)
	}

	// Verify alphabetical order: invoices < orders < subscriptions.
	idxInvoices := strings.Index(v, "invoices")
	idxOrders := strings.Index(v, "orders")
	idxSubs := strings.Index(v, "subscriptions")
	if idxInvoices < 0 || idxOrders < 0 || idxSubs < 0 {
		t.Fatal("one or more entries missing from view")
	}
	if idxInvoices > idxOrders || idxOrders > idxSubs {
		t.Errorf("entries not in alphabetical order: invoices=%d orders=%d subscriptions=%d", idxInvoices, idxOrders, idxSubs)
	}
}

func TestRefByPickerModel_FilterNarrowsList(t *testing.T) {
	m := sizedPicker(makePickerEntries(), "customers", "id", "1")

	// Type "inv" — should match only invoices.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("inv")})
	v := m.View()
	if !strings.Contains(v, "invoices") {
		t.Errorf("'inv' should match invoices, got:\n%s", v)
	}
	if strings.Contains(v, "orders") {
		t.Errorf("'inv' should not match orders, got:\n%s", v)
	}
	if strings.Contains(v, "subscriptions") {
		t.Errorf("'inv' should not match subscriptions, got:\n%s", v)
	}
}

func TestRefByPickerModel_FilterCaseInsensitive(t *testing.T) {
	m := sizedPicker(makePickerEntries(), "customers", "id", "1")
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("ORDERS")})
	v := m.View()
	if !strings.Contains(v, "orders") {
		t.Errorf("filter must be case-insensitive, got:\n%s", v)
	}
}

func TestRefByPickerModel_EmptyFilterShowsNoMatches(t *testing.T) {
	m := sizedPicker(makePickerEntries(), "customers", "id", "1")
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("zzzzz")})
	v := m.View()
	if !strings.Contains(v, "No matches") {
		t.Errorf("view must show 'No matches' when filter yields nothing, got:\n%s", v)
	}
}

func TestRefByPickerModel_EnterSelectsItem(t *testing.T) {
	m := sizedPicker(makePickerEntries(), "customers", "id", "1")

	// Cursor starts at index 0 (invoices after alpha sort).
	if m.IsSelected() {
		t.Fatal("must not be selected before Enter")
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !m.IsSelected() {
		t.Fatal("IsSelected must be true after Enter")
	}
	sel := m.Selection()
	if sel.FromTable != "invoices" {
		t.Errorf("selection FromTable = %q, want %q", sel.FromTable, "invoices")
	}
}

func TestRefByPickerModel_EscCancels(t *testing.T) {
	m := sizedPicker(makePickerEntries(), "customers", "id", "1")
	if m.IsCancelled() {
		t.Fatal("must not be cancelled before Esc")
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if !m.IsCancelled() {
		t.Fatal("IsCancelled must be true after Esc")
	}
	if m.IsSelected() {
		t.Error("must not be selected after Esc")
	}
}

func TestRefByPickerModel_UpDownMoveCursor(t *testing.T) {
	m := sizedPicker(makePickerEntries(), "customers", "id", "1")
	// Default cursor is 0 (invoices). Move down to orders.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !m.IsSelected() {
		t.Fatal("IsSelected must be true after Enter")
	}
	sel := m.Selection()
	if sel.FromTable != "orders" {
		t.Errorf("selection after Down: FromTable = %q, want %q", sel.FromTable, "orders")
	}
}

func TestRefByPickerModel_KJNavigation(t *testing.T) {
	m := sizedPicker(makePickerEntries(), "customers", "id", "1")
	// Press j (down) then k (up) to verify we stay at cursor 0.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	sel := m.Selection()
	if sel.FromTable != "invoices" {
		t.Errorf("k navigation: selection = %q, want invoices", sel.FromTable)
	}
}

func TestRefByPickerModel_EmptyEntriesNoSelect(t *testing.T) {
	m := sizedPicker(nil, "customers", "id", "1")
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.IsSelected() {
		t.Error("Enter on empty list must not select anything")
	}
}

func TestRefByPickerModel_SortedAlpha(t *testing.T) {
	// Create entries in non-alphabetical order; picker must sort them.
	entries := []schema.InboundFK{
		{FromTable: "zebra", FromColumn: "x", ToColumn: "id"},
		{FromTable: "apple", FromColumn: "x", ToColumn: "id"},
		{FromTable: "mango", FromColumn: "x", ToColumn: "id"},
	}
	m := sizedPicker(entries, "t", "id", "1")
	// First item selected by default (index 0 = apple after sort).
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !m.IsSelected() {
		t.Fatal("IsSelected must be true")
	}
	if m.Selection().FromTable != "apple" {
		t.Errorf("first entry after sort must be apple, got %q", m.Selection().FromTable)
	}
}
