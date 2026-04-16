package views

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-runewidth"

	"github.com/beetio/datacow/internal/core/dataset"
	"github.com/beetio/datacow/internal/core/db"
	"github.com/beetio/datacow/internal/core/export"
	"github.com/beetio/datacow/internal/tui/keys"
	"github.com/beetio/datacow/internal/tui/style"
)

type RowsLoadedMsg *dataset.QueryResult

type uiMode int

const (
	modeNormal      uiMode = iota
	modeFilterInput        // filter expression bar at bottom
	modeFilterPills        // navigating filter pills to remove
	modeExportMenu         // choosing export format
	modeExporting          // export in progress
)

// exportEvent is sent by the export goroutine to report progress and completion.
type exportEvent struct {
	n    int    // rows written so far
	done bool   // true when export is finished
	path string // set on successful completion
	err  error  // set on error completion
}

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

	filters        []dataset.Filter
	sort           *dataset.Sort
	mode           uiMode
	filterInput    textinput.Model
	filterPillIdx  int
	exportProgress int
	statusMsg      string
	exporter       *export.Exporter
	exportCh       chan exportEvent
}

// NewRowBrowserModel creates a RowBrowserModel in the initial loading state.
// executor and exporter may be nil for testing.
func NewRowBrowserModel(k keys.Map, executor *dataset.Executor, exporter *export.Exporter, ds dataset.Dataset) RowBrowserModel {
	ti := textinput.New()
	ti.Placeholder = "column=value  (ops: = > < >= <= like)"
	ti.Prompt = "Filter: "
	ti.CharLimit = 200

	return RowBrowserModel{
		ds:          ds,
		spinner:     newSpinner(),
		loading:     true,
		keys:        k,
		executor:    executor,
		exporter:    exporter,
		filterInput: ti,
	}
}

func (m RowBrowserModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.loadPageCmd(1))
}

func (m RowBrowserModel) loadPageCmd(page int) tea.Cmd {
	if m.executor == nil {
		return nil
	}
	filters := m.filters
	sort := m.sort
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		result, err := m.executor.Query(ctx, m.ds, dataset.QueryOptions{
			Page:     page,
			PageSize: 50,
			Filters:  filters,
			Sort:     sort,
		})
		if err != nil {
			return ErrMsg{err}
		}
		return RowsLoadedMsg(result)
	}
}

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
		return m, nil

	case ErrMsg:
		m.loading = false
		m.err = msg.Err
		return m, nil

	case exportEvent:
		m.exportProgress = msg.n
		if msg.done {
			if msg.err != nil {
				m.statusMsg = "Export failed: " + msg.err.Error()
			} else {
				m.statusMsg = fmt.Sprintf("Exported %s rows → %s", formatCount(int64(msg.n)), msg.path)
			}
			m.mode = modeNormal
			m.exportCh = nil
			return m, nil
		}
		ch := m.exportCh
		return m, func() tea.Msg { return <-ch }

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, cmd
}

func (m RowBrowserModel) handleKey(msg tea.KeyMsg) (RowBrowserModel, tea.Cmd) {
	switch m.mode {
	case modeFilterInput:
		return m.handleFilterInputKey(msg)
	case modeFilterPills:
		return m.handleFilterPillsKey(msg)
	case modeExportMenu:
		return m.handleExportMenuKey(msg)
	case modeExporting:
		return m, nil // no keys while exporting
	default:
		return m.handleNormalKey(msg)
	}
}

func (m RowBrowserModel) handleFilterInputKey(msg tea.KeyMsg) (RowBrowserModel, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		input := strings.TrimSpace(m.filterInput.Value())
		f, err := parseFilterInput(input)
		if err != nil {
			// Leave filter mode open so user can correct it; clear bad input
			m.filterInput.SetValue("")
			return m, nil
		}
		m.filters = append(m.filters, f)
		m.filterInput.SetValue("")
		m.filterInput.Blur()
		m.mode = modeNormal
		m.statusMsg = ""
		if m.executor != nil {
			m.loading = true
			return m, tea.Batch(m.spinner.Tick, m.loadPageCmd(1))
		}
		return m, nil

	case tea.KeyEsc:
		m.filterInput.SetValue("")
		m.filterInput.Blur()
		m.mode = modeNormal
		return m, nil

	default:
		var cmd tea.Cmd
		m.filterInput, cmd = m.filterInput.Update(msg)
		return m, cmd
	}
}

func (m RowBrowserModel) handleFilterPillsKey(msg tea.KeyMsg) (RowBrowserModel, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Right):
		if m.filterPillIdx < len(m.filters)-1 {
			m.filterPillIdx++
		}
	case key.Matches(msg, m.keys.Left):
		if m.filterPillIdx > 0 {
			m.filterPillIdx--
		}
	case key.Matches(msg, m.keys.RemoveFilter):
		if len(m.filters) > 0 && m.filterPillIdx < len(m.filters) {
			m.filters = append(m.filters[:m.filterPillIdx], m.filters[m.filterPillIdx+1:]...)
			if m.filterPillIdx >= len(m.filters) && m.filterPillIdx > 0 {
				m.filterPillIdx--
			}
			if len(m.filters) == 0 {
				m.mode = modeNormal
			}
			m.statusMsg = ""
			if m.executor != nil {
				m.loading = true
				return m, tea.Batch(m.spinner.Tick, m.loadPageCmd(1))
			}
		}
	case key.Matches(msg, m.keys.Back):
		m.mode = modeNormal
	}
	return m, nil
}

func (m RowBrowserModel) handleExportMenuKey(msg tea.KeyMsg) (RowBrowserModel, tea.Cmd) {
	switch msg.String() {
	case "c":
		return m.startExport(export.FormatCSV)
	case "x":
		return m.startExport(export.FormatExcel)
	case "esc":
		m.mode = modeNormal
	}
	return m, nil
}

func (m RowBrowserModel) handleNormalKey(msg tea.KeyMsg) (RowBrowserModel, tea.Cmd) {
	if m.loading || m.err != nil || m.result == nil {
		return m, nil
	}

	switch {
	case key.Matches(msg, m.keys.Filter):
		cmd := m.filterInput.Focus()
		m.mode = modeFilterInput
		return m, cmd

	case key.Matches(msg, m.keys.FilterPills):
		if len(m.filters) > 0 {
			m.filterPillIdx = 0
			m.mode = modeFilterPills
		}

	case key.Matches(msg, m.keys.Sort):
		m.cycleSort()
		m.statusMsg = ""
		if m.executor != nil {
			m.loading = true
			return m, tea.Batch(m.spinner.Tick, m.loadPageCmd(1))
		}

	case key.Matches(msg, m.keys.Export):
		m.mode = modeExportMenu

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

// cycleSort advances the sort state for the column at colOffset:
// no sort → ASC → DESC → no sort.
func (m *RowBrowserModel) cycleSort() {
	if m.result == nil || m.colOffset >= len(m.result.Columns) {
		return
	}
	col := m.result.Columns[m.colOffset].Name
	if m.sort == nil || m.sort.Column != col {
		m.sort = &dataset.Sort{Column: col, Desc: false}
	} else if !m.sort.Desc {
		m.sort = &dataset.Sort{Column: col, Desc: true}
	} else {
		m.sort = nil
	}
}

func (m RowBrowserModel) startExport(format export.Format) (RowBrowserModel, tea.Cmd) {
	if m.exporter == nil {
		m.mode = modeNormal
		m.statusMsg = "No exporter available"
		return m, nil
	}

	ext := "csv"
	if format == export.FormatExcel {
		ext = "xlsx"
	}
	timestamp := time.Now().Format("20060102_150405")
	path := fmt.Sprintf("%s_%s.%s", m.ds.Name, timestamp, ext)

	ch := make(chan exportEvent, 20)
	m.exportCh = ch
	m.exportProgress = 0
	m.mode = modeExporting
	m.statusMsg = ""

	opts := dataset.QueryOptions{Filters: m.filters, Sort: m.sort}
	ex := m.exporter

	go func() {
		total := 0
		err := ex.Export(context.Background(), m.ds, opts, format, path, func(n int) {
			total = n
			select {
			case ch <- exportEvent{n: n}:
			default:
			}
		})
		if err != nil {
			ch <- exportEvent{done: true, err: err}
		} else {
			ch <- exportEvent{done: true, path: path, n: total}
		}
	}()

	return m, func() tea.Msg { return <-ch }
}

// --- Accessors ---

func (m RowBrowserModel) Page() int {
	if m.result == nil {
		return 0
	}
	return m.result.Page
}

func (m RowBrowserModel) TotalPages() int {
	if m.result == nil {
		return 0
	}
	return m.result.TotalPages
}

func (m RowBrowserModel) TotalRows() int64 {
	if m.result == nil {
		return 0
	}
	return m.result.TotalRows
}

func (m RowBrowserModel) ColOffset() int            { return m.colOffset }
func (m RowBrowserModel) IsLoading() bool           { return m.loading }
func (m RowBrowserModel) Err() error                { return m.err }
func (m RowBrowserModel) Filters() []dataset.Filter { return m.filters }
func (m RowBrowserModel) ActiveSort() *dataset.Sort { return m.sort }
func (m RowBrowserModel) FilterInputActive() bool   { return m.mode == modeFilterInput }
func (m RowBrowserModel) FilterPillsActive() bool   { return m.mode == modeFilterPills }
func (m RowBrowserModel) ExportMenuActive() bool    { return m.mode == modeExportMenu }

// NeedsBackKey returns true when the row browser is consuming the Back key
// internally, so the app should not intercept it.
func (m RowBrowserModel) NeedsBackKey() bool { return m.mode != modeNormal }

func (m RowBrowserModel) StatusLine() string {
	if m.mode == modeExporting {
		return fmt.Sprintf("Exporting... %s rows written", formatCount(int64(m.exportProgress)))
	}
	if m.statusMsg != "" {
		return m.statusMsg
	}
	if m.result == nil {
		return m.ds.Name
	}

	base := fmt.Sprintf("%s  page %d/%d  %s rows",
		m.ds.Name,
		m.result.Page,
		m.result.TotalPages,
		formatCount(m.result.TotalRows),
	)

	if len(m.filters) > 0 {
		base += fmt.Sprintf("  [%d filter(s)]", len(m.filters))
	}
	if m.sort != nil {
		dir := "ASC"
		if m.sort.Desc {
			dir = "DESC"
		}
		base += fmt.Sprintf("  sort: %s %s", m.sort.Column, dir)
	}
	return base
}

// --- View ---

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
		return style.Content.Width(m.width).Height(m.height).Render(
			style.Error.Render("Error: " + m.err.Error()),
		)
	}

	if m.result == nil {
		return style.Content.Width(m.width).Height(m.height).Render("")
	}

	var sections []string

	// Filter pills row
	if len(m.filters) > 0 {
		sections = append(sections, m.renderFilterPills())
	}

	// Table (with available height)
	tableHeight := m.height - len(sections)
	if m.mode == modeFilterInput || m.mode == modeExportMenu || m.mode == modeExporting {
		tableHeight--
	}
	if tableHeight < 2 {
		tableHeight = 2
	}
	sections = append(sections, m.renderTable(tableHeight))

	// Bottom bar (filter input / export menu / exporting)
	switch m.mode {
	case modeFilterInput:
		bar := style.FilterBar.Width(m.width).Render(m.filterInput.View())
		sections = append(sections, bar)
	case modeExportMenu:
		bar := style.ExportBar.Width(m.width).Render(
			style.StatusKey.Render("c") + style.StatusDesc.Render(" CSV") +
				"  " +
				style.StatusKey.Render("x") + style.StatusDesc.Render(" Excel") +
				"  " +
				style.StatusKey.Render("esc") + style.StatusDesc.Render(" cancel"),
		)
		sections = append(sections, bar)
	case modeExporting:
		bar := style.ExportBar.Width(m.width).Render(
			style.Progress.Render(fmt.Sprintf("Exporting... %s rows written", formatCount(int64(m.exportProgress)))),
		)
		sections = append(sections, bar)
	}

	return style.Content.Width(m.width).Height(m.height).Render(
		strings.Join(sections, "\n"),
	)
}

func (m RowBrowserModel) renderFilterPills() string {
	var parts []string
	for i, f := range m.filters {
		label := fmt.Sprintf("%s%s%v", f.Column, f.Operator, f.Value)
		if m.mode == modeFilterPills && i == m.filterPillIdx {
			parts = append(parts, style.FilterPillSelected.Render(label+" ✕"))
		} else {
			parts = append(parts, style.FilterPill.Render(label))
		}
	}
	return strings.Join(parts, " ")
}

func (m RowBrowserModel) renderTable(height int) string {
	cols := m.result.Columns
	rows := m.result.Rows

	if len(cols) == 0 {
		return "No columns."
	}

	visible := visibleColumns(cols, m.colWidths, m.colOffset, m.width)
	if len(visible) == 0 {
		visible = []int{m.colOffset}
	}

	header := buildHeader(cols, m.colWidths, visible, m.sort)
	sep := buildSeparator(m.colWidths, visible)

	maxRows := height - 2
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

// --- Rendering helpers ---

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

func buildHeader(cols []db.Column, widths []int, visible []int, sort *dataset.Sort) string {
	parts := make([]string, len(visible))
	for j, i := range visible {
		name := cols[i].Name
		w := widths[i]
		var cell string
		if sort != nil && sort.Column == name {
			indicator := "↑"
			if sort.Desc {
				indicator = "↓"
			}
			// Reserve 2 display cols for " ↑"/" ↓"; truncate name into remaining space.
			usable := w - 2
			if usable < 0 {
				usable = 0
			}
			nameCell := runewidth.FillRight(runewidth.Truncate(name, usable, "…"), usable)
			cell = nameCell + " " + indicator
		} else {
			cell = runewidth.FillRight(runewidth.Truncate(name, w, "…"), w)
		}
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
			cell := runewidth.FillRight(runewidth.Truncate("null", widths[i], "…"), widths[i])
			parts[j] = style.NullValue.Render(cell)
		} else {
			cell := runewidth.FillRight(runewidth.Truncate(formatCellValue(v), widths[i], "…"), widths[i])
			parts[j] = cell
		}
	}
	return strings.Join(parts, "  ")
}

func computeColWidths(cols []db.Column, rows []map[string]any) []int {
	const maxColWidth = 40
	widths := make([]int, len(cols))
	for i, col := range cols {
		widths[i] = runewidth.StringWidth(col.Name)
	}
	for _, row := range rows {
		for i, col := range cols {
			if w := runewidth.StringWidth(formatCellValue(row[col.Name])); w > widths[i] {
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
