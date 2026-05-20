package views_test

// Coverage map:
//   SM01: View() no active sorts → contains "Available", column names, "Enter confirm", "Esc cancel"; no "Active"
//   SM02: View() with two active sorts → shows "Active", "1.", "2.", direction arrows, "Available" below divider
//   SM03: Space on available adds column to active; moves out of Available section
//   SM04: Space on active ↑ entry → ↓; and vice versa
//   SM05: Del on active removes it; remaining entries renumbered
//   SM06: J on active entry 1 swaps with entry 2
//   SM07: K on active entry 2 swaps with entry 1
//   SM08: Enter emits SortConfirmedMsg with correct []Sort in order
//   SM09: Esc does not emit SortConfirmedMsg; model reports no changes applied
//   SM10: s and S keys present in keys.Map, S in helpoverlay.go
//   SM11: S always opens sort manager (even with no active sort)

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/polesen/datacow/internal/core/dataset"
	"github.com/polesen/datacow/internal/core/db"
	"github.com/polesen/datacow/internal/tui/keys"
	"github.com/polesen/datacow/internal/tui/views"
)

func newSortManager(active []dataset.Sort, cols []string) views.SortManagerModel {
	return views.NewSortManagerModel(active, cols, 80, 24)
}

// SM01: freshly opened overlay with no active sorts.
func TestAC_SM01_NoActiveSorts(t *testing.T) {
	m := newSortManager(nil, []string{"id", "name", "status"})
	v := m.View()

	if !strings.Contains(v, "Available") {
		t.Errorf("expected 'Available' in view, got:\n%s", v)
	}
	for _, col := range []string{"id", "name", "status"} {
		if !strings.Contains(v, col) {
			t.Errorf("expected column %q in view, got:\n%s", col, v)
		}
	}
	if !strings.Contains(v, "Enter confirm") {
		t.Errorf("expected 'Enter confirm' hint, got:\n%s", v)
	}
	if !strings.Contains(v, "Esc cancel") {
		t.Errorf("expected 'Esc cancel' hint, got:\n%s", v)
	}
	if strings.Contains(v, "Active") {
		t.Errorf("expected 'Active' section header to be absent, got:\n%s", v)
	}
}

// SM02: two active sorts → Active section with numbered entries and arrows.
func TestAC_SM02_TwoActiveSorts(t *testing.T) {
	active := []dataset.Sort{
		{Column: "name", Desc: false},
		{Column: "created_at", Desc: true},
	}
	m := newSortManager(active, []string{"id", "name", "created_at", "status"})
	v := m.View()

	if !strings.Contains(v, "Active") {
		t.Errorf("expected 'Active' section, got:\n%s", v)
	}
	if !strings.Contains(v, "1.") {
		t.Errorf("expected '1.' in active section, got:\n%s", v)
	}
	if !strings.Contains(v, "2.") {
		t.Errorf("expected '2.' in active section, got:\n%s", v)
	}
	if !strings.Contains(v, "name") {
		t.Errorf("expected 'name' in active section, got:\n%s", v)
	}
	if !strings.Contains(v, "created_at") {
		t.Errorf("expected 'created_at' in active section, got:\n%s", v)
	}
	if !strings.Contains(v, "↑") {
		t.Errorf("expected '↑' for ASC entry, got:\n%s", v)
	}
	if !strings.Contains(v, "↓") {
		t.Errorf("expected '↓' for DESC entry, got:\n%s", v)
	}
	if !strings.Contains(v, "Available") {
		t.Errorf("expected 'Available' section, got:\n%s", v)
	}
}

// SM03: Space on available column adds it to active.
func TestAC_SM03_SpaceAddsAvailableColumn(t *testing.T) {
	m := newSortManager(nil, []string{"id", "name", "status"})
	// Cursor starts at 0 (first available = "id").
	m, _, _ = m.HandleKey(" ")
	v := m.View()

	if !strings.Contains(v, "1. ") {
		t.Errorf("expected '1. ' in active section after space, got:\n%s", v)
	}
	if !strings.Contains(v, "id") {
		t.Errorf("expected 'id' in view, got:\n%s", v)
	}
}

// SM04: Space on active ↑ toggles to ↓, and vice versa.
func TestAC_SM04_SpaceTogglesDirection(t *testing.T) {
	active := []dataset.Sort{{Column: "name", Desc: false}}
	m := newSortManager(active, []string{"id", "name", "status"})
	// cursor=0 is on the active entry "name ↑".
	v0 := m.View()
	if !strings.Contains(v0, "↑") {
		t.Fatalf("expected ↑ initially, got:\n%s", v0)
	}

	m, _, _ = m.HandleKey(" ")
	v1 := m.View()
	if !strings.Contains(v1, "↓") {
		t.Errorf("expected ↓ after space toggle, got:\n%s", v1)
	}

	m, _, _ = m.HandleKey(" ")
	v2 := m.View()
	if !strings.Contains(v2, "↑") {
		t.Errorf("expected ↑ after toggle back, got:\n%s", v2)
	}
}

// SM05: Del on active removes it; remaining entries renumbered.
func TestAC_SM05_DelRemovesActiveEntry(t *testing.T) {
	active := []dataset.Sort{
		{Column: "name", Desc: false},
		{Column: "id", Desc: true},
	}
	m := newSortManager(active, []string{"id", "name", "status"})
	// cursor=0 → delete "name".
	m, _, _ = m.HandleKey("delete")
	v := m.View()

	// "name" should no longer be in the active section (it moves to available).
	// The remaining entry "id" should now be numbered "1.".
	if !strings.Contains(v, "1. ") {
		t.Errorf("expected '1. ' after removal, got:\n%s", v)
	}
	if strings.Contains(v, "2. ") {
		t.Errorf("expected '2.' to be absent after removal, got:\n%s", v)
	}
}

// SM06: J on active entry 1 swaps it with entry 2.
func TestAC_SM06_JSwapsDown(t *testing.T) {
	active := []dataset.Sort{
		{Column: "name", Desc: false},
		{Column: "id", Desc: false},
	}
	m := newSortManager(active, []string{"id", "name", "status"})
	// cursor=0 (name), press J → name becomes 2nd, id becomes 1st.
	m, _, _ = m.HandleKey("J")

	result := m.Result()
	if len(result) != 2 {
		t.Fatalf("expected 2 active sorts, got %d", len(result))
	}
	if result[0].Column != "id" || result[1].Column != "name" {
		t.Errorf("expected [id, name] after J, got %v", result)
	}
}

// SM07: K on active entry 2 swaps it with entry 1.
func TestAC_SM07_KSwapsUp(t *testing.T) {
	active := []dataset.Sort{
		{Column: "name", Desc: false},
		{Column: "id", Desc: false},
	}
	m := newSortManager(active, []string{"id", "name", "status"})
	// Navigate to entry 2 (cursor=1), press K.
	m, _, _ = m.HandleKey("down")
	m, _, _ = m.HandleKey("K")

	result := m.Result()
	if len(result) != 2 {
		t.Fatalf("expected 2 active sorts, got %d", len(result))
	}
	if result[0].Column != "id" || result[1].Column != "name" {
		t.Errorf("expected [id, name] after K, got %v", result)
	}
}

// SM08: Enter emits SortConfirmedMsg with the correct slice.
func TestAC_SM08_EnterEmitsConfirmedMsg(t *testing.T) {
	active := []dataset.Sort{
		{Column: "name", Desc: false},
		{Column: "id", Desc: true},
	}
	m := newSortManager(active, []string{"id", "name", "status"})
	_, confirmed, emitted := m.HandleKey("enter")

	if !emitted {
		t.Fatal("expected SortConfirmedMsg to be emitted on Enter")
	}
	if len(confirmed.Sort) != 2 {
		t.Fatalf("expected 2 sorts, got %d", len(confirmed.Sort))
	}
	if confirmed.Sort[0].Column != "name" || confirmed.Sort[0].Desc {
		t.Errorf("sort[0]: expected name ASC, got %+v", confirmed.Sort[0])
	}
	if confirmed.Sort[1].Column != "id" || !confirmed.Sort[1].Desc {
		t.Errorf("sort[1]: expected id DESC, got %+v", confirmed.Sort[1])
	}
}

// SM09: Esc does not emit SortConfirmedMsg; model is cancelled.
func TestAC_SM09_EscDoesNotConfirm(t *testing.T) {
	m := newSortManager([]dataset.Sort{{Column: "name", Desc: false}}, []string{"id", "name"})
	updated, _, emitted := m.HandleKey("esc")

	if emitted {
		t.Error("expected no SortConfirmedMsg on Esc")
	}
	if !updated.IsCancelled() {
		t.Error("expected model to report cancelled after Esc")
	}
	if updated.IsConfirmed() {
		t.Error("expected model not to be confirmed after Esc")
	}
}

// SM10: s and S keys present in keys.Map; S in FullHelp.
func TestAC_SM10_SAndUpperSInKeys(t *testing.T) {
	k := keys.Default()

	// s key (Sort)
	sortKeys := k.Sort.Keys()
	found := false
	for _, key := range sortKeys {
		if key == "s" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 's' in Sort keys, got %v", sortKeys)
	}

	// S key (SortManager)
	smKeys := k.SortManager.Keys()
	found = false
	for _, key := range smKeys {
		if key == "S" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'S' in SortManager keys, got %v", smKeys)
	}

	// S must appear in FullHelp.
	fullHelp := k.FullHelp()
	smFound := false
	for _, group := range fullHelp {
		for _, b := range group {
			if b.Help().Key == "S" {
				smFound = true
			}
		}
	}
	if !smFound {
		t.Error("expected S binding in FullHelp()")
	}
}

// SM11: S (uppercase) always opens sort manager, even with no active sort.
func TestAC_SM11_UpperSAlwaysOpensSortManager(t *testing.T) {
	ds := dataset.Dataset{Name: "users", Table: "users"}
	m := newModel(ds)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	result := makeResult(1, 1, 1,
		[]db.Column{{Name: "id"}, {Name: "name"}},
		[]map[string]any{{"id": int64(1), "name": "Alice"}},
	)
	m, _ = m.Update(views.RowsLoadedMsg(result))

	// Press S with no sort active.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'S'}})
	v := m.View()

	if !strings.Contains(v, "Sort") {
		t.Errorf("expected sort manager overlay after S, got:\n%s", v)
	}
	if !strings.Contains(v, "Available") {
		t.Errorf("expected Available section in sort manager, got:\n%s", v)
	}
}
