package completions

import "strings"

// tok is one identifier-like token from the SQL text, plus its byte offset.
type tok struct {
	text string
	pos  int
}

// tokenize splits sql into identifier tokens. Whitespace and punctuation are
// dropped; dots are treated as identifier characters so that "table.column" is
// returned as one token. This is sufficient for context detection — we only
// look at tokens to find the surrounding keyword.
func tokenize(sql string) []tok {
	var out []tok
	i := 0
	for i < len(sql) {
		b := sql[i]
		if b == ' ' || b == '\t' || b == '\n' || b == '\r' {
			i++
			continue
		}
		if isIdentChar(b) {
			start := i
			for i < len(sql) && isIdentChar(sql[i]) {
				i++
			}
			out = append(out, tok{text: sql[start:i], pos: start})
			continue
		}
		i++
	}
	return out
}

// findLastContextKeyword walks the tokens backwards from beforePos and
// returns the upper-cased text of the first context keyword it finds.
// Returns "" when no keyword precedes the prefix.
func findLastContextKeyword(tokens []tok, beforePos int) string {
	for i := len(tokens) - 1; i >= 0; i-- {
		if tokens[i].pos >= beforePos {
			continue
		}
		u := strings.ToUpper(tokens[i].text)
		if allContextKeywords[u] {
			return u
		}
	}
	return ""
}

// parseAliases walks the token stream and builds an alias→table-name map by
// finding `FROM|JOIN|UPDATE|INTO <table> [AS] <alias>` patterns.
func parseAliases(tokens []tok) map[string]string {
	aliases := map[string]string{}
	for i := range tokens {
		u := strings.ToUpper(tokens[i].text)
		if !tableContextKeywords[u] {
			continue
		}
		if i+1 >= len(tokens) {
			continue
		}
		table := tokens[i+1].text
		if strings.Contains(table, ".") {
			// Skip schema-qualified names — alias extraction would be unreliable.
			continue
		}
		j := i + 2
		if j < len(tokens) && strings.EqualFold(tokens[j].text, "AS") {
			j++
		}
		if j >= len(tokens) {
			continue
		}
		alias := tokens[j].text
		aliasU := strings.ToUpper(alias)
		if allContextKeywords[aliasU] {
			continue
		}
		if strings.Contains(alias, ".") {
			continue
		}
		aliases[strings.ToLower(alias)] = table
	}
	return aliases
}
