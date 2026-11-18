package appui

import (
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/momaek/lastab/internal/redact"
	"github.com/momaek/lastab/internal/search"
	"github.com/momaek/lastab/locales"
)

// switcher is the recent switcher: a popup over the window listing the
// open tabs by when they were last edited. It stays open as modifiers are
// released; Tab and ↓ move down, Shift+Tab and ↑ up, Enter opens, Escape
// cancels, and typing filters.
type switcher struct {
	open   bool
	filter string
	sel    int
	items  []*Tab // ordered when opened; filtered each frame
}

// OpenSwitcher shows the switcher, the snippet edited before the current
// one selected.
func (a *App) OpenSwitcher() {
	items := slices.Clone(a.tabs)
	slices.SortStableFunc(items, func(x, y *Tab) int { return y.modifiedAt.Compare(x.modifiedAt) })
	// The current tab first, so the default is the one before it.
	cur := a.Active()
	if i := slices.Index(items, cur); i > 0 {
		items = slices.Delete(items, i, i+1)
		items = slices.Insert(items, 0, cur)
	}
	a.switcher = switcher{open: true, items: items, sel: min(1, len(items)-1)}
}

// SwitcherOpen reports whether the switcher shows.
func (a *App) SwitcherOpen() bool { return a.switcher.open }

func (s *switcher) visible() []*Tab {
	if strings.TrimSpace(s.filter) == "" {
		return s.items
	}
	var out []*Tab
	for _, t := range s.items {
		if search.Matches(tabTitle(t), t.buf().String(), s.filter) {
			out = append(out, t)
		}
	}
	return out
}

func (s *switcher) move(d, n int) {
	if n == 0 {
		s.sel = 0
		return
	}
	s.sel = ((s.sel+d)%n + n) % n
}

func (a *App) choose(t *Tab) {
	a.switcher.open = false
	if i := slices.Index(a.tabs, t); i >= 0 {
		if i == a.active {
			a.focusEditor = true
		}
		a.Activate(i)
	}
}

func (a *App) switcherView(c *ui.Context) {
	s := &a.switcher
	if !s.open {
		return
	}
	p := a.pal
	vis := s.visible()
	if s.sel >= len(vis) {
		s.sel = max(0, len(vis)-1)
	}
	ui.DialogBase(c, &s.open, func(backdrop, panel ui.Element) {
		backdrop.Background(p.Scrim).AlignItems(ui.Center).Justify(ui.Start).Padding(64, 16, 16, 16)
		panel.Column().Width(560).MaxHeight(480).Radius(12).Background(p.Panel).
			Border(1, p.Frame).Shadow(0, 16, 48, 0, ui.RGBA(0, 0, 0, 0.18)).Clip().
			Label(locales.SwitcherTitle())
		panel.Children(func() {
			in := ui.TextInput(c.Key("filter"), &s.filter).AutoFocus().
				Placeholder(locales.SwitcherFilter()).Label(locales.SwitcherFilter()).
				Role(ui.RoleComboBox).HasPopup(ui.PopupListBox).
				FontSize(14).Padding(12, 14).Background(p.Panel).Border(0, p.Panel).
				BorderWidth(0, 0, 1, 0).BorderColor(p.Line).Radius(0)
			if in.Changed() {
				s.sel = 0
				if len(s.visible()) > 1 && strings.TrimSpace(s.filter) == "" {
					s.sel = 1
				}
			}
			n := len(vis)
			if in.Shortcut(0, ui.KeyTab) || in.Shortcut(0, ui.KeyDown) || in.Shortcut(ui.Cmd, ui.KeyBackquote) {
				s.move(1, n)
			}
			if in.Shortcut(ui.Shift, ui.KeyTab) || in.Shortcut(0, ui.KeyUp) || in.Shortcut(ui.Cmd|ui.Shift, ui.KeyBackquote) {
				s.move(-1, n)
			}
			if (in.Submitted() || in.Shortcut(0, ui.KeyEnter)) && n > 0 {
				a.choose(vis[s.sel])
				return
			}
			list := ui.Scroll(c.Key("items")).Column().Grow(1).Shrink(1).Padding(6).Gap(2).Role(ui.RoleListBox)
			list.Children(func() {
				if n == 0 {
					ui.Text(c, locales.SwitcherEmpty()).TextColor(p.Faint).Padding(16).TextAlign(ui.Center)
				}
				for i, t := range vis {
					a.switcherRow(c, t, i == s.sel, i, n)
				}
			})
			ui.Text(c, locales.SwitcherHint()).FontSize(11).TextColor(p.Faint).
				Padding(8, 14).BorderWidth(1, 0, 0, 0).BorderColor(p.Line)
		})
	})
}

func (a *App) switcherRow(c *ui.Context, t *Tab, selected bool, i, n int) {
	p := a.pal
	row := ui.ButtonBase(c.Key(t.key)).Column().AlignItems(ui.Stretch).Gap(4).Padding(8, 10).Radius(8).
		Role(ui.RoleOption).Selected(selected).PositionInSet(i+1, n).Label(tabTitle(t))
	if selected {
		row.Background(a.acc.Sel)
		row.ScrollIntoView()
	} else if row.Hovered() {
		row.Background(p.Chrome)
	}
	row.Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, tabTitle(t)).TextColor(p.Text).SingleLine().Grow(1).Shrink(1)
			if t == a.Active() {
				ui.Text(c, "●").FontSize(8).TextColor(a.acc.Accent)
			}
			ui.Text(c, clock(t.modifiedAt)).FontSize(11).TextColor(p.Faint).FontFeatures("tnum")
		})
		if selected {
			preview := previewLines(redact.String(t.buf().String()), 3)
			if preview != "" {
				ui.Text(c, preview).FontSize(12).TextColor(p.Muted).MaxLines(3).TextAlign(ui.Start)
			}
		}
	})
	if row.Clicked() {
		a.choose(t)
	}
}

func previewLines(s string, n int) string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		if strings.TrimSpace(l) == "" {
			continue
		}
		out = append(out, strings.TrimRight(l, "\r"))
		if len(out) == n {
			break
		}
	}
	return strings.Join(out, "\n")
}
