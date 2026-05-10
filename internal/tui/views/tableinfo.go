package views

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/polesen/datacow/internal/core/db"
	"github.com/polesen/datacow/internal/tui/style"
)

// TableInfoModel displays catalog-sourced statistics for a single table.
type TableInfoModel struct {
	tableName string
	stats     *db.TableStats // nil = not yet loaded
	err       error
	loading   bool
	width     int
	height    int
	spinChar  string
}

type tableStatsLoadedMsg struct {
	tableName string
	stats     db.TableStats
	err       error
}

// NewTableInfoModel returns a TableInfoModel in the loading state.
func NewTableInfoModel() TableInfoModel {
	return TableInfoModel{loading: true}
}

// SetSize updates the display dimensions.
func (m *TableInfoModel) SetSize(w, h int) {
	m.width = w
	m.height = h
}

// SetSpinChar updates the spinner character shown while loading.
func (m *TableInfoModel) SetSpinChar(s string) {
	m.spinChar = s
}

// Load fires a tea.Cmd that fetches stats for the given table via client.
// If client does not implement StatsProvider the error is delivered immediately.
func (m TableInfoModel) Load(client db.Client, table string) tea.Cmd {
	return func() tea.Msg {
		sp, ok := client.(db.StatsProvider)
		if !ok {
			return tableStatsLoadedMsg{tableName: table, err: fmt.Errorf("statistics not available for this database type")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		stats, err := sp.TableStats(ctx, table)
		return tableStatsLoadedMsg{tableName: table, stats: stats, err: err}
	}
}

// Update handles incoming messages.
func (m TableInfoModel) Update(msg tea.Msg) (TableInfoModel, tea.Cmd) {
	if msg, ok := msg.(tableStatsLoadedMsg); ok {
		m.tableName = msg.tableName
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
		} else {
			s := msg.stats
			m.stats = &s
		}
	}
	return m, nil
}

// View renders the table info overlay.
func (m TableInfoModel) View() string {
	if m.width == 0 {
		return ""
	}

	var sb strings.Builder

	sb.WriteString("\n ")
	sb.WriteString(style.ColHeader.Render("Table Info — " + m.tableName))
	sb.WriteString("\n\n")

	switch {
	case m.loading:
		sb.WriteString(" ")
		sb.WriteString(m.spinChar)
		sb.WriteString(" loading…\n")

	case m.err != nil:
		sb.WriteString(" ")
		sb.WriteString(style.Error.Render(m.err.Error()))
		sb.WriteString("\n")

	case m.stats != nil:
		s := m.stats
		const labelW = 20

		writeRow := func(label, value string) {
			sb.WriteString(" ")
			sb.WriteString(style.StatusKey.Render(fmt.Sprintf("%-*s", labelW, label)))
			sb.WriteString(" ")
			sb.WriteString(value)
			sb.WriteString("\n")
		}

		if s.RowEstimate != nil {
			writeRow("Rows (estimate)", formatEstimate(*s.RowEstimate))
		}
		if s.TotalBytes != nil {
			writeRow("Total size", formatBytes(*s.TotalBytes))
		}
		if s.TableBytes != nil {
			writeRow("  Table data", formatBytes(*s.TableBytes))
		}
		if s.IndexBytes != nil {
			writeRow("  Indexes", formatBytes(*s.IndexBytes))
		}

		if s.Description != "" {
			sb.WriteString("\n")
			writeRow("Description", s.Description)
		}

		hasTime := s.LastAnalyzed != nil || s.LastVacuumed != nil || s.CreatedAt != nil
		if hasTime {
			sb.WriteString("\n")
		}
		if s.LastAnalyzed != nil {
			writeRow("Last analyzed", s.LastAnalyzed.Format("2006-01-02 15:04"))
		}
		if s.LastVacuumed != nil {
			writeRow("Last vacuumed", s.LastVacuumed.Format("2006-01-02 15:04"))
		}
		if s.CreatedAt != nil {
			writeRow("Created", s.CreatedAt.Format("2006-01-02 15:04"))
		}

		hasMeta := s.Engine != "" || s.FreeBytes != nil || s.NextAutoIncr != nil
		if hasMeta {
			sb.WriteString("\n")
		}
		if s.Engine != "" {
			writeRow("Engine", s.Engine)
		}
		if s.FreeBytes != nil {
			writeRow("Free space", formatBytes(*s.FreeBytes))
		}
		if s.NextAutoIncr != nil {
			writeRow("Auto-increment", formatCount(*s.NextAutoIncr))
		}
	}

	sb.WriteString("\n")
	sb.WriteString(style.Muted.Render("  i or esc   close"))

	return lipgloss.NewStyle().
		Width(m.width).
		Height(m.height).
		Render(sb.String())
}

// formatBytes converts a byte count to a human-readable string.
func formatBytes(n int64) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
		TB = 1024 * GB
	)
	switch {
	case n < KB:
		return fmt.Sprintf("%d B", n)
	case n < MB:
		return fmt.Sprintf("%d KB", n/KB)
	case n < GB:
		return fmt.Sprintf("%.1f MB", float64(n)/MB)
	case n < TB:
		return fmt.Sprintf("%.1f GB", float64(n)/GB)
	default:
		return fmt.Sprintf("%.1f TB", float64(n)/TB)
	}
}

// formatEstimate formats a row count estimate.
// 0 is rendered as "0"; ≥1000 gets a ~ prefix and K/M/B suffix.
func formatEstimate(n int64) string {
	switch {
	case n == 0:
		return "0"
	case n < 1_000:
		return fmt.Sprintf("%d", n)
	case n < 1_000_000:
		return fmt.Sprintf("~%dK", n/1_000)
	case n < 1_000_000_000:
		return fmt.Sprintf("~%.1fM", float64(n)/1_000_000)
	default:
		return fmt.Sprintf("~%.1fB", float64(n)/1_000_000_000)
	}
}
