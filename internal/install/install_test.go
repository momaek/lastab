package install

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/momaek/lastab/skill"
)

func TestSkill(t *testing.T) {
	e := Env{Home: t.TempDir()}
	if st := e.SkillStatus(); st.State != Missing || !strings.HasSuffix(st.Path, filepath.Join(".claude", "skills", "lastab", "SKILL.md")) {
		t.Fatalf("fresh %+v", st)
	}
	st, err := e.InstallSkill()
	if err != nil || st.State != Installed {
		t.Fatalf("install %+v %v", st, err)
	}
	if b, _ := os.ReadFile(st.Path); !bytes.Equal(b, skill.Template) {
		t.Error("installed skill differs from the template")
	}

	// An older skill, or one without a version, is offered for reinstalling.
	for _, old := range []string{"<!-- lastab-skill-version: 0 -->\nold", "# my own notes"} {
		os.WriteFile(st.Path, []byte(old), 0o644)
		if got := e.SkillStatus().State; got != Outdated {
			t.Errorf("%q: state %v", old, got)
		}
	}
	if st, err := e.InstallSkill(); err != nil || st.State != Installed {
		t.Errorf("reinstall %+v %v", st, err)
	}
	if skill.Version(skill.Template) < 1 {
		t.Error("the template has no version")
	}
}

func TestSkillNoPermission(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("read-only folders need Unix permissions, and a user other than root")
	}
	home := t.TempDir()
	os.MkdirAll(filepath.Join(home, ".claude", "skills"), 0o755)
	os.Chmod(filepath.Join(home, ".claude", "skills"), 0o555)
	t.Cleanup(func() { os.Chmod(filepath.Join(home, ".claude", "skills"), 0o755) })
	_, err := Env{Home: home}.InstallSkill()
	var de *DirError
	if !errors.Is(err, fs.ErrPermission) || !errors.As(err, &de) || !strings.HasSuffix(de.Dir, "lastab") {
		t.Errorf("err %v", err)
	}
}

func TestPathList(t *testing.T) {
	dir := `C:\Users\a\AppData\Local\Programs\Lastab\bin`
	expand := func(s string) string { return strings.ReplaceAll(s, "%LOCALAPPDATA%", `C:\Users\a\AppData\Local`) }
	for list, want := range map[string]bool{
		"":                    false,
		`C:\Windows;C:\Tools`: false,
		`C:\Windows;c:\users\a\appdata\local\programs\lastab\bin\`: true,
		`"C:\Users\a\AppData\Local\Programs\Lastab\bin"`:           true,
		`%LOCALAPPDATA%\Programs\Lastab\bin;C:\Tools`:              true,
	} {
		if got := pathListHas(list, dir, expand); got != want {
			t.Errorf("%q: %v", list, got)
		}
	}
	if got, ok := pathListAdd(`C:\Tools;`, dir, expand); !ok || got != `C:\Tools;`+dir {
		t.Errorf("add %q", got)
	}
	if got, ok := pathListAdd("", dir, expand); !ok || got != dir {
		t.Errorf("add to empty %q", got)
	}
	if _, ok := pathListAdd(dir, dir, expand); ok {
		t.Error("added twice")
	}
}
