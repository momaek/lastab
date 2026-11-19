package appui

import (
	"runtime"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/momaek/lastab/internal/ui/keys"
	"github.com/momaek/lastab/locales"
)

// HoldDelay is how long the primary modifier (⌘, or Ctrl on Windows) is
// held alone before the buttons show their shortcuts.
const HoldDelay = 500 * time.Millisecond

// holdHint shows each button's shortcut under it while ⌘ is held alone.
type holdHint struct {
	since   time.Time
	showing bool
}

// HintsShowing reports whether the buttons show their shortcuts.
func (a *App) HintsShowing() bool { return a.hold.showing }

// holdTick follows the modifier: held alone for HoldDelay, the hints
// show; another key or letting go hides them. Never while the switcher or
// a recording is up, or when the user turned hints off.
func (a *App) holdTick(c *ui.Context) {
	h := &a.hold
	alone := c.Modifiers() == ui.Cmd && len(c.KeysDown()) == 0
	if !alone || !a.settings.HoldHint || a.switcher.open || a.prefs.recording != "" {
		*h = holdHint{}
		return
	}
	now := c.Now()
	if h.since.IsZero() {
		h.since = now
	}
	if wait := HoldDelay - now.Sub(h.since); wait > 0 {
		c.After(wait)
		return
	}
	h.showing = true
}

// keyBadge shows an action's shortcut under an element while hints show.
func (a *App) keyBadge(c *ui.Context, anchor ui.Element, act keys.Action) {
	if !a.hold.showing || act == "" {
		return
	}
	acc := a.Shortcuts()[act]
	if acc == "" {
		return
	}
	p := a.pal
	ui.Overlay(c, func() {
		ui.Text(c.Key("hint-"+string(act)), keys.Display(acc, runtime.GOOS)).
			AttachTo(anchor, ui.AnchorBottom, ui.AnchorTop).Margin(4, 0, 0, 0).
			FontSize(11).Font(monoFont).Padding(2, 6).Radius(4).
			Background(p.InvBg).TextColor(p.InvText).NoWrap().PassThrough().AccessHidden(true)
	})
}

// headerTools are the buttons at the right of the editor's title bar.
func (a *App) headerTools(c *ui.Context) {
	ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
		for _, tool := range []struct {
			key, glyph, label string
			act               keys.Action
		}{
			{"search", "⌕", locales.AllSnippetsMenu(), keys.AllSnippets},
			{"recent", "⇆", locales.RecentMenu(), keys.Recent},
			{"history", "◷", locales.HistoryMenu(), keys.History},
		} {
			act := tool.act
			a.iconButton(c, tool.key, tool.glyph, tool.label, act).OnClick(func() { a.Run(act, c.Now()) })
		}
	})
}

// displayAccel writes an accelerator as this platform shows it.
func displayAccel(acc string) string { return keys.Display(acc, runtime.GOOS) }
