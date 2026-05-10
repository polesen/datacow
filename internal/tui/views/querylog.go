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
	queryLog   *db.QueryLog
	cursor     int
	width      int
	height     int
	spinChar   string
	showSystem bool
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
		filtered := filterHistory(history, v.showSystem)
		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("up", "k"))):
			if v.cursor > 0 {
				v.cursor--
			}
		case key.Matches(msg, key.NewBinding(key.WithKeys("down", "j"))):
			if v.cursor < len(filtered)-1 {
				v.cursor++
			}
		case key.Matches(msg, key.NewBinding(key.WithKeys("s"))):
			v.showSystem = !v.showSystem
			v.cursor = 0
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
	if v.height == 0 {
		return ""
	}

	running, fullHistory := v.queryLog.Snapshot()
	history := filterHistory(fullHistory, v.showSystem)

	// Clamp cursor to valid range (local copy only — value receiver).
	if len(history) > 0 && v.cursor >= len(history) {
		v.cursor = len(history) - 1
	}

	var sb strings.Builder
	usedLines := 0

	// --- Running section ---
	sb.WriteString(style.ColHeader.Render(fmt.Sprintf("Running (%d)", len(running))))
	sb.WriteString("\n")
	usedLines++
	if len(running) == 0 {
		sb.WriteString("  (none)\n")
		usedLines++
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
			usedLines++
		}
	}
	sb.WriteString("\n")
	usedLines++

	// --- History section ---
	// history is []QueryEntry newest-first (index 0 = newest); iterate forward so newest appears at the top.
	var filterLabel string
	if v.showSystem {
		filterLabel = "  [all queries — s: user only]"
	} else {
		filterLabel = "  [user only — s: show all]"
	}
	sb.WriteString(style.ColHeader.Render("History  (newest first)" + filterLabel))
	sb.WriteString("\n")
	usedLines++

	// Reserve lines for the SQL preview so it is always visible.
	previewLines := 0
	if len(history) > 0 {
		sel := history[v.cursor]
		previewLines = 2 // blank separator + At/Duration line
		if sel.Error != nil {
			previewLines++
		}
		if sel.SQL != "" {
			previewLines++
		}
	}

	// Number of history rows that fit between the headers and the preview.
	availRows := v.height - usedLines - previewLines
	if availRows < 1 {
		availRows = 1
	}

	if len(history) == 0 {
		sb.WriteString("  (none)\n")
	} else {
		// Scroll offset: keep cursor within the visible window.
		// When cursor moves below the last visible row, shift the window down.
		scrollOffset := 0
		if v.cursor >= availRows {
			scrollOffset = v.cursor - availRows + 1
		}

		end := scrollOffset + availRows
		if end > len(history) {
			end = len(history)
		}

		for i := scrollOffset; i < end; i++ {
			e := history[i]
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

	// --- SQL preview (always visible — space reserved above) ---
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

func filterHistory(history []db.QueryEntry, showSystem bool) []db.QueryEntry {
	if showSystem {
		return history
	}
	var out []db.QueryEntry
	for _, e := range history {
		if e.Kind == db.QueryKindUser {
			out = append(out, e)
		}
	}
	return out
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
