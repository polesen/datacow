package completions_test

// Acceptance tests for the SQL completion engine.
// Each test maps to one criterion in tasks/ready/sql-dataset-editor.md.
//
// Coverage map (CP section):
//   CP01: SELECT context returns keywords                  → TestAC_CP01_SelectReturnsKeywordSuggestion
//   CP02: FROM context lists all tables                    → TestAC_CP02_FromListsAllTables
//   CP03: FROM context filters by table prefix             → TestAC_CP03_FromFiltersByPrefix
//   CP04: dot-qualified prefix returns table columns       → TestAC_CP04_DotQualifiedReturnsColumns
//   CP05: SELECT bare prefix returns column names           → TestAC_CP05_SelectBarePrefixReturnsColumns
//   CP06: MySQL dialect includes STRAIGHT_JOIN             → TestAC_CP06_MySQLDialectKeywords
//   CP07: Postgres dialect includes RETURNING              → TestAC_CP07_PostgresDialectKeywords
//   CP08: empty SQL returns non-empty keyword list          → TestAC_CP08_EmptySQLReturnsKeywords
//   CP09: no matches returns empty non-nil slice            → TestAC_CP09_NoMatchesReturnsEmptyNonNil
//   CP10: aliased dot-qualifier returns aliased table cols  → TestAC_CP10_AliasDotQualifierReturnsColumns

import (
	"strings"
	"testing"

	"github.com/polesen/datacow/internal/core/completions"
	"github.com/polesen/datacow/internal/core/db"
	"github.com/polesen/datacow/internal/core/schema"
)

func tablesUsers() []schema.Table {
	return []schema.Table{
		{
			Name: "users",
			Columns: []db.Column{
				{Name: "id", Type: "integer"},
				{Name: "email", Type: "varchar(255)"},
			},
		},
	}
}

func tablesUsersOrders() []schema.Table {
	return []schema.Table{
		{
			Name: "users",
			Columns: []db.Column{
				{Name: "id", Type: "integer"},
				{Name: "email", Type: "varchar(255)"},
			},
		},
		{
			Name: "orders",
			Columns: []db.Column{
				{Name: "id", Type: "integer"},
				{Name: "user_id", Type: "integer"},
				{Name: "total", Type: "numeric"},
			},
		},
	}
}

func containsText(suggestions []completions.Suggestion, text string) bool {
	for _, s := range suggestions {
		if s.Text == text {
			return true
		}
	}
	return false
}

func countOfKind(suggestions []completions.Suggestion, k completions.Kind) int {
	n := 0
	for _, s := range suggestions {
		if s.Kind == k {
			n++
		}
	}
	return n
}

// CP01: Complete("SELECT ", 7) with a schema containing a table returns at least one
// KindKeyword suggestion; result is non-nil.
func TestAC_CP01_SelectReturnsKeywordSuggestion(t *testing.T) {
	c := completions.New(tablesUsers(), db.DialectPostgres)
	got := c.Complete("SELECT ", 7)
	if got == nil {
		t.Fatal("Complete returned nil slice; must be non-nil")
	}
	if countOfKind(got, completions.KindKeyword) == 0 {
		t.Errorf("expected at least one KindKeyword suggestion, got: %+v", got)
	}
}

// CP02: Complete after "FROM " with two tables returns both as KindTable.
func TestAC_CP02_FromListsAllTables(t *testing.T) {
	c := completions.New(tablesUsersOrders(), db.DialectPostgres)
	got := c.Complete("SELECT * FROM ", 14)
	if !containsText(got, "users") {
		t.Errorf("expected 'users' in suggestions, got: %+v", got)
	}
	if !containsText(got, "orders") {
		t.Errorf("expected 'orders' in suggestions, got: %+v", got)
	}
	for _, s := range got {
		if (s.Text == "users" || s.Text == "orders") && s.Kind != completions.KindTable {
			t.Errorf("expected %q to be KindTable, got %v", s.Text, s.Kind)
		}
	}
}

// CP03: prefix filter is applied — only matching tables returned.
func TestAC_CP03_FromFiltersByPrefix(t *testing.T) {
	c := completions.New(tablesUsersOrders(), db.DialectPostgres)
	got := c.Complete("SELECT * FROM us", 16)
	if !containsText(got, "users") {
		t.Errorf("expected 'users' in suggestions, got: %+v", got)
	}
	if containsText(got, "orders") {
		t.Errorf("'orders' must not appear when prefix is 'us', got: %+v", got)
	}
}

// CP04: dot-qualified prefix returns the table's columns with type Detail.
func TestAC_CP04_DotQualifiedReturnsColumns(t *testing.T) {
	c := completions.New(tablesUsers(), db.DialectPostgres)
	got := c.Complete("SELECT u.", 9)

	colCount := countOfKind(got, completions.KindColumn)
	if colCount != 2 {
		t.Errorf("expected exactly 2 KindColumn suggestions, got %d: %+v", colCount, got)
	}
	if !containsText(got, "id") {
		t.Errorf("expected 'id' column, got: %+v", got)
	}
	if !containsText(got, "email") {
		t.Errorf("expected 'email' column, got: %+v", got)
	}
	for _, s := range got {
		if s.Text == "id" {
			if !strings.Contains(s.Detail, "int") {
				t.Errorf("expected Detail of 'id' to contain 'int', got %q", s.Detail)
			}
		}
	}
}

// CP05: bare prefix in SELECT context returns matching columns from all tables.
func TestAC_CP05_SelectBarePrefixReturnsColumns(t *testing.T) {
	c := completions.New(tablesUsers(), db.DialectPostgres)
	got := c.Complete("SELECT em", 9)
	if !containsText(got, "email") {
		t.Errorf("expected 'email' in suggestions for prefix 'em', got: %+v", got)
	}
	for _, s := range got {
		if s.Text == "email" && s.Kind != completions.KindColumn {
			t.Errorf("expected 'email' to be KindColumn, got %v", s.Kind)
		}
	}
}

// CP06: MySQL dialect includes a MySQL-specific keyword (STRAIGHT_JOIN) that is not
// present in the equivalent Postgres result.
func TestAC_CP06_MySQLDialectKeywords(t *testing.T) {
	mc := completions.New(tablesUsers(), db.DialectMySQL)
	pc := completions.New(tablesUsers(), db.DialectPostgres)

	mySuggestions := mc.Complete("SELECT ", 7)
	pgSuggestions := pc.Complete("SELECT ", 7)

	if !containsText(mySuggestions, "STRAIGHT_JOIN") {
		t.Error("expected MySQL completions to include STRAIGHT_JOIN")
	}
	if containsText(pgSuggestions, "STRAIGHT_JOIN") {
		t.Error("Postgres completions must NOT include STRAIGHT_JOIN")
	}
}

// CP07: Postgres dialect includes a PG-specific keyword (RETURNING) that is not
// present in the equivalent MySQL result.
func TestAC_CP07_PostgresDialectKeywords(t *testing.T) {
	pc := completions.New(tablesUsers(), db.DialectPostgres)
	mc := completions.New(tablesUsers(), db.DialectMySQL)

	pgSuggestions := pc.Complete("SELECT ", 7)
	mySuggestions := mc.Complete("SELECT ", 7)

	if !containsText(pgSuggestions, "RETURNING") {
		t.Error("expected Postgres completions to include RETURNING")
	}
	if containsText(mySuggestions, "RETURNING") {
		t.Error("MySQL completions must NOT include RETURNING")
	}
}

// CP08: empty SQL returns a non-nil, non-empty slice (keywords by default).
func TestAC_CP08_EmptySQLReturnsKeywords(t *testing.T) {
	c := completions.New(tablesUsers(), db.DialectPostgres)
	got := c.Complete("", 0)
	if got == nil {
		t.Fatal("Complete('', 0) returned nil; must be non-nil")
	}
	if len(got) == 0 {
		t.Error("Complete('', 0) returned empty slice; must include keywords")
	}
}

// CP09: a prefix with no matches returns an empty, non-nil slice.
func TestAC_CP09_NoMatchesReturnsEmptyNonNil(t *testing.T) {
	c := completions.New(tablesUsers(), db.DialectPostgres)
	got := c.Complete("SELECT xyz", 10)
	if got == nil {
		t.Fatal("Complete returned nil; must be non-nil empty slice")
	}
	if len(got) != 0 {
		t.Errorf("expected empty slice for unmatchable prefix, got: %+v", got)
	}
}

// CP10: cursor positioned after a dot-qualified alias returns columns of the
// aliased table only — no table names in the result.
func TestAC_CP10_AliasDotQualifierReturnsColumns(t *testing.T) {
	c := completions.New(tablesUsersOrders(), db.DialectPostgres)
	sql := "SELECT * FROM orders o WHERE o."
	got := c.Complete(sql, len(sql))

	if !containsText(got, "id") {
		t.Errorf("expected 'id' column of orders, got: %+v", got)
	}
	if !containsText(got, "user_id") {
		t.Errorf("expected 'user_id' column of orders, got: %+v", got)
	}
	if !containsText(got, "total") {
		t.Errorf("expected 'total' column of orders, got: %+v", got)
	}
	if countOfKind(got, completions.KindTable) != 0 {
		t.Errorf("dot-qualified context must not return table names, got: %+v", got)
	}
}

// Bonus: cursor positioned between the alias letter and the dot must still be
// treated as dot-qualified. Defensive check for the off-by-one mentioned in the
// spec (cursorPos=30 vs string length 31).
func TestAC_CP10_AliasDotQualifierCursorBeforeDot(t *testing.T) {
	c := completions.New(tablesUsersOrders(), db.DialectPostgres)
	sql := "SELECT * FROM orders o WHERE o."
	got := c.Complete(sql, 30) // cursor sits between 'o' and '.'
	if countOfKind(got, completions.KindTable) != 0 {
		t.Errorf("dot-qualified context must not return table names, got: %+v", got)
	}
	if !containsText(got, "id") {
		t.Errorf("expected 'id' column of orders, got: %+v", got)
	}
}
