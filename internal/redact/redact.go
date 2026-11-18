// Package redact masks secrets in text shown outside the editor: list
// titles and previews, the switcher, and the CLI's output. It masks only
// values, keeping keys and structure so the user still recognizes the line.
package redact

import (
	"regexp"
	"strings"
)

// Mask is what a secret value is replaced with.
const Mask = "••••••••"

type rule struct {
	re *regexp.Regexp
	// group is the submatch holding the secret; 0 masks the whole match.
	group int
}

var rules = []rule{
	// PEM private keys: keep the armor lines.
	{regexp.MustCompile(`(?s)-----BEGIN [A-Z0-9 ]*PRIVATE KEY-----(.*?)-----END [A-Z0-9 ]*PRIVATE KEY-----`), 1},
	// Authorization headers: keep the scheme.
	{regexp.MustCompile(`(?i)\bauthorization\s*[:=]\s*["']?(?:bearer|basic|token|bot)?\s*([^\s"',;]+)`), 1},
	{regexp.MustCompile(`(?i)\bbearer\s+([A-Za-z0-9\-._~+/]{8,}=*)`), 1},
	// key = value, key: value, "key": "value", --key value.
	{regexp.MustCompile(`(?i)["']?\b((?:[a-z0-9_.-]*[_.-])?(?:api[_-]?key|apikey|secret|token|passwd|password|pwd|access[_-]?key|client[_-]?secret|private[_-]?key)(?:[_.-][a-z0-9_.-]*)?)["']?\s*(?:[:=]|=>)\s*["']?([^\s"',;&}]{4,})`), 2},
	// Well-known token formats.
	{regexp.MustCompile(`\bsk-ant-[A-Za-z0-9\-_]{16,}`), 0},
	{regexp.MustCompile(`\bsk-(?:proj-)?[A-Za-z0-9\-_]{16,}`), 0},
	{regexp.MustCompile(`\bgh[pousr]_[A-Za-z0-9]{20,}`), 0},
	{regexp.MustCompile(`\bgithub_pat_[A-Za-z0-9_]{20,}`), 0},
	{regexp.MustCompile(`\bxox[abposr]-[A-Za-z0-9-]{10,}`), 0},
	{regexp.MustCompile(`\b(?:AKIA|ASIA)[0-9A-Z]{16}\b`), 0},
	{regexp.MustCompile(`\bAIza[0-9A-Za-z\-_]{35}\b`), 0},
	{regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}`), 0},
}

// span is a byte range to mask.
type span struct{ start, end int }

// Text returns s with secret values masked, and whether anything was.
func Text(s string) (string, bool) {
	var spans []span
	for _, r := range rules {
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
