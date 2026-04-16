package views

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/beetio/datacow/internal/core/dataset"
	"github.com/beetio/datacow/internal/core/db"
	"github.com/beetio/datacow/internal/tui/keys"
	"github.com/beetio/datacow/internal/tui/style"
)

// RowsLoadedMsg is sent when a page of rows has been fetched.
type RowsLoadedMsg *dataset.QueryResult

// RowBrowserModel renders a paginated table of rows for a single dataset.
type RowBrowserModel struct {
	ds        dataset.Dataset
	result    *dataset.QueryResult
	colWidths []int
	colOffset int
	spinner   spinner.Model
	loading   bool
	err       error
	keys      keys.Map
	width     int
	height    int
	executor  *dataset.Executor
}

// NewRowBrowserModel creates a RowBrowserModel in the initial loading state.
// executor may be nil for testing.
func NewRowBrowserModel(k keys.Map, executor *dataset.Executor, ds dataset.Dataset) RowBrowserModel {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#7DCFFF"))
	return RowBrowserModel{
		ds:       ds,
		spinner:  sp,
		loading:  true,
		keys:     k,
		executor: executor,
	}
}

// Init starts the spinner and loads the first page.
func (m RowBrowserModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.loadPageCmd(1))
}

func (m RowBrowserModel) loadPageCmd(page int) tea.Cmd {
	if m.executor == nil {
		return nil
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		result, err := m.executor.Query(ctx, m.ds, dataset.QueryOptions{
			Page:     page,
			PageSize: 50,
		})
		if err != nil {
			return ErrMsg{err}
		}
		return RowsLoadedMsg(result)
	}
}

// Update processes incoming messages and key events.
func (m RowBrowserModel) Update(msg tea.Msg) (RowBrowserModel, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case spinner.TickMsg:
		if m.loading {
			m.spinner, cmd = m.spinner.Update(msg)
		}
		return m, cmd

	case RowsLoadedMsg:
		m.result = (*dataset.QueryResult)(msg)
		m.loading = false
		m.colWidths = computeColWidths(m.result.Columns, m.result.Rows)
		m.colOffset = 0
		return m, nil

	case ErrMsg:
		m.loading = false
		m.err = msg.Err
		return m, nil

	case tea.KeyMsg:
		if m.loading || m.err != nil || m.result == nil {
			return m, nil
		}
		switch {
		case key.Matches(msg, m.keys.NextPage):
			if m.result.Page < m.result.TotalPages {
				m.loading = true
				return m, tea.Batch(m.spinner.Tick, m.loadPageCmd(m.result.Page+1))
			}
		case key.Matches(msg, m.keys.PrevPage):
			if m.result.Page > 1 {
				m.loading = true
				return m, tea.Batch(m.spinner.Tick, m.loadPageCmd(m.result.Page-1))
			}
		case key.Matches(msg, m.keys.Right):
			if m.colOffset < len(m.result.Columns)-1 {
				m.colOffset++
			}
		case key.Matches(msg, m.keys.Left):
			if m.colOffset > 0 {
				m.colOffset--
			}
		}
		return m, nil
	}

	return m, cmd
}

// Page returns the current page number, or 0 if no data loaded.
func (m RowBrowserModel) Page() int {
	if m.result == nil {
		return 0
	}
	return m.result.Page
}

// TotalPages returns the total number of pages, or 0 if no data loaded.
func (m RowBrowserModel) TotalPages() int {
	if m.result == nil {
		return 0
	}
	return m.result.TotalPages
}

// TotalRows returns the total row count, or 0 if no data loaded.
func (m RowBrowserModel) TotalRows() int64 {
	if m.result == nil {
		return 0
	}
	return m.result.TotalRows
}

// ColOffset returns the current horizontal scroll column index.
func (m RowBrowserModel) ColOffset() int { return m.colOffset }

// IsLoading reports whether data is being fetched.
func (m RowBrowserModel) IsLoading() bool { return m.loading }

// Err returns the current error, if any.
func (m RowBrowserModel) Err() error { return m.err }

// StatusLine returns a compact summary for the App's status bar.
func (m RowBrowserModel) StatusLine() string {
	if m.result == nil {
		return m.ds.Name
	}
	return fmt.Sprintf("%s  page %d/%d  %s rows",
		m.ds.Name,
		m.result.Page,
		m.result.TotalPages,
		formatCount(m.result.TotalRows),
	)
}

// View renders the row browser panel.
func (m RowBrowserModel) View() string {
	if m.width == 0 {
		return ""
	}

	if m.loading {
		return style.Content.Width(m.width).Height(m.height).Render(
			m.spinner.View() + " Loading...",
		)
	}

	if m.err != nil {
		errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#F7768E"))
		return style.Content.Width(m.width).Height(m.height).Render(
			errStyle.Render("Error: " + m.err.Error()),
		)
	}

	if m.result == nil {
		return style.Content.Width(m.width).Height(m.height).Render("")
	}

	return m.renderTable()
}

func (m RowBrowserModel) renderTable() string {
	cols := m.result.Columns
	rows := m.result.Rows

	if len(cols) == 0 {
		return style.Content.Width(m.width).Height(m.height).Render("No columns.")
	}

	visible := visibleColumns(cols, m.colWidths, m.colOffset, m.width)
	if len(visible) == 0 && len(cols) > 0 {
		// Always show at least one column
		visible = []int{m.colOffset}
	}

	header := buildHeader(cols, m.colWidths, visible)
	sep := buildSeparator(m.colWidths, visible)

	// rows area: total height minus header and separator lines
	maxRows := m.height - 2
	if maxRows < 0 {
		maxRows = 0
	}

	lines := make([]string, 0, maxRows+2)
	lines = append(lines, header, sep)

	for i, row := range rows {
		if i >= maxRows {
			break
		}
		lines = append(lines, buildRow(row, cols, m.colWidths, visible))
	}

	return strings.Join(lines, "\n")
}

// visibleColumns returns the column indices visible from offset within totalWidth.
func visibleColumns(cols []db.Column, widths []int, offset, totalWidth int) []int {
	var visible []int
	used := 0
	for i := offset; i < len(cols); i++ {
		w := widths[i] + 2 // +2 for inter-column gap
		if used > 0 && used+w > totalWidth {
			break
		}
		visible = append(visible, i)
		used += w
	}
	return visible
}

func buildHeader(cols []db.Column, widths []int, visible []int) string {
	parts := make([]string, len(visible))
	for j, i := range visible {
		name := truncateStr(cols[i].Name, widths[i])
		cell := fmt.Sprintf("%-*s", widths[i], name)
		parts[j] = style.ColHeader.Render(cell)
	}
	return strings.Join(parts, "  ")
}

func buildSeparator(widths []int, visible []int) string {
	parts := make([]string, len(visible))
	for j, i := range visible {
		parts[j] = style.Separator.Render(strings.Repeat("─", widths[i]))
	}
	return strings.Join(parts, "  ")
}

func buildRow(row map[string]any, cols []db.Column, widths []int, visible []int) string {
	parts := make([]string, len(visible))
	for j, i := range visible {
		v := row[cols[i].Name]
		if v == nil {
			cell := fmt.Sprintf("%-*s", widths[i], truncateStr("null", widths[i]))
			parts[j] = style.NullValue.Render(cell)
		} else {
			s := truncateStr(formatCellValue(v), widths[i])
			parts[j] = fmt.Sprintf("%-*s", widths[i], s)
		}
	}
	return strings.Join(parts, "  ")
}

// computeColWidths calculates display widths for each column, capped at 40.
func computeColWidths(cols []db.Column, rows []map[string]any) []int {
	const maxColWidth = 40
	widths := make([]int, len(cols))
	for i, col := range cols {
		widths[i] = len([]rune(col.Name))
	}
	for _, row := range rows {
		for i, col := range cols {
			s := formatCellValue(row[col.Name])
			if w := len([]rune(s)); w > widths[i] {
				widths[i] = w
			}
		}
	}
	for i := range widths {
		if widths[i] > maxColWidth {
			widths[i] = maxColWidth
		}
		if widths[i] < 1 {
			widths[i] = 1
		}
	}
	return widths
}

// formatCellValue converts any cell value to its display string.
func formatCellValue(v any) string {
	if v == nil {
		return "null"
	}
	switch val := v.(type) {
	case []byte:
		return string(val)
	case string:
		return val
	case bool:
		if val {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprintf("%v", val)
	}
}

// truncateStr truncates s to at most max runes, appending "…" if truncated.
func truncateStr(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	if max <= 1 {
		return "…"
	}
	return string(r[:max-1]) + "…"
}
