// Package appui is Lastab's native UI: the tab bar, the editor, the status
// bar and the recent switcher, drawn by MyGo. It calls the business layer
// directly; slow work runs off the main thread through Deps.Async.
package appui

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"

	"github.com/momaek/lastab/internal/snippet"
	"github.com/momaek/lastab/internal/store"
	"github.com/momaek/lastab/internal/theme"
	"github.com/momaek/lastab/internal/todo"
)

// Timing of automatic keeping.
const (
	SaveDelay     = 300 * time.Millisecond // typing paused → write the body
	TitleDelay    = time.Second            // first content → automatic title
	SnapshotDelay = 3 * time.Second        // typing paused → version
	CopiedFor     = 2 * time.Second        // "Copied" stays on the button
)

// Clipboard is the system clipboard.
type Clipboard interface {
	WriteText(string)
	ReadText() string
}

// Deps are what the UI needs from outside.
type Deps struct {
	Service   *snippet.Service
	Clipboard Clipboard
	// Async runs work off the main thread, then the function it returns on
	// the main thread, drawing a new frame. Tests run both at once.
	Async func(work func() (apply func()))
	// Recovered says the last run ended abnormally.
	Recovered bool
	// Accent is the accent color; "" for the default.
	Accent string
	Log    *slog.Logger
	// Platform applies settings outside the window; nil in some tests.
	Platform Platform
	// DataDir holds lastab.db; AgentLog is agent_log.db, which the CLI
	// writes and Settings reads.
	DataDir  string
	AgentLog string
}

// Tab is one open snippet. A blank tab has no ID: it lives in memory until
// it first has real content.
type Tab struct {
	key  int // stable element key
	id   string
	kind store.Kind
	ed   *editor.State
	// todo is the list a to-do tab shows in place of the editor; ed's
	// buffer stays its body, the list's plain text (todo.go).
	todo *todoList

	rev         int
	title       string
	titleSource store.TitleSource
	createdAt   time.Time
	modifiedAt  time.Time
	// touchedAt is the last activity, an edit or opening the snippet: the
	// sidebar and the switcher list tabs by it, newest first.
	touchedAt time.Time

	seenVer   uint64    // buffer version last seen by tick
	editedAt  time.Time // frame time of the last edit
	savedVer  uint64    // version on disk (or blank and skipped)
	failedVer uint64    // version whose save failed; retried on the next edit
	snapVer   uint64    // version recorded in history
	saving    bool
	flushNow  bool
	saveErr   error
	wantTitle bool
	titling   bool

	charsVer uint64
	chars    int
}

func (t *Tab) buf() *editor.Buffer { return t.ed.Buffer }

// Body returns the text exactly, with each line's own line ending.
func (t *Tab) Body() string { return bodyOf(t.buf().Snapshot()) }

func bodyOf(s editor.Snapshot) string {
	var b strings.Builder
	s.WriteTo(&b)
	return b.String()
}

// ID returns the snippet ID, "" while blank.
func (t *Tab) ID() string { return t.id }

// App is the state of the main window.
type App struct {
	d       Deps
	tabs    []*Tab
	active  int
	nextKey int

	recovered bool
	// recoveredUntil is when the quiet "Restored" notice goes, unless an
	// edit takes it away first.
	recoveredUntil time.Time
	rename         renaming
	hold           holdHint
	copiedUntil    time.Time
	copiedItems    int // items of the to-do list copied, for the announcement
	switcher       switcher
	lib            library
	settings       Settings
	prefs          prefsPage
	history        history
	toast          *toast
	// frameNow is the time of the frame being built, by the window's clock.
	frameNow time.Time
	// pendingHistory opens the history of this snippet once its tab opens.
	pendingHistory string

	sidebarHidden bool
	// scrollActive brings the active tab into view in the sidebar, once.
	scrollActive bool
	lastActive   *Tab

	// focusEditor asks the next frame to focus the active editor.
	focusEditor bool
	// pal and acc are the frame's palette and accent shades.
	pal *theme.Palette
	acc theme.Derived
}

// New returns the app with one blank tab; Load restores the workspace.
func New(d Deps) *App {
	if d.Log == nil {
		d.Log = slog.Default()
	}
	a := &App{d: d, recovered: d.Recovered, focusEditor: true, settings: DefaultSettings()}
	a.tabs = []*Tab{a.blankTab(store.KindText)}
	return a
}

func (a *App) ctx() context.Context { return context.Background() }

func (a *App) blankTab(kind store.Kind) *Tab {
	a.nextKey++
	body := "" // a to-do list starts with no items, its add row focused
	now := time.Now()
	t := &Tab{key: a.nextKey, kind: kind, titleSource: store.TitlePending, createdAt: now, modifiedAt: now, touchedAt: now}
	t.ed = a.newEditor(body)
	t.seenVer = t.buf().Version()
	t.savedVer = t.seenVer
	t.snapVer = t.seenVer
	return t
}

func (a *App) tabFor(sn store.Snippet) *Tab {
	a.nextKey++
	t := &Tab{
		key: a.nextKey, id: sn.ID, kind: sn.Kind, rev: sn.Rev,
		title: sn.Title, titleSource: sn.TitleSource,
		createdAt: sn.CreatedAt, modifiedAt: sn.ModifiedAt, touchedAt: sn.ModifiedAt,
	}
	t.ed = a.newEditor(sn.Body)
	v := t.buf().Version()
	t.seenVer, t.savedVer, t.snapVer = v, v, v
	t.wantTitle = sn.TitleSource == store.TitlePending
	return t
}

func (a *App) newEditor(body string) *editor.State {
	buf := editor.NewBuffer(body)
	st := &editor.State{
		Buffer:      buf,
		SoftWrap:    true,
		NoAutoClose: true,
		NoFolding:   true,
		Indent:      editor.IndentTabs,
	}
	a.styleEditor(st)
	st.OnInput = func(ev ui.InputEvent) bool { return a.editorInput(st, ev) }
	return st
}

// editorInput keeps pasted text exactly as it was: the editor gives pasted
// line breaks the ending of the line they go into, so CRLF text pasted
// into a blank snippet would turn into LF. Into an empty buffer, the
// clipboard's text is loaded as it is instead.
func (a *App) editorInput(st *editor.State, ev ui.InputEvent) bool {
	if ev.Kind != ui.InputCommand || ev.Text != "paste" || a.d.Clipboard == nil {
		return false
	}
	if st.Buffer.Len() != 0 {
		return false
	}
	text := a.d.Clipboard.ReadText()
	if !strings.ContainsRune(text, '\r') {
		return false
	}
	st.Buffer.Set(text)
	st.GoTo(st.Buffer.Pos(st.Buffer.Len()))
	return true
}

// Load restores the open tabs.
func (a *App) Load() error {
	set, err := LoadSettings(a.ctx(), a.d.Service.Store)
	if err != nil {
		a.d.Log.Error("load settings", "err", err)
	}
	a.settings = set
	if a.d.Accent == "" {
		a.d.Accent = set.Accent
	}
	for _, t := range a.tabs {
		a.styleEditor(t.ed)
	}
	a.maskAutoTitles()
	sns, active, err := a.d.Service.Workspace(a.ctx())
	if err != nil {
		return err
	}
	if len(sns) == 0 {
		return nil
	}
	a.tabs = a.tabs[:0]
	for i, sn := range sns {
		a.tabs = append(a.tabs, a.tabFor(sn))
		if sn.ID == active {
			a.active = i
		}
	}
	a.Resort()
	a.focusEditor = true
	return nil
}

// Tabs returns the open tabs, for tests and the shell.
func (a *App) Tabs() []*Tab { return a.tabs }

// Active returns the active tab.
func (a *App) Active() *Tab { return a.tabs[a.active] }

// NewTab opens a blank tab, at the top as the newest, and focuses it.
func (a *App) NewTab(kind store.Kind) {
	a.leave(a.Active())
	a.tabs = append(a.tabs, a.blankTab(kind))
	a.active = len(a.tabs) - 1
	a.Resort()
	a.focusEditor = true
}

// Resort orders the tabs by last activity. It runs at settle points, as
// the user switches, opens or closes a snippet or comes back to the
// window, never while typing: the snippet being edited stays where it is
// until the user leaves it.
func (a *App) Resort() {
	if len(a.tabs) == 0 {
		return
	}
	cur := a.Active()
	slices.SortStableFunc(a.tabs, func(x, y *Tab) int { return y.touchedAt.Compare(x.touchedAt) })
	a.active = slices.Index(a.tabs, cur)
}

// Activate switches to tab i.
func (a *App) Activate(i int) {
	if i < 0 || i >= len(a.tabs) || i == a.active {
		return
	}
	a.leave(a.Active())
	a.active = i
	a.Resort()
	a.focusEditor = true
	a.persistOrder()
}

// leave saves the tab at once and records a version, as the user switches
// away from it.
func (a *App) leave(t *Tab) {
	t.flushNow = true
	if t.id == "" {
		return
	}
	id, body := t.id, t.Body()
	t.snapVer = t.buf().Version()
	a.d.Async(func() func() {
		if _, err := a.d.Service.Snapshot(a.ctx(), id, body, store.ReasonSwitch); err != nil {
			a.d.Log.Error("snapshot", "err", err)
		}
		return nil
	})
}

// CloseTab closes tab i: the snippet stays, findable later. Closing the
// last tab leaves a blank one.
func (a *App) CloseTab(i int) {
	if i < 0 || i >= len(a.tabs) {
		return
	}
	t := a.tabs[i]
	wasActive := i == a.active
	cur := a.Active()
	a.tabs = slices.Delete(a.tabs, i, i+1)
	if len(a.tabs) == 0 {
		a.tabs = []*Tab{a.blankTab(store.KindText)}
	}
	if wasActive {
		// Back to the most recent of the others.
		a.active = 0
		a.Resort()
		a.active = 0
	} else {
		a.active = max(slices.Index(a.tabs, cur), 0)
		a.Resort()
	}
	a.focusEditor = true

	body := t.Body()
	dirty := t.buf().Version() != t.savedVer
	id, rev, kind := t.id, t.rev, t.kind
	order, activeID := a.order()
	a.d.Async(func() func() {
		ctx := a.ctx()
		svc := a.d.Service
		var err error
		switch {
		case id == "" && !snippet.IsBlank(body):
			// Typed and closed before the first save: keep it, closed.
			var sn store.Snippet
			if sn, err = svc.Create(ctx, kind, body, 0); err == nil {
				svc.UpdateTitle(ctx, sn.ID)
				err = svc.Close(ctx, sn.ID, body)
			}
		case id != "":
			if dirty {
				_, err = svc.SaveBody(ctx, id, body, rev)
			}
			if err == nil {
				err = svc.Close(ctx, id, body)
			}
		}
		if err != nil {
			a.d.Log.Error("close tab", "err", err)
		}
		if err := svc.Reorder(ctx, order, activeID); err != nil {
			a.d.Log.Error("reorder", "err", err)
		}
		return nil
	})
}

// CloseActive closes the active tab.
func (a *App) CloseActive() { a.CloseTab(a.active) }

func (a *App) order() ([]string, string) {
	var ids []string
	for _, t := range a.tabs {
		if t.id != "" {
			ids = append(ids, t.id)
		}
	}
	return ids, a.Active().id
}

func (a *App) persistOrder() {
	ids, active := a.order()
	a.d.Async(func() func() {
		if err := a.d.Service.Reorder(a.ctx(), ids, active); err != nil {
			a.d.Log.Error("reorder", "err", err)
		}
		return nil
	})
}

// CopyAll copies the active snippet's body, only the body, with its line
// endings, and records a version.
func (a *App) CopyAll(now time.Time) {
	t := a.Active()
	body := t.Body()
	if a.d.Clipboard != nil {
		a.d.Clipboard.WriteText(body)
	}
	a.copiedUntil = now.Add(CopiedFor)
	if t.kind == store.KindTodo {
		a.copiedItems = len(todo.Parse(body))
	}
	if t.id != "" {
		id := t.id
		a.d.Async(func() func() {
			a.d.Service.Snapshot(a.ctx(), id, body, store.ReasonCopy)
			return nil
		})
	}
}

// tick keeps every tab: writes bodies once typing pauses, makes titles and
// records versions. It runs each frame and asks for the frames it needs.
func (a *App) tick(c *ui.Context) {
	now := c.Now()
	for i, t := range a.tabs {
		v := t.buf().Version()
		if v != t.seenVer {
			t.seenVer = v
			t.editedAt = now
			t.modifiedAt = time.Now()
			t.touchedAt = t.modifiedAt
		}
		if v != t.savedVer && !t.saving && v != t.failedVer {
			if wait := SaveDelay - now.Sub(t.editedAt); wait <= 0 || t.flushNow {
				a.save(t, i)
			} else {
				c.After(wait)
			}
		}
		t.flushNow = false
		if t.id != "" && t.wantTitle && !t.titling && !t.saving {
			if wait := TitleDelay - now.Sub(t.editedAt); wait <= 0 {
				a.makeTitle(t)
			} else {
				c.After(wait)
			}
		}
		if t.id != "" && v == t.savedVer && t.snapVer != v && !t.saving {
			if wait := SnapshotDelay - now.Sub(t.editedAt); wait <= 0 {
				a.snapshot(t, store.ReasonPaused)
			} else {
				c.After(wait)
			}
		}
	}
	if !a.copiedUntil.IsZero() && now.Before(a.copiedUntil) {
		c.After(a.copiedUntil.Sub(now))
	}
}

func (a *App) save(t *Tab, position int) {
	body := t.Body()
	ver := t.buf().Version()
	svc := a.d.Service
	if t.id == "" {
		if snippet.IsBlank(body) {
			t.savedVer = ver // a blank tab is never written
			return
		}
		t.saving = true
		kind := t.kind
		a.d.Async(func() func() {
			sn, err := svc.Create(a.ctx(), kind, body, position)
			return func() {
				t.saving = false
				if err != nil {
					t.saveErr, t.failedVer = err, ver
					return
				}
				t.id, t.rev, t.createdAt, t.modifiedAt = sn.ID, sn.Rev, sn.CreatedAt, sn.ModifiedAt
				t.savedVer, t.saveErr, t.failedVer = ver, nil, 0
				t.wantTitle = true
				a.persistOrder()
			}
		})
		return
	}
	t.saving = true
	id, rev := t.id, t.rev
	a.d.Async(func() func() {
		res, err := svc.SaveBody(a.ctx(), id, body, rev)
		if errors.Is(err, store.ErrConflict) {
			// Only this window writes; a conflict means an earlier write
			// raced. Take the stored revision and write again.
			if sn, gerr := svc.Store.Get(a.ctx(), id); gerr == nil {
				res, err = svc.SaveBody(a.ctx(), id, body, sn.Rev)
			}
		}
		return func() {
			t.saving = false
			if err != nil {
				t.saveErr, t.failedVer = err, ver
				return
			}
			t.rev, t.modifiedAt = res.Rev, res.ModifiedAt
			t.savedVer, t.saveErr, t.failedVer = ver, nil, 0
			if t.kind == store.KindTodo || t.titleSource == store.TitlePending {
				t.wantTitle = true
			}
		}
	})
}

// Retry writes a tab whose save failed.
func (a *App) Retry(t *Tab) {
	t.failedVer = 0
	t.flushNow = true
}

func (a *App) makeTitle(t *Tab) {
	t.titling = true
	t.wantTitle = false
	id := t.id
	a.d.Async(func() func() {
		title, src, err := a.d.Service.UpdateTitle(a.ctx(), id)
		return func() {
			t.titling = false
			if err != nil {
				a.d.Log.Error("title", "err", err)
				return
			}
			t.title, t.titleSource = title, src
		}
	})
}

func (a *App) snapshot(t *Tab, reason store.Reason) {
	t.snapVer = t.buf().Version()
	id, body := t.id, t.Body()
	a.d.Async(func() func() {
		if _, err := a.d.Service.Snapshot(a.ctx(), id, body, reason); err != nil {
			a.d.Log.Error("snapshot", "err", err)
		}
		return nil
	})
}

// FlushAll writes every unsaved body now, on the calling goroutine: as the
// app quits or the window loses the focus.
func (a *App) FlushAll() {
	svc := a.d.Service
	for i, t := range a.tabs {
		if t.buf().Version() == t.savedVer {
			continue
		}
		body := t.Body()
		if t.id == "" {
			if snippet.IsBlank(body) {
				continue
			}
			sn, err := svc.Create(a.ctx(), t.kind, body, i)
			if err != nil {
				a.d.Log.Error("flush create", "err", err)
				continue
			}
			t.id, t.rev = sn.ID, sn.Rev
			svc.UpdateTitle(a.ctx(), sn.ID)
		} else {
			res, err := svc.SaveBody(a.ctx(), t.id, body, t.rev)
			if err != nil {
				a.d.Log.Error("flush save", "err", err)
				continue
			}
			t.rev = res.Rev
		}
		t.savedVer = t.buf().Version()
		svc.Snapshot(a.ctx(), t.id, body, store.ReasonClose)
	}
	ids, active := a.order()
	svc.Reorder(a.ctx(), ids, active)
}

// Title returns the label of a tab.
func (a *App) Title(t *Tab) string {
	return tabTitle(t)
}

func charCount(t *Tab) int {
	if v := t.buf().Version(); v != t.charsVer || t.charsVer == 0 {
		t.chars = utf8.RuneCountInString(t.buf().String())
		t.charsVer = v
	}
	return t.chars
}

// RecoveredFor is how long the notice of a recovery after a crash shows,
// unless the user edits first.
const RecoveredFor = 8 * time.Second

// recoveredShowing reports whether the quiet notice of a recovery shows: a
// few seconds after launch, or until the first edit.
func (a *App) recoveredShowing(c *ui.Context) bool {
	if !a.recovered {
		return false
	}
	now := c.Now()
	if a.recoveredUntil.IsZero() {
		a.recoveredUntil = now.Add(RecoveredFor)
	}
	for _, t := range a.tabs {
		if t.buf().Version() != t.savedVer || !t.editedAt.IsZero() {
			a.recovered = false
		}
	}
	if !now.Before(a.recoveredUntil) {
		a.recovered = false
	}
	if a.recovered {
		c.After(a.recoveredUntil.Sub(now))
	}
	return a.recovered
}
