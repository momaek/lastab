package appui

import (
	"runtime"
	"time"

	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"

	"github.com/momaek/lastab/internal/store"
	"github.com/momaek/lastab/internal/theme"
	"github.com/momaek/lastab/internal/ui/keys"
	"github.com/momaek/lastab/locales"
)

// SidebarWidth is the width of the list of open snippets.
const SidebarWidth = 232

// ToggleSidebar shows or hides the list of open snippets.
func (a *App) ToggleSidebar() { a.sidebarHidden = !a.sidebarHidden }

// SidebarHidden reports whether the list of open snippets is hidden.
func (a *App) SidebarHidden() bool { return a.sidebarHidden }

// sidebar lists the open snippets down the left, the most recent first,
// each with its title and when it was last edited. The traffic lights of
// macOS sit over its top.
func (a *App) sidebar(c *ui.Context) {
	p := a.pal
	bar := c.TitleBar()
	now := time.Now()
	loc := i18n.Locale()
	c.After(30 * time.Second) // relative times move on

	ui.Column(c.Key("sidebar")).Width(SidebarWidth).AlignItems(ui.Stretch).
		Background(p.Side).BorderWidth(0, 1, 0, 0).BorderColor(p.ChromeLine).Children(func() {
		// The header: room for the traffic lights, then New and Hide.
		ui.Row(c).Height(max(bar.Height, theme.TitleBarHeight)).Padding(0, 8, 0, bar.Left+8).
			Gap(2).AlignItems(ui.Center).DragWindow().Children(func() {
			ui.Spacer(c)
			if a.iconButton(c, "settings", "⚙", locales.OpenSettings(), keys.Settings).Clicked() {
				a.OpenSettings(-1)
			}
			if a.iconButton(c, "hide", "⇤", locales.HideSidebar(), keys.ToggleSidebar).Clicked() {
				a.ToggleSidebar()
			}
			if a.iconButton(c, "new", "+", locales.NewSnippetMenu(), keys.NewText).Clicked() {
				a.NewTab(store.KindText)
			}
		})
		list := ui.Scroll(c.Key("tabs")).Column().Grow(1).Shrink(1).AlignItems(ui.Stretch).Gap(2).
			Padding(0, 8, 8, 8).Role(ui.RoleTabList).Orientation(ui.Vertical).Label(locales.TabsLabel())
		list.Children(func() {
			for i, t := range a.tabs {
				a.sidebarItem(c, i, t, now, loc)
			}
		})
	})
}

func (a *App) sidebarItem(c *ui.Context, i int, t *Tab, now time.Time, loc string) {
	p := a.pal
	active := i == a.active
	item := ui.ButtonBase(c.Key(t.key)).Row().AlignItems(ui.Center).Gap(8).Padding(7, 6, 7, 10).Radius(8).
		Role(ui.RoleTab).Selected(active).Label(tabTitle(t))
	hovered := item.Hovered()
	switch {
	case active:
		item.Background(a.acc.Sel)
		if a.scrollActive {
			item.ScrollIntoView()
		}
	case hovered:
		item.Background(p.Chrome)
	}
	item.Children(func() {
		ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(2).Children(func() {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				if t.kind == store.KindTodo {
					ui.Text(c, "☑").FontSize(12).TextColor(p.Muted)
				}
				col := p.Text2
				if active {
					col = p.Text
				}
				w := 400
				if active {
					w = 600
				}
				ui.Text(c, tabTitle(t)).FontSize(theme.UIFontSize).FontWeight(w).TextColor(col).SingleLine().Shrink(1)
			})
			when := relTime(now, t.touchedAt, loc)
			if c.Modifiers()&ui.Alt != 0 {
				when = exactTime(now, t.touchedAt, loc)
			}
			ui.Text(c, when).FontSize(11.5).TextColor(p.Faint).FontFeatures("tnum").SingleLine()
		})
		show := active || hovered
		cl := ui.ButtonBase(c.Key("close")).Size(20, 20).Radius(4).Label(locales.CloseTab()).
			IgnoreInput(!show).Opacity(map[bool]float32{true: 1, false: 0}[show])
		if cl.Hovered() {
			cl.Background(p.BadgeTab)
		}
		cl.Children(func() {
			ui.Text(c, "×").FontSize(14).TextColor(p.Faint).Fill().TextAlign(ui.Center)
		})
		// After the build: both change the list being built.
		cl.OnClick(func() { a.CloseTab(i) })
	})
	item.OnClick(func() { a.Activate(i) })
}

// iconButton is a small square button with a glyph, its name and key in
// the tooltip.
func (a *App) iconButton(c *ui.Context, key, glyph, label string, act keys.Action) ui.Element {
	p := a.pal
	tip := label
	if act != "" {
		tip += "  " + keys.Display(a.Shortcuts()[act], runtime.GOOS)
	}
	b := ui.ButtonBase(c.Key(key)).Size(28, 28).Radius(6).Label(label).Tooltip(tip)
	if b.Hovered() {
		b.Background(p.BadgeTab)
	}
	b.Children(func() {
		ui.Text(c, glyph).FontSize(16).TextColor(p.Muted).Fill().TextAlign(ui.Center)
	})
	return b
}

// mainHeader is the bar over the editor: the active snippet's title, and,
// with the sidebar hidden, room for the traffic lights and the button that
// shows it again.
func (a *App) mainHeader(c *ui.Context) {
	p := a.pal
	bar := c.TitleBar()
	left := float32(16)
	if a.sidebarHidden {
		left = bar.Left + 8
	}
	ui.Row(c).Height(max(bar.Height, theme.TitleBarHeight)).Padding(0, bar.Right+12, 0, left).
		Gap(8).AlignItems(ui.Center).Background(p.Bg).
		BorderWidth(0, 0, 1, 0).BorderColor(p.Line).DragWindow().Children(func() {
		if a.sidebarHidden {
			if a.iconButton(c, "show", "⇥", locales.ShowSidebar(), keys.ToggleSidebar).Clicked() {
				a.ToggleSidebar()
			}
			if a.iconButton(c, "new", "+", locales.NewSnippetMenu(), keys.NewText).Clicked() {
				a.NewTab(store.KindText)
			}
		}
		ui.Text(c, tabTitle(a.Active())).FontSize(13).FontWeight(600).TextColor(p.Text).SingleLine().Shrink(1).
			Role(ui.RoleHeading)
	})
}
