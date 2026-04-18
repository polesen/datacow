package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-runewidth"

	"github.com/beetio/datacow/internal/core/config"
	"github.com/beetio/datacow/internal/core/db"
	"github.com/beetio/datacow/internal/tui/keys"
	"github.com/beetio/datacow/internal/tui/style"
)

// ConnStatus represents the connection state of a datasource.
type ConnStatus int

const (
	StatusDisconnected ConnStatus = iota
	StatusConnecting
	StatusConnected
	StatusError
)

// DatasourceSelectMsg is emitted when the user presses Enter on a datasource entry.
type DatasourceSelectMsg struct {
	Name             string
	ConnectionString string
}

// DatasourceConnectingMsg is sent to update the list when a connection attempt begins.
type DatasourceConnectingMsg struct {
	Name string
}

// DatasourceConnectedMsg is sent when an async connection succeeds.
type DatasourceConnectedMsg struct {
	Name   string
	Client db.Client
}

// DatasourceErrorMsg is sent when an async connection fails.
type DatasourceErrorMsg struct {
	Name string
	Err  error
}

// DatasourceListModel renders a list of configured datasources with connection status.
type DatasourceListModel struct {
	datasources []config.DatasourceConfig
	statuses    map[string]ConnStatus
	errors      map[string]error
	cursor      int
	keys        keys.Map
	width       int
	height      int
}

// NewDatasourceListModel creates a model with all datasources initially disconnected.
func NewDatasourceListModel(k keys.Map, datasources []config.DatasourceConfig) DatasourceListModel {
	return DatasourceListModel{
		datasources: datasources,
		statuses:    make(map[string]ConnStatus),
		errors:      make(map[string]error),
		keys:        k,
	}
}

func (m DatasourceListModel) Update(msg tea.Msg) (DatasourceListModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Up):
			if m.cursor > 0 {
				m.cursor--
			}
		case key.Matches(msg, m.keys.Down):
			if m.cursor < len(m.datasources)-1 {
				m.cursor++
			}
		case key.Matches(msg, m.keys.Enter), key.Matches(msg, m.keys.Right):
			if len(m.datasources) == 0 {
				return m, nil
			}
			ds := m.datasources[m.cursor]
			return m, func() tea.Msg {
				return DatasourceSelectMsg{
					Name:             ds.Name,
					ConnectionString: ds.ConnectionString,
				}
			}
		}

	case DatasourceConnectingMsg:
		m.statuses[msg.Name] = StatusConnecting

	case DatasourceConnectedMsg:
		m.statuses[msg.Name] = StatusConnected

	case DatasourceErrorMsg:
		m.statuses[msg.Name] = StatusError
		m.errors[msg.Name] = msg.Err
	}

	return m, nil
}

// Cursor returns the current cursor position.
func (m DatasourceListModel) Cursor() int { return m.cursor }

// Status returns the connection status for the named datasource.
func (m DatasourceListModel) Status(name string) ConnStatus {
	if s, ok := m.statuses[name]; ok {
		return s
	}
	return StatusDisconnected
}

func (m DatasourceListModel) View() string {
	if m.width == 0 || len(m.datasources) == 0 {
		return ""
	}

	const statusWidth = 14
	const margin = 2

	nameWidth := m.width - statusWidth - margin*2
	if nameWidth < 8 {
		nameWidth = 8
	}

	lines := make([]string, 0, len(m.datasources))
	for i, ds := range m.datasources {
		name := runewidth.Truncate(ds.Name, nameWidth, "…")
		statusStr := m.statusText(ds.Name)
		pad := statusWidth - runewidth.StringWidth(statusStr)
		if pad < 0 {
			pad = 0
		}

		if i == m.cursor {
			row := "  " + runewidth.FillRight(name, nameWidth) + strings.Repeat(" ", pad) + statusStr
			lines = append(lines, style.RowSelected.Width(m.width).Render(row))
		} else {
			styled := m.statusStyled(ds.Name)
			row := fmt.Sprintf("  %-*s%s%s", nameWidth, name, strings.Repeat(" ", pad), styled)
			lines = append(lines, style.RowNormal.Width(m.width).Render(row))
		}
	}

	return style.Content.Width(m.width).Height(m.height).Render(
		strings.Join(lines, "\n"),
	)
}

func (m DatasourceListModel) statusText(name string) string {
	switch m.statuses[name] {
	case StatusConnecting:
		return "connecting…"
	case StatusConnected:
		return "connected"
	case StatusError:
		return "error"
	default:
		return "—"
	}
}

func (m DatasourceListModel) statusStyled(name string) string {
	text := m.statusText(name)
	switch m.statuses[name] {
	case StatusConnected:
		return style.StatusConnected.Render(text)
	case StatusError:
		return style.Error.Render(text)
	default:
		return style.Muted.Render(text)
	}
}
