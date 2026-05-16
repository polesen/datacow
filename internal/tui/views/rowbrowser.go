package views

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-runewidth"

	"github.com/polesen/datacow/internal/core/dataset"
	"github.com/polesen/datacow/internal/core/db"
	"github.com/polesen/datacow/internal/core/export"
	"github.com/polesen/datacow/internal/tui/keys"
	"github.com/polesen/datacow/internal/tui/style"
)

type RowsLoadedMsg *dataset.QueryResult

// FKsLoadedMsg carries FK metadata for the active table.
type FKsLoadedMsg []db.ForeignKey

// rowsLoadedInternal is the message returned by loadPageCmd, carrying a sequence
// number so stale results from cancelled drill-downs are silently discarded.
type rowsLoadedInternal struct {
	result *dataset.QueryResult
	seq    int
}

// fksLoadedInternal mirrors FKsLoadedMsg but carries a sequence number.
type fksLoadedInternal struct {
	fks []db.ForeignKey
	seq int
}

// pkColsLoadedInternal carries the primary-key column names for the active table.
type pkColsLoadedInternal struct {
	cols []string
	seq  int
}

type uiMode int

const (
	modeNormal      uiMode = iota
	modeFilterModal        // query filter modal is open
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

// savedLevel holds the complete state for one ancestor level of the drill-down stack.
type savedLevel struct {
	ds         dataset.Dataset
	result     *dataset.QueryResult
	fks        []db.ForeignKey
	pkCols     []string
	colWidths  []int
	colOffset  int
	colCursor  int
	rowOffset  int
	rowCursor  int
	filters    []dataset.Filter
	sort       *dataset.Sort
	breadcrumb string // e.g. "→ customer_id = 1001 → customers"
}

// compactParentRows is the maximum data rows shown per parent level in the drill view.
const compactParentRows = 4

type RowBrowserModel struct {
	ds         dataset.Dataset
	result     *dataset.QueryResult
	colWidths  []int
	colOffset  int
	colCursor  int
	rowOffset  int
	rowCursor  int
	fks        []db.ForeignKey
	pkCols     []string
	drillStack []savedLevel
	drillSeq   int // incremented on each drill/pop; stale async results are discarded
	spinner    spinner.Model
	loading    bool
	err        error
	keys       keys.Map
	width      int
	height     int
	executor   *dataset.Executor

	filters      []dataset.Filter
	sort         *dataset.Sort
	mode         uiMode
	filterModal  FilterModalModel
	localSearch  LocalSearchState
	exportProgress int
	statusMsg    string
	exporter     *export.Exporter
	exportCh     chan exportEvent
	exportCancel context.CancelFunc // non-nil only while modeExporting
}

// NewRowBrowserModel creates a RowBrowserModel in the initial loading state.
// executor and exporter may be nil for testing.
func NewRowBrowserModel(k keys.Map, executor *dataset.Executor, exporter *export.Exporter, ds dataset.Dataset) RowBrowserModel {
	return RowBrowserModel{
		ds:          ds,
		spinner:     newSpinner(),
		loading:     true,
		keys:        k,
		executor:    executor,
		exporter:    exporter,
		localSearch: newLocalSearch(),
		drillStack:  make([]savedLevel, 0, 4),
	}
}

func (m RowBrowserModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.loadPageCmd(1), m.loadFKsCmd(), m.loadPKColsCmd())
}

func (m RowBrowserModel) loadPageCmd(page int) tea.Cmd {
	if m.executor == nil {
		return nil
	}
	filters := m.filters
	sort := m.sort
	ds := m.ds
	seq := m.drillSeq
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		result, err := m.executor.Query(ctx, ds, dataset.QueryOptions{
			Page:     page,
			PageSize: 50,
			Filters:  filters,
			Sort:     sort,
		})
		if err != nil {
			return ErrMsg{err}
		}
		return rowsLoadedInternal{result: result, seq: seq}
	}
}

func (m RowBrowserModel) loadFKsCmd() tea.Cmd {
	if m.executor == nil || m.ds.Table == "" {
		return nil
	}
	table := m.ds.Table
	seq := m.drillSeq
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		fks, err := m.executor.ForeignKeys(ctx, table)
		if err != nil {
			return fksLoadedInternal{fks: nil, seq: seq}
		}
		return fksLoadedInternal{fks: fks, seq: seq}
	}
}

func (m RowBrowserModel) loadPKColsCmd() tea.Cmd {
	if m.executor == nil || m.ds.Table == "" {
		return nil
	}
	table := m.ds.Table
	seq := m.drillSeq
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		cols, err := m.executor.PrimaryKeyColumns(ctx, table)
		if err != nil {
			return pkColsLoadedInternal{cols: nil, seq: seq}
		}
		return pkColsLoadedInternal{cols: cols, seq: seq}
	}
}

func (m RowBrowserModel) applyLoadedResult(r *dataset.QueryResult) RowBrowserModel {
	m.result = r
	m.loading = false
	m.rowCursor = 0
	m.rowOffset = 0
	m.colWidths = computeColWidths(r.Columns, r.Rows)
	// Recompute local search against new page
	if m.localSearch.IsActive() {
		m.localSearch = m.localSearch.recompute(m.localSearch.Query(), r.Columns, r.Rows)
	}
	return m
}

func (m RowBrowserModel) Update(msg tea.Msg) (RowBrowserModel, tea.Cmd) {
	var cmd tea.Cmd

	// Route all messages to modal when it is open.
	if m.mode == modeFilterModal {
		if ws, ok := msg.(tea.WindowSizeMsg); ok {
			m.width = ws.Width
			m.height = ws.Height
			m.filterModal.SetWidth(ws.Width)
			return m, nil
		}
		m.filterModal, cmd = m.filterModal.Update(msg)
		if m.filterModal.IsApplied() {
			m.filters = m.filterModal.Filters()
			m.localSearch = m.localSearch.cleared()
			m.mode = modeNormal
			m.statusMsg = ""
			if m.executor != nil {
				m.loading = true
				return m, tea.Batch(m.spinner.Tick, m.loadPageCmd(1), cmd)
			}
			return m, cmd
		}
		if m.filterModal.IsCancelled() {
			m.mode = modeNormal
			return m, nil
		}
		// Also route spinner ticks to keep the spinner going
		if _, ok := msg.(spinner.TickMsg); ok {
			if m.loading {
				m.spinner, _ = m.spinner.Update(msg)
			}
		}
		return m, cmd
	}

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
		return m.applyLoadedResult((*dataset.QueryResult)(msg)), nil

	case rowsLoadedInternal:
		if msg.seq != m.drillSeq {
			return m, nil
		}
		return m.applyLoadedResult(msg.result), nil

	case FKsLoadedMsg:
		m.fks = []db.ForeignKey(msg)
		return m, nil

	case fksLoadedInternal:
		if msg.seq != m.drillSeq {
			return m, nil
		}
		m.fks = msg.fks
		return m, nil

	case pkColsLoadedInternal:
		if msg.seq != m.drillSeq {
			return m, nil
		}
		m.pkCols = msg.cols
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

	// Forward non-key messages to local search input when active
	if m.localSearch.InputActive() {
		var lsCmd tea.Cmd
		m.localSearch, lsCmd = m.localSearch.Update(msg)
		return m, lsCmd
	}

	return m, cmd
}

func (m RowBrowserModel) handleKey(msg tea.KeyMsg) (RowBrowserModel, tea.Cmd) {
	if m.localSearch.InputActive() {
		return m.handleLocalSearchKey(msg)
	}
	switch m.mode {
	case modeExportMenu:
		return m.handleExportMenuKey(msg)
	case modeExporting:
		if msg.Type == tea.KeyEsc {
			if m.exportCancel != nil {
				m.exportCancel()
				m.exportCancel = nil
			}
			m.mode = modeNormal
			m.statusMsg = "Export cancelled"
		}
		return m, nil
	default:
		return m.handleNormalKey(msg)
	}
}

func (m RowBrowserModel) handleLocalSearchKey(msg tea.KeyMsg) (RowBrowserModel, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		// Close input but keep highlights
		m.localSearch = m.localSearch.withInputClosed()
		return m, nil

	case tea.KeyEsc:
		m.localSearch = m.localSearch.cleared()
		return m, nil

	default:
		var cmd tea.Cmd
		m.localSearch, cmd = m.localSearch.Update(msg)
		// Recompute matches after input changes
		if m.result != nil {
			q := m.localSearch.textInput.Value()
			m.localSearch = m.localSearch.recompute(q, m.result.Columns, m.result.Rows)
		}
		return m, cmd
	}
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
	// Esc clears local search if active (before checking drill stack)
	if key.Matches(msg, m.keys.Back) && m.localSearch.IsActive() {
		m.localSearch = m.localSearch.cleared()
		return m, nil
	}

	// Back key pops the drill stack even while loading, so users can cancel a drill.
	if key.Matches(msg, m.keys.Back) && len(m.drillStack) > 0 {
		return m.popDrillStack()
	}

	if m.loading || m.err != nil || m.result == nil {
		return m, nil
	}

	switch {
	case key.Matches(msg, m.keys.QueryFilter):
		return m.openFilterModal()

	case key.Matches(msg, m.keys.LocalSearch):
		var cmd tea.Cmd
		m.localSearch, cmd = m.localSearch.withInputOpen()
		return m, cmd

	case key.Matches(msg, m.keys.QuickFilterCell):
		return m.openQuickFilter()

	case m.localSearch.IsActive() && key.Matches(msg, m.keys.NextMatch):
		m.localSearch = m.localSearch.withNextMatch()
		return m, nil

	case m.localSearch.IsActive() && key.Matches(msg, m.keys.PrevMatch):
		m.localSearch = m.localSearch.withPrevMatch()
		return m, nil

	case key.Matches(msg, m.keys.Sort):
		m = m.cycleSort()
		m.statusMsg = ""
		if m.executor != nil {
			m.loading = true
			return m, tea.Batch(m.spinner.Tick, m.loadPageCmd(1))
		}

	case key.Matches(msg, m.keys.Export):
		m.mode = modeExportMenu

	case key.Matches(msg, m.keys.NextPage):
		if m.result.Page < m.result.TotalPages {
			m.localSearch = m.localSearch.cleared()
			m.loading = true
			return m, tea.Batch(m.spinner.Tick, m.loadPageCmd(m.result.Page+1))
		}
	case key.Matches(msg, m.keys.PrevPage):
		if m.result.Page > 1 {
			m.localSearch = m.localSearch.cleared()
			m.loading = true
			return m, tea.Batch(m.spinner.Tick, m.loadPageCmd(m.result.Page-1))
		}

	case key.Matches(msg, m.keys.Down):
		if m.rowCursor < len(m.result.Rows)-1 {
			m.rowCursor++
			if visible := m.visibleRowCount(); visible > 0 && m.rowCursor >= m.rowOffset+visible {
				m.rowOffset = m.rowCursor - visible + 1
			}
		}
	case key.Matches(msg, m.keys.Up):
		if m.rowCursor > 0 {
			m.rowCursor--
			if m.rowCursor < m.rowOffset {
				m.rowOffset = m.rowCursor
			}
		}

	case key.Matches(msg, m.keys.Right):
		if m.colCursor < len(m.result.Columns)-1 {
			m.colCursor++
			visible := visibleColumns(m.result.Columns, m.colWidths, m.colOffset, m.width)
			if len(visible) > 0 && m.colCursor > visible[len(visible)-1] {
				m.colOffset++
			}
		}
	case key.Matches(msg, m.keys.Left):
		if m.colCursor > 0 {
			m.colCursor--
			if m.colCursor < m.colOffset {
				m.colOffset = m.colCursor
			}
		}

	case key.Matches(msg, m.keys.Enter):
		return m.handleDrillDown()

	case key.Matches(msg, m.keys.ViewCell):
		return m.openCellViewer()
	}
	return m, nil
}

// openFilterModal opens the query filter modal with the current filter state.
func (m RowBrowserModel) openFilterModal() (RowBrowserModel, tea.Cmd) {
	m.filterModal = NewFilterModal(m.ds, m.result.Columns, m.filters)
	m.filterModal.SetWidth(m.width)
	m.mode = modeFilterModal
	return m, nil
}

// openQuickFilter opens the filter modal pre-filled with the selected cell value.
func (m RowBrowserModel) openQuickFilter() (RowBrowserModel, tea.Cmd) {
	if m.result == nil || len(m.result.Rows) == 0 || m.rowCursor >= len(m.result.Rows) {
		return m, nil
	}
	row := m.result.Rows[m.rowCursor]
	col := m.result.Columns[m.colCursor]
	cellValue := row[col.Name]
	if cellValue == nil {
		m.statusMsg = "= cannot filter on NULL"
		return m, nil
	}
	m.filterModal = NewFilterModalQuickFilter(m.ds, m.result.Columns, m.filters, col.Name, formatCellValue(cellValue))
	m.filterModal.SetWidth(m.width)
	m.mode = modeFilterModal
	return m, nil
}

// handleDrillDown navigates from the selected FK cell into the referenced table.
func (m RowBrowserModel) handleDrillDown() (RowBrowserModel, tea.Cmd) {
	if len(m.result.Rows) == 0 || m.rowCursor >= len(m.result.Rows) {
		return m, nil
	}

	colName := m.result.Columns[m.colCursor].Name
	fk := findFK(m.fks, colName)
	if fk == nil {
		return m.openCellViewer()
	}

	row := m.result.Rows[m.rowCursor]
	cellValue := row[colName]
	if cellValue == nil {
		return m, nil
	}

	saved := savedLevel{
		ds:         m.ds,
		result:     m.result,
		fks:        m.fks,
		pkCols:     m.pkCols,
		colWidths:  m.colWidths,
		colOffset:  m.colOffset,
		colCursor:  m.colCursor,
		rowOffset:  m.rowOffset,
		rowCursor:  m.rowCursor,
		filters:    m.filters,
		sort:       m.sort,
		breadcrumb: fmt.Sprintf("→ %s = %v → %s", fk.Column, formatCellValue(cellValue), fk.ReferencedTable),
	}
	m.drillStack = append(m.drillStack, saved)

	m.ds = dataset.Dataset{Name: fk.ReferencedTable, Table: fk.ReferencedTable}
	m.result = nil
	m.fks = nil
	m.pkCols = nil
	m.colWidths = nil
	m.colOffset = 0
	m.colCursor = 0
	m.rowOffset = 0
	m.rowCursor = 0
	m.filters = []dataset.Filter{{
		Column:   fk.ReferencedColumn,
		Operator: "=",
		Value:    cellValue,
	}}
	m.sort = nil
	m.loading = true
	m.err = nil
	m.statusMsg = ""
	m.mode = modeNormal
	m.localSearch = m.localSearch.cleared()
	m.drillSeq++

	return m, tea.Batch(m.spinner.Tick, m.loadPageCmd(1), m.loadFKsCmd(), m.loadPKColsCmd())
}

// openCellViewer builds an OpenCellViewerMsg for the currently selected cell.
func (m RowBrowserModel) openCellViewer() (RowBrowserModel, tea.Cmd) {
	if m.result == nil || len(m.result.Rows) == 0 || m.rowCursor >= len(m.result.Rows) {
		return m, nil
	}
	col := m.result.Columns[m.colCursor]
	row := m.result.Rows[m.rowCursor]

	var pkValues []string
	var pkDisplayParts []string
	for _, pkCol := range m.pkCols {
		if v, ok := row[pkCol]; ok {
			s := formatCellValue(v)
			pkValues = append(pkValues, s)
			pkDisplayParts = append(pkDisplayParts, pkCol+"="+s)
		}
	}

	var raw []byte
	if v := row[col.Name]; v != nil {
		switch val := v.(type) {
		case []byte:
			raw = val
		default:
			raw = []byte(formatCellValue(v))
		}
	}

	msg := OpenCellViewerMsg{
		TableName:  m.ds.Name,
		PKValues:   pkValues,
		PKDisplay:  strings.Join(pkDisplayParts, ", "),
		ColumnName: col.Name,
		ColumnType: col.Type,
		Raw:        raw,
	}
	return m, func() tea.Msg { return msg }
}

// popDrillStack collapses the most recent drill level and restores the parent state.
func (m RowBrowserModel) popDrillStack() (RowBrowserModel, tea.Cmd) {
	if len(m.drillStack) == 0 {
		return m, nil
	}

	last := m.drillStack[len(m.drillStack)-1]
	m.drillStack = m.drillStack[:len(m.drillStack)-1]

	m.ds = last.ds
	m.result = last.result
	m.fks = last.fks
	m.pkCols = last.pkCols
	m.colWidths = last.colWidths
	m.colOffset = last.colOffset
	m.colCursor = last.colCursor
	m.rowOffset = last.rowOffset
	m.rowCursor = last.rowCursor
	m.filters = last.filters
	m.sort = last.sort
	m.loading = false
	m.err = nil
	m.statusMsg = ""
	m.mode = modeNormal
	m.localSearch = m.localSearch.cleared()
	m.drillSeq++ // invalidate any in-flight child loads

	return m, nil
}

// cycleSort advances the sort state for the column at colCursor:
// no sort → ASC → DESC → no sort.
func (m RowBrowserModel) cycleSort() RowBrowserModel {
	if m.result == nil || m.colCursor >= len(m.result.Columns) {
		return m
	}
	col := m.result.Columns[m.colCursor].Name
	if m.sort == nil || m.sort.Column != col {
		m.sort = &dataset.Sort{Column: col, Desc: false}
	} else if !m.sort.Desc {
		m.sort = &dataset.Sort{Column: col, Desc: true}
	} else {
		m.sort = nil
	}
	return m
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

	ctx, cancel := context.WithCancel(context.Background())
	m.exportCancel = cancel

	ch := make(chan exportEvent, 20)
	m.exportCh = ch
	m.exportProgress = 0
	m.mode = modeExporting
	m.statusMsg = ""

	opts := dataset.QueryOptions{Filters: m.filters, Sort: m.sort}
	ex := m.exporter

	go func() {
		defer cancel()
		total := 0
		err := ex.Export(ctx, m.ds, opts, format, path, func(n int) {
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

func (m RowBrowserModel) ColOffset() int               { return m.colOffset }
func (m RowBrowserModel) ColCursor() int               { return m.colCursor }
func (m RowBrowserModel) DatasetName() string          { return m.ds.Name }
func (m RowBrowserModel) RowCursor() int               { return m.rowCursor }
func (m RowBrowserModel) ForeignKeys() []db.ForeignKey { return m.fks }
func (m RowBrowserModel) DrillDepth() int              { return len(m.drillStack) }
func (m RowBrowserModel) IsLoading() bool              { return m.loading }
func (m RowBrowserModel) Err() error                   { return m.err }
func (m RowBrowserModel) Filters() []dataset.Filter    { return m.filters }
func (m RowBrowserModel) ActiveSort() *dataset.Sort    { return m.sort }
func (m RowBrowserModel) IsFilterModalOpen() bool      { return m.mode == modeFilterModal }
func (m RowBrowserModel) IsLocalSearchInputActive() bool {
	return m.localSearch.InputActive()
}
func (m RowBrowserModel) ExportMenuActive() bool { return m.mode == modeExportMenu }

// BlocksGlobalKeys returns true when the row browser is consuming keys that
// should not be intercepted by the App (modal open, local search input active).
func (m RowBrowserModel) BlocksGlobalKeys() bool {
	return m.mode == modeFilterModal || m.localSearch.InputActive()
}

// CancelExport cancels an in-progress export if one is running.
func (m RowBrowserModel) CancelExport() {
	if m.exportCancel != nil {
		m.exportCancel()
	}
}

// visibleRowCount returns the number of data rows that fit in the table area given
// the current height, drill-stack, filter pills, and mode bar.
func (m RowBrowserModel) visibleRowCount() int {
	if m.height == 0 {
		return 0
	}
	parentLines := 0
	for _, p := range m.drillStack {
		if p.result != nil {
			parentLines += parentLineCount(p) + 1
		}
	}
	filterPillLines := 0
	if len(m.filters) > 0 {
		filterPillLines = 1
	}
	bottomBarLines := 0
	if m.localSearch.InputActive() || m.mode == modeExportMenu || m.mode == modeExporting {
		bottomBarLines = 1
	}
	tableHeight := m.height - parentLines - filterPillLines - bottomBarLines
	return max(0, tableHeight-2) // subtract header + separator rows
}

// IsFKColumn reports whether the currently selected column is a foreign key.
func (m RowBrowserModel) IsFKColumn() bool {
	if m.result == nil || m.colCursor >= len(m.result.Columns) {
		return false
	}
	return findFK(m.fks, m.result.Columns[m.colCursor].Name) != nil
}

// NeedsBackKey returns true when the row browser is consuming the Back key
// internally, so the app should not intercept it.
func (m RowBrowserModel) NeedsBackKey() bool {
	return m.mode == modeFilterModal || m.localSearch.IsActive() || len(m.drillStack) > 0
}

// NeedsTabKey returns true when the row browser is consuming Tab internally
// (filter modal field cycling), so App should not intercept Tab as a focus switch.
func (m RowBrowserModel) NeedsTabKey() bool {
	return m.mode == modeFilterModal
}

func (m RowBrowserModel) exportProgressText() string {
	return fmt.Sprintf("Exporting... %s rows written", formatCount(int64(m.exportProgress)))
}

func (m RowBrowserModel) StatusLine() string {
	if m.mode == modeExporting {
		return m.exportProgressText()
	}
	if m.localSearch.IsActive() {
		return m.localSearch.StatusText()
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

	// Filter modal takes the full view area.
	if m.mode == modeFilterModal {
		return style.Content.Width(m.width).Height(m.height).Render(m.filterModal.View())
	}

	// Full-screen spinner for the initial root-level load (no parent levels yet).
	if m.loading && len(m.drillStack) == 0 {
		return style.Content.Width(m.width).Height(m.height).Render(
			m.spinner.View() + " Loading...",
		)
	}

	// Full-screen error for root-level failure.
	if m.err != nil && len(m.drillStack) == 0 {
		return style.Content.Width(m.width).Height(m.height).Render(
			style.Error.Render("Error: " + m.err.Error()),
		)
	}

	var sections []string

	// Render all ancestor levels compactly, each followed by its drill separator.
	parentLines := 0
	for _, parent := range m.drillStack {
		if parent.result != nil {
			sections = append(sections, m.renderSavedLevel(parent))
			sections = append(sections, renderDrillSeparator(parent.breadcrumb, m.width))
			parentLines += parentLineCount(parent) + 1
		}
	}

	// Render the active (bottom) level.
	if m.loading {
		sections = append(sections, m.spinner.View()+" Loading...")
		return style.Content.Width(m.width).Height(m.height).Render(strings.Join(sections, "\n"))
	}

	if m.err != nil {
		sections = append(sections, style.Error.Render("Error: "+m.err.Error()))
		return style.Content.Width(m.width).Height(m.height).Render(strings.Join(sections, "\n"))
	}

	if m.result == nil {
		return style.Content.Width(m.width).Height(m.height).Render(strings.Join(sections, "\n"))
	}

	filterPillLines := 0
	if len(m.filters) > 0 {
		sections = append(sections, m.renderFilterPills())
		filterPillLines = 1
	}
	bottomBarLines := 0
	if m.localSearch.InputActive() || m.mode == modeExportMenu || m.mode == modeExporting {
		bottomBarLines = 1
	}

	tableHeight := m.height - parentLines - filterPillLines - bottomBarLines
	if tableHeight < 2 {
		tableHeight = 2
	}
	sections = append(sections, m.renderTable(tableHeight))

	// Bottom bar (local search / export menu / exporting)
	switch {
	case m.localSearch.InputActive():
		bar := style.FilterBar.Width(m.width).Render(m.localSearch.View(m.width))
		sections = append(sections, bar)
	case m.mode == modeExportMenu:
		bar := style.ExportBar.Width(m.width).Render(
			style.StatusKey.Render("c") + style.StatusDesc.Render(" CSV") +
				"  " +
				style.StatusKey.Render("x") + style.StatusDesc.Render(" Excel") +
				"  " +
				style.StatusKey.Render("esc") + style.StatusDesc.Render(" cancel"),
		)
		sections = append(sections, bar)
	case m.mode == modeExporting:
		bar := style.ExportBar.Width(m.width).Render(
			style.Progress.Render(m.exportProgressText()),
		)
		sections = append(sections, bar)
	}

	return style.Content.Width(m.width).Height(m.height).Render(
		strings.Join(sections, "\n"),
	)
}

func (m RowBrowserModel) renderFilterPills() string {
	var parts []string
	for _, f := range m.filters {
		parts = append(parts, style.FilterPill.Render(formatFilterLabel(f)))
	}
	return strings.Join(parts, " ")
}

// renderSavedLevel renders a compact summary of an ancestor level.
func (m RowBrowserModel) renderSavedLevel(level savedLevel) string {
	if level.result == nil || len(level.result.Columns) == 0 {
		return ""
	}

	sectionTitle := style.DrillSep.Render(
		fmt.Sprintf("─ %s (%s rows) ", level.ds.Name, formatCount(level.result.TotalRows)),
	)

	cols := level.result.Columns
	rows := level.result.Rows
	visible := visibleColumns(cols, level.colWidths, level.colOffset, m.width)
	if len(visible) == 0 {
		visible = []int{level.colOffset}
	}

	fkCols := fkColSet(level.fks)
	header := buildHeader(cols, level.colWidths, visible, level.sort, level.colCursor, fkCols)
	sep := buildSeparator(level.colWidths, visible)

	lines := []string{sectionTitle, header, sep}
	for i, row := range rows {
		if i >= compactParentRows {
			break
		}
		lines = append(lines, buildRow(row, cols, level.colWidths, visible, i, level.rowCursor, level.colCursor, fkCols, nil, ""))
	}
	return strings.Join(lines, "\n")
}

// renderDrillSeparator renders the breadcrumb line between drill levels.
func renderDrillSeparator(breadcrumb string, width int) string {
	label := " " + breadcrumb + " "
	labelWidth := runewidth.StringWidth(label)
	fillLen := width - labelWidth - 1
	if fillLen < 0 {
		fillLen = 0
	}
	return style.DrillSep.Render("├" + label + strings.Repeat("─", fillLen))
}

// parentLineCount returns the number of lines a saved level's compact view occupies.
func parentLineCount(level savedLevel) int {
	if level.result == nil {
		return 0
	}
	rows := len(level.result.Rows)
	if rows > compactParentRows {
		rows = compactParentRows
	}
	return 3 + rows // section title + column header + separator + data rows
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

	fkCols := fkColSet(m.fks)
	header := buildHeader(cols, m.colWidths, visible, m.sort, m.colCursor, fkCols)
	sep := buildSeparator(m.colWidths, visible)

	maxRows := max(0, height-2)

	lines := make([]string, 0, maxRows+2)
	lines = append(lines, header, sep)

	if m.localSearch.IsActive() && m.localSearch.Query() != "" {
		matchRows := m.localSearch.MatchRows()
		if len(matchRows) == 0 {
			lines = append(lines, style.Muted.Render("  no matches for "+strconv.Quote(m.localSearch.Query())))
		} else {
			// Center the viewport on the current match.
			cur := m.localSearch.MatchCursor()
			start := max(0, cur-maxRows/2)
			if start+maxRows > len(matchRows) {
				start = max(0, len(matchRows)-maxRows)
			}
			for fi := start; fi < len(matchRows) && fi-start < maxRows; fi++ {
				rowIdx := matchRows[fi]
				lines = append(lines, buildRow(rows[rowIdx], cols, m.colWidths, visible, rowIdx, rowIdx, m.colCursor, fkCols, &m.localSearch, m.localSearch.Query()))
			}
		}
	} else {
		startRow := m.rowOffset
		if startRow > len(rows) {
			startRow = 0
		}
		for i, row := range rows[startRow:] {
			if i >= maxRows {
				break
			}
			rowIdx := startRow + i
			lines = append(lines, buildRow(row, cols, m.colWidths, visible, rowIdx, m.rowCursor, m.colCursor, fkCols, nil, ""))
		}
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

func fkColSet(fks []db.ForeignKey) map[string]bool {
	if len(fks) == 0 {
		return nil
	}
	m := make(map[string]bool, len(fks))
	for _, fk := range fks {
		m[fk.Column] = true
	}
	return m
}

func buildHeader(cols []db.Column, widths []int, visible []int, sort *dataset.Sort, cursor int, fkCols map[string]bool) string {
	parts := make([]string, len(visible))
	for j, i := range visible {
		name := cols[i].Name
		w := widths[i]
		isFKCol := fkCols[name]

		var cell string
		if sort != nil && sort.Column == name {
			indicator := "↑"
			if sort.Desc {
				indicator = "↓"
			}
			usable := w - 2
			if usable < 0 {
				usable = 0
			}
			nameCell := runewidth.FillRight(runewidth.Truncate(name, usable, "…"), usable)
			cell = nameCell + " " + indicator
		} else {
			cell = runewidth.FillRight(runewidth.Truncate(name, w, "…"), w)
		}

		switch {
		case i == cursor && isFKCol:
			parts[j] = style.FKColHeaderActive.Render(cell)
		case i == cursor:
			parts[j] = style.ColHeaderActive.Render(cell)
		case isFKCol:
			parts[j] = style.FKColHeader.Render(cell)
		default:
			parts[j] = style.ColHeader.Render(cell)
		}
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

// buildRow renders a single data row, applying local search dimming/highlighting.
func buildRow(row map[string]any, cols []db.Column, widths []int, visible []int, rowIdx, rowCursor, colCursor int, fkCols map[string]bool, ls *LocalSearchState, searchQuery string) string {
	isSelectedRow := rowIdx == rowCursor

	isSearchActive := ls != nil && ls.IsActive() && searchQuery != ""
	isCurrentMatch := isSearchActive && ls.CurrentMatchRow() == rowIdx

	parts := make([]string, len(visible))
	for j, i := range visible {
		colName := cols[i].Name
		v := row[colName]
		isCursorCell := isSelectedRow && i == colCursor

		var raw string
		if v == nil {
			raw = "null"
		} else {
			raw = formatCellValue(v)
		}

		switch {
		case isCursorCell && fkCols[colName]:
			display := "[" + raw + "]"
			cell := runewidth.FillRight(runewidth.Truncate(display, widths[i], "…"), widths[i])
			parts[j] = style.FKCell.Render(cell)
		case isCursorCell:
			cell := runewidth.FillRight(runewidth.Truncate(raw, widths[i], "…"), widths[i])
			parts[j] = style.CursorCell.Render(cell)
		case isCurrentMatch && !isSelectedRow:
			cell := runewidth.FillRight(runewidth.Truncate(raw, widths[i], "…"), widths[i])
			parts[j] = style.RowHighlight.Render(cell)
		case isSelectedRow:
			cell := runewidth.FillRight(runewidth.Truncate(raw, widths[i], "…"), widths[i])
			parts[j] = style.RowHighlight.Render(cell)
		case v == nil:
			cell := runewidth.FillRight(runewidth.Truncate("null", widths[i], "…"), widths[i])
			parts[j] = style.NullValue.Render(cell)
		default:
			cell := runewidth.FillRight(runewidth.Truncate(raw, widths[i], "…"), widths[i])
			parts[j] = cell
		}
	}
	return strings.Join(parts, "  ")
}

// findFK returns the ForeignKey for the given column name, or nil if none.
func findFK(fks []db.ForeignKey, colName string) *db.ForeignKey {
	for i := range fks {
		if fks[i].Column == colName {
			return &fks[i]
		}
	}
	return nil
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
