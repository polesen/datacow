package views_test

import (
	"errors"
	"testing"

	"github.com/beetio/datacow/internal/core/dataset"
	"github.com/beetio/datacow/internal/tui/keys"
	"github.com/beetio/datacow/internal/tui/views"
	tea "github.com/charmbracelet/bubbletea"
)

func TestTableListModel_StartsLoading(t *testing.T) {
	m := views.NewTableListModel(keys.Default(), nil, nil)
	if !m.IsLoading() {
		t.Error("expected loading state initially")
	}
	if m.DatasetCount() != 0 {
		t.Errorf("expected 0 datasets initially, got %d", m.DatasetCount())
	}
}

func TestTableListModel_TablesLoaded(t *testing.T) {
	m := views.NewTableListModel(keys.Default(), nil, nil)
	m, _ = m.Update(views.TablesLoadedMsg([]dataset.Dataset{
		{Name: "users", Table: "users"},
		{Name: "posts", Table: "posts"},
	}))
	if m.IsLoading() {
		t.Error("should not be loading after tables loaded")
	}
	if m.DatasetCount() != 2 {
		t.Errorf("expected 2 datasets, got %d", m.DatasetCount())
	}
	if m.Err() != nil {
		t.Errorf("unexpected error: %v", m.Err())
	}
}

func TestTableListModel_Navigation(t *testing.T) {
	m := views.NewTableListModel(keys.Default(), nil, nil)
	m, _ = m.Update(views.TablesLoadedMsg([]dataset.Dataset{
		{Name: "users", Table: "users"},
		{Name: "posts", Table: "posts"},
		{Name: "comments", Table: "comments"},
	}))

	if m.Cursor() != 0 {
		t.Errorf("expected cursor 0, got %d", m.Cursor())
	}

	// Move down twice
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.Cursor() != 1 {
		t.Errorf("expected cursor 1 after down, got %d", m.Cursor())
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.Cursor() != 2 {
		t.Errorf("expected cursor 2 after down, got %d", m.Cursor())
	}

	// Cannot go past last item
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.Cursor() != 2 {
		t.Errorf("cursor should stay at 2 at end, got %d", m.Cursor())
	}

	// Move up
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if m.Cursor() != 1 {
		t.Errorf("expected cursor 1 after up, got %d", m.Cursor())
	}

	// Cannot go before first item
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if m.Cursor() != 0 {
		t.Errorf("cursor should stay at 0, got %d", m.Cursor())
	}
}

func TestTableListModel_NavigationBlockedWhileLoading(t *testing.T) {
	m := views.NewTableListModel(keys.Default(), nil, nil)
	// Still loading — navigation should have no effect
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.Cursor() != 0 {
		t.Errorf("cursor should stay at 0 while loading, got %d", m.Cursor())
	}
}

func TestTableListModel_Error(t *testing.T) {
	m := views.NewTableListModel(keys.Default(), nil, nil)
	m, _ = m.Update(views.ErrMsg{Err: errors.New("connection refused")})
	if m.IsLoading() {
		t.Error("should not be loading after error")
	}
	if m.Err() == nil {
		t.Error("expected error to be set")
	}
}

func TestTableListModel_SelectedDataset(t *testing.T) {
	m := views.NewTableListModel(keys.Default(), nil, nil)
	if m.SelectedDataset() != nil {
		t.Error("expected nil SelectedDataset when no tables loaded")
	}

	m, _ = m.Update(views.TablesLoadedMsg([]dataset.Dataset{
		{Name: "users", Table: "users"},
		{Name: "posts", Table: "posts"},
	}))

	ds := m.SelectedDataset()
	if ds == nil {
		t.Fatal("expected non-nil SelectedDataset after load")
	}
	if ds.Name != "users" {
		t.Errorf("expected selected dataset 'users', got %q", ds.Name)
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	ds = m.SelectedDataset()
	if ds.Name != "posts" {
		t.Errorf("expected selected dataset 'posts', got %q", ds.Name)
	}
}

func TestTableListModel_View(t *testing.T) {
	m := views.NewTableListModel(keys.Default(), nil, nil)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 20})
	m, _ = m.Update(views.TablesLoadedMsg([]dataset.Dataset{
		{Name: "users", Table: "users"},
	}))
	v := m.View()
	if v == "" {
		t.Error("expected non-empty view")
	}
}

func TestTableListModel_RowCountReceived(t *testing.T) {
	m := views.NewTableListModel(keys.Default(), nil, nil)
	m, _ = m.Update(views.TablesLoadedMsg([]dataset.Dataset{
		{Name: "users", Table: "users"},
	}))
	_, _ = m.Update(views.RowCountMsg{Name: "users", Count: 1234})
	// No crash, count stored
}
