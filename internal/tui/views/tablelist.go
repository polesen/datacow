package views

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-runewidth"

	"github.com/beetio/datacow/internal/core/dataset"
	"github.com/beetio/datacow/internal/core/db"
	"github.com/beetio/datacow/internal/tui/keys"
	"github.com/beetio/datacow/internal/tui/style"
)

type ErrMsg struct{ Err error }

type TablesLoadedMsg []dataset.Dataset

type RowCountMsg struct {
	Name  string
	Count int64
}

// ExpansionLoadedMsg carries the columns + FKs for a dataset's expanded view.
type ExpansionLoadedMsg struct {
	Idx  int
	Cols []db.Column
	FKs  []db.ForeignKey
	Err  error
}

// IndexesLoadedMsg carries the indexes for a dataset's expanded view.
type IndexesLoadedMsg struct {
	Idx     int
	Indexes []db.Index
	Err     error
}

type indexLoadState int

const (
	indexIdle indexLoadState = iota
	indexLoading
	indexLoaded
	indexError
)

type expansionLoadState int

const (
	expIdle expansionLoadState = iota
	expLoading
	expLoaded
	expError
)

// treeNode holds the per-dataset expand state and lazily-loaded introspection data.
type treeNode struct {
	expanded   bool
	expState   expansionLoadState
	expErr     error
	cols       []db.Column
	fks        []db.ForeignKey
	indexState indexLoadState
	indexErr   error
	indexes    []db.Index
}

type TableListModel struct {
	datasets     []dataset.Dataset
	tree         []treeNode
	counts       map[string]int64
	cursor       int
	scrollOffset int // in visible-line space (not dataset-index space)
	nextCountIdx int
	spinner      spinner.Model
	loading      bool
	err          error
	keys         keys.Map
	width        int
	height       int
	resolver     *dataset.Resolver
	executor     *dataset.Executor
	client       db.Client
}

// NewTableListModel creates a TableListModel in the initial loading state.
// resolver, executor, and client may be nil for testing.
func NewTableListModel(k keys.Map, resolver *dataset.Resolver, executor *dataset.Executor, client db.Client) TableListModel {
	return TableListModel{
		spinner:  newSpinner(),
		loading:  true,
		keys:     k,
		counts:   make(map[string]int64),
		resolver: resolver,
		executor: executor,
		client:   client,
	}
}

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

func (m TableListModel) loadExpansionCmd(idx int, ds dataset.Dataset) tea.Cmd {
	if m.client == nil {
		return nil
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		cols, err := m.client.Describe(ctx, ds.Table)
		if err != nil {
			return ExpansionLoadedMsg{Idx: idx, Err: err}
		}
		fks, err := m.client.ForeignKeys(ctx, ds.Table)
		if err != nil {
			return ExpansionLoadedMsg{Idx: idx, Cols: cols, Err: err}
		}
		return ExpansionLoadedMsg{Idx: idx, Cols: cols, FKs: fks}
	}
}

func (m TableListModel) loadIndexesCmd(idx int, ds dataset.Dataset) tea.Cmd {
	if m.client == nil {
		return nil
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		idxs, err := m.client.Indexes(ctx, ds.Table)
		if err != nil {
			return IndexesLoadedMsg{Idx: idx, Err: err}
		}
		return IndexesLoadedMsg{Idx: idx, Indexes: idxs}
	}
}

func (m TableListModel) Update(msg tea.Msg) (TableListModel, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ensureCursorVisible()
		return m, nil

	case spinner.TickMsg:
		if m.loading || m.anyLoading() {
			m.spinner, cmd = m.spinner.Update(msg)
		}
		return m, cmd

	case TablesLoadedMsg:
		m.datasets = []dataset.Dataset(msg)
		m.tree = make([]treeNode, len(m.datasets))
		m.loading = false
		if len(m.datasets) == 0 {
			return m, nil
		}
		const maxConcurrent = 5
		limit := min(len(m.datasets), maxConcurrent)
		cmds := make([]tea.Cmd, limit)
		for i := range limit {
			cmds[i] = m.loadRowCountCmd(m.datasets[i])
		}
		m.nextCountIdx = limit
		return m, tea.Batch(cmds...)

	case RowCountMsg:
		m.counts[msg.Name] = msg.Count
		if m.nextCountIdx < len(m.datasets) {
			cmd = m.loadRowCountCmd(m.datasets[m.nextCountIdx])
			m.nextCountIdx++
		}
		return m, cmd

	case ExpansionLoadedMsg:
		if msg.Idx < 0 || msg.Idx >= len(m.tree) {
			return m, nil
		}
		n := &m.tree[msg.Idx]
		n.cols = msg.Cols
		n.fks = msg.FKs
		if msg.Err != nil {
			n.expState = expError
			n.expErr = msg.Err
		} else {
			n.expState = expLoaded
		}
		return m, nil

	case IndexesLoadedMsg:
		if msg.Idx < 0 || msg.Idx >= len(m.tree) {
			return m, nil
		}
		n := &m.tree[msg.Idx]
		n.indexes = msg.Indexes
		if msg.Err != nil {
			n.indexState = indexError
			n.indexErr = msg.Err
		} else {
			n.indexState = indexLoaded
		}
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
				m.ensureCursorVisible()
			}
		case key.Matches(msg, m.keys.Down):
			if m.cursor < len(m.datasets)-1 {
				m.cursor++
				m.ensureCursorVisible()
			}
		case key.Matches(msg, m.keys.Right):
			if m.FocusedExpandable() && !m.FocusedExpanded() {
				return m.expandFocused()
			}
		case key.Matches(msg, m.keys.Left):
			if m.FocusedExpanded() {
				m.tree[m.cursor].expanded = false
				m.ensureCursorVisible()
			}
		}
		return m, nil
	}

	return m, nil
}

// FocusedExpandable reports whether the currently-focused row can be expanded
// (i.e. is a table or view — not a YAML SQL dataset).
func (m TableListModel) FocusedExpandable() bool {
	if m.cursor < 0 || m.cursor >= len(m.datasets) {
		return false
	}
	return m.datasets[m.cursor].Kind != dataset.KindDataset
}

// FocusedExpanded reports whether the currently-focused row is expanded.
func (m TableListModel) FocusedExpanded() bool {
	if m.cursor < 0 || m.cursor >= len(m.tree) {
		return false
	}
	return m.tree[m.cursor].expanded
}

func (m TableListModel) expandFocused() (TableListModel, tea.Cmd) {
	idx := m.cursor
	n := &m.tree[idx]
	n.expanded = true
	ds := m.datasets[idx]
	var cmds []tea.Cmd
	if n.expState == expIdle {
		n.expState = expLoading
		if c := m.loadExpansionCmd(idx, ds); c != nil {
			cmds = append(cmds, c)
		}
	}
	// Views don't carry indexes in any meaningful way — skip the lookup.
	if ds.Kind == dataset.KindView {
		if n.indexState == indexIdle {
			n.indexState = indexLoaded
		}
	} else if n.indexState == indexIdle {
		n.indexState = indexLoading
		if c := m.loadIndexesCmd(idx, ds); c != nil {
			cmds = append(cmds, c)
		}
	}
	m.ensureCursorVisible()
	if len(cmds) == 0 {
		return m, nil
	}
	return m, tea.Batch(cmds...)
}

func (m *TableListModel) anyLoading() bool {
	for _, n := range m.tree {
		if n.expState == expLoading || n.indexState == indexLoading {
			return true
		}
	}
	return false
}

func (m TableListModel) SelectedDataset() *dataset.Dataset {
	if len(m.datasets) == 0 || m.cursor >= len(m.datasets) {
		return nil
	}
	ds := m.datasets[m.cursor]
	return &ds
}

func (m TableListModel) IsLoading() bool   { return m.loading }
func (m TableListModel) DatasetCount() int { return len(m.datasets) }
func (m TableListModel) Cursor() int       { return m.cursor }
func (m TableListModel) Err() error        { return m.err }

// visibleLine describes one rendered line in the list.
type visibleLine struct {
	datasetIdx int  // header row for this dataset
	isHeader   bool // true for the dataset row; false for expanded sub-rows
	sub        string
}

// buildLines flattens datasets + tree into a sequence of rendered lines.
func (m TableListModel) buildLines() []visibleLine {
	out := make([]visibleLine, 0, len(m.datasets))
	for i, ds := range m.datasets {
		out = append(out, visibleLine{datasetIdx: i, isHeader: true})
		if i >= len(m.tree) || !m.tree[i].expanded {
			continue
		}
		for _, ln := range m.subLines(i, ds) {
			out = append(out, visibleLine{datasetIdx: i, sub: ln})
		}
	}
	return out
}

// subLines returns the tree-drawing sub-rows for an expanded dataset.
func (m TableListModel) subLines(idx int, ds dataset.Dataset) []string {
	n := m.tree[idx]
	var lines []string

	// Columns
	lines = append(lines, "  ├─ Columns")
	switch n.expState {
	case expLoading:
		lines = append(lines, "  │   "+m.spinner.View()+" loading…")
	case expError:
		lines = append(lines, "  │   (error)")
	case expLoaded:
		if len(n.cols) == 0 {
			lines = append(lines, "  │   (none)")
		} else {
			for _, c := range n.cols {
				lines = append(lines, "  │   "+formatColumn(c))
			}
		}
	}

	// Indexes (only for tables — views skip this section's body).
	lines = append(lines, "  ├─ Indexes")
	if ds.Kind == dataset.KindView {
		lines = append(lines, "  │   (n/a for views)")
	} else {
		switch n.indexState {
		case indexLoading:
			lines = append(lines, "  │   "+m.spinner.View()+" loading…")
		case indexError:
			lines = append(lines, "  │   (error)")
		case indexLoaded:
			if len(n.indexes) == 0 {
				lines = append(lines, "  │   (none)")
			} else {
				for _, ix := range n.indexes {
					lines = append(lines, "  │   "+formatIndex(ix))
				}
			}
		}
	}

	// Foreign keys
	lines = append(lines, "  └─ Foreign Keys")
	switch n.expState {
	case expLoaded:
		if len(n.fks) == 0 {
			lines = append(lines, "      (none)")
		} else {
			for _, fk := range n.fks {
				lines = append(lines, "      "+formatFK(fk))
			}
		}
	case expError:
		lines = append(lines, "      (error)")
	}

	return lines
}

func (m *TableListModel) ensureCursorVisible() {
	if m.height <= 0 {
		m.scrollOffset = 0
		return
	}
	lines := m.buildLines()
	// Find the line index of the cursor's header row.
	cursorLine := -1
	for i, ln := range lines {
		if ln.isHeader && ln.datasetIdx == m.cursor {
			cursorLine = i
			break
		}
	}
	if cursorLine < 0 {
		return
	}
	if cursorLine < m.scrollOffset {
		m.scrollOffset = cursorLine
	} else if cursorLine >= m.scrollOffset+m.height {
		m.scrollOffset = cursorLine - m.height + 1
	}
	m.scrollOffset = max(m.scrollOffset, 0)
	maxOffset := max(len(lines)-m.height, 0)
	m.scrollOffset = min(m.scrollOffset, maxOffset)
}

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
		return style.Content.Width(m.width).Height(m.height).Render(
			style.Error.Render("Error: " + m.err.Error()),
		)
	}

	if len(m.datasets) == 0 {
		return style.Content.Width(m.width).Height(m.height).Render("No tables found.")
	}

	lines := m.buildLines()
	maxVisible := m.height
	if maxVisible <= 0 {
		maxVisible = len(lines)
	}

	end := min(m.scrollOffset+maxVisible, len(lines))

	rendered := make([]string, 0, end-m.scrollOffset)
	for i := m.scrollOffset; i < end; i++ {
		ln := lines[i]
		if ln.isHeader {
			rendered = append(rendered, m.renderHeaderRow(ln.datasetIdx))
		} else {
			sub := runewidth.Truncate(ln.sub, m.width, "")
			rendered = append(rendered, style.RowNormal.Width(m.width).Render(sub))
		}
	}

	return style.Content.Width(m.width).Height(m.height).Render(
		strings.Join(rendered, "\n"),
	)
}

func (m TableListModel) renderHeaderRow(i int) string {
	ds := m.datasets[i]
	const maxNameWidth = 40
	const countWidth = 12
	const margin = 2

	badge := datasetKindBadge(ds.Kind)
	var badgeW int
	if badge != "" {
		badgeW = runewidth.StringWidth(badge) + 1 // leading space
	}

	name := runewidth.Truncate(ds.Name, maxNameWidth, "…")

	count := "..."
	if c, ok := m.counts[ds.Name]; ok {
		if c < 0 {
			count = "?"
		} else {
			count = formatCount(c)
		}
	}

	nameWidth := min(max(m.width-countWidth-margin*2, 10), maxNameWidth)

	selected := i == m.cursor
	caret := "  "
	if m.tree != nil && i < len(m.tree) {
		switch {
		case m.tree[i].expanded:
			caret = "▼ "
		case ds.Kind == dataset.KindDataset:
			caret = "  "
		default:
			caret = "▶ "
		}
	}

	var line string
	if badge != "" {
		availNameW := max(nameWidth-badgeW, 1)
		label := " " + style.QueryLabel.Render(badge)
		if selected {
			label = " " + badge
		}
		line = caret + runewidth.FillRight(name, availNameW) + label + fmt.Sprintf("%*s", countWidth, count)
	} else {
		line = caret + runewidth.FillRight(name, nameWidth) + fmt.Sprintf("%*s", countWidth, count)
	}

	if selected {
		return style.RowSelected.Width(m.width).Render(line)
	}
	return style.RowNormal.Width(m.width).Render(line)
}

func datasetKindBadge(k dataset.Kind) string {
	switch k {
	case dataset.KindView:
		return "[view]"
	case dataset.KindDataset:
		return "[dataset]"
	default:
		return ""
	}
}

func formatColumn(c db.Column) string {
	suffix := ""
	if !c.Nullable {
		suffix = "  NN"
	}
	return fmt.Sprintf("%-20s %s%s", runewidth.Truncate(c.Name, 20, "…"), c.Type, suffix)
}

func formatIndex(ix db.Index) string {
	cols := "(" + strings.Join(ix.Columns, ", ") + ")"
	if ix.Unique {
		return fmt.Sprintf("%-20s %s UNIQUE", runewidth.Truncate(ix.Name, 20, "…"), cols)
	}
	return fmt.Sprintf("%-20s %s", runewidth.Truncate(ix.Name, 20, "…"), cols)
}

func formatFK(fk db.ForeignKey) string {
	return fmt.Sprintf("%s → %s.%s", fk.Column, fk.ReferencedTable, fk.ReferencedColumn)
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
