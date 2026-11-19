// Package title makes a snippet's automatic title from its content, on the
// device, without a network. Titles are list metadata, never written into
// the body, and pass through redact so a secret never shows as a title.
package title

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/momaek/lastab/internal/redact"
	"github.com/momaek/lastab/internal/todo"
)

// MaxRunes is the longest title, in characters.
const MaxRunes = 40

// Kind mirrors store.Kind without importing it.
type Kind string

const (
	Text Kind = "text"
	Todo Kind = "todo"
)

// Make returns the title of a body, or "" when it has nothing to say.
func Make(kind Kind, body string) string {
	if strings.TrimSpace(body) == "" {
		return ""
	}
	var t string
	if kind == Todo {
		t = todoTitle(body)
	} else if c := curlTitle(body); c != "" {
		t = c
	} else if j := jsonTitle(body); j != "" {
		t = j
	} else {
		t = firstLine(body)
	}
	return truncate(redact.String(t), MaxRunes)
}

var curlMethod = regexp.MustCompile(`(?:^|\s)(?:-X|--request)\s*['"]?([A-Za-z]+)`)
var curlURL = regexp.MustCompile(`https?://[^\s'"\\]+`)

func curlTitle(body string) string {
	s := strings.TrimSpace(body)
	if !strings.HasPrefix(s, "curl ") && !strings.HasPrefix(s, "curl\t") {
		return ""
	}
	method := "GET"
	if m := curlMethod.FindStringSubmatch(s); m != nil {
		method = strings.ToUpper(m[1])
	} else if regexp.MustCompile(`(?:^|\s)(?:-d|--data[\w-]*|-F|--form)\b`).MatchString(s) {
		method = "POST"
	}
	path := ""
	if u := curlURL.FindString(s); u != "" {
		if pu, err := url.Parse(u); err == nil {
			path = pu.EscapedPath()
			if path == "" {
				path = "/"
			}
		}
	}
	if path == "" {
		return "curl · " + method
	}
	return "curl · " + method + " " + path
}

// jsonTitle names a JSON document by its first telling key and value.
func jsonTitle(body string) string {
	s := strings.TrimSpace(body)
	if s == "" || (s[0] != '{' && s[0] != '[') {
		return ""
	}
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		return ""
	}
	if k, val, ok := tellingPair(v, 0); ok {
		return fmt.Sprintf("JSON · %s %s", k, val)
	}
	return "JSON"
}

var idKey = regexp.MustCompile(`(?i)(^|_|-)(id|name|title|key|type|code|no|number)$`)

func tellingPair(v any, depth int) (string, string, bool) {
	if depth > 3 {
		return "", "", false
	}
	switch x := v.(type) {
	case []any:
		for _, e := range x {
			if k, val, ok := tellingPair(e, depth+1); ok {
				return k, val, ok
			}
		}
	case map[string]any:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys) // JSON objects decode unordered; keep titles stable.
		// An identifying key with a scalar value first, then any scalar.
		for _, pass := range []bool{true, false} {
			for _, k := range keys {
				if pass && !idKey.MatchString(k) {
					continue
				}
				if val, ok := scalar(x[k]); ok {
					return k, val, true
				}
			}
		}
		for _, k := range keys {
			if kk, val, ok := tellingPair(x[k], depth+1); ok {
				return kk, val, ok
			}
		}
	}
	return "", "", false
}

func scalar(v any) (string, bool) {
	switch x := v.(type) {
	case string:
		if strings.TrimSpace(x) == "" {
			return "", false
		}
		return x, true
	case float64:
		return fmt.Sprint(x), true
	case bool:
		return fmt.Sprint(x), true
	}
	return "", false
}

func todoTitle(body string) string {
	items := todo.Parse(body)
	p := todo.Count(items)
	first := ""
	for _, it := range items {
		if strings.TrimSpace(it.Text) != "" {
			first = strings.TrimSpace(it.Text)
			break
		}
	}
	if first == "" {
		return ""
	}
	return fmt.Sprintf("%s · %d/%d", truncate(first, MaxRunes-8), p.Done, p.Total)
}

// firstLine returns the first line with a letter or digit in it.
func firstLine(body string) string {
	for _, line := range strings.FieldsFunc(body, func(r rune) bool { return r == '\n' || r == '\r' }) {
		line = strings.TrimSpace(line)
		if strings.IndexFunc(line, func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }) >= 0 {
			return strings.Join(strings.Fields(line), " ")
		}
	}
	return ""
}

func truncate(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return strings.TrimRightFunc(string(r[:n-1]), unicode.IsSpace) + "…"
}

// Clean makes a title someone else wrote, an agent through `lastab
// retitle`, follow the rules of an automatic one: one line, no control
// characters, secrets masked, at most MaxRunes. It returns "" when nothing
// is left.
func Clean(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || unicode.Is(unicode.Zl, r) || unicode.Is(unicode.Zp, r) {
			return ' '
		}
		return r
	}, s)
	s = strings.Join(strings.Fields(s), " ")
	if s == "" {
		return ""
	}
	return truncate(redact.String(s), MaxRunes)
}
