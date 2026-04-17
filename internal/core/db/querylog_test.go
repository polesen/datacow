package db

import (
	"sync"
	"testing"
	"time"
)

func TestQueryLog_RunningCount(t *testing.T) {
	ql := NewQueryLog()

	if got := ql.RunningCount(); got != 0 {
		t.Fatalf("want 0, got %d", got)
	}

	id1 := ql.begin("query 1", "SELECT 1", QueryKindUser)
	if got := ql.RunningCount(); got != 1 {
		t.Fatalf("want 1, got %d", got)
	}

	id2 := ql.begin("query 2", "SELECT 2", QueryKindSystem)
	if got := ql.RunningCount(); got != 2 {
		t.Fatalf("want 2, got %d", got)
	}

	ql.end(id1, 5, nil)
	if got := ql.RunningCount(); got != 1 {
		t.Fatalf("want 1 after end, got %d", got)
	}

	ql.end(id2, 0, nil)
	if got := ql.RunningCount(); got != 0 {
		t.Fatalf("want 0 after all ended, got %d", got)
	}
}

func TestQueryLog_CurrentLabel(t *testing.T) {
	ql := NewQueryLog()

	if got := ql.CurrentLabel(); got != "" {
		t.Fatalf("want empty, got %q", got)
	}

	id := ql.begin("list tables", "", QueryKindSystem)
	label := ql.CurrentLabel()
	if label == "" {
		t.Fatal("want non-empty label while running")
	}

	ql.end(id, 0, nil)
	if got := ql.CurrentLabel(); got != "" {
		t.Fatalf("want empty after end, got %q", got)
	}
}

func TestQueryLog_Snapshot(t *testing.T) {
	ql := NewQueryLog()

	id1 := ql.begin("query", "SELECT * FROM orders", QueryKindUser)
	id2 := ql.begin("list tables", "", QueryKindSystem)

	running, history := ql.Snapshot()
	if len(running) != 2 {
		t.Fatalf("want 2 running, got %d", len(running))
	}
	if len(history) != 0 {
		t.Fatalf("want 0 history, got %d", len(history))
	}

	// Running entries should report elapsed duration (not zero)
	for _, e := range running {
		if e.Duration <= 0 {
			t.Errorf("running entry duration should be > 0, got %v", e.Duration)
		}
	}

	ql.end(id1, 10, nil)
	ql.end(id2, 5, nil)

	running, history = ql.Snapshot()
	if len(running) != 0 {
		t.Fatalf("want 0 running, got %d", len(running))
	}
	if len(history) != 2 {
		t.Fatalf("want 2 history, got %d", len(history))
	}
	// newest-first: id2 ended last so it should be first
	if history[0].ID != id2 {
		t.Errorf("want newest-first ordering: first entry ID=%d, want %d", history[0].ID, id2)
	}
}

func TestQueryLog_HistoryCap(t *testing.T) {
	ql := NewQueryLog()

	for i := range 210 {
		id := ql.begin("query", "SELECT 1", QueryKindUser)
		ql.end(id, int64(i), nil)
	}

	_, history := ql.Snapshot()
	if len(history) != 200 {
		t.Fatalf("want history capped at 200, got %d", len(history))
	}
}

func TestQueryLog_HistoryNewestFirst(t *testing.T) {
	ql := NewQueryLog()

	id1 := ql.begin("first", "", QueryKindUser)
	ql.end(id1, 0, nil)

	id2 := ql.begin("second", "", QueryKindUser)
	ql.end(id2, 0, nil)

	_, history := ql.Snapshot()
	if len(history) < 2 {
		t.Fatal("expected at least 2 entries")
	}
	if history[0].Label != "second" {
		t.Errorf("want newest first, got label=%q", history[0].Label)
	}
	if history[1].Label != "first" {
		t.Errorf("want oldest second, got label=%q", history[1].Label)
	}
}

func TestQueryLog_ThreadSafety(t *testing.T) {
	ql := NewQueryLog()
	const goroutines = 50

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := range goroutines {
		go func(i int) {
			defer wg.Done()
			id := ql.begin("query", "SELECT 1", QueryKindUser)
			time.Sleep(time.Millisecond)
			ql.end(id, int64(i), nil)
			_ = ql.RunningCount()
			_ = ql.CurrentLabel()
			_, _ = ql.Snapshot()
		}(i)
	}

	wg.Wait()

	if got := ql.RunningCount(); got != 0 {
		t.Fatalf("want 0 running after all goroutines done, got %d", got)
	}
}

func TestQueryLog_EndUnknownID(t *testing.T) {
	ql := NewQueryLog()
	// Should not panic or error
	ql.end(9999, 0, nil)
}

func TestQueryLog_QueryEntryFields(t *testing.T) {
	ql := NewQueryLog()

	before := time.Now()
	id := ql.begin("list tables", "", QueryKindSystem)
	time.Sleep(2 * time.Millisecond)
	ql.end(id, 7, nil)

	_, history := ql.Snapshot()
	if len(history) == 0 {
		t.Fatal("expected 1 history entry")
	}
	e := history[0]

	if e.Label != "list tables" {
		t.Errorf("want label 'list tables', got %q", e.Label)
	}
	if e.Kind != QueryKindSystem {
		t.Errorf("want QueryKindSystem, got %v", e.Kind)
	}
	if e.RowCount != 7 {
		t.Errorf("want rowCount=7, got %d", e.RowCount)
	}
	if e.Duration <= 0 {
		t.Errorf("want duration > 0, got %v", e.Duration)
	}
	if e.StartedAt.Before(before) {
		t.Errorf("StartedAt should be after test start")
	}
}
