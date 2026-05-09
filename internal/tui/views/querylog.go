package views

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/polesen/datacow/internal/core/db"
	"github.com/polesen/datacow/internal/tui/style"
)

// QueryLogView renders the full-screen query log panel.
type QueryLogView struct {
	queryLog *db.QueryLog
	cursor   int
	width    int
	height   int
	spinChar string
}

// NewQueryLogView returns a QueryLogView backed by the given log.
func NewQueryLogView(ql *db.QueryLog) QueryLogView {
	return QueryLogView{
		queryLog: ql,
		spinChar: "⠸",
	}
}

// Update handles keyboard navigation within the query log.
func (v QueryLogView) Update(msg tea.Msg) (QueryLogView, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		_, history := v.queryLog.Snapshot()
		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("up", "k"))):
			if v.cursor > 0 {
				v.cursor--
			}
		case key.Matches(msg, key.NewBinding(key.WithKeys("down", "j"))):
			if v.cursor < len(history)-1 {
				v.cursor++
			}
		}
	case tea.WindowSizeMsg:
		v.width = msg.Width
		v.height = msg.Height
	}
	return v, nil
}

// SetSpinChar updates the spinner character shown next to running queries.
func (v QueryLogView) SetSpinChar(s string) QueryLogView {
	v.spinChar = s
	return v
}

// View renders the query log panel.
func (v QueryLogView) View() string {
	running, history := v.queryLog.Snapshot()

	// Clamp cursor
	if len(history) > 0 && v.cursor >= len(history) {
		v.cursor = len(history) - 1
	}

	var sb strings.Builder

	// --- Running section ---
	sb.WriteString(style.ColHeader.Render(fmt.Sprintf("Running (%d)", len(running))))
	sb.WriteString("\n")
	if len(running) == 0 {
		sb.WriteString("  (none)\n")
	} else {
		for _, e := range running {
			elapsed := formatDuration(e.Duration)
			kind := kindBadge(e.Kind)
			line := fmt.Sprintf("  %s %-30s %s   %s",
				v.spinChar,
				truncate(e.Label, 30),
				elapsed,
				kind,
			)
			sb.WriteString(line + "\n")
		}
	}

	sb.WriteString("\n")

	// --- History section ---
	// history is []QueryEntry newest-first (index 0 = newest); iterate forward so newest appears at the top.
	sb.WriteString(style.ColHeader.Render("History  (newest first)"))
	sb.WriteString("\n")
	if len(history) == 0 {
		sb.WriteString("  (none)\n")
	} else {
		for i, e := range history {
			ts := e.StartedAt.Format("15:04:05")
			kind := kindBadge(e.Kind)

			var durField, rowField string
			if e.Error != nil {
				durField = "ERR"
				rowField = truncate(e.Error.Error(), 14)
			} else {
				durField = formatDuration(e.Duration)
				rowField = formatRowCount(e.RowCount)
			}

			line := fmt.Sprintf("  %-8s %-6s %-30s %-14s %s",
				ts,
				durField,
				truncate(e.Label, 30),
				rowField,
				kind,
			)
			if i == v.cursor {
				line = style.RowSelected.Render(line)
			} else if e.Error != nil {
				line = style.Error.Render(line)
			} else {
				line = style.RowNormal.Render(line)
			}
			sb.WriteString(line + "\n")
		}
	}

	// --- SQL preview ---
	if len(history) > 0 && v.cursor < len(history) {
		selected := history[v.cursor]
		sb.WriteString("\n")

		atLine := fmt.Sprintf("At: %s  Duration: %s",
			selected.StartedAt.Format("15:04:05"),
			formatDuration(selected.Duration),
		)
		sb.WriteString(style.StatusDesc.Render(atLine))
		sb.WriteString("\n")

		if selected.Error != nil {
			sb.WriteString(style.StatusKey.Render("Error: "))
			maxW := v.width - 7
			if maxW < 20 {
				maxW = 20
			}
			sb.WriteString(style.Error.Render(truncate(selected.Error.Error(), maxW)))
			sb.WriteString("\n")
		}
		if selected.SQL != "" {
			sb.WriteString(style.StatusKey.Render("SQL: "))
			maxW := v.width - 5
			if maxW < 20 {
				maxW = 20
			}
			sb.WriteString(style.StatusDesc.Render(truncate(selected.SQL, maxW)))
			sb.WriteString("\n")
		}
	}

	content := sb.String()
	return lipgloss.NewStyle().
		Width(v.width).
		Height(v.height).
		Render(content)
}

func formatDuration(d time.Duration) string {
	if d < time.Millisecond {
		return fmt.Sprintf("%dµs", d.Microseconds())
	}
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
}

func formatRowCount(n int64) string {
	if n == 0 {
		return ""
	}
	return fmt.Sprintf("%s rows", formatInt(n))
}

func formatInt(n int64) string {
	s := fmt.Sprintf("%d", n)
	if n < 1000 {
		return s
	}
	// insert commas
	result := ""
	for i, ch := range s {
		pos := len(s) - i
		if i > 0 && pos%3 == 0 {
			result += ","
		}
		result += string(ch)
	}
	return result
}

func kindBadge(k db.QueryKind) string {
	switch k {
	case db.QueryKindSystem:
		return style.StatusDesc.Render("system")
	default:
		return style.StatusKey.Render("user")
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}
