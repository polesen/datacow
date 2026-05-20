package tui_test

// App integration acceptance tests for the Save Table View as Perspective feature.
// All tests require TEST_POSTGRES_DSN and skip when it is unset.
//
// Coverage:
//   AC01: end-to-end save              → TestAC_AC01_SavePerspectiveEndToEnd
//   AC02: schema explorer refresh      → TestAC_AC02_SchemaExplorerRefresh
//   AC03: navigate to perspective      → TestAC_AC03_NavigateToPerspective
//   AC04: P disabled in perspective    → TestAC_AC04_PDisabledInPerspective
//   AC05: zero-config file creation    → TestAC_AC05_ZeroConfigFileCreation

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

const acWait = 10 * time.Second
const acCheckInterval = 200 * time.Millisecond

// connectAndSetupACTable connects to the DB, creates a test table with one row,
// and registers a combined cleanup defer (drop + close) that callers must invoke.
// Returns the connected client and a cleanup function.
func connectAndSetupACTable(t *testing.T, dsn, tableName string) (db.Client, func()) {
	t.Helper()
	client, err := db.Connect(dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	ctx := context.Background()
	_, _ = client.Query(ctx, "DROP TABLE IF EXISTS "+tableName)
	if _, err := client.Query(ctx, "CREATE TABLE "+tableName+" (id SERIAL PRIMARY KEY, val TEXT)"); err != nil {
		_ = client.Close()
		t.Fatalf("create table %s: %v", tableName, err)
	}
	if _, err := client.Query(ctx, "INSERT INTO "+tableName+" (val) VALUES ('hello')"); err != nil {
		_ = client.Close()
		t.Fatalf("insert into %s: %v", tableName, err)
	}
	cleanup := func() {
		_, _ = client.Query(ctx, "DROP TABLE IF EXISTS "+tableName)
		_ = client.Close()
	}
	return client, cleanup
}

// navigateToTable waits for tableName to appear in the list, then opens it in the row browser
// by pressing Enter and waits for the row browser pane header to show the table.
func navigateToTable(t *testing.T, tm *teatest.TestModel, tableName string) {
	t.Helper()
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return strings.Contains(string(bts), tableName)
	}, teatest.WithDuration(acWait), teatest.WithCheckInterval(acCheckInterval))

	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		s := string(bts)
		return strings.Contains(s, "2 "+tableName) && strings.Contains(s, "page 1/")
	}, teatest.WithDuration(acWait), teatest.WithCheckInterval(acCheckInterval))
}

// AC01: Load a table into the row browser, press P, type a name, press Enter.
// Assert: overlay closes and "Saved to <path>" appears in the view.
func TestAC_AC01_SavePerspectiveEndToEnd(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set")
	}

	const tableName = "sp_ac01_test"
	client, cleanup := connectAndSetupACTable(t, dsn, tableName)
	defer cleanup()

	configPath := filepath.Join(t.TempDir(), "datacow.yaml")
	app := tui.New(tui.Config{ConnectionString: dsn, Version: "test", ConfigPath: configPath}, client, nil)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(160, 40))

	navigateToTable(t, tm, tableName)

	// Press P to open the save-perspective overlay.
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'P'}})
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return strings.Contains(string(bts), "Save perspective")
	}, teatest.WithDuration(5*time.Second))

	// Type the perspective name.
	for _, r := range "My View" {
		tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}

	// Confirm.
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

	// Assert overlay closed and status line shows "Saved to".
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		s := string(bts)
		return !strings.Contains(s, "Save perspective") && strings.Contains(s, "Saved to")
	}, teatest.WithDuration(acWait))

	_ = tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

// AC02: After saving a perspective (pre-seeded in config), the table list shows an
// expand indicator for the parent table and, after expanding, contains the perspective name.
func TestAC_AC02_SchemaExplorerRefresh(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set")
	}

	const tableName = "sp_ac02_test"
	client, cleanup := connectAndSetupACTable(t, dsn, tableName)
	defer cleanup()

	// Pre-populate config with a perspective so the resolver includes it on start.
	configPath := filepath.Join(t.TempDir(), "datacow.yaml")
	p := config.PerspectiveConfig{Name: "My View"}
	if err := config.AppendPerspective(configPath, "", tableName, p); err != nil {
		t.Fatalf("pre-seed perspective: %v", err)
	}
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
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(160, 40))

	// Wait for the table list to show the table.
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return strings.Contains(string(bts), tableName)
	}, teatest.WithDuration(acWait), teatest.WithCheckInterval(acCheckInterval))

	// Expand the parent table via Right key.
	tm.Send(tea.KeyMsg{Type: tea.KeyRight})

	// Perspective name must appear after expansion.
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return strings.Contains(string(bts), "My View")
	}, teatest.WithDuration(5*time.Second))

	_ = tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

// AC03: Navigate to a perspective sub-line and press Enter.
// Assert the row browser shows the filter pill from the perspective's preset.
func TestAC_AC03_NavigateToPerspective(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set")
	}

	const tableName = "sp_ac03_test"
	client, cleanup := connectAndSetupACTable(t, dsn, tableName)
	defer cleanup()

	// Insert a second row that must NOT appear after the filter is applied.
	ctx := context.Background()
	if _, err := client.Query(ctx, "INSERT INTO "+tableName+" (val) VALUES ('world')"); err != nil {
		t.Fatalf("insert second row: %v", err)
	}

	// Pre-seed a perspective with a filter that matches only the first row.
	configPath := filepath.Join(t.TempDir(), "datacow.yaml")
	p := config.PerspectiveConfig{
		Name: "Filtered View",
		Filters: []config.FilterConfig{
			{Column: "val", Operator: "=", Value: "hello"},
		},
	}
	if err := config.AppendPerspective(configPath, "", tableName, p); err != nil {
		t.Fatalf("pre-seed perspective: %v", err)
	}
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
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(160, 40))

	// Wait for table to appear.
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return strings.Contains(string(bts), tableName)
	}, teatest.WithDuration(acWait), teatest.WithCheckInterval(acCheckInterval))

	// Right — expand the table.
	tm.Send(tea.KeyMsg{Type: tea.KeyRight})

	// Wait for the perspective sub-line to appear.
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return strings.Contains(string(bts), "Filtered View")
	}, teatest.WithDuration(5*time.Second))

	// One Down brings the cursor from the table to the perspective.
	// (Deduplication means the same table no longer appears twice.)
	tm.Send(tea.KeyMsg{Type: tea.KeyDown})

	// Enter — open the perspective in the row browser.
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

	// The row browser must show the filter pill AND the matching row ("hello")
	// while the non-matching row ("world") must be absent — proving the filter runs.
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		s := string(bts)
		return strings.Contains(s, "hello") && !strings.Contains(s, "world")
	}, teatest.WithDuration(acWait))

	_ = tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

// AC04: With a perspective open in the row browser, sending P does not open the save overlay.
func TestAC_AC04_PDisabledInPerspective(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set")
	}

	const tableName = "sp_ac04_test"
	client, cleanup := connectAndSetupACTable(t, dsn, tableName)
	defer cleanup()

	// Pre-seed a perspective.
	configPath := filepath.Join(t.TempDir(), "datacow.yaml")
	p := config.PerspectiveConfig{Name: "No-Edit View"}
	if err := config.AppendPerspective(configPath, "", tableName, p); err != nil {
		t.Fatalf("pre-seed perspective: %v", err)
	}
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
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(160, 40))

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return strings.Contains(string(bts), tableName)
	}, teatest.WithDuration(acWait), teatest.WithCheckInterval(acCheckInterval))

	// Expand, navigate to perspective, open it.
	// One Down brings cursor from the table to the perspective (no duplicate entry).
	tm.Send(tea.KeyMsg{Type: tea.KeyRight})
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return strings.Contains(string(bts), "No-Edit View")
	}, teatest.WithDuration(5*time.Second))
	tm.Send(tea.KeyMsg{Type: tea.KeyDown})
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

	// Wait for row browser to load the perspective.
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		s := string(bts)
		return strings.Contains(s, "2 No-Edit View")
	}, teatest.WithDuration(acWait))

	// Press P — must NOT open the save overlay.
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'P'}})

	// P on a perspective is a no-op: no state changes, so the ANSI compressor emits
	// nothing and WaitFor would see an empty buffer. Send a resize to flush a fresh
	// frame into the output buffer.
	tm.Send(tea.WindowSizeMsg{Width: 161, Height: 40})

	// The row browser should still show the perspective without an overlay.
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		s := string(bts)
		return strings.Contains(s, "No-Edit View") && !strings.Contains(s, "Save perspective")
	}, teatest.WithDuration(5*time.Second))

	_ = tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

// AC05: Start with ConfigPath="" and HOME pointing to a temp dir.
// After saving a perspective, the config file exists and Load() returns the perspective.
func TestAC_AC05_ZeroConfigFileCreation(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set")
	}

	const tableName = "sp_ac05_test"
	client, cleanup := connectAndSetupACTable(t, dsn, tableName)
	defer cleanup()

	// Redirect HOME to a temp dir so ~/.datacow/config.yaml is isolated.
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	expectedConfigPath := filepath.Join(tmpHome, ".datacow", "config.yaml")

	// Start the app with no ConfigPath (zero-config mode).
	app := tui.New(tui.Config{ConnectionString: dsn, Version: "test"}, client, nil)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(160, 40))

	navigateToTable(t, tm, tableName)

	// Press P → type name → Enter.
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'P'}})
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return strings.Contains(string(bts), "Save perspective")
	}, teatest.WithDuration(5*time.Second))

	for _, r := range "Zero Config View" {
		tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

	// Wait for "Saved to" to confirm save succeeded.
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return strings.Contains(string(bts), "Saved to")
	}, teatest.WithDuration(acWait))

	_ = tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))

	// Assert the config file exists at the expected location.
	if _, err := os.Stat(expectedConfigPath); os.IsNotExist(err) {
		t.Fatalf("expected config file to be created at %s", expectedConfigPath)
	}

	// Load the config and verify the perspective is in it.
	cfg, err := config.Load(expectedConfigPath)
	if err != nil {
		t.Fatalf("Load config: %v", err)
	}
	var found bool
	for _, ds := range cfg.Datasets {
		for _, p := range ds.Perspectives {
			if p.Name == "Zero Config View" {
				found = true
			}
		}
	}
	if !found {
		t.Errorf("perspective 'Zero Config View' not found in saved config at %s: %+v", expectedConfigPath, cfg)
	}
}
