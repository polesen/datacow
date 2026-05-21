package views

import (
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/polesen/datacow/internal/core/completions"
	"github.com/polesen/datacow/internal/core/config"
	"github.com/polesen/datacow/internal/core/dataset"
	"github.com/polesen/datacow/internal/tui/style"
)

// OpenSQLEditorMsg is emitted by the table list or row browser when the user
// asks to edit the SQL of a KindDataset dataset. The App opens the editor
// overlay in response.
type OpenSQLEditorMsg struct {
	Dataset dataset.Dataset
}

// DatasetSQLSavedMsg is emitted by the SQL editor after a successful save.
// The App closes the overlay, shows a status line, and reloads the dataset list.
type DatasetSQLSavedMsg struct {
	DatasetName string
	SQL         string
	Path        string
}

type sqlEditorSavedInternal struct {
	path string
	sql  string
}

type sqlEditorSaveErrorInternal struct {
	err error
}

// SQLEditorModel is the textarea-based multi-line SQL editor with schema-aware
// completion popup. Mounted as a full-screen overlay by the App.
type SQLEditorModel struct {
	textarea    textarea.Model
	completer   *completions.Completer
	popup       []completions.Suggestion // nil = popup closed
	popupCursor int
	popupPrefix string // text replaced when a suggestion is accepted

	original    string
	datasetName string
	configPath  string

	cancelled bool
	saved     bool
	err       string

	width  int
	height int
}

// NewSQLEditorModel constructs the editor pre-populated with the dataset's SQL.
// completer may be nil — when nil, Tab opens an empty popup that closes
// immediately. configPath is required for save; if empty, save reports an error.
func NewSQLEditorModel(ds dataset.Dataset, completer *completions.Completer, configPath string) SQLEditorModel {
	ta := textarea.New()
	ta.ShowLineNumbers = false
	ta.CharLimit = 0
	ta.SetValue(ds.SQL)
	ta.CursorEnd()
	_ = ta.Focus()
	return SQLEditorModel{
		textarea:    ta,
		completer:   completer,
		original:    ds.SQL,
		datasetName: ds.Name,
		configPath:  configPath,
	}
}

// SetSize updates the editor's terminal dimensions.
// The textarea height is shrunk so that the popup (up to 8 entries) plus the
// title, hint, padding, and border always fit inside h without forcing the top
// of the box to scroll off the visible area.
func (m SQLEditorModel) SetSize(w, h int) SQLEditorModel {
	m.width = w
	m.height = h
	innerW := max(w-4, 10)
	// Reserve: 1 title + 8 popup + 1 hint + 2 border = 12 chrome lines.
	innerH := max(h-12, 3)
	m.textarea.SetWidth(innerW)
	m.textarea.SetHeight(innerH)
	return m
}

// DatasetName returns the name of the dataset being edited.
func (m SQLEditorModel) DatasetName() string { return m.datasetName }

// SQL returns the current editor contents.
func (m SQLEditorModel) SQL() string { return m.textarea.Value() }

// IsCancelled reports whether the user cancelled the editor with Esc.
func (m SQLEditorModel) IsCancelled() bool { return m.cancelled }

// IsSaved reports whether the editor has emitted a successful save.
func (m SQLEditorModel) IsSaved() bool { return m.saved }

// IsPopupOpen reports whether the completion popup is currently visible.
func (m SQLEditorModel) IsPopupOpen() bool { return m.popup != nil }

// Update handles all messages routed to the editor.
func (m SQLEditorModel) Update(msg tea.Msg) (SQLEditorModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return m.SetSize(msg.Width, msg.Height), nil

	case sqlEditorSavedInternal:
		m.saved = true
		path := msg.path
		sql := msg.sql
		name := m.datasetName
		return m, func() tea.Msg {
			return DatasetSQLSavedMsg{DatasetName: name, SQL: sql, Path: path}
		}

	case sqlEditorSaveErrorInternal:
		m.err = msg.err.Error()
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	var cmd tea.Cmd
	m.textarea, cmd = m.textarea.Update(msg)
	return m, cmd
}

func (m SQLEditorModel) handleKey(msg tea.KeyMsg) (SQLEditorModel, tea.Cmd) {
	// Popup-open key handling takes precedence.
	if m.popup != nil {
		switch msg.Type {
		case tea.KeyTab:
			m.popupCursor = (m.popupCursor + 1) % len(m.popup)
			return m, nil
		case tea.KeyShiftTab:
			m.popupCursor = (m.popupCursor - 1 + len(m.popup)) % len(m.popup)
			return m, nil
		case tea.KeyEnter:
			return m.acceptSuggestion(), nil
		case tea.KeyEsc:
			m.popup = nil
			m.popupCursor = 0
			m.popupPrefix = ""
			return m, nil
		default:
			// Any other key closes the popup and forwards to the textarea.
			m.popup = nil
			m.popupCursor = 0
			m.popupPrefix = ""
			var cmd tea.Cmd
			m.textarea, cmd = m.textarea.Update(msg)
			return m, cmd
		}
	}

	switch msg.Type {
	case tea.KeyTab:
		return m.openPopup(), nil
	case tea.KeyCtrlS:
		return m.confirm()
	case tea.KeyEsc:
		m.cancelled = true
		return m, nil
	}

	var cmd tea.Cmd
	m.textarea, cmd = m.textarea.Update(msg)
	m.err = ""
	return m, cmd
}

// openPopup computes the current prefix and asks the completer for suggestions.
// If no suggestions match, the popup stays closed.
func (m SQLEditorModel) openPopup() SQLEditorModel {
	if m.completer == nil {
		return m
	}
	sql := m.textarea.Value()
	cursorPos := m.cursorByteOffset()
	prefix := currentPrefix(sql, cursorPos)
	suggestions := m.completer.Complete(sql, cursorPos)
	if len(suggestions) == 0 {
		return m
	}
	m.popup = suggestions
	m.popupCursor = 0
	m.popupPrefix = prefix
	return m
}

// acceptSuggestion replaces the current prefix with the selected suggestion text.
func (m SQLEditorModel) acceptSuggestion() SQLEditorModel {
	if m.popupCursor < 0 || m.popupCursor >= len(m.popup) {
		m.popup = nil
		return m
	}
	suggestion := m.popup[m.popupCursor].Text
	prefixRunes := utf8.RuneCountInString(m.popupPrefix)
	for i := 0; i < prefixRunes; i++ {
		m.textarea, _ = m.textarea.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	}
	m.textarea.InsertString(suggestion)
	m.popup = nil
	m.popupCursor = 0
	m.popupPrefix = ""
	return m
}

// confirm validates and persists the SQL to the config file.
func (m SQLEditorModel) confirm() (SQLEditorModel, tea.Cmd) {
	sql := strings.TrimSpace(m.textarea.Value())
	if sql == "" {
		m.err = "SQL cannot be empty"
		return m, nil
	}
	if m.configPath == "" {
		m.err = "no config file path — cannot save"
		return m, nil
	}
	m.err = ""
	editorSQL := m.textarea.Value()
	configPath := m.configPath
	datasetName := m.datasetName
	return m, func() tea.Msg {
		if err := config.UpdateDatasetSQL(configPath, datasetName, editorSQL); err != nil {
			return sqlEditorSaveErrorInternal{err: err}
		}
		return sqlEditorSavedInternal{path: configPath, sql: editorSQL}
	}
}

// cursorByteOffset returns the byte offset of the textarea cursor within
// the full SQL value. Used to feed Complete() at the right position.
func (m SQLEditorModel) cursorByteOffset() int {
	value := m.textarea.Value()
	row := m.textarea.Line()
	info := m.textarea.LineInfo()
	runeCol := info.StartColumn + info.ColumnOffset

	lines := strings.Split(value, "\n")
	if row >= len(lines) {
		return len(value)
	}
	offset := 0
	for i := 0; i < row; i++ {
		offset += len(lines[i]) + 1 // +1 for the newline
	}
	lineRunes := []rune(lines[row])
	if runeCol > len(lineRunes) {
		runeCol = len(lineRunes)
	}
	offset += len(string(lineRunes[:runeCol]))
	return offset
}

// currentPrefix returns the run of identifier characters immediately left of
// cursorPos. Mirrors the prefix detection in the completer.
func currentPrefix(sql string, cursorPos int) string {
	if cursorPos < 0 {
		cursorPos = 0
	} else if cursorPos > len(sql) {
		cursorPos = len(sql)
	}
	start := cursorPos
	for start > 0 && isPrefixChar(sql[start-1]) {
		start--
	}
	return sql[start:cursorPos]
}

func isPrefixChar(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') ||
		(b >= '0' && b <= '9') || b == '_' || b == '.'
}

// View renders the editor overlay.
func (m SQLEditorModel) View() string {
	innerW := m.width - 4
	if innerW < 10 {
		innerW = 10
	}

	title := "Edit SQL — " + m.datasetName

	var sections []string
	sections = append(sections, style.ColHeader.Render(title))
	sections = append(sections, m.textarea.View())

	if m.popup != nil {
		sections = append(sections, m.renderPopup(innerW))
	}

	if m.err != "" {
		sections = append(sections, style.Error.Render(m.err))
	}

	hint := "Tab completions · Ctrl+S save · Esc cancel"
	sections = append(sections, style.Muted.Render(hint))

	body := strings.Join(sections, "\n")

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#7DCFFF")).
		Padding(0, 1).
		Width(m.width - 2)
	return box.Render(body)
}

func (m SQLEditorModel) renderPopup(maxWidth int) string {
	if len(m.popup) == 0 {
		return ""
	}
	const popupRows = 8
	rows := len(m.popup)
	if rows > popupRows {
		rows = popupRows
	}
	// Anchor the visible window so the cursor stays inside.
	start := 0
	if m.popupCursor >= popupRows {
		start = m.popupCursor - popupRows + 1
	}
	end := start + rows
	if end > len(m.popup) {
		end = len(m.popup)
	}

	var lines []string
	for i := start; i < end; i++ {
		marker := "  "
		if i == m.popupCursor {
			marker = "> "
		}
		text := m.popup[i].Text
		if m.popup[i].Detail != "" {
			text += "  " + m.popup[i].Detail
		}
		line := marker + text
		if maxWidth > 0 && lipgloss.Width(line) > maxWidth-2 {
			line = line[:maxWidth-2]
		}
		if i == m.popupCursor {
			line = style.RowSelected.Render(line)
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}
