package views

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"

	tkeys "github.com/polesen/datacow/internal/tui/keys"
	"github.com/polesen/datacow/internal/tui/style"
)

// HelpOverlayView renders the full-screen keybinding reference overlay.
type HelpOverlayView struct {
	keys        tkeys.Map
	width       int
	height      int
}

// NewHelpOverlayView constructs a HelpOverlayView with the given key map.
func NewHelpOverlayView(k tkeys.Map) HelpOverlayView {
	return HelpOverlayView{keys: k}
}

// SetSize updates the view dimensions.
func (v *HelpOverlayView) SetSize(w, h int) {
	v.width = w
	v.height = h
}

type helpGroup struct {
	title    string
	bindings []key.Binding
}

// View renders the full-screen help overlay.
func (v *HelpOverlayView) View() string {
	if v.width == 0 {
		return ""
	}

	groups := []helpGroup{
		{
			title: "Navigation",
			bindings: []key.Binding{
				v.keys.Up, v.keys.Down,
				v.keys.Left, v.keys.Right,
				v.keys.Enter, v.keys.Back,
				v.keys.PrevPage, v.keys.NextPage,
				v.keys.FirstPage, v.keys.LastPage,
				v.keys.PageSize,
			},
		},
		{
			title: "Query Filter",
			bindings: []key.Binding{
				v.keys.QueryFilter, v.keys.LocalSearch,
				v.keys.QuickFilterCell,
			},
		},
		{
			title: "Table List",
			bindings: []key.Binding{
				v.keys.TableListFilter,
			},
		},
		{
			title: "Data",
			bindings: []key.Binding{
				v.keys.Sort, v.keys.Export,
				v.keys.ViewCell, v.keys.ColumnPicker,
				v.keys.SavePerspective,
			},
		},
		{
			title: "Row Browser",
			bindings: []key.Binding{
				v.keys.DrillFwd, v.keys.DrillReverse,
			},
		},
		{
			title: "Layout",
			bindings: []key.Binding{
				v.keys.SwitchFocus, v.keys.SwitchFocusBack,
				v.keys.Pane1, v.keys.Pane2,
				v.keys.Pane3, v.keys.Maximize,
				v.keys.Goto,
			},
		},
		{
			title: "System",
			bindings: []key.Binding{
				v.keys.QueryLog, v.keys.Refresh,
				v.keys.TableInfo, v.keys.Help, v.keys.Quit,
			},
		},
	}

	colW := v.width / 2

	var sb strings.Builder
	sb.WriteString("\n ")
	sb.WriteString(style.ColHeader.Render("Keybindings"))
	sb.WriteString("\n\n")

	for _, g := range groups {
		sb.WriteString(" ")
		sb.WriteString(style.ColHeader.Render(g.title))
		sb.WriteString("\n")

		for i := 0; i < len(g.bindings); i += 2 {
			sb.WriteString(renderHelpBinding(g.bindings[i], colW))
			if i+1 < len(g.bindings) {
				sb.WriteString(renderHelpBinding(g.bindings[i+1], colW))
			}
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	sb.WriteString(style.Muted.Render("  ? or esc   close"))

	return lipgloss.NewStyle().
		Width(v.width).
		Height(v.height).
		Render(sb.String())
}

// renderHelpBinding renders a single binding padded to colW terminal columns.
func renderHelpBinding(b key.Binding, colW int) string {
	h := b.Help()
	entry := "   " + style.StatusKey.Render(h.Key) + "  " + style.StatusDesc.Render(h.Desc)
	w := lipgloss.Width(entry)
	if w < colW {
		entry += strings.Repeat(" ", colW-w)
	}
	return entry
}
