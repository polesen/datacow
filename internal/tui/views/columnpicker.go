package views

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/polesen/datacow/internal/core/db"
	"github.com/polesen/datacow/internal/tui/style"
)

// ColumnSelection represents one column and its visibility state.
type ColumnSelection struct {
	Name    string
	Visible bool
}

// registryEntry holds both the working selection and the original schema order.
type registryEntry struct {
	original []ColumnSelection // schema order, all visible — used for reset and default detection
	current  []ColumnSelection // working copy (may be reordered / hidden)
}

// ColumnRegistry tracks per-dataset column selections for the life of the process.
// It is owned by the App and shared across all RowBrowserModel instances so that
// column choices survive dataset switches and drill-downs.
type ColumnRegistry struct {
	entries map[string]*registryEntry
}

// NewColumnRegistry returns an empty registry.
func NewColumnRegistry() *ColumnRegistry {
	return &ColumnRegistry{entries: make(map[string]*registryEntry)}
}

// Get returns the current column selection for the named dataset, or nil if not set.
// Safe to call on a nil receiver.
func (r *ColumnRegistry) Get(name string) []ColumnSelection {
	if r == nil {
		return nil
	}
	e, ok := r.entries[name]
	if !ok {
		return nil
	}
	return e.current
}

// GetOriginal returns the schema-order column selection for the named dataset.
func (r *ColumnRegistry) GetOriginal(name string) []ColumnSelection {
	if r == nil {
		return nil
	}
	e, ok := r.entries[name]
	if !ok {
		return nil
	}
	return e.original
}

// Set stores the column selection for the named dataset.
// Safe to call on a nil receiver (no-op).
func (r *ColumnRegistry) Set(name string, sel []ColumnSelection) {
	if r == nil {
		return
	}
	e, ok := r.entries[name]
	if !ok {
		return
	}
	e.current = sel
}

// Seed initialises the registry entry for the named dataset from the given schema
// columns if no entry exists yet. All columns are visible in schema order.
func (r *ColumnRegistry) Seed(name string, cols []db.Column) {
	if r == nil {
		return
	}
	if _, ok := r.entries[name]; ok {
		return
	}
	sel := make([]ColumnSelection, len(cols))
	for i, c := range cols {
		sel[i] = ColumnSelection{Name: c.Name, Visible: true}
	}
	orig := make([]ColumnSelection, len(sel))
	copy(orig, sel)
	cur := make([]ColumnSelection, len(sel))
	copy(cur, sel)
	r.entries[name] = &registryEntry{original: orig, current: cur}
}

// VisibleColumns returns the ordered list of visible column names for the dataset,
// or nil if the selection is the default (no entry, or all visible in schema order).
func (r *ColumnRegistry) VisibleColumns(name string) []string {
	if r == nil {
		return nil
	}
	e, ok := r.entries[name]
	if !ok {
		return nil
	}
	// If selection matches the original (all visible, schema order), return nil → SELECT *.
	if selectionIsDefault(e.current, e.original) {
		return nil
	}
	var cols []string
	for _, s := range e.current {
		if s.Visible {
			cols = append(cols, s.Name)
		}
	}
	return cols
}

// IsDefault reports whether the column selection for the dataset is the default.
func (r *ColumnRegistry) IsDefault(name string) bool {
	return r.VisibleColumns(name) == nil
}

// CountVisible returns the number of visible columns and total columns for a dataset.
// Returns (0, 0) if no entry exists.
func (r *ColumnRegistry) CountVisible(name string) (visible, total int) {
	if r == nil {
		return 0, 0
	}
	e, ok := r.entries[name]
	if !ok {
		return 0, 0
	}
	total = len(e.current)
	for _, s := range e.current {
		if s.Visible {
			visible++
		}
	}
	return visible, total
}

// SeedFromSelections initialises the registry entry from a slice of ColumnSelection,
// without requiring db.Column (used in tests).
func (r *ColumnRegistry) SeedFromSelections(name string, sel []ColumnSelection) {
	if r == nil {
		return
	}
	if _, ok := r.entries[name]; ok {
		return
	}
	orig := make([]ColumnSelection, len(sel))
	copy(orig, sel)
	cur := make([]ColumnSelection, len(sel))
	copy(cur, sel)
	r.entries[name] = &registryEntry{original: orig, current: cur}
}

func selectionIsDefault(current, original []ColumnSelection) bool {
	if len(current) != len(original) {
		return false
	}
	for i, s := range current {
		if s.Name != original[i].Name || !s.Visible {
			return false
		}
	}
	return true
}

// --- ColumnPickerModel ---

const colPickerMaxVisible = 16

// ColumnPickerModel is a floating overlay that lets users choose which columns
// are visible and in what order.
type ColumnPickerModel struct {
	original  []ColumnSelection // schema order, used for reset
	current   []ColumnSelection // working copy (may be reordered)
	cursor    int
	errMsg    string
	width     int
	height    int
	confirmed bool
	cancelled bool
}

// NewColumnPickerModel creates a ColumnPickerModel from the current selection.
// original is the schema-order list (for reset); current is the active selection.
func NewColumnPickerModel(original, current []ColumnSelection, w, h int) ColumnPickerModel {
	cur := make([]ColumnSelection, len(current))
	copy(cur, current)
	orig := make([]ColumnSelection, len(original))
	copy(orig, original)
	return ColumnPickerModel{
		original: orig,
		current:  cur,
		width:    w,
		height:   h,
	}
}

// IsConfirmed reports whether the user confirmed the selection.
func (m ColumnPickerModel) IsConfirmed() bool { return m.confirmed }

// IsCancelled reports whether the user cancelled.
func (m ColumnPickerModel) IsCancelled() bool { return m.cancelled }

// Selection returns the confirmed column selection.
func (m ColumnPickerModel) Selection() []ColumnSelection { return m.current }

// HandleKey processes a key string for the column picker (exported for tests).
func (m ColumnPickerModel) HandleKey(key string) ColumnPickerModel {
	return m.handleKey(key)
}

func (m ColumnPickerModel) handleKey(key string) ColumnPickerModel {
	switch key {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
		m.errMsg = ""

	case "down", "j":
		if m.cursor < len(m.current)-1 {
			m.cursor++
		}
		m.errMsg = ""

	case " ":
		if m.cursor < len(m.current) {
			m.current[m.cursor].Visible = !m.current[m.cursor].Visible
		}
		m.errMsg = ""

	case "J":
		// Move focused column down.
		if m.cursor < len(m.current)-1 {
			m.current[m.cursor], m.current[m.cursor+1] = m.current[m.cursor+1], m.current[m.cursor]
			m.cursor++
		}
		m.errMsg = ""

	case "K":
		// Move focused column up.
		if m.cursor > 0 {
			m.current[m.cursor], m.current[m.cursor-1] = m.current[m.cursor-1], m.current[m.cursor]
			m.cursor--
		}
		m.errMsg = ""

	case "a":
		for i := range m.current {
			m.current[i].Visible = true
		}
		m.errMsg = ""

	case "d":
		for i := range m.current {
			m.current[i].Visible = false
		}
		m.errMsg = ""

	case "r":
		orig := make([]ColumnSelection, len(m.original))
		copy(orig, m.original)
		m.current = orig
		m.cursor = 0
		m.errMsg = ""
	}
	return m
}

// TryConfirm attempts to confirm (exported for tests).
func (m ColumnPickerModel) TryConfirm() ColumnPickerModel { return m.tryConfirm() }

// Cancel marks the picker as cancelled (exported for tests).
func (m ColumnPickerModel) Cancel() ColumnPickerModel { return m.cancel() }

// tryConfirm attempts to confirm. If zero columns are visible, sets error.
func (m ColumnPickerModel) tryConfirm() ColumnPickerModel {
	for _, s := range m.current {
		if s.Visible {
			m.confirmed = true
			return m
		}
	}
	m.errMsg = "at least one column required"
	return m
}

// cancel marks the picker as cancelled.
func (m ColumnPickerModel) cancel() ColumnPickerModel {
	m.cancelled = true
	return m
}

// View renders the column picker overlay.
func (m ColumnPickerModel) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	panelW := min(max(m.width-4, 30), 50)
	innerW := panelW - 2

	b := lipgloss.RoundedBorder()
	bs := style.BorderStrokeActive

	// Title line.
	titleText := style.PanelTitleActive.Render(" Columns ")
	titleW := lipgloss.Width(titleText)
	leftDashes := 1
	rightDashes := innerW - titleW - leftDashes
	if rightDashes < 0 {
		rightDashes = 0
	}
	topLine := bs.Render(b.TopLeft+strings.Repeat(b.Top, leftDashes)) +
		titleText +
		bs.Render(strings.Repeat(b.Top, rightDashes)+b.TopRight)

	// Hint line 1.
	hint1 := " Space toggle · J/K reorder"
	hint1 = padToWidth(hint1, innerW)
	hintLine1 := bs.Render(b.Left) + style.Muted.Render(hint1) + bs.Render(b.Right)

	// Hint line 2.
	hint2 := " a all · d none · r reset · ↵ confirm"
	hint2 = padToWidth(hint2, innerW)
	hintLine2 := bs.Render(b.Left) + style.Muted.Render(hint2) + bs.Render(b.Right)

	// Divider.
	divider := bs.Render(b.MiddleLeft + strings.Repeat(b.Top, innerW) + b.MiddleRight)

	// Column rows.
	// Scrolling: show a window of colPickerMaxVisible around the cursor.
	start := 0
	end := len(m.current)
	if end > colPickerMaxVisible {
		start = m.cursor - colPickerMaxVisible/2
		if start < 0 {
			start = 0
		}
		end = start + colPickerMaxVisible
		if end > len(m.current) {
			end = len(m.current)
			start = max(0, end-colPickerMaxVisible)
		}
	}

	var bodyLines []string
	for i := start; i < end; i++ {
		sel := m.current[i]
		check := "[ ]"
		if sel.Visible {
			check = "[✓]"
		}
		nameW := innerW - 4 - 1 // 4 for check+space, 1 for leading space
		if nameW < 1 {
			nameW = 1
		}
		name := padToWidth(sel.Name, nameW)
		if len(name) > nameW {
			name = name[:nameW]
		}
		line := " " + check + " " + name
		line = padToWidth(line, innerW)
		if i == m.cursor {
			line = style.RowSelected.Width(innerW).Render(line)
		}
		bodyLines = append(bodyLines, wrapRow(b, bs, line, innerW))
	}

	// Error line.
	var errLine string
	if m.errMsg != "" {
		errText := padToWidth(" "+m.errMsg, innerW)
		errLine = bs.Render(b.Left) + style.Error.Render(errText) + bs.Render(b.Right)
	}

	bottomLine := bs.Render(b.BottomLeft + strings.Repeat(b.Bottom, innerW) + b.BottomRight)

	parts := make([]string, 0, 6+len(bodyLines))
	parts = append(parts, topLine, hintLine1, hintLine2, divider)
	parts = append(parts, bodyLines...)
	if errLine != "" {
		parts = append(parts, errLine)
	}
	parts = append(parts, bottomLine)
	panel := strings.Join(parts, "\n")

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, panel)
}

// padToWidth pads or truncates s to exactly w display columns.
func padToWidth(s string, w int) string {
	cur := lipgloss.Width(s)
	if cur >= w {
		return s
	}
	return s + strings.Repeat(" ", w-cur)
}
