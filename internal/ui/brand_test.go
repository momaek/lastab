package appui

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestBrandMark: the sidebar's header shows the mark, the wordmark when the
// window has no traffic lights in it (full screen, Windows) and the symbol
// alone beside them; About shows the wordmark.
func TestBrandMark(t *testing.T) {
	dir := os.Getenv("LASTAB_SNAPSHOT_DIR")
	for _, dark := range []bool{false, true} {
		for _, bar := range []ui.TitleBar{{}, {Height: 28, Left: 72}} {
			h := newHarness(t, "")
			h.tt.SetDark(dark)
			h.tt.SetTitleBar(bar)
			if dir != "" {
				h.tt.SetScale(2)
			}
			h.tt.Type("curl https://api.example.com")
			h.settle()
			r, ok := h.tt.Find("Lastab")
			if !ok {
				t.Fatalf("dark %v, bar %v: no mark in the sidebar", dark, bar)
			}
			wide := bar.Left == 0
			if got := r.W > r.H*2; got != wide {
				t.Errorf("dark %v, bar %v: mark %v×%v, want the wordmark %v", dark, bar, r.W, r.H, wide)
			}
			if r.X < bar.Left {
				t.Errorf("dark %v, bar %v: mark at x %v, under the traffic lights", dark, bar, r.X)
			}
			if dir != "" {
				name := map[bool]string{false: "light", true: "dark"}[dark] + map[bool]string{true: "-full", false: "-window"}[wide]
				f, err := os.Create(filepath.Join(dir, "brand-"+name+".png"))
				if err != nil {
					t.Fatal(err)
				}
				png.Encode(f, h.tt.Image())
				f.Close()
			}
		}
	}

	h := newHarness(t, "")
	h.app.OpenSettings(secAbout)
	h.tt.Frame()
	if _, ok := h.tt.Find("Lastab"); !ok {
		t.Error("About shows no mark")
	}
	if dir != "" {
		f, _ := os.Create(filepath.Join(dir, "brand-about.png"))
		png.Encode(f, h.tt.Image())
		f.Close()
	}
}
