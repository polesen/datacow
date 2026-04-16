package tui

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/beetio/datacow/internal/core/dataset"
	"github.com/beetio/datacow/internal/core/db"
	"github.com/beetio/datacow/internal/tui/keys"
	"github.com/beetio/datacow/internal/tui/style"
	"github.com/beetio/datacow/internal/tui/views"
)

type screen int

const (
	screenTableList screen = iota
	screenRowBrowser
	screenError
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
	cfg        Config
	keys       keys.Map
	connLabel  string
	width      int
	height     int
	screen     screen
	tableList  views.TableListModel
	rowBrowser views.RowBrowserModel
	executor   *dataset.Executor
	initErr    error
}

// New creates a ready-to-run App.
// If client is nil or connErr is non-nil, the App shows the error screen.
func New(cfg Config, client db.Client, connErr error) *App {
	a := &App{
		cfg:       cfg,
		keys:      keys.Default(),
		connLabel: parseConnLabel(cfg.ConnectionString),
		initErr:   connErr,
	}

	if client != nil && connErr == nil {
		resolver := dataset.NewResolver(client)
		executor := dataset.NewExecutor(client)
		a.executor = executor
		a.tableList = views.NewTableListModel(a.keys, resolver, executor)
		a.screen = screenTableList
	} else {
		a.screen = screenError
	}

	return a
}

// Init implements tea.Model.
func (a *App) Init() tea.Cmd {
	if a.screen == screenTableList {
		return a.tableList.Init()
	}
	return nil
}

// Update implements tea.Model.
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if key.Matches(msg, a.keys.Quit) {
			return a, tea.Quit
		}
		// Open selected table
		if a.screen == screenTableList && (key.Matches(msg, a.keys.Enter) || key.Matches(msg, a.keys.Right)) {
			if ds := a.tableList.SelectedDataset(); ds != nil {
				a.rowBrowser = views.NewRowBrowserModel(a.keys, a.executor, *ds)
				// Pre-size the row browser with current dimensions
				h := a.contentHeight()
				a.rowBrowser, _ = a.rowBrowser.Update(tea.WindowSizeMsg{Width: a.width, Height: h})
				a.screen = screenRowBrowser
				return a, a.rowBrowser.Init()
			}
			return a, nil
		}
		// Go back to table list
		if a.screen == screenRowBrowser && key.Matches(msg, a.keys.Back) {
			a.screen = screenTableList
			return a, nil
		}

	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		h := a.contentHeight()
		inner := tea.WindowSizeMsg{Width: msg.Width, Height: h}
		switch a.screen {
		case screenTableList:
			a.tableList, _ = a.tableList.Update(inner)
		case screenRowBrowser:
			a.rowBrowser, _ = a.rowBrowser.Update(inner)
		}
		return a, nil
	}

	// Route remaining messages to the active screen.
	switch a.screen {
	case screenTableList:
		a.tableList, cmd = a.tableList.Update(msg)
	case screenRowBrowser:
		a.rowBrowser, cmd = a.rowBrowser.Update(msg)
	}

	return a, cmd
}

// View implements tea.Model.
func (a *App) View() string {
	if a.width == 0 {
		return ""
	}

	header := a.renderHeader()
	statusBar := a.renderStatusBar()
	content := a.renderContent()

	return lipgloss.JoinVertical(lipgloss.Left, header, content, statusBar)
}

func (a *App) contentHeight() int {
	// Header (1) + status bar (1) = 2 fixed lines.
	h := a.height - 2
	if h < 0 {
		h = 0
	}
	return h
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

func (a *App) renderContent() string {
	h := a.contentHeight()

	switch a.screen {
	case screenTableList:
		return a.tableList.View()
	case screenRowBrowser:
		return a.rowBrowser.View()
	case screenError:
		errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#F7768E"))
		var msg string
		if a.initErr != nil {
			msg = errStyle.Render("Connection error: " + a.initErr.Error())
		} else {
			msg = errStyle.Render("No connection. Use --connection-string to connect.")
		}
		return style.Content.Width(a.width).Height(h).Render(msg)
	}
	return style.Content.Width(a.width).Height(h).Render("")
}

func (a *App) renderStatusBar() string {
	if a.screen == screenRowBrowser && !a.rowBrowser.IsLoading() && a.rowBrowser.Err() == nil {
		return a.renderRowBrowserStatusBar()
	}
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

func (a *App) renderRowBrowserStatusBar() string {
	info := style.StatusDesc.Render(a.rowBrowser.StatusLine())

	keyParts := []string{
		style.StatusKey.Render("q") + style.StatusDesc.Render(" quit"),
		style.StatusKey.Render("esc") + style.StatusDesc.Render(" back"),
		style.StatusKey.Render("]") + style.StatusDesc.Render(" next"),
		style.StatusKey.Render("[") + style.StatusDesc.Render(" prev"),
		style.StatusKey.Render("←→") + style.StatusDesc.Render(" scroll"),
	}
	right := strings.Join(keyParts, "  ")

	gap := a.width - lipgloss.Width(info) - lipgloss.Width(right) - 2
	if gap < 1 {
		gap = 1
	}

	content := info + strings.Repeat(" ", gap) + right
	return style.StatusBar.Width(a.width).Render(content)
}

// parseConnLabel extracts a safe display string (host/db only, no credentials).
func parseConnLabel(cs string) string {
	if cs == "" {
		return "no connection"
	}
	if u, err := url.Parse(cs); err == nil && u.Host != "" {
		return u.Host + u.Path
	}
	return cs
}
