package appui

import (
	"image/png"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/momaek/lastab/internal/ui/keys"

	"github.com/momaek/lastab/internal/install"
)

// TestSnapshots writes PNGs of the main states for comparing with the
// design when LASTAB_SNAPSHOT_DIR is set.
func TestSnapshots(t *testing.T) {
	dir := os.Getenv("LASTAB_SNAPSHOT_DIR")
	if dir == "" {
		t.Skip("LASTAB_SNAPSHOT_DIR not set")
	}
	save := func(name string, tt *ui.Tester) {
		f, err := os.Create(filepath.Join(dir, name+".png"))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		png.Encode(f, tt.Image())
	}
	for _, dark := range []bool{false, true} {
		h := newHarness(t, "")
		h.tt.SetDark(dark)
		h.tt.Type("curl -X POST 'https://api.staging.example.com/v2/orders/search' \\\n  -H 'Authorization: Bearer abcdefgh12345678' \\\n  -d '{\"order_id\":\"A-20391\"}'")
		h.settle()
		h.tt.Key(ui.Cmd, ui.KeyN)
		h.tt.Type("周会纪要 第 3 周\n- 发版窗口改到周四\n- 回归测试由 QA 负责")
		h.settle()
		h.tt.Key(ui.Cmd, ui.KeyN)
		name := "light"
		if dark {
			name = "dark"
		}
		save(name+"-blank", h.tt)
		h.tt.Key(ui.Cmd, ui.KeyW)
		h.settle()
		save(name+"-editor", h.tt)
		h.tt.Key(ui.Cmd, ui.KeyN)
		h.tt.Type("- [ ] 发版前检查\n- [x] 跑完整测试")
		h.settle()
		h.tt.Key(ui.Cmd, ui.KeyN)
		h.tt.Type(`{"order_id":"A-20391","status":"paid"}`)
		h.settle()
		h.tt.Click("周会纪要 第 3 周")
		save(name+"-sidebar", h.tt)
		h.tt.Key(ui.Cmd, ui.KeyBackslash)
		save(name+"-sidebar-hidden", h.tt)
		h.tt.Key(ui.Cmd, ui.KeyBackslash)
		h.tt.Key(ui.Cmd, ui.KeyBackquote)
		save(name+"-switcher", h.tt)
		h.tt.Key(0, ui.KeyEscape)

		h.tt.SetSize(1200, 760)
		h.tt.Key(ui.Cmd, ui.KeyK)
		h.tt.Type("staging")
		h.tt.Frame()
		save(name+"-library", h.tt)
		h.tt.Click("Delete…")
		save(name+"-library-delete", h.tt)
		h.tt.Key(0, ui.KeyEscape)
		h.tt.Click("Search all snippets")
		h.tt.Type("zz")
		h.tt.Frame()
		save(name+"-library-empty", h.tt)
		h.tt.Key(0, ui.KeyEscape)
		h.tt.Click("curl · POST /v2/orders/search")
		h.press(keys.History) // no-op guard: opens below
		h.press(keys.History)
		h.press(keys.History)
		h.app.CloseHistory()
		h.tt.Frame()
		h.app.Active().buf().Set("curl -X POST 'https://api.staging.example.com/v2/orders/search' \\\n  -H 'Authorization: Bearer abcdefgh12345678' \\\n  -d '{\"order_id\":\"A-20391\",\"page_size\":50}'")
		h.settle()
		h.press(keys.History)
		save(name+"-history", h.tt)
		h.tt.Click("Restore this version")
		save(name+"-history-restored", h.tt)
		h.tt.Key(0, ui.KeyEscape)
		h.app.CloseHistory()
		h.tt.SetSize(1100, 720)
		h.app.OpenSettings(secGeneral)
		h.tt.Frame()
		save(name+"-settings", h.tt)
		h.app.OpenSettings(secShortcuts)
		h.tt.Frame()
		h.tt.Click("Change")
		h.tt.Key(ui.Cmd, ui.KeyT)
		h.tt.Click("Change")
		h.tt.Frame()
		save(name+"-settings-keys", h.tt)
		h.tt.Key(0, ui.KeyEscape)
		h.app.OpenSettings(secAgent)
		h.tt.Frame()
		save(name+"-settings-agent", h.tt)
		h.app.CloseSettings()

		// The Install group: AgentInstalling and AgentFailed of the design.
		h.tt.SetSize(960, 880)
		home, _ := os.UserHomeDir()
		h.plat.cli = install.Status{State: install.Installed, Path: "/usr/local/bin/lastab"}
		h.plat.skill = install.Status{Path: filepath.Join(home, ".claude", "skills", "lastab", "SKILL.md")}
		var held []func() func()
		async := h.app.d.Async
		h.app.d.Async = func(w func() func()) { held = append(held, w) }
		h.app.OpenSettings(secAgent)
		h.tt.Frame() // asks for the statuses
		for _, w := range held {
			if apply := w(); apply != nil {
				apply()
			}
		}
		held = nil
		h.tt.Frame()
		save(name+"-agent", h.tt)
		h.tt.Click("Install skill")
		h.tt.Frame()
		save(name+"-agent-installing", h.tt)
		h.app.d.Async = async
		h.plat.skillErr = &install.DirError{Dir: filepath.Join(home, ".claude", "skills"), Err: fs.ErrPermission}
		h.app.prefs.skill.busy = false
		h.app.InstallSkill()
		h.tt.Frame()
		save(name+"-agent-failed", h.tt)
		h.app.CloseSettings()
		h.tt.SetSize(900, 600)
	}
}
