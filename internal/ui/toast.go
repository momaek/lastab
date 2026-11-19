package appui

import (
	"fmt"
	"math"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/momaek/lastab/internal/theme"
)

// UndoFor is how long an undoable action can be taken back.
const UndoFor = 5 * time.Second

// toast is a short message at the top center, 12 DIPs under the title bar,
// one at a time. It never takes the focus; hovering it pauses its clock.
// With an action it counts down the seconds left to take it.
type toast struct {
	text     string
	action   string
	onAction func()
	deadline time.Time
	left     time.Duration // frozen while hovered
	count    bool          // show the seconds left on the action
}

// showToast replaces the toast showing.
func (a *App) showToast(text string, d time.Duration, action string, onAction func(), now time.Time) {
	a.toast = &toast{text: text, action: action, onAction: onAction, deadline: now.Add(d), left: d, count: action != ""}
}

// Toast returns the text of the toast showing, "" for none.
func (a *App) Toast() string {
	if a.toast == nil {
		return ""
	}
	return a.toast.text
}

func (a *App) toastView(c *ui.Context) {
	t := a.toast
	if t == nil {
		return
	}
	now := c.Now()
	if !now.Before(t.deadline) {
		a.toast = nil
		return
	}
	p := a.pal
	bar := c.TitleBar()
	ui.Overlay(c, func() {
		box := ui.Row(c.Key("toast")).Absolute().Top(max(bar.Height, theme.TitleBarHeight)+theme.ToastOffset).
			Gap(12).AlignItems(ui.Center).Padding(4, 4, 4, 12).Radius(8).
			Background(p.Toast).Border(1, p.ToastLine).Role(ui.RoleStatus)
		// Centered by its own width as the frame lays it out, so it is in
		// place from its first frame.
		w, _ := c.Size()
		box.Left(w/2).TranslatePercent(-50, 0)
		if box.Hovered() {
			t.deadline = now.Add(t.left) // paused
		} else {
			t.left = t.deadline.Sub(now)
		}
		if t.action == "" {
			box.Padding(8, 12)
		}
		box.Children(func() {
			ui.Icon(c, iconCheck).Size(14, 14).TextColor(p.ToastOk)
			ui.Text(c, t.text).FontSize(12.5).TextColor(p.ToastText).NoWrap()
			if t.action == "" {
				return
			}
			b := ui.ButtonBase(c.Key("action")).Row().Gap(8).AlignItems(ui.Center).Height(28).Padding(0, 8).Radius(4).
				Label(t.action).KeepFocus()
			b.Children(func() {
				ui.Text(c, t.action).FontSize(12.5).FontWeight(600).TextColor(p.ToastLink)
				if t.count {
					secs := int(math.Ceil(t.left.Seconds()))
					ui.Text(c, fmt.Sprint(secs)).FontSize(10.5).FontWeight(500).TextColor(p.ToastMuted).
						Size(18, 18).Radius(9).Border(1.5, p.ToastMuted).TextAlign(ui.Center).FontFeatures("tnum")
				}
			})
			if b.Clicked() {
				fn := t.onAction
				a.toast = nil
				if fn != nil {
					fn()
				}
			}
		})
	})
	// Redraw as the seconds tick and when it goes.
	next := t.left - t.left.Truncate(time.Second)
	if next <= 0 {
		next = time.Second
	}
	c.After(min(next, t.deadline.Sub(now)))
}
