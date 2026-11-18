package appui

import (
	"fmt"
	"runtime"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"

	"github.com/momaek/lastab/internal/theme"
	"github.com/momaek/lastab/internal/ui/keys"
	"github.com/momaek/lastab/locales"
)

// View builds the main window.
func (a *App) View(c *ui.Context) {
	dark := c.Theme().Dark
	th, pal, acc := theme.Build(dark, a.d.Accent)
	c.SetTheme(th)
	a.pal, a.acc = pal, acc

	a.frameNow = c.Now()
	a.tick(c)
	a.shortcuts(c)
	if a.lib.open {
		a.libraryView(c)
		return
	}
	if cur := a.Active(); cur != a.lastActive {
		a.lastActive, a.scrollActive = cur, true
	}
	if a.history.open && a.history.tab != a.Active() {
		// The history follows the active snippet.
		a.history = history{open: true, tab: a.Active()}
		a.loadHistory(true)
	}
	ui.Row(c).Fill().AlignItems(ui.Stretch).Background(pal.Bg).Children(func() {
		if !a.sidebarHidden {
			a.sidebar(c)
		}
		ui.Column(c.Key("main")).Grow(1).Shrink(1).MinWidth(0).AlignItems(ui.Stretch).Children(func() {
			a.mainHeader(c)
			if a.history.open {
				a.compareView(c)
				return
			}
			a.editorArea(c)
			a.statusBar(c)
		})
		if a.history.open {
			a.historyDrawer(c)
		}
	})
	a.scrollActive = false
	a.switcherView(c)
	a.toastView(c)
}

func (a *App) shortcuts(c *ui.Context) {
	run := func(act keys.Action) func() { return func() { a.Run(act, c.Now()) } }
	c.OnWindowShortcut(ui.Cmd, ui.KeyN, run(keys.NewText))
	c.OnWindowShortcut(ui.Cmd, ui.KeyT, run(keys.NewTodo))
	c.OnWindowShortcut(ui.Cmd, ui.KeyW, run(keys.CloseTab))
	c.OnWindowShortcut(ui.Cmd, ui.KeyK, run(keys.AllSnippets))
	c.OnWindowShortcut(ui.Cmd|ui.Shift, ui.KeyC, run(keys.CopyAll))
	c.OnWindowShortcut(ui.Cmd, ui.KeyBackslash, run(keys.ToggleSidebar))
	if !a.switcher.open {
		c.OnWindowShortcut(ui.Cmd, ui.KeyBackquote, run(keys.Recent))
	}
	if runtime.GOOS == "darwin" {
		c.OnWindowShortcut(ui.Cmd, ui.KeyY, run(keys.History))
	} else {
		c.OnWindowShortcut(ui.Cmd|ui.Shift, ui.KeyY, run(keys.History))
	}
	if a.history.open && !a.switcher.open {
		c.OnWindowShortcut(0, ui.KeyEscape, a.CloseHistory)
		c.OnWindowShortcut(0, ui.KeyDown, func() { a.moveHistory(1) })
		c.OnWindowShortcut(0, ui.KeyUp, func() { a.moveHistory(-1) })
	}
	if !a.lib.open || a.lib.confirm {
		return
	}
	c.OnWindowShortcut(0, ui.KeyEscape, a.CloseLibrary)
	// Wherever the focus is in the page; the search field takes them first.
	c.OnWindowShortcut(0, ui.KeyDown, func() { a.lib.move(1) })
	c.OnWindowShortcut(0, ui.KeyUp, func() { a.lib.move(-1) })
}

func tabTitle(t *Tab) string {
	switch {
	case t.title != "":
		return t.title
	case t.id == "":
		return locales.NewTab()
	default:
		return locales.Untitled(t.createdAt.Local().Format("15:04"))
	}
}

func (a *App) editorArea(c *ui.Context) {
	t := a.Active()
	p := a.pal
	th := editor.DefaultTheme(c.Theme())
	th.Background = p.Bg
	th.Text = p.Text
	th.ActiveLine = p.CurLine
	th.LineNumber = p.Faint
	th.ActiveLineNumber = p.Muted
	th.Match = p.Mark
	t.ed.Theme = th
	t.ed.Placeholder = locales.EditorPlaceholder()
	t.ed.Label = locales.EditorLabel()
	ed := editor.View(c.Key(t.key), t.ed).Grow(1).FillWidth().Role(ui.RoleTabPanel)
	if a.focusEditor && !a.switcher.open {
		ed.Focus()
		a.focusEditor = false
	}
}

func lineEndingName(e string) string {
	switch e {
	case "\r\n":
		return "CRLF"
	case "\r":
		return "CR"
	}
	return "LF"
}

func clock(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	t = t.Local()
	now := time.Now()
	if t.Year() == now.Year() && t.YearDay() == now.YearDay() {
		return t.Format("15:04")
	}
	return t.Format("2006-01-02 15:04")
}

func (a *App) statusBar(c *ui.Context) {
	p := a.pal
	t := a.Active()
	small := float32(theme.SmallFontSize)
	ui.Row(c).Height(theme.StatusBarHeight).Padding(0, 8, 0, 12).Gap(12).AlignItems(ui.Center).
		Background(p.Status).BorderWidth(1, 0, 0, 0).BorderColor(p.Line).
		Children(func() {
			// Keeping state, on the left; failures stay with a retry.
			ui.Row(c).Gap(6).AlignItems(ui.Center).Role(ui.RoleStatus).Children(func() {
				switch {
				case t.saveErr != nil:
					ui.Box(c).Size(6, 6).Radius(3).Background(p.Danger)
					ui.Text(c, locales.SaveFailed(t.saveErr.Error())).FontSize(small).TextColor(p.Danger).SingleLine().MaxWidth(320)
					if ui.Button(c, locales.Retry()).FontSize(small).Clicked() {
						a.Retry(t)
					}
				case t.saving || t.buf().Version() != t.savedVer:
					ui.Text(c, locales.Saving()).FontSize(small).TextColor(p.Faint)
				case a.recovered:
					ui.Box(c).Size(6, 6).Radius(3).Background(p.OkDot)
					ui.Text(c, locales.Recovered()).FontSize(small).TextColor(p.OkText)
				case t.id != "":
					ui.Box(c).Size(6, 6).Radius(3).Background(p.OkDot)
					ui.Text(c, locales.Saved()).FontSize(small).TextColor(p.Muted)
				}
			})
			if t.id != "" {
				ui.Text(c, locales.Modified(clock(t.modifiedAt))).FontSize(small).TextColor(p.Faint).
					FontFeatures("tnum").Tooltip(locales.Created(clock(t.createdAt)))
			}
			ui.Spacer(c)
			ui.Text(c, fmt.Sprintf("%s · UTF-8 · %s", locales.PlainText(), lineEndingName(t.buf().LineEnding()))).
				FontSize(small).TextColor(p.Faint)
			ui.Text(c, locales.Chars(charCount(t))).FontSize(small).TextColor(p.Faint).FontFeatures("tnum")
			a.copyButton(c)
		})
}

func (a *App) copyButton(c *ui.Context) {
	p := a.pal
	copied := c.Now().Before(a.copiedUntil)
	label := locales.CopyAll()
	if copied {
		label = locales.Copied()
	}
	b := ui.ButtonBase(c.Key("copyAll")).Row().AlignItems(ui.Center).Height(24).Padding(0, 10).Radius(6).
		Border(1, p.BtnLine).Background(p.Btn).Label(label).
		KeyShortcuts(ui.Cmd|ui.Shift, ui.KeyC).
		Tooltip(locales.CopyAll() + "  " + keys.Display(keys.Default(keys.CopyAll), runtime.GOOS))
	if b.Hovered() {
		b.Background(p.Chrome)
	}
	b.Children(func() {
		col := p.Text2
		if copied {
			col = p.OkText
			ui.Text(c, "✓ ").FontSize(theme.SmallFontSize).TextColor(col)
		}
		ui.Text(c, label).FontSize(theme.SmallFontSize).TextColor(col)
	})
	if b.Clicked() {
		a.CopyAll(c.Now())
		c.Announce(locales.CopiedAnnounce())
	}
}
