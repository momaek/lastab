// Package redact masks secrets in text shown outside the editor: list
// titles and previews, the switcher, and the CLI's output. It masks only
// values, keeping keys and structure so the user still recognizes the line.
package redact

import (
	"regexp"
	"strings"
	"unicode"
)

// Mask is what a secret value is replaced with.
const Mask = "••••••••"

type rule struct {
	re *regexp.Regexp
	// group is the submatch holding the secret; 0 masks the whole match.
	group int
	// lits are literals every match contains one of, to skip the regexp
	// when the text has none. For a (?i) rule (fold) they are lower case
	// and looked for in the folded text.
	lits []string
	fold bool
}

var rules = []rule{
	// PEM private keys: keep the armor lines.
	{re: regexp.MustCompile(`(?s)-----BEGIN [A-Z0-9 ]*PRIVATE KEY-----(.*?)-----END [A-Z0-9 ]*PRIVATE KEY-----`), group: 1,
		lits: []string{"PRIVATE KEY-----"}},
	// Authorization headers: keep the scheme.
	{re: regexp.MustCompile(`(?i)\bauthorization\s*[:=]\s*["']?(?:bearer|basic|token|bot)?\s*([^\s"',;]+)`), group: 1,
		lits: []string{"authorization"}, fold: true},
	{re: regexp.MustCompile(`(?i)\bbearer\s+([A-Za-z0-9\-._~+/]{8,}=*)`), group: 1, lits: []string{"bearer"}, fold: true},
	// key = value, key: value, "key": "value", --key value.
	{re: regexp.MustCompile(`(?i)["']?\b((?:[a-z0-9_.-]*[_.-])?(?:api[_-]?key|apikey|secret|token|passwd|password|pwd|access[_-]?key|client[_-]?secret|private[_-]?key)(?:[_.-][a-z0-9_.-]*)?)["']?\s*(?:[:=]|=>)\s*["']?([^\s"',;&}]{4,})`), group: 2,
		lits: []string{"key", "secret", "token", "passw", "pwd"}, fold: true},
	// Well-known token formats.
	{re: regexp.MustCompile(`\bsk-ant-[A-Za-z0-9\-_]{16,}`), lits: []string{"sk-ant-"}},
	{re: regexp.MustCompile(`\bsk-(?:proj-)?[A-Za-z0-9\-_]{16,}`), lits: []string{"sk-"}},
	{re: regexp.MustCompile(`\bgh[pousr]_[A-Za-z0-9]{20,}`), lits: []string{"ghp_", "gho_", "ghu_", "ghs_", "ghr_"}},
	{re: regexp.MustCompile(`\bgithub_pat_[A-Za-z0-9_]{20,}`), lits: []string{"github_pat_"}},
	{re: regexp.MustCompile(`\bxox[abposr]-[A-Za-z0-9-]{10,}`), lits: []string{"xox"}},
	{re: regexp.MustCompile(`\b(?:AKIA|ASIA)[0-9A-Z]{16}\b`), lits: []string{"AKIA", "ASIA"}},
	{re: regexp.MustCompile(`\bAIza[0-9A-Za-z\-_]{35}\b`), lits: []string{"AIza"}},
	{re: regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}`), lits: []string{"eyJ"}},
}

// fold maps s so that a (?i) match of a lower-case ASCII literal in s is a
// plain match in fold(s). Besides the usual lower-casing, (?i) folds the
// Kelvin sign to k and the long s to s.
func fold(s string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case '\u212A':
			return 'k'
		case '\u017F':
			return 's'
		}
		return unicode.ToLower(r)
	}, s)
}

func (r rule) mayMatch(s string, folded *string) bool {
	if r.lits == nil {
		return true
	}
	t := s
	if r.fold {
		if *folded == "" {
			*folded = fold(s)
		}
		t = *folded
	}
	for _, l := range r.lits {
		if strings.Contains(t, l) {
			return true
		}
	}
	return false
}

// span is a byte range to mask.
type span struct{ start, end int }

// Text returns s with secret values masked, and whether anything was.
func Text(s string) (string, bool) { return text(s, true) }

func text(s string, prefilter bool) (string, bool) {
	var spans []span
	var folded string
	for _, r := range rules {
		if prefilter && !r.mayMatch(s, &folded) {
			continue
		}
		for _, m := range r.re.FindAllStringSubmatchIndex(s, -1) {
			i := 2 * r.group
			if m[i] < 0 || m[i] == m[i+1] {
				continue
			}
			spans = append(spans, span{m[i], m[i+1]})
		}
	}
	spans = bareTokens(s, spans)
	if len(spans) == 0 {
		return s, false
	}
	spans = merge(spans)
	var b strings.Builder
	last := 0
	for _, sp := range spans {
		b.WriteString(s[last:sp.start])
		if strings.TrimSpace(s[sp.start:sp.end]) == "" {
			b.WriteString(s[sp.start:sp.end])
		} else if strings.Contains(s[sp.start:sp.end], "\n") {
			// A PEM body: keep its line breaks out of the mask.
			b.WriteString("\n" + Mask + "\n")
		} else {
			b.WriteString(Mask)
		}
		last = sp.end
	}
	b.WriteString(s[last:])
	return b.String(), true
}

// String is Text without the flag.
func String(s string) string {
	out, _ := Text(s)
	return out
}

func merge(spans []span) []span {
	// Insertion sort: there are few.
	for i := 1; i < len(spans); i++ {
		for j := i; j > 0 && spans[j].start < spans[j-1].start; j-- {
			spans[j], spans[j-1] = spans[j-1], spans[j]
		}
	}
	out := spans[:1]
	for _, sp := range spans[1:] {
		last := &out[len(out)-1]
		if sp.start <= last.end {
			if sp.end > last.end {
				last.end = sp.end
			}
			continue
		}
		out = append(out, sp)
	}
	return out
}

// bareTokens appends to spans every bare random token in s: a run of 20 or
// more letters and digits, with no key or prefix to give it away, as an
// API key pasted on its own, or a password after "----" in an account
// list. A run is taken whole, with the identifier, path and base64
// characters _ - . + / around it, so that a token inside a path, a domain
// or a snake_case name is left alone; bareToken then tells random runs
// from identifiers and model names. A byte scan rather than a regexp: it
// runs on every text, and needs no literal to skip it.
func bareTokens(s string, spans []span) []span {
	for i := 0; i < len(s); {
		if !isRunByte(s[i]) {
			i++
			continue
		}
		// A run ends before "--": the separator of an account list
		// (mail----password), or of a command-line flag.
		j := i
		for j < len(s) && isRunByte(s[j]) && !strings.HasPrefix(s[j:], "--") {
			j++
		}
		k := j
		for k < len(s) && s[k] == '=' { // base64 padding
			k++
		}
		// After a backslash the first letter is an escape (\n, \t in a
		// JSON string), not part of the run.
		start := i
		if i > 0 && s[i-1] == '\\' {
			start++
		}
		if j-start >= 20 && bareToken(s[start:j]) {
			spans = append(spans, span{start, k})
		}
		i = max(k, i+1)
		for i < len(s) && s[i] == '-' {
			i++
		}
	}
	return spans
}

// isRunByte reports whether b belongs in a run bareTokens looks at.
func isRunByte(b byte) bool {
	return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9' ||
		b == '_' || b == '-' || b == '.' || b == '+' || b == '/'
}

// wordChunk splits an identifier into words and digit runs, as camelCase
// does: Base64Encoded is Base, 64, Encoded.
var wordChunk = regexp.MustCompile(`[A-Z]?[a-z]+|[A-Z]+|[0-9]+`)

// clusters4 is four consonants in a row, which few words have.
var clusters4 = regexp.MustCompile(`[bcdfghjklmnpqrstvwxz]{4}`)

// bareToken reports whether s, 20 or more ASCII letters and digits, looks
// random rather than written. In mixed case (base62, as most API keys) it
// must have all three kinds of character, switch between them often and be
// mostly not words: on random strings that catches about 91% at 20
// characters and 98% at 40. In one case (base32, as 2FA secrets) it must
// be 32 or more long, not hex, have digits in several places and be mostly
// not words: about 81% of random base32 at 32 characters, 89% at 40.
// Identifiers with digits (Base64EncodedStringV2, utf8ToUnicodeConverter),
// model names, ULIDs and hex hashes stay readable.
func bareToken(s string) bool {
	var up, lo, dg, beyondHex bool
	switches, digitRuns, prev := 0, 0, -1
	for i := 0; i < len(s); i++ {
		c := 2
		switch ch := s[i]; {
		case ch >= 'A' && ch <= 'Z':
			c, up = 0, true
			beyondHex = beyondHex || ch > 'F'
		case ch >= 'a' && ch <= 'z':
			c, lo = 1, true
			beyondHex = beyondHex || ch > 'f'
		case ch >= '0' && ch <= '9':
			dg = true
			if prev != 2 {
				digitRuns++
			}
		default: // _ - . + /: a name, a path, a version or base64 with symbols
			return false
		}
		if prev >= 0 && c != prev {
			switches++
		}
		prev = c
	}
	if !dg {
		return false
	}
	if up && lo {
		return float64(switches) >= 0.4*float64(len(s)-1) && wordShare(wordChunk, s) <= 0.5
	}
	// One case: camel humps don't split words, so look for words between
	// the digits.
	return len(s) >= 32 && beyondHex && digitRuns >= 3 && wordShare(letterRun, strings.ToLower(s)) <= 0.5
}

// wordShare is the share of s in chunks (found by re) that could be words.
func wordShare(re *regexp.Regexp, s string) float64 {
	words := 0
	for _, w := range re.FindAllString(s, -1) {
		if w[0] > '9' && isWordLike(w) {
			words += len(w)
		}
	}
	return float64(words) / float64(len(s))
}

// letterRun is a run of lower-case letters.
var letterRun = regexp.MustCompile(`[a-z]+`)

// isWordLike reports whether a run of letters could be a word: three or
// more letters, a vowel, and no four consonants in a row.
func isWordLike(w string) bool {
	l := strings.ToLower(w)
	return len(l) >= 3 && strings.ContainsAny(l, "aeiouy") && !clusters4.MatchString(l)
}
