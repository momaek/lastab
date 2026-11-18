package appui

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func bodies(a *App) []string {
	var out []string
	for _, t := range a.Tabs() {
		out = append(out, t.Body())
	}
	return out
}

func TestSidebarOrdersByLastEditWithoutJumping(t *testing.T) {
	h := newHarness(t, "")
	for i, s := range []string{"alpha", "beta", "gamma"} {
		if i > 0 {
			h.tt.Key(ui.Cmd, ui.KeyN)
		}
		h.tt.Type(s)
		h.settle()
		time.Sleep(5 * time.Millisecond)
	}
	if got := bodies(h.app); got[0] != "gamma" || got[2] != "alpha" {
		t.Fatalf("order %q", got)
	}
	// Click the oldest and type: it stays where it is while being edited.
	if err := h.tt.Click("alpha"); err != nil {
		t.Fatal(err)
	}
	h.tt.Type(" edited")
	h.settle()
	if got := bodies(h.app); got[2] != "alpha edited" || h.app.Active().Body() != "alpha edited" {
		t.Fatalf("moved while typing: %q", got)
	}
	// Leaving it brings it to the top.
	h.tt.Click("beta")
	if got := bodies(h.app); got[0] != "alpha edited" || h.app.Active().Body() != "beta" {
		t.Fatalf("after switching: %q, active %q", got, h.app.Active().Body())
	}
	// The switcher lists the same order, the current one first.
	h.tt.Key(ui.Cmd, ui.KeyBackquote)
	vis := h.app.switcher.visible()
	if vis[0].Body() != "beta" || vis[1].Body() != "alpha edited" {
		t.Errorf("switcher order: %q, %q", vis[0].Body(), vis[1].Body())
	}
}

func TestSidebarCloseGoesToMostRecent(t *testing.T) {
	h := newHarness(t, "")
	for i, s := range []string{"one", "two", "three"} {
		if i > 0 {
			h.tt.Key(ui.Cmd, ui.KeyN)
		}
		h.tt.Type(s)
		h.settle()
		time.Sleep(5 * time.Millisecond)
	}
	h.tt.Click("one")
	h.tt.Key(ui.Cmd, ui.KeyW)
	if got := h.app.Active().Body(); got != "three" {
		t.Errorf("after closing, active %q", got)
	}
}

func TestSidebarToggle(t *testing.T) {
	h := newHarness(t, "")
	h.tt.Type("x")
	if _, ok := h.tt.Find("Hide sidebar"); !ok {
		t.Fatal("no Hide sidebar button")
	}
	h.tt.Key(ui.Cmd, ui.KeyBackslash)
	if !h.app.SidebarHidden() {
		t.Fatal("Cmd+\\ did not hide the sidebar")
	}
	if err := h.tt.Click("Show sidebar"); err != nil {
		t.Fatal(err)
	}
	if h.app.SidebarHidden() {
		t.Error("Show sidebar did not show it")
	}
}
