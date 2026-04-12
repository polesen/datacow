package db_test

import (
	"context"
	"testing"

	"github.com/beetio/datacow/internal/core/db"
)

// queryExec runs a DDL or DML statement via Query, ignoring the result set.
func queryExec(ctx context.Context, client db.Client, sql string) ([]map[string]any, error) {
	return client.Query(ctx, sql)
}

func assertContains(t *testing.T, haystack []string, needle string) {
	t.Helper()
	for _, s := range haystack {
		if s == needle {
			return
		}
	}
	t.Errorf("%q not found in %v", needle, haystack)
}

func assertColumn(t *testing.T, col db.Column, name string, nullable bool) {
	t.Helper()
	if col.Name != name {
		t.Errorf("column name: got %q, want %q", col.Name, name)
	}
	if col.Nullable != nullable {
		t.Errorf("column %q nullable: got %v, want %v", name, col.Nullable, nullable)
	}
}
