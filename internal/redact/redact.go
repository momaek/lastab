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
	{regexp.MustCompile(`(?s)-----BEGIN [A-Z0-9 ]*PRIVATE KEY-----(.*?)-----END [A-Z0-9 ]*PRIVATE KEY-----`), 1,
		[]string{"PRIVATE KEY-----"}, false},
	// Authorization headers: keep the scheme.
	{regexp.MustCompile(`(?i)\bauthorization\s*[:=]\s*["']?(?:bearer|basic|token|bot)?\s*([^\s"',;]+)`), 1,
		[]string{"authorization"}, true},
	{regexp.MustCompile(`(?i)\bbearer\s+([A-Za-z0-9\-._~+/]{8,}=*)`), 1, []string{"bearer"}, true},
	// key = value, key: value, "key": "value", --key value.
	{regexp.MustCompile(`(?i)["']?\b((?:[a-z0-9_.-]*[_.-])?(?:api[_-]?key|apikey|secret|token|passwd|password|pwd|access[_-]?key|client[_-]?secret|private[_-]?key)(?:[_.-][a-z0-9_.-]*)?)["']?\s*(?:[:=]|=>)\s*["']?([^\s"',;&}]{4,})`), 2,
		[]string{"key", "secret", "token", "passw", "pwd"}, true},
	// Well-known token formats.
	{regexp.MustCompile(`\bsk-ant-[A-Za-z0-9\-_]{16,}`), 0, []string{"sk-ant-"}, false},
	{regexp.MustCompile(`\bsk-(?:proj-)?[A-Za-z0-9\-_]{16,}`), 0, []string{"sk-"}, false},
	{regexp.MustCompile(`\bgh[pousr]_[A-Za-z0-9]{20,}`), 0, []string{"ghp_", "gho_", "ghu_", "ghs_", "ghr_"}, false},
	{regexp.MustCompile(`\bgithub_pat_[A-Za-z0-9_]{20,}`), 0, []string{"github_pat_"}, false},
	{regexp.MustCompile(`\bxox[abposr]-[A-Za-z0-9-]{10,}`), 0, []string{"xox"}, false},
	{regexp.MustCompile(`\b(?:AKIA|ASIA)[0-9A-Z]{16}\b`), 0, []string{"AKIA", "ASIA"}, false},
	{regexp.MustCompile(`\bAIza[0-9A-Za-z\-_]{35}\b`), 0, []string{"AIza"}, false},
	{regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}`), 0, []string{"eyJ"}, false},
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
