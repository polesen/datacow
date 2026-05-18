package views

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"

	"github.com/polesen/datacow/internal/core/schema"
	"github.com/polesen/datacow/internal/tui/style"
)

const refByMaxVisible = 12

// RefByPickerModel is a floating overlay picker listing tables that reference a cell's column.
type RefByPickerModel struct {
	allEntries []schema.InboundFK
	filtered   []schema.InboundFK
	input      textinput.Model
	cursor     int
	srcTable   string
	srcCol     string
	cellValue  string
	width      int
	height     int
	selected   *schema.InboundFK
	cancelled  bool
}

// NewRefByPickerModel creates a RefByPickerModel with entries sorted alpha by FromTable, then FromColumn.
// srcTable/srcCol are the table and column being referenced; cellValue is the formatted cell value.
func NewRefByPickerModel(entries []schema.InboundFK, srcTable, srcCol, cellValue string) RefByPickerModel {
	sorted := make([]schema.InboundFK, len(entries))
	copy(sorted, entries)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].FromTable != sorted[j].FromTable {
			return sorted[i].FromTable < sorted[j].FromTable
		}
		return sorted[i].FromColumn < sorted[j].FromColumn
	})

	ti := textinput.New()
	ti.Placeholder = "filter…"
	ti.Prompt = "> "
	ti.CharLimit = 100

	m := RefByPickerModel{
		allEntries: sorted,
		srcTable:   srcTable,
		srcCol:     srcCol,
		cellValue:  cellValue,
		input:      ti,
	}
	m.filtered = sorted
	return m
}

// Focus focuses the text input. Returns a tea.Cmd for the cursor blink tick.
func (m RefByPickerModel) Focus() (RefByPickerModel, tea.Cmd) {
	m.input.SetValue("")
	m.cursor = 0
	m.filtered = m.allEntries
	m.selected = nil
	m.cancelled = false
	cmd := m.input.Focus()
	return m, cmd
}

// IsSelected reports whether the user confirmed a selection.
func (m RefByPickerModel) IsSelected() bool { return m.selected != nil }

// Selection returns the selected InboundFK. Only valid when IsSelected() is true.
func (m RefByPickerModel) Selection() schema.InboundFK { return *m.selected }

// IsCancelled reports whether the user pressed Esc to close the picker.
func (m RefByPickerModel) IsCancelled() bool { return m.cancelled }

func (m RefByPickerModel) applyFilter(q string) []schema.InboundFK {
	if q == "" {
		return m.allEntries
	}
	lower := strings.ToLower(q)
	var result []schema.InboundFK
	for _, e := range m.allEntries {
		entry := e.FromTable + "." + e.FromColumn
		if strings.Contains(strings.ToLower(entry), lower) {
			result = append(result, e)
		}
	}
	return result
}

// Update implements tea.Model for RefByPickerModel.
func (m RefByPickerModel) Update(msg tea.Msg) (RefByPickerModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		panelW := m.panelW()
		m.input.Width = panelW - 6 // subtract border (2) + prompt "> " (2) + padding (2)
		return m, nil

	case tea.KeyMsg:
		switch {
		case msg.Type == tea.KeyUp || (msg.Type == tea.KeyRunes && msg.String() == "k"):
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil

		case msg.Type == tea.KeyDown || (msg.Type == tea.KeyRunes && msg.String() == "j"):
			if m.cursor < len(m.filtered)-1 {
				m.cursor++
			}
			return m, nil

		case msg.Type == tea.KeyEnter:
			if m.cursor < len(m.filtered) {
				sel := m.filtered[m.cursor]
				m.selected = &sel
			}
			return m, nil

		case msg.Type == tea.KeyEsc:
			m.cancelled = true
			return m, nil

		default:
			var tiCmd tea.Cmd
			m.input, tiCmd = m.input.Update(msg)
			q := m.input.Value()
			m.filtered = m.applyFilter(q)
			m.cursor = 0
			return m, tiCmd
		}
	}

	var tiCmd tea.Cmd
	m.input, tiCmd = m.input.Update(msg)
	return m, tiCmd
}

// View renders the floating picker centered within its allocated dimensions.
func (m RefByPickerModel) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	panelW := m.panelW()
	innerW := panelW - 2

	b := lipgloss.RoundedBorder()
	bs := style.BorderStrokeActive

	// Title line.
	titleText := style.PanelTitleActive.Render(" Referenced by ")
	titleW := lipgloss.Width(titleText)
	leftDashes := 1
	rightDashes := innerW - titleW - leftDashes
	if rightDashes < 0 {
		rightDashes = 0
	}
	topLine := bs.Render(b.TopLeft+strings.Repeat(b.Top, leftDashes)) +
		titleText +
		bs.Render(strings.Repeat(b.Top, rightDashes)+b.TopRight)

	// Source info line.
	srcText := fmt.Sprintf(" %s.%s = %s", m.srcTable, m.srcCol, m.cellValue)
	srcDisp := runewidth.Truncate(srcText, innerW, "…")
	srcPad := innerW - runewidth.StringWidth(srcDisp)
	if srcPad < 0 {
		srcPad = 0
	}
	srcLine := bs.Render(b.Left) + style.Muted.Render(srcDisp) + strings.Repeat(" ", srcPad) + bs.Render(b.Right)

	// Input line.
	inputView := m.input.View()
	inputDispW := lipgloss.Width(inputView)
	inputPad := innerW - inputDispW
	if inputPad < 0 {
		inputPad = 0
	}
	inputLine := bs.Render(b.Left) + inputView + strings.Repeat(" ", inputPad) + bs.Render(b.Right)

	// Divider.
	divider := bs.Render(b.MiddleLeft + strings.Repeat(b.Top, innerW) + b.MiddleRight)

	// Result rows.
	var bodyLines []string
	if len(m.filtered) == 0 {
		bodyLines = append(bodyLines, wrapRow(b, bs, style.Muted.Render("  No matches"), innerW))
	} else {
		end := refByMaxVisible
		if end > len(m.filtered) {
			end = len(m.filtered)
		}
		for i := 0; i < end; i++ {
			row := m.renderRow(m.filtered[i], i == m.cursor, innerW)
			bodyLines = append(bodyLines, wrapRow(b, bs, row, innerW))
		}
	}

	bottomLine := bs.Render(b.BottomLeft + strings.Repeat(b.Bottom, innerW) + b.BottomRight)

	footer := style.Muted.Render("esc close  ↵ select  ↑↓ move")

	parts := make([]string, 0, 4+len(bodyLines)+1)
	parts = append(parts, topLine, srcLine, inputLine, divider)
	parts = append(parts, bodyLines...)
	parts = append(parts, bottomLine)
	panel := strings.Join(parts, "\n")

	panelAndFooter := lipgloss.JoinVertical(lipgloss.Center, panel, footer)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, panelAndFooter)
}

// panelW returns the dialog panel width clamped to terminal width.
func (m RefByPickerModel) panelW() int {
	return min(max(m.width-4, 20), 60)
}

// renderRow renders one picker entry without border characters.
func (m RefByPickerModel) renderRow(fk schema.InboundFK, selected bool, innerW int) string {
	entry := fk.FromTable + "." + fk.FromColumn
	const prefix = 2
	nameW := innerW - prefix
	if nameW < 1 {
		nameW = 1
	}
	plain := runewidth.Truncate(entry, nameW, "…")
	plain = runewidth.FillRight(plain, nameW)
	line := strings.Repeat(" ", prefix) + plain
	if selected {
		return style.RowSelected.Width(innerW).Render(line)
	}
	return line
}
