package appui

import (
	"math"
	"runtime"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"

	"github.com/momaek/lastab/internal/theme"
	"github.com/momaek/lastab/internal/todo"
	"github.com/momaek/lastab/internal/ui/keys"
	"github.com/momaek/lastab/locales"
)

// SettleFor is how long a checked item stays struck through in place
// before it sinks to the Done group.
const SettleFor = 900 * time.Millisecond

// draftID is the focus target of the row that adds an item.
const draftID = -1

// todoItem is one row of a to-do list.
type todoItem struct {
	id   int
	text string
	done bool
	// checkedAt is when the user checked it: the item stays in place,
	// struck through, until SettleFor after. Zero for items read checked.
	checkedAt time.Time
	// fresh items came from a paste; they show highlighted for a moment.
	fresh bool
}

// todoList is the state of a to-do tab's view. The tab's buffer stays the
// body: the list is read from it, and every change is written back to it
// as todo.Format's plain text, so saving, versions, Copy all, search and
// titles work on to-dos as on any snippet.
type todoList struct {
	items  []*todoItem
	draft  string // the row that adds an item; part of the body once not blank
	nextID int

	// out is the body the list was last read from or written as, as
	// todo.Format writes it; ver is the buffer version that matches it.
	out string
	ver uint64

	hideDone   bool
	freshUntil time.Time
	// focus asks the next frame to focus the row of this item (draftID for
	// the add row), with the caret at caret runes, -1 for the end.
	focus int
	caret int

	drag    *todoDrag
	grips   map[int]ui.Element // this frame's grip of each item, by id
	rowBox  map[int]ui.Rect    // where each open row was laid out, by id
	pending *rowMove           // a drag dropped this frame, moved as the next one starts
}

// todoDrag is an open item being dragged by its grip. Once the pointer
// moved a few DIPs the item lifts: it follows the pointer, smaller and
// tilted, over the others, which part around a line where it would drop.
type todoDrag struct {
	id     int
	pressY float32 // the pointer as the press began, in the column
	grab   float32 // the pointer's offset into the row as it was pressed
	h      float32 // the row's height
	y      float32 // the lifted row's top, in the column
	to     int     // the other open items it would go before
	active bool    // moved far enough to lift
	cancel bool    // Escape put it back; it waits for the release
}

// Drag motion, after the design.
const (
	dragSlop    = 4    // DIPs the pointer moves before the item lifts
	dragScale   = 0.9  // the lifted row's size
	dragTilt    = -0.6 // its tilt, in degrees
	dragPart    = 8    // how far the rows next to the line move aside
	dragFalloff = 0.7  // and those further, each by this much less
)

type rowMove struct {
	rows []int
	to   int
}

// todoSnap is a list as it was, for undo.
type todoSnap struct {
	items []todoItem
	draft string
}

func (l *todoList) snap() todoSnap {
	s := todoSnap{draft: l.draft}
	for _, it := range l.items {
		s.items = append(s.items, *it)
	}
	return s
}

func (l *todoList) restore(s todoSnap) {
	l.items = l.items[:0]
	for _, it := range s.items {
		it := it
		it.fresh = false
		l.items = append(l.items, &it)
	}
	l.draft = s.draft
}

func (l *todoList) newItem(text string, done bool) *todoItem {
	l.nextID++
	return &todoItem{id: l.nextID, text: text, done: done}
}

// read loads the list from a body.
func (l *todoList) read(body string, ver uint64) {
	l.items = l.items[:0]
	for _, it := range todo.Parse(body) {
		l.items = append(l.items, l.newItem(it.Text, it.Done))
	}
	l.draft = ""
	l.normalize(time.Time{})
	l.out, l.ver = l.format(), ver
}

// format writes the list as its body: the open items, the draft once it is
// not blank, then the done ones.
func (l *todoList) format() string {
	items := make([]todo.Item, 0, len(l.items)+1)
	k := l.openCount()
	for _, it := range l.items[:k] {
		items = append(items, todo.Item{Text: it.text, Done: it.done})
	}
	if strings.TrimSpace(l.draft) != "" {
		items = append(items, todo.Item{Text: l.draft})
	}
	for _, it := range l.items[k:] {
		items = append(items, todo.Item{Text: it.text, Done: it.done})
	}
	return todo.Format(items)
}

// settling reports whether a checked item still shows in place.
func (it *todoItem) settling(now time.Time) bool {
	return it.done && !it.checkedAt.IsZero() && now.Before(it.checkedAt.Add(SettleFor))
}

func (it *todoItem) open(now time.Time) bool { return !it.done || it.settling(now) }

// normalize puts the open items first and the done ones after, each in
// their order, and reports whether anything moved. Items settling count
// as open until they sink.
func (l *todoList) normalize(now time.Time) bool {
	var open, done []*todoItem
	moved, sawDone := false, false
	for _, it := range l.items {
		if it.open(now) {
			open = append(open, it)
			moved = moved || sawDone
		} else {
			if !it.checkedAt.IsZero() {
				it.checkedAt = time.Time{}
			}
			done = append(done, it)
			sawDone = true
		}
	}
	if moved {
		l.items = append(open, done...)
	}
	return moved
}

// openCount returns the number of leading open items: the open group.
func (l *todoList) openCount() int {
	for i, it := range l.items {
		if it.done && it.checkedAt.IsZero() {
			return i
		}
	}
	return len(l.items)
}

func (l *todoList) index(id int) int {
	for i, it := range l.items {
		if it.id == id {
			return i
		}
	}
	return -1
}

func (l *todoList) focusOn(id, caret int) { l.focus, l.caret = id, caret }

// todoOf returns the list of a to-do tab, read from its buffer again when
// the buffer changed from elsewhere, as a version restored.
func (a *App) todoOf(t *Tab) *todoList {
	if t.todo == nil {
		t.todo = &todoList{}
		t.todo.read(t.Body(), t.buf().Version())
	} else if v := t.buf().Version(); v != t.todo.ver {
		t.todo.read(t.Body(), v)
	}
	return t.todo
}

// writeTodo writes the list into the tab's buffer, if it changed: the
// keeping of the tab takes it from there.
func (a *App) writeTodo(t *Tab) {
	l := t.todo
	if l == nil {
		return
	}
	out := l.format()
	if out == l.out {
		return
	}
	t.buf().Set(out)
	l.out, l.ver = out, t.buf().Version()
}

// TodoItems returns the items of a to-do tab as they show, for tests.
func (a *App) TodoItems(t *Tab) []todo.Item {
	l := a.todoOf(t)
	var items []todo.Item
	for _, it := range l.items {
		items = append(items, todo.Item{Text: it.text, Done: it.done})
	}
	return items
}

// Actions. Each changes the list and writes the body at once; the keys
// run them between frames, the buttons through OnClick, never while the
// rows are being built.

func (a *App) todoInsertAfter(t *Tab, id int) {
	l := t.todo
	i := l.index(id)
	at := l.openCount()
	if i >= 0 && i < at {
		at = i + 1
	}
	it := l.newItem("", false)
	l.items = append(l.items[:at], append([]*todoItem{it}, l.items[at:]...)...)
	l.focusOn(it.id, 0)
	a.writeTodo(t)
}

// todoCommitDraft makes the draft an item, at the end of the open group.
func (a *App) todoCommitDraft(t *Tab) bool {
	l := t.todo
	if strings.TrimSpace(l.draft) == "" {
		return false
	}
	it := l.newItem(l.draft, false)
	at := l.openCount()
	l.items = append(l.items[:at], append([]*todoItem{it}, l.items[at:]...)...)
	l.draft = ""
	l.focusOn(draftID, 0)
	a.writeTodo(t)
	return true
}

func (a *App) todoToggle(t *Tab, id int, now time.Time) {
	l := t.todo
	i := l.index(id)
	if i < 0 {
		return
	}
	it := l.items[i]
	it.done = !it.done
	a.todoChecked(t, it, now)
}

// todoChecked follows an item checked or unchecked: checked, it stays
// struck through in place for a moment; unchecked, it goes back up to the
// end of the open group as the next frame starts.
func (a *App) todoChecked(t *Tab, it *todoItem, now time.Time) {
	if it.done {
		it.checkedAt = now
	} else {
		it.checkedAt = time.Time{}
	}
	a.writeTodo(t)
}

// todoMove moves an item up (-1) or down (1) within its group.
func (a *App) todoMove(t *Tab, id, by int) {
	l := t.todo
	i := l.index(id)
	if i < 0 {
		return
	}
	k := l.openCount()
	lo, hi := 0, k
	if i >= k {
		lo, hi = k, len(l.items)
	}
	j := i + by
	if j < lo || j >= hi {
		return
	}
	l.items[i], l.items[j] = l.items[j], l.items[i]
	l.focusOn(id, -2) // keep the caret where it is
	a.writeTodo(t)
}

// todoDelete deletes an item. With undo, a toast offers to bring it back;
// the focus goes to the next item, or the one before.
func (a *App) todoDelete(t *Tab, id int, undo bool, now time.Time) {
	l := t.todo
	i := l.index(id)
	if i < 0 {
		return
	}
	before := l.snap()
	it := l.items[i]
	l.items = append(l.items[:i], l.items[i+1:]...)
	switch {
	case !undo && i > 0:
		// Backspace on an empty item: back to the end of the one before.
		l.focusOn(l.items[i-1].id, -1)
	case i < len(l.items):
		l.focusOn(l.items[i].id, -1)
	case i > 0:
		l.focusOn(l.items[i-1].id, -1)
	default:
		l.focusOn(draftID, -1)
	}
	a.writeTodo(t)
	if undo {
		a.showToast(locales.TodoDeleted(it.text), UndoFor, locales.Undo(), func() { a.todoUndo(t, before) }, now)
	}
}

// TodoClearDone deletes the done items, with a toast to undo it.
func (a *App) TodoClearDone(t *Tab, now time.Time) {
	l := a.todoOf(t)
	before := l.snap()
	n := 0
	kept := l.items[:0]
	for _, it := range l.items {
		if it.done {
			n++
			continue
		}
		kept = append(kept, it)
	}
	if n == 0 {
		return
	}
	l.items = kept
	if l.index(l.focus) < 0 {
		l.focusOn(draftID, -1)
	}
	a.writeTodo(t)
	a.showToast(locales.TodoCleared(n), UndoFor, locales.Undo(), func() { a.todoUndo(t, before) }, now)
}

// todoPaste splits pasted lines into items, after item id (draftID: after
// the open items). One line pastes as text does.
func (a *App) todoPaste(t *Tab, id int, text string, now time.Time) bool {
	pasted := todo.SplitPaste(text)
	if len(pasted) < 2 {
		return false
	}
	l := t.todo
	before := l.snap()
	if id == draftID {
		a.todoCommitDraft(t)
	}
	at := l.openCount()
	if i := l.index(id); i >= 0 {
		at = i + 1
		if strings.TrimSpace(l.items[i].text) == "" {
			// Into an empty item: it takes the first line.
			l.items = append(l.items[:i], l.items[i+1:]...)
			at = i
		}
	}
	var add []*todoItem
	for _, p := range pasted {
		it := l.newItem(p.Text, p.Done)
		it.fresh = true
		add = append(add, it)
	}
	for _, it := range l.items {
		if it.fresh && !containsItem(add, it) {
			it.fresh = false
		}
	}
	l.items = append(l.items[:at], append(add, l.items[at:]...)...)
	l.normalize(now)
	l.freshUntil = now.Add(UndoFor)
	if id == draftID {
		l.focusOn(draftID, -1)
	} else {
		l.focusOn(add[len(add)-1].id, -1)
	}
	a.writeTodo(t)
	a.showToast(locales.TodoPasted(len(pasted)), UndoFor, locales.Undo(), func() { a.todoUndo(t, before) }, now)
	return true
}

func containsItem(items []*todoItem, it *todoItem) bool {
	for _, x := range items {
		if x == it {
			return true
		}
	}
	return false
}

func (a *App) todoUndo(t *Tab, s todoSnap) {
	l := a.todoOf(t)
	l.restore(s)
	l.normalize(a.frameNow)
	l.freshUntil = time.Time{}
	a.writeTodo(t)
	a.showToast(locales.TodoUndone(), 3*time.Second, "", nil, a.frameNow)
}

// todoReorder moves the open rows a drag dropped before row to.
func (l *todoList) reorder(rows []int, to int) {
	k := l.openCount()
	var moved, rest []*todoItem
	in := map[int]bool{}
	for _, r := range rows {
		if r >= 0 && r < k {
			in[r] = true
			moved = append(moved, l.items[r])
		}
	}
	if len(moved) == 0 {
		return
	}
	at := 0
	for i, it := range l.items[:k] {
		if i == to {
			at = len(rest)
		}
		if !in[i] {
			rest = append(rest, it)
		}
	}
	if to >= k {
		at = len(rest)
	}
	out := append(append(append([]*todoItem{}, rest[:at]...), moved...), rest[at:]...)
	l.items = append(out, l.items[k:]...)
}

// itemKey handles the keys of an item's input. It runs between frames.
func (a *App) itemKey(t *Tab, id int, k ui.TextKey) bool {
	l := t.todo
	now := a.frameNow
	switch {
	case k.Key == ui.KeyEnter && k.Mods == 0:
		a.todoInsertAfter(t, id)
	case k.Key == ui.KeyEnter && k.Mods == ui.Cmd:
		a.todoToggle(t, id, now)
		l.focusOn(id, -2)
	case k.Key == ui.KeyUp && k.Mods == ui.Alt:
		a.todoMove(t, id, -1)
	case k.Key == ui.KeyDown && k.Mods == ui.Alt:
		a.todoMove(t, id, 1)
	case k.Key == ui.KeyBackspace && k.Mods == ui.Cmd|ui.Shift:
		a.todoDelete(t, id, true, now)
	case k.Key == ui.KeyBackspace && k.Mods == 0 && k.Text() == "":
		a.todoDelete(t, id, false, now)
	case k.Key == ui.KeyUp && k.Mods == 0:
		if i := l.index(id); i > 0 {
			l.focusOn(l.items[i-1].id, -1)
		}
	case k.Key == ui.KeyDown && k.Mods == 0:
		i := l.index(id)
		switch {
		case i < 0:
		case i+1 == l.openCount():
			l.focusOn(draftID, -1)
		case i+1 < len(l.items):
			l.focusOn(l.items[i+1].id, -1)
		}
	default:
		return false
	}
	return true
}

// draftKey handles the keys of the row that adds an item.
func (a *App) draftKey(t *Tab, k ui.TextKey) bool {
	l := t.todo
	switch {
	case k.Key == ui.KeyEnter && k.Mods == 0:
		l.draft = k.Text()
		a.todoCommitDraft(t)
		k.SetText("")
	case k.Key == ui.KeyBackspace && k.Mods == 0 && k.Text() == "":
		if n := l.openCount(); n > 0 {
			l.focusOn(l.items[n-1].id, -1)
		}
	case k.Key == ui.KeyUp && k.Mods == 0:
		if n := l.openCount(); n > 0 {
			l.focusOn(l.items[n-1].id, -1)
		}
	case k.Key == ui.KeyDown && k.Mods == 0:
		if n := l.openCount(); n < len(l.items) && !l.hideDone {
			l.focusOn(l.items[n].id, -1)
		}
	default:
		return false
	}
	return true
}

// todoView is the body of a to-do tab: its open items, the row that adds
// one, then the Done group.
func (a *App) todoView(c *ui.Context, t *Tab) {
	p := a.pal
	now := c.Now()
	l := a.todoOf(t)
	if m := l.pending; m != nil {
		l.pending = nil
		l.reorder(m.rows, m.to)
	}
	if l.normalize(now) {
		a.writeTodo(t)
	}
	for _, it := range l.items {
		if it.settling(now) {
			c.After(it.checkedAt.Add(SettleFor).Sub(now))
		}
	}
	fresh := now.Before(l.freshUntil)
	if fresh {
		c.After(l.freshUntil.Sub(now))
	}
	if a.focusEditor && !a.switcher.open {
		l.focusOn(draftID, -1)
		a.focusEditor = false
	}
	k := l.openCount()
	open := l.items[:k]
	done := l.items[k:]

	ui.Scroll(c.Key(t.key)).Column().Grow(1).Shrink(1).MinHeight(0).FillWidth().AlignItems(ui.Stretch).
		Padding(16, 48, 24, 24).Role(ui.RoleTabPanel).Label(tabTitle(t)).Children(func() {
		a.todoOpenRows(c, t, open, fresh)
		a.todoDraftRow(c, t)
		if len(done) == 0 {
			return
		}
		ui.Row(c.Key("doneHead")).Gap(8).AlignItems(ui.Center).Padding(16, 0, 4, 24).Children(func() {
			chev := "▾"
			if l.hideDone {
				chev = "▸"
			}
			label := locales.TodoDoneGroup(len(done))
			b := ui.ButtonBase(c.Key("doneToggle")).Row().Gap(8).AlignItems(ui.Center).Height(28).Padding(0, 8, 0, 0).
				Label(label).Expanded(!l.hideDone)
			b.Children(func() {
				ui.Text(c, chev).FontSize(12).TextColor(p.Muted)
				ui.Text(c, label).FontSize(12.5).FontWeight(500).TextColor(p.Muted).FontFeatures("tnum")
			})
			b.OnClick(func() { l.hideDone = !l.hideDone })
			ui.Spacer(c)
			clear := ui.ButtonBase(c.Key("clearDone")).Height(28).Padding(0, 8).Radius(8).Label(locales.TodoClearDone())
			if clear.Hovered() {
				clear.Background(p.Chrome)
			}
			clear.Children(func() {
				ui.Text(c, locales.TodoClearDone()).FontSize(12).TextColor(p.Muted)
			})
			clear.OnClick(func() { a.TodoClearDone(t, a.frameNow) })
		})
		if l.hideDone {
			return
		}
		for _, it := range done {
			a.todoRow(c, t, it, rowDone, false)
		}
	})
	a.writeTodo(t)
}

// todoOpenRows builds the open items. While one is dragged, it is built
// last, lifted over the others where the pointer holds it, and the others
// part around a line where it would drop.
func (a *App) todoOpenRows(c *ui.Context, t *Tab, open []*todoItem, fresh bool) {
	p := a.pal
	l := t.todo
	col := ui.Column(c.Key("open")).AlignItems(ui.Stretch).Role(ui.RoleList).Label(locales.TodoListLabel())
	colBox := col.LayoutBounds()
	_, py, _ := col.PointerPosition()
	d := l.drag
	if d != nil && l.index(d.id) < 0 {
		l.drag, d = nil, nil // deleted while held
	}
	var lifted *todoItem
	var others []*todoItem
	for _, it := range open {
		if d != nil && d.active && it.id == d.id {
			lifted = it
			continue
		}
		others = append(others, it)
	}
	if lifted != nil {
		d.y = py - d.grab
		// It goes before the first other row whose middle is below its own.
		mid := colBox.Y + d.y + d.h/2
		d.to = len(others)
		for j, it := range others {
			if b := l.rowBox[it.id]; mid < b.Y+b.H/2 {
				d.to = j
				break
			}
		}
		c.OnWindowShortcut(0, ui.KeyEscape, func() { d.active, d.cancel = false, true })
	}
	if l.rowBox == nil {
		l.rowBox = map[int]ui.Rect{}
	}
	col.Children(func() {
		for j, it := range others {
			var shift float32
			if lifted != nil {
				if j == d.to {
					a.todoDropLine(c)
				}
				// The rows next to the line move aside, those further less.
				if j < d.to {
					shift = -dragPart * float32(math.Pow(dragFalloff, float64(d.to-1-j)))
				} else {
					shift = dragPart * float32(math.Pow(dragFalloff, float64(j-d.to)))
				}
			}
			row := a.todoRow(c, t, it, rowOpen, fresh)
			row.Translate(0, row.AnimateTo("part", shift, ui.Smooth))
			l.rowBox[it.id] = row.LayoutBounds()
			a.todoGrip(c, t, it, row, colBox, py)
		}
		if lifted == nil {
			return
		}
		if d.to == len(others) {
			a.todoDropLine(c)
		}
		row := a.todoRow(c, t, lifted, rowLifted, false)
		row.Absolute().Top(d.y).Left(64).Width(560).ZIndex(1).
			Padding(0, 12).Radius(8).Background(p.Panel).Border(1, p.BtnLine).
			Shadow(0, 8, 24, 0, a.popShadow()).
			TransformOrigin(0, 0.5).Scale(dragScale, dragScale).Rotate(dragTilt).Cursor(ui.CursorGrabbing)
		a.todoGrip(c, t, lifted, row, colBox, py)
	})
}

// popShadow is the shadow of what floats over the page, as the design's
// pop shadow: 14% black in light, 45% in dark.
func (a *App) popShadow() ui.Color {
	if a.pal == &theme.Dark {
		return ui.RGBA(0, 0, 0, 0.45)
	}
	return ui.RGBA(0, 0, 0, 0.14)
}

// todoDropLine is where the lifted item would drop, under the text.
func (a *App) todoDropLine(c *ui.Context) {
	ui.Box(c.Key("dropLine")).Height(2).Margin(0, 0, 0, 48).Radius(1).Background(a.pal.DropLine).AccessHidden(true)
}

// todoGrip follows the press on an item's grip: it lifts the item once
// the pointer moved, and drops it where the line shows as it lets go.
func (a *App) todoGrip(c *ui.Context, t *Tab, it *todoItem, row ui.Element, colBox ui.Rect, py float32) {
	l := t.todo
	grip := l.grips[it.id]
	if !grip.Valid() {
		return
	}
	d := l.drag
	held := gripHeld(grip)
	switch {
	case held && d == nil:
		b := row.LayoutBounds()
		l.drag = &todoDrag{id: it.id, pressY: py, grab: py - (b.Y - colBox.Y), h: b.H, to: -1}
	case held && d.id == it.id:
		if !d.active && !d.cancel && abs32(py-d.pressY) > dragSlop {
			d.active = true
			c.Invalidate() // lift it now
		}
	case !held && d != nil && d.id == it.id:
		// Let go: it drops where the line is, as the next frame starts.
		if d.active && d.to >= 0 {
			from := l.index(it.id)
			to := d.to
			if to >= from {
				to++ // counted among the others, without it
			}
			l.pending = &rowMove{[]int{from}, to}
		}
		l.drag = nil
		c.Invalidate()
	}
}

// gripHeld reports whether the primary button holds the grip, wherever
// the pointer went since it pressed it (Pressed is false off the grip).
func gripHeld(grip ui.Element) bool {
	grip.Pressed() // the grip takes the press
	b, ok := grip.PressedButton()
	return ok && b == 0
}

func abs32(x float32) float32 {
	if x < 0 {
		return -x
	}
	return x
}

// How an item shows.
type rowKind int

const (
	rowOpen   rowKind = iota // in the open group
	rowLifted                // dragged over the open group
	rowDone                  // in the Done group
)

// todoRow builds an item: its grip, its check box, its text and, under the
// pointer, the button that deletes it.
func (a *App) todoRow(c *ui.Context, t *Tab, it *todoItem, kind rowKind, fresh bool) ui.Element {
	p := a.pal
	l := t.todo
	now := c.Now()
	dragging := l.drag != nil && l.drag.active
	row := ui.Row(c.Key(it.id)).MinHeight(36).Gap(8).AlignItems(ui.Center)
	if kind == rowOpen {
		row.BorderWidth(0, 0, 1, 0).BorderColor(p.LineSoft)
		if fresh && it.fresh {
			row.Background(a.acc.Sel)
		}
	}
	hovered := row.Hovered() && !dragging
	row.Children(func() {
		grip := ui.Box(c.Key("grip")).Width(16).Height(28).AccessHidden(true)
		if kind != rowDone {
			grip.Cursor(ui.CursorGrab)
			if l.grips == nil {
				l.grips = map[int]ui.Element{}
			}
			l.grips[it.id] = grip
		}
		if kind == rowDone || !hovered && kind != rowLifted && !gripHeld(grip) {
			grip.Opacity(0)
		}
		grip.Children(func() {
			ui.Text(c, "⠿").FontSize(12).TextColor(p.Faint).Fill().TextAlign(ui.Center).Padding(7, 0, 0, 0)
		})
		done := it.done
		cb := ui.Checkbox(c.Key("check"), &done, "").Label(locales.TodoCheckLabel(it.text)).Margin(0, 4, 0, 0)
		if cb.Changed() {
			// The box changed its copy. The item is checked in place; it
			// moves between the groups as a later frame starts, never
			// while the rows are built.
			a.todoCheckLater(t, it.id, done, now)
			c.Invalidate()
		}
		in := ui.TextInput(c.Key("text"), &it.text).Grow(1).Shrink(1).MinWidth(0).Height(32).
			Label(locales.TodoItemLabel()).FontSize(14).Background(ui.Transparent).Border(0, ui.Transparent).Padding(0)
		if it.done {
			in.TextColor(p.Muted).Strikethrough()
		} else {
			in.TextColor(p.Text)
		}
		id := it.id
		in.OnTextKey(func(k ui.TextKey) bool { return a.itemKey(t, id, k) })
		in.OnPaste(func(text string) bool { return a.todoPaste(t, id, text, a.frameNow) })
		if l.focus == id {
			in.Focus()
			switch {
			case l.caret == -1:
				n := utf8.RuneCountInString(it.text)
				in.SetTextSelection(n, n)
			case l.caret >= 0:
				in.SetTextSelection(l.caret, l.caret)
			}
			l.focus = 0
		}
		if in.Changed() && it.fresh {
			it.fresh = false
		}
		if kind == rowLifted {
			return
		}
		del := ui.ButtonBase(c.Key("delete")).Row().AlignItems(ui.Center).Justify(ui.Center).Size(28, 28).Radius(8).Label(locales.TodoDeleteItem()).
			Tooltip(locales.TodoDeleteItem() + "  " + todoKey("CmdOrCtrl+Shift+Backspace"))
		if !hovered {
			del.Opacity(0)
		} else if del.Hovered() {
			del.Background(p.Chrome)
		}
		del.Children(func() {
			ui.Text(c, "✕").FontSize(12).TextColor(p.Muted)
		})
		del.OnClick(func() { a.todoDelete(t, id, true, a.frameNow) })
	})
	return row
}

// todoCheckLater records a click on a check box for the next frame.
func (a *App) todoCheckLater(t *Tab, id int, done bool, now time.Time) {
	l := t.todo
	if i := l.index(id); i >= 0 {
		it := l.items[i]
		it.done = done
		a.todoChecked(t, it, now)
	}
}

// todoDraftRow is the row that adds an item: a dashed box and an input.
func (a *App) todoDraftRow(c *ui.Context, t *Tab) {
	p := a.pal
	l := t.todo
	ui.Row(c.Key("draft")).MinHeight(36).Gap(8).AlignItems(ui.Center).Padding(0, 0, 0, 24).Children(func() {
		ui.Box(c).Size(14, 14).Margin(1, 5, 1, 1).Radius(4).Border(1.5, p.Dashed).BorderStyle(ui.BorderDashed).AccessHidden(true)
		in := ui.TextInput(c.Key("draftInput"), &l.draft).Grow(1).Shrink(1).MinWidth(0).Height(32).
			Label(locales.TodoAddLabel()).Placeholder(locales.TodoAddPlaceholder()).
			FontSize(14).TextColor(p.Text).Background(ui.Transparent).Border(0, ui.Transparent).Padding(0)
		in.OnTextKey(func(k ui.TextKey) bool { return a.draftKey(t, k) })
		in.OnPaste(func(text string) bool { return a.todoPaste(t, draftID, text, a.frameNow) })
		if l.focus == draftID {
			in.Focus()
			l.focus = 0
		}
	})
}

// todoKey shows an accelerator as the status bar's hints write it.
func todoKey(acc string) string {
	s := keys.Display(acc, runtime.GOOS)
	if runtime.GOOS == "darwin" {
		return s
	}
	return strings.NewReplacer("ENTER", "Enter", "BACKSPACE", "Backspace", "UP", "↑", "DOWN", "↓").Replace(s)
}

func todoHints() string {
	move := todoKey("Alt+Up") + "↓"
	if runtime.GOOS != "darwin" {
		move = "Alt+↑↓"
	}
	return locales.TodoHints(todoKey("Enter"), todoKey("CmdOrCtrl+Enter"), move, todoKey("CmdOrCtrl+Shift+Backspace"))
}
