package appui

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/momaek/lastab/internal/ui/keys"

	"github.com/momaek/lastab/internal/store"
)

func TestHistoryListsAndCompares(t *testing.T) {
	h := newHarness(t, "")
	h.tt.Type("env=staging-a")
	h.settle()
	h.tt.Type("\npage_size=20")
	h.settle()
	h.press(keys.History)
	if !h.app.HistoryOpen() {
		t.Fatal("Cmd+Y did not open the history")
	}
	hs := &h.app.history
	if len(hs.versions) != 2 || hs.sel != 1 {
		t.Fatalf("versions %d sel %d", len(hs.versions), hs.sel)
	}
	if !h.tt.HasText("Version history") || !h.tt.HasText("First content") || !h.tt.HasText("1 change") {
		t.Errorf("texts %q", h.tt.Texts())
	}
	if hs.diff == nil {
		t.Fatal("no diff")
	}
	// One line added; the missing final newline is not a change.
	if added, deleted := hs.diff.Stats(); added != 1 || deleted != 0 {
		t.Errorf("stats +%d -%d", added, deleted)
	}
	h.tt.Key(0, ui.KeyUp)
	if hs.sel != 0 || !h.tt.HasText("Current · "+versionTime(time.Now(), hs.versions[0].CreatedAt, "en")) {
		t.Errorf("Up: sel %d texts %q", hs.sel, h.tt.Texts())
	}
	h.tt.Key(0, ui.KeyEscape)
	if h.app.HistoryOpen() {
		t.Error("Escape did not close the history")
	}
}

func TestHistoryRestoreAndUndo(t *testing.T) {
	h := newHarness(t, "")
	h.tt.Type("v1")
	h.settle()
	h.tt.Type(" then v2")
	h.settle()
	h.press(keys.History)
	if err := h.tt.Click("Restore this version"); err != nil {
		t.Fatal(err)
	}
	h.tt.Frame()
	tab := h.app.Active()
	if tab.Body() != "v1" {
		t.Fatalf("restored body %q", tab.Body())
	}
	if !strings.HasPrefix(h.app.Toast(), "Restored the ") || !h.tt.HasText("Undo") || !h.tt.HasText("5") {
		t.Fatalf("toast %q texts %q", h.app.Toast(), h.tt.Texts())
	}
	// The countdown ticks.
	h.tt.Advance(2 * time.Second)
	if !h.tt.HasText("3") {
		t.Errorf("countdown: %q", h.tt.Texts())
	}
	ctx := context.Background()
	vs, _ := h.st.Versions(ctx, tab.ID())
	if len(vs) != 3 || vs[0].Body != "v1" || vs[0].RestoredFrom == "" || vs[0].Reason != store.ReasonRestore {
		t.Fatalf("versions after restore %+v", vs)
	}
	if !h.tt.HasText("Restored " + versionTime(time.Now(), vs[2].CreatedAt, "en")) {
		t.Errorf("restored label missing: %q", h.tt.Texts())
	}
	if err := h.tt.Click("Undo"); err != nil {
		t.Fatal(err)
	}
	h.tt.Frame()
	if tab.Body() != "v1 then v2" || h.app.Toast() != "Restore undone — back to the latest text" {
		t.Fatalf("undo: body %q toast %q", tab.Body(), h.app.Toast())
	}
	// Hovering pauses it; away from it, it goes.
	h.tt.Advance(4 * time.Second)
	if h.app.Toast() == "" {
		t.Error("toast went while hovered")
	}
	h.tt.Move(0, 300)
	h.tt.Advance(4 * time.Second)
	if h.app.Toast() != "" {
		t.Errorf("toast stayed: %q", h.app.Toast())
	}
	h.settle()
	if sn, _ := h.st.Get(ctx, tab.ID()); sn.Body != "v1 then v2" {
		t.Errorf("stored body after undo %q", sn.Body)
	}
}

func TestHistoryToastExpires(t *testing.T) {
	h := newHarness(t, "")
	h.tt.Type("a")
	h.settle()
	h.tt.Type("b")
	h.settle()
	h.press(keys.History)
	h.tt.Click("Restore this version")
	h.tt.Move(0, 300) // not over the toast
	h.tt.Advance(UndoFor + time.Second)
	if h.app.Toast() != "" || h.tt.HasText("Undo") {
		t.Errorf("toast after %v: %q", UndoFor, h.app.Toast())
	}
	if h.app.Active().Body() != "a" {
		t.Errorf("body %q", h.app.Active().Body())
	}
}

func TestHistorySingleAndBlank(t *testing.T) {
	h := newHarness(t, "")
	h.press(keys.History)
	if !h.tt.HasText("No versions yet") {
		t.Errorf("blank: %q", h.tt.Texts())
	}
	h.press(keys.History)
	h.tt.Type("kubectl get pods")
	h.settle()
	h.press(keys.History)
	if !h.tt.HasText("Only one version so far") {
		t.Errorf("single: %q", h.tt.Texts())
	}
	if err := h.tt.Click("Restore this version"); err != nil {
		t.Fatal(err)
	}
	if h.app.Toast() != "" {
		t.Error("restored with one version")
	}
}

func TestHistoryFromLibrary(t *testing.T) {
	h := newHarness(t, "")
	h.tt.Type("closed one")
	h.settle()
	h.tt.Type(" edited")
	h.settle()
	h.tt.Key(ui.Cmd, ui.KeyW)
	h.settle()
	h.tt.Key(ui.Cmd, ui.KeyK)
	h.tt.Type("closed")
	h.tt.Frame()
	if err := h.tt.Click("Version history"); err != nil {
		t.Fatal(err)
	}
	h.tt.Frame()
	if h.app.LibraryOpen() || !h.app.HistoryOpen() || h.app.Active().Body() != "closed one edited" {
		t.Fatalf("library %v history %v active %q", h.app.LibraryOpen(), h.app.HistoryOpen(), h.app.Active().Body())
	}
	if n := len(h.app.history.versions); n < 2 {
		t.Errorf("versions %d", n)
	}
}
