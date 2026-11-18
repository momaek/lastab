package appui

import (
	"fmt"
	"runtime"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"

	"github.com/momaek/lastab/internal/store"
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

	a.shortcuts(c)
	a.tick(c)

	ui.Column(c).Fill().AlignItems(ui.Stretch).Background(pal.Bg).Children(func() {
		a.titleBar(c)
		a.editorArea(c)
		a.statusBar(c)
	})
	a.switcherView(c)
}

func (a *App) shortcuts(c *ui.Context) {
	c.OnWindowShortcut(ui.Cmd, ui.KeyN, func() { a.NewTab(store.KindText) })
	c.OnWindowShortcut(ui.Cmd, ui.KeyT, func() { a.NewTab(store.KindTodo) })
	c.OnWindowShortcut(ui.Cmd, ui.KeyW, a.CloseActive)
	c.OnWindowShortcut(ui.Cmd|ui.Shift, ui.KeyC, func() { a.CopyAll(c.Now()) })
	if !a.switcher.open {
		c.OnWindowShortcut(ui.Cmd, ui.KeyBackquote, a.OpenSwitcher)
	}
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

func (a *App) titleBar(c *ui.Context) {
	p := a.pal
	bar := c.TitleBar()
	left := bar.Left + 8
	if bar.Left == 0 && runtime.GOOS == "darwin" {
		left = 8
	}
	ui.Row(c).Height(max(bar.Height, theme.TitleBarHeight)).
		Padding(0, bar.Right+8, 0, left).
		Background(p.Chrome).BorderWidth(0, 0, 1, 0).BorderColor(p.ChromeLine).
		AlignItems(ui.End).Gap(2).DragWindow().
		Role(ui.RoleTabList).Label(locales.TabsLabel()).
		Children(func() {
			for i, t := range a.tabs {
				a.tabView(c, i, t)
			}
			plus := ui.ButtonBase(c.Key("new")).Size(28, 28).Radius(6).Margin(0, 0, 6, 4).
				Label(locales.NewSnippetMenu()).Tooltip(locales.NewSnippetMenu() + "  " + keys.Display(keys.Default(keys.NewText), runtime.GOOS))
			if plus.Hovered() {
				plus.Background(p.BadgeTab)
			}
			plus.Children(func() {
				ui.Text(c, "+").FontSize(16).TextColor(p.Muted).Fill().TextAlign(ui.Center)
			})
			if plus.Clicked() {
				a.NewTab(store.KindText)
			}
		})
}

func (a *App) tabView(c *ui.Context, i int, t *Tab) {
	p := a.pal
	active := i == a.active
	tab := ui.ButtonBase(c.Key(t.key)).Row().AlignItems(ui.Center).Gap(6).
		Height(34).MinWidth(96).MaxWidth(220).Padding(0, 6, 0, 12).
		Radius(8, 8, 0, 0).
		Role(ui.RoleTab).Selected(active).Label(tabTitle(t))
	hovered := tab.Hovered()
	switch {
	case active:
		tab.Background(p.Bg).BorderWidth(1, 1, 0, 1).BorderColor(p.ChromeLine).Margin(0, 0, -1, 0)
	case hovered:
		tab.Background(p.BadgeTab)
	}
	tab.Children(func() {
		col := p.Muted
		if active {
			col = p.Text
		}
		if t.kind == store.KindTodo {
			ui.Text(c, "☐").FontSize(12).TextColor(p.Faint)
		}
		ui.Text(c, tabTitle(t)).FontSize(theme.UIFontSize).TextColor(col).SingleLine().Grow(1).Shrink(1)
		show := active || hovered
		cl := ui.ButtonBase(c.Key("close")).Size(18, 18).Radius(4).Label(locales.CloseTab()).
			IgnoreInput(!show).Opacity(map[bool]float32{true: 1, false: 0}[show])
		if cl.Hovered() {
			cl.Background(p.BadgeTab)
		}
		cl.Children(func() {
			ui.Text(c, "×").FontSize(13).TextColor(p.Faint).Fill().TextAlign(ui.Center)
		})
		if cl.Clicked() {
			a.CloseTab(i)
		}
	})
	if tab.Clicked() {
		a.Activate(i)
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
