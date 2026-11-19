package appui

import (
	"context"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"

	"github.com/momaek/lastab/internal/store"
)

func TestRenameInHeaderAndLibrary(t *testing.T) {
	h := newHarness(t, "")
	h.tt.Type("curl https://x.dev/a")
	h.settle()
	if err := h.tt.Click("Rename title"); err != nil {
		t.Fatal(err)
	}
	h.tt.Command("selectAll")
	h.tt.Type("Staging orders")
	h.tt.Key(0, ui.KeyEnter)
	h.tt.Frame()
	tab := h.app.Active()
	if tab.title != "Staging orders" || tab.titleSource != store.TitleManual {
		t.Fatalf("title %q %q", tab.title, tab.titleSource)
	}
	if !h.tt.HasText("Staging orders") {
		t.Errorf("texts %q", h.tt.Texts())
	}

	// Escape cancels.
	h.tt.Click("Rename title")
	h.tt.Command("selectAll")
	h.tt.Type("nope")
	h.tt.Key(0, ui.KeyEscape)
	if tab.title != "Staging orders" || h.app.rename.where != "" {
		t.Errorf("escape: %q", tab.title)
	}

	// In the library, an empty title goes back to the automatic one, in
	// the open tab too.
	h.tt.Key(ui.Cmd, ui.KeyK)
	h.tt.Frame()
	h.tt.Click("Rename title")
	h.tt.Command("selectAll")
	h.tt.Key(0, ui.KeyBackspace)
	h.tt.Key(0, ui.KeyEnter)
	h.tt.Frame()
	if tab.title != "curl · GET /a" || tab.titleSource != store.TitleAuto {
		t.Errorf("cleared: %q %q", tab.title, tab.titleSource)
	}
	if it := h.app.lib.items[0]; it.Title != "curl · GET /a" {
		t.Errorf("library item %q", it.Title)
	}
}

func TestRecoveredNoticeGoes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lastab.db")
	h := newHarness(t, path)
	h.tt.Type("kept")
	h.settle()
	// A new window over the same data, after a crash.
	h2 := newHarness(t, path)
	h2.app.recovered = true
	h2.tt.Frame()
	if !h2.tt.HasText("Restored after an unexpected quit") {
		t.Fatalf("no notice: %q", h2.tt.Texts())
	}
	h2.tt.Advance(RecoveredFor + time.Second)
	if h2.tt.HasText("Restored after an unexpected quit") {
		t.Error("the notice stayed")
	}
	// The first edit takes it away sooner.
	h3 := newHarness(t, path)
	h3.app.recovered = true
	h3.tt.Frame()
	h3.tt.Type("!")
	h3.tt.Frame()
	if h3.tt.HasText("Restored after an unexpected quit") {
		t.Error("the notice stayed after an edit")
	}
}

func TestHoldShowsShortcuts(t *testing.T) {
	h := newHarness(t, "")
	h.tt.HoldModifiers(ui.Cmd)
	h.tt.Advance(HoldDelay / 2)
	if h.app.HintsShowing() {
		t.Fatal("hints before the delay")
	}
	h.tt.Advance(HoldDelay)
	if !h.app.HintsShowing() || !h.tt.HasText(displayOf(h, "allSnippets")) {
		t.Fatalf("hints %v, texts %q", h.app.HintsShowing(), h.tt.Texts())
	}
	h.tt.HoldModifiers(0)
	if h.app.HintsShowing() {
		t.Error("hints after letting go")
	}
	// Off in Settings.
	h.app.SetHoldHint(false)
	h.tt.HoldModifiers(ui.Cmd)
	h.tt.Advance(2 * HoldDelay)
	if h.app.HintsShowing() {
		t.Error("hints while turned off")
	}
}

func displayOf(h *harness, act string) string {
	for a, acc := range h.app.Shortcuts() {
		if string(a) == act {
			return displayAccel(acc)
		}
	}
	return ""
}

func TestHeaderTools(t *testing.T) {
	h := newHarness(t, "")
	h.tt.Type("x")
	h.settle()
	if err := h.tt.Click("All Snippets…"); err != nil {
		t.Fatal(err)
	}
	if !h.app.LibraryOpen() {
		t.Error("search tool did not open the library")
	}
	h.tt.Key(0, ui.KeyEscape)
	h.tt.Click("Version History")
	if !h.app.HistoryOpen() {
		t.Error("history tool did not open the history")
	}
}

func TestTraySetting(t *testing.T) {
	h := newHarness(t, "")
	h.app.OpenSettings(secGeneral)
	h.tt.Frame()
	h.clickRole(ui.RoleSwitch, trayLabel())
	if h.plat.tray == nil || *h.plat.tray || setting[bool](t, h.st, store.SettingTray) {
		t.Errorf("tray off: %v", h.plat.tray)
	}
}

func TestLibraryCustomRange(t *testing.T) {
	h := newHarness(t, "")
	ctx := context.Background()
	now := time.Now()
	for _, d := range []int{0, 3, 10, 40} {
		at := now.AddDate(0, 0, -d)
		h.st.SetClock(func() time.Time { return at })
		h.st.CreateSnippet(ctx, store.KindText, "day minus "+string(rune('0'+d%10))+" note", d)
	}
	h.st.SetClock(time.Now)
	h.tt.Key(ui.Cmd, ui.KeyK)
	h.tt.Frame()
	if err := h.tt.Click("Custom…"); err != nil {
		t.Fatal(err)
	}
	h.app.lib.draftFrom = startOfDay(now.AddDate(0, 0, -12))
	h.app.lib.draftTo = startOfDay(now.AddDate(0, 0, -2)) // given in the order picked: either works
	if err := h.tt.Click("Apply"); err != nil {
		t.Fatal(err)
	}
	h.tt.Frame()
	if n := len(h.app.lib.items); n != 2 {
		t.Fatalf("custom range: %d results", n)
	}
	want := i18n.FormatDateRange(i18n.Locale(), h.app.lib.from, h.app.lib.to, i18n.DateOptions{Skeleton: "MMMd"})
	if h.app.lib.when != timeCustom || !h.tt.HasText(want) {
		t.Errorf("chip should show %q: %q", want, h.tt.Texts())
	}
}

// Acceptance A8, A17: the switcher lists open tabs only, to-do lists among
// them. A13: the app starts in the editor, not a list.
func TestSwitcherListsOpenTabsOnly(t *testing.T) {
	h := newHarness(t, "")
	if h.app.LibraryOpen() || h.app.SettingsOpen() || h.app.HistoryOpen() || h.app.SwitcherOpen() {
		t.Fatal("the app should start in the editor")
	}
	if _, ok := h.tt.Find("Snippet text"); !ok {
		t.Fatal("no editor at launch")
	}
	h.tt.Type("closed one")
	h.settle()
	h.tt.Key(ui.Cmd, ui.KeyN)
	h.tt.Type("open one")
	h.settle()
	h.tt.Key(ui.Cmd, ui.KeyT)
	h.tt.Type("buy milk")
	h.settle()
	closed := ""
	for i, tab := range h.app.Tabs() {
		if tab.Body() == "closed one" {
			closed = tab.ID()
			h.app.CloseTab(i)
			break
		}
	}
	h.settle()
	h.tt.Key(ui.Cmd, ui.KeyBackquote)
	var kinds []store.Kind
	for _, tab := range h.app.switcher.visible() {
		if tab.ID() == closed {
			t.Error("a closed snippet is in the switcher")
		}
		kinds = append(kinds, tab.kind)
	}
	if !slices.Contains(kinds, store.KindTodo) || len(kinds) != 2 {
		t.Errorf("switcher kinds %v", kinds)
	}
	if sn, err := h.st.Get(context.Background(), closed); err != nil || sn.Open {
		t.Errorf("closed snippet: %v %v", sn.Open, err)
	}
}
