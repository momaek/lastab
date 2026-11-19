package appui

import (
	"context"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/momaek/lastab/internal/store"
	"github.com/momaek/lastab/internal/todo"
)

// newTodo opens a to-do tab and types its items, one per Enter.
func newTodo(t *testing.T, items ...string) *harness {
	t.Helper()
	h := newHarness(t, "")
	h.tt.Key(ui.Cmd, ui.KeyT)
	h.tt.Frame()
	if h.app.Active().kind != store.KindTodo {
		t.Fatalf("Cmd+T opened %v", h.app.Active().kind)
	}
	if !h.tt.Focused("Add an item") {
		t.Fatalf("the add row is not focused")
	}
	for _, s := range items {
		h.tt.Type(s)
		h.tt.Key(0, ui.KeyEnter)
		h.tt.Frame()
	}
	return h
}

func (h *harness) body() string { return h.app.Active().Body() }

func (h *harness) wantBody(want string) {
	h.t.Helper()
	if got := h.body(); got != want {
		h.t.Fatalf("body\n got %q\nwant %q", got, want)
	}
}

func (h *harness) stored() string {
	h.t.Helper()
	sn, err := h.st.Get(context.Background(), h.app.Active().id)
	if err != nil {
		h.t.Fatal(err)
	}
	return sn.Body
}

// undo takes the toast's action.
func (h *harness) undo() {
	h.t.Helper()
	if err := h.tt.Click("Undo"); err != nil {
		h.t.Fatal(err)
	}
	h.tt.Frame()
}

// clickText clicks the text of the open item showing s, as a pointer would.
func (h *harness) clickText(s string) {
	h.t.Helper()
	r := h.rowOf(s)
	h.tt.ClickAt(r.X+64, r.Y+r.H/2)
	h.tt.Frame()
}

// focusItem clicks into an item's text.
func (h *harness) focusItem(text string) {
	h.t.Helper()
	l := h.app.Active().todo
	for _, it := range l.items {
		if it.text == text {
			l.focusOn(it.id, -1)
			h.tt.Frame()
			h.tt.Frame()
			return
		}
	}
	h.t.Fatalf("no item %q", text)
}

func TestTodoEnterAddsAndSaves(t *testing.T) {
	h := newTodo(t, "Buy milk", "Call Bob")
	h.wantBody("- [ ] Buy milk\n- [ ] Call Bob")
	if !h.tt.Focused("Add an item") {
		t.Errorf("focus left the add row")
	}
	// Enter in an item adds an empty one below it, focused.
	h.focusItem("Buy milk")
	h.tt.Key(0, ui.KeyEnter)
	h.tt.Type("Eggs")
	h.tt.Frame()
	h.wantBody("- [ ] Buy milk\n- [ ] Eggs\n- [ ] Call Bob")
	h.settle()
	if got := h.stored(); got != h.body() {
		t.Fatalf("stored %q", got)
	}
	if !h.tt.HasText("Buy milk · 0/3") {
		t.Errorf("title: %q", h.tt.Texts())
	}
	if !h.tt.HasText("0/3 done") {
		t.Errorf("progress: %q", h.tt.Texts())
	}
}

func TestTodoDraftIsKeptBeforeEnter(t *testing.T) {
	h := newTodo(t, "one")
	h.tt.Type("two, not entered")
	h.settle()
	h.wantBody("- [ ] one\n- [ ] two, not entered")
	if got := h.stored(); got != h.body() {
		t.Fatalf("stored %q", got)
	}
}

func TestTodoCmdEnterTogglesAndSinks(t *testing.T) {
	h := newTodo(t, "a", "b", "c")
	h.focusItem("a")
	h.tt.Key(ui.Cmd, ui.KeyEnter)
	// Struck through in place for a moment.
	h.wantBody("- [x] a\n- [ ] b\n- [ ] c")
	if !h.tt.Focused("Item text") {
		t.Errorf("focus left the item")
	}
	h.tt.Advance(SettleFor + 100*time.Millisecond)
	h.tt.Frame()
	h.wantBody("- [ ] b\n- [ ] c\n- [x] a")
	if !h.tt.HasText("Done 1") || !h.tt.HasText("1/3 done") {
		t.Errorf("texts %q", h.tt.Texts())
	}
	// Unchecking brings it back to the end of the open items.
	h.focusItem("a")
	h.tt.Key(ui.Cmd, ui.KeyEnter)
	h.tt.Frame()
	h.wantBody("- [ ] b\n- [ ] c\n- [ ] a")
}

func TestTodoCheckboxToggles(t *testing.T) {
	h := newTodo(t, "a", "b")
	if err := h.tt.Click("Done: a"); err != nil {
		t.Fatal(err)
	}
	h.tt.Frame()
	h.wantBody("- [x] a\n- [ ] b")
	h.tt.Advance(time.Second)
	h.tt.Frame()
	h.wantBody("- [ ] b\n- [x] a")
	// Clicking the text edits it, without toggling.
	h.clickText("b")
	h.wantBody("- [ ] b\n- [x] a")
	if !h.tt.Focused("Item text") {
		t.Errorf("clicking the text did not focus it")
	}
}

func TestTodoAltArrowsReorder(t *testing.T) {
	h := newTodo(t, "a", "b", "c")
	h.focusItem("c")
	h.tt.Key(ui.Alt, ui.KeyUp)
	h.tt.Frame()
	h.wantBody("- [ ] a\n- [ ] c\n- [ ] b")
	h.tt.Key(ui.Alt, ui.KeyUp)
	h.tt.Key(ui.Alt, ui.KeyUp) // at the top already
	h.tt.Frame()
	h.wantBody("- [ ] c\n- [ ] a\n- [ ] b")
	h.tt.Key(ui.Alt, ui.KeyDown)
	h.tt.Frame()
	h.wantBody("- [ ] a\n- [ ] c\n- [ ] b")
	// The focus stays with the item as it moves.
	h.tt.Type("!")
	h.tt.Frame()
	h.wantBody("- [ ] a\n- [ ] c!\n- [ ] b")
}

func TestTodoCmdShiftBackspaceDeletesWithUndo(t *testing.T) {
	h := newTodo(t, "a", "b", "c")
	h.focusItem("b")
	h.tt.Key(ui.Cmd|ui.Shift, ui.KeyBackspace)
	h.tt.Frame()
	h.wantBody("- [ ] a\n- [ ] c")
	if h.app.Toast() != "Deleted “b”" || !h.tt.HasText("Undo") {
		t.Fatalf("toast %q", h.app.Toast())
	}
	h.undo()
	h.wantBody("- [ ] a\n- [ ] b\n- [ ] c")
}

func TestTodoBackspaceOnEmptyDeletes(t *testing.T) {
	h := newTodo(t, "a", "b")
	h.focusItem("a")
	h.tt.Key(0, ui.KeyEnter) // an empty item below a
	h.tt.Frame()
	h.wantBody("- [ ] a\n- [ ] \n- [ ] b")
	h.tt.Key(0, ui.KeyBackspace)
	h.tt.Frame()
	h.wantBody("- [ ] a\n- [ ] b")
	if h.app.Toast() != "" {
		t.Errorf("toast %q", h.app.Toast())
	}
	// The focus went back to the end of a: Backspace now edits it.
	h.tt.Key(0, ui.KeyBackspace)
	h.tt.Frame()
	h.wantBody("- [ ] \n- [ ] b")
}

func TestTodoPasteSplitsLines(t *testing.T) {
	h := newTodo(t, "first")
	h.tt.SetClipboard("Back up the orders table\r\n\r\n- [ ] Check alert thresholds\n- [x] Prepare the rollback script\n")
	h.tt.Command("paste")
	h.tt.Frame()
	h.wantBody("- [ ] first\n- [ ] Back up the orders table\n- [ ] Check alert thresholds\n- [x] Prepare the rollback script")
	if h.app.Toast() != "Split 3 pasted lines into 3 items" {
		t.Fatalf("toast %q", h.app.Toast())
	}
	h.undo()
	h.wantBody("- [ ] first")

	// One line pastes as text.
	h.tt.SetClipboard("just text")
	h.tt.Command("paste")
	h.tt.Frame()
	h.wantBody("- [ ] first\n- [ ] just text")
	if h.app.Toast() == "Split 1 pasted lines into 1 items" {
		t.Errorf("one line split")
	}

	// Into an empty item, it takes the first line.
	h.tt.Key(0, ui.KeyEnter)
	h.focusItem("first")
	h.tt.Key(0, ui.KeyEnter)
	h.tt.SetClipboard("x\ny")
	h.tt.Command("paste")
	h.tt.Frame()
	h.wantBody("- [ ] first\n- [ ] x\n- [ ] y\n- [ ] just text")
}

func TestTodoClearCompletedWithUndo(t *testing.T) {
	h := newTodo(t, "a", "b", "c")
	h.tt.Click("Done: a")
	h.tt.Click("Done: c")
	h.tt.Advance(time.Second)
	h.tt.Frame()
	h.wantBody("- [ ] b\n- [x] a\n- [x] c")
	if err := h.tt.Click("Clear completed"); err != nil {
		t.Fatal(err)
	}
	h.tt.Frame()
	h.wantBody("- [ ] b")
	if h.app.Toast() != "Cleared 2 completed items" {
		t.Fatalf("toast %q", h.app.Toast())
	}
	// The toast offers the undo for five seconds.
	h.tt.Advance(UndoFor - 200*time.Millisecond)
	h.tt.Frame()
	if !h.tt.HasText("Undo") {
		t.Fatalf("toast gone early")
	}
	if got := h.stored(); got != "- [ ] b" {
		t.Fatalf("stored %q", got)
	}
	h.undo()
	h.wantBody("- [ ] b\n- [x] a\n- [x] c")
	if h.app.Toast() != "Restored" {
		t.Errorf("toast %q", h.app.Toast())
	}
	h.settle()
	if got := h.stored(); got != "- [ ] b\n- [x] a\n- [x] c" {
		t.Fatalf("stored %q", got)
	}
}

func TestTodoDoneGroupCollapses(t *testing.T) {
	h := newTodo(t, "a", "b")
	h.tt.Click("Done: a")
	h.tt.Advance(time.Second)
	h.tt.Frame()
	if !h.tt.HasText("a") {
		t.Fatal("done item not shown")
	}
	h.tt.Click("Done 1")
	h.tt.Frame()
	if _, ok := h.tt.Access("Done: a"); ok {
		t.Errorf("collapsed group still shows its items")
	}
	h.tt.Click("Done 1")
	h.tt.Frame()
	if _, ok := h.tt.Access("Done: a"); !ok {
		t.Errorf("expanded group does not show its items")
	}
}

// TestTodoBodyRoundTrips opens stored lists and checks that showing them,
// and a change undone, leaves their bodies exactly as they were.
func TestTodoBodyRoundTrips(t *testing.T) {
	files, _ := filepath.Glob("../todo/testdata/*.txt")
	bodies := []string{"- [ ] a\n- [x] b", "- [ ] only"}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		bodies = append(bodies, string(b))
	}
	for _, body := range bodies {
		path := filepath.Join(t.TempDir(), "lastab.db")
		h := newHarness(t, path)
		ctx := context.Background()
		sn, err := h.app.d.Service.Create(ctx, store.KindTodo, body, 0)
		if err != nil {
			t.Fatal(err)
		}
		h2 := newHarness(t, path)
		i := -1
		for j, tb := range h2.app.Tabs() {
			if tb.id == sn.ID {
				i = j
			}
		}
		if i < 0 {
			t.Fatalf("not restored")
		}
		h2.app.Activate(i)
		h2.settle()
		if got := h2.body(); got != body {
			t.Errorf("shown: body changed\n got %q\nwant %q", got, body)
		}
		got, _ := h2.st.Get(ctx, sn.ID)
		if got.Body != body {
			t.Errorf("stored body changed: %q", got.Body)
		}
		// A canonical list writes back exactly as it was.
		if want := todo.Format(todo.Parse(body)); want == body {
			l := h2.app.Active().todo
			l.items[0].text += "x"
			h2.app.writeTodo(h2.app.Active())
			l.items[0].text = strings.TrimSuffix(l.items[0].text, "x")
			h2.app.writeTodo(h2.app.Active())
			if got := h2.body(); got != body {
				t.Errorf("rewritten\n got %q\nwant %q", got, body)
			}
		}
	}
}

func TestTodoRestoredVersionReloads(t *testing.T) {
	h := newTodo(t, "a")
	h.settle()
	h.app.Active().buf().Set("- [ ] z\n- [x] y")
	h.tt.Frame()
	items := h.app.TodoItems(h.app.Active())
	if len(items) != 2 || items[0].Text != "z" || !items[1].Done {
		t.Fatalf("items %+v", items)
	}
	h.wantBody("- [ ] z\n- [x] y")
}

func TestTodoCopyAllIsPlainText(t *testing.T) {
	h := newTodo(t, "a", "b")
	h.tt.Click("Done: b")
	h.tt.Key(ui.Cmd|ui.Shift, ui.KeyC)
	if h.clip.text != "- [ ] a\n- [x] b" {
		t.Fatalf("copied %q", h.clip.text)
	}
}

// rowOf returns where the open item showing text was laid out.
func (h *harness) rowOf(text string) ui.Rect {
	h.t.Helper()
	l := h.app.Active().todo
	for _, it := range l.items {
		if it.text == text {
			if r, ok := l.rowBox[it.id]; ok {
				return r
			}
		}
	}
	h.t.Fatalf("no open row %q", text)
	return ui.Rect{}
}

// dragGrip presses the grip of the item showing text and moves the
// pointer to y, a few DIPs a frame, without letting go.
func (h *harness) dragGrip(text string, y float32) (x float32) {
	h.t.Helper()
	r := h.rowOf(text)
	x, from := r.X+8, r.Y+r.H/2
	h.tt.Move(x, from)
	h.tt.Frame()
	h.tt.Press(x, from)
	h.tt.Frame()
	step := float32(6)
	if y < from {
		step = -step
	}
	for at := from; (step < 0 && at > y) || (step > 0 && at < y); at += step {
		h.tt.Move(x, at)
		h.tt.Frame()
	}
	h.tt.Move(x, y)
	h.tt.Frame()
	return x
}

func TestTodoDragReorders(t *testing.T) {
	h := newTodo(t, "a", "b", "c")
	ra := h.rowOf("a")
	x := h.dragGrip("c", ra.Y+4)
	// Lifted: it follows the pointer over the others, which part around
	// the line where it would drop, before a.
	d := h.app.Active().todo.drag
	if d == nil || !d.active || d.to != 0 {
		t.Fatalf("drag %+v", d)
	}
	h.wantBody("- [ ] a\n- [ ] b\n- [ ] c") // nothing moved yet
	h.tt.Release(x, ra.Y+4)
	h.tt.Frame()
	h.tt.Frame()
	h.wantBody("- [ ] c\n- [ ] a\n- [ ] b")
	if h.app.Active().todo.drag != nil {
		t.Errorf("drag left over")
	}

	// Down to the end.
	rb := h.rowOf("b")
	x = h.dragGrip("c", rb.Y+rb.H+20)
	if d := h.app.Active().todo.drag; d == nil || d.to != 2 {
		t.Fatalf("drag %+v", d)
	}
	h.tt.Release(x, rb.Y+rb.H+20)
	h.tt.Frame()
	h.tt.Frame()
	h.wantBody("- [ ] a\n- [ ] b\n- [ ] c")
}

func TestTodoDragEscapeCancels(t *testing.T) {
	h := newTodo(t, "a", "b", "c")
	ra := h.rowOf("a")
	x := h.dragGrip("c", ra.Y+4)
	h.tt.Key(0, ui.KeyEscape)
	h.tt.Frame()
	if d := h.app.Active().todo.drag; d == nil || d.active {
		t.Fatalf("Escape did not put it back: %+v", d)
	}
	h.tt.Release(x, ra.Y+4)
	h.tt.Frame()
	h.tt.Frame()
	h.wantBody("- [ ] a\n- [ ] b\n- [ ] c")
}

func TestTodoGripClickDoesNotMove(t *testing.T) {
	h := newTodo(t, "a", "b")
	r := h.rowOf("b")
	h.tt.ClickAt(r.X+8, r.Y+r.H/2)
	h.tt.Frame()
	h.tt.Frame()
	h.wantBody("- [ ] a\n- [ ] b")
	if h.app.Active().todo.drag != nil {
		t.Errorf("drag left over")
	}
}

// TestTodoSnapshots writes PNGs of a to-do list, as the design draws it,
// when LASTAB_SNAPSHOT_DIR is set.
func TestTodoSnapshots(t *testing.T) {
	dir := os.Getenv("LASTAB_SNAPSHOT_DIR")
	if dir == "" {
		t.Skip("LASTAB_SNAPSHOT_DIR not set")
	}
	save := func(name string, tt *ui.Tester) {
		f, err := os.Create(filepath.Join(dir, name+".png"))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		png.Encode(f, tt.Image())
	}
	for _, dark := range []bool{false, true} {
		name := "light"
		if dark {
			name = "dark"
		}
		h := newHarness(t, "")
		h.tt.SetDark(dark)
		h.tt.SetSize(1200, 760)
		h.tt.Type("curl -X POST 'https://api.staging.example.com/v2/orders/search'")
		h.settle()
		h.tt.Key(ui.Cmd, ui.KeyT)
		h.tt.Frame()
		h.app.Active().buf().Set("- [ ] Confirm staging-b regression passed\n- [x] Merge order-api release branch\n" +
			"- [ ] Make refund export async, 10% rollout\n- [x] Remove feature flag: refund_v1\n" +
			"- [x] Run kubectl rollout status\n- [ ] Update the order-api CHANGELOG\n" +
			"- [ ] Tell support the release window: Wed 22:00")
		h.settle()
		r := h.rowOf("Confirm staging-b regression passed")
		h.tt.Move(r.X+r.W/2, r.Y+r.H/2)
		h.tt.Frame()
		save(name+"-todo", h.tt)

		// Dragging the CHANGELOG item up between the first two.
		first := h.rowOf("Confirm staging-b regression passed")
		x := h.dragGrip("Update the order-api CHANGELOG", first.Y+first.H-6)
		h.tt.Advance(time.Second) // the rows have parted
		h.tt.Frame()
		save(name+"-todo-dragging", h.tt)
		h.tt.Key(0, ui.KeyEscape)
		h.tt.Release(x, first.Y+first.H-6)
		h.tt.Frame()

		h.app.Active().todo.focusOn(draftID, -1)
		h.tt.Frame()
		h.tt.SetClipboard("Back up the orders table\nCheck alert thresholds\nPrepare the rollback script")
		h.tt.Command("paste")
		h.tt.Frame()
		h.tt.Frame()
		save(name+"-todo-pasted", h.tt)
	}
}

func TestTodoDragOnTextSelects(t *testing.T) {
	h := newTodo(t, "alpha", "beta")
	ra, _ := h.tt.Find("alpha")
	rb, _ := h.tt.Find("beta")
	x := rb.X + 80
	h.tt.Press(x, rb.Y+rb.H/2)
	for y := rb.Y + rb.H/2; y > ra.Y+4; y -= 6 {
		h.tt.Move(x, y)
		h.tt.Frame()
	}
	h.tt.Release(x, ra.Y+4)
	h.tt.Frame()
	h.wantBody("- [ ] alpha\n- [ ] beta")
}

func TestTodoArrowsMoveFocus(t *testing.T) {
	h := newTodo(t, "a", "b")
	h.tt.Key(0, ui.KeyUp) // from the add row to b
	h.tt.Frame()
	h.tt.Key(0, ui.KeyUp)
	h.tt.Frame()
	h.tt.Type("1")
	h.tt.Frame()
	h.wantBody("- [ ] a1\n- [ ] b")
	h.tt.Key(0, ui.KeyDown)
	h.tt.Key(0, ui.KeyDown)
	h.tt.Frame()
	h.tt.Type("c")
	h.tt.Frame()
	h.wantBody("- [ ] a1\n- [ ] b\n- [ ] c")
	if !h.tt.Focused("Add an item") {
		t.Errorf("Down did not reach the add row")
	}
}
