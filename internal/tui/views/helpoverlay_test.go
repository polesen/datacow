package views_test

import (
	"strings"
	"testing"

	"github.com/polesen/datacow/internal/tui/keys"
	"github.com/polesen/datacow/internal/tui/views"
)

func TestHelpOverlayView_WidthZeroRendersEmpty(t *testing.T) {
	v := views.NewHelpOverlayView(keys.Default())
	// SetSize never called — width stays 0.
	if got := v.View(); got != "" {
		t.Errorf("width=0 should render empty string, got %q", got)
	}
}

func TestHelpOverlayView_SectionHeadings(t *testing.T) {
	v := views.NewHelpOverlayView(keys.Default())
	v.SetSize(120, 40)
	out := v.View()
	for _, heading := range []string{"Keybindings", "Navigation", "Data", "Layout", "System", "Table List"} {
		if !strings.Contains(out, heading) {
			t.Errorf("help overlay missing section heading %q", heading)
		}
	}
}

func TestHelpOverlayView_TableListFilterVisible(t *testing.T) {
	v := views.NewHelpOverlayView(keys.Default())
	v.SetSize(120, 40)
	out := v.View()
	if !strings.Contains(out, "filter tables") {
		t.Error("help overlay should contain 'filter tables' entry for TableListFilter")
	}
}

func TestHelpOverlayView_KeyBindingsVisible(t *testing.T) {
	v := views.NewHelpOverlayView(keys.Default())
	v.SetSize(120, 40)
	out := v.View()
	// A sample of bindings from each section that must be present.
	for _, key := range []string{"↑", "↓", "?", "L", "z", "q"} {
		if !strings.Contains(out, key) {
			t.Errorf("help overlay missing key %q", key)
		}
	}
}

func TestHelpOverlayView_CloseInstructionVisible(t *testing.T) {
	v := views.NewHelpOverlayView(keys.Default())
	v.SetSize(120, 40)
	out := v.View()
	if !strings.Contains(out, "esc") || !strings.Contains(out, "close") {
		t.Error("help overlay must show close instruction ('esc' and 'close')")
	}
}

func TestHelpOverlayView_NarrowWidth(t *testing.T) {
	v := views.NewHelpOverlayView(keys.Default())
	v.SetSize(40, 30)
	// Should render without panicking and still show section headings.
	out := v.View()
	if !strings.Contains(out, "Navigation") {
		t.Error("narrow-width help overlay should still render section headings")
	}
}
