package store

import (
	"context"
	"strings"
	"unicode/utf8"
)

// MinTrigram is the shortest query the trigram index can match. Shorter
// queries fall back to a LIKE scan.
const MinTrigram = 3

// Search returns snippets whose title or body contains q (case-insensitive
// substring), ranked by relevance and recency.
func (s *Store) Search(ctx context.Context, q string, o ListOptions) ([]Snippet, error) {
	q = strings.TrimSpace(q)
	if q == "" {
		return s.List(ctx, o)
	}
	var args []any
	var query string
	if utf8.RuneCountInString(q) >= MinTrigram {
		args = append(args, ftsPhrase(q))
		// bm25 is negative (more negative is better); nudge recent snippets up.
		query = `SELECT ` + snippetCols + ` FROM snippet_fts f JOIN snippet s ON s.rowid = f.rowid
			WHERE snippet_fts MATCH ?` + o.where(&args) + `
			ORDER BY bm25(snippet_fts, 2.0, 1.0) - (s.modified_at / 86400000.0) * 0.05, s.modified_at DESC`
	} else {
		// A full scan. LIKE folds only ASCII case, so a query without ASCII
		// letters (周会, {") can use instr, which scans about a third faster.
		match := `(s.title LIKE ? ESCAPE '\' OR s.body LIKE ? ESCAPE '\')`
		pat := "%" + likeEscape(q) + "%"
		if !hasASCIILetter(q) {
			match, pat = `(instr(s.title, ?) > 0 OR instr(s.body, ?) > 0)`, q
		}
		args = append(args, pat, pat)
		query = `SELECT ` + snippetCols + ` FROM snippet s
			WHERE ` + match + o.where(&args) + `
			ORDER BY s.modified_at DESC`
	}
	if o.Limit > 0 {
		query += ` LIMIT ?`
		args = append(args, o.Limit)
	}
	return s.querySnippets(ctx, query, args...)
}

// ftsPhrase quotes q as one FTS5 phrase, so symbols such as {" and -- are
// matched literally.
func ftsPhrase(q string) string {
	return `"` + strings.ReplaceAll(q, `"`, `""`) + `"`
}

func hasASCIILetter(q string) bool {
	for i := 0; i < len(q); i++ {
		if c := q[i] | 0x20; c >= 'a' && c <= 'z' {
			return true
		}
	}
	return false
}

func likeEscape(q string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return r.Replace(q)
}
