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
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"

	"github.com/polesen/datacow/internal/core/dataset"
	"github.com/polesen/datacow/internal/core/db"
	"github.com/polesen/datacow/internal/core/schema"
	"github.com/polesen/datacow/internal/tui/keys"
	"github.com/polesen/datacow/internal/tui/style"
)

type ErrMsg struct{ Err error }

type TablesLoadedMsg []dataset.Dataset

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

// filterMatch tracks how a dataset matched the current filter query.
type filterMatch struct {
	byName bool // matched dataset name
	bySub  bool // matched a column, FK target, or index name from the schema cache
}

// filterFlashExpiredMsg is sent when the 400ms filter-bar flash timer expires.
type filterFlashExpiredMsg struct{}

type TableListModel struct {
	datasets        []dataset.Dataset
	tree            []treeNode
	cursor          int
	scrollOffset    int // in visible-line space (not dataset-index space)
	spinner         spinner.Model
	loading         bool
	err             error
	keys            keys.Map
	width           int
	height          int
	resolver        *dataset.Resolver
	executor        *dataset.Executor
	client          db.Client
	schemaCache     *schema.Cache
	filterInputOpen bool
	filterQuery     string
	filterInput     textinput.Model
	filterFlashing  bool
	savedCursorName string // cursor name saved when filter first opened, restored on Esc
}

// NewTableListModel creates a TableListModel in the initial loading state.
// resolver, executor, client, and cache may be nil for testing.
func NewTableListModel(k keys.Map, resolver *dataset.Resolver, executor *dataset.Executor, client db.Client, cache *schema.Cache) TableListModel {
	ti := textinput.New()
	ti.Placeholder = "filter tables…"
	ti.Prompt = "/"
	ti.CharLimit = 100
	return TableListModel{
		spinner:     newSpinner(),
		loading:     true,
		keys:        k,
		resolver:    resolver,
		executor:    executor,
		client:      client,
		schemaCache: cache,
		filterInput: ti,
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
		m.filterInput.Width = max(1, m.width-2) // -2 for the "/" prompt
		m = m.ensureCursorVisible()
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
		return m, nil

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

	case filterFlashExpiredMsg:
		m.filterFlashing = false
		return m, nil

	case tea.KeyMsg:
		if m.loading || m.err != nil {
			return m, nil
		}

		// When filter input is open, intercept filter-specific keys and route the rest to textinput.
		if m.filterInputOpen {
			switch {
			case key.Matches(msg, m.keys.Back):
				m = m.clearFilter()
				return m, nil
			case msg.Type == tea.KeyEnter:
				m.filterInputOpen = false
				m.filterInput.Blur()
				if m.filterQuery != "" {
					m.filterFlashing = true
					return m, tea.Tick(400*time.Millisecond, func(time.Time) tea.Msg { return filterFlashExpiredMsg{} })
				}
				return m, nil
			case key.Matches(msg, m.keys.Up):
				visible := m.visibleDatasetIndices()
				for j := len(visible) - 1; j >= 0; j-- {
					if visible[j] < m.cursor {
						m.cursor = visible[j]
						m = m.ensureCursorVisible()
						break
					}
				}
				return m, nil
			case key.Matches(msg, m.keys.Down):
				visible := m.visibleDatasetIndices()
				for _, i := range visible {
					if i > m.cursor {
						m.cursor = i
						m = m.ensureCursorVisible()
						break
					}
				}
				return m, nil
			default:
				var inputCmd tea.Cmd
				m.filterInput, inputCmd = m.filterInput.Update(msg)
				newQuery := m.filterInput.Value()
				if newQuery != m.filterQuery {
					m.filterQuery = newQuery
					var filterCmd tea.Cmd
					m, filterCmd = m.applyFilter()
					return m, tea.Batch(inputCmd, filterCmd)
				}
				return m, inputCmd
			}
		}

		// Normal mode (input not open).
		if len(m.datasets) == 0 {
			return m, nil
		}
		switch {
		case key.Matches(msg, m.keys.TableListFilter):
			if m.filterQuery == "" && m.cursor >= 0 && m.cursor < len(m.datasets) {
				m.savedCursorName = m.datasets[m.cursor].Name
			}
			m.filterInput.SetValue(m.filterQuery)
			m.filterInput.CursorEnd()
			m.filterInputOpen = true
			cmd = m.filterInput.Focus()
			return m, cmd

		case key.Matches(msg, m.keys.Back):
			if m.filterQuery != "" {
				m = m.clearFilter()
				return m, nil
			}
			// No filter active — fall through; app.go handles remaining Esc cases.

		case key.Matches(msg, m.keys.Up):
			visible := m.visibleDatasetIndices()
			for j := len(visible) - 1; j >= 0; j-- {
				if visible[j] < m.cursor {
					m.cursor = visible[j]
					m = m.ensureCursorVisible()
					break
				}
			}

		case key.Matches(msg, m.keys.Down):
			visible := m.visibleDatasetIndices()
			for _, i := range visible {
				if i > m.cursor {
					m.cursor = i
					m = m.ensureCursorVisible()
					break
				}
			}

		case key.Matches(msg, m.keys.Right):
			if m.FocusedExpandable() && !m.FocusedExpanded() {
				return m.expandFocused()
			}

		case key.Matches(msg, m.keys.Left):
			if m.FocusedExpanded() {
				m.tree[m.cursor].expanded = false
				m = m.ensureCursorVisible()
			}
		}
		return m, nil
	}

	return m, nil
}

// ---- Filter methods ----

// FilterActive reports whether a filter query is currently set (input open or held).
func (m TableListModel) FilterActive() bool { return m.filterQuery != "" }

// FilterInputActive reports whether the filter text input is currently focused.
func (m TableListModel) FilterInputActive() bool { return m.filterInputOpen }

// BlocksGlobalKeys reports whether the filter input should block global key shortcuts.
func (m TableListModel) BlocksGlobalKeys() bool { return m.filterInputOpen }

// FilterStatus returns a status bar string describing the active filter, or "".
func (m TableListModel) FilterStatus() string {
	if m.filterQuery == "" {
		return ""
	}
	matches := m.computeFilter()
	return fmt.Sprintf("filter: %q  %d/%d", m.filterQuery, len(matches), len(m.datasets))
}

// ClearFilter resets any held filter and closes the input. Safe to call from app.go.
func (m TableListModel) ClearFilter() TableListModel { return m.clearFilter() }

// OnFocusGained should be called whenever the tables pane gains keyboard focus.
// If a filter is held, it triggers a 400ms attention flash on the filter bar.
func (m TableListModel) OnFocusGained() (TableListModel, tea.Cmd) {
	if m.filterQuery != "" {
		m.filterFlashing = true
		return m, tea.Tick(400*time.Millisecond, func(time.Time) tea.Msg { return filterFlashExpiredMsg{} })
	}
	return m, nil
}

// OnCacheReady re-applies the filter now that the schema cache has data.
// Should be called from app.go when schemaCacheReadyMsg arrives.
func (m TableListModel) OnCacheReady() (TableListModel, tea.Cmd) {
	if m.filterQuery == "" {
		return m, nil
	}
	return m.applyFilter()
}

// computeFilter returns the set of dataset indices that match the current filter query,
// along with how they matched. Returns nil when filterQuery is empty.
func (m TableListModel) computeFilter() map[int]filterMatch {
	if m.filterQuery == "" {
		return nil
	}
	q := strings.ToLower(m.filterQuery)

	var tableByName map[string]*schema.Table
	cacheReady := m.schemaCache != nil && m.schemaCache.Ready()
	if cacheReady {
		tables := m.schemaCache.Tables()
		tableByName = make(map[string]*schema.Table, len(tables))
		for i := range tables {
			tableByName[tables[i].Name] = &tables[i]
		}
	}

	// Build a name→index map for tables, so we can mark tables whose perspectives match.
	tableIdxByName := make(map[string]int, len(m.datasets))
	for i, ds := range m.datasets {
		if ds.Kind == dataset.KindTable || ds.Kind == dataset.KindView {
			tableIdxByName[ds.Name] = i
		}
	}

	result := make(map[int]filterMatch, len(m.datasets))
	for i, ds := range m.datasets {
		var fm filterMatch
		if strings.Contains(strings.ToLower(ds.Name), q) {
			fm.byName = true
		}
		// For perspectives: if the name matches, also mark the parent table as a name match.
		if ds.Kind == dataset.KindPerspective {
			if fm.byName {
				if parentIdx, ok := tableIdxByName[ds.ParentTable]; ok {
					pm := result[parentIdx]
					pm.byName = true
					result[parentIdx] = pm
				}
				// Perspective itself matches.
				result[i] = fm
			}
			continue
		}
		// YAML SQL datasets have no underlying table schema to inspect.
		if cacheReady && ds.Kind != dataset.KindDataset && ds.Table != "" {
			if t, ok := tableByName[ds.Table]; ok {
				for _, col := range t.Columns {
					if strings.Contains(strings.ToLower(col.Name), q) {
						fm.bySub = true
						break
					}
				}
				if !fm.bySub {
					for _, fk := range t.ForeignKeys {
						if strings.Contains(strings.ToLower(fk.ReferencedTable), q) {
							fm.bySub = true
							break
						}
					}
				}
				if !fm.bySub {
					for _, ix := range t.Indexes {
						if strings.Contains(strings.ToLower(ix.Name), q) {
							fm.bySub = true
							break
						}
					}
				}
			}
		}
		if fm.byName || fm.bySub {
			result[i] = fm
		}
	}
	return result
}

// applyFilter re-computes the filter and snaps the cursor if the current row is
// no longer visible. Sub-matched datasets are shown but not auto-expanded — the
// user expands them manually to see which sub-item caused the match.
func (m TableListModel) applyFilter() (TableListModel, tea.Cmd) {
	if m.filterQuery == "" {
		return m, nil
	}
	matches := m.computeFilter()
	m = m.snapCursorToFilter(matches)
	return m, nil
}

// snapCursorToFilter moves the cursor to the first visible dataset if the current one
// is no longer in the filtered set.
func (m TableListModel) snapCursorToFilter(matches map[int]filterMatch) TableListModel {
	if matches == nil {
		return m
	}
	if _, ok := matches[m.cursor]; ok {
		return m
	}
	for i := range m.datasets {
		if _, ok := matches[i]; ok {
			m.cursor = i
			m = m.ensureCursorVisible()
			return m
		}
	}
	m.cursor = len(m.datasets) // past end — empty result
	return m
}

// clearFilter resets all filter state and restores the saved cursor position.
func (m TableListModel) clearFilter() TableListModel {
	m.filterQuery = ""
	m.filterInputOpen = false
	m.filterInput.SetValue("")
	m.filterInput.Blur()
	if m.savedCursorName != "" {
		for i, ds := range m.datasets {
			if ds.Name == m.savedCursorName {
				m.cursor = i
				break
			}
		}
		m.savedCursorName = ""
	}
	m = m.ensureCursorVisible()
	return m
}

// visibleDatasetIndices returns the ordered list of dataset indices that are
// cursor-navigable under the current filter (or all visible indices when no filter is active).
// Perspectives are only included when their parent table is expanded.
func (m TableListModel) visibleDatasetIndices() []int {
	if m.filterQuery == "" {
		result := make([]int, 0, len(m.datasets))
		for i, ds := range m.datasets {
			if ds.Kind == dataset.KindPerspective {
				parentIdx := m.findParentIdx(ds)
				if parentIdx >= 0 && parentIdx < len(m.tree) && m.tree[parentIdx].expanded {
					result = append(result, i)
				}
				continue
			}
			result = append(result, i)
		}
		return result
	}
	matches := m.computeFilter()
	result := make([]int, 0, len(matches))
	for i := range m.datasets {
		if _, ok := matches[i]; ok {
			result = append(result, i)
		}
	}
	return result
}

// findParentIdx returns the dataset index of the parent table for a KindPerspective dataset.
// Returns -1 if not found.
func (m TableListModel) findParentIdx(ds dataset.Dataset) int {
	for i, d := range m.datasets {
		if (d.Kind == dataset.KindTable || d.Kind == dataset.KindView) &&
			(d.Table == ds.ParentTable || d.Name == ds.ParentTable) {
			return i
		}
	}
	return -1
}

// perspectiveIndicesFor returns the dataset indices of all KindPerspective datasets
// whose ParentTable matches the table at parentIdx.
func (m TableListModel) perspectiveIndicesFor(parentIdx int) []int {
	if parentIdx < 0 || parentIdx >= len(m.datasets) {
		return nil
	}
	parent := m.datasets[parentIdx]
	var result []int
	for i, ds := range m.datasets {
		if ds.Kind == dataset.KindPerspective &&
			(ds.ParentTable == parent.Table || ds.ParentTable == parent.Name) {
			result = append(result, i)
		}
	}
	return result
}

// ---- Tree / expansion ----

// FocusedExpandable reports whether the currently-focused row can be expanded
// (i.e. is a KindTable or KindView — not SQL datasets or perspectives, which are leaves).
func (m TableListModel) FocusedExpandable() bool {
	if m.cursor < 0 || m.cursor >= len(m.datasets) {
		return false
	}
	k := m.datasets[m.cursor].Kind
	return k != dataset.KindDataset && k != dataset.KindPerspective
}

// FocusedExpanded reports whether the currently-focused row is expanded.
func (m TableListModel) FocusedExpanded() bool {
	if m.cursor < 0 || m.cursor >= len(m.datasets) {
		return false
	}
	if m.datasets[m.cursor].Kind == dataset.KindPerspective {
		return false
	}
	if m.cursor >= len(m.tree) {
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
	m = m.ensureCursorVisible()
	if len(cmds) == 0 {
		return m, nil
	}
	return m, tea.Batch(cmds...)
}

func (m TableListModel) anyLoading() bool {
	for _, n := range m.tree {
		if n.expState == expLoading || n.indexState == indexLoading {
			return true
		}
	}
	return false
}

// ---- Dataset accessors ----

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

// Reload updates the resolver and re-fetches the dataset list.
// The App calls this after a perspective is saved so the new entry appears in the tree.
func (m TableListModel) Reload(resolver *dataset.Resolver) (TableListModel, tea.Cmd) {
	m.resolver = resolver
	m.loading = true
	return m, m.loadTablesCmd()
}

// SelectByName moves the cursor to the first dataset whose Name matches.
// Returns true if found. Used by the app after a goto selection.
func (m TableListModel) SelectByName(name string) (TableListModel, bool) {
	for i, ds := range m.datasets {
		if ds.Name == name {
			m.cursor = i
			m = m.ensureCursorVisible()
			return m, true
		}
	}
	return m, false
}

// ---- Line building ----

// visibleLine describes one rendered line in the list.
type visibleLine struct {
	datasetIdx  int
	isHeader    bool // true for the dataset header row
	sub         string
	placeholder bool // "no match" placeholder line
}

// buildLines flattens datasets + tree into a sequence of rendered lines.
// Perspectives (KindPerspective) appear as cursor-navigable header rows immediately
// after their parent's expand indicator, above the column/index sub-lines.
func (m TableListModel) buildLines() []visibleLine {
	if m.filterQuery != "" {
		return m.buildFilteredLines()
	}
	out := make([]visibleLine, 0, len(m.datasets))
	for i, ds := range m.datasets {
		// Perspectives are inserted by their parent's expansion logic; skip here.
		if ds.Kind == dataset.KindPerspective {
			continue
		}
		out = append(out, visibleLine{datasetIdx: i, isHeader: true})
		if i >= len(m.tree) || !m.tree[i].expanded {
			continue
		}
		// Add perspective sub-entries before column/FK sub-lines.
		for _, pIdx := range m.perspectiveIndicesFor(i) {
			out = append(out, visibleLine{datasetIdx: pIdx, isHeader: true})
		}
		for _, ln := range m.subLines(i, ds) {
			out = append(out, visibleLine{datasetIdx: i, sub: ln})
		}
	}
	return out
}

func (m TableListModel) buildFilteredLines() []visibleLine {
	matches := m.computeFilter()
	if len(matches) == 0 {
		return []visibleLine{{datasetIdx: -1, isHeader: true, placeholder: true}}
	}
	out := make([]visibleLine, 0)
	for i, ds := range m.datasets {
		// Perspectives are handled after their parent tables.
		if ds.Kind == dataset.KindPerspective {
			continue
		}
		if _, ok := matches[i]; !ok {
			continue
		}
		out = append(out, visibleLine{datasetIdx: i, isHeader: true})
		// Always show matching perspective sub-entries in filter mode (even if parent collapsed).
		for _, pIdx := range m.perspectiveIndicesFor(i) {
			if _, ok := matches[pIdx]; ok {
				out = append(out, visibleLine{datasetIdx: pIdx, isHeader: true})
			}
		}
		if i < len(m.tree) && m.tree[i].expanded {
			for _, ln := range m.subLines(i, ds) {
				out = append(out, visibleLine{datasetIdx: i, sub: ln})
			}
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

	// Foreign keys — now a middle section for table/view rows (Referenced By follows).
	// YAML SQL datasets (KindDataset) cannot be expanded so subLines is never called
	// for them, but guard defensively to ensure correct box-drawing prefixes.
	isLast := ds.Kind == dataset.KindDataset
	fkHeader := "  ├─ Foreign Keys"
	fkSubPrefix := "  │   "
	if isLast {
		fkHeader = "  └─ Foreign Keys"
		fkSubPrefix = "      "
	}
	lines = append(lines, fkHeader)
	switch n.expState {
	case expLoaded:
		if len(n.fks) == 0 {
			lines = append(lines, fkSubPrefix+"(none)")
		} else {
			for _, fk := range n.fks {
				lines = append(lines, fkSubPrefix+formatFK(fk))
			}
		}
	case expError:
		lines = append(lines, fkSubPrefix+"(error)")
	}

	// Referenced By — absent for YAML SQL datasets which have no underlying table.
	if !isLast {
		lines = append(lines, "  └─ Referenced By")
		if m.schemaCache == nil || !m.schemaCache.Ready() {
			lines = append(lines, "      "+m.spinner.View()+" loading…")
		} else {
			var refBy []schema.InboundFK
			for _, t := range m.schemaCache.Tables() {
				if t.Name == ds.Table {
					refBy = t.ReferencedBy
					break
				}
			}
			if len(refBy) == 0 {
				lines = append(lines, "      (none)")
			} else {
				for _, ibfk := range refBy {
					lines = append(lines, "      ← "+ibfk.FromTable+"."+ibfk.FromColumn)
				}
			}
		}
	}

	return lines
}

func (m TableListModel) ensureCursorVisible() TableListModel {
	if m.height <= 0 {
		m.scrollOffset = 0
		return m
	}
	lines := m.buildLines()
	cursorLine := -1
	for i, ln := range lines {
		if ln.isHeader && ln.datasetIdx == m.cursor {
			cursorLine = i
			break
		}
	}
	if cursorLine < 0 {
		return m
	}
	if cursorLine < m.scrollOffset {
		m.scrollOffset = cursorLine
	} else if cursorLine >= m.scrollOffset+m.height {
		m.scrollOffset = cursorLine - m.height + 1
	}
	m.scrollOffset = max(m.scrollOffset, 0)
	maxOffset := max(len(lines)-m.height, 0)
	m.scrollOffset = min(m.scrollOffset, maxOffset)
	return m
}

// ---- Rendering ----

func (m TableListModel) View() string {
	if m.width == 0 {
		return ""
	}

	// Build filter footer (1 line when filter is open or held).
	var footer string
	listHeight := m.height
	if m.filterInputOpen || m.filterQuery != "" {
		listHeight = max(1, m.height-1)
		if m.filterInputOpen {
			hint := ""
			if m.schemaCache == nil || !m.schemaCache.Ready() {
				hint = style.Muted.Render("  (schema loading — name match only)")
			}
			footer = style.FilterBar.Width(m.width).Render(m.filterInput.View() + hint)
		} else {
			matches := m.computeFilter()
			barText := fmt.Sprintf("/ %q  %d/%d  ·  / edit  ·  esc clear", m.filterQuery, len(matches), len(m.datasets))
			barStyle := style.FilterBarHeld
			if m.filterFlashing {
				barStyle = style.FilterBarFlash
			}
			footer = barStyle.Width(m.width).Render(barText)
		}
	}

	if m.loading {
		content := style.Content.Width(m.width).Height(listHeight).Render(
			m.spinner.View() + " Connecting...",
		)
		if footer != "" {
			return content + "\n" + footer
		}
		return content
	}

	if m.err != nil {
		content := style.Content.Width(m.width).Height(listHeight).Render(
			style.Error.Render("Error: " + m.err.Error()),
		)
		if footer != "" {
			return content + "\n" + footer
		}
		return content
	}

	if len(m.datasets) == 0 {
		content := style.Content.Width(m.width).Height(listHeight).Render("No tables found.")
		if footer != "" {
			return content + "\n" + footer
		}
		return content
	}

	lines := m.buildLines()
	maxVisible := listHeight
	if maxVisible <= 0 {
		maxVisible = len(lines)
	}

	end := min(m.scrollOffset+maxVisible, len(lines))

	rendered := make([]string, 0, end-m.scrollOffset)
	for i := m.scrollOffset; i < end; i++ {
		ln := lines[i]
		switch {
		case ln.placeholder:
			msg := fmt.Sprintf("No tables match %q", m.filterQuery)
			rendered = append(rendered, style.RowNormal.Width(m.width).Render(msg))
		case ln.isHeader:
			rendered = append(rendered, m.renderHeaderRow(ln.datasetIdx))
		default:
			rendered = append(rendered, m.renderSubRow(ln.sub))
		}
	}

	content := style.Content.Width(m.width).Height(listHeight).Render(
		strings.Join(rendered, "\n"),
	)
	if footer != "" {
		return content + "\n" + footer
	}
	return content
}

func (m TableListModel) renderHeaderRow(i int) string {
	ds := m.datasets[i]
	const maxNameWidth = 40
	const margin = 2

	// Perspectives use a distinct prefix and badge — no expand indicator.
	if ds.Kind == dataset.KindPerspective {
		return m.renderPerspectiveRow(i, ds)
	}

	badge := datasetKindBadge(ds.Kind)
	var badgeW int
	if badge != "" {
		badgeW = runewidth.StringWidth(badge) + 1 // leading space
	}

	name := runewidth.Truncate(ds.Name, maxNameWidth, "…")
	nameWidth := min(max(m.width-margin*2, 10), maxNameWidth)

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

	// Non-selected rows with an active filter get substring highlighting.
	if !selected && m.filterQuery != "" {
		var namePart string
		if badge != "" {
			availNameW := max(nameWidth-badgeW, 1)
			namePart = highlightSubstrRunes(name, m.filterQuery, style.SearchHighlight, availNameW)
			label := " " + style.QueryLabel.Render(badge)
			line := caret + namePart + label
			w := lipgloss.Width(line)
			if w < m.width {
				line += strings.Repeat(" ", m.width-w)
			}
			return line
		}
		namePart = highlightSubstrRunes(name, m.filterQuery, style.SearchHighlight, nameWidth)
		line := caret + namePart
		w := lipgloss.Width(line)
		if w < m.width {
			line += strings.Repeat(" ", m.width-w)
		}
		return line
	}

	var line string
	if badge != "" {
		availNameW := max(nameWidth-badgeW, 1)
		label := " " + style.QueryLabel.Render(badge)
		if selected {
			label = " " + badge
		}
		line = caret + runewidth.FillRight(name, availNameW) + label
	} else {
		line = caret + runewidth.FillRight(name, nameWidth)
	}

	if selected {
		return style.RowSelected.Width(m.width).Render(line)
	}
	return style.RowNormal.Width(m.width).Render(line)
}

// renderPerspectiveRow renders a KindPerspective dataset row with ⊙ prefix and [P] badge.
func (m TableListModel) renderPerspectiveRow(i int, ds dataset.Dataset) string {
	const maxNameWidth = 40
	const indent = "  ⊙ "
	badge := datasetKindBadge(ds.Kind)
	badgeW := runewidth.StringWidth(badge) + 1
	indentW := runewidth.StringWidth(indent)

	available := max(m.width-indentW-badgeW-1, 4)
	name := runewidth.Truncate(ds.Name, maxNameWidth, "…")
	selected := i == m.cursor

	if !selected && m.filterQuery != "" {
		namePart := highlightSubstrRunes(name, m.filterQuery, style.SearchHighlight, available)
		badgePart := " " + style.PerspectiveBadge.Render(badge)
		line := indent + namePart + badgePart
		w := lipgloss.Width(line)
		if w < m.width {
			line += strings.Repeat(" ", m.width-w)
		}
		return line
	}

	namePart := runewidth.FillRight(runewidth.Truncate(name, available, "…"), available)
	var badgePart string
	if selected {
		badgePart = " " + badge
	} else {
		badgePart = " " + style.PerspectiveBadge.Render(badge)
	}
	line := indent + namePart + badgePart
	if selected {
		return style.RowSelected.Width(m.width).Render(line)
	}
	return style.RowNormal.Width(m.width).Render(line)
}

// renderSubRow renders one expanded sub-row, with substring highlighting when a filter is active.
func (m TableListModel) renderSubRow(sub string) string {
	if m.filterQuery != "" {
		highlighted := highlightSubstrRunes(sub, m.filterQuery, style.SearchHighlight, m.width)
		w := lipgloss.Width(highlighted)
		if w < m.width {
			highlighted += strings.Repeat(" ", m.width-w)
		}
		return highlighted
	}
	sub = runewidth.Truncate(sub, m.width, "")
	return style.RowNormal.Width(m.width).Render(sub)
}

// highlightSubstrRunes returns a string of exactly maxW visible characters with the first
// occurrence of query (case-insensitive) highlighted using sty. Characters are rendered
// individually to guarantee correct display-width accounting.
func highlightSubstrRunes(text, query string, sty lipgloss.Style, maxW int) string {
	if maxW <= 0 {
		return ""
	}
	runes := []rune(text)
	if len(runes) > maxW {
		runes = runes[:maxW-1]
		runes = append(runes, '…')
	}

	matchStart, matchEnd := -1, -1
	if query != "" {
		lowerText := strings.ToLower(string(runes))
		lowerQ := strings.ToLower(query)
		byteIdx := strings.Index(lowerText, lowerQ)
		if byteIdx >= 0 {
			matchStart = len([]rune(lowerText[:byteIdx]))
			matchEnd = matchStart + len([]rune(query))
			if matchEnd > len(runes) {
				matchEnd = len(runes)
			}
		}
	}

	var sb strings.Builder
	for i, r := range runes {
		if matchStart >= 0 && i >= matchStart && i < matchEnd {
			sb.WriteString(sty.Render(string(r)))
		} else {
			sb.WriteRune(r)
		}
	}
	if pad := maxW - len(runes); pad > 0 {
		sb.WriteString(strings.Repeat(" ", pad))
	}
	return sb.String()
}

func datasetKindBadge(k dataset.Kind) string {
	switch k {
	case dataset.KindView:
		return "[view]"
	case dataset.KindDataset:
		return "[dataset]"
	case dataset.KindPerspective:
		return "[P]"
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
