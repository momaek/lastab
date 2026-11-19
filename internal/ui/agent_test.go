package appui

import (
	"io/fs"
	"runtime"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/momaek/lastab/internal/install"
)

func agentHarness(t *testing.T) *harness {
	t.Helper()
	h := newHarness(t, "")
	h.plat.cli = install.Status{Path: "/usr/local/bin/lastab"}
	h.plat.skill = install.Status{Path: "/Users/a/.claude/skills/lastab/SKILL.md"}
	h.tt.SetSize(900, 1000)
	return h
}

func TestInstallSkillOnClick(t *testing.T) {
	h := agentHarness(t)
	// Async that holds the work until the test lets it run.
	var pending []func() func()
	h.app.d.Async = func(work func() func()) { pending = append(pending, work) }
	run := func() {
		for len(pending) > 0 {
			w := pending[0]
			pending = pending[1:]
			if apply := w(); apply != nil {
				apply()
			}
		}
		h.tt.Frame()
	}

	h.app.OpenSettings(secAgent)
	h.tt.Frame()
	run()
	if h.plat.skillCalls != 0 || len(h.plat.cliCalls) != 0 {
		t.Fatal("installed without a click")
	}
	if !h.tt.HasText("/usr/local/bin/lastab · v0.1.0") {
		t.Errorf("cli path: %q", h.tt.Texts())
	}
	if err := h.tt.Click("Install skill"); err != nil {
		t.Fatal(err)
	}
	h.tt.Frame()
	if !h.tt.HasText("Installing…") || h.tt.HasText("Install skill") {
		t.Errorf("not installing: %q", h.tt.Texts())
	}
	run()
	if h.plat.skillCalls != 1 || !h.tt.HasText("Installed") || h.tt.HasText("Installing…") {
		t.Errorf("after install (%d calls): %q", h.plat.skillCalls, h.tt.Texts())
	}
}

func TestInstallFailureAndRetry(t *testing.T) {
	h := agentHarness(t)
	h.plat.skillErr = &install.DirError{Dir: "/Users/a/.claude/skills/lastab", Err: fs.ErrPermission}
	h.app.OpenSettings(secAgent)
	h.tt.Frame()
	if err := h.tt.Click("Install skill"); err != nil {
		t.Fatal(err)
	}
	h.tt.Frame()
	if !h.tt.HasText("no permission to write to /Users/a/.claude/skills/lastab") || h.tt.HasText("Install skill") {
		t.Fatalf("failure: %q", h.tt.Texts())
	}
	h.plat.skillErr = nil
	if err := h.tt.Click("Retry"); err != nil {
		t.Fatal(err)
	}
	h.tt.Frame()
	if h.plat.skillCalls != 2 || h.tt.HasText("no permission") || !h.tt.HasText("Installed") {
		t.Errorf("retry (%d calls): %q", h.plat.skillCalls, h.tt.Texts())
	}

	// Errors of the command-line tool.
	for err, want := range map[error]string{
		install.ErrNoCLI:        "no lastab command inside",
		install.ErrNotInstalled: "Move it to Applications",
		&install.ExistsError{Path: "/usr/local/bin/lastab"}: "/usr/local/bin/lastab already exists",
	} {
		if got := installErrorText(err); !strings.Contains(got, want) {
			t.Errorf("%v: %q", err, got)
		}
	}
}

func TestReinstallOutdatedSkill(t *testing.T) {
	h := agentHarness(t)
	h.plat.skill.State = install.Outdated
	h.app.OpenSettings(secAgent)
	h.tt.Frame()
	if !h.tt.HasText("comes with a newer skill") {
		t.Fatalf("no reinstall offer: %q", h.tt.Texts())
	}
	if err := h.tt.Click("Reinstall"); err != nil {
		t.Fatal(err)
	}
	h.tt.Frame()
	if h.plat.skillCalls != 1 || h.tt.HasText("newer skill") {
		t.Errorf("reinstall: %q", h.tt.Texts())
	}
}

func TestInstallCLIFallback(t *testing.T) {
	h := agentHarness(t)
	h.plat.cli = install.Status{Path: "/Users/a/.local/bin/lastab", NeedsPath: true, Elevate: true}
	h.plat.cliErr = nil
	h.app.OpenSettings(secAgent)
	h.tt.Frame()
	h.clickRole(ui.RoleButton, "Install")
	h.tt.Frame()
	if len(h.plat.cliCalls) != 1 || h.plat.cliCalls[0] || !h.tt.HasText("which your shell may not search") {
		t.Fatalf("fallback %v: %q", h.plat.cliCalls, h.tt.Texts())
	}
	if runtime.GOOS != "darwin" {
		return
	}
	h.plat.cliErr = install.ErrCanceled
	if err := h.tt.Click("Install in /usr/local/bin…"); err != nil {
		t.Fatal(err)
	}
	h.tt.Frame()
	if len(h.plat.cliCalls) != 2 || !h.plat.cliCalls[1] || h.tt.HasText("Couldn’t install") {
		t.Errorf("admin %v: %q", h.plat.cliCalls, h.tt.Texts())
	}
}
