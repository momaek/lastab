// Package skill holds the Claude Code skill that Settings > Agent access
// installs as ~/.claude/skills/lastab/SKILL.md.
package skill

import (
	_ "embed"
	"regexp"
	"strconv"
)

// Template is SKILL.md as Lastab installs it.
//
//go:embed SKILL.md
var Template []byte

var versionRe = regexp.MustCompile(`<!--\s*lastab-skill-version:\s*(\d+)`)

// Version returns the template version a SKILL.md declares, or 0 for none.
// Raise it in SKILL.md whenever the skill changes, so that Settings offers
// to reinstall it over an older one.
func Version(b []byte) int {
	m := versionRe.FindSubmatch(b)
	if m == nil {
		return 0
	}
	n, _ := strconv.Atoi(string(m[1]))
	return n
}
