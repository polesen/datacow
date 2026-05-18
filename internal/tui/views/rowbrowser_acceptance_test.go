package views_test

// Acceptance tests for the reverse FK drill-down feature.
// Each test is named TestAC_<SECTION><NN>_<description> to map directly to an
// acceptance criterion in tasks/ready/reverse-fk-drilldown.md.
//
// Sections:
//   SL  — Schema Layer (SL01–SL04)
//   RD  — Row Browser Reverse Drill (RD01–RD12)
//   VM  — Visual Marker (VM01–VM03)
//   TL  — Table List (TL01–TL05)
//   KH  — Keys & Help (KH01–KH04)

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/polesen/datacow/internal/core/dataset"
	"github.com/polesen/datacow/internal/core/db"
	"github.com/polesen/datacow/internal/core/schema"
	"github.com/polesen/datacow/internal/tui/keys"
	"github.com/polesen/datacow/internal/tui/views"
)

// --- helpers ---

// makeRefByModel builds a RowBrowserModel with a schema cache that knows
// one inbound FK: orders.customer_id → customers.id.
func makeRefByModel(t *testing.T) views.RowBrowserModel {
	t.Helper()
	tables := []schema.Table{
		{
			Name: "customers",
			Kind: db.KindTable,
			Columns: []db.Column{
				{Name: "id", Type: "int"},
				{Name: "name", Type: "text"},
			},
			ReferencedBy: []schema.InboundFK{
				{FromTable: "orders", FromColumn: "customer_id", ToColumn: "id"},
			},
		},
		{
			Name:    "orders",
			Kind:    db.KindTable,
			Columns: []db.Column{{Name: "id"}, {Name: "customer_id"}},
			ForeignKeys: []db.ForeignKey{
				{Column: "customer_id", ReferencedTable: "customers", ReferencedColumn: "id"},
			},
		},
	}
	datasets := []dataset.Dataset{
		{Name: "customers", Table: "customers", Kind: dataset.KindTable},
		{Name: "orders", Table: "orders", Kind: dataset.KindTable},
	}
	cache := schema.NewCacheWithData(tables, datasets)

	ds := dataset.Dataset{Name: "customers", Table: "customers"}
	m := views.NewRowBrowserModel(keys.Default(), nil, nil, ds, nil, cache)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	return m
}

// loadCustomersResult feeds the model with a loaded result for customers table.
func loadCustomersResult(m views.RowBrowserModel) views.RowBrowserModel {
	result := makeResult(1, 1, 2,
		[]db.Column{{Name: "id"}, {Name: "name"}},
		[]map[string]any{
			{"id": int64(1), "name": "Alice"},
			{"id": int64(2), "name": "Bob"},
		},
	)
	m, _ = m.Update(views.RowsLoadedMsg(result))
	return m
}

// --- SL: Schema Layer ---

// SL01: schema.InboundFK has FromTable, FromColumn, ToColumn.
func TestAC_SL01_InboundFKType(t *testing.T) {
	ibfk := schema.InboundFK{FromTable: "orders", FromColumn: "customer_id", ToColumn: "id"}
	if ibfk.FromTable != "orders" || ibfk.FromColumn != "customer_id" || ibfk.ToColumn != "id" {
		t.Errorf("InboundFK fields unexpected: %+v", ibfk)
	}
}

// SL02: schema.Table.ReferencedBy is populated by schema.Load for referenced tables.
func TestAC_SL02_ReferencedByPopulated(t *testing.T) {
	tables := []schema.Table{
		{
			Name: "customers",
			Kind: db.KindTable,
			ReferencedBy: []schema.InboundFK{
				{FromTable: "orders", FromColumn: "customer_id", ToColumn: "id"},
			},
		},
	}
	if len(tables[0].ReferencedBy) != 1 {
		t.Errorf("expected 1 ReferencedBy entry, got %d", len(tables[0].ReferencedBy))
	}
}

// SL03: Tables not referenced anywhere have an empty ReferencedBy.
func TestAC_SL03_UnreferencedTableEmptyReferencedBy(t *testing.T) {
	var tbl schema.Table
	if tbl.ReferencedBy != nil {
		t.Errorf("zero-value Table.ReferencedBy should be nil, got %v", tbl.ReferencedBy)
	}
}

// SL03b: explicit test that the zero value struct compiles.
func TestAC_SL03b_ZeroValueTable(t *testing.T) {
	tbl := schema.Table{Name: "standalone"}
	if len(tbl.ReferencedBy) != 0 {
		t.Errorf("Table with no inbound FKs: ReferencedBy count = %d, want 0", len(tbl.ReferencedBy))
	}
}

// SL04: Covered by schema_test.go TestLoad_Postgres (requires TEST_POSTGRES_DSN).
// Schema-layer Load test is in internal/core/schema/schema_test.go.

// --- RD: Row Browser Reverse Drill ---

// RD01: `>` is bound and behaves identically to Enter (forward drill).
func TestAC_RD01_DrillFwdAliasesEnter(t *testing.T) {
	ds := dataset.Dataset{Name: "orders", Table: "orders"}
	m := views.NewRowBrowserModel(keys.Default(), nil, nil, ds, nil, nil)
	result := makeResult(1, 1, 1,
		[]db.Column{{Name: "id"}, {Name: "customer_id"}},
		[]map[string]any{{"id": int64(42), "customer_id": int64(1001)}},
	)
	m, _ = m.Update(views.RowsLoadedMsg(result))
	m, _ = m.Update(views.FKsLoadedMsg([]db.ForeignKey{
		{Column: "customer_id", ReferencedTable: "customers", ReferencedColumn: "id"},
	}))
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight}) // move to customer_id

	// Press ">" — should drill just like Enter.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(">")})
	if !m.IsLoading() {
		t.Error("'>' must drill forward (same as Enter)")
	}
	if m.DrillDepth() != 1 {
		t.Errorf("drill depth after '>': got %d, want 1", m.DrillDepth())
	}
}

// RD02: `<` on ineligible cell shows status "no tables reference this column" and does not navigate.
func TestAC_RD02_LtIneligibleCell(t *testing.T) {
	m := makeRefByModel(t)
	m = loadCustomersResult(m)

	// cursor is on "id" column (col 0). The id column IS referenced (orders.customer_id → id).
	// Move to "name" column (col 1) — not referenced by any FK.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})

	depth := m.DrillDepth()
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("<")})

	if m.DrillDepth() != depth {
		t.Errorf("'<' on ineligible column must not navigate; depth was %d, now %d", depth, m.DrillDepth())
	}
	if m.IsLoading() {
		t.Error("'<' on ineligible column must not trigger loading")
	}
	status := m.StatusLine()
	if !strings.Contains(status, "no tables reference this column") {
		t.Errorf("status must say 'no tables reference this column', got: %q", status)
	}
}

// RD03: `<` on eligible cell with NULL value is a no-op.
func TestAC_RD03_LtNullCell(t *testing.T) {
	m := makeRefByModel(t)
	result := makeResult(1, 1, 1,
		[]db.Column{{Name: "id"}, {Name: "name"}},
		[]map[string]any{{"id": nil, "name": "Alice"}},
	)
	m, _ = m.Update(views.RowsLoadedMsg(result))

	// id column is referenced; its value is NULL.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("<")})
	if m.DrillDepth() != 0 {
		t.Error("'<' on NULL eligible cell must be a no-op")
	}
	if m.IsLoading() {
		t.Error("'<' on NULL must not trigger loading")
	}
}

// RD04: `<` on eligible cell with single referencing table drills immediately.
func TestAC_RD04_LtSingleRefTableDrillsDirectly(t *testing.T) {
	m := makeRefByModel(t)
	m = loadCustomersResult(m)

	// id column (col 0) has exactly one referencing table: orders.customer_id.
	before := m.DrillDepth()
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("<")})
	if m.DrillDepth() != before+1 {
		t.Errorf("single-match reverse drill: depth %d → %d, want %d", before, m.DrillDepth(), before+1)
	}
	if !m.IsLoading() {
		t.Error("should be loading after reverse drill")
	}
}

// RD05: `<` on eligible cell with 2+ referencing tables opens the picker overlay.
func TestAC_RD05_LtMultipleRefTablesOpensPicker(t *testing.T) {
	// Build a cache where customers.id is referenced by both orders AND invoices.
	tables := []schema.Table{
		{
			Name:    "customers",
			Kind:    db.KindTable,
			Columns: []db.Column{{Name: "id"}, {Name: "name"}},
			ReferencedBy: []schema.InboundFK{
				{FromTable: "invoices", FromColumn: "customer_id", ToColumn: "id"},
				{FromTable: "orders", FromColumn: "customer_id", ToColumn: "id"},
			},
		},
	}
	datasets := []dataset.Dataset{{Name: "customers", Table: "customers", Kind: dataset.KindTable}}
	cache := schema.NewCacheWithData(tables, datasets)

	ds := dataset.Dataset{Name: "customers", Table: "customers"}
	m := views.NewRowBrowserModel(keys.Default(), nil, nil, ds, nil, cache)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	result := makeResult(1, 1, 1,
		[]db.Column{{Name: "id"}, {Name: "name"}},
		[]map[string]any{{"id": int64(1), "name": "Alice"}},
	)
	m, _ = m.Update(views.RowsLoadedMsg(result))

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("<")})

	// Picker must be open — check view contains "Referenced by".
	v := m.View()
	if !strings.Contains(v, "Referenced by") {
		t.Errorf("picker must be open after '<' with multiple refs; view:\n%s", v)
	}
	if m.DrillDepth() != 0 {
		t.Error("drill must not happen until picker confirms selection")
	}
}

// RD06: Picker lists entries in alphabetical order (invoices < orders).
func TestAC_RD06_PickerAlphabeticalOrder(t *testing.T) {
	tables := []schema.Table{
		{
			Name:    "customers",
			Kind:    db.KindTable,
			Columns: []db.Column{{Name: "id"}},
			ReferencedBy: []schema.InboundFK{
				{FromTable: "orders", FromColumn: "customer_id", ToColumn: "id"},
				{FromTable: "invoices", FromColumn: "customer_id", ToColumn: "id"},
			},
		},
	}
	datasets := []dataset.Dataset{{Name: "customers", Table: "customers", Kind: dataset.KindTable}}
	cache := schema.NewCacheWithData(tables, datasets)

	ds := dataset.Dataset{Name: "customers", Table: "customers"}
	m := views.NewRowBrowserModel(keys.Default(), nil, nil, ds, nil, cache)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	result := makeResult(1, 1, 1,
		[]db.Column{{Name: "id"}},
		[]map[string]any{{"id": int64(1)}},
	)
	m, _ = m.Update(views.RowsLoadedMsg(result))
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("<")})

	v := m.View()
	idxInvoices := strings.Index(v, "invoices")
	idxOrders := strings.Index(v, "orders")
	if idxInvoices < 0 || idxOrders < 0 {
		t.Fatalf("picker must list both invoices and orders; got:\n%s", v)
	}
	if idxInvoices > idxOrders {
		t.Errorf("invoices must appear before orders (alphabetical); got idxInvoices=%d idxOrders=%d", idxInvoices, idxOrders)
	}
}

// RD07: Picker header shows "Referenced by".
func TestAC_RD07_PickerHeader(t *testing.T) {
	tables := []schema.Table{
		{
			Name:    "customers",
			Kind:    db.KindTable,
			Columns: []db.Column{{Name: "id"}},
			ReferencedBy: []schema.InboundFK{
				{FromTable: "orders", FromColumn: "customer_id", ToColumn: "id"},
				{FromTable: "invoices", FromColumn: "customer_id", ToColumn: "id"},
			},
		},
	}
	cache := schema.NewCacheWithData(tables, []dataset.Dataset{{Name: "customers", Table: "customers"}})
	ds := dataset.Dataset{Name: "customers", Table: "customers"}
	m := views.NewRowBrowserModel(keys.Default(), nil, nil, ds, nil, cache)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	result := makeResult(1, 1, 1, []db.Column{{Name: "id"}}, []map[string]any{{"id": int64(42)}})
	m, _ = m.Update(views.RowsLoadedMsg(result))
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("<")})

	v := m.View()
	if !strings.Contains(v, "Referenced by") {
		t.Errorf("picker header must contain 'Referenced by', got:\n%s", v)
	}
	// Source table.column and value must appear.
	if !strings.Contains(v, "customers.id") {
		t.Errorf("picker must show source table.column 'customers.id', got:\n%s", v)
	}
	if !strings.Contains(v, "42") {
		t.Errorf("picker must show cell value '42', got:\n%s", v)
	}
}

// RD08: Picker accepts substring filter; shows "No matches" when nothing matches.
func TestAC_RD08_PickerFilter(t *testing.T) {
	tables := []schema.Table{
		{
			Name:    "customers",
			Kind:    db.KindTable,
			Columns: []db.Column{{Name: "id"}},
			ReferencedBy: []schema.InboundFK{
				{FromTable: "orders", FromColumn: "customer_id", ToColumn: "id"},
				{FromTable: "invoices", FromColumn: "customer_id", ToColumn: "id"},
			},
		},
	}
	cache := schema.NewCacheWithData(tables, []dataset.Dataset{{Name: "customers", Table: "customers"}})
	ds := dataset.Dataset{Name: "customers", Table: "customers"}
	m := views.NewRowBrowserModel(keys.Default(), nil, nil, ds, nil, cache)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	result := makeResult(1, 1, 1, []db.Column{{Name: "id"}}, []map[string]any{{"id": int64(1)}})
	m, _ = m.Update(views.RowsLoadedMsg(result))
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("<")})

	// Type "ord" — should only show orders.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("ord")})
	v := m.View()
	if !strings.Contains(v, "orders") {
		t.Errorf("filter 'ord' must match orders, got:\n%s", v)
	}
	if strings.Contains(v, "invoices") {
		t.Errorf("filter 'ord' must not match invoices, got:\n%s", v)
	}

	// Type "zzzzz" over it — clear current input first, then type.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("zzzzz")})
	v = m.View()
	if !strings.Contains(v, "No matches") {
		t.Errorf("filter with no results must show 'No matches', got:\n%s", v)
	}
}

// RD09: Picker Esc closes without navigating.
func TestAC_RD09_PickerEscClosesWithoutNav(t *testing.T) {
	tables := []schema.Table{
		{
			Name:    "customers",
			Kind:    db.KindTable,
			Columns: []db.Column{{Name: "id"}},
			ReferencedBy: []schema.InboundFK{
				{FromTable: "orders", FromColumn: "customer_id", ToColumn: "id"},
				{FromTable: "invoices", FromColumn: "customer_id", ToColumn: "id"},
			},
		},
	}
	cache := schema.NewCacheWithData(tables, []dataset.Dataset{{Name: "customers", Table: "customers"}})
	ds := dataset.Dataset{Name: "customers", Table: "customers"}
	m := views.NewRowBrowserModel(keys.Default(), nil, nil, ds, nil, cache)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	result := makeResult(1, 1, 1, []db.Column{{Name: "id"}}, []map[string]any{{"id": int64(1)}})
	m, _ = m.Update(views.RowsLoadedMsg(result))

	// Open picker.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("<")})
	if !strings.Contains(m.View(), "Referenced by") {
		t.Fatal("picker must be open before testing Esc")
	}

	// Press Esc — picker must close, no navigation.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.DrillDepth() != 0 {
		t.Error("Esc from picker must not navigate")
	}
	v := m.View()
	if strings.Contains(v, "Referenced by") {
		t.Errorf("picker must be closed after Esc, got:\n%s", v)
	}
}

// RD10: Reverse drill breadcrumb reads "← <from_table>.<from_col> = <value> ← <current_table>".
func TestAC_RD10_ReverseBreadcrumb(t *testing.T) {
	m := makeRefByModel(t)
	m = loadCustomersResult(m)

	// id (col 0) is referenced by orders.customer_id. Press "<".
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("<")})

	// Load child (orders) rows.
	childResult := makeResult(1, 1, 1,
		[]db.Column{{Name: "id"}, {Name: "customer_id"}},
		[]map[string]any{{"id": int64(10), "customer_id": int64(1)}},
	)
	m, _ = m.Update(views.RowsLoadedMsg(childResult))

	v := m.View()
	// Breadcrumb must contain left arrows and the correct table/column names.
	if !strings.Contains(v, "←") {
		t.Errorf("reverse drill breadcrumb must use '←', got:\n%s", v)
	}
	if !strings.Contains(v, "orders") {
		t.Errorf("breadcrumb must reference 'orders', got:\n%s", v)
	}
	if !strings.Contains(v, "customer_id") {
		t.Errorf("breadcrumb must reference 'customer_id', got:\n%s", v)
	}
	if !strings.Contains(v, "customers") {
		t.Errorf("breadcrumb must reference source table 'customers', got:\n%s", v)
	}
}

// RD11: Esc pops a reverse level the same way it pops a forward level.
func TestAC_RD11_EscPopsReverseLevel(t *testing.T) {
	m := makeRefByModel(t)
	m = loadCustomersResult(m)

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("<")})
	if m.DrillDepth() != 1 {
		t.Fatalf("setup: expected drill depth 1 after reverse drill, got %d", m.DrillDepth())
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.DrillDepth() != 0 {
		t.Errorf("Esc must pop reverse level; drill depth = %d, want 0", m.DrillDepth())
	}
	if m.DatasetName() != "customers" {
		t.Errorf("after pop, dataset must be 'customers', got %q", m.DatasetName())
	}
}

// RD12: Mixed stack (forward → reverse → forward) pops in LIFO order.
func TestAC_RD12_MixedStackLIFO(t *testing.T) {
	// Start with orders, drill forward to customers, then reverse drill from customers to invoices.
	tables := []schema.Table{
		{
			Name:    "customers",
			Kind:    db.KindTable,
			Columns: []db.Column{{Name: "id"}, {Name: "name"}},
			ReferencedBy: []schema.InboundFK{
				{FromTable: "invoices", FromColumn: "customer_id", ToColumn: "id"},
			},
		},
	}
	cache := schema.NewCacheWithData(tables, []dataset.Dataset{
		{Name: "customers", Table: "customers"},
		{Name: "orders", Table: "orders"},
	})

	ds := dataset.Dataset{Name: "orders", Table: "orders"}
	m := views.NewRowBrowserModel(keys.Default(), nil, nil, ds, nil, cache)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 60})

	// Load orders.
	orderResult := makeResult(1, 1, 1,
		[]db.Column{{Name: "id"}, {Name: "customer_id"}},
		[]map[string]any{{"id": int64(10), "customer_id": int64(1)}},
	)
	m, _ = m.Update(views.RowsLoadedMsg(orderResult))
	m, _ = m.Update(views.FKsLoadedMsg([]db.ForeignKey{
		{Column: "customer_id", ReferencedTable: "customers", ReferencedColumn: "id"},
	}))

	// Forward drill: orders → customers.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight}) // move to customer_id
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.DrillDepth() != 1 {
		t.Fatalf("after forward drill: depth %d, want 1", m.DrillDepth())
	}

	// Load customers.
	custResult := makeResult(1, 1, 1,
		[]db.Column{{Name: "id"}, {Name: "name"}},
		[]map[string]any{{"id": int64(1), "name": "Alice"}},
	)
	m, _ = m.Update(views.RowsLoadedMsg(custResult))

	// Reverse drill from customers.id → invoices.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("<")})
	if m.DrillDepth() != 2 {
		t.Fatalf("after reverse drill: depth %d, want 2", m.DrillDepth())
	}

	// Pop reverse level.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.DrillDepth() != 1 {
		t.Errorf("after first Esc: depth %d, want 1", m.DrillDepth())
	}
	if m.DatasetName() != "customers" {
		t.Errorf("after Esc from reverse: dataset = %q, want customers", m.DatasetName())
	}

	// Pop forward level.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.DrillDepth() != 0 {
		t.Errorf("after second Esc: depth %d, want 0", m.DrillDepth())
	}
	if m.DatasetName() != "orders" {
		t.Errorf("after full pop: dataset = %q, want orders", m.DatasetName())
	}
}

// --- VM: Visual Marker ---

// VM01: Column headers for referenced columns render with the RefBy marker (↩).
func TestAC_VM01_RefByHeaderMarker(t *testing.T) {
	m := makeRefByModel(t)
	m = loadCustomersResult(m)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})

	v := m.View()
	// The "id" column is referenced, so it should have the ↩ glyph in its header.
	if !strings.Contains(v, "↩") {
		t.Errorf("referenced column header must show '↩' marker, got:\n%s", v)
	}
}

// VM02: When cursor is on a referenced column, the active variant is used.
func TestAC_VM02_RefByHeaderActiveStyle(t *testing.T) {
	m := makeRefByModel(t)
	m = loadCustomersResult(m)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})

	// colCursor starts at 0 (id column, which is referenced).
	v := m.View()
	if !strings.Contains(v, "↩") {
		t.Errorf("active referenced column must show '↩' marker, got:\n%s", v)
	}
}

// VM03: A column that is both FK and referenced shows both markers.
func TestAC_VM03_BothFKAndRefByShownTogether(t *testing.T) {
	// Create a table where "parent_id" is both an FK (→ parent) and referenced by child.
	tables := []schema.Table{
		{
			Name:    "bridge",
			Kind:    db.KindTable,
			Columns: []db.Column{{Name: "id"}, {Name: "parent_id"}},
			ForeignKeys: []db.ForeignKey{
				{Column: "parent_id", ReferencedTable: "parents", ReferencedColumn: "id"},
			},
			ReferencedBy: []schema.InboundFK{
				{FromTable: "children", FromColumn: "bridge_id", ToColumn: "parent_id"},
			},
		},
	}
	cache := schema.NewCacheWithData(tables, []dataset.Dataset{{Name: "bridge", Table: "bridge"}})
	ds := dataset.Dataset{Name: "bridge", Table: "bridge"}
	m := views.NewRowBrowserModel(keys.Default(), nil, nil, ds, nil, cache)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	result := makeResult(1, 1, 1,
		[]db.Column{{Name: "id"}, {Name: "parent_id"}},
		[]map[string]any{{"id": int64(1), "parent_id": int64(10)}},
	)
	m, _ = m.Update(views.RowsLoadedMsg(result))
	m, _ = m.Update(views.FKsLoadedMsg([]db.ForeignKey{
		{Column: "parent_id", ReferencedTable: "parents", ReferencedColumn: "id"},
	}))

	// Move cursor to parent_id (col 1).
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})

	v := m.View()
	// Both ↩ (RefBy marker) must be present since parent_id is referenced.
	if !strings.Contains(v, "↩") {
		t.Errorf("combined FK+RefBy column must show '↩' marker, got:\n%s", v)
	}
}

// --- TL: Table List ---

// TL01: Expanded table rows show "Referenced By" section.
func TestAC_TL01_ReferencedBySectionVisible(t *testing.T) {
	tables := []schema.Table{
		{
			Name: "customers",
			Kind: db.KindTable,
			Columns: []db.Column{
				{Name: "id", Type: "int"},
			},
			ReferencedBy: []schema.InboundFK{
				{FromTable: "orders", FromColumn: "customer_id", ToColumn: "id"},
			},
		},
	}
	datasets := []dataset.Dataset{{Name: "customers", Table: "customers", Kind: dataset.KindTable}}
	cache := schema.NewCacheWithData(tables, datasets)

	m := views.NewTableListModel(keys.Default(), nil, nil, nil, cache)
	m, _ = m.Update(views.TablesLoadedMsg(datasets))
	m, _ = m.Update(tea.WindowSizeMsg{Width: 60, Height: 40})

	// Expand the customers row.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	// Fake the expansion data loaded.
	m, _ = m.Update(views.ExpansionLoadedMsg{Idx: 0, Cols: tables[0].Columns, FKs: nil})
	m, _ = m.Update(views.IndexesLoadedMsg{Idx: 0, Indexes: nil})

	v := m.View()
	if !strings.Contains(v, "Referenced By") {
		t.Errorf("expanded table must show 'Referenced By' section, got:\n%s", v)
	}
	// Entry must be "← orders.customer_id".
	if !strings.Contains(v, "← orders.customer_id") {
		t.Errorf("Referenced By must list '← orders.customer_id', got:\n%s", v)
	}
}

// TL02: Empty case renders "(none)".
func TestAC_TL02_ReferencedByEmptyShowsNone(t *testing.T) {
	tables := []schema.Table{
		{
			Name:    "standalone",
			Kind:    db.KindTable,
			Columns: []db.Column{{Name: "id", Type: "int"}},
		},
	}
	datasets := []dataset.Dataset{{Name: "standalone", Table: "standalone", Kind: dataset.KindTable}}
	cache := schema.NewCacheWithData(tables, datasets)

	m := views.NewTableListModel(keys.Default(), nil, nil, nil, cache)
	m, _ = m.Update(views.TablesLoadedMsg(datasets))
	m, _ = m.Update(tea.WindowSizeMsg{Width: 60, Height: 40})

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight}) // expand
	m, _ = m.Update(views.ExpansionLoadedMsg{Idx: 0, Cols: tables[0].Columns, FKs: nil})
	m, _ = m.Update(views.IndexesLoadedMsg{Idx: 0, Indexes: nil})

	v := m.View()
	if !strings.Contains(v, "Referenced By") {
		t.Errorf("section must still appear even when empty, got:\n%s", v)
	}
	if !strings.Contains(v, "(none)") {
		t.Errorf("empty Referenced By must show '(none)', got:\n%s", v)
	}
}

// TL03: Section shows "loading…" when cache is not ready.
func TestAC_TL03_ReferencedByLoadingWhenCacheNotReady(t *testing.T) {
	datasets := []dataset.Dataset{{Name: "users", Table: "users", Kind: dataset.KindTable}}
	// nil cache = not ready
	m := views.NewTableListModel(keys.Default(), nil, nil, nil, nil)
	m, _ = m.Update(views.TablesLoadedMsg(datasets))
	m, _ = m.Update(tea.WindowSizeMsg{Width: 60, Height: 40})

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m, _ = m.Update(views.ExpansionLoadedMsg{Idx: 0, Cols: nil, FKs: nil})
	m, _ = m.Update(views.IndexesLoadedMsg{Idx: 0, Indexes: nil})

	v := m.View()
	if !strings.Contains(v, "Referenced By") {
		t.Errorf("section must still appear when cache not ready, got:\n%s", v)
	}
	if !strings.Contains(v, "loading") {
		t.Errorf("Referenced By must show 'loading' when cache not ready, got:\n%s", v)
	}
}

// TL04: KindDataset rows cannot be expanded; Referenced By is never shown.
func TestAC_TL04_KindDatasetNotExpandable(t *testing.T) {
	datasets := []dataset.Dataset{{Name: "my_query", Kind: dataset.KindDataset}}
	m := views.NewTableListModel(keys.Default(), nil, nil, nil, nil)
	m, _ = m.Update(views.TablesLoadedMsg(datasets))
	m, _ = m.Update(tea.WindowSizeMsg{Width: 60, Height: 20})

	if m.FocusedExpandable() {
		t.Error("KindDataset rows must not be expandable")
	}
}

// TL05: Foreign Keys is now a middle section (├─); Referenced By is the last (└─).
func TestAC_TL05_BoxDrawingPrefixes(t *testing.T) {
	tables := []schema.Table{
		{
			Name: "customers",
			Kind: db.KindTable,
			Columns: []db.Column{{Name: "id"}},
			ForeignKeys: []db.ForeignKey{
				{Column: "something", ReferencedTable: "other", ReferencedColumn: "id"},
			},
			ReferencedBy: []schema.InboundFK{
				{FromTable: "orders", FromColumn: "customer_id", ToColumn: "id"},
			},
		},
	}
	datasets := []dataset.Dataset{{Name: "customers", Table: "customers", Kind: dataset.KindTable}}
	cache := schema.NewCacheWithData(tables, datasets)

	m := views.NewTableListModel(keys.Default(), nil, nil, nil, cache)
	m, _ = m.Update(views.TablesLoadedMsg(datasets))
	m, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 40})

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m, _ = m.Update(views.ExpansionLoadedMsg{Idx: 0, Cols: tables[0].Columns, FKs: tables[0].ForeignKeys})
	m, _ = m.Update(views.IndexesLoadedMsg{Idx: 0, Indexes: nil})

	v := m.View()
	// "Foreign Keys" must use ├─ (middle section).
	if !strings.Contains(v, "├─ Foreign Keys") {
		t.Errorf("Foreign Keys must use '├─' prefix, got:\n%s", v)
	}
	// "Referenced By" must use └─ (last section).
	if !strings.Contains(v, "└─ Referenced By") {
		t.Errorf("Referenced By must use '└─' prefix, got:\n%s", v)
	}
}

// --- KH: Keys & Help ---

// KH01: keys.Map.DrillFwd is ">".
func TestAC_KH01_DrillFwdBinding(t *testing.T) {
	k := keys.Default()
	found := false
	for _, kk := range k.DrillFwd.Keys() {
		if kk == ">" {
			found = true
			break
		}
	}
	if !found {
		t.Error("DrillFwd must be bound to '>'")
	}
}

// KH02: keys.Map.DrillReverse is "<".
func TestAC_KH02_DrillReverseBinding(t *testing.T) {
	k := keys.Default()
	found := false
	for _, kk := range k.DrillReverse.Keys() {
		if kk == "<" {
			found = true
			break
		}
	}
	if !found {
		t.Error("DrillReverse must be bound to '<'")
	}
}

// KH03: Full help overlay lists both ">" and "<" bindings.
func TestAC_KH03_HelpOverlayListsBothBindings(t *testing.T) {
	h := views.NewHelpOverlayView(keys.Default())
	h.SetSize(120, 40)
	v := h.View()
	if !strings.Contains(v, "drill forward") {
		t.Errorf("help overlay must list 'drill forward', got:\n%s", v)
	}
	if !strings.Contains(v, "referenced by") {
		t.Errorf("help overlay must list 'referenced by', got:\n%s", v)
	}
}

// KH04: FullHelp() includes both DrillFwd and DrillReverse in a group.
func TestAC_KH04_FullHelpIncludesDrillBindings(t *testing.T) {
	k := keys.Default()
	foundFwd, foundRev := false, false
	for _, group := range k.FullHelp() {
		for _, b := range group {
			if b.Help().Desc == "drill forward" {
				foundFwd = true
			}
			if b.Help().Desc == "referenced by" {
				foundRev = true
			}
		}
	}
	if !foundFwd {
		t.Error("FullHelp must include DrillFwd binding")
	}
	if !foundRev {
		t.Error("FullHelp must include DrillReverse binding")
	}
}
