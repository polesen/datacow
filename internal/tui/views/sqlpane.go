package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/beetio/datacow/internal/core/db"
	"github.com/beetio/datacow/internal/tui/keys"
	"github.com/beetio/datacow/internal/tui/style"
)

// SQLPaneModel renders recent query history in a compact bottom strip.
type SQLPaneModel struct {
	queryLog *db.QueryLog
	cursor   int
	focused  bool
	width    int
	height   int
	keys     keys.Map
}

// NewSQLPaneModel returns a SQLPaneModel backed by the given log.
func NewSQLPaneModel(k keys.Map, ql *db.QueryLog) SQLPaneModel {
	return SQLPaneModel{keys: k, queryLog: ql}
}

// SetFocused returns a copy with the focused flag set. Used at render time.
func (m SQLPaneModel) SetFocused(f bool) SQLPaneModel {
	m.focused = f
	return m
}

// Update handles window resizes and, when focused, up/down cursor navigation.
func (m SQLPaneModel) Update(msg tea.Msg) (SQLPaneModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyMsg:
		_, history := m.queryLog.Snapshot()
		switch {
		case key.Matches(msg, m.keys.Up):
			if m.cursor > 0 {
				m.cursor--
			}
		case key.Matches(msg, m.keys.Down):
			if m.cursor < len(history)-1 {
				m.cursor++
			}
		}
	}
	return m, nil
}

// View renders the SQL pane content (without its outer border — the App applies that).
func (m SQLPaneModel) View() string {
	if m.queryLog == nil || m.width == 0 {
		return ""
	}

	running, history := m.queryLog.Snapshot()

	maxCursor := len(history) - 1
	if m.cursor > maxCursor && maxCursor >= 0 {
		m.cursor = maxCursor
	}

	sqlW := m.width - 22
	if sqlW < 10 {
		sqlW = 10
	}

	var lines []string

	for _, e := range running {
		if len(lines) >= m.height {
			break
		}
		line := fmt.Sprintf(" ⠸ %-8s %-12s %s",
			formatDuration(e.Duration),
			truncate(e.Label, 12),
			truncate(e.SQL, sqlW),
		)
		lines = append(lines, style.StatusKey.Render(line))
	}

	for i := len(history) - 1; i >= 0; i-- {
		if len(lines) >= m.height {
			break
		}
		e := history[i]
		displayPos := len(history) - 1 - i
		line := fmt.Sprintf("   %-8s %-12s %s",
			formatDuration(e.Duration),
			truncate(e.Label, 12),
			truncate(e.SQL, sqlW),
		)
		if m.focused && displayPos == m.cursor {
			lines = append(lines, style.RowSelected.Width(m.width).Render(line))
		} else {
			lines = append(lines, style.StatusDesc.Render(line))
		}
	}

	if len(lines) == 0 {
		lines = append(lines, style.Muted.Render("  (no queries yet)"))
	}

	content := strings.Join(lines, "\n")
	return style.Content.Width(m.width).Height(m.height).Render(content)
}
