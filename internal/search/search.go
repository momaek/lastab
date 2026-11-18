// Package search finds snippets and makes the previews lists show: a few
// lines of the body around the first match, masked, with the matches'
// ranges for highlighting.
package search

import (
	"context"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/momaek/lastab/internal/redact"
	"github.com/momaek/lastab/internal/store"
)

// Range is a highlighted byte range of a preview.
type Range struct{ Start, End int }

// Result is a snippet with its preview.
type Result struct {
	store.Snippet
	Preview    string
	Highlights []Range
}

// Searcher runs queries against a store.
type Searcher struct {
	Store *store.Store
}

// Query searches all snippets, closed ones too. An empty query lists them.
func (s Searcher) Query(ctx context.Context, q string, o store.ListOptions) ([]Result, error) {
	sns, err := s.Store.Search(ctx, q, o)
	if err != nil {
		return nil, err
	}
	out := make([]Result, len(sns))
	for i, sn := range sns {
		p, hs := Preview(sn.Body, q, 160)
		out[i] = Result{Snippet: sn, Preview: p, Highlights: hs}
	}
	return out, nil
}

// Matches reports whether the title or body contains q, ignoring case: the
// switcher's filter, which runs in memory over the open tabs.
func Matches(title, body, q string) bool {
	q = strings.TrimSpace(q)
	if q == "" {
		return true
	}
	return indexFold(title, q) >= 0 || indexFold(body, q) >= 0
}

// Preview returns about max bytes of body around the first match of q,
// masked, with the matches in it. Without a match it is the body's start.
func Preview(body, q string, max int) (string, []Range) {
	masked := redact.String(body)
	q = strings.TrimSpace(q)
	at := -1
	if q != "" {
		at = indexFold(masked, q)
	}
	start := 0
	if at > max/3 {
		start = at - max/3
		// Start at a line or a word, not mid-character.
		if nl := strings.LastIndexByte(masked[:at], '\n'); nl >= start {
			start = nl + 1
		} else {
			for start < at && !utf8.RuneStart(masked[start]) {
				start++
			}
		}
	}
	end := min(len(masked), start+max)
	for end < len(masked) && !utf8.RuneStart(masked[end]) {
		end++
	}
	text := masked[start:end]
	prefix := ""
	if start > 0 {
		prefix = "…"
	}
	if end < len(masked) {
		text = strings.TrimRightFunc(text, unicode.IsSpace) + "…"
	}
	text = prefix + text
	return text, Highlights(text, q)
}

// Highlights returns the byte ranges of text matching q, ignoring case.
func Highlights(text, q string) []Range {
	q = strings.TrimSpace(q)
	if q == "" {
		return nil
	}
	var hs []Range
	off := 0
	for off < len(text) {
		i := indexFold(text[off:], q)
		if i < 0 {
			break
		}
		l := foldLen(text[off+i:], q)
		hs = append(hs, Range{off + i, off + i + l})
		off += i + max1(l)
	}
	return hs
}

func max1(n int) int {
	if n < 1 {
		return 1
	}
	return n
}

// indexFold is strings.Index ignoring case (Unicode simple folding).
func indexFold(s, sub string) int {
	if sub == "" {
		return 0
	}
	for i := range s {
		if hasPrefixFold(s[i:], sub) {
			return i
		}
	}
	return -1
}

func hasPrefixFold(s, prefix string) bool {
	return foldLen(s, prefix) >= 0
}

// foldLen returns how many bytes of s match prefix ignoring case, or -1.
func foldLen(s, prefix string) int {
	i := 0
	for _, pr := range prefix {
		if i >= len(s) {
			return -1
		}
		sr, size := utf8.DecodeRuneInString(s[i:])
		if sr != pr && unicode.SimpleFold(sr) != pr && !equalFold(sr, pr) {
			return -1
		}
		i += size
	}
	return i
}

func equalFold(a, b rune) bool {
	return unicode.ToLower(a) == unicode.ToLower(b)
}
