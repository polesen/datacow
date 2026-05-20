package views

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-runewidth"

	"github.com/polesen/datacow/internal/core/config"
	"github.com/polesen/datacow/internal/core/dataset"
	"github.com/polesen/datacow/internal/core/db"
	"github.com/polesen/datacow/internal/core/export"
	"github.com/polesen/datacow/internal/core/schema"
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

// countLoadedMsg carries the result of a COUNT-only query (for G/End goto-last).
type countLoadedMsg struct {
	result *dataset.QueryResult
	err    error
	seq    int
}

// CountLoadedMsgForTest creates a countLoadedMsg for use in external test packages.
func CountLoadedMsgForTest(result *dataset.QueryResult) tea.Msg {
	return countLoadedMsg{result: result, seq: 0}
}

type uiMode int

const (
	modeNormal              uiMode = iota
	modeFilterModal                // query filter modal is open
	modeExportMenu                 // choosing export format
	modeExporting                  // export in progress
	modePageSizeInput              // page-size input bar is open
	modeRefByPicker                // "referenced by" picker overlay is open
	modeColumnPicker               // column picker overlay is open
	modeSavePerspective            // save-perspective name overlay is open
)

// PerspectiveSavedMsg is emitted by the row browser when a perspective is successfully saved.
// The App handles it to refresh the dataset list and update its ConfigPath.
type PerspectiveSavedMsg struct {
	Path string // path of the config file that was written
}

// perspectiveSavedInternal carries a successful save result back to the row browser.
type perspectiveSavedInternal struct{ path string }

// perspectiveSaveErrorInternal carries a failed save result back to the row browser.
type perspectiveSaveErrorInternal struct{ err error }

// localSearchFlashExpiredMsg is sent when the 400ms local-search flash timer expires.
type localSearchFlashExpiredMsg struct{}

// exportEvent is sent by the export goroutine to report progress and completion.
type exportEvent struct {
	n    int    // rows written so far
	done bool   // true when export is finished
	path string // set on successful completion
	err  error  // set on error completion
}

// savedLevel holds the complete state for one ancestor level of the drill-down stack.
type savedLevel struct {
	ds              dataset.Dataset
	result          *dataset.QueryResult
	fks             []db.ForeignKey
	pkCols          []string
	colWidths        []int
	colOffset        int
	colCursor        int
	rowOffset        int
	rowCursor        int
	filters          []dataset.Filter
	sort             *dataset.Sort
	breadcrumb       string // e.g. "→ customer_id = 1001 → customers"
	pageSize         int    // snapshot of effective page size for this level
	knownTotalPages  *int
	knownTotalRows   *int64
	knownTotalExact  bool
}

// compactParentRows is the maximum data rows shown per parent level in the drill view.
const compactParentRows = 4

type RowBrowserModel struct {
	ds         dataset.Dataset
	result     *dataset.QueryResult
	colWidths   []int
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
	schemaCache *schema.Cache

	filters        []dataset.Filter
	sort           *dataset.Sort
	mode           uiMode
	filterModal    FilterModalModel
	refByPicker    RefByPickerModel
	localSearch         LocalSearchState
	localSearchOffset   int
	localSearchFlashing bool
	exportProgress int
	statusMsg      string
	exporter       *export.Exporter
	exportCh       chan exportEvent
	exportCancel   context.CancelFunc // non-nil only while modeExporting

	// page size
	pageSizes     *PageSizeRegistry
	pageSizeInput textinput.Model
	pageSizeError string

	// column picker
	columns      *ColumnRegistry
	columnPicker ColumnPickerModel

	// save-perspective overlay
	savePerspective  SavePerspectiveModel
	configPath       string // path to config file; empty in zero-config mode
	activeDatasource string // name of the active datasource
	presetApplied    bool   // tracks whether the perspective preset has been applied to the registry

	// discovered totals
	knownTotalPages *int
	knownTotalRows  *int64
	knownTotalExact bool // true when from COUNT(*) (no tilde); false when inferred

	// pendingRowCursor is set by applyPageSizeInput so that when the next page
	// arrives the cursor lands on the row that was selected before the resize.
	pendingRowCursor *int
}

// NewRowBrowserModel creates a RowBrowserModel in the initial loading state.
// executor, exporter, and schemaCache may be nil for testing.
// pageSizes and columns may be nil (pageSizes falls back to default 50; columns falls back to SELECT *).
func NewRowBrowserModel(k keys.Map, executor *dataset.Executor, exporter *export.Exporter, ds dataset.Dataset, pageSizes *PageSizeRegistry, schemaCache *schema.Cache) RowBrowserModel {
	return NewRowBrowserModelWithColumns(k, executor, exporter, ds, pageSizes, schemaCache, nil)
}

// NewRowBrowserModelWithColumns creates a RowBrowserModel with an explicit column registry.
func NewRowBrowserModelWithColumns(k keys.Map, executor *dataset.Executor, exporter *export.Exporter, ds dataset.Dataset, pageSizes *PageSizeRegistry, schemaCache *schema.Cache, columns *ColumnRegistry) RowBrowserModel {
	ti := textinput.New()
	ti.CharLimit = 5
	ti.Width = 8
	m := RowBrowserModel{
		ds:            ds,
		spinner:       newSpinner(),
		loading:       true,
		keys:          k,
		executor:      executor,
		exporter:      exporter,
		schemaCache:   schemaCache,
		localSearch:   newLocalSearch(),
		drillStack:    make([]savedLevel, 0, 4),
		pageSizes:     pageSizes,
		pageSizeInput: ti,
		columns:       columns,
	}
	// Pre-seed filters and sort from perspective preset (columns applied after first load).
	if ds.Kind == dataset.KindPerspective && ds.Preset != nil {
		m.filters = ds.Preset.Filters
		m.sort = ds.Preset.Sort
	}
	return m
}

// WithConfigPath returns a copy of the model with the config path and active datasource set.
// The App calls this after constructing the row browser so the save-perspective overlay
// can write to the correct file.
func (m RowBrowserModel) WithConfigPath(configPath, activeDatasource string) RowBrowserModel {
	m.configPath = configPath
	m.activeDatasource = activeDatasource
	return m
}

// UpdateConfigPath updates the config path stored in the model.
// The App calls this after a successful perspective save to keep the path in sync.
func (m RowBrowserModel) UpdateConfigPath(path string) RowBrowserModel {
	m.configPath = path
	return m
}

func (m RowBrowserModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.loadPageCmd(1), m.loadFKsCmd(), m.loadPKColsCmd())
}

// currentPageSize returns the effective page size for the active dataset.
func (m RowBrowserModel) currentPageSize() int {
	return m.pageSizes.Get(m.ds.Name)
}

func (m RowBrowserModel) loadPageCmd(page int) tea.Cmd {
	if m.executor == nil {
		return nil
	}
	filters := m.filters
	sort := m.sort
	ds := m.ds
	seq := m.drillSeq
	pageSize := m.currentPageSize()
	cols := m.columns.VisibleColumns(ds.Name)
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		result, err := m.executor.Query(ctx, ds, dataset.QueryOptions{
			Page:      page,
			PageSize:  pageSize,
			Filters:   filters,
			Sort:      sort,
			Columns:   cols,
			SkipCount: true,
		})
		if err != nil {
			return ErrMsg{err}
		}
		return rowsLoadedInternal{result: result, seq: seq}
	}
}

// loadCountCmd issues a COUNT-only query for the G/End goto-last flow.
func (m RowBrowserModel) loadCountCmd() tea.Cmd {
	if m.executor == nil {
		return nil
	}
	filters := m.filters
	sort := m.sort
	ds := m.ds
	seq := m.drillSeq
	pageSize := m.currentPageSize()
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		result, err := m.executor.Query(ctx, ds, dataset.QueryOptions{
			Page:      1,
			PageSize:  pageSize,
			Filters:   filters,
			Sort:      sort,
			OnlyCount: true,
		})
		return countLoadedMsg{result: result, err: err, seq: seq}
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
	// Seed column registry on first load for this dataset.
	m.columns.Seed(m.ds.Name, r.Columns)

	// Apply the column preset from a perspective on the very first load.
	if !m.presetApplied && m.ds.Kind == dataset.KindPerspective && m.ds.Preset != nil && len(m.ds.Preset.Columns) > 0 {
		m = m.applyPresetColumns(m.ds.Preset.Columns)
		m.presetApplied = true
	}

	m.result = r
	m.loading = false
	m.rowCursor = 0
	m.rowOffset = 0
	if m.pendingRowCursor != nil {
		target := *m.pendingRowCursor
		m.pendingRowCursor = nil
		if target >= len(r.Rows) {
			target = max(0, len(r.Rows)-1)
		}
		m.rowCursor = target
		if vis := m.visibleRowCount(); vis > 0 {
			m.rowOffset = max(0, m.rowCursor-vis/2)
		}
	}
	m.colWidths = computeColWidths(r.Columns, r.Rows)
	// Clamp column cursor/offset in case a column projection reduced the column count.
	if m.colCursor >= len(r.Columns) {
		m.colCursor = max(0, len(r.Columns)-1)
	}
	if m.colOffset >= len(r.Columns) {
		m.colOffset = max(0, len(r.Columns)-1)
	}
	// Discover total when we reach the last page, unless we already have an exact total.
	if !r.HasMore && !m.knownTotalExact {
		page := r.Page
		m.knownTotalPages = &page
		rows := (int64(r.Page-1)*int64(r.PageSize)) + int64(len(r.Rows))
		m.knownTotalRows = &rows
		// knownTotalExact stays false (tilde shown)
	}
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
			m = m.clearLocalSearch()
			m.mode = modeNormal
			m.statusMsg = ""
			m.knownTotalPages = nil
			m.knownTotalRows = nil
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

	// Route all messages to page size input when it is open.
	if m.mode == modePageSizeInput {
		if ws, ok := msg.(tea.WindowSizeMsg); ok {
			m.width = ws.Width
			m.height = ws.Height
			return m, nil
		}
		if tkMsg, ok := msg.(spinner.TickMsg); ok {
			if m.loading {
				m.spinner, _ = m.spinner.Update(tkMsg)
			}
			return m, nil
		}
		if kmMsg, ok := msg.(tea.KeyMsg); ok {
			return m.handlePageSizeInputKey(kmMsg)
		}
		// Forward other messages (cursor blink, etc.) to text input.
		m.pageSizeInput, cmd = m.pageSizeInput.Update(msg)
		return m, cmd
	}

	// Route all key messages to the column picker when it is open.
	if m.mode == modeColumnPicker {
		if ws, ok := msg.(tea.WindowSizeMsg); ok {
			m.width = ws.Width
			m.height = ws.Height
			m.columnPicker.width = ws.Width
			m.columnPicker.height = ws.Height
			return m, nil
		}
		if _, ok := msg.(spinner.TickMsg); ok {
			return m, nil
		}
		if km, ok := msg.(tea.KeyMsg); ok {
			switch km.Type {
			case tea.KeyEnter:
				m.columnPicker = m.columnPicker.tryConfirm()
				if m.columnPicker.IsConfirmed() {
					m.columns.Set(m.ds.Name, m.columnPicker.Selection())
					m.mode = modeNormal
					m.knownTotalPages = nil
					m.knownTotalRows = nil
					m = m.clearLocalSearch()
					if m.executor != nil {
						m.loading = true
						return m, tea.Batch(m.spinner.Tick, m.loadPageCmd(1))
					}
				}
			case tea.KeyEsc:
				m.columnPicker = m.columnPicker.cancel()
				m.mode = modeNormal
			default:
				m.columnPicker = m.columnPicker.handleKey(km.String())
			}
		}
		return m, nil
	}

	// Route all messages to the save-perspective overlay when it is open.
	if m.mode == modeSavePerspective {
		if ws, ok := msg.(tea.WindowSizeMsg); ok {
			m.width = ws.Width
			m.height = ws.Height
			return m, nil
		}
		if _, ok := msg.(spinner.TickMsg); ok {
			return m, nil
		}
		// Handle internal result messages from the save command.
		if sm, ok := msg.(perspectiveSavedInternal); ok {
			m.mode = modeNormal
			m.statusMsg = "Saved to " + sm.path
			return m, func() tea.Msg { return PerspectiveSavedMsg{Path: sm.path} }
		}
		if se, ok := msg.(perspectiveSaveErrorInternal); ok {
			m.savePerspective = m.savePerspective.SetError(se.err.Error())
			return m, nil
		}
		var cmd tea.Cmd
		m.savePerspective, cmd = m.savePerspective.Update(msg)
		if m.savePerspective.IsConfirmed() {
			return m, m.savePerspectiveCmd(m.savePerspective.Name())
		}
		if m.savePerspective.IsCancelled() {
			m.mode = modeNormal
			return m, nil
		}
		return m, cmd
	}

	// Route all messages to the referenced-by picker when it is open.
	if m.mode == modeRefByPicker {
		if ws, ok := msg.(tea.WindowSizeMsg); ok {
			m.width = ws.Width
			m.height = ws.Height
			m.refByPicker, _ = m.refByPicker.Update(ws)
			return m, nil
		}
		if _, ok := msg.(spinner.TickMsg); ok {
			if m.loading {
				m.spinner, _ = m.spinner.Update(msg)
			}
			return m, nil
		}
		m.refByPicker, cmd = m.refByPicker.Update(msg)
		if m.refByPicker.IsSelected() {
			m.mode = modeNormal
			sel := m.refByPicker.Selection()
			if m.result != nil && len(m.result.Rows) > 0 && m.rowCursor < len(m.result.Rows) {
				colName := m.result.Columns[m.colCursor].Name
				cellValue := m.result.Rows[m.rowCursor][colName]
				return m.drillReverse(sel, cellValue)
			}
			return m, nil
		}
		if m.refByPicker.IsCancelled() {
			m.mode = modeNormal
			return m, nil
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
		wasApplied := m.presetApplied
		m = m.applyLoadedResult((*dataset.QueryResult)(msg))
		if !wasApplied && m.presetApplied {
			m.loading = true
			return m, tea.Batch(m.spinner.Tick, m.loadPageCmd(1))
		}
		return m, nil

	case rowsLoadedInternal:
		if msg.seq != m.drillSeq {
			return m, nil
		}
		wasApplied := m.presetApplied
		m = m.applyLoadedResult(msg.result)
		if !wasApplied && m.presetApplied {
			m.loading = true
			return m, tea.Batch(m.spinner.Tick, m.loadPageCmd(1))
		}
		return m, nil

	case countLoadedMsg:
		if msg.seq != m.drillSeq {
			return m, nil
		}
		if msg.err != nil {
			m.statusMsg = "goto last failed: " + msg.err.Error()
			return m, nil
		}
		// Store exact totals from COUNT(*).
		if msg.result.TotalPages != nil {
			m.knownTotalPages = msg.result.TotalPages
		}
		if msg.result.TotalRows != nil {
			m.knownTotalRows = msg.result.TotalRows
		}
		m.knownTotalExact = true
		m.statusMsg = ""
		if m.knownTotalPages != nil {
			lastPage := *m.knownTotalPages
			m.loading = true
			return m, tea.Batch(m.spinner.Tick, m.loadPageCmd(lastPage))
		}
		return m, nil

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

	case localSearchFlashExpiredMsg:
		m.localSearchFlashing = false
		return m, nil

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
	case modePageSizeInput:
		return m.handlePageSizeInputKey(msg)
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
		m.localSearch = m.localSearch.withInputClosed()
		if m.localSearch.Query() != "" {
			m.localSearchFlashing = true
			return m, tea.Tick(400*time.Millisecond, func(time.Time) tea.Msg { return localSearchFlashExpiredMsg{} })
		}
		return m, nil

	case tea.KeyEsc:
		m = m.clearLocalSearch()
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

func (m RowBrowserModel) handlePageSizeInputKey(msg tea.KeyMsg) (RowBrowserModel, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		return m.applyPageSizeInput()
	case tea.KeyEsc:
		m.mode = modeNormal
		m.pageSizeError = ""
		return m, nil
	case tea.KeyRunes:
		// Silently drop non-digit characters.
		for _, r := range msg.Runes {
			if r < '0' || r > '9' {
				return m, nil
			}
		}
		var cmd tea.Cmd
		m.pageSizeInput, cmd = m.pageSizeInput.Update(msg)
		m.pageSizeError = ""
		return m, cmd
	default:
		// Allow Backspace, Delete, navigation keys.
		var cmd tea.Cmd
		m.pageSizeInput, cmd = m.pageSizeInput.Update(msg)
		return m, cmd
	}
}

func (m RowBrowserModel) applyPageSizeInput() (RowBrowserModel, tea.Cmd) {
	val := strings.TrimSpace(m.pageSizeInput.Value())
	if val == "" {
		m.pageSizeError = "must be between 1 and 10000"
		return m, nil
	}
	n, err := strconv.Atoi(val)
	if err != nil || n < 1 || n > 10000 {
		m.pageSizeError = "must be between 1 and 10000"
		return m, nil
	}
	m.pageSizes.Set(m.ds.Name, n)
	m.mode = modeNormal
	m.pageSizeError = ""
	// Changing page size invalidates the discovered total.
	m.knownTotalPages = nil
	m.knownTotalRows = nil
	m = m.clearLocalSearch()
	// Stay near the current position: compute the absolute row of the cursor
	// and derive which page that row falls on with the new size.
	targetPage := 1
	if m.result != nil {
		absRow := (m.result.Page-1)*m.result.PageSize + m.rowCursor
		targetPage = absRow/n + 1
		cursorInPage := absRow % n
		m.pendingRowCursor = &cursorInPage
	}
	if m.executor != nil {
		m.loading = true
		return m, tea.Batch(m.spinner.Tick, m.loadPageCmd(targetPage))
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
	// Esc clears local search if active (before checking drill stack)
	if key.Matches(msg, m.keys.Back) && m.localSearch.IsActive() {
		m = m.clearLocalSearch()
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

	case key.Matches(msg, m.keys.Sort):
		m = m.cycleSort()
		m.statusMsg = ""
		m.knownTotalPages = nil
		m.knownTotalRows = nil
		if m.executor != nil {
			m.loading = true
			return m, tea.Batch(m.spinner.Tick, m.loadPageCmd(1))
		}

	case key.Matches(msg, m.keys.Export):
		m.mode = modeExportMenu

	case key.Matches(msg, m.keys.NextPage):
		if m.result.HasMore {
			m = m.clearLocalSearch()
			m.loading = true
			return m, tea.Batch(m.spinner.Tick, m.loadPageCmd(m.result.Page+1))
		}

	case key.Matches(msg, m.keys.PrevPage):
		if m.result.Page > 1 {
			m = m.clearLocalSearch()
			m.loading = true
			return m, tea.Batch(m.spinner.Tick, m.loadPageCmd(m.result.Page-1))
		}

	case key.Matches(msg, m.keys.FirstPage):
		if m.result.Page != 1 {
			m = m.clearLocalSearch()
			m.loading = true
			return m, tea.Batch(m.spinner.Tick, m.loadPageCmd(1))
		}

	case key.Matches(msg, m.keys.LastPage):
		m.statusMsg = "Finding last page..."
		return m, m.loadCountCmd()

	case key.Matches(msg, m.keys.PageSize):
		sz := m.currentPageSize()
		m.pageSizeInput.SetValue(strconv.Itoa(sz))
		m.pageSizeInput.CursorEnd()
		m.mode = modePageSizeInput
		m.pageSizeError = ""
		return m, m.pageSizeInput.Focus()

	case key.Matches(msg, m.keys.Down):
		if m.localSearch.IsActive() {
			prevCur := m.localSearch.MatchCursor()
			m.localSearch = m.localSearch.withNextMatch()
			cur := m.localSearch.MatchCursor()
			visible := m.visibleRowCount()
			if cur < prevCur {
				// wrapped around to first match
				m.localSearchOffset = 0
			} else if visible > 0 && cur >= m.localSearchOffset+visible {
				m.localSearchOffset = cur - visible + 1
			}
		} else if m.rowCursor < len(m.result.Rows)-1 {
			m.rowCursor++
			if visible := m.visibleRowCount(); visible > 0 && m.rowCursor >= m.rowOffset+visible {
				m.rowOffset = m.rowCursor - visible + 1
			}
		}
	case key.Matches(msg, m.keys.Up):
		if m.localSearch.IsActive() {
			prevCur := m.localSearch.MatchCursor()
			m.localSearch = m.localSearch.withPrevMatch()
			cur := m.localSearch.MatchCursor()
			visible := m.visibleRowCount()
			if cur > prevCur {
				// wrapped around to last match — scroll to show it
				m.localSearchOffset = max(0, m.localSearch.MatchCount()-visible)
			} else if cur < m.localSearchOffset {
				m.localSearchOffset = cur
			}
		} else if m.rowCursor > 0 {
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

	case key.Matches(msg, m.keys.Enter), key.Matches(msg, m.keys.DrillFwd):
		return m.handleDrillDown()

	case key.Matches(msg, m.keys.DrillReverse):
		return m.handleReverseDrillDown()

	case key.Matches(msg, m.keys.ViewCell):
		return m.openCellViewer()

	case key.Matches(msg, m.keys.ColumnPicker):
		return m.openColumnPicker()

	case key.Matches(msg, m.keys.SavePerspective):
		if m.ds.Kind == dataset.KindTable || m.ds.Kind == dataset.KindView || m.ds.Kind == dataset.KindPerspective {
			return m.openSavePerspective()
		}
	}
	return m, nil
}

// openSavePerspective opens the save-perspective name overlay.
// When re-opening from a KindPerspective view, the input is pre-filled with the
// current perspective name so the user can confirm or rename before saving.
func (m RowBrowserModel) openSavePerspective() (RowBrowserModel, tea.Cmd) {
	if m.result == nil {
		return m, nil
	}
	sp := NewSavePerspectiveModel()
	if m.ds.Kind == dataset.KindPerspective {
		sp = sp.WithInitialName(m.ds.Name)
	}
	var focusCmd tea.Cmd
	m.savePerspective, focusCmd = sp.Focus()
	m.mode = modeSavePerspective
	return m, focusCmd
}

// savePerspectiveCmd attempts to write the perspective to disk and returns a
// perspectiveSavedInternal or perspectiveSaveErrorInternal message.
func (m RowBrowserModel) savePerspectiveCmd(name string) tea.Cmd {
	// Collect active state.
	cols := m.columns.VisibleColumns(m.ds.Name) // nil = all columns in schema order
	filters := m.filters
	sort := m.sort
	table := m.ds.Table
	datasource := m.activeDatasource
	configPath := m.configPath

	p := buildPerspectiveConfig(name, cols, filters, sort)

	return func() tea.Msg {
		path, err := resolveSavePath(configPath)
		if err != nil {
			return perspectiveSaveErrorInternal{err: err}
		}
		if err := config.AppendPerspective(path, datasource, table, p); err != nil {
			return perspectiveSaveErrorInternal{err: err}
		}
		return perspectiveSavedInternal{path: path}
	}
}

// resolveSavePath determines the config file path to write to.
// It tries configPath first, then ~/.datacow/config.yaml, then ./datacow.yaml.
func resolveSavePath(configPath string) (string, error) {
	if configPath != "" {
		return configPath, nil
	}
	home, err := os.UserHomeDir()
	if err == nil {
		homePath := filepath.Join(home, ".datacow", "config.yaml")
		if err2 := os.MkdirAll(filepath.Dir(homePath), 0o755); err2 == nil {
			return homePath, nil
		}
	}
	return "./datacow.yaml", nil
}

// buildPerspectiveConfig constructs a PerspectiveConfig from active row browser state.
func buildPerspectiveConfig(name string, cols []string, filters []dataset.Filter, sort *dataset.Sort) config.PerspectiveConfig {
	p := config.PerspectiveConfig{Name: name, Columns: cols}
	for _, f := range filters {
		p.Filters = append(p.Filters, config.FilterConfig{
			Column:   f.Column,
			Operator: f.Operator,
			Value:    f.Value,
		})
	}
	if sort != nil {
		p.Sort = []config.SortConfig{{Column: sort.Column, Desc: sort.Desc}}
	}
	return p
}

// applyPresetColumns sets the column registry selection to match the perspective's preset.
func (m RowBrowserModel) applyPresetColumns(presetCols []string) RowBrowserModel {
	original := m.columns.GetOriginal(m.ds.Name)
	if original == nil {
		return m
	}
	presetSet := make(map[string]bool, len(presetCols))
	for _, c := range presetCols {
		presetSet[c] = true
	}
	newSel := make([]ColumnSelection, 0, len(original))
	for _, c := range presetCols {
		newSel = append(newSel, ColumnSelection{Name: c, Visible: true})
	}
	for _, c := range original {
		if !presetSet[c.Name] {
			newSel = append(newSel, ColumnSelection{Name: c.Name, Visible: false})
		}
	}
	m.columns.Set(m.ds.Name, newSel)
	return m
}

// openColumnPicker opens the column picker overlay for the active dataset.
func (m RowBrowserModel) openColumnPicker() (RowBrowserModel, tea.Cmd) {
	if m.result == nil {
		return m, nil
	}
	sel := m.columns.Get(m.ds.Name)
	if sel == nil {
		return m, nil
	}
	orig := m.columns.GetOriginal(m.ds.Name)
	m.columnPicker = NewColumnPickerModel(orig, sel, m.width, m.height)
	m.mode = modeColumnPicker
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
		ds:             m.ds,
		result:         m.result,
		fks:            m.fks,
		pkCols:         m.pkCols,
		colWidths:       m.colWidths,
		colOffset:      m.colOffset,
		colCursor:      m.colCursor,
		rowOffset:      m.rowOffset,
		rowCursor:      m.rowCursor,
		filters:        m.filters,
		sort:           m.sort,
		breadcrumb:     fmt.Sprintf("→ %s = %v → %s", fk.Column, formatCellValue(cellValue), fk.ReferencedTable),
		pageSize:       m.currentPageSize(),
		knownTotalPages: m.knownTotalPages,
		knownTotalRows:  m.knownTotalRows,
		knownTotalExact: m.knownTotalExact,
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
	m = m.clearLocalSearch()
	m.knownTotalPages = nil
	m.knownTotalRows = nil
	m.knownTotalExact = false
	m.drillSeq++

	return m, tea.Batch(m.spinner.Tick, m.loadPageCmd(1), m.loadFKsCmd(), m.loadPKColsCmd())
}

// handleReverseDrillDown implements the `<` key: show tables referencing the current cell's column.
func (m RowBrowserModel) handleReverseDrillDown() (RowBrowserModel, tea.Cmd) {
	if m.result == nil || len(m.result.Rows) == 0 || m.rowCursor >= len(m.result.Rows) {
		return m, nil
	}
	if m.schemaCache == nil || !m.schemaCache.Ready() {
		m.statusMsg = "schema loading — try again"
		return m, nil
	}

	colName := m.result.Columns[m.colCursor].Name

	var inboundFKs []schema.InboundFK
	for _, t := range m.schemaCache.Tables() {
		if t.Name == m.ds.Table {
			for _, ibfk := range t.ReferencedBy {
				if ibfk.ToColumn == colName {
					inboundFKs = append(inboundFKs, ibfk)
				}
			}
			break
		}
	}

	if len(inboundFKs) == 0 {
		m.statusMsg = "no tables reference this column"
		return m, nil
	}

	row := m.result.Rows[m.rowCursor]
	cellValue := row[colName]
	if cellValue == nil {
		return m, nil
	}

	if len(inboundFKs) == 1 {
		return m.drillReverse(inboundFKs[0], cellValue)
	}

	picker := NewRefByPickerModel(inboundFKs, m.ds.Table, colName, formatCellValue(cellValue))
	picker, _ = picker.Update(tea.WindowSizeMsg{Width: m.width, Height: m.height})
	var focusCmd tea.Cmd
	m.refByPicker, focusCmd = picker.Focus()
	m.mode = modeRefByPicker
	return m, focusCmd
}

// drillReverse pushes a reverse-FK level onto the drill stack and loads the referencing table.
func (m RowBrowserModel) drillReverse(ibfk schema.InboundFK, cellValue any) (RowBrowserModel, tea.Cmd) {
	breadcrumb := fmt.Sprintf("← %s.%s = %s ← %s", ibfk.FromTable, ibfk.FromColumn, formatCellValue(cellValue), m.ds.Table)

	saved := savedLevel{
		ds:              m.ds,
		result:          m.result,
		fks:             m.fks,
		pkCols:          m.pkCols,
		colWidths:       m.colWidths,
		colOffset:       m.colOffset,
		colCursor:       m.colCursor,
		rowOffset:       m.rowOffset,
		rowCursor:       m.rowCursor,
		filters:         m.filters,
		sort:            m.sort,
		breadcrumb:      breadcrumb,
		pageSize:        m.currentPageSize(),
		knownTotalPages: m.knownTotalPages,
		knownTotalRows:  m.knownTotalRows,
		knownTotalExact: m.knownTotalExact,
	}
	m.drillStack = append(m.drillStack, saved)

	m.ds = dataset.Dataset{Name: ibfk.FromTable, Table: ibfk.FromTable}
	m.result = nil
	m.fks = nil
	m.pkCols = nil
	m.colWidths = nil
	m.colOffset = 0
	m.colCursor = 0
	m.rowOffset = 0
	m.rowCursor = 0
	m.filters = []dataset.Filter{{
		Column:   ibfk.FromColumn,
		Operator: "=",
		Value:    cellValue,
	}}
	m.sort = nil
	m.loading = true
	m.err = nil
	m.statusMsg = ""
	m.mode = modeNormal
	m = m.clearLocalSearch()
	m.knownTotalPages = nil
	m.knownTotalRows = nil
	m.knownTotalExact = false
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
	m = m.clearLocalSearch()
	m.knownTotalPages = last.knownTotalPages
	m.knownTotalRows = last.knownTotalRows
	m.knownTotalExact = last.knownTotalExact
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

	opts := dataset.QueryOptions{Filters: m.filters, Sort: m.sort, Columns: m.columns.VisibleColumns(m.ds.Name)}
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

// TotalPages returns the discovered total page count and whether it is known.
func (m RowBrowserModel) TotalPages() (int, bool) {
	if m.knownTotalPages == nil {
		return 0, false
	}
	return *m.knownTotalPages, true
}

// TotalRows returns the discovered total row count and whether it is known.
func (m RowBrowserModel) TotalRows() (int64, bool) {
	if m.knownTotalRows == nil {
		return 0, false
	}
	return *m.knownTotalRows, true
}

func (m RowBrowserModel) ColOffset() int               { return m.colOffset }
func (m RowBrowserModel) ColCursor() int               { return m.colCursor }
func (m RowBrowserModel) DatasetName() string          { return m.ds.Name }
func (m RowBrowserModel) DatasetKind() dataset.Kind    { return m.ds.Kind }
func (m RowBrowserModel) RowCursor() int               { return m.rowCursor }
func (m RowBrowserModel) RowOffset() int               { return m.rowOffset }
func (m RowBrowserModel) VisibleRowCount() int         { return m.visibleRowCount() }
func (m RowBrowserModel) ForeignKeys() []db.ForeignKey { return m.fks }
func (m RowBrowserModel) DrillDepth() int              { return len(m.drillStack) }
func (m RowBrowserModel) IsLoading() bool              { return m.loading }
func (m RowBrowserModel) Err() error                   { return m.err }
func (m RowBrowserModel) Filters() []dataset.Filter    { return m.filters }
func (m RowBrowserModel) ActiveSort() *dataset.Sort    { return m.sort }
func (m RowBrowserModel) IsFilterModalOpen() bool      { return m.mode == modeFilterModal }
func (m RowBrowserModel) IsPageSizeInputOpen() bool    { return m.mode == modePageSizeInput }
func (m RowBrowserModel) IsSavePerspectiveOpen() bool  { return m.mode == modeSavePerspective }
func (m RowBrowserModel) IsLocalSearchInputActive() bool {
	return m.localSearch.InputActive()
}

// OnFocusGained is called by app.go when the row browser pane gains keyboard focus.
// If a local search is held, it triggers the 400ms attention flash.
func (m RowBrowserModel) OnFocusGained() (RowBrowserModel, tea.Cmd) {
	if m.localSearch.IsActive() && !m.localSearch.InputActive() {
		m.localSearchFlashing = true
		return m, tea.Tick(400*time.Millisecond, func(time.Time) tea.Msg { return localSearchFlashExpiredMsg{} })
	}
	return m, nil
}
// clearLocalSearch resets the local search state and its scroll offset.
func (m RowBrowserModel) clearLocalSearch() RowBrowserModel {
	m.localSearch = newLocalSearch()
	m.localSearchOffset = 0
	return m
}


func (m RowBrowserModel) ExportMenuActive() bool { return m.mode == modeExportMenu }

// BlocksGlobalKeys returns true when the row browser is consuming keys that
// should not be intercepted by the App (modal open, local search input active).
func (m RowBrowserModel) BlocksGlobalKeys() bool {
	return m.mode == modeFilterModal || m.localSearch.InputActive() || m.mode == modePageSizeInput || m.mode == modeRefByPicker || m.mode == modeColumnPicker || m.mode == modeSavePerspective
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
	if m.hasActivePills() {
		filterPillLines = 1
	}
	bottomBarLines := 0
	if m.localSearch.IsActive() || m.mode == modeExportMenu || m.mode == modeExporting || m.mode == modePageSizeInput {
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

// IsRefByColumn reports whether the currently selected column is referenced by at least one inbound FK.
func (m RowBrowserModel) IsRefByColumn() bool {
	if m.result == nil || m.colCursor >= len(m.result.Columns) {
		return false
	}
	return m.refByColSetFromCache()[m.result.Columns[m.colCursor].Name]
}

// refByColSetFromCache returns a set of column names in the active table that are referenced by inbound FKs.
func (m RowBrowserModel) refByColSetFromCache() map[string]bool {
	if m.schemaCache == nil || !m.schemaCache.Ready() || m.ds.Table == "" {
		return nil
	}
	for _, t := range m.schemaCache.Tables() {
		if t.Name == m.ds.Table {
			if len(t.ReferencedBy) == 0 {
				return nil
			}
			result := make(map[string]bool, len(t.ReferencedBy))
			for _, ibfk := range t.ReferencedBy {
				result[ibfk.ToColumn] = true
			}
			return result
		}
	}
	return nil
}

// NeedsBackKey returns true when the row browser is consuming the Back key
// internally, so the app should not intercept it.
func (m RowBrowserModel) NeedsBackKey() bool {
	return m.mode == modeFilterModal || m.localSearch.IsActive() || len(m.drillStack) > 0 || m.mode == modePageSizeInput || m.mode == modeRefByPicker || m.mode == modeColumnPicker || m.mode == modeSavePerspective
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
	if m.localSearch.InputActive() {
		return m.localSearch.StatusText()
	}
	if m.statusMsg != "" {
		return m.statusMsg
	}
	if m.result == nil {
		return m.ds.Name
	}

	var base string
	if m.knownTotalPages != nil && m.knownTotalRows != nil {
		rowsStr := formatCount(*m.knownTotalRows)
		if !m.knownTotalExact {
			rowsStr = "~" + rowsStr
		}
		base = fmt.Sprintf("%s  page %d/%d  %s rows",
			m.ds.Name,
			m.result.Page,
			*m.knownTotalPages,
			rowsStr,
		)
	} else {
		base = fmt.Sprintf("%s  page %d", m.ds.Name, m.result.Page)
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

	// RefBy picker overlay takes the full view area.
	if m.mode == modeRefByPicker {
		return style.Content.Width(m.width).Height(m.height).Render(m.refByPicker.View())
	}

	// Column picker overlay takes the full view area.
	if m.mode == modeColumnPicker {
		return style.Content.Width(m.width).Height(m.height).Render(m.columnPicker.View())
	}

	// Save-perspective overlay: centered box over the content.
	if m.mode == modeSavePerspective {
		overlay := m.savePerspective.View()
		return style.Content.Width(m.width).Height(m.height).Render(overlay)
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
	if m.hasActivePills() {
		sections = append(sections, m.renderActivePills())
		filterPillLines = 1
	}
	bottomBarLines := 0
	if m.localSearch.IsActive() || m.mode == modeExportMenu || m.mode == modeExporting || m.mode == modePageSizeInput {
		bottomBarLines = 1
	}

	tableHeight := m.height - parentLines - filterPillLines - bottomBarLines
	if tableHeight < 2 {
		tableHeight = 2
	}
	sections = append(sections, m.renderTable(tableHeight))

	// Build the bottom bar separately so it anchors to the window bottom regardless
	// of how many data rows are visible. renderTable() does not pad to tableHeight,
	// so appending the bar to sections would leave it floating just after the last
	// data row. Instead: render sections into Height(m.height-1) (which forces
	// lipgloss to pad the empty space), then concatenate the bar — same pattern as
	// tablelist.go.
	var bottomBar string
	switch {
	case m.localSearch.IsActive() && !m.localSearch.InputActive():
		barText := m.localSearch.StatusText() + "  ·  ↑/↓ navigate  ·  esc clear"
		barStyle := style.FilterBarHeld
		if m.localSearchFlashing {
			barStyle = style.FilterBarFlash
		}
		bottomBar = barStyle.Width(m.width).Render(barText)
	case m.localSearch.InputActive():
		bottomBar = style.FilterBar.Width(m.width).Render(m.localSearch.View(m.width))
	case m.mode == modeExportMenu:
		bottomBar = style.ExportBar.Width(m.width).Render(
			style.StatusKey.Render("c") + style.StatusDesc.Render(" CSV") +
				"  " +
				style.StatusKey.Render("x") + style.StatusDesc.Render(" Excel") +
				"  " +
				style.StatusKey.Render("esc") + style.StatusDesc.Render(" cancel"),
		)
	case m.mode == modeExporting:
		bottomBar = style.ExportBar.Width(m.width).Render(
			style.Progress.Render(m.exportProgressText()),
		)
	case m.mode == modePageSizeInput:
		psContent := "Page size: " + m.pageSizeInput.View()
		if m.pageSizeError != "" {
			psContent += "  (" + m.pageSizeError + ")"
		}
		bottomBar = style.FilterBar.Width(m.width).Render(psContent)
	}

	mainHeight := m.height
	if bottomBar != "" {
		mainHeight--
	}
	main := style.Content.Width(m.width).Height(mainHeight).Render(strings.Join(sections, "\n"))
	if bottomBar != "" {
		return main + "\n" + bottomBar
	}
	return main
}

func (m RowBrowserModel) hasActivePills() bool {
	if len(m.filters) > 0 || m.sort != nil {
		return true
	}
	return !m.columns.IsDefault(m.ds.Name)
}

func (m RowBrowserModel) renderActivePills() string {
	var parts []string
	for _, f := range m.filters {
		parts = append(parts, style.FilterPill.Render(formatFilterLabel(f)))
	}
	if m.sort != nil {
		arrow := "↑"
		if m.sort.Desc {
			arrow = "↓"
		}
		parts = append(parts, style.FilterPillSelected.Render(m.sort.Column+" "+arrow))
	}
	if !m.columns.IsDefault(m.ds.Name) {
		visible, total := m.columns.CountVisible(m.ds.Name)
		parts = append(parts, style.FilterPillSelected.Render(fmt.Sprintf("cols %d/%d", visible, total)))
	}
	return strings.Join(parts, " ")
}

// renderSavedLevel renders a compact summary of an ancestor level.
func (m RowBrowserModel) renderSavedLevel(level savedLevel) string {
	if level.result == nil || len(level.result.Columns) == 0 {
		return ""
	}

	var titleSuffix string
	if level.knownTotalRows != nil {
		titleSuffix = fmt.Sprintf(" (%s rows)", formatCount(*level.knownTotalRows))
	}
	sectionTitle := style.DrillSep.Render(
		fmt.Sprintf("─ %s%s ", level.ds.Name, titleSuffix),
	)

	cols := level.result.Columns
	rows := level.result.Rows
	visible := visibleColumns(cols, level.colWidths, level.colOffset, m.width)
	if len(visible) == 0 {
		visible = []int{level.colOffset}
	}

	fkCols := fkColSet(level.fks)
	header := buildHeader(cols, level.colWidths, visible, level.sort, level.colCursor, fkCols, nil)
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
	refByCols := m.refByColSetFromCache()
	header := buildHeader(cols, m.colWidths, visible, m.sort, m.colCursor, fkCols, refByCols)
	sep := buildSeparator(m.colWidths, visible)

	maxRows := max(0, height-2)

	lines := make([]string, 0, maxRows+2)
	lines = append(lines, header, sep)

	if m.localSearch.IsActive() && m.localSearch.Query() != "" {
		matchRows := m.localSearch.MatchRows()
		if len(matchRows) == 0 {
			lines = append(lines, style.Muted.Render("  no matches for "+strconv.Quote(m.localSearch.Query())))
		} else {
			cur := m.localSearch.MatchCursor()
			cursorRowIdx := matchRows[cur]
			start := m.localSearchOffset
			if start+maxRows > len(matchRows) {
				start = max(0, len(matchRows)-maxRows)
			}
			for fi := start; fi < len(matchRows) && fi-start < maxRows; fi++ {
				rowIdx := matchRows[fi]
				lines = append(lines, buildRow(rows[rowIdx], cols, m.colWidths, visible, rowIdx, cursorRowIdx, m.colCursor, fkCols, &m.localSearch, m.localSearch.Query()))
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

func buildHeader(cols []db.Column, widths []int, visible []int, sort *dataset.Sort, cursor int, fkCols map[string]bool, refByCols map[string]bool) string {
	parts := make([]string, len(visible))
	for j, i := range visible {
		name := cols[i].Name
		w := widths[i]
		isFKCol := fkCols[name]
		isRefByCol := refByCols[name]

		// Build the name/sort portion and the optional RefBy glyph separately.
		var nameStr string
		var glyphStr string
		if sort != nil && sort.Column == name {
			indicator := "↑"
			if sort.Desc {
				indicator = "↓"
			}
			usable := w - 2
			if isRefByCol {
				usable--
			}
			if usable < 0 {
				usable = 0
			}
			nameCell := runewidth.FillRight(runewidth.Truncate(name, usable, "…"), usable)
			nameStr = nameCell + " " + indicator
		} else {
			usable := w
			if isRefByCol {
				usable--
			}
			if usable < 0 {
				usable = 0
			}
			nameStr = runewidth.FillRight(runewidth.Truncate(name, usable, "…"), usable)
		}
		if isRefByCol {
			glyphStr = "↩"
		}

		switch {
		case i == cursor && isFKCol && isRefByCol:
			parts[j] = style.FKColHeaderActive.Render(nameStr) + style.RefByColHeaderActive.Render(glyphStr)
		case i == cursor && isFKCol:
			parts[j] = style.FKColHeaderActive.Render(nameStr)
		case i == cursor && isRefByCol:
			parts[j] = style.RefByColHeaderActive.Render(nameStr + glyphStr)
		case i == cursor:
			parts[j] = style.ColHeaderActive.Render(nameStr)
		case isFKCol && isRefByCol:
			parts[j] = style.FKColHeader.Render(nameStr) + style.RefByColHeader.Render(glyphStr)
		case isFKCol:
			parts[j] = style.FKColHeader.Render(nameStr)
		case isRefByCol:
			parts[j] = style.RefByColHeader.Render(nameStr + glyphStr)
		default:
			parts[j] = style.ColHeader.Render(nameStr)
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
