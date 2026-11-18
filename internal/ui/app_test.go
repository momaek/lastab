package appui

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/momaek/lastab/internal/snippet"
	"github.com/momaek/lastab/internal/store"
)

type fakeClipboard struct{ text string }

func (f *fakeClipboard) WriteText(s string) { f.text = s }
func (f *fakeClipboard) ReadText() string   { return f.text }

type harness struct {
	t    *testing.T
	st   *store.Store
	app  *App
	tt   *ui.Tester
	clip *fakeClipboard
}

func newHarness(t *testing.T, path string) *harness {
	t.Helper()
	if path == "" {
		path = filepath.Join(t.TempDir(), "lastab.db")
	}
	st, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	clip := &fakeClipboard{}
	app := New(Deps{
		Service:   snippet.New(st),
		Clipboard: clip,
		Async: func(work func() func()) {
			if apply := work(); apply != nil {
				apply()
			}
		},
	})
	if err := app.Load(); err != nil {
		t.Fatal(err)
	}
	tt := ui.NewTester(app.View, 900, 600)
	return &harness{t, st, app, tt, clip}
}

// settle advances the clock past every delay of automatic keeping.
func (h *harness) settle() {
	h.tt.Frame()
	h.tt.Advance(SnapshotDelay + time.Second)
	h.tt.Frame()
}

func TestBlankTabIsNotStored(t *testing.T) {
	h := newHarness(t, "")
	h.tt.Type("   \n\n")
	h.settle()
	if n, _ := h.st.Count(context.Background()); n != 0 {
		t.Fatalf("blank tab stored: %d snippets", n)
	}
	if !h.tt.HasText("New snippet") {
		t.Errorf("texts %q", h.tt.Texts())
	}
}

func TestTypeKeepsAndTitles(t *testing.T) {
	h := newHarness(t, "")
	h.tt.Type("curl https://example.com/v2/orders")
	h.settle()
	ctx := context.Background()
	sns, _ := h.st.List(ctx, store.ListOptions{})
	if len(sns) != 1 {
		t.Fatalf("snippets: %d", len(sns))
	}
	if sns[0].Body != "curl https://example.com/v2/orders" || !sns[0].Open {
		t.Fatalf("stored %+v", sns[0])
	}
	if sns[0].Title != "curl · GET /v2/orders" {
		t.Errorf("title %q", sns[0].Title)
	}
	if !h.tt.HasText("curl · GET /v2/orders") || !h.tt.HasText("Kept automatically") {
		t.Errorf("texts %q", h.tt.Texts())
	}
	vs, _ := h.st.Versions(ctx, sns[0].ID)
	if len(vs) != 1 || vs[0].Reason != store.ReasonPaused {
		t.Errorf("versions %+v", vs)
	}
}

func TestCopyAllKeepsLineEndings(t *testing.T) {
	h := newHarness(t, "")
	h.clip.text = "a\r\nb\r\n"
	h.tt.Command("paste")
	h.settle()
	if got := h.app.Active().Body(); got != "a\r\nb\r\n" {
		t.Fatalf("pasted body %q", got)
	}
	h.clip.text = ""
	h.tt.Key(ui.Cmd|ui.Shift, ui.KeyC)
	if h.clip.text != "a\r\nb\r\n" {
		t.Fatalf("copied %q", h.clip.text)
	}
	if !h.tt.HasText("Copied") {
		t.Errorf("no Copied state: %q", h.tt.Texts())
	}
	h.tt.Advance(CopiedFor + time.Second)
	if h.tt.HasText("Copied") {
		t.Errorf("Copied stayed")
	}
	if !h.tt.HasText("Plain text · UTF-8 · CRLF") {
		t.Errorf("line ending not shown: %q", h.tt.Texts())
	}
	sns, _ := h.st.List(context.Background(), store.ListOptions{})
	if len(sns) != 1 || sns[0].Body != "a\r\nb\r\n" {
		t.Fatalf("stored %+v", sns)
	}
}

func TestCloseIsNotDeleteAndRestore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lastab.db")
	h := newHarness(t, path)
	h.tt.Type("first")
	h.settle()
	h.tt.Key(ui.Cmd, ui.KeyN)
	h.tt.Type("second")
	h.settle()
	// The newest is at the top of the sidebar, and active.
	if len(h.app.Tabs()) != 2 || h.app.active != 0 || h.app.Active().Body() != "second" {
		t.Fatalf("tabs %d active %d", len(h.app.Tabs()), h.app.active)
	}
	h.tt.Key(ui.Cmd, ui.KeyN)
	h.tt.Type("third")
	h.tt.Key(ui.Cmd, ui.KeyW) // closed before its first save
	h.settle()
	ctx := context.Background()
	if n, _ := h.st.Count(ctx); n != 3 {
		t.Fatalf("snippets %d", n)
	}
	open, _ := h.st.List(ctx, store.ListOptions{OpenOnly: true})
	if len(open) != 2 {
		t.Fatalf("open %d", len(open))
	}

	// A new window over the same database restores the tabs, the most
	// recently edited first.
	h2 := newHarness(t, path)
	tabs := h2.app.Tabs()
	if len(tabs) != 2 || tabs[0].Body() != "second" || tabs[1].Body() != "first" {
		var bodies []string
		for _, t := range tabs {
			bodies = append(bodies, t.Body())
		}
		t.Fatalf("restored %q", bodies)
	}
	if h2.app.Active().Body() != "second" {
		t.Errorf("active %q", h2.app.Active().Body())
	}
}

func TestSwitcher(t *testing.T) {
	h := newHarness(t, "")
	for i, s := range []string{"alpha", "beta", "gamma"} {
		if i > 0 {
			h.tt.Key(ui.Cmd, ui.KeyN)
		}
		h.tt.Type(s)
		h.settle()
		time.Sleep(5 * time.Millisecond) // distinct modified times
	}
	// gamma is current; beta was edited before it.
	h.tt.Key(ui.Cmd, ui.KeyBackquote)
	if !h.app.SwitcherOpen() {
		t.Fatal("switcher did not open")
	}
	vis := h.app.switcher.visible()
	if len(vis) != 3 || vis[0].Body() != "gamma" || vis[h.app.switcher.sel].Body() != "beta" {
		t.Fatalf("order/selection wrong: sel %d", h.app.switcher.sel)
	}
	h.tt.Key(0, ui.KeyTab)
	if vis[h.app.switcher.sel].Body() != "alpha" {
		t.Errorf("Tab did not move down")
	}
	h.tt.Key(ui.Shift, ui.KeyTab)
	h.tt.Key(0, ui.KeyEnter)
	if h.app.SwitcherOpen() || h.app.Active().Body() != "beta" {
		t.Fatalf("Enter: open %v active %q", h.app.SwitcherOpen(), h.app.Active().Body())
	}

	// Filtering, then Escape cancels.
	h.tt.Key(ui.Cmd, ui.KeyBackquote)
	h.tt.Type("GAM")
	vis = h.app.switcher.visible()
	if len(vis) != 1 || vis[0].Body() != "gamma" {
		t.Fatalf("filter: %d", len(vis))
	}
	h.tt.Key(0, ui.KeyEscape)
	if h.app.SwitcherOpen() || h.app.Active().Body() != "beta" {
		t.Errorf("Escape: open %v active %q", h.app.SwitcherOpen(), h.app.Active().Body())
	}
	if !strings.Contains(strings.Join(h.tt.Texts(), "|"), "beta") {
		t.Errorf("texts %q", h.tt.Texts())
	}
}

func TestSaveFailureStaysWithRetry(t *testing.T) {
	h := newHarness(t, "")
	h.tt.Type("x")
	h.settle()
	h.st.Close() // the disk goes away
	h.tt.Type("y")
	h.settle()
	if h.app.Active().saveErr == nil || !h.tt.HasText("Retry") {
		t.Fatalf("no failure shown: %q", h.tt.Texts())
	}
}
