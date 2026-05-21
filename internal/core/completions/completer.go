// Package completions implements a vendor-neutral SQL completion engine.
//
// Suggestions are computed from an in-memory schema (tables and columns) plus
// a per-dialect keyword list. Adding a new vendor is a matter of supplying a
// db.Dialect value and an entry in dialectKeywords — the engine itself stays
// the same.
package completions

import (
	"sort"
	"strings"

	"github.com/polesen/datacow/internal/core/db"
	"github.com/polesen/datacow/internal/core/schema"
)

// Kind classifies a Suggestion by its source.
type Kind int

const (
	KindTable Kind = iota
	KindColumn
	KindKeyword
)

// Suggestion is a single ranked completion candidate.
type Suggestion struct {
	Text   string // text to insert (already quoted if needed)
	Kind   Kind
	Detail string // e.g. column type for KindColumn; empty for keywords
}

// Completer ranks completions for SQL text.
type Completer struct {
	tables   []schema.Table
	dialect  db.Dialect
	keywords []string
	openQ    byte
	closeQ   byte
}

// New builds a Completer from the schema tables already in memory.
// dialect selects the per-vendor keyword extensions and identifier quoting.
func New(tables []schema.Table, dialect db.Dialect) *Completer {
	kws := make([]string, 0, len(commonKeywords)+len(dialectKeywords[dialect]))
	kws = append(kws, commonKeywords...)
	kws = append(kws, dialectKeywords[dialect]...)
	open, close := quoteChars(dialect)
	return &Completer{
		tables:   tables,
		dialect:  dialect,
		keywords: kws,
		openQ:    open,
		closeQ:   close,
	}
}

// Complete returns ranked suggestions for the current cursor position.
// sql is the full editor content; cursorPos is a byte offset into sql.
// The returned slice is never nil.
func (c *Completer) Complete(sql string, cursorPos int) []Suggestion {
	if cursorPos < 0 {
		cursorPos = 0
	} else if cursorPos > len(sql) {
		cursorPos = len(sql)
	}

	// Identify the prefix at the cursor. The prefix is the run of identifier
	// characters immediately left of the cursor. If the character at the cursor
	// is a dot, include it so "o|." is treated the same as "o.|".
	start := cursorPos
	for start > 0 && isIdentChar(sql[start-1]) {
		start--
	}
	end := cursorPos
	if end < len(sql) && sql[end] == '.' {
		end++
	}
	prefix := sql[start:end]

	tokens := tokenize(sql)
	lastKeyword := findLastContextKeyword(tokens, start)
	aliases := parseAliases(tokens)

	qualifier := ""
	bare := prefix
	if before, after, ok := strings.Cut(prefix, "."); ok {
		qualifier = before
		bare = after
	}

	out := []Suggestion{}

	switch {
	case isTableContextKW(lastKeyword):
		for _, t := range c.tables {
			if hasPrefixFold(t.Name, prefix) {
				out = append(out, Suggestion{
					Text: c.quoteIdent(t.Name),
					Kind: KindTable,
				})
			}
		}

	case isColumnContextKW(lastKeyword) && qualifier != "":
		tbl := c.resolveQualifier(qualifier, aliases)
		if tbl != nil {
			for _, col := range tbl.Columns {
				if hasPrefixFold(col.Name, bare) {
					out = append(out, Suggestion{
						Text:   c.quoteIdent(col.Name),
						Kind:   KindColumn,
						Detail: col.Type,
					})
				}
			}
		}

	case isColumnContextKW(lastKeyword):
		seen := make(map[string]bool)
		for _, t := range c.tables {
			for _, col := range t.Columns {
				if seen[col.Name] {
					continue
				}
				if hasPrefixFold(col.Name, bare) {
					seen[col.Name] = true
					out = append(out, Suggestion{
						Text:   c.quoteIdent(col.Name),
						Kind:   KindColumn,
						Detail: col.Type,
					})
				}
			}
		}
		for _, kw := range c.keywords {
			if hasPrefixFold(kw, bare) {
				out = append(out, Suggestion{Text: kw, Kind: KindKeyword})
			}
		}

	default:
		for _, kw := range c.keywords {
			if hasPrefixFold(kw, prefix) {
				out = append(out, Suggestion{Text: kw, Kind: KindKeyword})
			}
		}
	}

	sortSuggestions(out, prefix)
	return out
}

// resolveQualifier finds the table referenced by a qualifier in `qual.col`.
// Resolution order: alias map, exact table name, unique prefix match.
// Returns nil when the qualifier is ambiguous or unknown.
func (c *Completer) resolveQualifier(qual string, aliases map[string]string) *schema.Table {
	if qual == "" {
		return nil
	}
	qualLower := strings.ToLower(qual)

	if real, ok := aliases[qualLower]; ok {
		for i := range c.tables {
			if strings.EqualFold(c.tables[i].Name, real) {
				return &c.tables[i]
			}
		}
	}
	for i := range c.tables {
		if strings.EqualFold(c.tables[i].Name, qual) {
			return &c.tables[i]
		}
	}
	var match *schema.Table
	for i := range c.tables {
		if strings.HasPrefix(strings.ToLower(c.tables[i].Name), qualLower) {
			if match != nil {
				return nil
			}
			match = &c.tables[i]
		}
	}
	return match
}

// quoteIdent wraps an identifier in the dialect's quote characters if it is a
// reserved word or contains a character that would break unquoted use.
func (c *Completer) quoteIdent(name string) string {
	if !needsQuoting(name) {
		return name
	}
	return string(c.openQ) + name + string(c.closeQ)
}

func needsQuoting(name string) bool {
	if name == "" {
		return false
	}
	for i := 0; i < len(name); i++ {
		if !isBareIdentByte(name[i]) {
			return true
		}
	}
	return reservedQuoteWords[strings.ToUpper(name)]
}

func isBareIdentByte(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') ||
		(b >= '0' && b <= '9') || b == '_'
}

// reservedQuoteWords names that should be quoted on insertion to avoid
// colliding with SQL keywords.
var reservedQuoteWords = map[string]bool{
	"SELECT": true, "FROM": true, "WHERE": true, "ORDER": true, "GROUP": true,
	"USER": true, "TABLE": true, "INDEX": true, "JOIN": true, "ON": true,
	"AS": true, "AND": true, "OR": true, "NOT": true, "NULL": true, "IS": true,
	"IN": true, "LIKE": true, "BETWEEN": true, "DISTINCT": true, "UNION": true,
}

func quoteChars(d db.Dialect) (byte, byte) {
	switch d {
	case db.DialectMySQL:
		return '`', '`'
	case db.DialectMSSQL:
		return '[', ']'
	default:
		return '"', '"'
	}
}

func sortSuggestions(s []Suggestion, prefix string) {
	sort.SliceStable(s, func(i, j int) bool {
		ei := prefix != "" && strings.HasPrefix(s[i].Text, prefix)
		ej := prefix != "" && strings.HasPrefix(s[j].Text, prefix)
		if ei != ej {
			return ei
		}
		if s[i].Kind != s[j].Kind {
			return kindRank(s[i].Kind) < kindRank(s[j].Kind)
		}
		return s[i].Text < s[j].Text
	})
}

func kindRank(k Kind) int {
	switch k {
	case KindTable:
		return 0
	case KindColumn:
		return 1
	default:
		return 2
	}
}

func hasPrefixFold(s, prefix string) bool {
	if prefix == "" {
		return true
	}
	if len(s) < len(prefix) {
		return false
	}
	return strings.EqualFold(s[:len(prefix)], prefix)
}

func isIdentChar(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') ||
		(b >= '0' && b <= '9') || b == '_' || b == '.'
}
