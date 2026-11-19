// Command autospace spaces Chinese interface text from the Latin letters,
// digits, key symbols and placeholders beside it with a thin space
// (U+2009), as CSS's text-autospace would. MyGo's text has no autospace,
// so the catalog carries the spacing, the same everywhere, and no message
// is spaced by hand. Snippet text is never touched: only the catalog.
//
//	go run ./scripts/autospace          # rewrite locales/zh-Hans.json
//	go run ./scripts/autospace -check   # fail if it needs rewriting (CI)
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"
	"unicode"
)

const thin = " "

// placeholder is a simple ICU argument, {name}, which shows as a word.
var placeholder = regexp.MustCompile(`^\{[A-Za-z_][A-Za-z0-9_]*\}`)

// line is a message of the catalog as `json.MarshalIndent` writes it.
var line = regexp.MustCompile(`^(\s*"([^"@][^"]*)":\s*)("(?:[^"\\]|\\.)*")(,?)$`)

type class int

const (
	other class = iota
	han
	latin
)

func classify(r rune) class {
	switch {
	case unicode.Is(unicode.Han, r):
		return han
	case r < 128 && (unicode.IsLetter(r) || unicode.IsDigit(r)), r == '#', r == '`':
		return latin
	case strings.ContainsRune("⌘⌥⇧⌃", r):
		return latin
	}
	return other
}

type token struct {
	text  string
	class class
	space bool // a single ASCII space
}

// Space returns s with a thin space between Han and Latin text, in place
// of one ordinary space or where there was none.
func Space(s string) string {
	var toks []token
	for i := 0; i < len(s); {
		if m := placeholder.FindString(s[i:]); m != "" {
			toks = append(toks, token{text: m, class: latin})
			i += len(m)
			continue
		}
		r := []rune(s[i:])[0]
		n := len(string(r))
		switch {
		case r == ' ':
			toks = append(toks, token{text: " ", space: true})
		case r == []rune(thin)[0]:
			toks = append(toks, token{text: thin, space: true})
		default:
			toks = append(toks, token{text: string(r), class: classify(r)})
		}
		i += n
	}
	var b strings.Builder
	for i := 0; i < len(toks); i++ {
		t := toks[i]
		if t.space && i > 0 && i+1 < len(toks) && pair(toks[i-1].class, toks[i+1].class) {
			b.WriteString(thin)
			continue
		}
		b.WriteString(t.text)
		if !t.space && i+1 < len(toks) && pair(t.class, toks[i+1].class) {
			b.WriteString(thin)
		}
	}
	return b.String()
}

func pair(a, b class) bool { return (a == han && b == latin) || (a == latin && b == han) }

func main() {
	check := flag.Bool("check", false, "report whether the catalog needs spacing, and change nothing")
	path := flag.String("file", "locales/zh-Hans.json", "catalog to space")
	flag.Parse()

	src, err := os.ReadFile(*path)
	if err != nil {
		fail(err)
	}
	var out bytes.Buffer
	var changed []string
	for _, l := range strings.SplitAfter(string(src), "\n") {
		body := strings.TrimSuffix(l, "\n")
		m := line.FindStringSubmatch(body)
		if m == nil {
			out.WriteString(l)
			continue
		}
		var msg string
		if err := json.Unmarshal([]byte(m[3]), &msg); err != nil {
			fail(fmt.Errorf("%s: %w", m[2], err))
		}
		spaced := Space(msg)
		if spaced != msg {
			changed = append(changed, m[2])
		}
		var q bytes.Buffer
		enc := json.NewEncoder(&q)
		enc.SetEscapeHTML(false)
		if err := enc.Encode(spaced); err != nil {
			fail(err)
		}
		out.WriteString(m[1] + strings.TrimSuffix(q.String(), "\n") + m[4] + strings.TrimPrefix(l, body))
	}
	if *check {
		if len(changed) > 0 {
			fail(fmt.Errorf("%s needs spacing (go run ./scripts/autospace): %s", *path, strings.Join(changed, ", ")))
		}
		return
	}
	if err := os.WriteFile(*path, out.Bytes(), 0o644); err != nil {
		fail(err)
	}
	fmt.Printf("spaced %d messages\n", len(changed))
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "autospace:", err)
	os.Exit(1)
}
