package views_test

// View-level tests for the SQL editor overlay.
//
// Coverage map (ED section, from tasks/ready/sql-dataset-editor.md):
//   ED01: View() contains dataset name, SQL, "Ctrl+S save", "Esc cancel"
//         → TestAC_ED01_FreshlyOpenedEditorViewContainsRequiredStrings
//   ED02: Ctrl+S on empty editor keeps overlay open with "cannot be empty"
//         → TestAC_ED02_EmptyConfirmShowsError
//   ED03: Tab opens popup with at least one suggestion text
//         → TestAC_ED03_TabOpensPopup
//   ED04: Enter accepts the first suggestion and closes the popup
//         → TestAC_ED04_EnterInsertsSuggestionAndClosesPopup
//   ED05: Esc with popup open closes the popup, editor stays open
//         → TestAC_ED05_EscClosesPopupNotEditor
//   ED06: Esc with popup closed sets cancelled = true
//         → TestAC_ED06_EscClosedPopupCancelsEditor
//   ED07: EditSQL key wired in keys.Map and visible in helpoverlay's Dataset section
//         → TestAC_ED07_EditSQLKeyWiredAndInHelp
//   ED08: E in the schema explorer never opens the editor (any kind, including
//         KindDataset) — the editor is row-browser-only.
//         → TestAC_ED08_EInSchemaExplorerNeverOpens
//   ED09: Down/Up keys navigate the popup; typing narrows the suggestion list.
//         → TestAC_ED09_PopupArrowKeysAndNarrowing
//   ED10: Accepting a column from a dot-qualified prefix ("t.col") keeps the
//         qualifier intact — only the bare column part is replaced.
//         → TestAC_ED10_AcceptKeepsTableQualifier

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/polesen/datacow/internal/core/completions"
	"github.com/polesen/datacow/internal/core/db"
	"github.com/polesen/datacow/internal/core/dataset"
	"github.com/polesen/datacow/internal/core/schema"
	"github.com/polesen/datacow/internal/tui/keys"
	"github.com/polesen/datacow/internal/tui/views"
)

func newEditorFixture(sql string) views.SQLEditorModel {
	ds := dataset.Dataset{
		Name: "active_users",
		Kind: dataset.KindDataset,
		SQL:  sql,
	}
	tables := []schema.Table{
		{
			Name: "orders",
			Columns: []db.Column{
				{Name: "id", Type: "integer"},
				{Name: "total", Type: "numeric"},
			},
		},
		{
			Name: "users",
			Columns: []db.Column{
				{Name: "id", Type: "integer"},
				{Name: "email", Type: "varchar(255)"},
			},
		},
	}
	c := completions.New(tables, db.DialectPostgres)
	m := views.NewSQLEditorModel(ds, c, "/tmp/datacow_unused.yaml")
	return m.SetSize(120, 30)
}

// ED01 — freshly opened editor renders the dataset name, the pre-populated SQL,
// and the key-hint footer with "Ctrl+S save" and "Esc cancel".
func TestAC_ED01_FreshlyOpenedEditorViewContainsRequiredStrings(t *testing.T) {
	const sql = "SELECT * FROM users WHERE active = true"
	m := newEditorFixture(sql)
	v := m.View()

	for _, want := range []string{
		"Edit SQL",
		"active_users",
		"Ctrl+S save",
		"Esc cancel",
	} {
		if !strings.Contains(v, want) {
			t.Errorf("expected %q in editor view, got:\n%s", want, v)
		}
	}
	// The textarea must render the pre-populated SQL. Match a distinctive substring
	// — the full text may be reflowed by the textarea's internal soft-wrap.
	if !strings.Contains(v, "FROM users") {
		t.Errorf("expected pre-populated SQL substring %q in view, got:\n%s", "FROM users", v)
	}
}

// ED02 — Ctrl+S on an empty textarea keeps the overlay open and renders an
// error explaining the SQL is empty.
func TestAC_ED02_EmptyConfirmShowsError(t *testing.T) {
	m := newEditorFixture("")
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})

	if m.IsCancelled() {
		t.Error("editor must not be cancelled by Ctrl+S on empty SQL")
	}
	if m.IsSaved() {
		t.Error("editor must not be marked saved when Ctrl+S validates empty SQL")
	}
	v := m.View()
	if !strings.Contains(v, "cannot be empty") {
		t.Errorf("expected 'cannot be empty' error in view, got:\n%s", v)
	}
	if !strings.Contains(v, "Edit SQL") {
		t.Errorf("editor must still be visible after empty Ctrl+S, got:\n%s", v)
	}
}

// ED03 — Tab opens the popup; at least one suggestion text is rendered.
func TestAC_ED03_TabOpensPopup(t *testing.T) {
	m := newEditorFixture("SELECT * FROM ord")
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})

	if !m.IsPopupOpen() {
		t.Fatal("expected popup to be open after Tab")
	}
	v := m.View()
	if !strings.Contains(v, "orders") {
		t.Errorf("expected suggestion 'orders' in popup, got:\n%s", v)
	}
}

// ED04 — Enter accepts the first suggestion; the popup closes and the
// editor content now contains the suggestion text.
func TestAC_ED04_EnterInsertsSuggestionAndClosesPopup(t *testing.T) {
	m := newEditorFixture("SELECT * FROM ord")
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if !m.IsPopupOpen() {
		t.Fatal("expected popup to be open after Tab")
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.IsPopupOpen() {
		t.Error("popup must close after Enter")
	}
	if !strings.Contains(m.SQL(), "orders") {
		t.Errorf("expected editor content to contain 'orders' after accept, got %q", m.SQL())
	}
}

// ED05 — Esc with the popup open closes the popup but leaves the editor open.
func TestAC_ED05_EscClosesPopupNotEditor(t *testing.T) {
	m := newEditorFixture("SELECT * FROM ord")
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if !m.IsPopupOpen() {
		t.Fatal("expected popup to be open after Tab")
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.IsPopupOpen() {
		t.Error("popup must close after Esc")
	}
	if m.IsCancelled() {
		t.Error("editor must NOT be cancelled when Esc only closes the popup")
	}
	v := m.View()
	if !strings.Contains(v, "Edit SQL") {
		t.Errorf("editor must remain visible after Esc closes the popup, got:\n%s", v)
	}
}

// ED06 — Esc with the popup closed cancels the editor.
func TestAC_ED06_EscClosedPopupCancelsEditor(t *testing.T) {
	m := newEditorFixture("SELECT 1")
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if !m.IsCancelled() {
		t.Error("editor must be cancelled after Esc when popup is closed")
	}
}

// ED07 — EditSQL is bound to 'E' in keys.Map, the row browser emits
// OpenSQLEditorMsg on a KindDataset dataset, and the help overlay shows the
// binding in the Dataset section.
func TestAC_ED07_EditSQLKeyWiredAndInHelp(t *testing.T) {
	k := keys.Default()
	bindingKeys := k.EditSQL.Keys()
	if len(bindingKeys) == 0 {
		t.Fatal("EditSQL must have at least one bound key")
	}
	hasE := false
	for _, b := range bindingKeys {
		if b == "E" {
			hasE = true
		}
	}
	if !hasE {
		t.Errorf("EditSQL must bind 'E', got %v", bindingKeys)
	}

	// Row browser: 'E' on a KindDataset emits OpenSQLEditorMsg.
	ds := dataset.Dataset{Name: "active_users", Kind: dataset.KindDataset, SQL: "SELECT 1"}
	reg := views.NewColumnRegistry()
	rb := views.NewRowBrowserModelWithColumns(k, nil, nil, ds, nil, nil, reg)
	rb, _ = rb.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	result := makeResult(1, 1, 1,
		[]db.Column{{Name: "n"}},
		[]map[string]any{{"n": 1}},
	)
	rb, _ = rb.Update(views.RowsLoadedMsg(result))
	_, rbCmd := rb.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'E'}})
	if rbCmd == nil {
		t.Fatal("expected non-nil cmd from row browser 'E' on KindDataset")
	}
	rbMsg := rbCmd()
	if _, ok := rbMsg.(views.OpenSQLEditorMsg); !ok {
		t.Errorf("expected OpenSQLEditorMsg from row browser on KindDataset, got %T", rbMsg)
	}

	// Help overlay must include 'edit SQL'.
	ho := views.NewHelpOverlayView(k)
	ho.SetSize(120, 40)
	hov := ho.View()
	if !strings.Contains(hov, "edit SQL") {
		t.Errorf("help overlay must contain 'edit SQL', got:\n%s", hov)
	}
}

// ED08 — the schema explorer is no longer an entry point. 'E' on any cursor
// row (KindDataset, KindTable, KindView, KindPerspective) must NOT emit
// OpenSQLEditorMsg. The editor is reachable only from the row browser.
func TestAC_ED08_EInSchemaExplorerNeverOpens(t *testing.T) {
	cases := []dataset.Dataset{
		{Name: "users", Table: "users", Kind: dataset.KindTable},
		{Name: "v_users", Table: "v_users", Kind: dataset.KindView},
		{Name: "saved_query", Kind: dataset.KindDataset, SQL: "SELECT 1"},
		{Name: "My Lens", Table: "users", Kind: dataset.KindPerspective, ParentTable: "users"},
	}
	for _, ds := range cases {
		tl := loadedTableListModel(t, nil, []dataset.Dataset{ds})
		_, cmd := tl.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'E'}})
		if cmd == nil {
			continue // correct: no command emitted
		}
		msg := cmd()
		if _, ok := msg.(views.OpenSQLEditorMsg); ok {
			t.Errorf("schema explorer 'E' on Kind=%v must NOT emit OpenSQLEditorMsg", ds.Kind)
		}
	}
}

// ED10 — when the popup prefix is dot-qualified, accepting a column
// suggestion must keep the qualifier intact. Regression for: "SELECT u.em"
// → accept "email" → previously became "SELECT email", now stays
// "SELECT u.email".
func TestAC_ED10_AcceptKeepsTableQualifier(t *testing.T) {
	const sql = "SELECT u.em FROM users u"
	// Cursor at the end of "u.em" (position 11). The popup will offer columns
	// of the users table whose name starts with "em" — "email" is the match.
	ds := dataset.Dataset{Name: "q", Kind: dataset.KindDataset, SQL: sql}
	tables := []schema.Table{
		{
			Name: "users",
			Columns: []db.Column{
				{Name: "id", Type: "integer"},
				{Name: "email", Type: "varchar(255)"},
			},
		},
	}
	c := completions.New(tables, db.DialectPostgres)
	m := views.NewSQLEditorModel(ds, c, "/tmp/datacow_unused.yaml").SetSize(120, 30)

	// Move cursor from end-of-text back to just after "u.em" (position 11).
	const wantCursorAt = 11
	for i := len(sql); i > wantCursorAt; i-- {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if !m.IsPopupOpen() {
		t.Fatal("expected popup to be open after Tab on 'u.em'")
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := m.SQL()
	const want = "SELECT u.email FROM users u"
	if got != want {
		t.Errorf("accept on dot-qualified prefix lost the qualifier:\n  got:  %q\n  want: %q", got, want)
	}
}

// ED09 — with the popup open, Down/Up cycle the popup cursor and typing
// (runes / Backspace) narrows the suggestion list against the new prefix.
func TestAC_ED09_PopupArrowKeysAndNarrowing(t *testing.T) {
	m := newEditorFixture("SELECT * FROM o")
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if !m.IsPopupOpen() {
		t.Fatal("expected popup to be open after Tab")
	}
	v := m.View()
	if !strings.Contains(v, "orders") {
		t.Fatalf("expected 'orders' suggestion in popup, got:\n%s", v)
	}

	// Down should advance the popup cursor — the selected marker '> ' moves to
	// the next entry. With only "orders" matching "o" via tables, but multiple
	// keyword suggestions also start with O (e.g. ORDER BY, OR, OUTER), Down
	// must be a no-crash navigation.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if !m.IsPopupOpen() {
		t.Error("Down must not close the popup")
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if !m.IsPopupOpen() {
		t.Error("Up must not close the popup")
	}

	// Typing 'r' (prefix becomes "or") narrows the list. The popup stays open
	// but with a smaller, refreshed list (orders still matches).
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	if !m.IsPopupOpen() {
		t.Fatal("typing must keep the popup open while matches exist")
	}
	v2 := m.View()
	if !strings.Contains(v2, "orders") {
		t.Errorf("expected 'orders' still in popup after narrowing, got:\n%s", v2)
	}

	// Typing 'z' (prefix becomes "orz" — no matches) closes the popup.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'z'}})
	if m.IsPopupOpen() {
		t.Error("popup must close when typing produces no matches")
	}
}
