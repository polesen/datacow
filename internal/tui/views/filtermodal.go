package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"

	"github.com/polesen/datacow/internal/core/dataset"
	"github.com/polesen/datacow/internal/core/db"
	"github.com/polesen/datacow/internal/tui/style"
)

type filterFormField int

const (
	fieldColumn filterFormField = iota
	fieldOp
	fieldValue
)

// FilterModalModel is the query filter modal dialog.
// It manages a working copy of the filter list, editable before applying.
type FilterModalModel struct {
	// context (set on open, read-only)
	ds      dataset.Dataset
	columns []db.Column

	// working filter list (pending, not yet applied to row browser)
	filters    []dataset.Filter
	listCursor int // -1 = no selection; index into filters
	editingIdx int // -1 = adding new; >= 0 = replacing filter at that index

	// form state
	activeField filterFormField
	columnInput textinput.Model
	colDropdown []string // column names matching current column input text
	dropIdx     int      // -1 = none highlighted in dropdown; else index into colDropdown
	opIdx       int      // index into allowedOps(typeCat)
	valueInput  textinput.Model
	boolValue   bool // for boolean columns

	// column context (updated whenever columnInput changes)
	selColIdx int          // index in m.columns; -1 if not matched
	typeCat   typeCategory

	// feedback
	valErr string

	// result flags (consumed once by caller)
	applied   bool
	cancelled bool

	width int
}

// NewFilterModal creates a FilterModalModel pre-populated with the given filters.
func NewFilterModal(
	ds dataset.Dataset,
	columns []db.Column,
	filters []dataset.Filter,
) FilterModalModel {
	col := textinput.New()
	col.Placeholder = "column name"
	col.Width = 16

	val := textinput.New()
	val.Placeholder = "value"
	val.Width = 30

	m := FilterModalModel{
		ds:      ds,
		columns: columns,
		filters: make([]dataset.Filter, len(filters)),
		listCursor:    -1,
		editingIdx:    -1,
		activeField:   fieldColumn,
		columnInput:   col,
		dropIdx:       -1,
		selColIdx:     -1,
		typeCat:       typeCatText,
		valueInput:    val,
	}
	copy(m.filters, filters)
	if len(m.filters) > 0 {
		m.listCursor = 0
	}
	m.colDropdown = m.computeDropdown("")
	m.columnInput.Focus()
	return m
}

// NewFilterModalQuickFilter creates a modal with the Column and Value form fields
// pre-filled for a quick filter from the selected cell.
func NewFilterModalQuickFilter(
	ds dataset.Dataset,
	columns []db.Column,
	filters []dataset.Filter,
	colName, value string,
) FilterModalModel {
	m := NewFilterModal(ds, columns, filters)
	m.columnInput.SetValue(colName)
	m.selColIdx = m.findColumnIdx(colName)
	if m.selColIdx >= 0 {
		m.typeCat = resolveTypeCategory(m.columns[m.selColIdx].Type)
	}
	m.opIdx = 0
	m.colDropdown = m.computeDropdown(colName)
	m.dropIdx = -1
	m.valueInput.SetValue(value)
	m.activeField = fieldValue
	m.columnInput.Blur()
	m.valueInput.Focus()
	return m
}

// SetWidth sets the modal's render width (used by tests and the row browser on resize).
func (m *FilterModalModel) SetWidth(w int) {
	m.width = w
	// keep value input sized to fill the remaining row: 4(prompt)+16(col)+2+8(op)+2 = 32
	if valW := w - 4 - 32 - 4; valW > 10 {
		m.valueInput.Width = valW
	}
}

// IsApplied returns true if the user confirmed the filter changes (Ctrl+Enter).
func (m FilterModalModel) IsApplied() bool { return m.applied }

// IsCancelled returns true if the user dismissed the modal without applying (Esc).
func (m FilterModalModel) IsCancelled() bool { return m.cancelled }

// Filters returns the working filter list (to be used when applied).
func (m FilterModalModel) Filters() []dataset.Filter {
	result := make([]dataset.Filter, len(m.filters))
	copy(result, m.filters)
	return result
}

// Update processes messages for the filter modal.
func (m FilterModalModel) Update(msg tea.Msg) (FilterModalModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(msg)
	case tea.WindowSizeMsg:
		m.width = msg.Width
		return m, nil
	default:
		// Forward to focused textinput for cursor blink etc.
		var cmd tea.Cmd
		switch m.activeField {
		case fieldColumn:
			m.columnInput, cmd = m.columnInput.Update(msg)
		case fieldValue:
			m.valueInput, cmd = m.valueInput.Update(msg)
		}
		return m, cmd
	}
}

func (m FilterModalModel) handleKey(msg tea.KeyMsg) (FilterModalModel, tea.Cmd) {
	// Global keys: apply (ctrl+enter / ctrl+j) and cancel (esc)
	switch {
	case msg.Type == tea.KeyCtrlJ || msg.String() == "ctrl+enter":
		m.applied = true
		return m, nil
	case msg.Type == tea.KeyEsc:
		m.cancelled = true
		return m, nil
	case msg.Type == tea.KeyTab:
		return m.nextField(), nil
	case msg.Type == tea.KeyShiftTab:
		return m.prevField(), nil
	}

	// Route by active field
	switch m.activeField {
	case fieldColumn:
		return m.handleColumnKey(msg)
	case fieldOp:
		return m.handleOpKey(msg)
	case fieldValue:
		return m.handleValueKey(msg)
	}
	return m, nil
}

func (m FilterModalModel) handleColumnKey(msg tea.KeyMsg) (FilterModalModel, tea.Cmd) {
	switch {
	case msg.Type == tea.KeyDown:
		if len(m.colDropdown) > 0 {
			if m.dropIdx < len(m.colDropdown)-1 {
				m.dropIdx++
			}
			return m, nil
		}
		// Navigate filter list
		if m.listCursor < len(m.filters)-1 {
			m.listCursor++
		}
		return m, nil

	case msg.Type == tea.KeyUp:
		if len(m.colDropdown) > 0 {
			if m.dropIdx > 0 {
				m.dropIdx--
			}
			return m, nil
		}
		// Navigate filter list
		if m.listCursor > 0 {
			m.listCursor--
		} else if m.listCursor == -1 && len(m.filters) > 0 {
			m.listCursor = len(m.filters) - 1
		}
		return m, nil

	case msg.Type == tea.KeyEnter:
		if m.dropIdx >= 0 && m.dropIdx < len(m.colDropdown) {
			return m.acceptDropdownSelection(), nil
		}
		// Column empty: Enter on a highlighted filter edits it; with no selection, apply.
		if m.columnInput.Value() == "" {
			if m.listCursor >= 0 && m.listCursor < len(m.filters) {
				return m.loadSelectedFilter(), nil
			}
			m.applied = true
			return m, nil
		}
		return m.nextField(), nil

	case msg.String() == "j":
		// j always navigates filter list
		if m.listCursor < len(m.filters)-1 {
			m.listCursor++
		}
		return m, nil

	case msg.String() == "k":
		// k always navigates filter list
		if m.listCursor > 0 {
			m.listCursor--
		}
		return m, nil

	case msg.String() == "d" || msg.Type == tea.KeyDelete || msg.Type == tea.KeyBackspace:
		// d deletes selected filter — but only when column input is empty
		if m.columnInput.Value() == "" {
			return m.deleteSelectedFilter(), nil
		}
		// Otherwise let the textinput handle backspace
		var cmd tea.Cmd
		m.columnInput, cmd = m.columnInput.Update(msg)
		m = m.updateDropdown()
		return m, cmd
	}

	// Forward to column textinput and recompute dropdown
	var cmd tea.Cmd
	m.columnInput, cmd = m.columnInput.Update(msg)
	m = m.updateDropdown()
	return m, cmd
}

func (m FilterModalModel) handleOpKey(msg tea.KeyMsg) (FilterModalModel, tea.Cmd) {
	ops := allowedOps(m.typeCat)
	switch {
	case msg.Type == tea.KeyLeft || msg.String() == " ":
		if m.opIdx > 0 {
			m.opIdx--
		} else {
			m.opIdx = len(ops) - 1
		}
	case msg.Type == tea.KeyRight:
		m.opIdx = (m.opIdx + 1) % len(ops)
	case msg.Type == tea.KeyDown:
		if m.listCursor < len(m.filters)-1 {
			m.listCursor++
		}
	case msg.Type == tea.KeyUp:
		if m.listCursor > 0 {
			m.listCursor--
		}
	case msg.String() == "j":
		if m.listCursor < len(m.filters)-1 {
			m.listCursor++
		}
	case msg.String() == "k":
		if m.listCursor > 0 {
			m.listCursor--
		}
	case msg.String() == "d":
		return m.deleteSelectedFilter(), nil
	case msg.Type == tea.KeyEnter:
		if m.listCursor >= 0 && m.listCursor < len(m.filters) {
			return m.loadSelectedFilter(), nil
		}
	}
	return m, nil
}

func (m FilterModalModel) handleValueKey(msg tea.KeyMsg) (FilterModalModel, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		return m.submitForm(), nil

	case tea.KeyDown:
		if m.listCursor < len(m.filters)-1 {
			m.listCursor++
		}
		return m, nil

	case tea.KeyUp:
		if m.listCursor > 0 {
			m.listCursor--
		}
		return m, nil
	}

	// Type-aware input filtering
	if msg.Type == tea.KeyRunes && len(msg.Runes) == 1 {
		r := msg.Runes[0]
		if m.typeCat == typeCatBoolean {
			// Toggle true/false on any keystroke
			m.boolValue = !m.boolValue
			return m, nil
		}
		if !isValidValueRune(m.typeCat, m.valueInput.Value(), r) {
			return m, nil // silently drop disallowed rune
		}
	}

	var cmd tea.Cmd
	m.valueInput, cmd = m.valueInput.Update(msg)
	return m, cmd
}

// nextField advances to the next form field (Column → Op → Value → Column).
func (m FilterModalModel) nextField() FilterModalModel {
	switch m.activeField {
	case fieldColumn:
		// Accept dropdown selection if one is highlighted
		if m.dropIdx >= 0 && m.dropIdx < len(m.colDropdown) {
			m = m.acceptDropdownSelection()
		}
		// Don't advance if column name is not in the schema
		if m.selColIdx < 0 {
			if m.columnInput.Value() != "" {
				m.valErr = fmt.Sprintf("unknown column %q — pick from the list", m.columnInput.Value())
			}
			return m
		}
		m.valErr = ""
		m.activeField = fieldOp
		m.columnInput.Blur()

	case fieldOp:
		m.activeField = fieldValue
		m.valueInput.Focus()

	case fieldValue:
		m.activeField = fieldColumn
		m.valueInput.Blur()
		m.columnInput.Focus()
	}
	return m
}

// prevField moves to the previous form field (Column → Value → Op → Column).
func (m FilterModalModel) prevField() FilterModalModel {
	switch m.activeField {
	case fieldColumn:
		m.activeField = fieldValue
		m.columnInput.Blur()
		m.valueInput.Focus()

	case fieldOp:
		m.activeField = fieldColumn
		m.columnInput.Focus()

	case fieldValue:
		m.activeField = fieldOp
		m.valueInput.Blur()
	}
	return m
}

// submitForm validates and adds/replaces a filter from the current form state.
func (m FilterModalModel) submitForm() FilterModalModel {
	if m.selColIdx < 0 {
		m.valErr = fmt.Sprintf("unknown column %q — pick from the list", m.columnInput.Value())
		return m
	}

	col := m.columns[m.selColIdx]
	ops := allowedOps(m.typeCat)
	op := ops[0]
	if m.opIdx < len(ops) {
		op = ops[m.opIdx]
	}

	var value any
	if m.typeCat == typeCatBoolean {
		if m.boolValue {
			value = "true"
		} else {
			value = "false"
		}
	} else {
		value = m.valueInput.Value()
	}

	f := dataset.Filter{Column: col.Name, Operator: op, Value: value}

	if m.editingIdx >= 0 && m.editingIdx < len(m.filters) {
		m.filters[m.editingIdx] = f
	} else {
		m.filters = append(m.filters, f)
	}
	m.listCursor = -1

	return m.resetForm()
}

// loadSelectedFilter loads the currently selected filter from the list into the form.
func (m FilterModalModel) loadSelectedFilter() FilterModalModel {
	if m.listCursor < 0 || m.listCursor >= len(m.filters) {
		return m
	}
	f := m.filters[m.listCursor]
	m.editingIdx = m.listCursor

	m.columnInput.SetValue(f.Column)
	m.selColIdx = m.findColumnIdx(f.Column)
	if m.selColIdx >= 0 {
		m.typeCat = resolveTypeCategory(m.columns[m.selColIdx].Type)
	}
	m.colDropdown = m.computeDropdown(f.Column)
	m.dropIdx = -1

	// Op
	ops := allowedOps(m.typeCat)
	m.opIdx = 0
	for i, op := range ops {
		if op == f.Operator {
			m.opIdx = i
			break
		}
	}

	// Value
	if m.typeCat == typeCatBoolean {
		m.boolValue = fmt.Sprintf("%v", f.Value) == "true"
	} else {
		m.valueInput.SetValue(fmt.Sprintf("%v", f.Value))
	}

	m.valErr = ""
	m.activeField = fieldColumn
	m.columnInput.Focus()
	m.valueInput.Blur()
	return m
}

// deleteSelectedFilter removes the filter at listCursor.
func (m FilterModalModel) deleteSelectedFilter() FilterModalModel {
	if m.listCursor < 0 || m.listCursor >= len(m.filters) {
		return m
	}
	m.filters = append(m.filters[:m.listCursor], m.filters[m.listCursor+1:]...)
	if m.listCursor >= len(m.filters) && m.listCursor > 0 {
		m.listCursor--
	}
	if len(m.filters) == 0 {
		m.listCursor = -1
	}
	// Reset editing index if we deleted the edited filter
	if m.editingIdx >= len(m.filters) {
		m.editingIdx = -1
	}
	return m
}

// acceptDropdownSelection sets the column field to the highlighted dropdown entry.
func (m FilterModalModel) acceptDropdownSelection() FilterModalModel {
	if m.dropIdx < 0 || m.dropIdx >= len(m.colDropdown) {
		return m
	}
	name := m.colDropdown[m.dropIdx]
	m.columnInput.SetValue(name)
	m.selColIdx = m.findColumnIdx(name)
	if m.selColIdx >= 0 {
		m.typeCat = resolveTypeCategory(m.columns[m.selColIdx].Type)
		// If current op is not valid for new type, reset to first valid op
		ops := allowedOps(m.typeCat)
		valid := false
		for _, op := range ops {
			if m.opIdx < len(allowedOps(m.typeCat)) && allowedOps(m.typeCat)[m.opIdx] == op {
				valid = true
				break
			}
		}
		if !valid {
			m.opIdx = 0
		}
	}
	m.colDropdown = m.computeDropdown(name)
	m.dropIdx = -1
	m.valErr = ""
	return m
}

// resetForm clears the add/edit form back to "add new" state.
func (m FilterModalModel) resetForm() FilterModalModel {
	m.editingIdx = -1
	m.columnInput.SetValue("")
	m.colDropdown = m.computeDropdown("")
	m.dropIdx = -1
	m.opIdx = 0
	m.valueInput.SetValue("")
	m.typeCat = typeCatText
	m.selColIdx = -1
	m.valErr = ""
	m.boolValue = false
	m.activeField = fieldColumn
	m.columnInput.Focus()
	m.valueInput.Blur()
	return m
}

// updateDropdown refreshes the dropdown and column context based on current column input.
func (m FilterModalModel) updateDropdown() FilterModalModel {
	text := m.columnInput.Value()
	m.colDropdown = m.computeDropdown(text)
	m.dropIdx = -1
	m.selColIdx = m.findColumnIdx(text)
	if m.selColIdx >= 0 {
		newCat := resolveTypeCategory(m.columns[m.selColIdx].Type)
		if newCat != m.typeCat {
			m.typeCat = newCat
			// Reset op if no longer valid
			ops := allowedOps(m.typeCat)
			if m.opIdx >= len(ops) {
				m.opIdx = 0
			}
		}
	} else {
		m.typeCat = typeCatText
	}
	m.valErr = ""
	return m
}

// computeDropdown returns column names matching query (substring, case-insensitive).
// When query is empty, all column names are returned for discovery.
func (m FilterModalModel) computeDropdown(query string) []string {
	q := strings.ToLower(query)
	var matches []string
	for _, col := range m.columns {
		if q == "" || strings.Contains(strings.ToLower(col.Name), q) {
			matches = append(matches, col.Name)
		}
	}
	return matches
}

// findColumnIdx returns the index in m.columns for the given name (case-sensitive), or -1.
func (m FilterModalModel) findColumnIdx(name string) int {
	for i, col := range m.columns {
		if col.Name == name {
			return i
		}
	}
	return -1
}

// View renders the filter modal as a centered box.
func (m FilterModalModel) View() string {
	innerW := m.width - 4
	if innerW < 40 {
		innerW = 40
	}
	if innerW > 76 {
		innerW = 76
	}

	var lines []string

	// --- Active filters ---
	lines = append(lines, "")
	lines = append(lines, " Active filters:")
	if len(m.filters) == 0 {
		lines = append(lines, style.Muted.Render("   (none)"))
	} else {
		for i, f := range m.filters {
			cursor := "  "
			if i == m.listCursor {
				cursor = "▸ "
			}
			label := fmt.Sprintf("   %d %s%s", i+1, cursor, formatFilterLabel(f))
			if i == m.listCursor {
				lines = append(lines, style.RowHighlight.Render(label))
			} else {
				lines = append(lines, label)
			}
		}
	}

	// --- Edit / add filter (single inline row) ---
	lines = append(lines, "")
	lines = append(lines, " Edit / add filter:")

	colFocused := m.activeField == fieldColumn
	opFocused := m.activeField == fieldOp
	valFocused := m.activeField == fieldValue

	// Column segment
	var colSeg string
	if colFocused {
		colSeg = m.columnInput.View()
	} else {
		v := m.columnInput.Value()
		if v == "" {
			colSeg = style.Muted.Render(runewidth.FillRight("column…", 16))
		} else {
			colSeg = runewidth.FillRight(runewidth.Truncate(v, 16, "…"), 16)
		}
	}

	// Op segment
	ops := allowedOps(m.typeCat)
	opText := ops[0]
	if m.opIdx < len(ops) {
		opText = ops[m.opIdx]
	}
	var opSeg string
	if opFocused {
		opSeg = style.CursorCell.Render(" " + opText + " ▾ ")
	} else {
		opSeg = opText + " ▾"
	}

	// Value segment
	var valSeg string
	if m.typeCat == typeCatBoolean {
		boolStr := "false"
		if m.boolValue {
			boolStr = "true"
		}
		if valFocused {
			valSeg = style.CursorCell.Render(" " + boolStr + " ")
		} else {
			valSeg = boolStr
		}
	} else if valFocused {
		valSeg = m.valueInput.View()
	} else {
		v := m.valueInput.Value()
		if v == "" {
			valSeg = style.Muted.Render("value…")
		} else {
			valSeg = v
		}
	}

	lines = append(lines, "  > "+colSeg+"  "+opSeg+"  "+valSeg)

	// Dropdown (shown when Column field is focused)
	if colFocused && len(m.colDropdown) > 0 {
		maxShow := 5
		if maxShow > len(m.colDropdown) {
			maxShow = len(m.colDropdown)
		}
		for i := 0; i < maxShow; i++ {
			name := m.colDropdown[i]
			if i == m.dropIdx {
				lines = append(lines, style.CursorCell.Render("    ▶ "+name))
			} else {
				lines = append(lines, style.Muted.Render("      "+name))
			}
		}
	}

	// Validation error
	if m.valErr != "" {
		lines = append(lines, style.Error.Render(" "+m.valErr))
	}

	// --- Help ---
	lines = append(lines, "")
	lines = append(lines, renderModalSep("Help", innerW))

	if m.selColIdx >= 0 {
		col := m.columns[m.selColIdx]
		nullable := "NOT NULL"
		if col.Nullable {
			nullable = "nullable"
		}
		ops := allowedOps(m.typeCat)
		helpLine := fmt.Sprintf(" %s: %s %s · ops: %s", col.Name, col.Type, nullable, strings.Join(ops, " "))
		lines = append(lines, style.Muted.Render(helpLine))
	}
	lines = append(lines, style.Muted.Render(" "+typeTip(m.typeCat)))

	// --- Footer ---
	lines = append(lines, "")
	lines = append(lines, style.Muted.Render(" Tab col→op→val  ←→/Space cycle op  Enter add · Enter again apply"))
	lines = append(lines, style.Muted.Render(" ↑↓/jk select  Enter edit  d delete  Esc cancel"))

	return renderModalBox("Query Filter · "+m.ds.Name, lines, innerW)
}


// renderModalSep renders a horizontal separator with a title.
func renderModalSep(title string, innerW int) string {
	right := strings.Repeat("─", max2(0, innerW-len(title)-4))
	return style.Separator.Render(" ─ " + title + " " + right)
}

// renderModalBox wraps lines in a bordered box with a title on the top edge.
func renderModalBox(title string, lines []string, innerW int) string {
	titleStr := " " + title + " "
	titleW := lipgloss.Width(titleStr)
	dashLen := innerW - titleW - 1
	if dashLen < 0 {
		dashLen = 0
	}

	top := style.BorderStrokeActive.Render("┌─") +
		style.PanelTitleActive.Render(titleStr) +
		style.BorderStrokeActive.Render(strings.Repeat("─", dashLen)+"┐")

	bottom := style.BorderStrokeActive.Render("└" + strings.Repeat("─", innerW+2) + "┘")

	var body []string
	for _, line := range lines {
		w := lipgloss.Width(line)
		pad := innerW - w
		if pad < 0 {
			pad = 0
		}
		body = append(body, style.BorderStrokeActive.Render("│")+line+strings.Repeat(" ", pad)+style.BorderStrokeActive.Render("│"))
	}

	all := make([]string, 0, len(body)+2)
	all = append(all, top)
	all = append(all, body...)
	all = append(all, bottom)
	return strings.Join(all, "\n")
}

func max2(a, b int) int {
	if a > b {
		return a
	}
	return b
}
