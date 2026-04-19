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
	}, teatest.WithDuration(3*time.Second))

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
	}, teatest.WithDuration(3*time.Second))

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
	}, teatest.WithDuration(3*time.Second))

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
	}, teatest.WithDuration(3*time.Second))

	// Press "1" → back to tables pane; status bar shows table list bindings (↵ select).
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return strings.Contains(string(bts), "↵")
	}, teatest.WithDuration(3*time.Second))

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
	defer func() { _ = client.Close() }()

	ctx := context.Background()
	if _, err := client.Query(ctx, "CREATE TABLE IF NOT EXISTS tui_test_items (id SERIAL PRIMARY KEY, name TEXT)"); err != nil {
		t.Fatalf("create fixture: %v", err)
	}
	t.Cleanup(func() {
		_, _ = client.Query(ctx, "DROP TABLE IF EXISTS tui_test_items")
	})

	app := tui.New(tui.Config{ConnectionString: dsn, Version: "test"}, client, nil)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))

	// Wait for the fixture table to appear in the table list (tables load async).
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		s := string(bts)
		return strings.Contains(s, "tui_test_items") && !strings.Contains(s, "Connecting")
	}, teatest.WithDuration(10*time.Second), teatest.WithCheckInterval(200*time.Millisecond))

	_ = tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}
