package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/polesen/datacow/internal/core/dataset"
	"github.com/polesen/datacow/internal/tui/style"
)

// SortConfirmedMsg is emitted when the user confirms the sort selection.
type SortConfirmedMsg struct {
	Sort []dataset.Sort
}

// SortManagerModel is a floating overlay for building a multi-column sort order.
type SortManagerModel struct {
	active    []dataset.Sort // current sort levels in priority order
	available []string       // column names not yet in active, in schema order
	cursor    int            // position: 0..len(active)-1 = active, len(active).. = available
	width     int
	height    int
	confirmed bool
	cancelled bool
}

// NewSortManagerModel creates a SortManagerModel pre-populated with the current
// sort state and the full schema column list.
func NewSortManagerModel(current []dataset.Sort, columns []string, w, h int) SortManagerModel {
	activeSet := make(map[string]bool, len(current))
	for _, s := range current {
		activeSet[s.Column] = true
	}
	active := make([]dataset.Sort, len(current))
	copy(active, current)

	var avail []string
	for _, c := range columns {
		if !activeSet[c] {
			avail = append(avail, c)
		}
	}

	return SortManagerModel{
		active:    active,
		available: avail,
		cursor:    0,
		width:     w,
		height:    h,
	}
}

// IsConfirmed reports whether the user pressed Enter.
func (m SortManagerModel) IsConfirmed() bool { return m.confirmed }

// IsCancelled reports whether the user pressed Esc.
func (m SortManagerModel) IsCancelled() bool { return m.cancelled }

// Result returns the confirmed sort order.
func (m SortManagerModel) Result() []dataset.Sort { return m.active }

// totalItems returns the total number of navigable items (active + available).
func (m SortManagerModel) totalItems() int {
	return len(m.active) + len(m.available)
}

// cursorInActive reports whether the cursor is on an active entry.
func (m SortManagerModel) cursorInActive() bool {
	return m.cursor < len(m.active)
}

// HandleKey processes a key string for the sort manager (exported for tests).
func (m SortManagerModel) HandleKey(k string) (SortManagerModel, SortConfirmedMsg, bool) {
	return m.handleKey(k)
}

func (m SortManagerModel) handleKey(k string) (SortManagerModel, SortConfirmedMsg, bool) {
	switch k {
	case "up":
		if m.cursor > 0 {
			m.cursor--
		}

	case "down":
		if m.cursor < m.totalItems()-1 {
			m.cursor++
		}

	case " ":
		if m.cursorInActive() {
			// Toggle direction on active entry.
			m.active[m.cursor].Desc = !m.active[m.cursor].Desc
		} else {
			// Add available column to active.
			availIdx := m.cursor - len(m.active)
			if availIdx < len(m.available) {
				col := m.available[availIdx]
				m.available = append(m.available[:availIdx], m.available[availIdx+1:]...)
				m.active = append(m.active, dataset.Sort{Column: col, Desc: false})
				// Move cursor to the newly added active entry.
				m.cursor = len(m.active) - 1
			}
		}

	case "J":
		// Move active entry down (lower priority).
		if m.cursorInActive() && m.cursor < len(m.active)-1 {
			m.active[m.cursor], m.active[m.cursor+1] = m.active[m.cursor+1], m.active[m.cursor]
			m.cursor++
		}

	case "K":
		// Move active entry up (higher priority).
		if m.cursorInActive() && m.cursor > 0 {
			m.active[m.cursor], m.active[m.cursor-1] = m.active[m.cursor-1], m.active[m.cursor]
			m.cursor--
		}

	case "delete", "d":
		if m.cursorInActive() {
			removed := m.active[m.cursor]
			m.active = append(m.active[:m.cursor], m.active[m.cursor+1:]...)
			// Re-insert into available in schema order by appending.
			m.available = append(m.available, removed.Column)
			// Adjust cursor.
			if m.cursor >= len(m.active) && m.cursor > 0 {
				m.cursor--
			}
		}

	case "enter":
		m.confirmed = true
		return m, SortConfirmedMsg{Sort: m.active}, true

	case "esc":
		m.cancelled = true
	}
	return m, SortConfirmedMsg{}, false
}

// View renders the sort manager overlay centered in the terminal.
func (m SortManagerModel) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	panelW := min(max(m.width-4, 30), 56)
	innerW := panelW - 2

	b := lipgloss.RoundedBorder()
	bs := style.BorderStrokeActive

	// Title line.
	titleText := style.PanelTitleActive.Render(" Sort ")
	titleW := lipgloss.Width(titleText)
	leftDashes := 1
	rightDashes := innerW - titleW - leftDashes
	if rightDashes < 0 {
		rightDashes = 0
	}
	topLine := bs.Render(b.TopLeft+strings.Repeat(b.Top, leftDashes)) +
		titleText +
		bs.Render(strings.Repeat(b.Top, rightDashes)+b.TopRight)

	// Hint lines depend on whether there are active sorts.
	var hint1, hint2 string
	if len(m.active) > 0 {
		hint1 = " J/K reorder · Space dir · Del remove"
		hint2 = " Enter confirm · Esc cancel"
	} else {
		hint1 = " Space add · Enter confirm · Esc cancel"
		hint2 = ""
	}
	hint1 = padToWidth(hint1, innerW)
	hintLine1 := bs.Render(b.Left) + style.Muted.Render(hint1) + bs.Render(b.Right)

	var hintLine2 string
	if hint2 != "" {
		hint2 = padToWidth(hint2, innerW)
		hintLine2 = bs.Render(b.Left) + style.Muted.Render(hint2) + bs.Render(b.Right)
	}

	divider := bs.Render(b.MiddleLeft + strings.Repeat(b.Top, innerW) + b.MiddleRight)

	var bodyLines []string

	// Active section.
	if len(m.active) > 0 {
		header := padToWidth(" Active", innerW)
		bodyLines = append(bodyLines, wrapRow(b, bs, style.ColHeader.Render(header), innerW))
		for i, s := range m.active {
			arrow := "↑"
			if s.Desc {
				arrow = "↓"
			}
			label := fmt.Sprintf(" %d. %-20s %s", i+1, s.Column, arrow)
			label = padToWidth(label, innerW)
			if i == m.cursor {
				label = style.RowSelected.Width(innerW).Render(label)
			}
			bodyLines = append(bodyLines, wrapRow(b, bs, label, innerW))
		}

		// Divider between active and available.
		sepLine := strings.Repeat("─", innerW)
		bodyLines = append(bodyLines, wrapRow(b, bs, style.Muted.Render(sepLine), innerW))
	}

	// Available section.
	availHeader := padToWidth(" Available", innerW)
	bodyLines = append(bodyLines, wrapRow(b, bs, style.ColHeader.Render(availHeader), innerW))
	for i, col := range m.available {
		label := padToWidth(" "+col, innerW)
		cur := len(m.active) + i
		if cur == m.cursor {
			label = style.RowSelected.Width(innerW).Render(label)
		}
		bodyLines = append(bodyLines, wrapRow(b, bs, label, innerW))
	}

	bottomLine := bs.Render(b.BottomLeft + strings.Repeat(b.Bottom, innerW) + b.BottomRight)

	parts := []string{topLine, hintLine1}
	if hintLine2 != "" {
		parts = append(parts, hintLine2)
	}
	parts = append(parts, divider)
	parts = append(parts, bodyLines...)
	parts = append(parts, bottomLine)

	panel := strings.Join(parts, "\n")
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, panel)
}
