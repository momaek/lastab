// Package todo parses and writes to-do lists stored as plain text, one item
// per line: "- [ ] item" and "- [x] item". Copy all and the CLI see exactly
// this text. Lines in other forms are kept as unchecked items, never lost.
package todo

import (
	"strings"
)

// Item is one to-do.
type Item struct {
	Text string `json:"text"`
	Done bool   `json:"done"`
}

// Progress counts done and total items.
type Progress struct {
	Done  int `json:"done"`
	Total int `json:"total"`
}

// Parse reads a list. Blank lines are skipped.
func Parse(body string) []Item {
	body = strings.ReplaceAll(body, "\r\n", "\n")
	body = strings.ReplaceAll(body, "\r", "\n")
	var items []Item
	for _, line := range strings.Split(body, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		items = append(items, ParseLine(line))
	}
	return items
}

// ParseLine reads one line.
func ParseLine(line string) Item {
	s := strings.TrimSpace(line)
	for _, bullet := range []string{"- ", "* ", "+ "} {
		if rest, ok := strings.CutPrefix(s, bullet); ok {
			s = rest
			break
		}
	}
	if len(s) >= 3 && s[0] == '[' && s[2] == ']' {
		switch s[1] {
		case ' ':
			return Item{Text: strings.TrimPrefix(s[3:], " ")}
		case 'x', 'X':
			return Item{Text: strings.TrimPrefix(s[3:], " "), Done: true}
		}
	}
	return Item{Text: s}
}

// Format writes a list, one "- [ ] text" line per item, LF line endings.
func Format(items []Item) string {
	var b strings.Builder
	for i, it := range items {
		if i > 0 {
			b.WriteByte('\n')
		}
		if it.Done {
			b.WriteString("- [x] ")
		} else {
			b.WriteString("- [ ] ")
		}
		b.WriteString(strings.ReplaceAll(it.Text, "\n", " "))
	}
	return b.String()
}

// Count returns the list's progress.
func Count(items []Item) Progress {
	p := Progress{Total: len(items)}
	for _, it := range items {
		if it.Done {
			p.Done++
		}
	}
	return p
}

// SplitPaste turns pasted text into items, one per non-blank line; a
// pasted "- [x] item" keeps its state.
func SplitPaste(text string) []Item {
	return Parse(text)
}
