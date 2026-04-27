package keys

import "github.com/charmbracelet/bubbles/key"

// Map holds every keybinding used by the TUI. All key handling must go
// through this map — never hardcode key checks in view or model code.
type Map struct {
	Quit         key.Binding
	Help         key.Binding
	Filter       key.Binding
	FilterPills  key.Binding
	RemoveFilter key.Binding
	Sort         key.Binding
	Export       key.Binding
	QueryLog     key.Binding
	Maximize     key.Binding
	SwitchFocus  key.Binding
	Pane1        key.Binding
	Pane2        key.Binding
	Pane3        key.Binding
	Up           key.Binding
	Down         key.Binding
	Left         key.Binding
	Right        key.Binding
	Enter        key.Binding
	Back         key.Binding
	NextPage     key.Binding
	PrevPage     key.Binding
	ViewCell     key.Binding
	Goto         key.Binding
	Refresh      key.Binding
}

// Default returns the default keybindings.
func Default() Map {
	return Map{
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Filter: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "filter"),
		),
		FilterPills: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "select filter"),
		),
		RemoveFilter: key.NewBinding(
			key.WithKeys("x"),
			key.WithHelp("x", "remove filter"),
		),
		Sort: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "sort"),
		),
		Export: key.NewBinding(
			key.WithKeys("e"),
			key.WithHelp("e", "export"),
		),
		QueryLog: key.NewBinding(
			key.WithKeys("L"),
			key.WithHelp("L", "query log"),
		),
		Maximize: key.NewBinding(
			key.WithKeys("z"),
			key.WithHelp("z", "maximize"),
		),
		SwitchFocus: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "next pane"),
		),
		Pane1: key.NewBinding(
			key.WithKeys("1"),
			key.WithHelp("1", "tables"),
		),
		Pane2: key.NewBinding(
			key.WithKeys("2"),
			key.WithHelp("2", "browser"),
		),
		Pane3: key.NewBinding(
			key.WithKeys("3"),
			key.WithHelp("3", "sql"),
		),
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		Left: key.NewBinding(
			key.WithKeys("left", "h"),
			key.WithHelp("←/h", "back"),
		),
		Right: key.NewBinding(
			key.WithKeys("right", "l"),
			key.WithHelp("→/l", "select"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("↵", "select"),
		),
		Back: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "back"),
		),
		NextPage: key.NewBinding(
			key.WithKeys("]", "pgdown"),
			key.WithHelp("]", "next page"),
		),
		PrevPage: key.NewBinding(
			key.WithKeys("[", "pgup"),
			key.WithHelp("[", "prev page"),
		),
		ViewCell: key.NewBinding(
			key.WithKeys("v"),
			key.WithHelp("v", "view cell"),
		),
		Goto: key.NewBinding(
			key.WithKeys("ctrl+p"),
			key.WithHelp("ctrl+p", "goto"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("ctrl+r"),
			key.WithHelp("ctrl+r", "refresh schema"),
		),
	}
}

// ShortHelp returns a subset of bindings for the compact status bar.
func (m Map) ShortHelp() []key.Binding {
	return []key.Binding{m.Quit, m.Help, m.Filter, m.Sort, m.Export}
}

// TableListHelp returns bindings shown in the status bar when the table list is focused.
func (m Map) TableListHelp() []key.Binding {
	return []key.Binding{m.Quit, m.Up, m.Down, m.Enter, m.SwitchFocus}
}

// FullHelp returns all bindings grouped for a full help overlay.
func (m Map) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{m.Up, m.Down, m.Left, m.Right},
		{m.Enter, m.Back, m.Filter, m.Sort, m.Export, m.Help, m.Quit},
		{m.SwitchFocus, m.Pane1, m.Pane2, m.Pane3, m.QueryLog, m.Maximize},
	}
}
