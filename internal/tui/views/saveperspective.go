package views

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/polesen/datacow/internal/tui/style"
)

// SavePerspectiveModel is the inline overlay for naming and saving a perspective.
type SavePerspectiveModel struct {
	input     textinput.Model
	errMsg    string
	confirmed bool
	cancelled bool
}

// NewSavePerspectiveModel creates a fresh overlay with the text input focused.
func NewSavePerspectiveModel() SavePerspectiveModel {
	ti := textinput.New()
	ti.Placeholder = "perspective name"
	ti.CharLimit = 80
	ti.Width = 30
	return SavePerspectiveModel{input: ti}
}

// Focus focuses the text input and returns the Cmd needed.
func (m SavePerspectiveModel) Focus() (SavePerspectiveModel, tea.Cmd) {
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(nil)
	cmd2 := m.input.Focus()
	if cmd == nil {
		return m, cmd2
	}
	return m, tea.Batch(cmd, cmd2)
}

// Update handles key messages for the overlay.
func (m SavePerspectiveModel) Update(msg tea.Msg) (SavePerspectiveModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			if strings.TrimSpace(m.input.Value()) == "" {
				m.errMsg = "name is required"
				return m, nil
			}
			m.confirmed = true
			return m, nil
		case tea.KeyEsc:
			m.cancelled = true
			return m, nil
		default:
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			m.errMsg = ""
			return m, cmd
		}
	default:
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}
}

// Name returns the trimmed name value.
func (m SavePerspectiveModel) Name() string { return strings.TrimSpace(m.input.Value()) }

// IsConfirmed reports whether the user pressed Enter with a valid name.
func (m SavePerspectiveModel) IsConfirmed() bool { return m.confirmed }

// IsCancelled reports whether the user pressed Esc.
func (m SavePerspectiveModel) IsCancelled() bool { return m.cancelled }

// SetError updates the error message (used to show IO errors from a failed save).
func (m SavePerspectiveModel) SetError(msg string) SavePerspectiveModel {
	m.confirmed = false
	m.errMsg = msg
	return m
}

// View renders the overlay box.
func (m SavePerspectiveModel) View() string {
	boxW := 42
	inner := boxW - 4 // 2 border chars + 2 padding spaces

	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#7DCFFF")).
		Padding(0, 1)

	var lines []string
	lines = append(lines, style.ColHeader.Render("Save perspective"))
	lines = append(lines, "Name: "+m.input.View())
	if m.errMsg != "" {
		lines = append(lines, style.Error.Render(m.errMsg))
	} else {
		lines = append(lines, "")
	}
	lines = append(lines, style.Muted.Render("Enter confirm · Esc cancel"))

	_ = inner
	content := strings.Join(lines, "\n")
	return borderStyle.Width(boxW).Render(content)
}
