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
	"github.com/beetio/datacow/internal/tui/keys"
	"github.com/beetio/datacow/internal/tui/style"
)

// ErrMsg is sent when an async operation fails.
type ErrMsg struct{ Err error }

// TablesLoadedMsg is sent when the table list has been resolved.
type TablesLoadedMsg []dataset.Dataset

// RowCountMsg is sent when the row count for a single table has been fetched.
type RowCountMsg struct {
	Name  string
	Count int64
}

// TableListModel renders a navigable list of all tables with lazy-loaded row counts.
type TableListModel struct {
	datasets     []dataset.Dataset
	counts       map[string]int64
	cursor       int
	scrollOffset int
	spinner      spinner.Model
	loading      bool
	err          error
	keys         keys.Map
	width        int
	height       int
	resolver     *dataset.Resolver
	executor     *dataset.Executor
}

// NewTableListModel creates a TableListModel in the initial loading state.
// resolver and executor may be nil for testing.
func NewTableListModel(k keys.Map, resolver *dataset.Resolver, executor *dataset.Executor) TableListModel {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#7DCFFF"))
	return TableListModel{
		spinner:  sp,
		loading:  true,
		keys:     k,
		counts:   make(map[string]int64),
		resolver: resolver,
		executor: executor,
	}
}

// Init starts the spinner and kicks off table discovery.
func (m TableListModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.loadTablesCmd())
}

func (m TableListModel) loadTablesCmd() tea.Cmd {
	if m.resolver == nil {
		return nil
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		datasets, err := m.resolver.Resolve(ctx)
		if err != nil {
			return ErrMsg{err}
		}
		return TablesLoadedMsg(datasets)
	}
}

func (m TableListModel) loadRowCountCmd(ds dataset.Dataset) tea.Cmd {
	if m.executor == nil {
		return nil
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		result, err := m.executor.Query(ctx, ds, dataset.QueryOptions{Page: 1, PageSize: 1})
		if err != nil {
			return RowCountMsg{Name: ds.Name, Count: -1}
		}
		return RowCountMsg{Name: ds.Name, Count: result.TotalRows}
	}
}

// Update processes all incoming messages and key events.
func (m TableListModel) Update(msg tea.Msg) (TableListModel, tea.Cmd) {
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

	case TablesLoadedMsg:
		m.datasets = []dataset.Dataset(msg)
		m.loading = false
		if len(m.datasets) == 0 {
			return m, nil
		}
		cmds := make([]tea.Cmd, len(m.datasets))
		for i, ds := range m.datasets {
			cmds[i] = m.loadRowCountCmd(ds)
		}
		return m, tea.Batch(cmds...)

	case RowCountMsg:
		m.counts[msg.Name] = msg.Count
		return m, nil

	case ErrMsg:
		m.loading = false
		m.err = msg.Err
		return m, nil

	case tea.KeyMsg:
		if m.loading || m.err != nil || len(m.datasets) == 0 {
			return m, nil
		}
		switch {
		case key.Matches(msg, m.keys.Up):
			if m.cursor > 0 {
				m.cursor--
				if m.cursor < m.scrollOffset {
					m.scrollOffset--
				}
			}
		case key.Matches(msg, m.keys.Down):
			if m.cursor < len(m.datasets)-1 {
				m.cursor++
				if m.height > 0 && m.cursor >= m.scrollOffset+m.height {
					m.scrollOffset++
				}
			}
		}
		return m, nil
	}

	return m, nil
}

// SelectedDataset returns the dataset at the cursor position, or nil.
func (m TableListModel) SelectedDataset() *dataset.Dataset {
	if len(m.datasets) == 0 || m.cursor >= len(m.datasets) {
		return nil
	}
	ds := m.datasets[m.cursor]
	return &ds
}

// IsLoading reports whether data is still being fetched.
func (m TableListModel) IsLoading() bool { return m.loading }

// DatasetCount returns the number of loaded datasets.
func (m TableListModel) DatasetCount() int { return len(m.datasets) }

// Cursor returns the current cursor position.
func (m TableListModel) Cursor() int { return m.cursor }

// Err returns the current error, if any.
func (m TableListModel) Err() error { return m.err }

// View renders the table list panel.
func (m TableListModel) View() string {
	if m.width == 0 {
		return ""
	}

	if m.loading {
		return style.Content.Width(m.width).Height(m.height).Render(
			m.spinner.View() + " Connecting...",
		)
	}

	if m.err != nil {
		errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#F7768E"))
		return style.Content.Width(m.width).Height(m.height).Render(
			errStyle.Render("Error: "+m.err.Error()),
		)
	}

	if len(m.datasets) == 0 {
		return style.Content.Width(m.width).Height(m.height).Render("No tables found.")
	}

	maxVisible := m.height
	if maxVisible <= 0 {
		maxVisible = len(m.datasets)
	}

	lines := make([]string, 0, maxVisible)
	end := m.scrollOffset + maxVisible
	if end > len(m.datasets) {
		end = len(m.datasets)
	}
	for i := m.scrollOffset; i < end; i++ {
		lines = append(lines, m.renderTableRow(i, m.datasets[i]))
	}

	return strings.Join(lines, "\n")
}

func (m TableListModel) renderTableRow(i int, ds dataset.Dataset) string {
	const maxNameWidth = 40
	const countWidth = 12
	const margin = 2

	name := ds.Name
	if len([]rune(name)) > maxNameWidth {
		r := []rune(name)
		name = string(r[:maxNameWidth-1]) + "…"
	}

	count := "..."
	if c, ok := m.counts[ds.Name]; ok {
		if c < 0 {
			count = "?"
		} else {
			count = formatCount(c)
		}
	}

	nameWidth := m.width - countWidth - margin*2
	if nameWidth < 10 {
		nameWidth = 10
	}
	if nameWidth > maxNameWidth {
		nameWidth = maxNameWidth
	}

	line := fmt.Sprintf("  %-*s%*s", nameWidth, name, countWidth, count)

	if i == m.cursor {
		return style.RowSelected.Width(m.width).Render(line)
	}
	return style.RowNormal.Width(m.width).Render(line)
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
