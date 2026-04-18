package views_test

import (
	"errors"
	"testing"

	"github.com/beetio/datacow/internal/core/config"
	"github.com/beetio/datacow/internal/tui/keys"
	"github.com/beetio/datacow/internal/tui/views"
	tea "github.com/charmbracelet/bubbletea"
)

var testDatasources = []config.DatasourceConfig{
	{Name: "production", ConnectionString: "postgres://prod/mydb"},
	{Name: "staging", ConnectionString: "postgres://staging/mydb"},
	{Name: "local", ConnectionString: "postgres://localhost/mydb"},
}

func TestDatasourceListModel_InitialState(t *testing.T) {
	m := views.NewDatasourceListModel(keys.Default(), testDatasources)
	if m.Cursor() != 0 {
		t.Errorf("expected cursor 0, got %d", m.Cursor())
	}
	for _, ds := range testDatasources {
		if m.Status(ds.Name) != views.StatusDisconnected {
			t.Errorf("expected %q to be disconnected initially", ds.Name)
		}
	}
}

func TestDatasourceListModel_Navigation(t *testing.T) {
	m := views.NewDatasourceListModel(keys.Default(), testDatasources)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.Cursor() != 1 {
		t.Errorf("expected cursor 1 after down, got %d", m.Cursor())
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.Cursor() != 2 {
		t.Errorf("expected cursor 2, got %d", m.Cursor())
	}
	// Cannot go past last item
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.Cursor() != 2 {
		t.Errorf("cursor should stay at 2 at end, got %d", m.Cursor())
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if m.Cursor() != 1 {
		t.Errorf("expected cursor 1 after up, got %d", m.Cursor())
	}
}

func TestDatasourceListModel_SelectEmitsMsg(t *testing.T) {
	m := views.NewDatasourceListModel(keys.Default(), testDatasources)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected non-nil cmd on Enter")
	}
	msg := cmd()
	sel, ok := msg.(views.DatasourceSelectMsg)
	if !ok {
		t.Fatalf("expected DatasourceSelectMsg, got %T", msg)
	}
	if sel.Name != "production" {
		t.Errorf("expected Name=production, got %q", sel.Name)
	}
	if sel.ConnectionString != "postgres://prod/mydb" {
		t.Errorf("unexpected ConnectionString: %q", sel.ConnectionString)
	}
}

func TestDatasourceListModel_SelectOnSecondItem(t *testing.T) {
	m := views.NewDatasourceListModel(keys.Default(), testDatasources)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	msg := cmd()
	sel, ok := msg.(views.DatasourceSelectMsg)
	if !ok {
		t.Fatalf("expected DatasourceSelectMsg, got %T", msg)
	}
	if sel.Name != "staging" {
		t.Errorf("expected Name=staging, got %q", sel.Name)
	}
}

func TestDatasourceListModel_StatusConnecting(t *testing.T) {
	m := views.NewDatasourceListModel(keys.Default(), testDatasources)
	m, _ = m.Update(views.DatasourceConnectingMsg{Name: "production"})
	if m.Status("production") != views.StatusConnecting {
		t.Errorf("expected StatusConnecting, got %v", m.Status("production"))
	}
}

func TestDatasourceListModel_StatusConnected(t *testing.T) {
	m := views.NewDatasourceListModel(keys.Default(), testDatasources)
	m, _ = m.Update(views.DatasourceConnectedMsg{Name: "production"})
	if m.Status("production") != views.StatusConnected {
		t.Errorf("expected StatusConnected, got %v", m.Status("production"))
	}
}

func TestDatasourceListModel_StatusError(t *testing.T) {
	m := views.NewDatasourceListModel(keys.Default(), testDatasources)
	m, _ = m.Update(views.DatasourceErrorMsg{Name: "staging", Err: errors.New("refused")})
	if m.Status("staging") != views.StatusError {
		t.Errorf("expected StatusError, got %v", m.Status("staging"))
	}
}

func TestDatasourceListModel_View(t *testing.T) {
	m := views.NewDatasourceListModel(keys.Default(), testDatasources)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 20})
	v := m.View()
	if v == "" {
		t.Error("expected non-empty view")
	}
}
