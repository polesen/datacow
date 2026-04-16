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
var HeaderFill = headerBase

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

// NullValue styles NULL cell values.
var NullValue = lipgloss.NewStyle().
	Foreground(colorMuted).
	Italic(true)
