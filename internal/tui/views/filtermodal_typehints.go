package views

import "strings"

type typeCategory int

const (
	typeCatText     typeCategory = iota
	typeCatInteger
	typeCatNumeric
	typeCatDateTime
	typeCatBoolean
	typeCatJSON
)

// resolveTypeCategory maps a driver type string to a broad category using
// case-insensitive substring matching. Unknown types fall through to text.
func resolveTypeCategory(colType string) typeCategory {
	t := strings.ToLower(colType)
	switch {
	case strings.Contains(t, "bool"):
		return typeCatBoolean
	case strings.Contains(t, "json"):
		return typeCatJSON
	case strings.Contains(t, "int") || strings.Contains(t, "serial"):
		return typeCatInteger
	case strings.Contains(t, "numeric") || strings.Contains(t, "decimal") ||
		strings.Contains(t, "float") || strings.Contains(t, "double") ||
		strings.Contains(t, "real"):
		return typeCatNumeric
	case strings.Contains(t, "date") || strings.Contains(t, "time") ||
		strings.Contains(t, "timestamp"):
		return typeCatDateTime
	default:
		return typeCatText
	}
}

// allowedOps returns the operators valid for a type category.
func allowedOps(cat typeCategory) []string {
	switch cat {
	case typeCatText:
		return []string{"=", "like"}
	case typeCatBoolean, typeCatJSON:
		return []string{"="}
	default: // integer, numeric, date/time
		return []string{"=", ">", "<", ">=", "<="}
	}
}

// typeTip returns a one-line user tip for the given category.
func typeTip(cat typeCategory) string {
	switch cat {
	case typeCatInteger:
		return "integer: whole number, e.g. 42"
	case typeCatNumeric:
		return "numeric: decimals allowed, e.g. 3.14"
	case typeCatDateTime:
		return "date/time: ISO format, e.g. 2024-01-15 or 2024-01-15 10:00:00"
	case typeCatBoolean:
		return "boolean: true or false"
	case typeCatJSON:
		return "JSON: only = is supported (exact equality)"
	default:
		return "text: no quotes needed. like accepts % wildcards, e.g. foo%"
	}
}

// isValidValueRune reports whether rune r may be appended to existing in
// the value field for the given type category.
func isValidValueRune(cat typeCategory, existing string, r rune) bool {
	switch cat {
	case typeCatInteger:
		if r == '-' {
			return existing == ""
		}
		return r >= '0' && r <= '9'
	case typeCatNumeric:
		if r == '-' {
			return existing == ""
		}
		if r == '.' {
			return !strings.Contains(existing, ".")
		}
		return r >= '0' && r <= '9'
	case typeCatBoolean:
		return false // value is toggled, not typed
	default:
		return true
	}
}
