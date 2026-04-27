package views

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
	"github.com/sahilm/fuzzy"

	"github.com/polesen/datacow/internal/core/config"
	"github.com/polesen/datacow/internal/core/dataset"
	"github.com/polesen/datacow/internal/core/schema"
	"github.com/polesen/datacow/internal/tui/style"
)

const gotoMaxVisible = 12

// GotoSelectedMsg is emitted when the user confirms a selection.
type GotoSelectedMsg struct {
	Dataset    *dataset.Dataset // nil when navigating to a datasource
	Datasource string           // non-empty when switching datasource
}

// GotoModel is the floating fuzzy-search dialog opened with ctrl+p.
type GotoModel struct {
	cache        *schema.Cache
	datasources  []config.DatasourceConfig
	input        textinput.Model
	results      []schema.MatchResult
	cursor       int
	scrollOffset int
	width        int
	height       int
}

// NewGotoModel creates a GotoModel backed by the given cache.
// datasources is the list of configured connections (may be nil for single-datasource mode).
func NewGotoModel(cache *schema.Cache, datasources []config.DatasourceConfig) GotoModel {
	ti := textinput.New()
	ti.Placeholder = "search tables, views, datasets, columns…"
	ti.Prompt = "> "
	ti.CharLimit = 100
	return GotoModel{
		cache:       cache,
		datasources: datasources,
		input:       ti,
	}
}

// Focus focuses the text input and resets the dialog state.
// Returns a tea.Cmd for the cursor blink tick.
func (m *GotoModel) Focus() tea.Cmd {
	m.input.SetValue("")
	m.cursor = 0
	m.scrollOffset = 0
	m.refreshResults()
	return m.input.Focus()
}

// Update implements tea.Model for GotoModel.
func (m GotoModel) Update(msg tea.Msg) (GotoModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		panelW := m.panelW()
		m.input.Width = panelW - 2 - 2 // innerW minus prompt "> "
		return m, nil

	case tea.KeyMsg:
		switch {
		case msg.Type == tea.KeyUp || (msg.Type == tea.KeyRunes && msg.String() == "k"):
			if m.cursor > 0 {
				m.cursor--
				m.ensureCursorVisible()
			}
			return m, nil

		case msg.Type == tea.KeyDown || (msg.Type == tea.KeyRunes && msg.String() == "j"):
			if m.cursor < len(m.results)-1 {
				m.cursor++
				m.ensureCursorVisible()
			}
			return m, nil

		case msg.Type == tea.KeyEnter:
			if m.cursor < len(m.results) {
				entry := m.results[m.cursor].Entry
				var out GotoSelectedMsg
				if entry.Kind == schema.EntryKindDatasource {
					out = GotoSelectedMsg{Datasource: entry.DSName}
				} else {
					out = GotoSelectedMsg{Dataset: entry.Dataset}
				}
				return m, func() tea.Msg { return out }
			}
			return m, nil

		default:
			var tiCmd tea.Cmd
			m.input, tiCmd = m.input.Update(msg)
			m.refreshResults()
			m.cursor = 0
			m.scrollOffset = 0
			return m, tiCmd
		}
	}
	return m, nil
}

// View renders the floating goto dialog centered in the terminal.
func (m GotoModel) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	panelW := m.panelW()
	innerW := panelW - 2

	b := lipgloss.RoundedBorder()
	bs := style.BorderStrokeActive

	// Top border with "Goto" title embedded.
	titleText := style.PanelTitleActive.Render(" Goto ")
	titleW := lipgloss.Width(titleText)
	leftDashes := 1
	rightDashes := innerW - titleW - leftDashes
	if rightDashes < 0 {
		rightDashes = 0
	}
	topLine := bs.Render(b.TopLeft+strings.Repeat(b.Top, leftDashes)) +
		titleText +
		bs.Render(strings.Repeat(b.Top, rightDashes)+b.TopRight)

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
	if m.cache == nil || !m.cache.Ready() {
		bodyLines = append(bodyLines, wrapRow(b, bs, style.Muted.Render("  Loading schema…"), innerW))
	} else if len(m.results) == 0 {
		if m.input.Value() == "" {
			bodyLines = append(bodyLines, wrapRow(b, bs, style.Muted.Render("  (no entries)"), innerW))
		} else {
			bodyLines = append(bodyLines, wrapRow(b, bs, style.Muted.Render("  (no matches)"), innerW))
		}
	} else {
		end := m.scrollOffset + gotoMaxVisible
		if end > len(m.results) {
			end = len(m.results)
		}
		for i := m.scrollOffset; i < end; i++ {
			row := m.renderRow(m.results[i], i == m.cursor, innerW)
			bodyLines = append(bodyLines, wrapRow(b, bs, row, innerW))
		}
	}

	bottomLine := bs.Render(b.BottomLeft + strings.Repeat(b.Bottom, innerW) + b.BottomRight)

	// Assemble panel.
	parts := make([]string, 0, 3+len(bodyLines)+1)
	parts = append(parts, topLine, inputLine, divider)
	parts = append(parts, bodyLines...)
	parts = append(parts, bottomLine)
	panel := strings.Join(parts, "\n")

	// Footer hint below panel.
	footer := style.Muted.Render("esc close  ↵ goto  ↑↓ select")

	panelAndFooter := lipgloss.JoinVertical(lipgloss.Center, panel, footer)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, panelAndFooter)
}

// panelW returns the dialog panel width clamped to terminal width.
func (m GotoModel) panelW() int {
	return min(max(m.width-4, 20), 72)
}

// wrapRow wraps a row string in border characters and pads to innerW.
func wrapRow(b lipgloss.Border, bs lipgloss.Style, row string, innerW int) string {
	dispW := lipgloss.Width(row)
	pad := innerW - dispW
	if pad < 0 {
		pad = 0
	}
	return bs.Render(b.Left) + row + strings.Repeat(" ", pad) + bs.Render(b.Right)
}

// renderRow renders a single result row without border characters.
func (m GotoModel) renderRow(r schema.MatchResult, selected bool, innerW int) string {
	badge := kindGotoBadge(r.Entry.Kind)
	badgeW := len(badge) // all badges are ASCII

	const prefix = 2 // leading spaces
	const gap = 2    // space between name and badge
	nameW := innerW - prefix - gap - badgeW
	if nameW < 1 {
		nameW = 1
	}

	if selected {
		plain := runewidth.Truncate(r.Entry.Name, nameW, "…")
		plain = runewidth.FillRight(plain, nameW)
		line := strings.Repeat(" ", prefix) + plain + strings.Repeat(" ", gap) + badge
		return style.RowSelected.Width(innerW).Render(line)
	}

	var namePart string
	if len(r.MatchedIndexes) > 0 {
		namePart = highlightMatchedRunes(r.Entry.Name, r.MatchedIndexes, nameW)
	} else {
		plain := runewidth.Truncate(r.Entry.Name, nameW, "…")
		namePart = runewidth.FillRight(plain, nameW)
	}
	return strings.Repeat(" ", prefix) + namePart + style.Muted.Render(strings.Repeat(" ", gap)+badge)
}

// highlightMatchedRunes renders the name with matched characters highlighted.
// Returns a string that is exactly maxW display characters wide.
// indexes must be sorted ascending (fuzzy library guarantees this).
func highlightMatchedRunes(name string, indexes []int, maxW int) string {
	runes := []rune(name)
	if len(runes) > maxW {
		runes = runes[:maxW]
	}

	var sb strings.Builder
	j := 0 // pointer into indexes
	for i, r := range runes {
		if j < len(indexes) && indexes[j] == i {
			sb.WriteString(style.GotoMatch.Render(string(r)))
			j++
		} else {
			sb.WriteRune(r)
		}
	}

	// Pad to maxW; runes was already truncated to maxW so len(runes) == display width.
	if pad := maxW - len(runes); pad > 0 {
		sb.WriteString(strings.Repeat(" ", pad))
	}
	return sb.String()
}

// kindGotoBadge returns the right-aligned badge text for an entry kind.
func kindGotoBadge(k schema.EntryKind) string {
	switch k {
	case schema.EntryKindView:
		return "[view]"
	case schema.EntryKindDataset:
		return "[dataset]"
	case schema.EntryKindColumn:
		return "[column]"
	case schema.EntryKindDatasource:
		return "[datasource]"
	default:
		return "[table]"
	}
}

// refreshResults rebuilds m.results from the current cache state and input query.
// Must be called on a pointer receiver.
func (m *GotoModel) refreshResults() {
	query := m.input.Value()

	if m.cache == nil || !m.cache.Ready() {
		m.results = nil
		return
	}

	cacheResults := m.cache.Search(query)

	if query == "" {
		results := make([]schema.MatchResult, 0, len(m.datasources)+len(cacheResults))
		for _, ds := range m.datasources {
			results = append(results, schema.MatchResult{
				Entry: schema.SearchEntry{
					Kind:   schema.EntryKindDatasource,
					Name:   ds.Name,
					DSName: ds.Name,
				},
			})
		}
		results = append(results, cacheResults...)
		m.results = results
		return
	}

	// Fuzzy-match datasource names and prepend scored results.
	dsNames := make([]string, len(m.datasources))
	for i, ds := range m.datasources {
		dsNames[i] = ds.Name
	}
	dsMatches := fuzzy.Find(query, dsNames)

	results := make([]schema.MatchResult, 0, len(dsMatches)+len(cacheResults))
	for _, dm := range dsMatches {
		ds := m.datasources[dm.Index]
		results = append(results, schema.MatchResult{
			Entry: schema.SearchEntry{
				Kind:   schema.EntryKindDatasource,
				Name:   ds.Name,
				DSName: ds.Name,
			},
			MatchedIndexes: dm.MatchedIndexes,
		})
	}
	results = append(results, cacheResults...)
	m.results = results

	// Clamp cursor to valid range.
	if m.cursor >= len(m.results) && len(m.results) > 0 {
		m.cursor = len(m.results) - 1
	}
}

// ensureCursorVisible adjusts scrollOffset so the cursor row is visible.
func (m *GotoModel) ensureCursorVisible() {
	if m.cursor < m.scrollOffset {
		m.scrollOffset = m.cursor
	} else if m.cursor >= m.scrollOffset+gotoMaxVisible {
		m.scrollOffset = m.cursor - gotoMaxVisible + 1
	}
}
