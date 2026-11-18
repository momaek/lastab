package theme

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestOnAccent(t *testing.T) {
	if d := Derive(&Light, ui.Hex("#2F5BD3"), false); d.OnAccent != ui.Hex("#FFFFFF") {
		t.Errorf("blue accent: %v", d.OnAccent)
	}
	if d := Derive(&Dark, Accent("#1C1C1A", true), true); d.OnAccent != ui.Hex("#111111") {
		t.Errorf("lifted near-black accent on dark: %v", d.OnAccent)
	}
}

func TestBuild(t *testing.T) {
	th, p, _ := Build(true, "")
	if th.Background != Dark.Bg || p != &Dark || !th.Dark {
		t.Error("dark theme")
	}
}
