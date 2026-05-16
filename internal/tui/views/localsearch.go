package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/polesen/datacow/internal/core/db"
)

// LocalSearchState holds the state for the k9s-style local page search.
// It is embedded in RowBrowserModel as a value type.
type LocalSearchState struct {
	query       string          // current search string
	inputActive bool            // true while the search input has focus
	textInput   textinput.Model // the bottom-bar text input
	matchRows   []int           // row indices that contain at least one match
	matchCursor int             // position in matchRows for n/N navigation
}

func newLocalSearch() LocalSearchState {
	ti := textinput.New()
	ti.Placeholder = "search…"
	ti.Prompt = "/"
	ti.CharLimit = 100
	return LocalSearchState{textInput: ti}
}

// IsActive returns true when a search is in progress (input open or query set).
func (ls LocalSearchState) IsActive() bool {
	return ls.query != "" || ls.inputActive
}

// InputActive returns true when the search bar text input has keyboard focus.
func (ls LocalSearchState) InputActive() bool { return ls.inputActive }

// Query returns the current search string.
func (ls LocalSearchState) Query() string { return ls.query }

// MatchCount returns the number of rows that matched the search query.
func (ls LocalSearchState) MatchCount() int { return len(ls.matchRows) }

// MatchRows returns the ordered slice of row indices that match the query.
func (ls LocalSearchState) MatchRows() []int { return ls.matchRows }

// MatchCursor returns the index into MatchRows() of the currently selected match.
func (ls LocalSearchState) MatchCursor() int { return ls.matchCursor }

// CurrentMatchRow returns the row index of the current match for cursor nav, or -1.
func (ls LocalSearchState) CurrentMatchRow() int {
	if len(ls.matchRows) == 0 {
		return -1
	}
	if ls.matchCursor >= len(ls.matchRows) {
		return ls.matchRows[0]
	}
	return ls.matchRows[ls.matchCursor]
}

// IsMatch reports whether the row at rowIdx matches the current query.
func (ls LocalSearchState) IsMatch(rowIdx int) bool {
	for _, idx := range ls.matchRows {
		if idx == rowIdx {
			return true
		}
	}
	return false
}

// StatusText returns the search status bar text (e.g. `search: "foo"  3/12 matches`).
func (ls LocalSearchState) StatusText() string {
	if ls.query == "" {
		return ""
	}
	pos := ls.matchCursor + 1
	if len(ls.matchRows) == 0 {
		pos = 0
	}
	return fmt.Sprintf("search: %q  %d/%d matches", ls.query, pos, len(ls.matchRows))
}

// withInputOpen returns a copy with the input activated and focused.
func (ls LocalSearchState) withInputOpen() (LocalSearchState, tea.Cmd) {
	ls.inputActive = true
	cmd := ls.textInput.Focus()
	return ls, cmd
}

// withInputClosed returns a copy with input closed but query/highlights retained.
func (ls LocalSearchState) withInputClosed() LocalSearchState {
	ls.inputActive = false
	ls.textInput.Blur()
	return ls
}

// cleared returns a zeroed copy — clears query, highlights, and navigation.
func (ls LocalSearchState) cleared() LocalSearchState {
	ls = newLocalSearch()
	return ls
}

// withNextMatch advances the match cursor, wrapping around.
func (ls LocalSearchState) withNextMatch() LocalSearchState {
	if len(ls.matchRows) == 0 {
		return ls
	}
	ls.matchCursor = (ls.matchCursor + 1) % len(ls.matchRows)
	return ls
}

// withPrevMatch moves the match cursor backward, wrapping around.
func (ls LocalSearchState) withPrevMatch() LocalSearchState {
	if len(ls.matchRows) == 0 {
		return ls
	}
	ls.matchCursor = (ls.matchCursor - 1 + len(ls.matchRows)) % len(ls.matchRows)
	return ls
}

// recompute rebuilds matchRows from the given query and page rows.
// Returns the updated state with the cursor clamped.
func (ls LocalSearchState) recompute(query string, cols []db.Column, rows []map[string]any) LocalSearchState {
	ls.query = query
	ls.matchRows = nil
	if query == "" {
		return ls
	}
	q := strings.ToLower(query)
	for i, row := range rows {
		for _, col := range cols {
			v := formatCellValue(row[col.Name])
			if strings.Contains(strings.ToLower(v), q) {
				ls.matchRows = append(ls.matchRows, i)
				break
			}
		}
	}
	if ls.matchCursor >= len(ls.matchRows) {
		ls.matchCursor = 0
	}
	return ls
}

// View renders the search bar line.
func (ls LocalSearchState) View(width int) string {
	return ls.textInput.View()
}

// Update forwards a message to the text input and returns updated state + cmd.
func (ls LocalSearchState) Update(msg tea.Msg) (LocalSearchState, tea.Cmd) {
	var cmd tea.Cmd
	ls.textInput, cmd = ls.textInput.Update(msg)
	return ls, cmd
}
