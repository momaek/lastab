// Package install puts the lastab command on the user's PATH and the Claude
// Code skill in ~/.claude/skills, when the user asks for it in Settings >
// Agent access. Nothing here runs on its own: the app calls it on a click.
//
// The command is never copied. On macOS a symbolic link in /usr/local/bin,
// or else ~/.local/bin, points into the app; on Windows the app's bin folder
// goes on the user's PATH. Either way an updated app updates the command.
package install

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/momaek/lastab/skill"
)

// State is how far something is installed.
type State int

const (
	Missing   State = iota
	Installed       // in place and current
	Outdated        // in place, but an older skill, or a link to another copy of the app
)

// Status is where the command or the skill is, or would go.
type Status struct {
	State State
	// Path is the installed file or link, or where Install would put it.
	Path string
	// NeedsPath says the command went to ~/.local/bin, which the user's
	// shell may not have on its PATH.
	NeedsPath bool
	// Elevate says /usr/local/bin needs an administrator, which InstallCLI
	// asks for when told to (macOS).
	Elevate bool
}

// Env is the places the package works in; tests point it at a temp dir.
type Env struct {
	Home string // the user's home folder
	// CLI is the lastab executable inside the app.
	CLI string
	// GlobalBin is where the command goes first on macOS, /usr/local/bin.
	GlobalBin string
}

// Errors InstallCLI and InstallSkill report.
var (
	// ErrNoCLI says the app has no lastab executable inside: a build that
	// skipped scripts/build.
	ErrNoCLI = errors.New("the app holds no lastab command")
	// ErrCanceled says the user dismissed the administrator prompt.
	ErrCanceled = errors.New("canceled")
	// ErrNotInstalled says the app runs from its disk image, or from where
	// macOS moved it to run it once (App Translocation): a link there
	// would break, so the app must be moved to Applications first.
	ErrNotInstalled = errors.New("Lastab runs from its disk image or a temporary copy; move it to Applications first")
)

// ExistsError says a file not made by Lastab is where the link goes, which
// is left alone.
type ExistsError struct{ Path string }

func (e *ExistsError) Error() string { return e.Path + " exists and is not Lastab's" }

// DirError is a failure to write in a folder; Unwrap gives the cause, as
// fs.ErrPermission.
type DirError struct {
	Dir string
	Err error
}

func (e *DirError) Error() string { return fmt.Sprintf("%s: %v", e.Dir, e.Err) }
func (e *DirError) Unwrap() error { return e.Err }

// SkillFile is where the skill goes: ~/.claude/skills/lastab/SKILL.md.
func (e Env) SkillFile() string {
	return filepath.Join(e.Home, ".claude", "skills", "lastab", "SKILL.md")
}

// SkillStatus reports whether the skill is installed, and whether it is
// older than the one this app ships. A SKILL.md without a version, as one
// the user wrote, counts as older: it is offered for reinstalling, never
// overwritten unasked.
func (e Env) SkillStatus() Status {
	st := Status{Path: e.SkillFile()}
	b, err := os.ReadFile(st.Path)
	switch {
	case err != nil:
		st.State = Missing
	case bytes.Equal(b, skill.Template) || skill.Version(b) >= skill.Version(skill.Template):
		st.State = Installed
	default:
		st.State = Outdated
	}
	return st
}

// InstallSkill writes the skill, replacing what is there.
func (e Env) InstallSkill() (Status, error) {
	path := e.SkillFile()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return e.SkillStatus(), &DirError{dir, cause(err)}
	}
	if err := writeFile(path, skill.Template); err != nil {
		return e.SkillStatus(), &DirError{dir, cause(err)}
	}
	return e.SkillStatus(), nil
}

// writeFile replaces path whole, through a temporary file beside it.
func writeFile(path string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".SKILL-*.md")
	if err != nil {
		return err
	}
	tmp := f.Name()
	_, err = f.Write(data)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Chmod(tmp, 0o644)
	}
	if err == nil {
		err = os.Rename(tmp, path)
	}
	if err != nil {
		os.Remove(tmp)
	}
	return err
}

// cause strips the *fs.PathError of an error, whose path DirError already
// names.
func cause(err error) error {
	var pe *fs.PathError
	if errors.As(err, &pe) {
		return pe.Err
	}
	var le *os.LinkError
	if errors.As(err, &le) {
		return le.Err
	}
	return err
}

func (e Env) cliExists() error {
	if e.CLI == "" {
		return ErrNoCLI
	}
	if fi, err := os.Stat(e.CLI); err != nil || fi.IsDir() {
		return ErrNoCLI
	}
	if strings.Contains(e.CLI, "/AppTranslocation/") || strings.HasPrefix(e.CLI, "/Volumes/") {
		return ErrNotInstalled
	}
	return nil
}
