package views

import (
	"fmt"
	"strings"

	"github.com/beetio/datacow/internal/core/dataset"
)

// parseFilterInput parses "column<op>value" expressions (e.g. "price>=10", "name like foo").
func parseFilterInput(input string) (dataset.Filter, error) {
	input = strings.TrimSpace(input)

	type opDef struct {
		token string
		norm  string
	}
	// Order: longer operators first so ">=" matches before ">"
	ops := []opDef{
		{">=", ">="},
		{"<=", "<="},
		{"like", "like"},
		{"=", "="},
		{">", ">"},
		{"<", "<"},
	}

	lower := strings.ToLower(input)
	for _, op := range ops {
		idx := strings.Index(lower, op.token)
		if idx <= 0 {
			continue
		}
		col := strings.TrimSpace(input[:idx])
		val := strings.TrimSpace(input[idx+len(op.token):])
		if col == "" {
			continue
		}
		return dataset.Filter{Column: col, Operator: op.norm, Value: val}, nil
	}
	return dataset.Filter{}, fmt.Errorf("invalid filter %q: expected column<op>value (op: =, like, >, <, >=, <=)", input)
}
