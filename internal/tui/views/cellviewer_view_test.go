package views_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/polesen/datacow/internal/tui/keys"
	"github.com/polesen/datacow/internal/tui/views"
)

func newCellViewer(tableName, column, colType string, raw []byte) views.CellViewerModel {
	msg := views.OpenCellViewerMsg{
		TableName:  tableName,
		PKDisplay:  "id=1",
		PKValues:   []string{"1"},
		ColumnName: column,
		ColumnType: colType,
		Raw:        raw,
	}
	m := views.NewCellViewerModel(keys.Default(), msg)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	return m
}

func TestCellViewerModel_WidthZeroRendersEmpty(t *testing.T) {
	msg := views.OpenCellViewerMsg{
		TableName: "users", ColumnName: "bio", ColumnType: "text", Raw: []byte("hello"),
	}
	m := views.NewCellViewerModel(keys.Default(), msg)
	// No WindowSizeMsg — width stays 0.
	if got := m.View(); got != "" {
		t.Errorf("width=0 should render empty string, got %q", got)
	}
}

func TestCellViewerModel_HeaderContainsMetadata(t *testing.T) {
	m := newCellViewer("users", "bio", "text", []byte("hello world"))
	out := m.View()
	for _, want := range []string{"users", "bio", "text"} {
		if !strings.Contains(out, want) {
			t.Errorf("cell viewer header missing %q", want)
		}
	}
}

func TestCellViewerModel_PlainTextContentVisible(t *testing.T) {
	m := newCellViewer("users", "bio", "text", []byte("hello world"))
	out := m.View()
	if !strings.Contains(out, "hello world") {
		t.Error("plain text content must be visible in the cell viewer")
	}
}

func TestCellViewerModel_JSONPrettyPrinted(t *testing.T) {
	raw := []byte(`{"name":"alice","age":30}`)
	m := newCellViewer("users", "meta", "jsonb", raw)
	out := m.View()
	// Pretty-printed JSON puts each key on its own line.
	if !strings.Contains(out, "\"name\"") || !strings.Contains(out, "\"age\"") {
		t.Error("JSON content should be pretty-printed in cell viewer")
	}
}

func TestCellViewerModel_EmptyRawShowsPlaceholder(t *testing.T) {
	m := newCellViewer("users", "bio", "text", []byte{})
	out := m.View()
	if !strings.Contains(out, "(empty)") {
		t.Error("empty raw bytes should show '(empty)' placeholder")
	}
}

func TestCellViewerModel_FooterIdleState(t *testing.T) {
	m := newCellViewer("users", "bio", "text", []byte("hello"))
	out := m.View()
	for _, want := range []string{"save", "copy", "scroll", "close"} {
		if !strings.Contains(out, want) {
			t.Errorf("idle footer missing %q hint", want)
		}
	}
}

func TestCellViewerModel_FooterSavePromptState(t *testing.T) {
	m := newCellViewer("users", "bio", "text", []byte("hello"))
	// Press 's' to open save prompt.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	out := m.View()
	if !strings.Contains(out, "Save as:") {
		t.Error("footer should show 'Save as:' prompt after pressing 's'")
	}
}

func TestCellViewerModel_EscCancelsSavePrompt(t *testing.T) {
	m := newCellViewer("users", "bio", "text", []byte("hello"))
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	out := m.View()
	// Should be back to idle state.
	if strings.Contains(out, "Save as:") {
		t.Error("esc should dismiss the save prompt and return to idle footer")
	}
	if !strings.Contains(out, "save") {
		t.Error("idle footer hints should be visible after esc")
	}
}

// TestCellViewerModel_ScrollChangesContent verifies that scrolling moves the
// visible window through the content — the top line changes after scrolling down.
func TestCellViewerModel_ScrollChangesContent(t *testing.T) {
	// Build a tall content block: 40 numbered lines.
	var lines []string
	for i := range 40 {
		lines = append(lines, strings.Repeat("x", 10))
		_ = i
	}
	// Use distinct first and last lines so we can identify the scroll position.
	raw := []byte("line-FIRST\n" + strings.Repeat("middle\n", 38) + "line-LAST")
	m := newCellViewer("t", "col", "text", raw)

	outBefore := m.View()

	// Scroll down enough to push "line-FIRST" off screen.
	for i := 0; i < 20; i++ {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	}
	outAfter := m.View()

	if outBefore == outAfter {
		t.Error("output should change after scrolling down through tall content")
	}
	if strings.Contains(outAfter, "line-FIRST") {
		t.Error("'line-FIRST' should have scrolled off screen after 20 down-presses")
	}
}
