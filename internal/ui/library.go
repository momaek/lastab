package appui

import (
	"context"
	"runtime"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"

	"github.com/momaek/lastab/internal/redact"
	"github.com/momaek/lastab/internal/search"
	"github.com/momaek/lastab/internal/store"
	"github.com/momaek/lastab/internal/theme"
	"github.com/momaek/lastab/internal/ui/keys"
	"github.com/momaek/lastab/locales"
)

// LibraryLimit is the most results the library shows.
const LibraryLimit = 300

// monoFont is the monospaced text of lists and previews: JetBrains Mono,
// Chinese falling back to the system's, without ligatures (monoFeatures).
const monoFont = theme.MonoFamily

// monoFeatures turns JetBrains Mono's ligatures off, so code shows as typed.
var monoFeatures = []string{"calt=0", "liga=0"}

// previewMax is the most bytes of a body the preview lays out.
const previewMax = 20000

// Filter choices; 0 is "all" in each.
const (
	timeAll = iota
	timeToday
	time7Days
	time30Days
	timeCustom // from and to, whole days
)

const (
	scopeAll = iota
	scopeOpen
	scopeClosed
)

const (
	kindAll = iota
	kindText
	kindTodo
)

// library is All snippets and search (Cmd+K): a page over the editor that
// finds every snippet, closed ones too, and opens, copies or deletes it.
type library struct {
	open   bool
	query  string
	when   int
	scope  int
	kind   int
	items  []libItem
	total  int
	sel    int
	loaded bool

	stale     bool
	gen       int
	cancel    context.CancelFunc
	scrollSel bool
	reveal    bool
	confirm   bool
	focus     bool

	// from and to are the days of the custom time range, inclusive;
	// draftFrom and draftTo what its popover shows until applied.
	from, to           time.Time
	draftFrom, draftTo time.Time
	rangeOpen          bool

	// rows are the list's rows: a header for each day, then its results.
	rows []libRow
	list ui.ListState
	// scrollTo shows the selected row once, wherever it is, as after a
	// query restores the selection further down.
	scrollTo bool
}

// libRow is a row of the result list: a day's header, or a result.
type libRow struct {
	header string
	item   int // index in items; -1 for a header
}

// group lays the results out under the days they were edited. The results
// are in order of their last edit, so each day comes once.
func (l *library) group(now time.Time, loc string) {
	l.rows = l.rows[:0]
	day := ""
	for i := range l.items {
		if d := dayLabel(now, l.items[i].ModifiedAt, loc); d != day {
			day = d
			l.rows = append(l.rows, libRow{header: d, item: -1})
		}
		l.rows = append(l.rows, libRow{item: i})
	}
}

// rowOf returns the row showing result i.
func (l *library) rowOf(i int) int {
	for r, row := range l.rows {
		if row.item == i {
			return r
		}
	}
	return 0
}

type libItem struct {
	search.Result
	versions int
}

func (l *library) filtered() bool {
	return l.when != timeAll || l.scope != scopeAll || l.kind != kindAll
}

func (l *library) options(now time.Time) store.ListOptions {
	o := store.ListOptions{Limit: LibraryLimit}
	switch l.when {
	case timeToday:
		o.Since = startOfDay(now)
	case time7Days:
		o.Since = now.AddDate(0, 0, -7)
	case time30Days:
		o.Since = now.AddDate(0, 0, -30)
	case timeCustom:
		o.Since = startOfDay(l.from)
		o.Until = startOfDay(l.to).AddDate(0, 0, 1)
	}
	switch l.scope {
	case scopeOpen:
		o.OpenOnly = true
	case scopeClosed:
		o.ClosedOnly = true
	}
	switch l.kind {
	case kindText:
		o.Kind = store.KindText
	case kindTodo:
		o.Kind = store.KindTodo
	}
	return o
}

func (l *library) selected() *libItem {
	if l.sel < 0 || l.sel >= len(l.items) {
		return nil
	}
	return &l.items[l.sel]
}

func (l *library) move(d int) {
	if len(l.items) == 0 {
		return
	}
	l.sel = min(max(l.sel+d, 0), len(l.items)-1)
	l.scrollSel = true
	l.reveal, l.confirm = false, false
}

// OpenLibrary shows All snippets, writing unsaved bodies first so the
// search sees them.
func (a *App) OpenLibrary() {
	a.FlushAll()
	a.switcher.open = false
	l := &a.lib
	l.open, l.query, l.sel, l.stale, l.focus = true, "", 0, true, true
	l.reveal, l.confirm = false, false
}

// ToggleLibrary shows or hides All snippets, as Cmd+K does.
func (a *App) ToggleLibrary() {
	if a.lib.open {
		a.CloseLibrary()
		return
	}
	a.OpenLibrary()
}

// CloseLibrary goes back to the editor.
func (a *App) CloseLibrary() {
	if l := &a.lib; l.cancel != nil {
		l.cancel()
		l.cancel = nil
	}
	a.lib.open = false
	a.Resort()
	a.focusEditor = true
}

// LibraryOpen reports whether All snippets shows.
func (a *App) LibraryOpen() bool { return a.lib.open }

// refresh runs the query when it changed, cancelling the one before.
func (a *App) refresh(now time.Time) {
	l := &a.lib
	if !l.stale {
		return
	}
	l.stale = false
	if l.cancel != nil {
		l.cancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	l.cancel = cancel
	l.gen++
	gen, q, o := l.gen, l.query, l.options(now)
	keep := ""
	if it := l.selected(); it != nil {
		keep = it.ID
	}
	st := a.d.Service.Store
	a.d.Async(func() func() {
		res, err := search.Searcher{Store: st}.Query(ctx, q, o)
		if err != nil {
			if ctx.Err() == nil {
				a.d.Log.Error("search", "err", err)
			}
			return nil
		}
		ids := make([]string, len(res))
		for i, r := range res {
			ids[i] = r.ID
		}
		counts, _ := st.VersionCounts(ctx, ids)
		total, _ := st.Count(ctx)
		return func() {
			if gen != l.gen {
				return // a newer query is on its way
			}
			// Relevance chose which results come back; they show in order
			// of their last edit, as the page says, grouped by day.
			slices.SortStableFunc(res, func(x, y search.Result) int { return y.ModifiedAt.Compare(x.ModifiedAt) })
			l.items = l.items[:0]
			for _, r := range res {
				l.items = append(l.items, libItem{r, counts[r.ID]})
			}
			l.total, l.loaded = total, true
			l.sel = 0
			for i, it := range l.items {
				if it.ID == keep && q == "" {
					l.sel = i
				}
			}
			l.group(time.Now(), i18n.Locale())
			l.list = ui.ListState{} // another set of rows: from the top
			l.scrollTo = l.sel > 0
		}
	})
}

// OpenSelected opens the selected snippet in a tab, or switches to its tab.
func (a *App) OpenSelected() {
	it := a.lib.selected()
	if it == nil {
		return
	}
	a.openSnippet(it.ID)
}

func (a *App) openSnippet(id string) {
	a.CloseLibrary()
	for i, t := range a.tabs {
		if t.id == id {
			a.Activate(i)
			return
		}
	}
	a.leave(a.Active())
	a.d.Async(func() func() {
		sn, err := a.d.Service.Open(a.ctx(), id)
		if err != nil {
			a.d.Log.Error("open snippet", "err", err)
			return nil
		}
		return func() {
			// A blank tab with nothing typed gives its place to the snippet.
			if cur := a.Active(); cur.id == "" && cur.buf().Len() == 0 && len(a.tabs) == 1 {
				a.tabs = a.tabs[:0]
			}
			t := a.tabFor(sn)
			t.touchedAt = time.Now()
			a.tabs = append(a.tabs, t)
			a.active = len(a.tabs) - 1
			a.Resort()
			a.focusEditor = true
			a.persistOrder()
			if a.pendingHistory == sn.ID {
				a.pendingHistory = ""
				a.OpenHistory()
			}
		}
	})
}

// HistoryOfSelected opens the selected snippet with its version history.
func (a *App) HistoryOfSelected() {
	it := a.lib.selected()
	if it == nil {
		return
	}
	a.pendingHistory = it.ID
	a.openSnippet(it.ID)
	if a.pendingHistory != "" && a.Active().id == it.ID { // it was open already
		a.pendingHistory = ""
		a.OpenHistory()
	}
}

// CopySelected copies the selected snippet's body exactly.
func (a *App) CopySelected(now time.Time) {
	it := a.lib.selected()
	if it == nil {
		return
	}
	if a.d.Clipboard != nil {
		a.d.Clipboard.WriteText(it.Body)
	}
	a.copiedUntil = now.Add(CopiedFor)
}

// DeleteSelected deletes the selected snippet for good, with its versions.
func (a *App) DeleteSelected() {
	it := a.lib.selected()
	if it == nil {
		return
	}
	id := it.ID
	a.lib.confirm = false
	a.lib.items = slices.Delete(a.lib.items, a.lib.sel, a.lib.sel+1)
	a.lib.total--
	a.lib.sel = min(a.lib.sel, max(len(a.lib.items)-1, 0))
	a.lib.group(time.Now(), i18n.Locale())
	if i := slices.IndexFunc(a.tabs, func(t *Tab) bool { return t.id == id }); i >= 0 {
		a.tabs = slices.Delete(a.tabs, i, i+1)
		if len(a.tabs) == 0 {
			a.tabs = []*Tab{a.blankTab(store.KindText)}
		}
		if a.active >= i && a.active > 0 {
			a.active--
		}
		a.active = min(a.active, len(a.tabs)-1)
	}
	a.d.Async(func() func() {
		if err := a.d.Service.Delete(a.ctx(), id); err != nil {
			a.d.Log.Error("delete", "err", err)
		}
		return func() { a.lib.stale = true }
	})
}

func (a *App) libraryView(c *ui.Context) {
	l := &a.lib
	a.refresh(time.Now())
	p := a.pal
	ui.Column(c.Key("library")).Fill().AlignItems(ui.Stretch).Background(p.Bg).Children(func() {
		a.libraryHeader(c)
		a.librarySearch(c)
		ui.Row(c).Grow(1).AlignItems(ui.Stretch).Children(func() {
			list := ui.Column(c.Key("results")).Width(472).AlignItems(ui.Stretch).
				BorderWidth(0, 1, 0, 0).BorderColor(p.Line)
			list.Children(func() {
				if l.loaded && len(l.items) == 0 {
					a.libraryEmpty(c)
					return
				}
				a.libraryList(c)
			})
			ui.Column(c.Key("preview")).Grow(1).Shrink(1).MinWidth(0).AlignItems(ui.Stretch).Children(func() {
				if it := l.selected(); it != nil {
					a.libraryPreview(c, it)
				} else {
					ui.Column(c).Grow(1).Background(p.Sunken).AlignItems(ui.Center).Justify(ui.Center).Padding(32).Children(func() {
						ui.Text(c, locales.NoPreview()).FontSize(12.5).TextColor(p.Muted)
					})
				}
			})
		})
	})
}

func (a *App) libraryHeader(c *ui.Context) {
	p := a.pal
	bar := c.TitleBar()
	ui.Row(c).Height(max(bar.Height, theme.TitleBarHeight)).Padding(0, bar.Right+8, 0, bar.Left+16).
		Gap(16).AlignItems(ui.Center).Background(p.Chrome).
		BorderWidth(0, 0, 1, 0).BorderColor(p.ChromeLine).DragWindow().Children(func() {
		ui.Text(c, locales.AllSnippets()).FontSize(13).FontWeight(600).Role(ui.RoleHeading)
		ui.Spacer(c)
		back := ui.ButtonBase(c.Key("back")).Row().Gap(8).AlignItems(ui.Center).Height(28).Padding(0, 12).
			Radius(8).Border(1, p.BtnLine).Label(locales.BackToEditor()).KeyShortcuts(0, ui.KeyEscape)
		if back.Hovered() {
			back.Background(p.BadgeTab)
		}
		back.Children(func() {
			ui.Text(c, locales.BackToEditor()).FontSize(12).TextColor(p.Text)
			ui.Text(c, keyName("esc")).FontSize(12).TextColor(p.Muted)
		})
		if back.Clicked() {
			a.CloseLibrary()
		}
	})
}

// keyName writes a key as the platform does: esc and ↵ on macOS, Esc and
// Enter elsewhere.
func keyName(k string) string {
	mac := runtime.GOOS == "darwin"
	switch k {
	case "esc":
		if mac {
			return "esc"
		}
		return "Esc"
	case "enter":
		if mac {
			return "↵"
		}
		return "Enter"
	case "alt":
		if mac {
			return "⌥"
		}
		return "Alt"
	}
	return k
}

func (a *App) librarySearch(c *ui.Context) {
	l := &a.lib
	p := a.pal
	ui.Column(c).Gap(12).Padding(16, 20, 12, 20).BorderWidth(0, 0, 1, 0).BorderColor(p.Line).Children(func() {
		ui.Row(c).Height(40).Gap(8).AlignItems(ui.Center).Padding(0, 12).
			Border(1.5, a.acc.Accent).Radius(8).Background(p.Bg).Role(ui.RoleSearch).Children(func() {
			ui.Text(c, "⌕").FontSize(16).TextColor(p.Muted)
			in := ui.TextInput(c.Key("query"), &l.query).Grow(1).Shrink(1).
				Placeholder(locales.SearchPlaceholder()).Label(locales.SearchAll()).
				Role(ui.RoleComboBox).HasPopup(ui.PopupListBox).
				FontSize(14).Font(monoFont).FontFeatures(monoFeatures...).Background(ui.Transparent).Border(0, ui.Transparent).Padding(0)
			if l.focus {
				in.Focus()
				l.focus = false
			}
			if in.Changed() {
				l.stale = true
				l.reveal, l.confirm = false, false
			}
			if in.Shortcut(0, ui.KeyDown) {
				l.move(1)
			}
			if in.Shortcut(0, ui.KeyUp) {
				l.move(-1)
			}
			if in.Submitted() {
				a.OpenSelected()
			}
			count := locales.TotalCount(l.total)
			if strings.TrimSpace(l.query) != "" || l.filtered() {
				count = locales.MatchCount(len(l.items), l.total)
			}
			if l.loaded {
				ui.Text(c, count).FontSize(12).TextColor(p.Muted).FontFeatures("tnum").Role(ui.RoleStatus)
			}
		})
		ui.Row(c).Wrap().GapX(20).GapY(8).AlignItems(ui.Center).Children(func() {
			a.chipsWith(c, "when", locales.FilterTime(), &l.when, func() { a.customRangeChip(c) },
				locales.OptAll(), locales.OptToday(), locales.Opt7Days(), locales.Opt30Days())
			a.chips(c, "scope", locales.FilterScope(), &l.scope, locales.OptAll(), locales.OptOpen(), locales.OptClosed())
			a.chips(c, "kind", locales.FilterKind(), &l.kind, locales.OptAll(), locales.OptText(), locales.OptTodo())
			ui.Spacer(c)
			ui.Text(c, locales.SortedByEdit()).FontSize(12.5).TextColor(p.Muted)
		})
	})
}

// chips is a group of pill buttons choosing one value.
func (a *App) chips(c *ui.Context, key, label string, value *int, opts ...string) {
	a.chipsWith(c, key, label, value, nil, opts...)
}

// chipsWith is chips with more built after the options, in the group.
func (a *App) chipsWith(c *ui.Context, key, label string, value *int, extra func(), opts ...string) {
	p := a.pal
	ui.Row(c.Key(key)).Gap(4).AlignItems(ui.Center).Role(ui.RoleRadioGroup).Label(label).Children(func() {
		ui.Text(c, label).FontSize(12.5).TextColor(p.Muted).Margin(0, 4, 0, 0)
		for i, o := range opts {
			on := *value == i
			b := ui.ButtonBase(c.Key(i)).Height(28).Padding(0, 12).Radius(14).
				Role(ui.RoleRadio).Checked(on).Label(o)
			fg := p.Text
			switch {
			case on:
				b.Background(p.InvBg).Border(1, p.InvBg)
				fg = p.InvText
			case b.Hovered():
				b.Background(p.Chrome).Border(1, p.BtnLine)
			default:
				b.Background(p.Btn).Border(1, p.BtnLine)
			}
			b.Children(func() { ui.Text(c, o).FontSize(12.5).TextColor(fg) })
			if b.Clicked() && !on {
				*value = i
				a.lib.stale = true
				a.lib.sel = 0
				a.lib.focus = true
			}
		}
		if extra != nil {
			extra()
		}
	})
}

// customRangeChip is the last time chip: "Custom…", or the range chosen,
// which a popover of two dates sets.
func (a *App) customRangeChip(c *ui.Context) {
	l := &a.lib
	p := a.pal
	on := l.when == timeCustom
	label := locales.OptCustom()
	if on {
		label = i18n.FormatDateRange(i18n.Locale(), l.from, l.to, i18n.DateOptions{Skeleton: "MMMd"})
	}
	b := ui.ButtonBase(c.Key("custom")).Height(28).Padding(0, 12).Radius(14).
		Role(ui.RoleRadio).Checked(on).Label(locales.OptCustom()).HasPopup(ui.PopupDialog).Expanded(l.rangeOpen)
	fg := p.Text
	switch {
	case on:
		b.Background(p.InvBg).Border(1, p.InvBg)
		fg = p.InvText
	case b.Hovered():
		b.Background(p.Chrome).Border(1, p.BtnLine)
	default:
		b.Background(p.Btn).Border(1, p.BtnLine)
	}
	b.Children(func() { ui.Text(c, label).FontSize(12.5).TextColor(fg).FontFeatures("tnum") })
	if b.Clicked() {
		today := startOfDay(time.Now())
		l.draftFrom, l.draftTo = today.AddDate(0, 0, -7), today
		if on {
			l.draftFrom, l.draftTo = l.from, l.to
		}
		l.rangeOpen = !l.rangeOpen
	}
	ui.PopoverBase(c, b, &l.rangeOpen, func(panel ui.Element) {
		panel.Column().Gap(12).Padding(16).Radius(12).Background(p.Panel).Border(1, p.BtnLine).
			Shadow(0, 8, 24, 0, ui.RGBA(0, 0, 0, 0.14)).Role(ui.RoleDialog).Label(locales.OptCustom())
		panel.Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Text(c, locales.RangeFrom()).FontSize(12.5).TextColor(p.Muted)
				ui.DateInput(c.Key("from"), &l.draftFrom).Label(locales.RangeFrom())
				ui.Text(c, locales.RangeTo()).FontSize(12.5).TextColor(p.Muted)
				ui.DateInput(c.Key("to"), &l.draftTo).Label(locales.RangeTo())
			})
			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
				if a.outlineButton(c, "cancel", locales.Cancel(), "").Clicked() {
					l.rangeOpen = false
				}
				if ui.PrimaryButton(c.Key("apply"), locales.Apply()).Clicked() {
					from, to := startOfDay(l.draftFrom), startOfDay(l.draftTo)
					if to.Before(from) {
						from, to = to, from
					}
					l.from, l.to, l.when = from, to, timeCustom
					l.rangeOpen, l.stale, l.sel, l.focus = false, true, 0, true
				}
			})
		})
	})
}

func (a *App) libraryList(c *ui.Context) {
	l := &a.lib
	p := a.pal
	now := time.Now()
	loc := i18n.Locale()
	exact := c.Modifiers()&ui.Alt != 0
	// A virtual list: only the rows in view are built, so 300 results lay
	// out as fast as ten.
	st := &l.list
	st.Key = func(r int) any {
		if row := l.rows[r]; row.item >= 0 {
			return l.items[row.item].ID
		}
		return "day:" + l.rows[r].header
	}
	st.Header = func(r int) bool { return l.rows[r].item < 0 }
	st.Label = func(r int) string {
		if row := l.rows[r]; row.item >= 0 {
			return listTitle(l.items[row.item].Snippet)
		}
		return l.rows[r].header
	}
	if l.scrollTo {
		st.ScrollTo(l.rowOf(l.sel), ui.Center)
		l.scrollTo = false
	}
	ui.List(c.Key("list"), st, len(l.rows), func(r int) {
		row := l.rows[r]
		if row.item < 0 {
			ui.Text(c, row.header).FontSize(11.5).FontWeight(600).TextColor(p.Muted).
				Padding(8, 12, 4, 12).Background(p.Bg).AccessHidden(true)
			return
		}
		a.libraryRow(c, &l.items[row.item], row.item, now, loc, exact)
	}).Grow(1).Shrink(1).Padding(8).Role(ui.RoleListBox).Label(locales.AllSnippets())
	hint := locales.LibraryHint(keyName("enter"), keyName("alt"))
	if exact {
		hint = locales.LibraryHintExact(keyName("enter"), keyName("alt"))
	}
	ui.Text(c, hint).FontSize(11.5).TextColor(p.Muted).Padding(8, 20).SingleLine()
	l.scrollSel = false
}

func (a *App) libraryRow(c *ui.Context, it *libItem, i int, now time.Time, loc string, exact bool) {
	l := &a.lib
	p := a.pal
	sel := i == l.sel
	row := ui.ButtonBase(c.Key(it.ID)).Column().AlignItems(ui.Stretch).Gap(4).Padding(8, 12).Radius(8).
		Role(ui.RoleOption).Selected(sel).PositionInSet(i+1, len(l.items)).Label(it.Title)
	switch {
	case sel:
		row.Background(a.acc.Sel)
		if l.scrollSel {
			row.ScrollIntoView()
		}
	case row.Hovered():
		row.Background(p.Chrome)
	}
	row.Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			icon := "≡"
			if it.Kind == store.KindTodo {
				icon = "☑"
			}
			ui.Text(c, icon).FontSize(13).TextColor(p.Muted)
			ui.Text(c, listTitle(it.Snippet)).FontSize(13.5).FontWeight(500).TextColor(p.Text).SingleLine().Shrink(1)
			if !it.Open {
				ui.Text(c, locales.ClosedBadge()).FontSize(11).TextColor(p.Muted).
					Background(p.Badge).Radius(4).Padding(0, 4)
			}
			ui.Spacer(c)
			when := relTime(now, it.ModifiedAt, loc)
			if exact {
				when = exactTime(now, it.ModifiedAt, loc)
			}
			ui.Text(c, when).FontSize(12).TextColor(p.Muted).FontFeatures("tnum").NoWrap()
		})
		ui.RichText(c, marked(oneLine(it.Preview), search.Highlights(oneLine(it.Preview), l.query), p)...).
			FontSize(12).Font(monoFont).FontFeatures(monoFeatures...).LineHeight(1.5).TextColor(p.Muted).SingleLine()
		created := locales.CreatedLine(exactTime(now, it.CreatedAt, loc), it.versions)
		if it.Kind == store.KindTodo {
			created += " · " + locales.TodoKind()
		}
		ui.Text(c, created).FontSize(11.5).TextColor(p.Muted).FontFeatures("tnum")
	})
	if row.Clicked() {
		l.sel = i
		l.reveal, l.confirm = false, false
		if row.Clicks() >= 2 {
			a.OpenSelected()
		}
	}
}

func listTitle(sn store.Snippet) string {
	if sn.Title != "" {
		return sn.Title
	}
	return locales.Untitled(sn.CreatedAt.Local().Format("15:04"))
}

// oneLine joins a preview's lines for a single-line row.
func oneLine(s string) string {
	s = strings.ReplaceAll(s, "\r\n", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	return strings.ReplaceAll(s, "\r", " ")
}

// marked splits text into spans with the ranges highlighted.
func marked(text string, hs []search.Range, p *theme.Palette) []ui.Span {
	var spans []ui.Span
	last := 0
	for _, h := range hs {
		if h.Start < last || h.End > len(text) {
			continue
		}
		if h.Start > last {
			spans = append(spans, ui.Span{Text: text[last:h.Start]})
		}
		spans = append(spans, ui.Span{Text: text[h.Start:h.End], Background: p.Mark, Color: p.MarkText, Radius: 2})
		last = h.End
	}
	if last < len(text) || len(spans) == 0 {
		spans = append(spans, ui.Span{Text: text[last:]})
	}
	return spans
}

func (a *App) libraryEmpty(c *ui.Context) {
	l := &a.lib
	p := a.pal
	ui.Column(c).Grow(1).Gap(12).Padding(32).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
		ui.Text(c, "⌕").FontSize(32).TextColor(p.Faint)
		title := locales.EmptyNone()
		if q := strings.TrimSpace(l.query); q != "" {
			title = locales.EmptyQuery(q)
		}
		ui.Text(c, title).FontSize(14).FontWeight(600).TextAlign(ui.Center)
		body := locales.EmptyBody()
		if l.filtered() {
			body = locales.EmptyFilteredBody()
		}
		ui.Text(c, body).FontSize(12.5).LineHeight(1.5).TextColor(p.Muted).MaxWidth(320).TextAlign(ui.Center)
		if l.filtered() && ui.Button(c, locales.ClearFilters()).Clicked() {
			l.when, l.scope, l.kind = timeAll, scopeAll, kindAll
			l.stale, l.focus = true, true
		}
	})
}

func (a *App) libraryPreview(c *ui.Context, it *libItem) {
	l := &a.lib
	p := a.pal
	now := time.Now()
	loc := i18n.Locale()
	body := it.Body
	truncated := len(body) > previewMax
	if truncated {
		cut := previewMax
		for cut > 0 && !utf8.RuneStart(body[cut]) {
			cut--
		}
		body = body[:cut]
	}
	shown, masked := redact.Text(body)
	if l.reveal {
		shown = body
	}
	if truncated {
		shown += "\n…"
	}

	// Title, state and facts.
	ui.Column(c).Gap(8).Padding(16, 24).BorderWidth(0, 0, 1, 0).BorderColor(p.Line).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			if !a.renameField(c, "library", it.ID, 17) {
				ui.Text(c, listTitle(it.Snippet)).FontSize(17).FontWeight(600).Shrink(1).MaxLines(2).Role(ui.RoleHeading)
				a.renameButton(c, "library", it.ID, it.Title)
			}
			ui.Spacer(c)
			if it.Open {
				ui.Text(c, locales.OpenInWorkspace()).FontSize(11.5).TextColor(p.OkText).
					Background(p.OkBg).Radius(4).Padding(1, 8).NoWrap()
			}
		})
		ui.Row(c).Wrap().GapX(20).GapY(4).Children(func() {
			small := func(s string) { ui.Text(c, s).FontSize(12).TextColor(p.Muted).FontFeatures("tnum") }
			small(locales.Created(exactTime(now, it.CreatedAt, loc)))
			modified := exactTime(now, it.ModifiedAt, loc)
			if sameDay(now, it.ModifiedAt) { // "14:36 (4 min ago)"; older ones say the date alone
				modified += " (" + relTime(now, it.ModifiedAt, loc) + ")"
			}
			small(locales.Modified(modified))
			small(locales.VersionsCount(it.versions))
			small(locales.LinesChars(lineCount(it.Body), utf8.RuneCountInString(it.Body)))
		})
	})

	// The body, masked unless revealed, with the matches marked.
	ui.Scroll(c.Key("body-"+it.ID)).Column().Grow(1).Shrink(1).AlignItems(ui.Stretch).
		Padding(16, 24).Background(p.Sunken).Children(func() {
		ui.RichText(c, marked(shown, search.Highlights(shown, l.query), p)...).
			FontSize(12.5).Font(monoFont).FontFeatures(monoFeatures...).LineHeight(20.0 / 12.5).TextColor(p.Text).PreserveSpaces().Selectable()
	})

	// Actions.
	ui.Row(c).Wrap().Gap(8).AlignItems(ui.Center).Padding(12, 24).BorderWidth(1, 0, 0, 0).BorderColor(p.Line).Children(func() {
		open := ui.ButtonBase(c.Key("open")).Row().Gap(8).AlignItems(ui.Center).Height(32).Padding(0, 16).Radius(8).
			Background(a.acc.Accent).Label(locales.OpenEdit()).KeyShortcuts(0, ui.KeyEnter)
		if open.Hovered() {
			open.Background(c.Theme().AccentHover)
		}
		open.Children(func() {
			ui.Text(c, locales.OpenEdit()).FontSize(13).FontWeight(500).TextColor(a.acc.OnAccent)
			ui.Text(c, keyName("enter")).FontSize(13).TextColor(a.acc.OnAccent).Opacity(0.8)
		})
		if open.Clicked() {
			a.OpenSelected()
		}
		copied := c.Now().Before(a.copiedUntil)
		label := locales.CopyAll()
		if copied {
			label = locales.Copied()
		}
		cp := a.outlineButton(c, "copy", label, keys.Display(a.Shortcuts()[keys.CopyAll], runtime.GOOS))
		if cp.Clicked() {
			a.CopySelected(c.Now())
			c.Announce(locales.CopiedAnnounce())
		}
		if a.outlineButton(c, "history", locales.HistoryTitle(), keys.Display(a.Shortcuts()[keys.History], runtime.GOOS)).Clicked() {
			a.HistoryOfSelected()
		}
		if masked {
			lbl := locales.RevealSecrets()
			if l.reveal {
				lbl = locales.HideSecrets()
			}
			if a.outlineButton(c, "reveal", lbl, "").Clicked() {
				l.reveal = !l.reveal
			}
		}
		ui.Spacer(c)
		del := ui.ButtonBase(c.Key("delete")).Height(32).Padding(0, 12).Radius(8).
			Label(locales.DeleteEllipsis()).Expanded(l.confirm).HasPopup(ui.PopupDialog)
		if l.confirm || del.Hovered() {
			del.Background(p.DangerBg)
		}
		del.Children(func() { ui.Text(c, locales.DeleteEllipsis()).FontSize(13).TextColor(p.Danger) })
		if del.Clicked() {
			l.confirm = !l.confirm
		}
		a.confirmDelete(c, del, it)
	})
}

func (a *App) outlineButton(c *ui.Context, key, label, hint string) ui.Element {
	p := a.pal
	b := ui.ButtonBase(c.Key(key)).Row().Gap(8).AlignItems(ui.Center).Height(32).Padding(0, 12).Radius(8).
		Border(1, p.BtnLine).Background(p.Btn).Label(label)
	if b.Hovered() {
		b.Background(p.Chrome)
	}
	b.Children(func() {
		ui.Text(c, label).FontSize(13).TextColor(p.Text)
		if hint != "" {
			ui.Text(c, hint).FontSize(13).TextColor(p.Muted)
		}
	})
	return b
}

// confirmDelete is the confirmation anchored on Delete…: it names what goes
// and says it cannot come back.
func (a *App) confirmDelete(c *ui.Context, anchor ui.Element, it *libItem) {
	l := &a.lib
	p := a.pal
	ui.PopoverBase(c, anchor, &l.confirm, func(panel ui.Element) {
		panel.AttachTo(anchor, ui.AnchorTopRight, ui.AnchorBottomRight).Margin(0, 0, 8, 0)
		panel.Column().Width(320).Gap(8).Padding(16).Radius(12).Background(p.Panel).
			Border(1, p.BtnLine).Shadow(0, 8, 24, 0, ui.RGBA(0, 0, 0, 0.14)).
			Role(ui.RoleAlertDialog).Label(locales.ConfirmDeleteTitle())
		panel.Children(func() {
			ui.Text(c, locales.ConfirmDeleteTitle()).FontSize(13.5).FontWeight(600)
			ui.Text(c, locales.ConfirmDeleteBody(listTitle(it.Snippet), it.versions)).
				FontSize(12.5).LineHeight(1.5).TextColor(p.Muted)
			ui.Row(c).Gap(8).Justify(ui.End).Margin(4, 0, 0, 0).Children(func() {
				if a.outlineButton(c, "cancel", locales.Cancel(), "").Clicked() {
					l.confirm = false
				}
				ok := ui.ButtonBase(c.Key("confirm")).Height(32).Padding(0, 12).Radius(8).
					Background(p.Danger).Label(locales.DeleteSnippet())
				onDanger := ui.Hex("#FFFFFF")
				if c.Theme().Dark {
					onDanger = p.Bg
				}
				ok.Children(func() {
					ui.Text(c, locales.DeleteSnippet()).FontSize(13).FontWeight(500).TextColor(onDanger)
				})
				if ok.Clicked() {
					a.DeleteSelected()
				}
			})
		})
	})
}

func lineCount(s string) int {
	if s == "" {
		return 0
	}
	n := strings.Count(s, "\n") + strings.Count(s, "\r") - strings.Count(s, "\r\n")
	if !strings.HasSuffix(s, "\n") && !strings.HasSuffix(s, "\r") {
		n++
	}
	return n
}
