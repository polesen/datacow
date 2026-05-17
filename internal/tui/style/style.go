package style

import "github.com/charmbracelet/lipgloss"

var (
	colorPrimary = lipgloss.AdaptiveColor{Dark: "#7DCFFF", Light: "#0066CC"}
	colorMuted   = lipgloss.AdaptiveColor{Dark: "#565F89", Light: "#8C909E"}
	colorText    = lipgloss.AdaptiveColor{Dark: "#C0CAF5", Light: "#1A1A2E"}
	colorBg      = lipgloss.AdaptiveColor{Dark: "#1A1B26", Light: "#FAFAFA"}
	colorBorder  = lipgloss.AdaptiveColor{Dark: "#3B4261", Light: "#D0D5E3"}
	colorKey     = lipgloss.AdaptiveColor{Dark: "#BB9AF7", Light: "#6A5ACD"}
)

// headerBase is the shared foundation for all three header segment styles.
var headerBase = lipgloss.NewStyle().
	Background(colorPrimary).
	Padding(0, 1)

// HeaderTitle is the app name portion of the header.
var HeaderTitle = headerBase.Bold(true).Foreground(colorBg)

// HeaderMeta is the connection info portion of the header.
var HeaderMeta = headerBase.Foreground(colorBg)

// HeaderFill is the background fill between the title and meta segments.
var HeaderFill = lipgloss.NewStyle().Background(colorPrimary)

// Content is the main area that panels live inside.
var Content = lipgloss.NewStyle().
	Foreground(colorText)

// StatusBar is the bottom help bar.
var StatusBar = lipgloss.NewStyle().
	Foreground(colorMuted).
	Background(colorBorder).
	Padding(0, 1)

// StatusKey highlights a keybinding label.
var StatusKey = lipgloss.NewStyle().
	Foreground(colorKey).
	Bold(true)

// StatusDesc is the description text next to a key.
var StatusDesc = lipgloss.NewStyle().
	Foreground(colorMuted)

// Separator is a thin divider between header / content / status.
var Separator = lipgloss.NewStyle().
	Foreground(colorBorder)

// RowSelected is the highlighted row in a list.
var RowSelected = lipgloss.NewStyle().
	Background(colorPrimary).
	Foreground(colorBg).
	Bold(true)

// RowNormal is an unselected list row.
var RowNormal = lipgloss.NewStyle().
	Foreground(colorText)

// ColHeader styles column name cells in the row browser header.
var ColHeader = lipgloss.NewStyle().
	Foreground(colorPrimary).
	Bold(true)

// ColHeaderActive styles the first visible column (the sort target) to make it visually distinct.
var ColHeaderActive = lipgloss.NewStyle().
	Foreground(colorPrimary).
	Bold(true).
	Underline(true)

// NullValue styles NULL cell values.
var NullValue = lipgloss.NewStyle().
	Foreground(colorMuted).
	Italic(true)

// Error styles inline error messages.
var Error = lipgloss.NewStyle().Foreground(lipgloss.Color("#F7768E"))

var FilterPill = lipgloss.NewStyle().
	Foreground(colorBg).
	Background(colorKey).
	Padding(0, 1)

var FilterPillSelected = lipgloss.NewStyle().
	Foreground(colorBg).
	Background(colorPrimary).
	Bold(true).
	Padding(0, 1)

var FilterBar = lipgloss.NewStyle().
	Foreground(colorText).
	Background(colorBorder).
	Padding(0, 1)

var ExportBar = lipgloss.NewStyle().
	Foreground(colorText).
	Background(colorBorder).
	Padding(0, 1)

var Progress = lipgloss.NewStyle().Foreground(colorPrimary)

// FKColHeader styles FK column name cells - different color to show they are navigable.
var FKColHeader = lipgloss.NewStyle().
	Foreground(colorKey).
	Bold(true)

// FKColHeaderActive styles the active (cursor) FK column header.
var FKColHeaderActive = lipgloss.NewStyle().
	Foreground(colorKey).
	Bold(true).
	Underline(true)

// RowHighlight is a subtle background applied to all non-cursor cells in the selected row.
var RowHighlight = lipgloss.NewStyle().
	Background(colorBorder).
	Foreground(colorText)

// CursorCell is the exact cursor position (selected row + selected column).
var CursorCell = lipgloss.NewStyle().
	Background(colorPrimary).
	Foreground(colorBg).
	Bold(true)

// FKCell styles the FK cursor cell (cursor row + FK column) — drillable.
var FKCell = lipgloss.NewStyle().
	Background(colorKey).
	Foreground(colorBg).
	Bold(true)

// DrillSep styles the separator line between drill-down levels.
var DrillSep = lipgloss.NewStyle().
	Foreground(colorPrimary).
	Bold(true)

// PanelActive is the border style for the focused pane.
var PanelActive = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(colorPrimary)

// PanelInactive is the border style for unfocused panes.
var PanelInactive = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(colorBorder)

// PanelTitleActive styles the title embedded in the border of a focused pane.
var PanelTitleActive = lipgloss.NewStyle().
	Foreground(colorPrimary).
	Bold(true)

// PanelTitleInactive styles the title embedded in the border of an unfocused pane.
var PanelTitleInactive = lipgloss.NewStyle().
	Foreground(colorMuted)

// BorderStrokeActive colors border characters for active panels.
var BorderStrokeActive = lipgloss.NewStyle().Foreground(colorPrimary)

// BorderStrokeInactive colors border characters for inactive panels.
var BorderStrokeInactive = lipgloss.NewStyle().Foreground(colorBorder)

// Muted renders dimmed helper text.
var Muted = lipgloss.NewStyle().Foreground(colorMuted)

// QueryLabel is the dim suffix shown next to custom SQL datasets in the table list.
var QueryLabel = lipgloss.NewStyle().Foreground(colorMuted).Italic(true)

// StatusConnected styles the "connected" indicator in the datasource list.
var StatusConnected = lipgloss.NewStyle().Foreground(colorPrimary)

// GotoMatch styles characters in the goto dialog that matched the search query.
var GotoMatch = lipgloss.NewStyle().Foreground(colorKey).Bold(true)

// SearchHighlight styles matching substrings in the table list inline filter.
var SearchHighlight = lipgloss.NewStyle().Foreground(colorKey).Bold(true)
