package tui_test

// App integration acceptance tests for the SQL Dataset Editor feature.
// All tests require TEST_POSTGRES_DSN and skip when it is unset.
//
// Coverage map (AC section, from tasks/done/000032-sql-dataset-editor.md):
//   AC01: schema-explorer 'E' is a no-op (editor is row-browser-only)
//         → TestAC_AC01_SchemaExplorerEIsNoop
//   AC02: open from row browser
//         → TestAC_AC02_OpenFromRowBrowser
//   AC03: completions in context
//         → TestAC_AC03_CompletionsInContext
//   AC04: insert completion
//         → TestAC_AC04_InsertCompletion
//   AC05: save and reload
//         → TestAC_AC05_SaveAndReload
//   AC06: row browser re-fetches after save
//         → TestAC_AC06_RowBrowserRefetchesAfterSave
//   AC07: Esc reverts
//         → TestAC_AC07_EscReverts
//   AC08: Esc closes popup, not editor
//         → TestAC_AC08_EscClosesPopupNotEditor

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	teatest "github.com/charmbracelet/x/exp/teatest"

	"github.com/polesen/datacow/internal/core/config"
	"github.com/polesen/datacow/internal/core/db"
	"github.com/polesen/datacow/internal/tui"
)

const sqlEditorWait = 15 * time.Second
const sqlEditorCheckInterval = 200 * time.Millisecond

// connectAndSetupSQLEditorTable creates a temp table with two rows for the
// SQL editor tests and returns a cleanup function.
func connectAndSetupSQLEditorTable(t *testing.T, dsn, tableName string) (db.Client, func()) {
	t.Helper()
	client, err := db.Connect(dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	ctx := context.Background()
	_, _ = client.Query(ctx, "DROP TABLE IF EXISTS "+tableName)
	if _, err := client.Query(ctx, "CREATE TABLE "+tableName+" (id SERIAL PRIMARY KEY, name TEXT, label TEXT)"); err != nil {
		_ = client.Close()
		t.Fatalf("create table %s: %v", tableName, err)
	}
	if _, err := client.Query(ctx, "INSERT INTO "+tableName+" (name, label) VALUES ('alpha','x'), ('beta','y')"); err != nil {
		_ = client.Close()
		t.Fatalf("insert into %s: %v", tableName, err)
	}
	cleanup := func() {
		_, _ = client.Query(ctx, "DROP TABLE IF EXISTS "+tableName)
		_ = client.Close()
	}
	return client, cleanup
}

func writeDatasetConfig(t *testing.T, path, datasetName, sql string) {
	t.Helper()
	cfg := &config.Config{
		Datasets: []config.DatasetConfig{
			{Name: datasetName, SQL: sql},
		},
	}
	if err := config.Save(path, cfg); err != nil {
		t.Fatalf("save initial config: %v", err)
	}
}

func startAppWithDataset(t *testing.T, dsn, configPath string) *teatest.TestModel {
	t.Helper()
	client, err := db.Connect(dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	app := tui.New(tui.Config{
		ConnectionString: dsn,
		Version:          "test",
		ConfigPath:       configPath,
		ConfigDatasets:   cfg.Datasets,
	}, client, nil)
	return teatest.NewTestModel(t, app, teatest.WithInitialTermSize(160, 40))
}

// waitForSchemaReady waits until the schema cache load completes (the
// "schema loading…" status footer disappears).
func waitForSchemaReady(t *testing.T, tm *teatest.TestModel) {
	t.Helper()
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return !strings.Contains(string(bts), "schema loading")
	}, teatest.WithDuration(sqlEditorWait), teatest.WithCheckInterval(sqlEditorCheckInterval))
}

// openDatasetInRowBrowser navigates to the dataset by filtering the table
// list, then opens it in the row browser. This is the only path that can
// reach the SQL editor — the schema explorer no longer opens it directly.
func openDatasetInRowBrowser(t *testing.T, tm *teatest.TestModel, datasetName string) {
	t.Helper()
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return strings.Contains(string(bts), datasetName)
	}, teatest.WithDuration(sqlEditorWait), teatest.WithCheckInterval(sqlEditorCheckInterval))

	// Filter to the dataset row.
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	for _, r := range datasetName {
		tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter}) // close filter input
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter}) // open dataset in row browser

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return strings.Contains(string(bts), "2 "+datasetName)
	}, teatest.WithDuration(sqlEditorWait))
}

// AC01 — pressing 'E' in the schema explorer (table list focused) does NOT
// open the editor on any kind, including KindDataset. The editor is reachable
// only from the row browser.
func TestAC_AC01_SchemaExplorerEIsNoop(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set")
	}
	const tableName = "sqledit_ac01_test"
	_, cleanup := connectAndSetupSQLEditorTable(t, dsn, tableName)
	defer cleanup()

	configPath := filepath.Join(t.TempDir(), "datacow.yaml")
	const datasetName = "all_rows_ac01"
	initialSQL := "SELECT id, name FROM " + tableName
	writeDatasetConfig(t, configPath, datasetName, initialSQL)

	tm := startAppWithDataset(t, dsn, configPath)

	// Wait for the dataset row to appear, then filter to it without opening
	// the row browser — cursor sits on the dataset in the schema explorer.
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return strings.Contains(string(bts), datasetName)
	}, teatest.WithDuration(sqlEditorWait), teatest.WithCheckInterval(sqlEditorCheckInterval))
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	for _, r := range datasetName {
		tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter}) // close filter

	// Pressing E in the schema explorer must not open the editor.
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'E'}})

	// Wait a beat for any (unwanted) editor frame to render.
	tm.Send(tea.WindowSizeMsg{Width: 161, Height: 40})

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return !strings.Contains(string(bts), "Edit SQL")
	}, teatest.WithDuration(3*time.Second))

	_ = tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

// AC02 — with a KindDataset open in the row browser, 'E' opens the editor.
func TestAC_AC02_OpenFromRowBrowser(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set")
	}
	const tableName = "sqledit_ac02_test"
	_, cleanup := connectAndSetupSQLEditorTable(t, dsn, tableName)
	defer cleanup()

	configPath := filepath.Join(t.TempDir(), "datacow.yaml")
	const datasetName = "all_rows_ac02"
	initialSQL := "SELECT id, name FROM " + tableName
	writeDatasetConfig(t, configPath, datasetName, initialSQL)

	tm := startAppWithDataset(t, dsn, configPath)

	openDatasetInRowBrowser(t, tm, datasetName)

	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'E'}})
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		s := string(bts)
		return strings.Contains(s, "Edit SQL") && strings.Contains(s, "FROM "+tableName)
	}, teatest.WithDuration(sqlEditorWait))

	_ = tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

// AC03 — Tab in the editor opens a popup with the matching table name.
func TestAC_AC03_CompletionsInContext(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set")
	}
	const tableName = "sqledit_ac03_orders"
	_, cleanup := connectAndSetupSQLEditorTable(t, dsn, tableName)
	defer cleanup()

	configPath := filepath.Join(t.TempDir(), "datacow.yaml")
	const datasetName = "starter_ac03"
	const prefix = "sqledit_ac03_o"
	initialSQL := "SELECT * FROM " + prefix
	writeDatasetConfig(t, configPath, datasetName, initialSQL)

	tm := startAppWithDataset(t, dsn, configPath)

	waitForSchemaReady(t, tm)
	openDatasetInRowBrowser(t, tm, datasetName)

	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'E'}})
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return strings.Contains(string(bts), "Edit SQL")
	}, teatest.WithDuration(sqlEditorWait))

	tm.Send(tea.KeyMsg{Type: tea.KeyTab})
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return strings.Contains(string(bts), tableName)
	}, teatest.WithDuration(sqlEditorWait))

	_ = tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

// AC04 — Enter accepts the first suggestion; the editor body and the
// persisted SQL on disk both contain the full table name.
func TestAC_AC04_InsertCompletion(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set")
	}
	const tableName = "sqledit_ac04_orders"
	_, cleanup := connectAndSetupSQLEditorTable(t, dsn, tableName)
	defer cleanup()

	configPath := filepath.Join(t.TempDir(), "datacow.yaml")
	const datasetName = "starter_ac04"
	const prefix = "sqledit_ac04_o"
	initialSQL := "SELECT * FROM " + prefix
	writeDatasetConfig(t, configPath, datasetName, initialSQL)

	tm := startAppWithDataset(t, dsn, configPath)

	waitForSchemaReady(t, tm)
	openDatasetInRowBrowser(t, tm, datasetName)

	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'E'}})
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return strings.Contains(string(bts), "Edit SQL")
	}, teatest.WithDuration(sqlEditorWait))

	tm.Send(tea.KeyMsg{Type: tea.KeyTab})
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return strings.Contains(string(bts), tableName)
	}, teatest.WithDuration(sqlEditorWait))

	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlS})
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		s := string(bts)
		return !strings.Contains(s, "Edit SQL") && strings.Contains(s, "Saved to")
	}, teatest.WithDuration(sqlEditorWait))

	_ = tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))

	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	var saved string
	for _, ds := range cfg.Datasets {
		if ds.Name == datasetName {
			saved = ds.SQL
		}
	}
	if !strings.Contains(saved, tableName) {
		t.Errorf("expected saved SQL to contain full table name %q after suggestion accept, got %q", tableName, saved)
	}
}

// AC05 — edit the SQL, press Ctrl+S; overlay closes, "Saved to" appears, and
// the config file reflects the new SQL on disk.
func TestAC_AC05_SaveAndReload(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set")
	}
	const tableName = "sqledit_ac05_test"
	_, cleanup := connectAndSetupSQLEditorTable(t, dsn, tableName)
	defer cleanup()

	configPath := filepath.Join(t.TempDir(), "datacow.yaml")
	const datasetName = "all_rows_ac05"
	initialSQL := "SELECT id FROM " + tableName
	writeDatasetConfig(t, configPath, datasetName, initialSQL)

	tm := startAppWithDataset(t, dsn, configPath)

	openDatasetInRowBrowser(t, tm, datasetName)

	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'E'}})
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return strings.Contains(string(bts), "Edit SQL")
	}, teatest.WithDuration(sqlEditorWait))

	for _, r := range ", name" {
		tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlS})

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		s := string(bts)
		return !strings.Contains(s, "Edit SQL") && strings.Contains(s, "Saved to")
	}, teatest.WithDuration(sqlEditorWait))

	_ = tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))

	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	var saved string
	for _, ds := range cfg.Datasets {
		if ds.Name == datasetName {
			saved = ds.SQL
		}
	}
	if !strings.Contains(saved, "name") {
		t.Errorf("expected saved SQL to contain 'name' after edit, got %q", saved)
	}
}

// AC06 — with a KindDataset open in the row browser, edit the SQL to surface
// a new column, save, and verify the row browser re-renders with that column.
func TestAC_AC06_RowBrowserRefetchesAfterSave(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set")
	}
	const tableName = "sqledit_ac06_test"
	_, cleanup := connectAndSetupSQLEditorTable(t, dsn, tableName)
	defer cleanup()

	configPath := filepath.Join(t.TempDir(), "datacow.yaml")
	const datasetName = "all_rows_ac06"
	initialSQL := "SELECT id FROM " + tableName
	writeDatasetConfig(t, configPath, datasetName, initialSQL)

	tm := startAppWithDataset(t, dsn, configPath)

	openDatasetInRowBrowser(t, tm, datasetName)

	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'E'}})
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return strings.Contains(string(bts), "Edit SQL")
	}, teatest.WithDuration(sqlEditorWait))

	for range 80 {
		tm.Send(tea.KeyMsg{Type: tea.KeyBackspace})
	}
	for _, r := range "SELECT id, label FROM " + tableName {
		tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlS})

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		s := string(bts)
		return !strings.Contains(s, "Edit SQL") && strings.Contains(s, "label")
	}, teatest.WithDuration(sqlEditorWait))

	_ = tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

// AC07 — Esc closes the editor; the config file is unchanged and the
// dataset SQL in memory equals the original.
func TestAC_AC07_EscReverts(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set")
	}
	const tableName = "sqledit_ac07_test"
	_, cleanup := connectAndSetupSQLEditorTable(t, dsn, tableName)
	defer cleanup()

	configPath := filepath.Join(t.TempDir(), "datacow.yaml")
	const datasetName = "all_rows_ac07"
	initialSQL := "SELECT id, name FROM " + tableName
	writeDatasetConfig(t, configPath, datasetName, initialSQL)

	originalBytes, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read original config: %v", err)
	}

	tm := startAppWithDataset(t, dsn, configPath)

	openDatasetInRowBrowser(t, tm, datasetName)

	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'E'}})
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return strings.Contains(string(bts), "Edit SQL")
	}, teatest.WithDuration(sqlEditorWait))

	for _, r := range " xxx" {
		tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	tm.Send(tea.KeyMsg{Type: tea.KeyEsc})

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return !strings.Contains(string(bts), "Edit SQL")
	}, teatest.WithDuration(sqlEditorWait))

	_ = tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))

	afterBytes, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config after Esc: %v", err)
	}
	if string(afterBytes) != string(originalBytes) {
		t.Errorf("config file changed after Esc; want unchanged.\nbefore:\n%s\nafter:\n%s",
			originalBytes, afterBytes)
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("load config after Esc: %v", err)
	}
	for _, ds := range cfg.Datasets {
		if ds.Name == datasetName && ds.SQL != initialSQL {
			t.Errorf("dataset SQL was mutated after Esc: got %q, want %q", ds.SQL, initialSQL)
		}
	}
}

// AC08 — Tab opens popup; Esc closes the popup but leaves the editor open and
// the SQL text unchanged.
func TestAC_AC08_EscClosesPopupNotEditor(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set")
	}
	const tableName = "sqledit_ac08_orders"
	_, cleanup := connectAndSetupSQLEditorTable(t, dsn, tableName)
	defer cleanup()

	configPath := filepath.Join(t.TempDir(), "datacow.yaml")
	const datasetName = "starter_ac08"
	const prefix = "sqledit_ac08_o"
	initialSQL := "SELECT * FROM " + prefix
	writeDatasetConfig(t, configPath, datasetName, initialSQL)

	tm := startAppWithDataset(t, dsn, configPath)

	waitForSchemaReady(t, tm)
	openDatasetInRowBrowser(t, tm, datasetName)

	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'E'}})
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return strings.Contains(string(bts), "Edit SQL")
	}, teatest.WithDuration(sqlEditorWait))

	tm.Send(tea.KeyMsg{Type: tea.KeyTab})
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return strings.Contains(string(bts), tableName)
	}, teatest.WithDuration(sqlEditorWait))

	tm.Send(tea.KeyMsg{Type: tea.KeyEsc})

	// The editor must still be open after Esc closes the popup: confirm by
	// sending Ctrl+S — if Esc had cancelled the editor instead, this Ctrl+S
	// would fall through to the table list and produce no save status.
	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlS})
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return strings.Contains(string(bts), "Saved to")
	}, teatest.WithDuration(sqlEditorWait))

	_ = tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))

	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	for _, ds := range cfg.Datasets {
		if ds.Name == datasetName && ds.SQL != initialSQL {
			t.Errorf("Esc-after-Tab mutated the SQL: got %q, want %q", ds.SQL, initialSQL)
		}
	}
}
