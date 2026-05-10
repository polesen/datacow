package views_test

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/polesen/datacow/internal/core/db"
	"github.com/polesen/datacow/internal/tui/views"
)

func TestQueryLogView_HeightZeroRendersEmpty(t *testing.T) {
	ql := db.NewQueryLog()
	addQuery(ql, "SELECT 1")
	v := views.NewQueryLogView(ql)
	// No WindowSizeMsg sent — height stays 0 → must return "".
	if got := v.View(); got != "" {
		t.Errorf("height=0 should render empty string, got %q", got)
	}
}

func TestQueryLogView_EmptyLog(t *testing.T) {
	ql := db.NewQueryLog()
	v := sizedQueryLogView(ql, 120, 20)
	out := v.View()
	if !strings.Contains(out, "(none)") {
		t.Error("empty history should show '(none)' placeholder")
	}
	if strings.Contains(out, "At:") {
		t.Error("empty log should not show SQL preview")
	}
}

func TestQueryLogView_HistoryHeaderNewestFirst(t *testing.T) {
	ql := db.NewQueryLog()
	addQuery(ql, "SELECT 1")
	v := sizedQueryLogView(ql, 120, 20)
	if !strings.Contains(v.View(), "newest first") {
		t.Error("history header must read 'newest first'")
	}
}

func TestQueryLogView_TimestampInHistoryRow(t *testing.T) {
	ql := db.NewQueryLog()
	addQuery(ql, "SELECT * FROM users")
	out := sizedQueryLogView(ql, 120, 20).View()
	// HH:MM:SS always contains two colons.
	colons := strings.Count(out, ":")
	if colons < 2 {
		t.Errorf("history row should contain HH:MM:SS timestamp (≥2 colons), got %d", colons)
	}
}

// TestQueryLogView_PreviewAlwaysVisible is the regression test for the viewport bug:
// the SQL preview was clipped when history rows filled the panel height.
func TestQueryLogView_PreviewAlwaysVisible(t *testing.T) {
	ql := db.NewQueryLog()
	const height = 20
	for i := 0; i < 30; i++ {
		addQuery(ql, "SELECT * FROM users")
	}
	v := sizedQueryLogView(ql, 120, height)

	// Move the cursor well into the list so all available row slots are consumed.
	for i := 0; i < 10; i++ {
		v, _ = v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	}
	out := v.View()

	if !strings.Contains(out, "At:") {
		t.Error("SQL preview ('At:') must remain visible after scrolling — viewport not reserving space")
	}
	if !strings.Contains(out, "SQL:") {
		t.Error("SQL preview ('SQL:') must remain visible after scrolling")
	}
	// lipgloss.Height counts trailing '\n' as an extra line, so allow height+1.
	if got := lipgloss.Height(out); got > height+1 {
		t.Errorf("rendered output (%d lines) exceeds panel height (%d) — content overflow", got, height)
	}
}

// TestQueryLogView_ScrollKeepsCursorVisible verifies the cursor stays in the visible
// window even when navigated to the oldest entry in a long history.
func TestQueryLogView_ScrollKeepsCursorVisible(t *testing.T) {
	ql := db.NewQueryLog()
	const height = 15
	const entries = 20
	for i := 0; i < entries; i++ {
		addQuery(ql, "SELECT * FROM users")
	}
	v := sizedQueryLogView(ql, 120, height)

	for i := 0; i < entries-1; i++ {
		v, _ = v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	}
	out := v.View()

	if !strings.Contains(out, "At:") {
		t.Error("preview must still be visible when cursor is at the oldest entry")
	}
	// lipgloss.Height counts trailing '\n' as an extra line, so allow height+1.
	if got := lipgloss.Height(out); got > height+1 {
		t.Errorf("output height %d > panel height %d", got, height)
	}
}

func TestQueryLogView_ErrorRowShowsERR(t *testing.T) {
	ql := db.NewQueryLog()
	addErrorQuery(ql, "SELECT * FROM bad", "connection reset by peer")
	out := sizedQueryLogView(ql, 120, 20).View()
	if !strings.Contains(out, "ERR") {
		t.Error("error entry must display 'ERR' in the history row")
	}
}

func TestQueryLogView_ErrorPreviewShowsMessage(t *testing.T) {
	ql := db.NewQueryLog()
	addErrorQuery(ql, "SELECT * FROM bad", "connection reset by peer")
	out := sizedQueryLogView(ql, 120, 20).View()
	if !strings.Contains(out, "Error:") {
		t.Error("error entry must show 'Error:' label in the SQL preview section")
	}
	if !strings.Contains(out, "connection reset") {
		t.Error("error entry must show the error message text in the SQL preview")
	}
}

func TestQueryLogView_NormalEntryShowsSQL(t *testing.T) {
	ql := db.NewQueryLog()
	addQuery(ql, "SELECT id FROM orders")
	out := sizedQueryLogView(ql, 120, 20).View()
	if !strings.Contains(out, "SQL:") {
		t.Error("normal entry must show 'SQL:' label in preview")
	}
	if !strings.Contains(out, "SELECT id FROM orders") {
		t.Error("normal entry must show the SQL text in preview")
	}
}

func TestQueryLogView_CursorNavigation(t *testing.T) {
	ql := db.NewQueryLog()
	for i := 0; i < 5; i++ {
		addQuery(ql, "SELECT 1")
	}
	v := sizedQueryLogView(ql, 120, 30)

	// Move down twice then back up — should not panic and should stay in bounds.
	v, _ = v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	v, _ = v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	v, _ = v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})

	// Render must succeed and still contain the preview.
	out := v.View()
	if !strings.Contains(out, "At:") {
		t.Error("preview must be visible after cursor navigation")
	}
}

func TestQueryLogView_CursorClamped(t *testing.T) {
	ql := db.NewQueryLog()
	addQuery(ql, "SELECT 1")
	v := sizedQueryLogView(ql, 120, 20)

	// Try to move past the end and before the start.
	for i := 0; i < 10; i++ {
		v, _ = v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	}
	for i := 0; i < 10; i++ {
		v, _ = v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	}

	// Should not panic and should still render the preview.
	out := v.View()
	if !strings.Contains(out, "At:") {
		t.Error("preview must be visible after clamped navigation")
	}
}

func TestQueryLogView_DefaultHidesSystemQueries(t *testing.T) {
	ql := db.NewQueryLog()
	addQuery(ql, "SELECT * FROM users")
	addQuery(ql, "SELECT COUNT(*) AS _dc_count FROM t")
	v := sizedQueryLogView(ql, 120, 20)
	out := v.View()
	if strings.Contains(out, "system") {
		t.Error("default user-only mode should hide system queries")
	}
}

func TestQueryLogView_ToggleShowsSystemQueries(t *testing.T) {
	ql := db.NewQueryLog()
	addQuery(ql, "SELECT * FROM users")
	addQuery(ql, "SELECT COUNT(*) AS _dc_count FROM t")
	v := sizedQueryLogView(ql, 120, 20)
	v, _ = v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	out := v.View()
	if !strings.Contains(out, "system") {
		t.Error("after toggle, system queries should appear with 'system' badge")
	}
}

func TestQueryLogView_ToggleBackHidesSystem(t *testing.T) {
	ql := db.NewQueryLog()
	addQuery(ql, "SELECT * FROM users")
	addQuery(ql, "SELECT COUNT(*) AS _dc_count FROM t")
	v := sizedQueryLogView(ql, 120, 20)
	v, _ = v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	v, _ = v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	out := v.View()
	if strings.Contains(out, "system") {
		t.Error("after two toggles, system queries should be hidden again")
	}
}

func TestQueryLogView_HeaderReflectsFilterState(t *testing.T) {
	ql := db.NewQueryLog()
	v := sizedQueryLogView(ql, 120, 20)
	out := v.View()
	if !strings.Contains(out, "user only") {
		t.Error("default header should contain 'user only'")
	}
	v, _ = v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	out = v.View()
	if !strings.Contains(out, "all queries") {
		t.Error("after toggle, header should contain 'all queries'")
	}
}

func TestQueryLogView_EmptyAfterFilter(t *testing.T) {
	ql := db.NewQueryLog()
	addQuery(ql, "SELECT COUNT(*) AS _dc_count FROM t")
	v := sizedQueryLogView(ql, 120, 20)
	out := v.View()
	if !strings.Contains(out, "(none)") {
		t.Error("user-only mode with only system queries should show '(none)' placeholder")
	}
}
