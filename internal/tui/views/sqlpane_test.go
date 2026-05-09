package views_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/polesen/datacow/internal/core/db"
	"github.com/polesen/datacow/internal/tui/keys"
	"github.com/polesen/datacow/internal/tui/views"
)

func newSQLPane(ql *db.QueryLog, w, h int) views.SQLPaneModel {
	m := views.NewSQLPaneModel(keys.Default(), ql)
	m, _ = m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return m
}

func TestSQLPaneModel_NoQueriesYet(t *testing.T) {
	ql := db.NewQueryLog()
	out := newSQLPane(ql, 120, 5).View()
	if !strings.Contains(out, "no queries yet") {
		t.Errorf("empty pane should show placeholder, got: %q", out)
	}
}

func TestSQLPaneModel_TimestampVisible(t *testing.T) {
	ql := db.NewQueryLog()
	addQuery(ql, "SELECT * FROM users")
	out := newSQLPane(ql, 120, 5).View()
	// HH:MM:SS contains two colons.
	if strings.Count(out, ":") < 2 {
		t.Error("SQL pane history row should contain a HH:MM:SS timestamp")
	}
}

// TestSQLPaneModel_ErrorRowRendered is a smoke test: error entries must not
// panic and the row must appear in the output alongside the normal entry.
func TestSQLPaneModel_ErrorRowRendered(t *testing.T) {
	ql := db.NewQueryLog()
	addQuery(ql, "SELECT * FROM users")
	addErrorQuery(ql, "SELECT * FROM bad", "dial tcp: connection refused")
	out := newSQLPane(ql, 120, 10).View()
	// Both entries produce lines — at minimum two timestamp patterns.
	if strings.Count(out, ":") < 4 {
		t.Errorf("expected at least two HH:MM:SS timestamps (two entries), got output: %q", out)
	}
}

// TestSQLPaneModel_SQLTruncated verifies long SQL is shown (even if truncated)
// rather than left blank.
func TestSQLPaneModel_SQLVisible(t *testing.T) {
	ql := db.NewQueryLog()
	addQuery(ql, "SELECT id, name, email FROM users WHERE active = true")
	out := newSQLPane(ql, 120, 5).View()
	if !strings.Contains(out, "SELECT") {
		t.Error("SQL pane should show the SQL text (possibly truncated)")
	}
}

// TestSQLPaneModel_NewestAtBottom verifies the ordering: newest entry appears
// after older entries in the output (streaming / live-feed style).
func TestSQLPaneModel_NewestAtBottom(t *testing.T) {
	ql := db.NewQueryLog()
	addQuery(ql, "SELECT * FROM alpha")
	addQuery(ql, "SELECT * FROM zeta") // added last → newest → should appear last in pane
	out := newSQLPane(ql, 120, 10).View()
	posAlpha := strings.Index(out, "alpha")
	posZeta := strings.Index(out, "zeta")
	if posAlpha == -1 || posZeta == -1 {
		t.Skip("one of the SQL strings was truncated — cannot verify order")
	}
	if posZeta < posAlpha {
		t.Error("newest entry ('zeta') should appear after older entry ('alpha') in the SQL pane")
	}
}
