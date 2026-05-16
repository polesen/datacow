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
