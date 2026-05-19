package tui

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/polesen/datacow/internal/core/config"
	"github.com/polesen/datacow/internal/core/dataset"
	"github.com/polesen/datacow/internal/core/db"
	"github.com/polesen/datacow/internal/core/export"
	"github.com/polesen/datacow/internal/core/schema"
	"github.com/polesen/datacow/internal/tui/keys"
	"github.com/polesen/datacow/internal/tui/style"
	"github.com/polesen/datacow/internal/tui/views"
)

type screen int

const (
	screenDatasourcePicker screen = iota // datasource selection (multi-datasource mode)
	screenSplit                          // normal 3-pane split view
	screenQueryLog                       // full-screen query log overlay
	screenCellViewer                     // full-screen cell viewer overlay
	screenError                          // connection failed
	screenGoto                           // fuzzy goto dialog overlay
	screenHelp                           // full-screen keybinding reference overlay
	screenTableInfo                      // table info overlay
)

type focus int

const (
	focusTables     focus = iota // pane 1: table list (left)
	focusRowBrowser              // pane 2: row browser (right)
	focusSQL                     // pane 3: SQL log (bottom)
)

// sqlPaneContentH is the number of content lines in the SQL strip (excludes border).
const sqlPaneContentH = 3

// sqlPaneOuterH is the total rendered height of the SQL pane (border top + content + border bottom).
const sqlPaneOuterH = sqlPaneContentH + 2

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

// schemaCacheReadyMsg signals that the initial schema cache load completed.
type schemaCacheReadyMsg struct{}

// schemaCacheErrMsg signals that schema cache load failed.
type schemaCacheErrMsg struct{ Err error }

// schemaCacheRefreshedMsg signals that a ctrl+r refresh completed.
type schemaCacheRefreshedMsg struct{}

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
	maximized           bool
	datasourcePicker    views.DatasourceListModel
	multiDatasource     bool
	connections         map[string]db.Client
	tableList           views.TableListModel
	rowBrowser          views.RowBrowserModel
	rowBrowserReady     bool
	cellViewer          views.CellViewerModel
	sqlPane             views.SQLPaneModel
	queryLogView        views.QueryLogView
	helpView            views.HelpOverlayView
	executor            *dataset.Executor
	exporter            *export.Exporter
	queryLog            *db.QueryLog
	appSpinner          spinner.Model
	initErr             error
	schemaCache         *schema.Cache
	gotoModel           views.GotoModel
	tableInfoModel      views.TableInfoModel
	cacheLoading        bool
	activeClient        db.Client
	activeResolver      *dataset.Resolver
	cacheInitCmd        tea.Cmd
	pageSizeRegistry    *views.PageSizeRegistry
	columnRegistry      *views.ColumnRegistry
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
	a.helpView = views.NewHelpOverlayView(a.keys)

	switch {
	case len(cfg.Datasources) > 1 && client == nil && connErr == nil:
		// Multi-datasource mode: show the picker first.
		a.datasourcePicker = views.NewDatasourceListModel(a.keys, cfg.Datasources)
		a.multiDatasource = true
		a.connLabel = "select a datasource"
		a.screen = screenDatasourcePicker

	case client != nil && connErr == nil:
		// Single connection already established.
		a.cacheInitCmd = a.activateConnection(cfg.ActiveDatasource, client)
		a.screen = screenSplit
		a.focus = focusTables

	default:
		a.screen = screenError
	}

	return a
}

// activateConnection wires up the executor/resolver/tableList for an open connection.
// Returns a tea.Cmd that starts the schema cache load in the background.
func (a *App) activateConnection(name string, client db.Client) tea.Cmd {
	queryLog := db.NewQueryLog()
	lc := db.NewLoggingClient(client, queryLog)
	resolver := dataset.NewResolver(lc, a.cfg.ConfigDatasets, name)
	executor := dataset.NewExecutor(lc)
	a.queryLog = queryLog
	a.queryLogView = views.NewQueryLogView(queryLog)
	a.executor = executor
	a.exporter = export.NewExporter(executor)
	a.schemaCache = schema.NewCache()
	a.pageSizeRegistry = views.NewPageSizeRegistry(50)
	a.columnRegistry = views.NewColumnRegistry()
	a.tableList = views.NewTableListModel(a.keys, resolver, executor, lc, a.schemaCache)
	a.sqlPane = views.NewSQLPaneModel(a.keys, queryLog)
	a.rowBrowserReady = false
	a.focus = focusTables
	if name != "" {
		a.connLabel = name
	}

	a.activeClient = lc
	a.activeResolver = resolver
	a.gotoModel = views.NewGotoModel(a.schemaCache, a.cfg.Datasources)
	a.cacheLoading = true
	return a.cacheLoadCmd()
}

// cacheLoadCmd starts a background cache load and returns the appropriate message.
func (a *App) cacheLoadCmd() tea.Cmd {
	cache := a.schemaCache
	client := a.activeClient
	resolver := a.activeResolver
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		if err := cache.Load(ctx, client, resolver); err != nil {
			return schemaCacheErrMsg{Err: err}
		}
		return schemaCacheReadyMsg{}
	}
}

// cacheRefreshCmd starts a background cache refresh.
func (a *App) cacheRefreshCmd() tea.Cmd {
	cache := a.schemaCache
	client := a.activeClient
	resolver := a.activeResolver
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		if err := cache.Refresh(ctx, client, resolver); err != nil {
			return schemaCacheErrMsg{Err: err}
		}
		return schemaCacheRefreshedMsg{}
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
		if a.cacheInitCmd != nil {
			cmds = append(cmds, a.cacheInitCmd)
		}
	}
	return tea.Batch(cmds...)
}

// ---- Sizing helpers ----

// contentHeight is total lines available between header and status bar.
func (a *App) contentHeight() int {
	h := a.height - 2 // header (1) + status bar (1)
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

// modelH is the height given to each model's View (topSectionOuterH minus top and bottom borders).
func (a *App) modelH() int {
	h := a.topSectionOuterH() - 2
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

func (a *App) maximizedPanelH() int {
	h := a.contentHeight() - 2
	if h < 1 {
		h = 1
	}
	return h
}

func (a *App) maximizedPanelInnerW() int {
	w := a.width - 2
	if w < 1 {
		w = 1
	}
	return w
}

func (a *App) pushNormalSizes() {
	a.tableList, _ = a.tableList.Update(
		tea.WindowSizeMsg{Width: a.leftInnerW(), Height: a.modelH()})
	if a.rowBrowserReady {
		a.rowBrowser, _ = a.rowBrowser.Update(
			tea.WindowSizeMsg{Width: a.rightInnerW(), Height: a.modelH()})
	}
	a.sqlPane, _ = a.sqlPane.Update(
		tea.WindowSizeMsg{Width: a.sqlInnerW(), Height: sqlPaneContentH})
}

func (a *App) pushMaximizedSizes() {
	switch a.focus {
	case focusTables:
		a.tableList, _ = a.tableList.Update(
			tea.WindowSizeMsg{Width: a.maximizedPanelInnerW(), Height: a.maximizedPanelH()})
	case focusRowBrowser:
		if a.rowBrowserReady {
			a.rowBrowser, _ = a.rowBrowser.Update(
				tea.WindowSizeMsg{Width: a.maximizedPanelInnerW(), Height: a.maximizedPanelH()})
		}
	}
}

// Update implements tea.Model.
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	// Always advance the spinner and sync its frame to the query log view.
	if tickMsg, ok := msg.(spinner.TickMsg); ok {
		a.appSpinner, cmd = a.appSpinner.Update(tickMsg)
		if a.queryLog != nil {
			a.queryLogView = a.queryLogView.SetSpinChar(a.appSpinner.View())
		}
		a.tableList, _ = a.tableList.Update(msg)
		if a.rowBrowserReady {
			a.rowBrowser, _ = a.rowBrowser.Update(msg)
		}
		if a.screen == screenTableInfo {
			a.tableInfoModel.SetSpinChar(a.appSpinner.View())
		}
		return a, cmd
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if key.Matches(msg, a.keys.Quit) {
			if a.rowBrowserReady {
				a.rowBrowser.CancelExport()
			}
			return a, tea.Quit
		}

		// Whether a text-input field (table-list filter, row-browser local search) is
		// currently capturing keys. Global shortcuts must not fire while these are active.
		inModalInput := a.screen == screenSplit &&
			((a.focus == focusTables && a.tableList.BlocksGlobalKeys()) ||
				(a.rowBrowserReady && a.rowBrowser.BlocksGlobalKeys()))

		// ctrl+p: open goto dialog from any screen with an active connection.
		if !inModalInput && key.Matches(msg, a.keys.Goto) &&
			a.screen != screenDatasourcePicker &&
			a.screen != screenGoto &&
			a.schemaCache != nil {
			a.screenBeforeOverlay = a.screen
			a.screen = screenGoto
			a.gotoModel, _ = a.gotoModel.Update(
				tea.WindowSizeMsg{Width: a.width, Height: a.contentHeight()})
			var focusCmd tea.Cmd
			a.gotoModel, focusCmd = a.gotoModel.Focus()
			return a, focusCmd
		}

		// ctrl+r: trigger schema refresh when a connection is active.
		if !inModalInput && key.Matches(msg, a.keys.Refresh) &&
			a.schemaCache != nil &&
			!a.cacheLoading &&
			a.screen != screenDatasourcePicker {
			a.cacheLoading = true
			return a, a.cacheRefreshCmd()
		}

		// Esc from goto dialog: close without navigating.
		if a.screen == screenGoto && key.Matches(msg, a.keys.Back) {
			a.screen = a.screenBeforeOverlay
			return a, nil
		}

		// Query log overlay toggle — only available when in split view.
		if a.screen == screenSplit || a.screen == screenQueryLog {
			if !inModalInput && key.Matches(msg, a.keys.QueryLog) {
				if a.screen == screenQueryLog {
					a.screen = a.screenBeforeOverlay
				} else {
					a.screenBeforeOverlay = a.screen
					a.screen = screenQueryLog
					a.queryLogView, _ = a.queryLogView.Update(
						tea.WindowSizeMsg{Width: a.width - 1, Height: a.contentHeight()})
				}
				return a, nil
			}
			if a.screen == screenQueryLog && key.Matches(msg, a.keys.Back) {
				a.screen = a.screenBeforeOverlay
				return a, nil
			}
		}

		// Help overlay toggle — only available when in split view.
		if a.screen == screenSplit || a.screen == screenHelp {
			if !inModalInput && key.Matches(msg, a.keys.Help) {
				if a.screen == screenHelp {
					a.screen = a.screenBeforeOverlay
				} else {
					a.screenBeforeOverlay = a.screen
					a.screen = screenHelp
				}
				return a, nil
			}
			if a.screen == screenHelp && key.Matches(msg, a.keys.Back) {
				a.screen = a.screenBeforeOverlay
				return a, nil
			}
		}

		// Table info overlay — only when split view, table list focused, KindTable selected.
		if a.screen == screenSplit && a.focus == focusTables && !inModalInput && key.Matches(msg, a.keys.TableInfo) {
			if ds := a.tableList.SelectedDataset(); ds != nil && ds.Kind == dataset.KindTable {
				a.tableInfoModel = views.NewTableInfoModel()
				a.tableInfoModel.SetSize(a.width, a.contentHeight())
				a.screenBeforeOverlay = a.screen
				a.screen = screenTableInfo
				return a, a.tableInfoModel.Load(a.activeClient, ds.Table)
			}
			return a, nil
		}
		if a.screen == screenTableInfo {
			if key.Matches(msg, a.keys.TableInfo) || key.Matches(msg, a.keys.Back) {
				a.screen = a.screenBeforeOverlay
				return a, nil
			}
		}

		if a.screen == screenSplit {
			tableListBlocksKeys := a.focus == focusTables && a.tableList.BlocksGlobalKeys()
			if !inModalInput {
				switch msg.String() {
				case "1":
					a.focus = focusTables
					if a.maximized {
						a.pushMaximizedSizes()
					}
					a.tableList, cmd = a.tableList.OnFocusGained()
					return a, cmd
				case "2":
					a.focus = focusRowBrowser
					if a.maximized {
						a.pushMaximizedSizes()
					}
					if a.rowBrowserReady {
						a.rowBrowser, cmd = a.rowBrowser.OnFocusGained()
					}
					return a, cmd
				case "3":
					if a.maximized {
						a.maximized = false
						a.pushNormalSizes()
					}
					a.focus = focusSQL
					return a, nil
				}
			}

			if !inModalInput && key.Matches(msg, a.keys.Maximize) {
				if a.focus == focusSQL {
					a.screenBeforeOverlay = a.screen
					a.screen = screenQueryLog
					a.queryLogView, _ = a.queryLogView.Update(
						tea.WindowSizeMsg{Width: a.width - 1, Height: a.contentHeight()})
					return a, nil
				}
				a.maximized = !a.maximized
				if a.maximized {
					a.pushMaximizedSizes()
				} else {
					a.pushNormalSizes()
				}
				return a, nil
			}

			if key.Matches(msg, a.keys.SwitchFocus) {
				if !tableListBlocksKeys &&
					(!a.rowBrowserReady || a.focus != focusRowBrowser || !a.rowBrowser.NeedsTabKey()) {
					a.focus = focus((int(a.focus) + 1) % 3)
					if a.focus == focusTables {
						a.tableList, cmd = a.tableList.OnFocusGained()
					} else if a.focus == focusRowBrowser && a.rowBrowserReady {
						a.rowBrowser, cmd = a.rowBrowser.OnFocusGained()
					}
					return a, cmd
				}
			}

			if key.Matches(msg, a.keys.SwitchFocusBack) {
				if !tableListBlocksKeys &&
					(!a.rowBrowserReady || a.focus != focusRowBrowser || !a.rowBrowser.NeedsTabKey()) {
					a.focus = focus((int(a.focus) + 2) % 3)
					if a.focus == focusTables {
						a.tableList, cmd = a.tableList.OnFocusGained()
					} else if a.focus == focusRowBrowser && a.rowBrowserReady {
						a.rowBrowser, cmd = a.rowBrowser.OnFocusGained()
					}
					return a, cmd
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

			if !inModalInput && a.focus == focusTables && (key.Matches(msg, a.keys.Enter) || key.Matches(msg, a.keys.Right)) {
				if ds := a.tableList.SelectedDataset(); ds != nil {
					a.rowBrowser = views.NewRowBrowserModelWithColumns(a.keys, a.executor, a.exporter, *ds, a.pageSizeRegistry, a.schemaCache, a.columnRegistry)
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
				a.tableList, cmd = a.tableList.OnFocusGained()
				return a, cmd
			}

			// Esc from row browser with nothing to collapse → exit maximized or focus table list.
			if a.focus == focusRowBrowser && key.Matches(msg, a.keys.Back) &&
				a.rowBrowserReady && !a.rowBrowser.NeedsBackKey() {
				if a.maximized {
					a.maximized = false
					a.pushNormalSizes()
				} else {
					a.focus = focusTables
					a.tableList, cmd = a.tableList.OnFocusGained()
				}
				return a, cmd
			}

			// Esc from table list while filter is active → handled by tableList.Update below.
			// Only intercept if no filter is active.
			if a.focus == focusTables && key.Matches(msg, a.keys.Back) && !a.tableList.FilterActive() {
				// Esc from table list while maximized → restore split.
				if a.maximized {
					a.maximized = false
					a.pushNormalSizes()
					return a, nil
				}
				// Esc from table list in multi-datasource mode → back to picker.
				if a.multiDatasource {
					a.screen = screenDatasourcePicker
					return a, nil
				}
			}
		}

	case views.OpenCellViewerMsg:
		cv := views.NewCellViewerModel(a.keys, msg)
		cv, _ = cv.Update(tea.WindowSizeMsg{Width: a.width, Height: a.contentHeight()})
		a.cellViewer = cv
		a.screenBeforeOverlay = a.screen
		a.screen = screenCellViewer
		return a, nil

	case views.CloseCellViewerMsg:
		a.screen = a.screenBeforeOverlay
		return a, nil

	case views.GotoSelectedMsg:
		a.screen = a.screenBeforeOverlay
		if msg.Datasource != "" {
			// Find the connection string for this datasource and emit DatasourceSelectMsg.
			for _, ds := range a.cfg.Datasources {
				if ds.Name == msg.Datasource {
					return a, func() tea.Msg {
						return views.DatasourceSelectMsg{
							Name:             ds.Name,
							ConnectionString: ds.ConnectionString,
						}
					}
				}
			}
			return a, nil
		}
		if msg.Dataset != nil {
			a.tableList, _ = a.tableList.SelectByName(msg.Dataset.Name)
			a.rowBrowser = views.NewRowBrowserModelWithColumns(a.keys, a.executor, a.exporter, *msg.Dataset, a.pageSizeRegistry, a.schemaCache, a.columnRegistry)
			sizeMsg := tea.WindowSizeMsg{Width: a.rightInnerW(), Height: a.modelH()}
			a.rowBrowser, _ = a.rowBrowser.Update(sizeMsg)
			a.rowBrowserReady = true
			a.screen = screenSplit
			a.focus = focusRowBrowser
			return a, a.rowBrowser.Init()
		}
		return a, nil

	case schemaCacheReadyMsg:
		a.cacheLoading = false
		var cacheCmd tea.Cmd
		a.tableList, cacheCmd = a.tableList.OnCacheReady()
		return a, cacheCmd

	case schemaCacheErrMsg:
		a.cacheLoading = false
		return a, nil

	case schemaCacheRefreshedMsg:
		a.cacheLoading = false
		return a, nil

	case views.DatasourceSelectMsg:
		if existing, ok := a.connections[msg.Name]; ok {
			// Reuse the existing open connection.
			cacheCmd := a.activateConnection(msg.Name, existing)
			a.tableList, _ = a.tableList.Update(tea.WindowSizeMsg{Width: a.leftInnerW(), Height: a.modelH()})
			a.sqlPane, _ = a.sqlPane.Update(tea.WindowSizeMsg{Width: a.sqlInnerW(), Height: sqlPaneContentH})
			a.screen = screenSplit
			return a, tea.Batch(a.tableList.Init(), cacheCmd)
		}
		// Begin async connect and update picker status.
		connecting := views.DatasourceConnectingMsg{Name: msg.Name}
		a.datasourcePicker, _ = a.datasourcePicker.Update(connecting)
		return a, a.connectCmd(msg.Name, msg.ConnectionString)

	case views.DatasourceConnectedMsg:
		a.connections[msg.Name] = msg.Client
		a.datasourcePicker, _ = a.datasourcePicker.Update(msg)
		cacheCmd := a.activateConnection(msg.Name, msg.Client)
		// Push current terminal size into the freshly created components — no
		// new WindowSizeMsg arrives just because the screen transitions.
		a.tableList, _ = a.tableList.Update(tea.WindowSizeMsg{Width: a.leftInnerW(), Height: a.modelH()})
		a.sqlPane, _ = a.sqlPane.Update(tea.WindowSizeMsg{Width: a.sqlInnerW(), Height: sqlPaneContentH})
		a.screen = screenSplit
		return a, tea.Batch(a.tableList.Init(), cacheCmd)

	case views.DatasourceErrorMsg:
		a.datasourcePicker, _ = a.datasourcePicker.Update(msg)
		return a, nil

	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.datasourcePicker, _ = a.datasourcePicker.Update(tea.WindowSizeMsg{Width: a.leftInnerW(), Height: a.modelH()})
		a.pushNormalSizes()
		if a.maximized {
			a.pushMaximizedSizes()
		}
		if a.screen == screenQueryLog {
			a.queryLogView, _ = a.queryLogView.Update(tea.WindowSizeMsg{Width: a.width - 1, Height: a.contentHeight()})
		}
		if a.screen == screenCellViewer {
			a.cellViewer, _ = a.cellViewer.Update(tea.WindowSizeMsg{Width: a.width, Height: a.contentHeight()})
		}
		a.gotoModel, _ = a.gotoModel.Update(tea.WindowSizeMsg{Width: a.width, Height: a.contentHeight()})
		a.helpView.SetSize(a.width, a.contentHeight())
		a.tableInfoModel.SetSize(a.width, a.contentHeight())
		return a, nil
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
	case screenCellViewer:
		a.cellViewer, cmd = a.cellViewer.Update(msg)
	case screenGoto:
		a.gotoModel, cmd = a.gotoModel.Update(msg)
	case screenHelp:
		// static overlay — no message routing needed
	case screenTableInfo:
		a.tableInfoModel, cmd = a.tableInfoModel.Update(msg)
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
		if a.maximized {
			return a.renderMaximizedContent()
		}
		return a.renderSplitContent()
	case screenQueryLog:
		leftPad := lipgloss.NewStyle().PaddingLeft(1)
		return leftPad.Render(a.queryLogView.View())
	case screenCellViewer:
		return a.renderCellViewer()
	case screenGoto:
		return a.gotoModel.View()
	case screenHelp:
		return a.helpView.View()
	case screenTableInfo:
		return a.tableInfoModel.View()
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
	topH := a.topSectionOuterH()
	topRow := lipgloss.JoinHorizontal(lipgloss.Top,
		a.renderPanel("Datasources", true, a.leftOuterW(), topH, a.datasourcePicker.View()),
		a.renderPanel("", false, a.rightOuterW(), topH,
			style.Muted.Render("\n  Select a datasource\n  to browse its data.")),
	)
	sqlBox := a.renderPanel("", false, a.width, sqlPaneOuterH, "")
	return lipgloss.JoinVertical(lipgloss.Left, topRow, sqlBox)
}

func (a *App) renderSplitContent() string {
	topH := a.topSectionOuterH()

	rowBrowserTitle := "2 Row Browser"
	if a.rowBrowserReady {
		rowBrowserTitle = "2 " + a.rowBrowser.DatasetName()
	}

	topRow := lipgloss.JoinHorizontal(lipgloss.Top,
		a.renderPanel("1 Tables", a.focus == focusTables, a.leftOuterW(), topH, a.tableList.View()),
		a.renderPanel(rowBrowserTitle, a.focus == focusRowBrowser, a.rightOuterW(), topH, a.renderRightPane()),
	)
	sqlBox := a.renderPanel("3 SQL", a.focus == focusSQL, a.width, sqlPaneOuterH,
		a.sqlPane.SetFocused(a.focus == focusSQL).View())
	return lipgloss.JoinVertical(lipgloss.Left, topRow, sqlBox)
}

func (a *App) renderMaximizedContent() string {
	switch a.focus {
	case focusTables:
		return a.renderPanel("1 Tables", true, a.width, a.contentHeight(), a.tableList.View())
	case focusRowBrowser:
		title := "2 Row Browser"
		if a.rowBrowserReady {
			title = "2 " + a.rowBrowser.DatasetName()
		}
		return a.renderPanel(title, true, a.width, a.contentHeight(), a.renderRightPane())
	default:
		return a.renderSplitContent()
	}
}

func (a *App) renderRightPane() string {
	if !a.rowBrowserReady {
		return style.Muted.Render("  Select a table with ↵ or → or press 1")
	}
	return a.rowBrowser.View()
}

func (a *App) renderCellViewer() string {
	// Cell viewer owns its own border, title, content, and footer.
	// It receives the full available space and renders exactly that many lines.
	return a.cellViewer.View()
}

// renderPanel builds a rounded-border panel with the title embedded in the top border line.
func (a *App) renderPanel(title string, active bool, outerW, outerH int, content string) string {
	b := lipgloss.RoundedBorder()
	innerW := outerW - 2
	innerH := outerH - 2
	if innerW < 1 {
		innerW = 1
	}
	if innerH < 1 {
		innerH = 1
	}

	var borderSty, titleSty lipgloss.Style
	if active {
		borderSty = style.BorderStrokeActive
		titleSty = style.PanelTitleActive
	} else {
		borderSty = style.BorderStrokeInactive
		titleSty = style.PanelTitleInactive
	}

	var topLine string
	if title == "" {
		topLine = borderSty.Render(b.TopLeft + strings.Repeat(b.Top, innerW) + b.TopRight)
	} else {
		titleText := titleSty.Render(" " + title + " ")
		titleW := lipgloss.Width(titleText)
		dashTotal := innerW - titleW
		if dashTotal < 0 {
			dashTotal = 0
		}
		leftDash := 1
		rightDash := dashTotal - leftDash
		if rightDash < 0 {
			rightDash = 0
			leftDash = dashTotal
		}
		topLine = borderSty.Render(b.TopLeft+strings.Repeat(b.Top, leftDash)) +
			titleText +
			borderSty.Render(strings.Repeat(b.Top, rightDash)+b.TopRight)
	}

	bottomLine := borderSty.Render(b.BottomLeft + strings.Repeat(b.Bottom, innerW) + b.BottomRight)

	left := borderSty.Render(b.Left)
	right := borderSty.Render(b.Right)

	lines := strings.Split(strings.TrimRight(content, "\n"), "\n")
	for len(lines) < innerH {
		lines = append(lines, "")
	}
	lines = lines[:innerH]

	body := make([]string, innerH)
	for i, ln := range lines {
		pad := innerW - lipgloss.Width(ln)
		if pad > 0 {
			ln += strings.Repeat(" ", pad)
		}
		body[i] = left + ln + right
	}

	parts := make([]string, 0, 2+innerH)
	parts = append(parts, topLine)
	parts = append(parts, body...)
	parts = append(parts, bottomLine)
	return strings.Join(parts, "\n")
}

func (a *App) renderStatusBar() string {
	var runningPart string
	if a.queryLog != nil && a.queryLog.RunningCount() > 0 {
		count := a.queryLog.RunningCount()
		label := a.queryLog.CurrentLabel()
		runningPart = style.StatusKey.Render(a.appSpinner.View()) +
			style.StatusDesc.Render(fmt.Sprintf(" %d running: %s", count, label))
	}

	if a.cacheLoading {
		if runningPart != "" {
			runningPart += "  "
		}
		runningPart += style.StatusKey.Render(a.appSpinner.View()) +
			style.StatusDesc.Render(" schema loading…")
	}

	if a.screen == screenSplit && a.focus == focusRowBrowser &&
		a.rowBrowserReady && !a.rowBrowser.IsLoading() && a.rowBrowser.Err() == nil {
		return a.renderRowBrowserStatusBar(runningPart)
	}

	if a.screen == screenSplit && a.focus == focusTables {
		return a.renderTableListStatusBar(runningPart)
	}

	var bindings []key.Binding
	switch {
	case a.screen == screenCellViewer:
		bindings = []key.Binding{a.keys.Back}
	case a.screen == screenDatasourcePicker:
		bindings = []key.Binding{a.keys.Quit, a.keys.Up, a.keys.Down, a.keys.Enter}
	case a.screen == screenQueryLog:
		bindings = []key.Binding{a.keys.Up, a.keys.Down, a.keys.QueryLog, a.keys.Back}
	case a.screen == screenHelp:
		bindings = nil // footer rendered inside the HelpOverlayView
	case a.screen == screenTableInfo:
		bindings = nil // footer rendered inside the TableInfoModel.View()
	case a.screen == screenGoto:
		bindings = nil // hint is rendered inside the GotoModel.View()
	case a.focus == focusSQL:
		bindings = []key.Binding{a.keys.Quit, a.keys.Up, a.keys.Down, a.keys.SwitchFocus}
	default:
		bindings = a.keys.ShortHelp()
	}
	parts := make([]string, 0, len(bindings)+2)
	if runningPart != "" {
		parts = append(parts, runningPart)
	}
	if a.screen == screenSplit && a.maximized {
		parts = append(parts, style.StatusKey.Render("z")+style.StatusDesc.Render(" restore"))
	}
	for _, b := range bindings {
		parts = append(parts, style.StatusKey.Render(b.Help().Key)+style.StatusDesc.Render(" "+b.Help().Desc))
	}
	return style.StatusBar.Width(a.width).Render(strings.Join(parts, "  "))
}

func (a *App) renderTableListStatusBar(runningPart string) string {
	keyParts := []string{}
	if a.maximized {
		keyParts = append(keyParts, style.StatusKey.Render("z")+style.StatusDesc.Render(" restore"))
	}
	for _, b := range a.keys.TableListHelp() {
		keyParts = append(keyParts, style.StatusKey.Render(b.Help().Key)+style.StatusDesc.Render(" "+b.Help().Desc))
	}
	right := strings.Join(keyParts, "  ")

	gap := a.width - lipgloss.Width(runningPart) - lipgloss.Width(right) - 2
	if gap < 1 {
		gap = 1
	}

	content := runningPart + strings.Repeat(" ", gap) + right
	return style.StatusBar.Width(a.width).Render(content)
}

func (a *App) renderRowBrowserStatusBar(runningPart string) string {
	info := style.StatusDesc.Render(a.rowBrowser.StatusLine())

	escDesc := " pane 1"
	if a.rowBrowser.DrillDepth() > 0 {
		escDesc = " collapse"
	} else if a.maximized {
		escDesc = " restore"
	}

	keyParts := []string{}
	if a.maximized {
		keyParts = append(keyParts, style.StatusKey.Render("z")+style.StatusDesc.Render(" restore"))
	}
	keyParts = append(keyParts,
		style.StatusKey.Render("Q")+style.StatusDesc.Render(" quit"),
		style.StatusKey.Render("esc")+style.StatusDesc.Render(escDesc),
		style.StatusKey.Render("[")+style.StatusDesc.Render(" prev"),
		style.StatusKey.Render("]")+style.StatusDesc.Render(" next"),
		style.StatusKey.Render("↑↓")+style.StatusDesc.Render(" row"),
		style.StatusKey.Render("←→")+style.StatusDesc.Render(" col"),
		style.StatusKey.Render("q")+style.StatusDesc.Render(" filter"),
		style.StatusKey.Render("/")+style.StatusDesc.Render(" search"),
		style.StatusKey.Render("s")+style.StatusDesc.Render(" sort"),
		style.StatusKey.Render("C")+style.StatusDesc.Render(" cols"),
		style.StatusKey.Render("e")+style.StatusDesc.Render(" export"),
	)
	if a.rowBrowser.IsFKColumn() {
		keyParts = append(keyParts, style.StatusKey.Render("↵")+style.StatusDesc.Render(" drill"))
	}
	keyParts = append(keyParts, style.StatusKey.Render("<")+style.StatusDesc.Render(" ref by"))
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
