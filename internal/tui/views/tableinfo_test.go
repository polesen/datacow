package views_test

import (
	"strings"
	"testing"

	"github.com/polesen/datacow/internal/tui/views"
)

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		n    int64
		want string
	}{
		{0, "0 B"},
		{1, "1 B"},
		{512, "512 B"},
		{1023, "1023 B"},
		{1024, "1 KB"},
		{2048, "2 KB"},
		{1048575, "1023 KB"},
		{1048576, "1.0 MB"},
		{44040192, "42.0 MB"},
		{1073741824, "1.0 GB"},
		{3435973836, "3.2 GB"},
		{1099511627776, "1.0 TB"},
	}
	for _, tc := range tests {
		got := views.FormatBytesExported(tc.n)
		if got != tc.want {
			t.Errorf("formatBytes(%d) = %q, want %q", tc.n, got, tc.want)
		}
	}
}

func TestFormatEstimate(t *testing.T) {
	tests := []struct {
		n    int64
		want string
	}{
		{0, "0"},
		{1, "1"},
		{999, "999"},
		{1000, "~1K"},
		{42000, "~42K"},
		{999999, "~999K"},
		{1000000, "~1.0M"},
		{1234567, "~1.2M"},
		{1000000000, "~1.0B"},
		{3400000000, "~3.4B"},
	}
	for _, tc := range tests {
		got := views.FormatEstimateExported(tc.n)
		if got != tc.want {
			t.Errorf("formatEstimate(%d) = %q, want %q", tc.n, got, tc.want)
		}
	}
}

func TestTableInfoModel_View_Loading(t *testing.T) {
	m := views.NewTableInfoModel()
	m.SetSize(80, 24)
	m.SetSpinChar("·")
	v := m.View()
	if v == "" {
		t.Fatal("expected non-empty view while loading")
	}
	if !strings.Contains(v, "loading") {
		t.Errorf("expected loading indicator in view, got:\n%s", v)
	}
}

func TestTableInfoModel_View_NotAvailable(t *testing.T) {
	m := views.NewTableInfoModel()
	m.SetSize(80, 24)
	stub := &stubClient{}
	cmd := m.Load(stub, "orders")
	msg := cmd()
	m, _ = m.Update(msg)
	v := m.View()
	if !strings.Contains(v, "statistics not available") {
		t.Errorf("expected not-available message in view, got:\n%s", v)
	}
}

func TestTableInfoModel_View_CloseHint(t *testing.T) {
	m := views.NewTableInfoModel()
	m.SetSize(80, 24)
	stub := &stubClient{}
	cmd := m.Load(stub, "orders")
	msg := cmd()
	m, _ = m.Update(msg)
	v := m.View()
	if !strings.Contains(v, "i or esc") {
		t.Errorf("expected close hint in view, got:\n%s", v)
	}
}
