package appui

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"

	"github.com/momaek/lastab/internal/agentlog"
	"github.com/momaek/lastab/internal/paths"
	"github.com/momaek/lastab/internal/store"
	"github.com/momaek/lastab/internal/theme"
	"github.com/momaek/lastab/internal/ui/keys"
	"github.com/momaek/lastab/locales"
)

// Settings sections, in the order of the page's navigation.
const (
	secGeneral = iota
	secShortcuts
	secEditing
	secAgent
	secStorage
	secAbout
)

// prefsPage is the settings page (Cmd+,): a list of sections on the left,
// the chosen one on the right. Every change applies at once.
type prefsPage struct {
	open      bool
	section   int
	recording keys.Action
	focusRec  bool
	errs      map[keys.Action]string
	loginErr  string

	log       []logRow
	logLoaded bool
	storage   storageInfo
	fonts     []string
}

type logRow struct {
	agentlog.Entry
	target string
}

type storageInfo struct {
	loaded   bool
	snippets int
	versions int
	bytes    int64
}

// OpenSettings shows the settings page, on section sec when sec >= 0.
func (a *App) OpenSettings(sec int) {
	a.FlushAll()
	a.switcher.open = false
	if a.lib.open {
		a.CloseLibrary()
	}
	if a.history.open {
		a.CloseHistory()
	}
	p := &a.prefs
	cur := p.section
	*p = prefsPage{open: true, section: cur, errs: map[keys.Action]string{}}
	if sec >= 0 {
		p.section = sec
	}
	if a.d.Platform != nil {
		a.settings.OpenAtLogin = a.d.Platform.OpenAtLogin()
	}
}

// CloseSettings goes back to the editor.
func (a *App) CloseSettings() {
	a.stopRecording()
	a.prefs.open = false
	a.focusEditor = true
}

// SettingsOpen reports whether the settings page shows.
func (a *App) SettingsOpen() bool { return a.prefs.open }

// Recording returns the action whose new shortcut is being recorded.
func (a *App) Recording() keys.Action { return a.prefs.recording }

func (a *App) startRecording(act keys.Action) {
	a.prefs.recording, a.prefs.focusRec = act, true
	delete(a.prefs.errs, act)
	if a.d.Platform != nil {
		a.d.Platform.SuspendShortcuts(true)
	}
}

func (a *App) stopRecording() {
	if a.prefs.recording == "" {
		return
	}
	a.prefs.recording = ""
	if a.d.Platform != nil {
		a.d.Platform.SuspendShortcuts(false)
	}
}

// record takes a key pressed while recording: Escape cancels, a modifier
// alone waits for the key, anything else becomes the shortcut if allowed.
func (a *App) record(ev ui.InputEvent) bool {
	act := a.prefs.recording
	if act == "" || ev.Kind != ui.InputKeyDown {
		return act != "" && (ev.Kind == ui.InputText || ev.Kind == ui.InputKeyUp)
	}
	if ev.Key == ui.KeyEscape && ev.Mods == 0 {
		a.stopRecording()
		return true
	}
	acc, ok := formatAccel(ev.Mods, ev.Key)
	if !ok {
		return true // a modifier alone, or a key shortcuts can't use
	}
	a.stopRecording()
	if err := a.SetShortcut(act, acc); err != nil {
		a.prefs.errs[act] = shortcutErrorText(err, acc)
	}
	return true
}

func shortcutErrorText(err error, acc string) string {
	var se *ShortcutError
	if !errors.As(err, &se) {
		return err.Error()
	}
	switch {
	case se.Conflict != "":
		return locales.ShortcutConflict(keys.Display(acc, runtime.GOOS), actionLabel(se.Conflict))
	case se.NoModifier:
		return locales.ShortcutNeedsModifier(modifierHint())
	}
	return locales.ShortcutTaken(keys.Display(acc, runtime.GOOS))
}

func modifierHint() string {
	if runtime.GOOS == "darwin" {
		return "⌘ ⌃ ⌥"
	}
	return "Ctrl / Alt"
}

func actionLabel(a keys.Action) string {
	switch a {
	case keys.NewText:
		return locales.NewSnippetMenu()
	case keys.NewTodo:
		return locales.NewTodoMenu()
	case keys.Summon:
		return locales.SummonAction()
	case keys.Recent:
		return locales.SwitcherTitle()
	case keys.AllSnippets:
		return locales.AllSnippets()
	case keys.CopyAll:
		return locales.CopyAll()
	case keys.History:
		return locales.HistoryTitle()
	case keys.CloseTab:
		return locales.CloseSnippetMenu()
	case keys.ToggleSidebar:
		return locales.ToggleSidebarMenu()
	case keys.Settings:
		return locales.SettingsTitle()
	}
	return string(a)
}

func actionDesc(a keys.Action) string {
	switch a {
	case keys.NewText:
		return locales.DescNewText()
	case keys.NewTodo:
		return locales.DescNewTodo()
	case keys.Summon:
		return locales.DescSummon()
	case keys.Recent:
		return locales.DescRecent()
	case keys.AllSnippets:
		return locales.DescAllSnippets()
	case keys.CopyAll:
		return locales.DescCopyAll()
	case keys.History:
		if runtime.GOOS != "darwin" {
			return locales.DescHistoryWin()
		}
		return locales.DescHistory()
	case keys.CloseTab:
		return locales.DescCloseTab()
	case keys.ToggleSidebar:
		return locales.DescToggleSidebar()
	}
	return ""
}

func (a *App) prefsView(c *ui.Context) {
	p := a.pal
	pg := &a.prefs
	ui.Column(c.Key("settings")).Fill().AlignItems(ui.Stretch).Background(p.Bg).Children(func() {
		a.pageHeader(c, locales.SettingsTitle(), a.CloseSettings)
		ui.Row(c).Grow(1).AlignItems(ui.Stretch).Children(func() {
			nav := ui.Column(c.Key("nav")).Width(200).Gap(2).Padding(12, 8).Background(p.Side).
				BorderWidth(0, 1, 0, 0).BorderColor(p.Line).Role(ui.RoleTabList).Orientation(ui.Vertical).
				Label(locales.SettingsSections())
			nav.Children(func() {
				for i, label := range []string{locales.SecGeneral(), locales.SecShortcuts(), locales.SecEditing(),
					locales.SecAgent(), locales.SecStorage(), locales.SecAbout()} {
					on := pg.section == i
					b := ui.ButtonBase(c.Key(i)).Height(32).Padding(0, 12).Radius(8).Row().AlignItems(ui.Center).Justify(ui.Start).
						Role(ui.RoleTab).Selected(on).Label(label)
					switch {
					case on:
						b.Background(a.acc.Sel)
					case b.Hovered():
						b.Background(p.Chrome)
					}
					w, col := 400, p.Text2
					if on {
						w, col = 500, p.Text
					}
					b.Children(func() { ui.Text(c, label).FontSize(13).FontWeight(w).TextColor(col) })
					if b.Clicked() && !on {
						a.stopRecording()
						pg.section = i
					}
				}
			})
			ui.Scroll(c.Key(fmt.Sprint("sec", pg.section))).Column().Grow(1).Shrink(1).MinWidth(0).
				AlignItems(ui.Stretch).Padding(24, 32).Gap(4).Role(ui.RoleTabPanel).Children(func() {
				ui.Column(c).MaxWidth(720).AlignItems(ui.Stretch).Gap(4).Children(func() {
					switch pg.section {
					case secGeneral:
						a.prefsGeneral(c)
					case secShortcuts:
						a.prefsShortcuts(c)
					case secEditing:
						a.prefsEditing(c)
					case secAgent:
						a.prefsAgent(c)
					case secStorage:
						a.prefsStorage(c)
					case secAbout:
						a.prefsAbout(c)
					}
				})
			})
		})
	})
}

// pageHeader is the title bar of a full page: its title, and Back.
func (a *App) pageHeader(c *ui.Context, title string, back func()) {
	p := a.pal
	bar := c.TitleBar()
	ui.Row(c).Height(max(bar.Height, theme.TitleBarHeight)).Padding(0, bar.Right+8, 0, bar.Left+16).
		Gap(16).AlignItems(ui.Center).Background(p.Chrome).
		BorderWidth(0, 0, 1, 0).BorderColor(p.ChromeLine).DragWindow().Children(func() {
		ui.Text(c, title).FontSize(13).FontWeight(600).Role(ui.RoleHeading)
		ui.Spacer(c)
		b := ui.ButtonBase(c.Key("back")).Row().Gap(8).AlignItems(ui.Center).Height(28).Padding(0, 12).
			Radius(8).Border(1, p.BtnLine).Label(locales.BackToEditor()).KeyShortcuts(0, ui.KeyEscape)
		if b.Hovered() {
			b.Background(p.BadgeTab)
		}
		b.Children(func() {
			ui.Text(c, locales.BackToEditor()).FontSize(12).TextColor(p.Text)
			ui.Text(c, keyName("esc")).FontSize(12).TextColor(p.Muted)
		})
		if b.Clicked() {
			back()
		}
	})
}

func (a *App) sectionTitle(c *ui.Context, title, sub string) {
	ui.Text(c, title).FontSize(17).FontWeight(600).Role(ui.RoleHeading).Margin(0, 0, 4, 0)
	if sub != "" {
		ui.Text(c, sub).FontSize(12.5).LineHeight(1.5).TextColor(a.pal.Muted).Margin(0, 0, 12, 0)
	}
}

// prefRow is a setting: its name and description on the left, its control
// on the right, a line under it.
func (a *App) prefRow(c *ui.Context, key, label, desc string, control func()) {
	p := a.pal
	ui.Row(c.Key(key)).Gap(24).AlignItems(ui.Center).Padding(14, 0).
		BorderWidth(0, 0, 1, 0).BorderColor(p.LineSoft).Children(func() {
		ui.Column(c).Grow(1).Shrink(1).Gap(4).Children(func() {
			ui.Text(c, label).FontSize(13.5).FontWeight(500)
			if desc != "" {
				ui.Text(c, desc).FontSize(12).LineHeight(1.5).TextColor(p.Muted)
			}
		})
		control()
	})
}

func (a *App) prefsGeneral(c *ui.Context) {
	set := &a.settings
	a.sectionTitle(c, locales.SecGeneral(), "")

	modes := []string{"system", "light", "dark"}
	a.prefRow(c, "appearance", locales.Appearance(), locales.AppearanceDesc(), func() {
		i := max(slices.Index(modes, set.Appearance), 0)
		if ui.Segmented(c.Key("seg"), &i, locales.FollowSystem(), locales.Light(), locales.Dark()).
			Label(locales.Appearance()).Changed() {
			a.SetAppearance(modes[i])
		}
	})
	langs := []string{"system", "zh-Hans", "en"}
	a.prefRow(c, "language", locales.Language(), locales.LanguageDesc(), func() {
		i := max(slices.Index(langs, set.Language), 0)
		if ui.Segmented(c.Key("seg"), &i, locales.FollowSystem(), "简体中文", "English").
			Label(locales.Language()).Changed() {
			a.SetLanguage(langs[i])
		}
	})
	a.prefRow(c, "accent", locales.AccentColor(), "", func() {
		ui.Row(c).Gap(10).Role(ui.RoleRadioGroup).Label(locales.AccentColor()).Children(func() {
			for _, hex := range theme.AccentPresets {
				on := strings.EqualFold(set.Accent, hex)
				sw := ui.ButtonBase(c.Key(hex)).Size(24, 24).Radius(12).
					Background(theme.Accent(hex, c.Theme().Dark)).Role(ui.RoleRadio).Checked(on).Label(hex)
				if on {
					sw.Border(2, a.pal.Bg).Shadow(0, 0, 0, 2, theme.Accent(hex, c.Theme().Dark))
				}
				if sw.Clicked() && !on {
					a.SetAccent(hex)
				}
			}
		})
	})
	a.prefRow(c, "login", locales.OpenAtLoginLabel(), locales.OpenAtLoginDesc(), func() {
		on := set.OpenAtLogin
		if ui.Switch(c.Key("sw"), &on).Label(locales.OpenAtLoginLabel()).Changed() {
			a.prefs.loginErr = ""
			if err := a.SetOpenAtLogin(on); err != nil {
				a.prefs.loginErr = err.Error()
			}
		}
	})
	if a.prefs.loginErr != "" {
		ui.Text(c, a.prefs.loginErr).FontSize(12).TextColor(a.pal.Danger)
	}
}

func (a *App) prefsShortcuts(c *ui.Context) {
	p := a.pal
	pg := &a.prefs
	table := a.Shortcuts()
	a.sectionTitle(c, locales.SecShortcuts(), locales.ShortcutsDesc())
	for _, act := range keys.Customizable {
		recording := pg.recording == act
		ui.Column(c.Key(string(act))).Gap(6).Padding(12, 0).BorderWidth(0, 0, 1, 0).BorderColor(p.LineSoft).Children(func() {
			ui.Row(c).Gap(16).AlignItems(ui.Center).Children(func() {
				ui.Column(c).Grow(1).Shrink(1).Gap(4).Children(func() {
					ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
						ui.Text(c, actionLabel(act)).FontSize(13.5).FontWeight(500)
						if keys.IsGlobal(act) {
							ui.Text(c, locales.GlobalBadge()).FontSize(11).TextColor(a.acc.Accent).
								Background(a.acc.Sel).Radius(4).Padding(1, 6)
						}
					})
					ui.Text(c, actionDesc(act)).FontSize(12).TextColor(p.Muted)
				})
				if recording {
					rec := ui.Box(c.Key("rec")).Row().Gap(8).AlignItems(ui.Center).Height(28).Padding(0, 10).Radius(6).
						Border(1.5, a.acc.Accent).Background(a.acc.Sel).Focusable().Role(ui.RoleStatus).
						Label(locales.Recording()).HandleInput(a.record)
					if pg.focusRec {
						rec.Focus()
						pg.focusRec = false
					}
					rec.Children(func() {
						ui.Text(c, locales.Recording()).FontSize(12.5).TextColor(p.Text)
						ui.Text(c, locales.EscCancels(keyName("esc"))).FontSize(12).TextColor(p.Muted)
					})
				} else {
					keyCaps(c, p, keys.Display(table[act], runtime.GOOS))
				}
				label := locales.Change()
				if recording {
					label = locales.Cancel()
				}
				b := a.outlineButton(c, "change", label, "")
				b.Height(28)
				if b.Clicked() {
					if recording {
						a.stopRecording()
					} else {
						a.stopRecording()
						a.startRecording(act)
					}
				}
			})
			if msg := pg.errs[act]; msg != "" {
				ui.Text(c, msg).FontSize(12).TextColor(p.Danger).Role(ui.RoleAlert)
			}
		})
	}
	ui.Row(c).Justify(ui.End).Padding(16, 0, 0, 0).Children(func() {
		if a.outlineButton(c, "reset", locales.ResetShortcuts(), "").Clicked() {
			a.stopRecording()
			pg.errs = map[keys.Action]string{}
			if err := a.ResetShortcuts(); err != nil {
				pg.errs[keys.Summon] = shortcutErrorText(&ShortcutError{System: err}, a.Shortcuts()[keys.Summon])
			}
		}
	})
}

// keyCaps draws a shortcut as key caps: ⌘ and K, or Ctrl and K.
func keyCaps(c *ui.Context, p *theme.Palette, display string) {
	var caps []string
	if runtime.GOOS == "darwin" {
		for _, r := range display {
			caps = append(caps, string(r))
		}
	} else {
		caps = strings.Split(display, "+")
	}
	ui.Row(c).Gap(4).Label(display).Children(func() {
		for i, k := range caps {
			ui.Text(c.Key(i), k).FontSize(12).FontWeight(500).TextColor(p.Text2).MinWidth(22).Height(22).
				TextAlign(ui.Center).Padding(2, 6).Radius(4).Background(p.Badge).Border(1, p.BtnLine).AccessHidden(true)
		}
	})
}

func (a *App) prefsEditing(c *ui.Context) {
	p := a.pal
	pg := &a.prefs
	set := &a.settings
	a.sectionTitle(c, locales.SecEditing(), locales.FidelityDesc())
	ui.Column(c).Gap(8).Padding(4, 0, 16, 0).BorderWidth(0, 0, 1, 0).BorderColor(p.LineSoft).Children(func() {
		for i, s := range []string{locales.FidelityPaste(), locales.FidelityQuotes(), locales.FidelityAutocorrect(), locales.FidelityWhitespace()} {
			on := true
			ui.Checkbox(c.Key(i), &on, s).Disabled(true).DimDisabled(false)
		}
	})
	if pg.fonts == nil {
		pg.fonts = []string{locales.SystemMonospace()}
		for _, f := range ui.FontFamilies() {
			if f.Monospaced {
				pg.fonts = append(pg.fonts, f.Name)
			}
		}
	}
	a.prefRow(c, "font", locales.EditorFont(), locales.MonoOnly(), func() {
		chosen := set.EditorFont
		if chosen == "" {
			chosen = locales.SystemMonospace()
		}
		if ui.Select(c.Key("family"), &chosen, pg.fonts).Label(locales.EditorFont()).Width(220).Changed() {
			family := chosen
			if family == locales.SystemMonospace() {
				family = ""
			}
			a.SetEditorFont(family, set.EditorSize)
		}
	})
	a.prefRow(c, "size", locales.EditorSize(), "", func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			if a.outlineButton(c, "smaller", "−", "").Width(32).Disabled(set.EditorSize <= MinEditorSize).Clicked() {
				a.SetEditorFont(set.EditorFont, set.EditorSize-0.5)
			}
			ui.Text(c, fmt.Sprintf("%g", set.EditorSize)).FontSize(13).FontFeatures("tnum").MinWidth(36).TextAlign(ui.Center).
				Role(ui.RoleStatus).Label(locales.EditorSize())
			if a.outlineButton(c, "larger", "+", "").Width(32).Disabled(set.EditorSize >= MaxEditorSize).Clicked() {
				a.SetEditorFont(set.EditorFont, set.EditorSize+0.5)
			}
		})
	})
	ui.Text(c, "curl -X POST 'https://api.example.com/v2/orders' -d '{\"page_size\": 50}'  “”—").
		Font(set.EditorFont).FontSize(set.EditorSize).TextColor(p.Text).Padding(12, 16).Radius(8).
		Background(p.Sunken).Border(1, p.Line).Margin(12, 0, 0, 0).Font(fontOrMono(set.EditorFont))
}

func fontOrMono(f string) string {
	if f == "" {
		return monoFont
	}
	return f
}

func (a *App) prefsAgent(c *ui.Context) {
	p := a.pal
	pg := &a.prefs
	set := &a.settings
	a.sectionTitle(c, locales.SecAgent(), locales.AgentDesc())
	a.prefRow(c, "access", locales.AgentAccessLabel(), locales.AgentAccessDesc(), func() {
		on := set.AgentAccess
		if ui.Switch(c.Key("sw"), &on).Label(locales.AgentAccessLabel()).Changed() {
			a.SetAgentAccess(on)
		}
	})
	a.prefRow(c, "mask", locales.AgentMaskLabel(), locales.AgentMaskDesc(), func() {
		on := set.AgentMask
		if ui.Switch(c.Key("sw"), &on).Label(locales.AgentMaskLabel()).Changed() {
			a.SetAgentMask(on)
		}
	})
	ui.Row(c).Gap(10).AlignItems(ui.Start).Padding(12, 14).Radius(8).Background(p.Info).Margin(16, 0, 8, 0).Children(func() {
		ui.Text(c, "ⓘ").FontSize(14).TextColor(p.Muted)
		ui.Text(c, locales.AgentInfo()).FontSize(12.5).LineHeight(1.5).TextColor(p.Text2).Shrink(1)
	})
	ui.Text(c, locales.AgentCLIHint()).FontSize(12).LineHeight(1.5).TextColor(p.Muted)
	ui.Text(c, `lastab search "curl staging" --since 1d`).Font(monoFont).FontSize(12).TextColor(p.Text).
		Padding(6, 10).Radius(6).Background(p.Sunken).Border(1, p.Line).Selectable()

	// Access log.
	if !pg.logLoaded {
		pg.logLoaded = true
		a.loadAccessLog()
	}
	ui.Row(c).AlignItems(ui.Center).Margin(24, 0, 8, 0).Children(func() {
		ui.Text(c, locales.AccessLog()).FontSize(14).FontWeight(600).Role(ui.RoleHeading)
	})
	if len(pg.log) == 0 {
		ui.Text(c, locales.AccessLogEmpty()).FontSize(12.5).TextColor(p.Muted).Padding(12, 0)
		return
	}
	now := time.Now()
	loc := i18n.Locale()
	ui.Column(c.Key("log")).AlignItems(ui.Stretch).Radius(8).Border(1, p.Line).Clip().Role(ui.RoleTable).
		Label(locales.AccessLog()).Children(func() {
		row := func(key string, head bool, cells ...string) {
			r := ui.Row(c.Key(key)).Gap(12).Padding(8, 12).BorderWidth(0, 0, 1, 0).BorderColor(p.LineSoft).Role(ui.RoleRow)
			if head {
				r.Background(p.Status)
			}
			r.Children(func() {
				for i, cell := range cells {
					t := ui.Text(c.Key(i), cell).FontSize(12).SingleLine()
					switch i {
					case 0:
						t.Width(110).FontFeatures("tnum")
					case 1:
						t.Width(100)
					case 2:
						t.Grow(1).Shrink(1).Font(monoFont)
					case 3:
						t.Width(180)
					}
					if head {
						t.TextColor(p.Muted).FontWeight(500).Role(ui.RoleColumnHeader)
					} else {
						t.Role(ui.RoleCell)
					}
				}
			})
		}
		row("head", true, locales.ColTime(), locales.ColSource(), locales.ColAction(), locales.ColSnippet())
		for i, e := range pg.log {
			row(fmt.Sprint(i), false, relTime(now, e.At, loc), e.Client, e.Op+argsText(e.Args), e.target)
		}
	})
}

// argsText writes an access's arguments as a command line would.
func argsText(args map[string]any) string {
	var b strings.Builder
	if q, ok := args["query"].(string); ok {
		fmt.Fprintf(&b, " %q", q)
	}
	if id, ok := args["id"].(string); ok {
		b.WriteString(" " + id)
	}
	var names []string
	for k := range args {
		if k != "query" && k != "id" && k != "limit" {
			names = append(names, k)
		}
	}
	slices.Sort(names)
	for _, k := range names {
		fmt.Fprintf(&b, " --%s %v", k, args[k])
	}
	return b.String()
}

func (a *App) loadAccessLog() {
	path := a.d.AgentLog
	if path == "" {
		return
	}
	st := a.d.Service.Store
	a.d.Async(func() func() {
		if _, err := os.Stat(path); err != nil {
			return nil // no agent has read anything yet
		}
		l, err := agentlog.Open(path)
		if err != nil {
			a.d.Log.Error("access log", "err", err)
			return nil
		}
		defer l.Close()
		es, err := l.Recent(a.ctx(), 20)
		if err != nil {
			a.d.Log.Error("access log", "err", err)
			return nil
		}
		rows := make([]logRow, len(es))
		for i, e := range es {
			rows[i] = logRow{Entry: e, target: locales.ResultsCount(len(e.SnippetIDs))}
			if e.Op == "read" && len(e.SnippetIDs) == 1 {
				if sn, err := st.Get(a.ctx(), e.SnippetIDs[0]); err == nil {
					rows[i].target = listTitle(sn)
				}
			}
		}
		return func() { a.prefs.log = rows }
	})
}

func (a *App) prefsStorage(c *ui.Context) {
	p := a.pal
	pg := &a.prefs
	a.sectionTitle(c, locales.SecStorage(), locales.StorageDesc())
	if !pg.storage.loaded {
		pg.storage.loaded = true
		a.loadStorage()
	}
	dir := a.d.DataDir
	a.prefRow(c, "dir", locales.DataFolder(), "", func() {
		ui.Text(c, dir).Font(monoFont).FontSize(12).TextColor(p.Text2).Shrink(1).Selectable()
		if a.outlineButton(c, "reveal", locales.ShowInFolder(), "").Height(28).Clicked() && a.d.Platform != nil {
			a.d.Platform.ShowInFolder(filepath.Join(dir, paths.DBFile))
		}
	})
	s := pg.storage
	a.prefRow(c, "counts", locales.StoredLabel(), "", func() {
		ui.Text(c, locales.StoredCounts(s.snippets, s.versions, humanBytes(s.bytes))).FontSize(12.5).
			TextColor(p.Text2).FontFeatures("tnum")
	})
}

func (a *App) loadStorage() {
	st := a.d.Service.Store
	dir := a.d.DataDir
	a.d.Async(func() func() {
		var info storageInfo
		info.loaded = true
		info.snippets, _ = st.Count(a.ctx())
		info.versions, _ = st.VersionCount(a.ctx())
		for _, name := range []string{paths.DBFile, paths.DBFile + "-wal", paths.AgentLogFile} {
			if fi, err := os.Stat(filepath.Join(dir, name)); err == nil {
				info.bytes += fi.Size()
			}
		}
		return func() { a.prefs.storage = info }
	})
}

func humanBytes(n int64) string {
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(n)/(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.0f KB", float64(n)/(1<<10))
	}
	return fmt.Sprintf("%d B", n)
}

func (a *App) prefsAbout(c *ui.Context) {
	p := a.pal
	a.sectionTitle(c, locales.SecAbout(), "")
	ui.Text(c, "Lastab").FontSize(20).FontWeight(600)
	v := ""
	if a.d.Platform != nil {
		v = a.d.Platform.Version()
	}
	ui.Text(c, v).FontSize(12.5).TextColor(p.Muted).FontFeatures("tnum").Selectable()
	ui.Text(c, locales.AboutDesc()).FontSize(12.5).LineHeight(1.5).TextColor(p.Text2).Margin(12, 0, 0, 0).MaxWidth(520)
	ui.Text(c, locales.SchemaVersion(store.LatestVersion())).FontSize(12).TextColor(p.Faint).Margin(12, 0, 0, 0)
}
