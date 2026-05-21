package completions

import "github.com/polesen/datacow/internal/core/db"

// commonKeywords are ANSI SQL keywords offered for every dialect.
var commonKeywords = []string{
	"SELECT", "FROM", "WHERE", "JOIN", "LEFT", "INNER", "RIGHT", "FULL",
	"OUTER", "CROSS", "GROUP BY", "ORDER BY", "HAVING", "LIMIT", "OFFSET",
	"INSERT", "UPDATE", "DELETE", "WITH", "AS", "ON", "AND", "OR", "NOT",
	"NULL", "IS", "IN", "LIKE", "BETWEEN", "CASE", "WHEN", "THEN", "ELSE",
	"END", "DISTINCT", "EXISTS", "UNION", "ALL", "COUNT", "SUM", "AVG",
	"MIN", "MAX", "COALESCE", "CAST", "ASC", "DESC",
}

// dialectKeywords are per-vendor keyword extensions added on top of commonKeywords.
// Adding a new vendor = add a []string entry here.
var dialectKeywords = map[db.Dialect][]string{
	db.DialectPostgres: {
		"RETURNING", "ILIKE", "ON CONFLICT", "JSONB", "ARRAY",
		"LATERAL", "WINDOW", "OVER", "PARTITION BY",
	},
	db.DialectMySQL: {
		"STRAIGHT_JOIN", "SQL_NO_CACHE", "REPLACE INTO",
		"CALC_FOUND_ROWS", "ON DUPLICATE KEY",
	},
	db.DialectSQLite: {
		"ROWID", "WITHOUT ROWID", "STRICT", "UNIXEPOCH", "GLOB",
	},
	db.DialectMSSQL: {
		"TOP", "WITH (NOLOCK)", "CROSS APPLY", "OUTER APPLY",
		"IDENTITY", "NOCOUNT", "IIF",
	},
	db.DialectOracle: {
		"ROWNUM", "CONNECT BY", "START WITH", "PRIOR",
		"DUAL", "MINUS", "NVL", "DECODE",
	},
}

// tableContextKeywords are the keywords after which a table name should be suggested.
var tableContextKeywords = map[string]bool{
	"FROM": true, "JOIN": true, "UPDATE": true, "INTO": true,
}

// columnContextKeywords are the keywords after which a column name should be suggested.
var columnContextKeywords = map[string]bool{
	"SELECT": true, "WHERE": true, "ON": true, "SET": true,
	"BY": true, "HAVING": true, "GROUP": true, "ORDER": true,
	"AND": true, "OR": true,
}

// allContextKeywords is the union, used for the left-scan keyword discovery.
var allContextKeywords = func() map[string]bool {
	out := make(map[string]bool, len(tableContextKeywords)+len(columnContextKeywords))
	for k := range tableContextKeywords {
		out[k] = true
	}
	for k := range columnContextKeywords {
		out[k] = true
	}
	return out
}()

func isTableContextKW(kw string) bool  { return tableContextKeywords[kw] }
func isColumnContextKW(kw string) bool { return columnContextKeywords[kw] }
