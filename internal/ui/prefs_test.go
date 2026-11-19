package appui

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/momaek/lastab/internal/agentlog"
	"github.com/momaek/lastab/internal/store"
	"github.com/momaek/lastab/internal/ui/keys"
)

type fakePlatform struct {
	appearance, language string
	login                bool
	loginErr             error
	applied              []map[keys.Action]string
	applyErr             error
	suspends             []bool
	shown                string
}

func (f *fakePlatform) SetAppearance(m string)  { f.appearance = m }
func (f *fakePlatform) SetLanguage(t string)    { f.language = t }
func (f *fakePlatform) OpenAtLogin() bool       { return f.login }
func (f *fakePlatform) SuspendShortcuts(s bool) { f.suspends = append(f.suspends, s) }
func (f *fakePlatform) ShowInFolder(p string)   { f.shown = p }
func (f *fakePlatform) Version() string         { return "0.1.0 · MyGo test" }
func (f *fakePlatform) SetOpenAtLogin(on bool) error {
	if f.loginErr != nil {
		return f.loginErr
	}
	f.login = on
	return nil
}
func (f *fakePlatform) ApplyShortcuts(t map[keys.Action]string) error {
	if f.applyErr != nil {
		return f.applyErr
	}
	f.applied = append(f.applied, t)
	return nil
}

// clickRole clicks the control with a role and accessible name, where a
// text of the same name comes first, as a switch beside its label.
func (h *harness) clickRole(role ui.Role, name string) {
	h.t.Helper()
	info, ok := h.tt.AccessByRole(role, name)
	if !ok {
		h.t.Fatalf("no %v named %q", role, name)
	}
	b := info.Bounds
	h.tt.ClickAt(b.X+b.W/2, b.Y+b.H/2)
}

func setting[T any](t *testing.T, st *store.Store, key string) T {
	t.Helper()
	var v T
	if _, err := st.GetSetting(context.Background(), key, &v); err != nil {
		t.Fatal(err)
	}
	return v
}

func TestSettingsOpenAndGeneral(t *testing.T) {
	h := newHarness(t, "")
	h.tt.Key(ui.Cmd, ui.KeyComma)
	if !h.app.SettingsOpen() || !h.tt.HasText("Appearance") {
		t.Fatalf("Cmd+, : %q", h.tt.Texts())
	}
	if err := h.tt.Click("Dark"); err != nil {
		t.Fatal(err)
	}
	if h.plat.appearance != "dark" || setting[string](t, h.st, store.SettingAppearance) != "dark" {
		t.Errorf("appearance %q", h.plat.appearance)
	}
	h.tt.Click("English")
	if h.plat.language != "en" || setting[string](t, h.st, store.SettingLanguage) != "en" {
		t.Errorf("language %q", h.plat.language)
	}
	h.tt.Click("System") // the first "System": appearance
	if h.plat.appearance != "system" {
		t.Errorf("appearance %q", h.plat.appearance)
	}
	if err := h.tt.Click("#0F766E"); err != nil {
		t.Fatal(err)
	}
	if h.app.d.Accent != "#0F766E" || setting[string](t, h.st, store.SettingAccent) != "#0F766E" {
		t.Errorf("accent %q", h.app.d.Accent)
	}
	h.clickRole(ui.RoleSwitch, "Open at login")
	if !h.plat.login || !setting[bool](t, h.st, store.SettingOpenAtLogin) {
		t.Error("open at login")
	}
	h.plat.loginErr = errors.New("not allowed")
	h.clickRole(ui.RoleSwitch, "Open at login")
	if !h.tt.HasText("not allowed") || !h.plat.login {
		t.Errorf("login error not shown: %q", h.tt.Texts())
	}
	h.tt.Key(0, ui.KeyEscape)
	if h.app.SettingsOpen() {
		t.Error("Escape did not close settings")
	}

	// A new window over the same database starts with the choices.
	h2 := newHarness(t, filepath.Join(h.dir, "lastab.db"))
	if s := h2.app.Settings(); s.Appearance != "system" || s.Language != "en" || s.Accent != "#0F766E" || h2.app.d.Accent != "#0F766E" {
		t.Errorf("reloaded %+v", s)
	}
}

func TestRecordShortcut(t *testing.T) {
	h := newHarness(t, "")
	h.app.OpenSettings(secShortcuts)
	h.tt.Frame()
	if err := h.tt.Click("Change"); err != nil { // the first row: New snippet
		t.Fatal(err)
	}
	if h.app.Recording() != keys.NewText || !h.tt.HasText("Press a new shortcut…") {
		t.Fatalf("not recording: %q", h.tt.Texts())
	}
	// Window shortcuts wait while recording: Cmd+N makes no tab.
	tabs := len(h.app.Tabs())
	h.tt.Key(ui.Cmd|ui.Alt, ui.KeyN)
	if h.app.Recording() != "" || len(h.app.Tabs()) != tabs {
		t.Fatalf("recording %q tabs %d", h.app.Recording(), len(h.app.Tabs()))
	}
	if got := h.app.Shortcuts()[keys.NewText]; got != "CmdOrCtrl+Alt+N" {
		t.Fatalf("shortcut %q", got)
	}
	if len(h.plat.suspends) != 2 || !h.plat.suspends[0] || h.plat.suspends[1] {
		t.Errorf("suspends %v", h.plat.suspends)
	}
	if n := len(h.plat.applied); n == 0 || h.plat.applied[n-1][keys.NewText] != "CmdOrCtrl+Alt+N" {
		t.Errorf("not applied to menus: %v", h.plat.applied)
	}
	saved := setting[map[keys.Action]string](t, h.st, store.SettingShortcuts)
	if saved[keys.NewText] != "CmdOrCtrl+Alt+N" {
		t.Errorf("saved %v", saved)
	}

	// The new keys work at once; the old ones no longer do.
	h.tt.Key(0, ui.KeyEscape)
	h.tt.Key(ui.Cmd, ui.KeyN)
	if len(h.app.Tabs()) != tabs {
		t.Error("old shortcut still makes a tab")
	}
	h.tt.Key(ui.Cmd|ui.Alt, ui.KeyN)
	if len(h.app.Tabs()) != tabs+1 {
		t.Error("new shortcut makes no tab")
	}
	// The editor gave the new keys up, not the old.
	var found bool
	for _, b := range h.app.Active().ed.Bindings {
		if b.Mods == ui.Cmd|ui.Alt && b.Key == ui.KeyN {
			found = true
		}
	}
	if !found {
		t.Error("editor still binds the new keys")
	}
}

func TestShortcutErrors(t *testing.T) {
	h := newHarness(t, "")
	h.app.OpenSettings(secShortcuts)
	h.tt.Frame()
	h.tt.Click("Change")
	h.tt.Key(ui.Cmd, ui.KeyT) // New to-do list's
	if !h.tt.HasText("⌘T is already used by “New To-do List”") && !h.tt.HasText("Ctrl+T is already used by “New To-do List”") {
		t.Errorf("conflict not shown: %q", h.tt.Texts())
	}
	h.tt.Click("Change")
	h.tt.Key(0, ui.KeyA)
	if !strings.Contains(strings.Join(h.tt.Texts(), "|"), "A shortcut needs one of") {
		t.Errorf("modifier error not shown: %q", h.tt.Texts())
	}
	h.tt.Click("Change")
	h.tt.Key(0, ui.KeyEscape)
	if h.app.Recording() != "" || !h.app.SettingsOpen() {
		t.Error("Escape should cancel recording and keep settings open")
	}
	// The system refuses a global shortcut: the old one stays.
	h.plat.applyErr = errors.New("taken")
	if err := h.app.SetShortcut(keys.Summon, "Ctrl+Alt+Space"); err == nil {
		t.Fatal("no error")
	}
	if h.app.Shortcuts()[keys.Summon] != "Ctrl+Alt+N" {
		t.Errorf("summon %q", h.app.Shortcuts()[keys.Summon])
	}
	h.plat.applyErr = nil
	h.app.SetShortcut(keys.CopyAll, "CmdOrCtrl+Alt+C")
	h.tt.SetSize(900, 1400) // the button is below the rows
	if err := h.tt.Click("Reset all to defaults"); err != nil {
		t.Fatal(err)
	}
	if h.app.Shortcuts()[keys.CopyAll] != "CmdOrCtrl+Shift+C" {
		t.Errorf("reset: %q", h.app.Shortcuts()[keys.CopyAll])
	}
}

func TestEditorFontAndAgent(t *testing.T) {
	h := newHarness(t, "")
	h.app.OpenSettings(secEditing)
	h.tt.Frame()
	before := h.app.Active().ed.Font.Size
	if err := h.tt.Click("+"); err != nil {
		t.Fatal(err)
	}
	if got := h.app.Active().ed.Font.Size; got != before+0.5 || setting[float32](t, h.st, store.SettingEditorSize) != got {
		t.Errorf("size %v -> %v", before, got)
	}

	h.tt.Click("Agent access")
	if setting[bool](t, h.st, store.SettingAgentAccess) {
		t.Fatal("agent access on by default")
	}
	h.clickRole(ui.RoleSwitch, "Allow agent access")
	if !setting[bool](t, h.st, store.SettingAgentAccess) {
		t.Error("access not saved")
	}
	h.clickRole(ui.RoleSwitch, "Mask likely secrets (recommended)")
	if !setting[bool](t, h.st, store.SettingAgentRaw) {
		t.Error("raw not saved")
	}
	if !h.tt.HasText("No agent has read anything yet.") {
		t.Errorf("texts %q", h.tt.Texts())
	}
}

func TestAccessLogAndStorage(t *testing.T) {
	h := newHarness(t, "")
	h.tt.Type("curl https://x.dev/orders")
	h.settle()
	l, err := agentlog.Open(filepath.Join(h.dir, "agent_log.db"))
	if err != nil {
		t.Fatal(err)
	}
	l.Add(context.Background(), agentlog.Entry{At: time.Now(), Client: "claude-code", Op: "read",
		Args: map[string]any{"id": "01J"}, SnippetIDs: []string{h.app.Active().ID()}})
	l.Add(context.Background(), agentlog.Entry{At: time.Now(), Client: "claude-code", Op: "search",
		Args: map[string]any{"query": "curl", "since": "1d", "limit": 20}, SnippetIDs: []string{"a", "b"}})
	l.Close()

	h.app.OpenSettings(secAgent)
	h.tt.Frame()
	all := strings.Join(h.tt.Texts(), "|")
	for _, want := range []string{`search "curl" --since 1d`, "2 results", "read 01J", "curl · GET /orders"} {
		if !strings.Contains(all, want) {
			t.Errorf("access log lacks %q: %q", want, all)
		}
	}
	h.tt.Click("Storage")
	if !h.tt.HasText("1 snippet · 1 version · " + humanBytes(h.app.prefs.storage.bytes)) {
		t.Errorf("storage: %q", h.tt.Texts())
	}
	h.tt.Click("Show in folder")
	if h.plat.shown != filepath.Join(h.dir, "lastab.db") {
		t.Errorf("shown %q", h.plat.shown)
	}
}
