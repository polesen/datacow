package views_test

// Acceptance tests for the Save Table View as Perspective feature.
// Each test maps to one or more acceptance criteria in tasks/ready/save-table-perspective.md.
//
// Sections:
//   CF  — Config layer (CF01–CF07)     → internal/core/config/config_test.go + dataset/resolver_test.go
//   SP  — Save-perspective overlay     → TestAC_SP01 – TestAC_SP04 (this file)
//   TL  — Table list perspectives      → TestAC_TL01 – TestAC_TL05 (this file)
//   RB  — Row browser pre-seeding      → TestAC_RB01 – TestAC_RB05 (this file)
//   AC  — App integration tests        → internal/tui/saveperspective_acceptance_test.go
//
// Coverage map (criteria in order from the task spec):
//   CF01: Load() parses perspectives                             → config_test.go: TestAC_CF01_LoadParsesPerspectives
//   CF02: Load() rejects sql+perspectives                        → config_test.go: TestAC_CF02_LoadRejectsSQLDatasetWithPerspectives
//   CF03: AppendPerspective creates file                         → config_test.go: TestAC_CF03_AppendPerspectiveCreatesFile
//   CF04: AppendPerspective appends new entry                    → config_test.go: TestAC_CF04_AppendPerspectiveAddsNewEntry
//   CF05: AppendPerspective upsert                               → config_test.go: TestAC_CF05_AppendPerspectiveUpsert
//   CF06: Save is atomic                                         → config_test.go: TestAC_CF06_SaveIsAtomic
//   CF07: Resolver emits KindPerspective entries                 → resolver_test.go: TestAC_CF07_ResolverEmitsPerspectives
//   SP01: overlay View() contains required strings               → TestAC_SP01_OverlayViewContainsRequiredStrings
//   SP02: Enter with empty name shows error                      → TestAC_SP02_EmptyNameShowsError
//   SP03: P key in keys.Map and helpoverlay                      → TestAC_SP03_PKeyMappedAndInHelp
//   SP04: P on KindPerspective opens overlay pre-filled with name → TestAC_SP04_PPrefilledOnPerspective
//   TL01: table with perspective has expand indicator            → TestAC_TL01_TableWithPerspectiveHasExpandIndicator
//   TL02: perspective name and [P] appear after expand           → TestAC_TL02_PerspectiveVisibleAfterExpand
//   TL03: filter "failed" shows parent table and perspective     → TestAC_TL03_FilterShowsPerspective
//   TL04: FocusedExpandable() false for perspective cursor       → TestAC_TL04_PerspectiveNotExpandable
//   TL05: datasetKindBadge for KindPerspective contains "P"     → TestAC_TL05_PerspectiveBadge
//   RB01: preset columns → cols N/M pill, extra column absent    → TestAC_RB01_PresetColumnsApplied
//   RB02: preset filters → filter pill visible                   → TestAC_RB02_PresetFiltersApplied
//   RB03: preset sort → sort pill with column and ↓             → TestAC_RB03_PresetSortApplied
//   RB04: P on KindPerspective opens overlay pre-filled with name → TestAC_RB04_PPrefilledOnPerspective
//   RB05: P on KindTable opens save overlay                      → TestAC_RB05_PEnabledOnTable
//   AC01: end-to-end save                                        → tui/saveperspective_acceptance_test.go: TestAC_AC01_SavePerspectiveEndToEnd
//   AC02: schema explorer refresh after save                     → tui/saveperspective_acceptance_test.go: TestAC_AC02_SchemaExplorerRefresh
//   AC03: navigate to perspective                                 → tui/saveperspective_acceptance_test.go: TestAC_AC03_NavigateToPerspective
//   AC04: P opens pre-filled overlay on perspective              → tui/saveperspective_acceptance_test.go: TestAC_AC04_PPrefilledOnPerspective
//   AC05: zero-config file creation                              → tui/saveperspective_acceptance_test.go: TestAC_AC05_ZeroConfigFileCreation

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/polesen/datacow/internal/core/dataset"
	"github.com/polesen/datacow/internal/core/db"
	"github.com/polesen/datacow/internal/core/schema"
	"github.com/polesen/datacow/internal/tui/keys"
	"github.com/polesen/datacow/internal/tui/views"
)

// --- SP: Save-perspective overlay ---

// SP01: View() of a freshly opened overlay contains the title, input area, and key hints.
func TestAC_SP01_OverlayViewContainsRequiredStrings(t *testing.T) {
	m := views.NewSavePerspectiveModel()
	m, _ = m.Focus()
	v := m.View()

	if !strings.Contains(v, "Save perspective") {
		t.Errorf("expected 'Save perspective' title in view, got:\n%s", v)
	}
	if !strings.Contains(v, "Name:") {
		t.Errorf("expected 'Name:' label in view, got:\n%s", v)
	}
	if !strings.Contains(v, "Enter confirm") {
		t.Errorf("expected 'Enter confirm' hint in view, got:\n%s", v)
	}
	if !strings.Contains(v, "Esc cancel") {
		t.Errorf("expected 'Esc cancel' hint in view, got:\n%s", v)
	}
}

// SP02: Sending Enter with an empty name input renders "name is required" and stays open.
func TestAC_SP02_EmptyNameShowsError(t *testing.T) {
	m := views.NewSavePerspectiveModel()
	m, _ = m.Focus()

	// Press Enter without typing anything.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if m.IsConfirmed() {
		t.Error("expected overlay to stay open (not confirmed) after empty Enter")
	}
	if m.IsCancelled() {
		t.Error("expected overlay to stay open (not cancelled) after empty Enter")
	}
	v := m.View()
	if !strings.Contains(v, "name is required") {
		t.Errorf("expected 'name is required' error in view, got:\n%s", v)
	}
	// Overlay must still show input and hints.
	if !strings.Contains(v, "Enter confirm") {
		t.Errorf("overlay must stay open (hints visible), got:\n%s", v)
	}
}

// SP03: P key is present in keys.Map with value "P", and in FullHelp().
func TestAC_SP03_PKeyMappedAndInHelp(t *testing.T) {
	k := keys.Default()

	bindings := k.SavePerspective.Keys()
	if len(bindings) == 0 {
		t.Fatal("SavePerspective key binding must have at least one key")
	}
	found := false
	for _, b := range bindings {
		if b == "P" {
			found = true
		}
	}
	if !found {
		t.Errorf("SavePerspective key must be 'P', got %v", bindings)
	}

	// Must appear in FullHelp.
	full := k.FullHelp()
	for _, group := range full {
		for _, b := range group {
			if b.Help().Key == k.SavePerspective.Help().Key {
				return
			}
		}
	}
	t.Error("SavePerspective must appear in FullHelp")
}

// SP04: P key on a KindPerspective dataset opens the overlay pre-filled with the perspective name.
func TestAC_SP04_PPrefilledOnPerspective(t *testing.T) {
	ds := dataset.Dataset{
		Name:        "Failed Calls",
		Table:       "api_logs",
		Kind:        dataset.KindPerspective,
		ParentTable: "api_logs",
		Preset:      &dataset.QueryOptionsPreset{},
	}
	reg := views.NewColumnRegistry()
	m := views.NewRowBrowserModelWithColumns(keys.Default(), nil, nil, ds, nil, nil, reg)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	result := makeResult(1, 1, 1,
		[]db.Column{{Name: "id"}, {Name: "result"}},
		[]map[string]any{{"id": 1, "result": 200}},
	)
	m, _ = m.Update(views.RowsLoadedMsg(result))

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'P'}})
	if !m.IsSavePerspectiveOpen() {
		t.Error("P on KindPerspective must open the save overlay")
	}
	v := m.View()
	if !strings.Contains(v, "Save perspective") {
		t.Errorf("expected 'Save perspective' overlay on P, got:\n%s", v)
	}
	if !strings.Contains(v, "Failed Calls") {
		t.Errorf("expected pre-filled name 'Failed Calls' in overlay, got:\n%s", v)
	}
}

// --- TL: Table list perspectives ---

// perspectiveDatasets returns a table + perspective pair for testing.
func perspectiveDatasets() []dataset.Dataset {
	return []dataset.Dataset{
		{Name: "api_logs", Table: "api_logs", Kind: dataset.KindTable},
		{
			Name:        "Failed Calls",
			Table:       "api_logs",
			Kind:        dataset.KindPerspective,
			ParentTable: "api_logs",
			Preset:      &dataset.QueryOptionsPreset{},
		},
	}
}

// TL01: A table with a perspective renders an expand indicator.
func TestAC_TL01_TableWithPerspectiveHasExpandIndicator(t *testing.T) {
	m := loadedTableListModel(t, nil, perspectiveDatasets())
	v := m.View()

	// "▶" should appear for the api_logs row (unexpanded table).
	if !strings.Contains(v, "▶") {
		t.Errorf("expected expand indicator '▶' in view for table with perspective, got:\n%s", v)
	}
}

// TL02: After expanding the table, the perspective name and [P] badge appear before
// column sub-lines.
func TestAC_TL02_PerspectiveVisibleAfterExpand(t *testing.T) {
	m := loadedTableListModel(t, nil, perspectiveDatasets())

	// Send Right to expand (collapsed + expandable → expand).
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})

	v := m.View()
	if !strings.Contains(v, "Failed Calls") {
		t.Errorf("expected perspective name 'Failed Calls' after expand, got:\n%s", v)
	}
	if !strings.Contains(v, "[P]") {
		t.Errorf("expected '[P]' badge after expand, got:\n%s", v)
	}
	// Perspective sub-line must appear before the Columns section.
	perspIdx := strings.Index(v, "Failed Calls")
	colsIdx := strings.Index(v, "Columns")
	if perspIdx < 0 || colsIdx < 0 {
		t.Fatalf("could not find both 'Failed Calls' (%d) and 'Columns' (%d) in:\n%s", perspIdx, colsIdx, v)
	}
	if perspIdx > colsIdx {
		t.Errorf("perspective sub-line (%d) must appear before 'Columns' section (%d)", perspIdx, colsIdx)
	}
}

// TL03: Filter query "failed" makes the parent table visible and "Failed Calls" visible.
func TestAC_TL03_FilterShowsPerspective(t *testing.T) {
	datasets := []dataset.Dataset{
		{Name: "api_logs", Table: "api_logs", Kind: dataset.KindTable},
		{
			Name:        "Failed Calls",
			Table:       "api_logs",
			Kind:        dataset.KindPerspective,
			ParentTable: "api_logs",
			Preset:      &dataset.QueryOptionsPreset{},
		},
		{Name: "users", Table: "users", Kind: dataset.KindTable},
	}
	m := loadedTableListModel(t, nil, datasets)

	// Open filter input and type "failed".
	m, _ = pressSlash(m)
	m, _ = typeText(m, "failed")

	v := m.View()
	if !strings.Contains(v, "api_logs") {
		t.Errorf("expected parent table 'api_logs' in filtered view, got:\n%s", v)
	}
	if !strings.Contains(v, "Failed Calls") {
		t.Errorf("expected matching perspective 'Failed Calls' in filtered view, got:\n%s", v)
	}
}

// TL04: When the cursor is on a perspective sub-line, FocusedExpandable() returns false.
func TestAC_TL04_PerspectiveNotExpandable(t *testing.T) {
	m := loadedTableListModel(t, nil, perspectiveDatasets())

	// Expand the parent table.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})

	// Move cursor down to the perspective sub-line.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})

	if m.FocusedExpandable() {
		t.Error("FocusedExpandable() must return false when cursor is on a KindPerspective row")
	}
}

// TL05: The rendered perspective row contains "[P]" (indirectly verifies datasetKindBadge).
func TestAC_TL05_PerspectiveBadge(t *testing.T) {
	m := loadedTableListModel(t, nil, perspectiveDatasets())

	// Expand to show the perspective row.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})

	v := m.View()
	if !strings.Contains(v, "[P]") {
		t.Errorf("expected '[P]' badge in rendered perspective row, got:\n%s", v)
	}
	if strings.Contains(v, "[P") && !strings.Contains(v, "[P]") {
		t.Errorf("badge must contain exactly '[P]', got partial match in:\n%s", v)
	}
}

// --- RB: Row browser pre-seeding ---

// makePerspectiveModel returns a RowBrowserModel for a KindPerspective dataset
// with the given preset.
func makePerspectiveModel(preset *dataset.QueryOptionsPreset) views.RowBrowserModel {
	ds := dataset.Dataset{
		Name:        "Failed Calls",
		Table:       "api_logs",
		Kind:        dataset.KindPerspective,
		ParentTable: "api_logs",
		Preset:      preset,
	}
	reg := views.NewColumnRegistry()
	m := views.NewRowBrowserModelWithColumns(keys.Default(), nil, nil, ds, nil, nil, reg)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 160, Height: 40})
	return m
}

// RB01: Preset.Columns applied → "cols N/M" pill; projected result has no "extra" header.
func TestAC_RB01_PresetColumnsApplied(t *testing.T) {
	preset := &dataset.QueryOptionsPreset{
		Columns: []string{"id", "name"},
	}
	m := makePerspectiveModel(preset)

	// First load: schema has 3 columns. Preset is applied on first RowsLoadedMsg.
	initial := makeResult(1, 1, 1,
		[]db.Column{{Name: "id"}, {Name: "name"}, {Name: "extra"}},
		[]map[string]any{{"id": 1, "name": "Alice", "extra": "x"}},
	)
	var cmd tea.Cmd
	m, cmd = m.Update(views.RowsLoadedMsg(initial))

	// A non-nil reload command must be returned so the real executor will re-fetch
	// with the projected columns. (executor is nil in this unit test so no result follows.)
	if cmd == nil {
		t.Error("expected non-nil reload command after column preset applied, got nil")
	}

	// Simulate the projected re-fetch that the real executor would deliver.
	// The "cols 2/3" pill and absent "extra" column are only visible after this second result.
	projected := makeResult(1, 1, 1,
		[]db.Column{{Name: "id"}, {Name: "name"}},
		[]map[string]any{{"id": 1, "name": "Alice"}},
	)
	m, _ = m.Update(views.RowsLoadedMsg(projected))

	v := m.View()
	if !strings.Contains(v, "cols 2/3") {
		t.Errorf("expected 'cols 2/3' pill after projected re-fetch, got:\n%s", v)
	}
	if strings.Contains(v, "extra") {
		t.Errorf("column 'extra' must be absent after projected re-fetch, got:\n%s", v)
	}
	if !strings.Contains(v, "id") || !strings.Contains(v, "name") {
		t.Errorf("expected 'id' and 'name' columns in view, got:\n%s", v)
	}
}

// RB02: Preset.Filters applied → filter pill visible with column and operator.
func TestAC_RB02_PresetFiltersApplied(t *testing.T) {
	preset := &dataset.QueryOptionsPreset{
		Filters: []dataset.Filter{
			{Column: "result", Operator: "!=", Value: 200},
		},
	}
	m := makePerspectiveModel(preset)

	result := makeResult(1, 1, 1,
		[]db.Column{{Name: "id"}, {Name: "result"}},
		[]map[string]any{{"id": 1, "result": 201}},
	)
	m, _ = m.Update(views.RowsLoadedMsg(result))

	v := m.View()
	if !strings.Contains(v, "result") {
		t.Errorf("expected 'result' in filter pill, got:\n%s", v)
	}
	if !strings.Contains(v, "!=") {
		t.Errorf("expected '!=' operator in filter pill, got:\n%s", v)
	}
	if !strings.Contains(v, "200") {
		t.Errorf("expected value '200' in filter pill, got:\n%s", v)
	}
}

// RB03: Preset.Sort applied → sort pill with column name and "↓".
func TestAC_RB03_PresetSortApplied(t *testing.T) {
	preset := &dataset.QueryOptionsPreset{
		Sort: &dataset.Sort{Column: "timestamp", Desc: true},
	}
	m := makePerspectiveModel(preset)

	result := makeResult(1, 1, 1,
		[]db.Column{{Name: "id"}, {Name: "timestamp"}},
		[]map[string]any{{"id": 1, "timestamp": "2024-01-01"}},
	)
	m, _ = m.Update(views.RowsLoadedMsg(result))

	v := m.View()
	if !strings.Contains(v, "timestamp") {
		t.Errorf("expected 'timestamp' in sort pill, got:\n%s", v)
	}
	if !strings.Contains(v, "↓") {
		t.Errorf("expected '↓' descending indicator in sort pill, got:\n%s", v)
	}
}

// RB04: P key while viewing a KindPerspective dataset opens the overlay pre-filled with the perspective name.
func TestAC_RB04_PPrefilledOnPerspective(t *testing.T) {
	preset := &dataset.QueryOptionsPreset{}
	m := makePerspectiveModel(preset)

	result := makeResult(1, 1, 1,
		[]db.Column{{Name: "id"}},
		[]map[string]any{{"id": 1}},
	)
	m, _ = m.Update(views.RowsLoadedMsg(result))

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'P'}})
	if !m.IsSavePerspectiveOpen() {
		t.Error("P on KindPerspective must open the save overlay")
	}
	v := m.View()
	if !strings.Contains(v, "Failed Calls") {
		t.Errorf("overlay must be pre-filled with perspective name 'Failed Calls', got:\n%s", v)
	}
}

// RB05: P key on a KindTable dataset opens the save overlay.
func TestAC_RB05_PEnabledOnTable(t *testing.T) {
	ds := dataset.Dataset{Name: "api_logs", Table: "api_logs", Kind: dataset.KindTable}
	reg := views.NewColumnRegistry()
	m := views.NewRowBrowserModelWithColumns(keys.Default(), nil, nil, ds, nil, nil, reg)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 160, Height: 40})

	result := makeResult(1, 1, 1,
		[]db.Column{{Name: "id"}},
		[]map[string]any{{"id": 1}},
	)
	m, _ = m.Update(views.RowsLoadedMsg(result))

	// P must open the overlay on a KindTable dataset.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'P'}})
	v := m.View()
	if !strings.Contains(v, "Save perspective") {
		t.Errorf("P on KindTable must open save overlay, got:\n%s", v)
	}
	if !m.IsSavePerspectiveOpen() {
		t.Error("IsSavePerspectiveOpen() must return true after P on KindTable")
	}
}

// Ensure HelpOverlay contains the save-perspective entry.
func TestAC_SP03_HelpOverlayContainsSavePerspective(t *testing.T) {
	ho := views.NewHelpOverlayView(keys.Default())
	ho.SetSize(120, 40)
	v := ho.View()
	if !strings.Contains(v, "save perspective") {
		t.Errorf("help overlay must contain 'save perspective', got:\n%s", v)
	}
}

// Ensure the schema cache can be used with table list for TL tests.
var _ *schema.Cache = nil
