package views

import (
	"strings"
	"testing"

	"github.com/polesen/datacow/internal/core/db"
)

func makeSearchRows() ([]db.Column, []map[string]any) {
	cols := []db.Column{{Name: "name"}, {Name: "email"}}
	rows := []map[string]any{
		{"name": "Alice", "email": "alice@example.com"},
		{"name": "Bob", "email": "bob@example.com"},
		{"name": "Charlie", "email": "charlie@example.com"},
	}
	return cols, rows
}

func TestLocalSearch_Compute_CaseInsensitive(t *testing.T) {
	cols, rows := makeSearchRows()
	ls := newLocalSearch()
	ls = ls.recompute("ALICE", cols, rows)

	if ls.MatchCount() != 1 {
		t.Fatalf("expected 1 match, got %d", ls.MatchCount())
	}
	if ls.CurrentMatchRow() != 0 {
		t.Errorf("expected match at row 0, got %d", ls.CurrentMatchRow())
	}
}

func TestLocalSearch_Compute_SubstringMatch(t *testing.T) {
	cols, rows := makeSearchRows()
	ls := newLocalSearch()
	ls = ls.recompute("example.com", cols, rows)
	if ls.MatchCount() != 3 {
		t.Errorf("expected 3 matches (all rows have example.com), got %d", ls.MatchCount())
	}
}

func TestLocalSearch_Compute_MultipleColumns(t *testing.T) {
	cols, rows := makeSearchRows()
	ls := newLocalSearch()
	ls = ls.recompute("bob", cols, rows)
	if ls.MatchCount() != 1 {
		t.Errorf("expected 1 match (Bob), got %d", ls.MatchCount())
	}
	if ls.CurrentMatchRow() != 1 {
		t.Errorf("expected match at row 1, got %d", ls.CurrentMatchRow())
	}
}

func TestLocalSearch_Compute_NoMatch(t *testing.T) {
	cols, rows := makeSearchRows()
	ls := newLocalSearch()
	ls = ls.recompute("zzz", cols, rows)
	if ls.MatchCount() != 0 {
		t.Errorf("expected 0 matches, got %d", ls.MatchCount())
	}
	if ls.CurrentMatchRow() != -1 {
		t.Errorf("expected CurrentMatchRow -1, got %d", ls.CurrentMatchRow())
	}
}

func TestLocalSearch_Compute_EmptyQueryClearsMatches(t *testing.T) {
	cols, rows := makeSearchRows()
	ls := newLocalSearch()
	ls = ls.recompute("alice", cols, rows)
	ls = ls.recompute("", cols, rows)
	if ls.MatchCount() != 0 {
		t.Errorf("expected 0 matches for empty query, got %d", ls.MatchCount())
	}
}

func TestLocalSearch_NextPrev_Wrap(t *testing.T) {
	cols, rows := makeSearchRows()
	ls := newLocalSearch()
	ls = ls.recompute("example.com", cols, rows) // 3 matches: rows 0,1,2

	if ls.CurrentMatchRow() != 0 {
		t.Errorf("initial: expected row 0, got %d", ls.CurrentMatchRow())
	}

	ls = ls.withNextMatch()
	if ls.CurrentMatchRow() != 1 {
		t.Errorf("after first next: expected row 1, got %d", ls.CurrentMatchRow())
	}

	ls = ls.withNextMatch()
	if ls.CurrentMatchRow() != 2 {
		t.Errorf("after second next: expected row 2, got %d", ls.CurrentMatchRow())
	}

	ls = ls.withNextMatch()
	if ls.CurrentMatchRow() != 0 {
		t.Errorf("wrap-around: expected row 0, got %d", ls.CurrentMatchRow())
	}

	ls = ls.withPrevMatch()
	if ls.CurrentMatchRow() != 2 {
		t.Errorf("prev wrap: expected row 2, got %d", ls.CurrentMatchRow())
	}
}

func TestLocalSearch_IsMatch(t *testing.T) {
	cols, rows := makeSearchRows()
	ls := newLocalSearch()
	ls = ls.recompute("alice", cols, rows)

	if !ls.IsMatch(0) {
		t.Error("row 0 should match")
	}
	if ls.IsMatch(1) {
		t.Error("row 1 should not match")
	}
	if ls.IsMatch(2) {
		t.Error("row 2 should not match")
	}
}

func TestLocalSearch_IsActive(t *testing.T) {
	ls := newLocalSearch()
	if ls.IsActive() {
		t.Error("should not be active initially")
	}

	var cmd interface{}
	_ = cmd
	ls, _ = ls.withInputOpen()
	if !ls.IsActive() {
		t.Error("should be active when input is open")
	}

	ls = ls.withInputClosed()
	if ls.IsActive() {
		t.Error("should not be active after closing with no query")
	}

	// Active with query set (highlights stay active even after input closed)
	cols, rows := makeSearchRows()
	ls = ls.recompute("bob", cols, rows)
	ls, _ = ls.withInputOpen()
	ls = ls.withInputClosed()
	if !ls.IsActive() {
		t.Error("should be active when query is non-empty")
	}
}

func TestLocalSearch_StatusText(t *testing.T) {
	cols, rows := makeSearchRows()
	ls := newLocalSearch()
	ls = ls.recompute("example.com", cols, rows)

	txt := ls.StatusText()
	if txt == "" {
		t.Error("StatusText should not be empty when search is active")
	}
	if !strings.Contains(txt, "example.com") {
		t.Errorf("StatusText %q missing query string", txt)
	}
	if !strings.Contains(txt, "3") {
		t.Errorf("StatusText %q missing match count 3", txt)
	}
}

func TestLocalSearch_Cleared(t *testing.T) {
	cols, rows := makeSearchRows()
	ls := newLocalSearch()
	ls = ls.recompute("alice", cols, rows)
	ls = ls.cleared()
	if ls.IsActive() {
		t.Error("cleared search should not be active")
	}
	if ls.MatchCount() != 0 {
		t.Error("cleared search should have 0 matches")
	}
}
