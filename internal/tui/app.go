package tui

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/beetio/datacow/internal/core/db"
	"github.com/beetio/datacow/internal/tui/keys"
	"github.com/beetio/datacow/internal/tui/style"
)

// Config holds the startup configuration passed from cmd/main.go.
type Config struct {
	// ConnectionString is the raw DSN supplied via --connection-string.
	ConnectionString string

	// Version is the binary version string (e.g. "0.1.0-dev").
	Version string
}

// App is the root Bubble Tea model for Datacow.
type App struct {
	cfg       Config
	client    db.Client // nil until M5 wires up a real connection
	keys      keys.Map
	connLabel string // pre-computed from cfg.ConnectionString

	width  int
	height int
}

// New creates a ready-to-run App. client may be nil for the TUI shell milestone.
func New(cfg Config, client db.Client) *App {
	return &App{
		cfg:       cfg,
		client:    client,
		keys:      keys.Default(),
		connLabel: parseConnLabel(cfg.ConnectionString),
	}
}

// Init implements tea.Model.
func (a *App) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height

	case tea.KeyMsg:
		if key.Matches(msg, a.keys.Quit) {
			return a, tea.Quit
		}
	}
	return a, nil
}

// View implements tea.Model.
func (a *App) View() string {
	if a.width == 0 {
		return ""
	}

	header := a.renderHeader()
	statusBar := a.renderStatusBar()

	contentHeight := a.height - lipgloss.Height(header) - lipgloss.Height(statusBar)
	if contentHeight < 0 {
		contentHeight = 0
	}
	content := a.renderContent(contentHeight)

	return lipgloss.JoinVertical(lipgloss.Left, header, content, statusBar)
}

func (a *App) renderHeader() string {
	title := style.HeaderTitle.Render(fmt.Sprintf("datacow %s", a.cfg.Version))
	connInfo := style.HeaderMeta.Render(a.connLabel)

	gap := a.width - lipgloss.Width(title) - lipgloss.Width(connInfo)
	if gap < 0 {
		gap = 0
	}
	fill := style.HeaderFill.Render(strings.Repeat(" ", gap))

	return lipgloss.JoinHorizontal(lipgloss.Top, title, fill, connInfo)
}

func (a *App) renderContent(height int) string {
	return style.Content.Width(a.width).Height(height).Render("")
}

func (a *App) renderStatusBar() string {
	bindings := a.keys.ShortHelp()
	parts := make([]string, 0, len(bindings)*2)
	for _, b := range bindings {
		parts = append(parts,
			style.StatusKey.Render(b.Help().Key),
			style.StatusDesc.Render(" "+b.Help().Desc),
		)
	}
	return style.StatusBar.Width(a.width).Render(strings.Join(parts, "  "))
}

// parseConnLabel extracts a safe display string (host/db only, no credentials)
// from a DSN. Falls back to the raw string if parsing fails.
func parseConnLabel(cs string) string {
	if cs == "" {
		return "no connection"
	}
	if u, err := url.Parse(cs); err == nil && u.Host != "" {
		return u.Host + u.Path
	}
	return cs
}
