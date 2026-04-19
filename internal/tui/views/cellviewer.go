package views

import (
	"encoding/json"
	"os"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"

	"github.com/polesen/datacow/internal/tui/keys"
	"github.com/polesen/datacow/internal/tui/style"
)

// OpenCellViewerMsg is emitted by the row browser to open the full-screen cell viewer.
type OpenCellViewerMsg struct {
	TableName  string
	PKValues   []string // PK values only (no column names), for filename generation
	PKDisplay  string   // "col=val" or "col1=v1, col2=v2", for the header
	ColumnName string
	ColumnType string
	Raw        []byte
}

// CloseCellViewerMsg is emitted by the cell viewer to request it be closed.
type CloseCellViewerMsg struct{}

type cvSaveState int

const (
	cvSaveIdle      cvSaveState = iota
	cvSavePrompting             // filename input is visible
	cvSaveDone                  // confirmation shown after save or copy
	cvSaveError                 // error shown
)

// CellViewerModel is a full-screen overlay that shows the full contents of one cell.
type CellViewerModel struct {
	tableName  string
	pkValues   []string
	pkDisplay  string
	columnName string
	columnType string
	raw        []byte

	displayText  string   // content to show (pretty-printed when JSON)
	displayLines []string // displayText word-wrapped to current contentWidth
	scrollOffset int
	width        int
	height       int

	saveInput textinput.Model
	saveState cvSaveState
	saveMsg   string
	keys      keys.Map
}

// NewCellViewerModel constructs a CellViewerModel from an OpenCellViewerMsg.
func NewCellViewerModel(k keys.Map, msg OpenCellViewerMsg) CellViewerModel {
	ti := textinput.New()
	ti.Prompt = "Save as: "
	ti.CharLimit = 200

	return CellViewerModel{
		tableName:   msg.TableName,
		pkValues:    msg.PKValues,
		pkDisplay:   msg.PKDisplay,
		columnName:  msg.ColumnName,
		columnType:  msg.ColumnType,
		raw:         msg.Raw,
		saveInput:   ti,
		saveState:   cvSaveIdle,
		keys:        k,
		displayText: prepareDisplayText(msg.Raw),
	}
}

// prepareDisplayText formats raw bytes for display: pretty-prints JSON, plain string otherwise.
func prepareDisplayText(raw []byte) string {
	if len(raw) == 0 {
		return "(empty)"
	}
	var v any
	if err := json.Unmarshal(raw, &v); err == nil {
		if b, err := json.MarshalIndent(v, "", "  "); err == nil {
			return string(b)
		}
	}
	return string(raw)
}

// Update implements tea.Model.
func (m CellViewerModel) Update(msg tea.Msg) (CellViewerModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m = m.rebuildLines()
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m CellViewerModel) rebuildLines() CellViewerModel {
	iw := m.innerWidth()
	promptW := runewidth.StringWidth(m.saveInput.Prompt)
	m.saveInput.Width = max(10, iw-promptW-2)
	m.displayLines = wrapText(m.displayText, m.contentWidth())
	maxScroll := max(0, len(m.displayLines)-m.contentLineCount())
	if m.scrollOffset > maxScroll {
		m.scrollOffset = maxScroll
	}
	return m
}

func (m CellViewerModel) handleKey(msg tea.KeyMsg) (CellViewerModel, tea.Cmd) {
	if m.saveState == cvSavePrompting {
		return m.handleSaveInputKey(msg)
	}
	switch {
	case key.Matches(msg, m.keys.Back):
		return m, func() tea.Msg { return CloseCellViewerMsg{} }

	case key.Matches(msg, m.keys.Down):
		lines := m.lines()
		max := len(lines) - m.contentLineCount()
		if max < 0 {
			max = 0
		}
		if m.scrollOffset < max {
			m.scrollOffset++
		}

	case key.Matches(msg, m.keys.Up):
		if m.scrollOffset > 0 {
			m.scrollOffset--
		}

	case msg.String() == "s":
		suggested := suggestFilename(m.tableName, m.pkValues, m.columnName, m.raw)
		m.saveInput.SetValue(suggested)
		m.saveInput.CursorEnd()
		m.saveState = cvSavePrompting
		m.saveMsg = ""
		return m, m.saveInput.Focus()

	case msg.String() == "c":
		if err := clipboard.WriteAll(string(m.raw)); err != nil {
			m.saveMsg = "Copy failed: " + err.Error()
			m.saveState = cvSaveError
		} else {
			m.saveMsg = "Copied to clipboard"
			m.saveState = cvSaveDone
		}
	}
	return m, nil
}

func (m CellViewerModel) handleSaveInputKey(msg tea.KeyMsg) (CellViewerModel, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		filename := strings.TrimSpace(m.saveInput.Value())
		if filename == "" {
			return m, nil
		}
		content := cellSaveContent(m.raw)
		if err := os.WriteFile(filename, content, 0o644); err != nil {
			m.saveMsg = "Save failed: " + err.Error()
			m.saveState = cvSaveError
		} else {
			m.saveMsg = "Saved to " + filename
			m.saveState = cvSaveDone
		}
		m.saveInput.Blur()
		return m, nil

	case tea.KeyEsc:
		m.saveInput.SetValue("")
		m.saveInput.Blur()
		m.saveState = cvSaveIdle
		m.saveMsg = ""
		return m, nil

	default:
		var cmd tea.Cmd
		m.saveInput, cmd = m.saveInput.Update(msg)
		return m, cmd
	}
}

// cellSaveContent returns bytes to write: JSON is pretty-printed, everything else is raw.
func cellSaveContent(raw []byte) []byte {
	var v any
	if err := json.Unmarshal(raw, &v); err == nil {
		if b, err := json.MarshalIndent(v, "", "  "); err == nil {
			return b
		}
	}
	return raw
}

// --- sizing ---

// innerWidth is the usable width inside the border (m.width minus 2 border chars).
func (m CellViewerModel) innerWidth() int { return max(1, m.width-2) }

// innerHeight is the usable height inside the border (m.height minus 2 border chars).
func (m CellViewerModel) innerHeight() int { return max(0, m.height-2) }

// contentWidth is the line width for text display (innerWidth minus 2-char left margin).
func (m CellViewerModel) contentWidth() int { return max(1, m.innerWidth()-2) }

// contentLineCount is the number of scrollable content rows (innerHeight minus title + footer).
func (m CellViewerModel) contentLineCount() int { return max(0, m.innerHeight()-2) }

// HeaderTitle returns the string used as the pane title.
func (m CellViewerModel) HeaderTitle() string {
	if m.pkDisplay != "" {
		return m.tableName + " · " + m.pkDisplay + " · " + m.columnName + " (" + m.columnType + ")"
	}
	return m.tableName + " · " + m.columnName + " (" + m.columnType + ")"
}

// lines returns pre-computed wrapped lines, computing on demand if not yet built.
func (m CellViewerModel) lines() []string {
	if len(m.displayLines) > 0 {
		return m.displayLines
	}
	return wrapText(m.displayText, m.contentWidth())
}

// --- View ---

// View renders a fully self-contained bordered overlay of exactly m.width × m.height cells.
// The app calls this directly — no outer border or title is added by the caller.
func (m CellViewerModel) View() string {
	if m.width == 0 {
		return ""
	}
	iw := m.innerWidth()
	ih := m.innerHeight()
	cw := m.contentWidth()
	contentH := m.contentLineCount()

	// Title bar (1 line inside border).
	title := style.PanelTitleActive.Width(iw).Render(m.HeaderTitle())

	// Content lines.
	lines := m.lines()
	var sb strings.Builder
	for i := range contentH {
		idx := m.scrollOffset + i
		var line string
		if idx < len(lines) {
			line = lines[idx]
		}
		w := runewidth.StringWidth(line)
		if w > cw {
			line = runewidth.Truncate(line, cw, "…")
		} else if w < cw {
			line += strings.Repeat(" ", cw-w)
		}
		sb.WriteString("  ")
		sb.WriteString(line)
		if i < contentH-1 {
			sb.WriteString("\n")
		}
	}
	contentBlock := sb.String()

	// Footer bar (1 line inside border).
	footer := style.FilterBar.Width(iw).Render(m.footerContent())

	// inner = title (1) + content (ih-2) + footer (1) = ih lines total.
	inner := lipgloss.JoinVertical(lipgloss.Left, title, contentBlock, footer)

	// Wrap in border: outer size = (iw+2) × (ih+2) = m.width × m.height.
	return style.PanelActive.Width(iw).Height(ih).Render(inner)
}

func (m CellViewerModel) footerContent() string {
	switch m.saveState {
	case cvSavePrompting:
		return m.saveInput.View()
	case cvSaveDone:
		return style.Progress.Render(m.saveMsg)
	case cvSaveError:
		return style.Error.Render(m.saveMsg)
	default:
		return style.StatusKey.Render("s") + style.StatusDesc.Render(" save") +
			"   " +
			style.StatusKey.Render("c") + style.StatusDesc.Render(" copy") +
			"   " +
			style.StatusKey.Render("↑↓") + style.StatusDesc.Render(" scroll") +
			"   " +
			style.StatusKey.Render("esc") + style.StatusDesc.Render(" close")
	}
}

// --- filename suggestion ---

var cvSanitizeRe = regexp.MustCompile(`[^a-zA-Z0-9_\-.]`)

// sanitizeSegment makes a string safe to use as a filename segment.
func sanitizeSegment(s string) string {
	s = strings.ReplaceAll(s, " ", "-")
	s = cvSanitizeRe.ReplaceAllString(s, "")
	if len(s) > 40 {
		s = s[:40]
	}
	if s == "" {
		s = "_"
	}
	return s
}

// suggestFilename produces a suggested filename like "users-42-bio.json".
func suggestFilename(table string, pkValues []string, column string, raw []byte) string {
	ext := inferExtension(raw)
	parts := []string{sanitizeSegment(table)}
	for _, v := range pkValues {
		parts = append(parts, sanitizeSegment(v))
	}
	parts = append(parts, sanitizeSegment(column))
	return strings.Join(parts, "-") + ext
}

// inferExtension detects the file type from content and returns the appropriate extension.
func inferExtension(data []byte) string {
	if len(data) == 0 {
		return ".txt"
	}

	// JSON
	var v any
	if json.Unmarshal(data, &v) == nil {
		return ".json"
	}

	// XML: trim whitespace and check for '<'
	if strings.HasPrefix(strings.TrimSpace(string(data)), "<") {
		return ".xml"
	}

	// Magic bytes
	switch {
	case len(data) >= 4 && string(data[:4]) == "\x89PNG":
		return ".png"
	case len(data) >= 3 && string(data[:3]) == "\xFF\xD8\xFF":
		return ".jpg"
	case len(data) >= 4 && string(data[:4]) == "GIF8":
		return ".gif"
	case len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP":
		return ".webp"
	case len(data) >= 4 && string(data[:4]) == "%PDF":
		return ".pdf"
	case len(data) >= 4 && string(data[:4]) == "PK\x03\x04":
		return ".zip"
	}

	if utf8.Valid(data) {
		return ".txt"
	}
	return ".bin"
}

// wrapText wraps text to fit within width columns, splitting on rune boundaries.
func wrapText(text string, width int) []string {
	if width < 1 {
		width = 1
	}
	var out []string
	for line := range strings.SplitSeq(text, "\n") {
		if runewidth.StringWidth(line) <= width {
			out = append(out, line)
			continue
		}
		for runewidth.StringWidth(line) > width {
			cut := 0
			w := 0
			for _, r := range line {
				rw := runewidth.RuneWidth(r)
				if w+rw > width {
					break
				}
				w += rw
				cut += utf8.RuneLen(r)
			}
			if cut == 0 {
				cut = 1
			}
			out = append(out, line[:cut])
			line = line[cut:]
		}
		if line != "" {
			out = append(out, line)
		}
	}
	if len(out) == 0 {
		out = []string{""}
	}
	return out
}
