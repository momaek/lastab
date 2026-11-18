package appui

import (
	"context"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/momaek/lastab/internal/store"
)

// seed makes three snippets through the UI and closes the middle one.
func seed(h *harness) {
	h.tt.Type("curl -X POST 'https://api.staging.example.com/v2/orders' -H 'Authorization: Bearer abcdefgh12345678'")
	h.settle()
	h.tt.Key(ui.Cmd, ui.KeyN)
	h.tt.Type("周会纪要：staging-b 回归通过后发版")
	h.settle()
	h.tt.Key(ui.Cmd, ui.KeyW) // closed, still findable
	h.settle()
	h.tt.Key(ui.Cmd, ui.KeyT)
	h.tt.Type("确认 staging 数据")
	h.settle()
}

func libTitles(a *App) []string {
	var out []string
	for _, it := range a.lib.items {
		out = append(out, it.Body)
	}
	return out
}

func TestLibrarySearchFindsClosed(t *testing.T) {
	h := newHarness(t, "")
	seed(h)
	if len(h.app.Tabs()) != 2 {
		t.Fatalf("tabs %d", len(h.app.Tabs()))
	}
	h.tt.Key(ui.Cmd, ui.KeyK)
	if !h.app.LibraryOpen() || len(h.app.lib.items) != 3 || h.app.lib.total != 3 {
		t.Fatalf("library %v items %d", h.app.LibraryOpen(), len(h.app.lib.items))
	}
	if !h.tt.HasText("3 snippets") || !h.tt.HasText("Today") {
		t.Errorf("texts %q", h.tt.Texts())
	}
	h.tt.Type("周会")
	h.tt.Frame()
	if len(h.app.lib.items) != 1 || !strings.HasPrefix(h.app.lib.items[0].Body, "周会") || h.app.lib.items[0].Open {
		t.Fatalf("search 周会: %q", libTitles(h.app))
	}
	if !h.tt.HasText("1 match · 3 total") || !h.tt.HasText("Closed") {
		t.Errorf("texts %q", h.tt.Texts())
	}

	// Enter opens the closed snippet in a new tab and goes back to the editor.
	h.tt.Key(0, ui.KeyEnter)
	h.tt.Frame()
	if h.app.LibraryOpen() || !strings.HasPrefix(h.app.Active().Body(), "周会") || len(h.app.Tabs()) != 3 {
		t.Fatalf("open: library %v active %q tabs %d", h.app.LibraryOpen(), h.app.Active().Body(), len(h.app.Tabs()))
	}
	if sn, _ := h.st.Get(context.Background(), h.app.Active().ID()); !sn.Open {
		t.Error("opened snippet not in the workspace")
	}
}

func TestLibraryFiltersAndKeys(t *testing.T) {
	h := newHarness(t, "")
	seed(h)
	h.tt.Key(ui.Cmd, ui.KeyK)
	h.tt.Type("staging")
	h.tt.Frame()
	if len(h.app.lib.items) != 3 {
		t.Fatalf("staging: %q", libTitles(h.app))
	}
	if err := h.tt.Click("To-do"); err != nil {
		t.Fatal(err)
	}
	h.tt.Frame()
	if len(h.app.lib.items) != 1 || h.app.lib.items[0].Kind != store.KindTodo {
		t.Fatalf("to-do filter: %q", libTitles(h.app))
	}
	h.tt.Click("Closed")
	h.tt.Frame()
	if len(h.app.lib.items) != 0 || !h.tt.HasText("Nothing matches “staging”") || !h.tt.HasText("Clear filters") {
		t.Fatalf("closed to-dos: %q / %q", libTitles(h.app), h.tt.Texts())
	}
	h.tt.Click("Clear filters")
	h.tt.Frame()
	if len(h.app.lib.items) != 3 {
		t.Fatalf("cleared: %q", libTitles(h.app))
	}
	h.tt.Key(0, ui.KeyDown)
	h.tt.Key(0, ui.KeyDown)
	h.tt.Key(0, ui.KeyDown) // stops at the last
	if h.app.lib.sel != 2 {
		t.Errorf("sel %d", h.app.lib.sel)
	}
	h.tt.Key(0, ui.KeyUp)
	if h.app.lib.sel != 1 {
		t.Errorf("sel %d", h.app.lib.sel)
	}
	h.tt.Key(0, ui.KeyEscape)
	if h.app.LibraryOpen() {
		t.Error("Escape did not go back")
	}
}

func TestLibraryMasksAndCopies(t *testing.T) {
	h := newHarness(t, "")
	seed(h)
	h.tt.Key(ui.Cmd, ui.KeyK)
	h.tt.Type("curl")
	h.tt.Frame()
	all := strings.Join(h.tt.Texts(), "\n")
	if strings.Contains(all, "abcdefgh") {
		t.Fatal("secret shown in the library")
	}
	h.tt.Click("Reveal secrets")
	if !strings.Contains(strings.Join(h.tt.Texts(), "\n"), "abcdefgh") {
		t.Error("reveal did not show the secret")
	}
	// Copy all copies the selected snippet exactly, secrets included, and
	// the menu's key does the same while the library shows.
	h.tt.Key(ui.Cmd|ui.Shift, ui.KeyC)
	if !strings.Contains(h.clip.text, "Bearer abcdefgh12345678") {
		t.Errorf("copied %q", h.clip.text)
	}
	if !h.app.LibraryOpen() || len(h.app.Tabs()) != 2 {
		t.Error("copy changed the window")
	}
}

func TestLibraryDelete(t *testing.T) {
	h := newHarness(t, "")
	seed(h)
	h.tt.Key(ui.Cmd, ui.KeyK)
	h.tt.Type("确认")
	h.tt.Frame()
	if err := h.tt.Click("Delete…"); err != nil {
		t.Fatal(err)
	}
	if !h.tt.HasText("Delete this snippet?") {
		t.Fatalf("no confirmation: %q", h.tt.Texts())
	}
	// Escape closes the confirmation, not the library.
	h.tt.Key(0, ui.KeyEscape)
	if h.tt.HasText("Delete this snippet?") || !h.app.LibraryOpen() {
		t.Fatalf("escape: %q", h.tt.Texts())
	}
	h.tt.Click("Delete…")
	if err := h.tt.Click("Delete snippet"); err != nil {
		t.Fatal(err)
	}
	h.tt.Frame()
	ctx := context.Background()
	if n, _ := h.st.Count(ctx); n != 2 {
		t.Fatalf("snippets after delete: %d", n)
	}
	if len(h.app.Tabs()) != 1 {
		t.Errorf("deleted snippet's tab stayed: %d tabs", len(h.app.Tabs()))
	}
	if len(h.app.lib.items) != 0 {
		t.Errorf("results %q", libTitles(h.app))
	}
}

func TestLibrarySeesUnsavedEdits(t *testing.T) {
	h := newHarness(t, "")
	h.tt.Type("typed just now")
	h.tt.Key(ui.Cmd, ui.KeyK) // before the save delay
	h.tt.Type("just now")
	h.tt.Frame()
	if len(h.app.lib.items) != 1 {
		t.Fatalf("results %q", libTitles(h.app))
	}
}
