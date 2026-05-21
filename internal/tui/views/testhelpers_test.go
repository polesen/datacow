package views_test

import (
	"context"
	"errors"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/polesen/datacow/internal/core/db"
	"github.com/polesen/datacow/internal/tui/views"
)

// stubClient is a minimal db.Client that lets tests control success/error outcomes.
type stubClient struct{ err error }

func (s *stubClient) Ping(_ context.Context) error { return nil }
func (s *stubClient) ListTables(_ context.Context) ([]db.TableEntry, error) {
	return []db.TableEntry{{Name: "users", Kind: db.KindTable}}, s.err
}
func (s *stubClient) Describe(_ context.Context, _ string) ([]db.Column, error) {
	return nil, s.err
}
func (s *stubClient) ForeignKeys(_ context.Context, _ string) ([]db.ForeignKey, error) {
	return nil, s.err
}
func (s *stubClient) Indexes(_ context.Context, _ string) ([]db.Index, error) {
	return nil, s.err
}
func (s *stubClient) Query(_ context.Context, _ string, _ ...any) ([]map[string]any, error) {
	if s.err != nil {
		return nil, s.err
	}
	return []map[string]any{{"id": 1}}, nil
}
func (s *stubClient) Placeholder(_ int) string { return "?" }
func (s *stubClient) Dialect() db.Dialect      { return db.DialectPostgres }
func (s *stubClient) Close() error             { return nil }

// addQuery records one successful completed entry in ql via a LoggingClient.
func addQuery(ql *db.QueryLog, sql string) {
	lc := db.NewLoggingClient(&stubClient{}, ql)
	_, _ = lc.Query(context.Background(), sql)
}

// addErrorQuery records one failed completed entry in ql.
func addErrorQuery(ql *db.QueryLog, sql, msg string) {
	lc := db.NewLoggingClient(&stubClient{err: errors.New(msg)}, ql)
	_, _ = lc.Query(context.Background(), sql)
}

// sizedQueryLogView returns a QueryLogView sized to w×h.
func sizedQueryLogView(ql *db.QueryLog, w, h int) views.QueryLogView {
	v := views.NewQueryLogView(ql)
	v, _ = v.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return v
}
