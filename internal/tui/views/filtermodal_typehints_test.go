package views

import "testing"

func TestResolveTypeCategory(t *testing.T) {
	tests := []struct {
		colType string
		want    typeCategory
	}{
		{"text", typeCatText},
		{"character varying", typeCatText},
		{"varchar(255)", typeCatText},
		{"uuid", typeCatText},
		{"jsonb", typeCatJSON},
		{"json", typeCatJSON},
		{"unknown_type", typeCatText},
		{"integer", typeCatInteger},
		{"int4", typeCatInteger},
		{"bigint", typeCatInteger},
		{"smallint", typeCatInteger},
		{"serial", typeCatInteger},
		{"bigserial", typeCatInteger},
		{"numeric", typeCatNumeric},
		{"decimal(10,2)", typeCatNumeric},
		{"double precision", typeCatNumeric},
		{"float8", typeCatNumeric},
		{"real", typeCatNumeric},
		{"date", typeCatDateTime},
		{"timestamp", typeCatDateTime},
		{"timestamptz", typeCatDateTime},
		{"time", typeCatDateTime},
		{"boolean", typeCatBoolean},
		{"bool", typeCatBoolean},
		{"BOOLEAN", typeCatBoolean},
		{"INT4", typeCatInteger},
	}
	for _, tc := range tests {
		t.Run(tc.colType, func(t *testing.T) {
			got := resolveTypeCategory(tc.colType)
			if got != tc.want {
				t.Errorf("resolveTypeCategory(%q) = %d, want %d", tc.colType, got, tc.want)
			}
		})
	}
}

func TestAllowedOps(t *testing.T) {
	tests := []struct {
		cat  typeCategory
		want []string
	}{
		{typeCatText, []string{"=", "like"}},
		{typeCatBoolean, []string{"="}},
		{typeCatJSON, []string{"="}},
		{typeCatInteger, []string{"=", ">", "<", ">=", "<="}},
		{typeCatNumeric, []string{"=", ">", "<", ">=", "<="}},
		{typeCatDateTime, []string{"=", ">", "<", ">=", "<="}},
	}
	for _, tc := range tests {
		t.Run("", func(t *testing.T) {
			ops := allowedOps(tc.cat)
			if len(ops) != len(tc.want) {
				t.Fatalf("allowedOps(%d): got %v, want %v", tc.cat, ops, tc.want)
			}
			for i, op := range ops {
				if op != tc.want[i] {
					t.Errorf("allowedOps(%d)[%d]: got %q, want %q", tc.cat, i, op, tc.want[i])
				}
			}
		})
	}
}

func TestIsValidValueRune(t *testing.T) {
	tests := []struct {
		cat      typeCategory
		existing string
		r        rune
		want     bool
	}{
		// text: accepts any rune
		{typeCatText, "", 'a', true},
		{typeCatText, "hello", ' ', true},
		{typeCatText, "", '%', true},
		// integer: digits and leading minus only
		{typeCatInteger, "", '-', true},
		{typeCatInteger, "1", '-', false},
		{typeCatInteger, "", '5', true},
		{typeCatInteger, "", 'a', false},
		{typeCatInteger, "", '.', false},
		// numeric: digits, one dot, leading minus
		{typeCatNumeric, "", '-', true},
		{typeCatNumeric, "1", '-', false},
		{typeCatNumeric, "1", '.', true},
		{typeCatNumeric, "1.5", '.', false},
		{typeCatNumeric, "", '3', true},
		{typeCatNumeric, "", 'x', false},
		// boolean: never accepts typed runes
		{typeCatBoolean, "", 't', false},
		{typeCatBoolean, "", 'f', false},
		// datetime: accepts any rune
		{typeCatDateTime, "", '2', true},
		{typeCatDateTime, "", '-', true},
		{typeCatDateTime, "", ' ', true},
	}
	for _, tc := range tests {
		got := isValidValueRune(tc.cat, tc.existing, tc.r)
		if got != tc.want {
			t.Errorf("isValidValueRune(%d, %q, %q) = %v, want %v", tc.cat, tc.existing, tc.r, got, tc.want)
		}
	}
}

func TestTypeTip(t *testing.T) {
	// Each category must return a non-empty tip.
	cats := []typeCategory{typeCatText, typeCatInteger, typeCatNumeric, typeCatDateTime, typeCatBoolean, typeCatJSON}
	for _, cat := range cats {
		if tip := typeTip(cat); tip == "" {
			t.Errorf("typeTip(%d) returned empty string", cat)
		}
	}
}
