package views_test

import (
	"testing"

	"github.com/polesen/datacow/internal/tui/views"
)

func TestPageSizeRegistry_DefaultForUnknownName(t *testing.T) {
	r := views.NewPageSizeRegistry(50)
	if got := r.Get("anything"); got != 50 {
		t.Errorf("got %d, want 50", got)
	}
}

func TestPageSizeRegistry_SetAndGet(t *testing.T) {
	r := views.NewPageSizeRegistry(50)
	r.Set("users", 25)
	if got := r.Get("users"); got != 25 {
		t.Errorf("got %d, want 25", got)
	}
}

func TestPageSizeRegistry_NamesAreIsolated(t *testing.T) {
	r := views.NewPageSizeRegistry(50)
	r.Set("users", 25)
	if got := r.Get("orders"); got != 50 {
		t.Errorf("orders should use default 50, got %d", got)
	}
}

func TestPageSizeRegistry_NilReceiver_GetReturns50(t *testing.T) {
	var r *views.PageSizeRegistry
	if got := r.Get("anything"); got != 50 {
		t.Errorf("nil receiver Get should return 50, got %d", got)
	}
}

func TestPageSizeRegistry_NilReceiver_SetIsNoop(t *testing.T) {
	var r *views.PageSizeRegistry
	r.Set("users", 25) // must not panic
}
