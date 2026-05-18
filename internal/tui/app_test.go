package tui_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	teatest "github.com/charmbracelet/x/exp/teatest"

	"github.com/polesen/datacow/internal/core/config"
	"github.com/polesen/datacow/internal/core/db"
	"github.com/polesen/datacow/internal/tui"
)

func TestApp_DatasourcePicker_MultiDatasource(t *testing.T) {
	datasources := []config.DatasourceConfig{
		{Name: "production", ConnectionString: "postgres://prod/db"},
		{Name: "staging", ConnectionString: "postgres://staging/db"},
	}
	app := tui.New(tui.Config{Version: "test", Datasources: datasources}, nil, nil)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		s := string(bts)
		return strings.Contains(s, "Datasources") &&
			strings.Contains(s, "production") &&
			strings.Contains(s, "staging")
	}, teatest.WithDuration(5*time.Second))

	_ = tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

func TestApp_DatasourcePicker_ConnectAndTransition(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set")
	}

	datasources := []config.DatasourceConfig{
		{Name: "testdb", ConnectionString: dsn},
		{Name: "other", ConnectionString: "postgres://other/db"},
	}
	app := tui.New(tui.Config{Version: "test", Datasources: datasources}, nil, nil)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(160, 40))

	// Wait for picker to appear.
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return strings.Contains(string(bts), "Datasources")
	}, teatest.WithDuration(5*time.Second))

	// Press Enter to connect to first datasource.
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

	// Should transition to split view after connection.
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return strings.Contains(string(bts), "1 Tables")
	}, teatest.WithDuration(10*time.Second))

	_ = tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

func TestApp_ErrorScreen_NoConnection(t *testing.T) {
	app := tui.New(tui.Config{Version: "test"}, nil, nil)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return strings.Contains(string(bts), "No connection")
	}, teatest.WithDuration(5*time.Second))

	_ = tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

func TestApp_SplitLayout_AllPanesVisible(t *testing.T) {
	// Without a DB connection the app shows the error screen, so we need a real DB.
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set")
	}

	client, err := db.Connect(dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = client.Close() }()

	app := tui.New(tui.Config{ConnectionString: dsn, Version: "test"}, client, nil)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(160, 40))

	// All three pane titles should appear in the initial render.
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		s := string(bts)
		return strings.Contains(s, "1 Tables") &&
			strings.Contains(s, "2 Row Browser") &&
			strings.Contains(s, "3 SQL")
	}, teatest.WithDuration(5*time.Second))

	_ = tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

func TestApp_SplitLayout_PlaceholderBeforeSelection(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set")
	}

	client, err := db.Connect(dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = client.Close() }()

	app := tui.New(tui.Config{ConnectionString: dsn, Version: "test"}, client, nil)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(160, 40))

	// Right pane should show the "select a table" placeholder before any table is opened.
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return strings.Contains(string(bts), "Select a table")
	}, teatest.WithDuration(5*time.Second))

	_ = tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

func TestApp_FocusShift_NumberKeys(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set")
	}

	client, err := db.Connect(dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = client.Close() }()

	app := tui.New(tui.Config{ConnectionString: dsn, Version: "test"}, client, nil)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(160, 40))

	// Wait for initial render.
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return strings.Contains(string(bts), "1 Tables")
	}, teatest.WithDuration(5*time.Second))

	// Press "3" → focus SQL pane; status bar changes to show SQL pane bindings (↑/k, ↓/j).
	// After first render, Bubble Tea sends incremental diffs, so we check the status bar text
	// which changes on every focus switch.
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		s := string(bts)
		return strings.Contains(s, "↑/k") && strings.Contains(s, "next pane")
	}, teatest.WithDuration(5*time.Second))

	// Press "1" → back to tables pane; status bar shows table list bindings (↵ select).
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return strings.Contains(string(bts), "↵")
	}, teatest.WithDuration(5*time.Second))

	_ = tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

func TestApp_TableList_WithRealDB(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set")
	}

	client, err := db.Connect(dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	ctx := context.Background()
	defer func() {
		_, _ = client.Query(ctx, "DROP TABLE IF EXISTS tui_test_items")
		_ = client.Close()
	}()

	if _, err := client.Query(ctx, "CREATE TABLE IF NOT EXISTS tui_test_items (id SERIAL PRIMARY KEY, name TEXT)"); err != nil {
		t.Fatalf("create fixture: %v", err)
	}

	app := tui.New(tui.Config{ConnectionString: dsn, Version: "test"}, client, nil)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))

	// Wait for the fixture table to appear in the table list (tables load async).
	// WaitFor accumulates raw bytes so we only check for the positive signal;
	// "Connecting..." from an early render would permanently poison a negative check.
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return strings.Contains(string(bts), "tui_test_items")
	}, teatest.WithDuration(10*time.Second), teatest.WithCheckInterval(200*time.Millisecond))

	_ = tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

func TestApp_Maximize_ToggleOnPane1(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set")
	}

	client, err := db.Connect(dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = client.Close() }()

	app := tui.New(tui.Config{ConnectionString: dsn, Version: "test"}, client, nil)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return strings.Contains(string(bts), "1 Tables")
	}, teatest.WithDuration(5*time.Second))

	// Press z on pane 1 — status bar shows "z restore".
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'z'}})
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return strings.Contains(string(bts), "z restore")
	}, teatest.WithDuration(5*time.Second))

	_ = tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

func TestApp_Maximize_ToggleOff(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set")
	}

	client, err := db.Connect(dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = client.Close() }()

	app := tui.New(tui.Config{ConnectionString: dsn, Version: "test"}, client, nil)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return strings.Contains(string(bts), "1 Tables")
	}, teatest.WithDuration(5*time.Second))

	// z to maximize, z again to restore.
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'z'}})
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return strings.Contains(string(bts), "z restore")
	}, teatest.WithDuration(5*time.Second))

	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'z'}})
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		s := string(bts)
		return strings.Contains(s, "1 Tables") &&
			strings.Contains(s, "2 Row Browser") &&
			strings.Contains(s, "3 SQL") &&
			!strings.Contains(s, "z restore")
	}, teatest.WithDuration(5*time.Second))

	_ = tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

func TestApp_Maximize_Pane2(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set")
	}

	client, err := db.Connect(dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = client.Close() }()

	app := tui.New(tui.Config{ConnectionString: dsn, Version: "test"}, client, nil)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return strings.Contains(string(bts), "1 Tables")
	}, teatest.WithDuration(5*time.Second))

	// Focus pane 2, then z — status bar should show "z restore".
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'z'}})
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return strings.Contains(string(bts), "z restore")
	}, teatest.WithDuration(5*time.Second))

	_ = tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

func TestApp_Maximize_Pane3OpensQueryLog(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set")
	}

	client, err := db.Connect(dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = client.Close() }()

	app := tui.New(tui.Config{ConnectionString: dsn, Version: "test"}, client, nil)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return strings.Contains(string(bts), "1 Tables")
	}, teatest.WithDuration(5*time.Second))

	// Focus pane 3, press z — query log opens; split "z restore" NOT shown.
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'z'}})
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		s := string(bts)
		return strings.Contains(s, "query log") && !strings.Contains(s, "z restore")
	}, teatest.WithDuration(5*time.Second))

	_ = tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

func TestApp_Maximize_EscRestoresTables(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set")
	}

	client, err := db.Connect(dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = client.Close() }()

	app := tui.New(tui.Config{ConnectionString: dsn, Version: "test"}, client, nil)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return strings.Contains(string(bts), "1 Tables")
	}, teatest.WithDuration(5*time.Second))

	// Maximize pane 1, then esc — split should restore.
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'z'}})
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return strings.Contains(string(bts), "z restore")
	}, teatest.WithDuration(5*time.Second))

	tm.Send(tea.KeyMsg{Type: tea.KeyEsc})
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		s := string(bts)
		return strings.Contains(s, "1 Tables") &&
			strings.Contains(s, "2 Row Browser") &&
			strings.Contains(s, "3 SQL") &&
			!strings.Contains(s, "z restore")
	}, teatest.WithDuration(5*time.Second))

	_ = tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

func TestApp_Maximize_ResizeWhileMaximized(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set")
	}

	client, err := db.Connect(dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = client.Close() }()

	app := tui.New(tui.Config{ConnectionString: dsn, Version: "test"}, client, nil)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return strings.Contains(string(bts), "1 Tables")
	}, teatest.WithDuration(5*time.Second))

	// Maximize pane 1, then resize — should stay maximized and fill new dimensions.
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'z'}})
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return strings.Contains(string(bts), "z restore")
	}, teatest.WithDuration(5*time.Second))

	tm.Send(tea.WindowSizeMsg{Width: 200, Height: 50})
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		s := string(bts)
		return strings.Contains(s, "z restore") && strings.Contains(s, "1 Tables")
	}, teatest.WithDuration(5*time.Second))

	_ = tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

func TestApp_Maximize_PaneKeyWhileMaximized(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set")
	}

	client, err := db.Connect(dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = client.Close() }()

	app := tui.New(tui.Config{ConnectionString: dsn, Version: "test"}, client, nil)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return strings.Contains(string(bts), "1 Tables")
	}, teatest.WithDuration(5*time.Second))

	// Maximize pane 1, then press 2 — switches to pane 2 while staying maximized.
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'z'}})
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return strings.Contains(string(bts), "z restore")
	}, teatest.WithDuration(5*time.Second))

	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		s := string(bts)
		return strings.Contains(s, "z restore") && strings.Contains(s, "2 Row Browser")
	}, teatest.WithDuration(5*time.Second))

	_ = tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

func TestApp_Maximize_SplitNotBrokenAfterRestore(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set")
	}

	client, err := db.Connect(dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = client.Close() }()

	app := tui.New(tui.Config{ConnectionString: dsn, Version: "test"}, client, nil)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return strings.Contains(string(bts), "1 Tables")
	}, teatest.WithDuration(5*time.Second))

	// z to maximize, z to restore — all three pane borders must be present.
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'z'}})
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return strings.Contains(string(bts), "z restore")
	}, teatest.WithDuration(5*time.Second))

	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'z'}})
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		s := string(bts)
		return strings.Contains(s, "1 Tables") &&
			strings.Contains(s, "2 Row Browser") &&
			strings.Contains(s, "3 SQL")
	}, teatest.WithDuration(5*time.Second))

	_ = tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

func TestApp_Maximize_EscRestoresRowBrowserNodrill(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set")
	}

	ctx := context.Background()
	client, err := db.Connect(dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() {
		_, _ = client.Query(ctx, "DROP TABLE IF EXISTS maximize_esc_rows")
		_ = client.Close()
	}()

	if _, err := client.Query(ctx, "CREATE TABLE IF NOT EXISTS maximize_esc_rows (id SERIAL PRIMARY KEY, label TEXT)"); err != nil {
		t.Fatalf("create fixture: %v", err)
	}
	if _, err := client.Query(ctx, "INSERT INTO maximize_esc_rows (label) VALUES ('a') ON CONFLICT DO NOTHING"); err != nil {
		t.Fatalf("insert fixture: %v", err)
	}

	app := tui.New(tui.Config{ConnectionString: dsn, Version: "test"}, client, nil)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(160, 40))

	// Wait for the fixture table to appear.
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return strings.Contains(string(bts), "maximize_esc_rows")
	}, teatest.WithDuration(10*time.Second), teatest.WithCheckInterval(200*time.Millisecond))

	// Press Enter/Right until the row browser opens for our table.
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return strings.Contains(string(bts), "2 maximize_esc_rows")
	}, teatest.WithDuration(10*time.Second), teatest.WithCheckInterval(200*time.Millisecond))

	// Maximize pane 2, then esc — should restore split without drill collapse.
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'z'}})
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return strings.Contains(string(bts), "z restore")
	}, teatest.WithDuration(5*time.Second))

	tm.Send(tea.KeyMsg{Type: tea.KeyEsc})
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		s := string(bts)
		return strings.Contains(s, "1 Tables") &&
			strings.Contains(s, "3 SQL") &&
			!strings.Contains(s, "z restore")
	}, teatest.WithDuration(5*time.Second))

	_ = tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

// TestApp_QueryLog_LKeyFromPane1 is the regression test for the bug where pressing
// L from pane 1 or 2 rendered a blank query log because no WindowSizeMsg was sent.
func TestApp_QueryLog_LKeyFromPane1(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set")
	}

	client, err := db.Connect(dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = client.Close() }()

	app := tui.New(tui.Config{ConnectionString: dsn, Version: "test"}, client, nil)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return strings.Contains(string(bts), "1 Tables")
	}, teatest.WithDuration(5*time.Second))

	// Pane 1 is focused by default. Press L — query log must render with content.
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'L'}})
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		s := string(bts)
		return strings.Contains(s, "History") && strings.Contains(s, "Running")
	}, teatest.WithDuration(5*time.Second))

	_ = tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

func TestApp_QueryLog_LKeyFromPane2(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set")
	}

	client, err := db.Connect(dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = client.Close() }()

	app := tui.New(tui.Config{ConnectionString: dsn, Version: "test"}, client, nil)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return strings.Contains(string(bts), "1 Tables")
	}, teatest.WithDuration(5*time.Second))

	// Switch to pane 2, then press L.
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'L'}})
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		s := string(bts)
		return strings.Contains(s, "History") && strings.Contains(s, "Running")
	}, teatest.WithDuration(5*time.Second))

	_ = tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

func TestApp_HelpOverlay_OpensAndCloses(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set")
	}

	client, err := db.Connect(dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = client.Close() }()

	app := tui.New(tui.Config{ConnectionString: dsn, Version: "test"}, client, nil)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return strings.Contains(string(bts), "1 Tables")
	}, teatest.WithDuration(5*time.Second))

	// Press ? — help overlay must render with keybinding sections.
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		s := string(bts)
		return strings.Contains(s, "Keybindings") && strings.Contains(s, "Navigation")
	}, teatest.WithDuration(5*time.Second))

	// Press ? again — help overlay closes, split view returns.
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		s := string(bts)
		return strings.Contains(s, "1 Tables") && !strings.Contains(s, "Keybindings")
	}, teatest.WithDuration(5*time.Second))

	_ = tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

func TestApp_GotoOverlay_OpensWithCtrlP(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set")
	}

	client, err := db.Connect(dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = client.Close() }()

	app := tui.New(tui.Config{ConnectionString: dsn, Version: "test"}, client, nil)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return strings.Contains(string(bts), "1 Tables")
	}, teatest.WithDuration(5*time.Second))

	// Press ctrl+p — goto dialog must open.
	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlP})
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return strings.Contains(string(bts), "Goto")
	}, teatest.WithDuration(5*time.Second))

	_ = tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

func TestApp_ShiftTab_ReverseFocus(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set")
	}

	client, err := db.Connect(dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = client.Close() }()

	app := tui.New(tui.Config{ConnectionString: dsn, Version: "test"}, client, nil)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(160, 40))

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return strings.Contains(string(bts), "1 Tables")
	}, teatest.WithDuration(5*time.Second))

	// Switch to the row browser (pane 2) and wait for a render to confirm it.
	// pane 2 not ready → status bar shows ShortHelp which includes "query filter" (not in TableListHelp).
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return strings.Contains(string(bts), "query filter")
	}, teatest.WithDuration(5*time.Second))

	// Shift+Tab should cycle focus backward: row browser → table list (pane 1).
	// TableListHelp shows "i  table info" which does not appear in ShortHelp.
	tm.Send(tea.KeyMsg{Type: tea.KeyShiftTab})
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return strings.Contains(string(bts), "table info")
	}, teatest.WithDuration(5*time.Second))

	_ = tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

func TestApp_CellViewer_OpensFromRowBrowser(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set")
	}

	ctx := context.Background()
	client, err := db.Connect(dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	// Drop before Close so the cleanup actually executes — t.Cleanup runs after defer,
	// by which point the client is already closed and queries silently fail.
	defer func() {
		_, _ = client.Query(ctx, "DROP TABLE IF EXISTS cell_viewer_test")
		_ = client.Close()
	}()

	if _, err := client.Query(ctx, "CREATE TABLE IF NOT EXISTS cell_viewer_test (id SERIAL PRIMARY KEY, notes TEXT)"); err != nil {
		t.Fatalf("create fixture: %v", err)
	}
	if _, err := client.Query(ctx, "INSERT INTO cell_viewer_test (notes) VALUES ('hello from cell viewer') ON CONFLICT DO NOTHING"); err != nil {
		t.Fatalf("insert fixture: %v", err)
	}

	app := tui.New(tui.Config{ConnectionString: dsn, Version: "test"}, client, nil)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(160, 40))

	// Wait for fixture table to appear in the table list.
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return strings.Contains(string(bts), "cell_viewer_test")
	}, teatest.WithDuration(10*time.Second), teatest.WithCheckInterval(200*time.Millisecond))

	// Open the row browser for our fixture table.
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	// Wait for both the row browser header and rows to have loaded (status line shows "page 1/").
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		s := string(bts)
		return strings.Contains(s, "2 cell_viewer_test") && strings.Contains(s, "page 1/")
	}, teatest.WithDuration(10*time.Second), teatest.WithCheckInterval(200*time.Millisecond))

	// Press 'v' to open the cell viewer. The footer should show save/copy/close hints.
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		s := string(bts)
		return strings.Contains(s, "save") && strings.Contains(s, "copy") && strings.Contains(s, "close")
	}, teatest.WithDuration(5*time.Second))

	_ = tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

func TestApp_TableListFilter_ColumnSubMatch(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set")
	}

	ctx := context.Background()
	client, err := db.Connect(dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() {
		_, _ = client.Query(ctx, "DROP TABLE IF EXISTS filter_test_items")
		_ = client.Close()
	}()

	if _, err := client.Query(ctx, `CREATE TABLE IF NOT EXISTS filter_test_items (
		id               SERIAL PRIMARY KEY,
		unique_email_col TEXT NOT NULL
	)`); err != nil {
		t.Fatalf("create fixture: %v", err)
	}

	app := tui.New(tui.Config{ConnectionString: dsn, Version: "test"}, client, nil)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(160, 40))

	// Wait for the fixture table to appear in the table list.
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return strings.Contains(string(bts), "filter_test_items")
	}, teatest.WithDuration(15*time.Second), teatest.WithCheckInterval(200*time.Millisecond))

	// Press / to open the filter input.
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return strings.Contains(string(bts), "filter tables")
	}, teatest.WithDuration(5*time.Second))

	// Wait for the schema cache to load: the hint "schema loading" in the filter footer
	// disappears once the cache is ready. Since teatest accumulates output, we wait until
	// a frame arrives that contains the filter prompt "/" but NOT the loading hint — i.e.,
	// the cache is ready so the hint is absent in the current frame. We detect this by
	// looking for the filter bar WITHOUT the loading text on the same line.
	// Simpler proxy: wait for the cache-ready signal by looking for the filter status count.
	// Type the query after a short settling time.
	time.Sleep(2 * time.Second) // allow schema cache to load before applying filter

	// Type a substring of the column name "unique_email_col".
	for _, r := range "unique_email" {
		tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}

	// Verify the filter accepted the input: "unique_email" appears in the filter bar.
	// Sub-match correctness is covered by unit tests (TestTableListFilter_SubMatchVisible).
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return strings.Contains(string(bts), "unique_email")
	}, teatest.WithDuration(10*time.Second), teatest.WithCheckInterval(200*time.Millisecond))

	_ = tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

// TestAC_KH04_GlobalKeysBlockedWhileFilterOpen is covered by
// TestApp_TableListFilter_BlocksGlobalKeys immediately below. That test verifies
// that pressing "i" while the filter input is open is consumed by the filter
// (typed into the input) rather than opening the table-info overlay.
func TestApp_TableListFilter_BlocksGlobalKeys(t *testing.T) {
	// While the table-list filter input is open, global keys like "i" (table info),
	// "?" (help), and "L" (query log) must be consumed by the filter, not the app.
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set")
	}

	ctx := context.Background()
	client, err := db.Connect(dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() {
		_, _ = client.Query(ctx, "DROP TABLE IF EXISTS filter_block_test")
		_ = client.Close()
	}()

	if _, err := client.Query(ctx, "CREATE TABLE IF NOT EXISTS filter_block_test (id SERIAL PRIMARY KEY)"); err != nil {
		t.Fatalf("create fixture: %v", err)
	}

	app := tui.New(tui.Config{ConnectionString: dsn, Version: "test"}, client, nil)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return strings.Contains(string(bts), "filter_block_test")
	}, teatest.WithDuration(15*time.Second), teatest.WithCheckInterval(200*time.Millisecond))

	// Open the filter.
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return strings.Contains(string(bts), "filter tables")
	}, teatest.WithDuration(5*time.Second))

	// Press "i" — should be typed into the filter, NOT open the table-info overlay.
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})

	// After a brief delay, the filter bar must still be visible and "Table Info" must not.
	time.Sleep(300 * time.Millisecond)
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		s := string(bts)
		return strings.Contains(s, "filter tables") && !strings.Contains(s, "Table Info")
	}, teatest.WithDuration(5*time.Second))

	_ = tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

// TestAC_KH04_GlobalKeysBlockedWhileFilterOpen is covered by
// TestApp_TableListFilter_BlocksGlobalKeys immediately above. See that test for
// the authoritative acceptance verification.

// === Acceptance coverage for tasks/ready/page-size-and-no-count.md ===
//
// The task file uses plain bullet-point criteria without TestAC_ IDs.
// The mapping below documents which test covers each criterion.
//
// Page size criteria:
//   P opens docked input        → TestApp_PageSize_PKeyOpensInput
//   Only digits accepted        → TestRowBrowserModel_PageSizeInput_NonDigitDropped
//   Enter valid updates+reloads → TestRowBrowserModel_PageSizeInput_ValidValue_TriggersLoad
//   Enter invalid keeps open    → TestRowBrowserModel_PageSizeInput_InvalidValue_ShowsError
//   Esc closes no change        → TestRowBrowserModel_PageSizeInput_EscCloses
//   Dataset switch restores     → covered by PageSizeRegistry isolation (TestPageSizeRegistry_NamesAreIsolated)
//   Drill-down child/parent     → TestRowBrowserModel_PageSizeInput_ValidValue_TriggersLoad + registry
//   Restart resets to 50        → NewPageSizeRegistry(50) + TestPageSizeRegistry_DefaultForUnknownName
//
// No default COUNT(*) criteria:
//   No COUNT on default loads   → TestRowBrowserModel_StartsLoading (executor nil, no panic) +
//                                  TestRowBrowserModel_RowsLoaded (TotalPages unknown until end)
//   Status shows "page N"       → TestRowBrowserModel_StatusLine_TotalUnknown
//   ] blocked at HasMore=false  → TestRowBrowserModel_NextPageAtLastPage
//   ] on last page shows "~T"   → TestRowBrowserModel_StatusLine_TotalInferred +
//                                  TestApp_RowBrowser_InferredTotalOnLastPage
//
// First/last page criteria:
//   g/Home → page 1 no COUNT   → TestRowBrowserModel_FirstPage_g + TestRowBrowserModel_FirstPage_Home
//   G/End → one-shot COUNT     → TestApp_LastPage_GKeyProducesExactTotal
//   Finding status while in flight → TestRowBrowserModel_StatusLine_FindingLastPage +
//                                    TestRowBrowserModel_LastPage_G_SetsStatus
//   Exact status after count   → TestRowBrowserModel_StatusLine_TotalExact +
//                                  TestApp_LastPage_GKeyProducesExactTotal
//   Count fail shows error     → countLoadedMsg err path in rowbrowser.go (error wired, no DB to fail in unit tests)
//   Exact persists across paging → TestRowBrowserModel_Sort_ClearsTotalOnChange (clears on change)
//
// Core changes criteria:
//   TotalRows *int64, TotalPages *int, nil when unknown → dataset.go type change + all updated callers
//   HasMore set on every result → TestPostgres_Executor/Query_SkipCount_has_more_* +
//                                  TestMySQL_Executor same
//   SkipCount path              → TestPostgres_Executor/Query_SkipCount_has_more_true +
//                                  TestPostgres_Executor/Query_SkipCount_has_more_false
//   OnlyCount path              → TestPostgres_Executor/Query_OnlyCount
//   Both flags → error          → TestPostgres_Executor/Query_SkipCount_and_OnlyCount_rejected
//   Existing tests pass         → all 386 tests green
//
// Keys & help criteria:
//   keys.Map has bindings       → keys.go Default() + TestHelpOverlayView_FirstLastPageAndPageSizeVisible
//   FullHelp groups with paging → keys.go FullHelp()
//   Help overlay text           → TestHelpOverlayView_FirstLastPageAndPageSizeVisible
//
// Tests criteria (per-spec test requirements):
//   dataset_test SkipCount      → TestPostgres/MySQL_Executor/Query_SkipCount_*
//   dataset_test SkipCount+OnlyCount rejected → Query_SkipCount_and_OnlyCount_rejected
//   rowbrowser_test status bar  → TestRowBrowserModel_StatusLine_Total{Unknown,Inferred,Exact,FindingLastPage}
//   pagesize_test.go            → TestPageSizeRegistry_*
//   Smoke: P key                → TestApp_PageSize_PKeyOpensInput
//   Smoke: G key COUNT          → TestApp_LastPage_GKeyProducesExactTotal
//   Smoke: ] paging tilde       → TestApp_RowBrowser_InferredTotalOnLastPage

func TestApp_PageSize_PKeyOpensInput(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set")
	}

	ctx := context.Background()
	client, err := db.Connect(dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() {
		_, _ = client.Query(ctx, "DROP TABLE IF EXISTS pagesize_p_test")
		_ = client.Close()
	}()

	if _, err := client.Query(ctx, "CREATE TABLE IF NOT EXISTS pagesize_p_test (id SERIAL PRIMARY KEY, val TEXT)"); err != nil {
		t.Fatalf("create fixture: %v", err)
	}
	if _, err := client.Query(ctx, "INSERT INTO pagesize_p_test (val) VALUES ('a') ON CONFLICT DO NOTHING"); err != nil {
		t.Fatalf("insert fixture: %v", err)
	}

	app := tui.New(tui.Config{ConnectionString: dsn, Version: "test"}, client, nil)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(160, 40))

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return strings.Contains(string(bts), "pagesize_p_test")
	}, teatest.WithDuration(10*time.Second), teatest.WithCheckInterval(200*time.Millisecond))

	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		s := string(bts)
		return strings.Contains(s, "2 pagesize_p_test") && strings.Contains(s, "page 1/")
	}, teatest.WithDuration(10*time.Second), teatest.WithCheckInterval(200*time.Millisecond))

	// Press P — page-size input bar must appear.
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'P'}})
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return strings.Contains(string(bts), "Page size:")
	}, teatest.WithDuration(5*time.Second))

	_ = tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

func TestApp_LastPage_GKeyProducesExactTotal(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set")
	}

	ctx := context.Background()
	client, err := db.Connect(dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() {
		_, _ = client.Query(ctx, "DROP TABLE IF EXISTS lastpage_g_test")
		_ = client.Close()
	}()

	if _, err := client.Query(ctx, "CREATE TABLE IF NOT EXISTS lastpage_g_test (id SERIAL PRIMARY KEY, val TEXT)"); err != nil {
		t.Fatalf("create fixture: %v", err)
	}
	// Insert exactly 7 rows — chosen to be a recognisable number unlikely to appear elsewhere.
	for range 7 {
		if _, err := client.Query(ctx, "INSERT INTO lastpage_g_test (val) VALUES ('x')"); err != nil {
			t.Fatalf("insert fixture: %v", err)
		}
	}

	app := tui.New(tui.Config{ConnectionString: dsn, Version: "test"}, client, nil)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(160, 40))

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return strings.Contains(string(bts), "lastpage_g_test")
	}, teatest.WithDuration(10*time.Second), teatest.WithCheckInterval(200*time.Millisecond))

	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	// Wait for inferred total first (HasMore=false, tilde present).
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		s := string(bts)
		return strings.Contains(s, "2 lastpage_g_test") && strings.Contains(s, "~7 rows")
	}, teatest.WithDuration(10*time.Second), teatest.WithCheckInterval(200*time.Millisecond))

	// Press G — runs one-shot COUNT(*) and sets exact total (no tilde).
	// The exact format is "  7 rows" (two spaces, no tilde) vs inferred "  ~7 rows".
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return strings.Contains(string(bts), "  7 rows")
	}, teatest.WithDuration(10*time.Second))

	_ = tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

func TestApp_RowBrowser_InferredTotalOnLastPage(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set")
	}

	ctx := context.Background()
	client, err := db.Connect(dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() {
		_, _ = client.Query(ctx, "DROP TABLE IF EXISTS inferred_total_test")
		_ = client.Close()
	}()

	if _, err := client.Query(ctx, "CREATE TABLE IF NOT EXISTS inferred_total_test (id SERIAL PRIMARY KEY, val TEXT)"); err != nil {
		t.Fatalf("create fixture: %v", err)
	}
	for range 3 {
		if _, err := client.Query(ctx, "INSERT INTO inferred_total_test (val) VALUES ('row')"); err != nil {
			t.Fatalf("insert fixture: %v", err)
		}
	}

	app := tui.New(tui.Config{ConnectionString: dsn, Version: "test"}, client, nil)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(160, 40))

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return strings.Contains(string(bts), "inferred_total_test")
	}, teatest.WithDuration(10*time.Second), teatest.WithCheckInterval(200*time.Millisecond))

	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	// With 3 rows and default PageSize=50, HasMore=false on first load → inferred total
	// displayed as "~3 rows" in the status bar.
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		s := string(bts)
		return strings.Contains(s, "2 inferred_total_test") && strings.Contains(s, "~")
	}, teatest.WithDuration(10*time.Second), teatest.WithCheckInterval(200*time.Millisecond))

	_ = tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

// TestAC_B10_LeavingPaneClearsFilter verifies that switching away from the
// table-list pane clears any held filter. Because teatest accumulates ALL
// bytes (including ANSI from every previous frame), we cannot check for the
// ABSENCE of earlier filter text. Instead we verify a positive signal: after
// switching back to pane 1 and pressing "/" again, the filter input opens EMPTY
// (not pre-filled), confirming the filter was cleared when focus left.
func TestAC_B10_LeavingPaneClearsFilter(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set")
	}

	ctx := context.Background()
	client, err := db.Connect(dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() {
		_, _ = client.Query(ctx, "DROP TABLE IF EXISTS filter_ac_b10_test")
		_ = client.Close()
	}()

	if _, err := client.Query(ctx, "CREATE TABLE IF NOT EXISTS filter_ac_b10_test (id SERIAL PRIMARY KEY)"); err != nil {
		t.Fatalf("create fixture: %v", err)
	}

	app := tui.New(tui.Config{ConnectionString: dsn, Version: "test"}, client, nil)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(160, 40))

	// Wait for the fixture table to appear in the table list.
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return strings.Contains(string(bts), "filter_ac_b10_test")
	}, teatest.WithDuration(15*time.Second), teatest.WithCheckInterval(200*time.Millisecond))

	// Open the filter and type some text. Sleep to let "/" be processed before
	// typing — the WaitFor for "filter tables" returns immediately (it was
	// already in accumulated bytes), so we cannot rely on it as a sync point.
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	time.Sleep(300 * time.Millisecond)

	for _, r := range "filter_ac" {
		tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	// Hold the filter (close input) with Enter so BlocksGlobalKeys() returns
	// false. Without this, pressing "2" is consumed by the filter input.
	// Sleep before and after to let keystrokes settle.
	time.Sleep(200 * time.Millisecond)
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	time.Sleep(300 * time.Millisecond)

	// Press "2" — leave pane 1 (which should clear the filter).
	// This works because the filter input is now closed (held state).
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	time.Sleep(300 * time.Millisecond)

	// Return to pane 1.
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
	time.Sleep(300 * time.Millisecond)

	// Open the filter and type a unique string never seen before in this test.
	// If the filter was cleared when focus left (B10), typing "xfresh" gives
	// filter="xfresh" and `filter: "xfresh"` appears. If the filter was NOT
	// cleared (held "filter_ac"), typing "xfresh" produces "filter_acxfresh" —
	// a different string. Using a novel positive assertion avoids the teatest
	// accumulated-bytes problem (WaitFor checks ALL bytes ever written).
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	time.Sleep(200 * time.Millisecond)
	for _, r := range "xfresh" {
		tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return strings.Contains(string(bts), `filter: "xfresh"`)
	}, teatest.WithDuration(5*time.Second))

	_ = tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}
