package tui

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/beetio/datacow/internal/core/config"
	"github.com/beetio/datacow/internal/core/dataset"
	"github.com/beetio/datacow/internal/core/db"
	"github.com/beetio/datacow/internal/core/export"
	"github.com/beetio/datacow/internal/tui/keys"
	"github.com/beetio/datacow/internal/tui/style"
	"github.com/beetio/datacow/internal/tui/views"
)

type screen int

const (
	screenDatasourcePicker screen = iota // datasource selection (multi-datasource mode)
	screenSplit                          // normal 3-pane split view
	screenQueryLog                       // full-screen query log overlay
	screenError                          // connection failed
)

type focus int

const (
	focusTables     focus = iota // pane 1: table list (left)
	focusRowBrowser              // pane 2: row browser (right)
	focusSQL                     // pane 3: SQL log (bottom)
)

// sqlPaneContentH is the number of content lines in the SQL strip (excludes border and title).
const sqlPaneContentH = 3

// sqlPaneOuterH is the total rendered height of the SQL pane (border top + title + content + border bottom).
const sqlPaneOuterH = 1 + sqlPaneContentH + 2

// Config holds the startup configuration passed from cmd/main.go.
type Config struct {
	// ConnectionString is the raw DSN supplied via --connection-string.
	ConnectionString string

	// Version is the binary version string (e.g. "0.1.0-dev").
	Version string

	// ConfigDatasets are datasets loaded from the config file.
	ConfigDatasets []config.DatasetConfig

	// ActiveDatasource is the name of the active datasource (empty for --connection-string only).
	ActiveDatasource string

	// Datasources are all configured datasources, used when showing the picker.
	Datasources []config.DatasourceConfig
}

// App is the root Bubble Tea model for Datacow.
type App struct {
	cfg                 Config
	keys                keys.Map
	connLabel           string
	width               int
	height              int
	screen              screen
	screenBeforeOverlay screen
	focus               focus
	datasourcePicker    views.DatasourceListModel
	multiDatasource     bool
	connections         map[string]db.Client
	tableList           views.TableListModel
	rowBrowser          views.RowBrowserModel
	rowBrowserReady     bool
	sqlPane             views.SQLPaneModel
	queryLogView        views.QueryLogView
	executor            *dataset.Executor
	exporter            *export.Exporter
	queryLog            *db.QueryLog
	appSpinner          spinner.Model
	initErr             error
}

// New creates a ready-to-run App.
// If client is nil or connErr is non-nil (and no multi-datasource config), the App shows the error screen.
func New(cfg Config, client db.Client, connErr error) *App {
	s := spinner.New()
	s.Spinner = spinner.MiniDot

	a := &App{
		cfg:         cfg,
		keys:        keys.Default(),
		connLabel:   parseConnLabel(cfg.ConnectionString),
		initErr:     connErr,
		appSpinner:  s,
		connections: make(map[string]db.Client),
	}

	switch {
	case len(cfg.Datasources) > 1 && client == nil && connErr == nil:
		// Multi-datasource mode: show the picker first.
		a.datasourcePicker = views.NewDatasourceListModel(a.keys, cfg.Datasources)
		a.multiDatasource = true
		a.connLabel = "select a datasource"
		a.screen = screenDatasourcePicker

	case client != nil && connErr == nil:
		// Single connection already established.
		a.activateConnection(cfg.ActiveDatasource, client)
		a.screen = screenSplit
		a.focus = focusTables

	default:
		a.screen = screenError
	}

	return a
}

// activateConnection wires up the executor/resolver/tableList for an open connection.
func (a *App) activateConnection(name string, client db.Client) {
	queryLog := db.NewQueryLog()
	lc := db.NewLoggingClient(client, queryLog)
	resolver := dataset.NewResolver(lc, a.cfg.ConfigDatasets, name)
	executor := dataset.NewExecutor(lc)
	a.queryLog = queryLog
	a.queryLogView = views.NewQueryLogView(queryLog)
	a.executor = executor
	a.exporter = export.NewExporter(executor)
	a.tableList = views.NewTableListModel(a.keys, resolver, executor, lc)
	a.sqlPane = views.NewSQLPaneModel(a.keys, queryLog)
	a.rowBrowserReady = false
	a.focus = focusTables
	if name != "" {
		a.connLabel = name
	}
}

// connectCmd fires an async connection attempt and returns the result as a message.
func (a *App) connectCmd(name, connStr string) tea.Cmd {
	return func() tea.Msg {
		client, err := db.Connect(connStr)
		if err != nil {
			return views.DatasourceErrorMsg{Name: name, Err: err}
		}
		return views.DatasourceConnectedMsg{Name: name, Client: client}
	}
}

// Close releases all open database connections managed by the App.
func (a *App) Close() {
	for _, client := range a.connections {
		_ = client.Close()
	}
}

// Init implements tea.Model.
func (a *App) Init() tea.Cmd {
	cmds := []tea.Cmd{a.appSpinner.Tick}
	if a.screen == screenSplit {
		cmds = append(cmds, a.tableList.Init())
	}
	return tea.Batch(cmds...)
}

// ---- Sizing helpers ----

// contentHeight is total lines available between header and status bar.
func (a *App) contentHeight() int {
	h := a.height - 3 // header (1) + blank spacer (1) + status bar (1)
	if h < 0 {
		h = 0
	}
	return h
}

// topSectionOuterH is the height allocated to the left+right panel row.
func (a *App) topSectionOuterH() int {
	h := a.contentHeight() - sqlPaneOuterH
	if h < 4 {
		h = 4
	}
	return h
}

// panelInnerH is the inner height of the left/right bordered panels (title line + model content).
func (a *App) panelInnerH() int {
	h := a.topSectionOuterH() - 2 // subtract top + bottom border
	if h < 2 {
		h = 2
	}
	return h
}

// modelH is the height given to each model's View (panelInnerH minus the title line).
func (a *App) modelH() int {
	h := a.panelInnerH() - 1
	if h < 1 {
		h = 1
	}
	return h
}

func (a *App) leftOuterW() int {
	w := a.width * 28 / 100
	if w < 20 {
		w = 20
	}
	return w
}

func (a *App) leftInnerW() int {
	w := a.leftOuterW() - 2
	if w < 1 {
		w = 1
	}
	return w
}

func (a *App) rightOuterW() int { return a.width - a.leftOuterW() }

func (a *App) rightInnerW() int {
	w := a.rightOuterW() - 2
	if w < 1 {
		w = 1
	}
	return w
}

func (a *App) sqlInnerW() int {
	w := a.width - 2
	if w < 1 {
		w = 1
	}
	return w
}

// Update implements tea.Model.
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	// Always advance the spinner and sync its frame to the query log view.
	if tickMsg, ok := msg.(spinner.TickMsg); ok {
		a.appSpinner, cmd = a.appSpinner.Update(tickMsg)
		if a.queryLog != nil {
			a.queryLogView.SetSpinChar(a.appSpinner.View())
		}
		a.tableList, _ = a.tableList.Update(msg)
		if a.rowBrowserReady {
			a.rowBrowser, _ = a.rowBrowser.Update(msg)
		}
		return a, cmd
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if key.Matches(msg, a.keys.Quit) {
			return a, tea.Quit
		}

		// Query log overlay toggle — only available when in split view.
		if a.screen == screenSplit || a.screen == screenQueryLog {
			if key.Matches(msg, a.keys.QueryLog) {
				if a.screen == screenQueryLog {
					a.screen = a.screenBeforeOverlay
				} else {
					a.screenBeforeOverlay = a.screen
					a.screen = screenQueryLog
				}
				return a, nil
			}
			if a.screen == screenQueryLog && key.Matches(msg, a.keys.Back) {
				a.screen = a.screenBeforeOverlay
				return a, nil
			}
		}

		if a.screen == screenSplit {
			inFilterInput := a.rowBrowserReady && a.rowBrowser.FilterInputActive()
			if !inFilterInput {
				switch msg.String() {
				case "1":
					a.focus = focusTables
					return a, nil
				case "2":
					a.focus = focusRowBrowser
					return a, nil
				case "3":
					a.focus = focusSQL
					return a, nil
				}
			}

			if key.Matches(msg, a.keys.SwitchFocus) {
				if !a.rowBrowserReady || a.focus != focusRowBrowser || !a.rowBrowser.NeedsTabKey() {
					a.focus = focus((int(a.focus) + 1) % 3)
					return a, nil
				}
			}

			// Right on a collapsed, expandable table-list row expands the tree
			// instead of drilling; Right on an already-expanded row (or Enter)
			// drills into the row browser.
			if a.focus == focusTables && key.Matches(msg, a.keys.Right) &&
				a.tableList.FocusedExpandable() && !a.tableList.FocusedExpanded() {
				a.tableList, cmd = a.tableList.Update(msg)
				return a, cmd
			}
			// Left on an expanded row collapses the tree — intercept before
			// the row-browser focus-shift logic below (which ignores Left on
			// focusTables anyway, but keep this explicit).
			if a.focus == focusTables && key.Matches(msg, a.keys.Left) && a.tableList.FocusedExpanded() {
				a.tableList, cmd = a.tableList.Update(msg)
				return a, cmd
			}

			if a.focus == focusTables && (key.Matches(msg, a.keys.Enter) || key.Matches(msg, a.keys.Right)) {
				if ds := a.tableList.SelectedDataset(); ds != nil {
					a.rowBrowser = views.NewRowBrowserModel(a.keys, a.executor, a.exporter, *ds)
					sizeMsg := tea.WindowSizeMsg{Width: a.rightInnerW(), Height: a.modelH()}
					a.rowBrowser, _ = a.rowBrowser.Update(sizeMsg)
					a.rowBrowserReady = true
					a.focus = focusRowBrowser
					return a, a.rowBrowser.Init()
				}
				return a, nil
			}

			// Left from row browser at column 0 with no modal/drill → focus table list.
			if a.focus == focusRowBrowser && key.Matches(msg, a.keys.Left) &&
				a.rowBrowserReady && !a.rowBrowser.NeedsBackKey() &&
				a.rowBrowser.ColCursor() == 0 {
				a.focus = focusTables
				return a, nil
			}

			// Esc from row browser with nothing to collapse → focus table list.
			if a.focus == focusRowBrowser && key.Matches(msg, a.keys.Back) &&
				a.rowBrowserReady && !a.rowBrowser.NeedsBackKey() {
				a.focus = focusTables
				return a, nil
			}

			// Esc from table list in multi-datasource mode → back to picker.
			if a.focus == focusTables && key.Matches(msg, a.keys.Back) && a.multiDatasource {
				a.screen = screenDatasourcePicker
				return a, nil
			}
		}

	case views.DatasourceSelectMsg:
		if existing, ok := a.connections[msg.Name]; ok {
			// Reuse the existing open connection.
			a.activateConnection(msg.Name, existing)
			a.screen = screenSplit
			return a, a.tableList.Init()
		}
		// Begin async connect and update picker status.
		connecting := views.DatasourceConnectingMsg{Name: msg.Name}
		a.datasourcePicker, _ = a.datasourcePicker.Update(connecting)
		return a, a.connectCmd(msg.Name, msg.ConnectionString)

	case views.DatasourceConnectedMsg:
		a.connections[msg.Name] = msg.Client
		a.datasourcePicker, _ = a.datasourcePicker.Update(msg)
		a.activateConnection(msg.Name, msg.Client)
		// Push current terminal size into the freshly created components — no
		// new WindowSizeMsg arrives just because the screen transitions.
		a.tableList, _ = a.tableList.Update(tea.WindowSizeMsg{Width: a.leftInnerW(), Height: a.modelH()})
		a.sqlPane, _ = a.sqlPane.Update(tea.WindowSizeMsg{Width: a.sqlInnerW(), Height: sqlPaneContentH})
		a.screen = screenSplit
		return a, a.tableList.Init()

	case views.DatasourceErrorMsg:
		a.datasourcePicker, _ = a.datasourcePicker.Update(msg)
		return a, nil

	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.datasourcePicker, _ = a.datasourcePicker.Update(tea.WindowSizeMsg{Width: a.leftInnerW(), Height: a.modelH()})
		a.tableList, _ = a.tableList.Update(tea.WindowSizeMsg{Width: a.leftInnerW(), Height: a.modelH()})
		if a.rowBrowserReady {
			a.rowBrowser, _ = a.rowBrowser.Update(tea.WindowSizeMsg{Width: a.rightInnerW(), Height: a.modelH()})
		}
		a.sqlPane, _ = a.sqlPane.Update(tea.WindowSizeMsg{Width: a.sqlInnerW(), Height: sqlPaneContentH})
		if a.screen == screenQueryLog {
			a.queryLogView, _ = a.queryLogView.Update(tea.WindowSizeMsg{Width: a.width - 1, Height: a.contentHeight()})
		}
		return a, nil
	}

	// RowCountMsg always goes to the table list.
	if _, ok := msg.(views.RowCountMsg); ok {
		a.tableList, cmd = a.tableList.Update(msg)
		return a, cmd
	}

	// Route remaining messages to the active screen/pane.
	switch a.screen {
	case screenDatasourcePicker:
		a.datasourcePicker, cmd = a.datasourcePicker.Update(msg)
	case screenSplit:
		switch a.focus {
		case focusTables:
			a.tableList, cmd = a.tableList.Update(msg)
		case focusRowBrowser:
			if a.rowBrowserReady {
				a.rowBrowser, cmd = a.rowBrowser.Update(msg)
			}
		case focusSQL:
			a.sqlPane, cmd = a.sqlPane.Update(msg)
		}
	case screenQueryLog:
		a.queryLogView, cmd = a.queryLogView.Update(msg)
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
	return lipgloss.JoinVertical(lipgloss.Left, header, "", content, statusBar)
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
	switch a.screen {
	case screenDatasourcePicker:
		return a.renderDatasourcePicker()
	case screenSplit:
		return a.renderSplitContent()
	case screenQueryLog:
		leftPad := lipgloss.NewStyle().PaddingLeft(1)
		return leftPad.Render(a.queryLogView.View())
	case screenError:
		var msg string
		if a.initErr != nil {
			msg = style.Error.Render("Connection error: " + a.initErr.Error())
		} else {
			msg = style.Error.Render("No connection. Use --connection-string to connect.")
		}
		return style.Content.Width(a.width).Height(a.contentHeight()).Render(msg)
	}
	return ""
}

func (a *App) renderDatasourcePicker() string {
	lw := a.leftInnerW()
	rw := a.rightInnerW()
	ph := a.panelInnerH()
	sw := a.sqlInnerW()
	sqlH := 1 + sqlPaneContentH

	leftContent := lipgloss.JoinVertical(lipgloss.Left,
		a.paneTitle("Datasources", true, lw),
		a.datasourcePicker.View(),
	)
	leftBox := paneBorder(true).Width(lw).Height(ph).Render(leftContent)

	hint := style.Muted.Render("\n  Select a datasource\n  to browse its data.")
	rightBox := paneBorder(false).Width(rw).Height(ph).Render(hint)

	topRow := lipgloss.JoinHorizontal(lipgloss.Top, leftBox, rightBox)
	sqlBox := paneBorder(false).Width(sw).Height(sqlH).Render("")

	return lipgloss.JoinVertical(lipgloss.Left, topRow, sqlBox)
}

func (a *App) renderSplitContent() string {
	lw := a.leftInnerW()
	rw := a.rightInnerW()
	ph := a.panelInnerH()
	sw := a.sqlInnerW()
	sqlH := 1 + sqlPaneContentH // title line + content lines

	leftContent := lipgloss.JoinVertical(lipgloss.Left,
		a.paneTitle("1 Tables", a.focus == focusTables, lw),
		a.tableList.View(),
	)
	leftBox := paneBorder(a.focus == focusTables).Width(lw).Height(ph).Render(leftContent)

	rightContent := lipgloss.JoinVertical(lipgloss.Left,
		a.paneTitle("2 Row Browser", a.focus == focusRowBrowser, rw),
		a.renderRightPane(),
	)
	rightBox := paneBorder(a.focus == focusRowBrowser).Width(rw).Height(ph).Render(rightContent)

	topRow := lipgloss.JoinHorizontal(lipgloss.Top, leftBox, rightBox)

	sqlContent := lipgloss.JoinVertical(lipgloss.Left,
		a.paneTitle("3 SQL", a.focus == focusSQL, sw),
		a.sqlPane.SetFocused(a.focus == focusSQL).View(),
	)
	sqlBox := paneBorder(a.focus == focusSQL).Width(sw).Height(sqlH).Render(sqlContent)

	return lipgloss.JoinVertical(lipgloss.Left, topRow, sqlBox)
}

func (a *App) renderRightPane() string {
	if !a.rowBrowserReady {
		return style.Muted.Render("  Select a table with ↵ or → or press 1")
	}
	return a.rowBrowser.View()
}

func (a *App) paneTitle(title string, active bool, width int) string {
	if active {
		return style.PanelTitleActive.Width(width).Render(title)
	}
	return style.PanelTitleInactive.Width(width).Render(title)
}

func paneBorder(active bool) lipgloss.Style {
	if active {
		return style.PanelActive
	}
	return style.PanelInactive
}

func (a *App) renderStatusBar() string {
	var runningPart string
	if a.queryLog != nil && a.queryLog.RunningCount() > 0 {
		count := a.queryLog.RunningCount()
		label := a.queryLog.CurrentLabel()
		runningPart = style.StatusKey.Render(a.appSpinner.View()) +
			style.StatusDesc.Render(fmt.Sprintf(" %d running: %s", count, label))
	}

	if a.screen == screenSplit && a.focus == focusRowBrowser &&
		a.rowBrowserReady && !a.rowBrowser.IsLoading() && a.rowBrowser.Err() == nil {
		return a.renderRowBrowserStatusBar(runningPart)
	}

	var bindings []key.Binding
	switch {
	case a.screen == screenDatasourcePicker:
		bindings = []key.Binding{a.keys.Quit, a.keys.Up, a.keys.Down, a.keys.Enter}
	case a.screen == screenQueryLog:
		bindings = []key.Binding{a.keys.Up, a.keys.Down, a.keys.QueryLog, a.keys.Back}
	case a.focus == focusTables:
		bindings = a.keys.TableListHelp()
	case a.focus == focusSQL:
		bindings = []key.Binding{a.keys.Quit, a.keys.Up, a.keys.Down, a.keys.SwitchFocus}
	default:
		bindings = a.keys.ShortHelp()
	}
	parts := make([]string, 0, len(bindings)+1)
	if runningPart != "" {
		parts = append(parts, runningPart)
	}
	for _, b := range bindings {
		parts = append(parts, style.StatusKey.Render(b.Help().Key)+style.StatusDesc.Render(" "+b.Help().Desc))
	}
	return style.StatusBar.Width(a.width).Render(strings.Join(parts, "  "))
}

func (a *App) renderRowBrowserStatusBar(runningPart string) string {
	info := style.StatusDesc.Render(a.rowBrowser.StatusLine())

	escDesc := " pane 1"
	if a.rowBrowser.DrillDepth() > 0 {
		escDesc = " collapse"
	}

	keyParts := []string{
		style.StatusKey.Render("q") + style.StatusDesc.Render(" quit"),
		style.StatusKey.Render("esc") + style.StatusDesc.Render(escDesc),
		style.StatusKey.Render("[") + style.StatusDesc.Render(" prev"),
		style.StatusKey.Render("]") + style.StatusDesc.Render(" next"),
		style.StatusKey.Render("↑↓") + style.StatusDesc.Render(" row"),
		style.StatusKey.Render("←→") + style.StatusDesc.Render(" col"),
		style.StatusKey.Render("/") + style.StatusDesc.Render(" filter"),
		style.StatusKey.Render("s") + style.StatusDesc.Render(" sort"),
		style.StatusKey.Render("e") + style.StatusDesc.Render(" export"),
	}
	if a.rowBrowser.IsFKColumn() {
		keyParts = append(keyParts, style.StatusKey.Render("↵")+style.StatusDesc.Render(" drill"))
	}
	right := strings.Join(keyParts, "  ")

	left := info
	if runningPart != "" {
		left = runningPart + "  " + info
	}

	gap := a.width - lipgloss.Width(left) - lipgloss.Width(right) - 2
	if gap < 1 {
		gap = 1
	}

	content := left + strings.Repeat(" ", gap) + right
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
