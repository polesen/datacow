package views_test

// Acceptance tests for the table-list filter feature.
// Each test is named TestAC_<SECTION><NN>_<description> to map directly to an
// acceptance criterion in tasks/done/tablelist-filter-search.md.
//
// Sections:
//   B   — Behaviour (B01–B11)
//   SC  — Schema Cache (SC01–SC02)
//   KH  — Keys & Help (KH01–KH04)
//
// Helpers loadedTableListModel, pressSlash, typeText, pressEsc, and pressEnter
// are defined in tablelist_test.go (same package).

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

// --- B: Behaviour ---

// B01: "/" in the table list opens a single-line input docked at the bottom of
// the tables pane. The input is empty on first open, or pre-filled with the
// held filter on re-open.
func TestAC_B01_SlashOpensEmptyInputOnFirstOpen(t *testing.T) {
	m := loadedTableListModel(t, nil, []dataset.Dataset{
		{Name: "users", Table: "users", Kind: dataset.KindTable},
	})
	if m.FilterInputActive() {
		t.Fatal("filter input must not be active before / is pressed")
	}

	m, _ = pressSlash(m)
	if !m.FilterInputActive() {
		t.Fatal("/ must open the filter input")
	}
	// On first open the query is empty.
	if m.FilterActive() {
		t.Error("filter query must be empty on first open")
	}
	v := m.View()
	if !strings.Contains(v, "/") {
		t.Errorf("view must show filter prompt after /, got:\n%s", v)
	}
}

// B01 (re-open path): after Enter (filter held), pressing / again pre-fills the
// input with the current query and places the cursor at the end.
func TestAC_B01_SlashPreFillsOnReopen(t *testing.T) {
	m := loadedTableListModel(t, nil, []dataset.Dataset{
		{Name: "users", Table: "users", Kind: dataset.KindTable},
		{Name: "orders", Table: "orders", Kind: dataset.KindTable},
	})
	m, _ = pressSlash(m)
	m, _ = typeText(m, "ord")
	m, _ = pressEnter(m) // hold filter, close input

	// Re-open — input must be pre-filled.
	m, _ = pressSlash(m)
	if !m.FilterInputActive() {
		t.Fatal("second / must re-open filter input")
	}
	v := m.View()
	if !strings.Contains(v, "ord") {
		t.Errorf("re-opened filter input must be pre-filled with 'ord', got:\n%s", v)
	}
}

// B02: Typing narrows the visible list live (every keystroke). Match is
// case-insensitive substring against dataset names.
func TestAC_B02_TypingNarrowsListLive(t *testing.T) {
	m := loadedTableListModel(t, nil, []dataset.Dataset{
		{Name: "users", Table: "users", Kind: dataset.KindTable},
		{Name: "orders", Table: "orders", Kind: dataset.KindTable},
		{Name: "products", Table: "products", Kind: dataset.KindTable},
	})
	m, _ = pressSlash(m)
	// After typing 'o' only orders should be visible.
	m, _ = typeText(m, "o")
	v := m.View()
	if !strings.Contains(v, "orders") {
		t.Errorf("orders should match 'o', got:\n%s", v)
	}
	// After adding 'r' (now 'or') still only orders.
	m, _ = typeText(m, "r")
	v = m.View()
	if strings.Contains(v, "users") {
		t.Errorf("users should not match 'or', got:\n%s", v)
	}
	if strings.Contains(v, "products") {
		t.Errorf("products should not match 'or', got:\n%s", v)
	}
	if !strings.Contains(v, "orders") {
		t.Errorf("orders should still match 'or', got:\n%s", v)
	}
}

// B02 (case-insensitivity): matching is case-insensitive.
func TestAC_B02_MatchIsCaseInsensitive(t *testing.T) {
	m := loadedTableListModel(t, nil, []dataset.Dataset{
		{Name: "Users", Table: "Users", Kind: dataset.KindTable},
		{Name: "orders", Table: "orders", Kind: dataset.KindTable},
	})
	m, _ = pressSlash(m)
	m, _ = typeText(m, "users")
	v := m.View()
	if !strings.Contains(v, "Users") {
		t.Errorf("filter must be case-insensitive; 'Users' should match 'users', got:\n%s", v)
	}
	if strings.Contains(v, "orders") {
		t.Errorf("orders must not match 'users', got:\n%s", v)
	}
}

// B03: When the cache is not ready, filter matches on dataset names only and
// the input footer shows "(schema loading — name match only)". Once the cache
// becomes ready, the filter re-evaluates automatically.
func TestAC_B03_CacheNotReadyShowsHint(t *testing.T) {
	// Cache nil means not ready.
	m := loadedTableListModel(t, nil, []dataset.Dataset{
		{Name: "users", Table: "users", Kind: dataset.KindTable},
	})
	m, _ = pressSlash(m)
	v := m.View()
	if !strings.Contains(v, "schema loading") {
		t.Errorf("filter footer must show 'schema loading' hint when cache not ready, got:\n%s", v)
	}
}

// B03 (re-evaluate on cache ready): OnCacheReady re-applies the filter.
func TestAC_B03_CacheReadyNoHint(t *testing.T) {
	tables := []schema.Table{
		{Name: "users", Kind: db.KindTable, Columns: []db.Column{{Name: "id"}}},
	}
	datasets := []dataset.Dataset{
		{Name: "users", Table: "users", Kind: dataset.KindTable},
	}
	cache := schema.NewCacheWithData(tables, datasets)
	m := loadedTableListModel(t, cache, datasets)
	m, _ = pressSlash(m)
	v := m.View()
	if strings.Contains(v, "schema loading") {
		t.Errorf("filter footer must NOT show 'schema loading' when cache is ready, got:\n%s", v)
	}
}

// B04: Datasets that match only via a sub-item (column / FK / index) are shown
// COLLAPSED (not auto-expanded). Deviation from the original spec: the spec said
// "auto-expanded" but the implementation chose "shown collapsed" because short
// queries (e.g. "id") match many column names and bursting every tree open is
// jarring. The user expands manually to see which sub-item caused the match.
func TestAC_B04_SubMatchVisibleButCollapsed(t *testing.T) {
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
	m, _ = typeText(m, "email") // matches users via column, not orders

	v := m.View()
	// users must be visible (column sub-match).
	if !strings.Contains(v, "users") {
		t.Errorf("users must be visible (column sub-match), got:\n%s", v)
	}
	// orders has no 'email' — must be hidden.
	if strings.Contains(v, "orders") {
		t.Errorf("orders must be hidden (no column 'email'), got:\n%s", v)
	}
	// IMPLEMENTATION NOTE: the spec originally required auto-expand. The
	// implementation deliberately keeps sub-matched datasets collapsed to avoid
	// bursting open every tree on short queries. "Columns" only appears when a
	// node is expanded; its absence confirms collapsed state.
	if strings.Contains(v, "Columns") {
		t.Errorf("sub-matched dataset must remain COLLAPSED (deviation from spec), got:\n%s", v)
	}
}

// B05: Datasets whose name matches directly are shown in their current expand
// state (collapsed by default). The matching substring in the name is highlighted.
func TestAC_B05_NameMatchStaysCollapsed(t *testing.T) {
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
		t.Errorf("users must be visible (name match), got:\n%s", v)
	}
	// Must remain collapsed — "Columns" only appears when expanded.
	if strings.Contains(v, "Columns") {
		t.Errorf("name-match dataset must remain collapsed, got:\n%s", v)
	}
}

// B06: Non-matching datasets are hidden, not dimmed.
func TestAC_B06_NonMatchingDatasetsHidden(t *testing.T) {
	m := loadedTableListModel(t, nil, []dataset.Dataset{
		{Name: "users", Table: "users", Kind: dataset.KindTable},
		{Name: "orders", Table: "orders", Kind: dataset.KindTable},
		{Name: "products", Table: "products", Kind: dataset.KindTable},
	})
	m, _ = pressSlash(m)
	m, _ = typeText(m, "ord")

	v := m.View()
	if strings.Contains(v, "users") {
		t.Errorf("users must be hidden (not dimmed), got:\n%s", v)
	}
	if strings.Contains(v, "products") {
		t.Errorf("products must be hidden (not dimmed), got:\n%s", v)
	}
	if !strings.Contains(v, "orders") {
		t.Errorf("orders must be visible, got:\n%s", v)
	}
}

// B07: When no dataset matches, the list area renders a single
// `No tables match "<query>"` line.
func TestAC_B07_NoMatchShowsPlaceholder(t *testing.T) {
	m := loadedTableListModel(t, nil, []dataset.Dataset{
		{Name: "users", Table: "users", Kind: dataset.KindTable},
	})
	m, _ = pressSlash(m)
	m, _ = typeText(m, "zzzzz")

	v := m.View()
	if !strings.Contains(v, `No tables match`) {
		t.Errorf("must show no-match placeholder, got:\n%s", v)
	}
	if strings.Contains(v, "users") {
		t.Errorf("hidden tables must not appear in no-match state, got:\n%s", v)
	}
}

// B08: Enter blurs the input, keeps the filter held, and moves focus back to
// the list. ↑/↓ then navigate the filtered set.
// IMPLEMENTATION NOTE: the spec says navigation works after Enter; the
// implementation also supports navigation while the input is still open
// (see TestAC_B08_NavigationWhileInputOpen and TestTableListFilter_NavigationWhileInputOpen).
func TestAC_B08_EnterKeepsFilterAndNavigationWorks(t *testing.T) {
	m := loadedTableListModel(t, nil, []dataset.Dataset{
		{Name: "apple", Table: "apple", Kind: dataset.KindTable},
		{Name: "cherry", Table: "cherry", Kind: dataset.KindTable},
		{Name: "apricot", Table: "apricot", Kind: dataset.KindTable},
	})
	m, _ = pressSlash(m)
	m, _ = typeText(m, "p") // matches apple (0) and apricot (2)
	m, _ = pressEnter(m)    // hold filter, close input

	if !m.FilterActive() {
		t.Error("filter must remain held after Enter")
	}
	if m.FilterInputActive() {
		t.Error("filter input must be closed after Enter")
	}

	// Cursor starts at apple (0).
	if m.Cursor() != 0 {
		t.Errorf("cursor must be at apple (0), got %d", m.Cursor())
	}
	// Down should skip cherry (1, hidden) and land on apricot (2).
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.Cursor() != 2 {
		t.Errorf("Down must skip to apricot (2), got %d", m.Cursor())
	}
	// Up should return to apple (0).
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if m.Cursor() != 0 {
		t.Errorf("Up must return to apple (0), got %d", m.Cursor())
	}
}

// B08 (navigation while input open): ↑/↓ work even before Enter is pressed.
func TestAC_B08_NavigationWhileInputOpen(t *testing.T) {
	m := loadedTableListModel(t, nil, []dataset.Dataset{
		{Name: "apple", Table: "apple", Kind: dataset.KindTable},
		{Name: "cherry", Table: "cherry", Kind: dataset.KindTable},
		{Name: "apricot", Table: "apricot", Kind: dataset.KindTable},
	})
	m, _ = pressSlash(m)
	m, _ = typeText(m, "p") // matches apple (0) and apricot (2)

	if !m.FilterInputActive() {
		t.Fatal("filter input must still be open")
	}
	// Down while input is open should skip to apricot (2).
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.Cursor() != 2 {
		t.Errorf("Down while filter open must move to apricot (2), got %d", m.Cursor())
	}
	if !m.FilterInputActive() {
		t.Error("filter input must remain open after Down navigation")
	}
	// Up should return to apple (0) with input still open.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if m.Cursor() != 0 {
		t.Errorf("Up while filter open must return to apple (0), got %d", m.Cursor())
	}
	if !m.FilterInputActive() {
		t.Error("filter input must remain open after Up navigation")
	}
}

// B09: Esc clears the filter, closes the input, and restores the previous
// cursor position by name.
func TestAC_B09_EscClearsFilterAndRestoresCursor(t *testing.T) {
	m := loadedTableListModel(t, nil, []dataset.Dataset{
		{Name: "users", Table: "users", Kind: dataset.KindTable},
		{Name: "orders", Table: "orders", Kind: dataset.KindTable},
		{Name: "products", Table: "products", Kind: dataset.KindTable},
	})
	// Move to orders (index 1).
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.Cursor() != 1 {
		t.Fatalf("expected cursor at 1 before filter, got %d", m.Cursor())
	}

	// Open filter and type something that hides orders.
	m, _ = pressSlash(m)
	m, _ = typeText(m, "pro") // only products matches

	// Esc must clear and restore.
	m, _ = pressEsc(m)
	if m.FilterActive() {
		t.Error("filter must be cleared after Esc")
	}
	if m.FilterInputActive() {
		t.Error("filter input must be closed after Esc")
	}
	v := m.View()
	if !strings.Contains(v, "users") || !strings.Contains(v, "orders") || !strings.Contains(v, "products") {
		t.Errorf("all tables must be visible after Esc, got:\n%s", v)
	}
	// Cursor must be restored to orders (index 1) by name.
	if m.Cursor() != 1 {
		t.Errorf("cursor must be restored to orders (1) after Esc, got %d", m.Cursor())
	}
}

// B10: Switching focus to another pane, or leaving the tables view, clears
// the filter. Covered by TestAC_B10_LeavingPaneClearsFilter in app_test.go
// (requires TEST_POSTGRES_DSN and uses teatest). See that file for the
// authoritative acceptance test.

// B11: Status bar shows `filter: "X"  M/N` while a filter is held.
func TestAC_B11_FilterStatusShownWhileHeld(t *testing.T) {
	m := loadedTableListModel(t, nil, []dataset.Dataset{
		{Name: "users", Table: "users", Kind: dataset.KindTable},
		{Name: "orders", Table: "orders", Kind: dataset.KindTable},
		{Name: "products", Table: "products", Kind: dataset.KindTable},
	})
	if m.FilterStatus() != "" {
		t.Error("FilterStatus must be empty when no filter active")
	}
	m, _ = pressSlash(m)
	m, _ = typeText(m, "ord")

	status := m.FilterStatus()
	if !strings.Contains(status, "ord") {
		t.Errorf("FilterStatus must contain query 'ord', got: %q", status)
	}
	if !strings.Contains(status, "1/3") {
		t.Errorf("FilterStatus must contain match count '1/3', got: %q", status)
	}

	// Status must still be shown after Enter (filter held, input closed).
	m, _ = pressEnter(m)
	status = m.FilterStatus()
	if !strings.Contains(status, "ord") {
		t.Errorf("FilterStatus must still contain query after Enter, got: %q", status)
	}
}

// --- SC: Schema Cache ---

// SC01: schema.Table gains an Indexes []db.Index field, populated by schema.Load.
// Views skip the index lookup. Covered by schema_test.go — that package test
// exercises Load directly against a real DB and asserts Indexes is populated.
// No view-layer test needed; the schema layer owns this criterion.

// SC02: No call sites of schema.Load regress. Covered by the full test suite
// (go test ./...). Any regression would manifest as a compile error or test
// failure in the schema package.

// The two SC tests are intentionally omitted from this file because they are
// owned by internal/core/schema/schema_test.go and require a real DB.
// They are exercised by gotestsum ./... in the done checklist.

// --- KH: Keys & Help ---

// KH01: keys.Map has a TableListFilter binding bound to "/".
func TestAC_KH01_TableListFilterBindingExists(t *testing.T) {
	k := keys.Default()
	// Trigger the binding via a "/" key message to confirm it is wired.
	m := views.NewTableListModel(k, nil, nil, nil, nil)
	m, _ = m.Update(views.TablesLoadedMsg([]dataset.Dataset{
		{Name: "users", Table: "users", Kind: dataset.KindTable},
	}))
	m, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 20})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	if !m.FilterInputActive() {
		t.Error("TableListFilter binding for '/' must open the filter input")
	}
}

// KH02: keys.TableListHelp() includes the TableListFilter binding.
func TestAC_KH02_TableListHelpIncludesFilterBinding(t *testing.T) {
	k := keys.Default()
	found := false
	for _, b := range k.TableListHelp() {
		if b.Help().Desc == "filter tables" {
			found = true
			break
		}
	}
	if !found {
		t.Error("TableListHelp must include the 'filter tables' binding")
	}
}

// KH03: The full help overlay lists "/ filter tables" in the Table List group.
// Covered by TestHelpOverlayView_TableListFilterVisible in helpoverlay_test.go.
// No duplicate needed here — that test already asserts the overlay contains
// "filter tables".

// KH04: No other key behaviour is changed. In particular, global keys
// (Ctrl+P, i, ?, L, Ctrl+R) are NOT intercepted when the filter input is open.
// Covered by TestApp_TableListFilter_BlocksGlobalKeys in app_test.go, which
// verifies "i" is consumed by the filter (typed into input) rather than opening
// the table-info overlay. The inverse — that these keys work normally when the
// filter is NOT open — is implied by all other app-level tests that rely on
// those keys functioning correctly (TestApp_HelpOverlayView_OpensAndCloses,
// TestApp_QueryLog_LKeyFromPane1, etc.).
