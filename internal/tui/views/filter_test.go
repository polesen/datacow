package views

import (
	"testing"

	"github.com/beetio/datacow/internal/core/dataset"
)

func TestParseFilterInput(t *testing.T) {
	tests := []struct {
		input   string
		want    dataset.Filter
		wantErr bool
	}{
		{"name=Alice", dataset.Filter{Column: "name", Operator: "=", Value: "Alice"}, false},
		{"price>100", dataset.Filter{Column: "price", Operator: ">", Value: "100"}, false},
		{"price<50", dataset.Filter{Column: "price", Operator: "<", Value: "50"}, false},
		{"price>=10", dataset.Filter{Column: "price", Operator: ">=", Value: "10"}, false},
		{"price<=99", dataset.Filter{Column: "price", Operator: "<=", Value: "99"}, false},
		{"name like %foo%", dataset.Filter{Column: "name", Operator: "like", Value: "%foo%"}, false},
		{"name LIKE %foo%", dataset.Filter{Column: "name", Operator: "like", Value: "%foo%"}, false},
		{" name = Alice ", dataset.Filter{Column: "name", Operator: "=", Value: "Alice"}, false},
		{"noop", dataset.Filter{}, true},
		{"=value", dataset.Filter{}, true},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got, err := parseFilterInput(tc.input)
			if (err != nil) != tc.wantErr {
				t.Fatalf("parseFilterInput(%q) err=%v, wantErr=%v", tc.input, err, tc.wantErr)
			}
			if !tc.wantErr {
				if got.Column != tc.want.Column {
					t.Errorf("column: got %q, want %q", got.Column, tc.want.Column)
				}
				if got.Operator != tc.want.Operator {
					t.Errorf("operator: got %q, want %q", got.Operator, tc.want.Operator)
				}
				if got.Value != tc.want.Value {
					t.Errorf("value: got %q, want %q", got.Value, tc.want.Value)
				}
			}
		})
	}
}
