package views

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/lipgloss"

	"github.com/polesen/datacow/internal/core/dataset"
)

// formatFilterLabel formats a filter as a human-readable string, e.g.
// "status = 'active'" or "price > 100".
// String values are wrapped in single quotes; numbers and booleans are not.
func formatFilterLabel(f dataset.Filter) string {
	v := fmt.Sprintf("%v", f.Value)
	if v != "true" && v != "false" {
		if _, err := strconv.ParseFloat(v, 64); err != nil {
			v = "'" + v + "'"
		}
	}
	return f.Column + " " + f.Operator + " " + v
}

func newSpinner() spinner.Model {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#7DCFFF"))
	return sp
}

func formatCount(n int64) string {
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}
	var b strings.Builder
	start := len(s) % 3
	if start > 0 {
		b.WriteString(s[:start])
	}
	for i := start; i < len(s); i += 3 {
		if b.Len() > 0 {
			b.WriteByte(',')
		}
		b.WriteString(s[i : i+3])
	}
	return b.String()
}
