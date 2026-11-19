package appui

import (
	"context"
	"fmt"
	"runtime"
	"slices"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"

	"github.com/momaek/lastab/internal/install"
	"github.com/momaek/lastab/internal/store"
	"github.com/momaek/lastab/internal/theme"
	"github.com/momaek/lastab/internal/ui/keys"
)

// Platform is what settings change outside the window: the app's
// appearance and language, menus and the global shortcut, login items.
// The shell implements it with MyGo; tests use a fake.
type Platform interface {
	SetAppearance(mode string) // "system", "light" or "dark"
	SetLanguage(tag string)    // "" follows the system
	OpenAtLogin() bool
	SetOpenAtLogin(on bool) error
	// ApplyShortcuts puts a table in effect in the menus and registers the
	// global shortcut, reporting an error when the system refuses it.
	ApplyShortcuts(table map[keys.Action]string) error
	// SuspendShortcuts takes the menus' and the global shortcut's keys
	// away while a new shortcut is recorded, and gives them back.
	SuspendShortcuts(suspend bool)
	ShowInFolder(path string)
	Version() string

	// The lastab command and the Claude Code skill, which Settings >
	// Agent access installs only when the user clicks. The statuses read
	// the disk and the installs write it, and InstallCLI(true) shows the
	// system's administrator dialog, so the UI calls them off the main
	// thread.
	CLIStatus() install.Status
	SkillStatus() install.Status
	// InstallCLI puts lastab on the PATH; admin asks for an administrator
	// to write /usr/local/bin (macOS).
	InstallCLI(admin bool) (install.Status, error)
	InstallSkill() (install.Status, error)
	// SetTray shows or hides the icon in the menu bar (macOS) or the
	// notification area (Windows).
	SetTray(on bool)
}

// Settings are the user's choices, kept in the setting table.
type Settings struct {
	Appearance  string // "system", "light", "dark"
	Language    string // "system", "zh-Hans", "en"
	Accent      string
	Shortcuts   map[keys.Action]string // the user's own, over the defaults
	EditorFont  string                 // a family; "" is the system's monospaced font
	EditorSize  float32
	OpenAtLogin bool
	AgentAccess bool
	AgentMask   bool // mask secrets in what agents read
	HoldHint    bool // holding ⌘ alone shows the buttons' shortcuts
	Tray        bool // an icon in the menu bar or notification area
}

// Editor font sizes Settings offers.
const (
	MinEditorSize = 11
	MaxEditorSize = 20
)

// DefaultSettings are the choices before the user makes any.
func DefaultSettings() Settings {
	return Settings{
		Appearance: "system", Language: "system", Accent: theme.DefaultAccent,
		Shortcuts: map[keys.Action]string{}, EditorSize: theme.EditorFontSize, EditorFont: theme.MonoEditor,
		AgentMask: true, // agent access stays off until the user turns it on
		HoldHint:  true,
		Tray:      true,
	}
}

// LoadSettings reads the settings, defaults for what was never set.
func LoadSettings(ctx context.Context, st *store.Store) (Settings, error) {
	s := DefaultSettings()
	var raw bool
	for _, f := range []struct {
		key string
		v   any
	}{
		{store.SettingAppearance, &s.Appearance},
		{store.SettingLanguage, &s.Language},
		{store.SettingAccent, &s.Accent},
		{store.SettingShortcuts, &s.Shortcuts},
		{store.SettingEditorFont, &s.EditorFont},
		{store.SettingEditorSize, &s.EditorSize},
		{store.SettingOpenAtLogin, &s.OpenAtLogin},
		{store.SettingAgentAccess, &s.AgentAccess},
		{store.SettingAgentRaw, &raw},
		{store.SettingHoldHint, &s.HoldHint},
		{store.SettingTray, &s.Tray},
	} {
		if _, err := st.GetSetting(ctx, f.key, f.v); err != nil {
			return DefaultSettings(), fmt.Errorf("setting %s: %w", f.key, err)
		}
	}
	s.AgentMask = !raw
	if s.Shortcuts == nil {
		s.Shortcuts = map[keys.Action]string{}
	}
	s.EditorSize = min(max(s.EditorSize, MinEditorSize), MaxEditorSize)
	return s, nil
}

// saveSetting writes one setting off the main thread.
func (a *App) saveSetting(key string, v any) {
	st := a.d.Service.Store
	a.d.Async(func() func() {
		if err := st.SetSetting(a.ctx(), key, v); err != nil {
			a.d.Log.Error("save setting", "key", key, "err", err)
		}
		return nil
	})
}

// Settings returns the settings in effect.
func (a *App) Settings() Settings { return a.settings }

// Shortcuts returns the accelerators in effect, for menus.
func (a *App) Shortcuts() map[keys.Action]string {
	return keys.Table(runtime.GOOS, a.settings.Shortcuts)
}

// SetAppearance follows the system or fixes light or dark, at once.
func (a *App) SetAppearance(mode string) {
	a.settings.Appearance = mode
	if a.d.Platform != nil {
		a.d.Platform.SetAppearance(mode)
	}
	a.saveSetting(store.SettingAppearance, mode)
}

// SetLanguage follows the system or fixes the interface language, at once.
func (a *App) SetLanguage(lang string) {
	a.settings.Language = lang
	if a.d.Platform != nil {
		tag := lang
		if lang == "system" {
			tag = ""
		}
		a.d.Platform.SetLanguage(tag)
	}
	a.saveSetting(store.SettingLanguage, lang)
}

// SetAccent changes the accent color, at once.
func (a *App) SetAccent(hex string) {
	a.settings.Accent = hex
	a.d.Accent = hex
	a.saveSetting(store.SettingAccent, hex)
}

// SetEditorFont changes the editors' font and size, at once.
func (a *App) SetEditorFont(family string, size float32) {
	a.settings.EditorFont = family
	a.settings.EditorSize = min(max(size, MinEditorSize), MaxEditorSize)
	for _, t := range a.tabs {
		a.styleEditor(t.ed)
	}
	a.saveSetting(store.SettingEditorFont, family)
	a.saveSetting(store.SettingEditorSize, a.settings.EditorSize)
}

func (a *App) styleEditor(st *editor.State) {
	family := a.settings.EditorFont
	if family == "" {
		family = "monospace" // the system's monospaced font
	}
	st.Font = ui.Font{Family: family, Size: a.settings.EditorSize, Features: theme.NoLigatures}
	st.LineHeight = theme.EditorLineHeight / theme.EditorFontSize
	st.Bindings = a.editorBindings()
}

// SetOpenAtLogin starts Lastab when the user logs in, or stops it.
func (a *App) SetOpenAtLogin(on bool) error {
	if a.d.Platform != nil {
		if err := a.d.Platform.SetOpenAtLogin(on); err != nil {
			return err
		}
	}
	a.settings.OpenAtLogin = on
	a.saveSetting(store.SettingOpenAtLogin, on)
	return nil
}

// SetHoldHint turns the shortcut hints of a held ⌘ on or off.
func (a *App) SetHoldHint(on bool) {
	a.settings.HoldHint = on
	a.saveSetting(store.SettingHoldHint, on)
}

// SetTray shows or hides the icon in the menu bar or notification area.
func (a *App) SetTray(on bool) {
	a.settings.Tray = on
	if a.d.Platform != nil {
		a.d.Platform.SetTray(on)
	}
	a.saveSetting(store.SettingTray, on)
}

// SetAgentAccess lets the lastab CLI read snippets, or refuses it.
func (a *App) SetAgentAccess(on bool) {
	a.settings.AgentAccess = on
	a.saveSetting(store.SettingAgentAccess, on)
}

// SetAgentMask masks secrets in what agents read, or not.
func (a *App) SetAgentMask(on bool) {
	a.settings.AgentMask = on
	a.saveSetting(store.SettingAgentRaw, !on)
}

// ShortcutError explains why a shortcut wasn't taken.
type ShortcutError struct {
	Conflict   keys.Action // the action that has it already
	NoModifier bool
	System     error // the system refused the global shortcut
}

func (e *ShortcutError) Error() string {
	switch {
	case e.Conflict != "":
		return "conflicts with " + string(e.Conflict)
	case e.NoModifier:
		return "needs a modifier"
	}
	return e.System.Error()
}

// SetShortcut gives an action a new accelerator, at once, unless it lacks
// a modifier, another action has it, or the system refuses it.
func (a *App) SetShortcut(act keys.Action, acc string) error {
	goos := runtime.GOOS
	if !keys.HasModifier(acc, goos) {
		return &ShortcutError{NoModifier: true}
	}
	if other, ok := keys.Conflict(a.Shortcuts(), act, acc, goos); ok {
		return &ShortcutError{Conflict: other}
	}
	prev, had := a.settings.Shortcuts[act]
	a.settings.Shortcuts[act] = acc
	if keys.Canonical(acc, goos) == keys.Canonical(keys.Defaults(goos)[act], goos) {
		delete(a.settings.Shortcuts, act)
	}
	if err := a.applyShortcuts(); err != nil {
		if had {
			a.settings.Shortcuts[act] = prev
		} else {
			delete(a.settings.Shortcuts, act)
		}
		a.applyShortcuts()
		return &ShortcutError{System: err}
	}
	a.saveSetting(store.SettingShortcuts, a.settings.Shortcuts)
	return nil
}

// ResetShortcuts puts every shortcut back to its default.
func (a *App) ResetShortcuts() error {
	a.settings.Shortcuts = map[keys.Action]string{}
	a.saveSetting(store.SettingShortcuts, a.settings.Shortcuts)
	return a.applyShortcuts()
}

func (a *App) applyShortcuts() error {
	for _, t := range a.tabs {
		t.ed.Bindings = a.editorBindings()
	}
	if a.d.Platform == nil {
		return nil
	}
	return a.d.Platform.ApplyShortcuts(a.Shortcuts())
}

// editorBindings takes the app's shortcuts away from the editor, which
// would otherwise run its own commands on some of them (Cmd+K starts its
// chords, as Cmd+K Cmd+0 folding).
func (a *App) editorBindings() []editor.Binding {
	var bs []editor.Binding
	table := a.Shortcuts()
	for _, act := range append(slices.Clone(keys.Customizable), keys.Settings) {
		if keys.IsGlobal(act) {
			continue
		}
		if m, k, ok := parseAccel(table[act]); ok {
			bs = append(bs, editor.Binding{Mods: m, Key: k, Command: editor.NoCommand})
		}
	}
	return bs
}
