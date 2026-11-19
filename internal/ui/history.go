package appui

import (
	"strings"
	"time"

	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"

	"github.com/momaek/lastab/internal/redact"
	"github.com/momaek/lastab/internal/store"
	"github.com/momaek/lastab/internal/theme"
	"github.com/momaek/lastab/locales"
)

// HistoryWidth is the width of the drawer of versions.
const HistoryWidth = 340

// History modes: the changes from the version to the current text, or the
// version's text alone.
const (
	modeDiff = iota
	modeFull
)

// history is Version history (Cmd+Y): a drawer of the active snippet's
// versions on the right, and in place of the editor, what changed from the
// selected version to the current text. A version restores with an undo.
type history struct {
	open     bool
	tab      *Tab
	versions []versionRow
	loaded   bool
	sel      int
	mode     int
	gen      int

	diff    *editor.DiffState
	full    *editor.State
	builtID string // the version diff and full show
}

type versionRow struct {
	store.Version
	peek string // the first line it changed, masked
}

// OpenHistory shows the active snippet's versions, recording its current
// text first so the newest version is what the editor shows.
func (a *App) OpenHistory() {
	a.switcher.open = false
	t := a.Active()
	a.history = history{open: true, tab: t, mode: modeDiff}
	a.loadHistory(true)
}

// CloseHistory goes back to the editor.
func (a *App) CloseHistory() {
	a.history = history{}
	a.focusEditor = true
}

// HistoryOpen reports whether Version history shows.
func (a *App) HistoryOpen() bool { return a.history.open }

// loadHistory reads the versions again, selecting the one before the
// current, or the current alone.
func (a *App) loadHistory(snapshot bool) {
	h := &a.history
	t := h.tab
	h.gen++
	gen := h.gen
	if t.id == "" {
		h.versions, h.loaded = nil, true
		return
	}
	if snapshot {
		a.FlushAll()
	}
	id, body := t.id, t.Body()
	svc := a.d.Service
	a.d.Async(func() func() {
		if snapshot {
			svc.Snapshot(a.ctx(), id, body, store.ReasonPaused)
		}
		vs, err := svc.Versions(a.ctx(), id)
		if err != nil {
			a.d.Log.Error("versions", "err", err)
		}
		rows := make([]versionRow, len(vs))
		for i, v := range vs {
			prev := ""
			if i+1 < len(vs) {
				prev = vs[i+1].Body
			}
			rows[i] = versionRow{Version: v, peek: redact.String(firstChanged(prev, v.Body))}
		}
		return func() {
			if gen != h.gen || !h.open {
				return
			}
			h.versions, h.loaded = rows, true
			h.sel = min(1, max(len(rows)-1, 0))
			h.builtID = ""
		}
	})
}

// diffText is a version's text as the history shows it: masked, ending
// in a line break.
func diffText(s string) string {
	s = redact.String(s)
	if !strings.HasSuffix(s, "\n") && !strings.HasSuffix(s, "\r") {
		s += "\n"
	}
	return s
}

// firstChanged returns the first line of body that prev doesn't have.
func firstChanged(prev, body string) string {
	had := map[string]bool{}
	for _, l := range splitLines(prev) {
		had[l] = true
	}
	for _, l := range splitLines(body) {
		if strings.TrimSpace(l) != "" && !had[l] {
			return strings.TrimSpace(l)
		}
	}
	for _, l := range splitLines(body) {
		if strings.TrimSpace(l) != "" {
			return strings.TrimSpace(l)
		}
	}
	return ""
}

func splitLines(s string) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.Split(strings.ReplaceAll(s, "\r", "\n"), "\n")
}

func (h *history) selected() *versionRow {
	if h.sel < 0 || h.sel >= len(h.versions) {
		return nil
	}
	return &h.versions[h.sel]
}

// reasonLabel says why a version was recorded.
func (h *history) reasonLabel(i int, now time.Time, loc string) string {
	v := h.versions[i]
	if i == len(h.versions)-1 {
		return locales.ReasonFirst()
	}
	switch v.Reason {
	case store.ReasonSwitch:
		return locales.ReasonSwitch()
	case store.ReasonClose:
		return locales.ReasonClose()
	case store.ReasonCopy:
		return locales.ReasonCopy()
	case store.ReasonRestore:
		if v.RestoredFrom == "" {
			return locales.ReasonBeforeRestore()
		}
		for _, o := range h.versions {
			if o.ID == v.RestoredFrom {
				return locales.ReasonRestoredFrom(versionTime(now, o.CreatedAt, loc))
			}
		}
		return locales.ReasonRestoredFrom("")
	}
	return locales.ReasonPaused()
}

// versionTime is a version's time: the clock today, else the date too.
func versionTime(now, t time.Time, loc string) string {
	if sameDay(now, t) {
		return i18n.FormatDate(loc, t.Local(), i18n.DateOptions{Skeleton: "Hm"})
	}
	return exactTime(now, t, loc)
}

// RestoreSelected puts the selected version's text in the editor, saving
// the current text as a version first, and offers to undo it.
func (a *App) RestoreSelected(now time.Time) {
	h := &a.history
	v := h.selected()
	if v == nil || h.sel == 0 || h.tab == nil || h.tab.id == "" {
		return
	}
	t := h.tab
	before, restored, fromID := t.Body(), v.Body, v.ID
	when := versionTime(now, v.CreatedAt, i18n.Locale())
	t.buf().Set(restored)
	t.flushNow = true
	t.snapVer = t.buf().Version() // recorded below, not again when typing pauses
	id := t.id
	a.d.Async(func() func() {
		if err := a.d.Service.RecordRestore(a.ctx(), id, before, restored, fromID); err != nil {
			a.d.Log.Error("restore", "err", err)
		}
		return func() {
			if h.open && h.tab == t {
				a.loadHistory(false)
			}
		}
	})
	a.showToast(locales.RestoredToast(when), UndoFor, locales.Undo(), func() { a.undoRestore(t, before) }, now)
}

// undoRestore puts back the text a restore replaced.
func (a *App) undoRestore(t *Tab, before string) {
	t.buf().Set(before)
	t.flushNow = true
	t.snapVer = t.buf().Version()
	id := t.id
	a.d.Async(func() func() {
		a.d.Service.Snapshot(a.ctx(), id, before, store.ReasonPaused)
		return func() {
			if a.history.open && a.history.tab == t {
				a.loadHistory(false)
			}
		}
	})
	a.showToast(locales.UndoneToast(), 3*time.Second, "", nil, a.frameNow)
}

// CopyVersion copies the selected version's text exactly.
func (a *App) CopyVersion() {
	if v := a.history.selected(); v != nil && a.d.Clipboard != nil {
		a.d.Clipboard.WriteText(v.Body)
	}
}

func (a *App) moveHistory(d int) {
	h := &a.history
	if len(h.versions) == 0 {
		return
	}
	h.sel = min(max(h.sel+d, 0), len(h.versions)-1)
}

// compareView takes the editor's place while the history shows.
func (a *App) compareView(c *ui.Context) {
	h := &a.history
	p := a.pal
	now := time.Now()
	loc := i18n.Locale()
	v := h.selected()
	single := h.loaded && len(h.versions) <= 1

	if v != nil && h.builtID != v.ID {
		h.builtID = v.ID
		// Masked, as lists are, and compared as snippets rather than files:
		// no "No newline at end of file" notes. Copy and restore use the
		// versions' own text.
		old := editor.NewBuffer(diffText(v.Body))
		cur := editor.NewBuffer(diffText(h.tab.Body()))
		h.diff = &editor.DiffState{
			Old: old, New: cur, Layout: editor.DiffUnified, Indicators: editor.DiffClassic,
			Wrap: true, Font: ui.Font{Family: theme.MonoEditor, Size: 13, Features: theme.NoLigatures}, LineHeight: 22.0 / 13, ContextLines: 3,
		}
		h.full = &editor.State{Buffer: old, ReadOnly: true, SoftWrap: true, NoFolding: true,
			Font: ui.Font{Family: theme.MonoEditor, Size: 13, Features: theme.NoLigatures}, LineHeight: 22.0 / 13, Label: locales.FullText()}
	}

	ui.Column(c.Key("compare")).Grow(1).Shrink(1).MinWidth(0).AlignItems(ui.Stretch).Children(func() {
		ui.Row(c).Height(44).Gap(12).AlignItems(ui.Center).Padding(0, 20).
			BorderWidth(0, 0, 1, 0).BorderColor(p.Line).Children(func() {
			switch {
			case v == nil:
			case h.sel == 0:
				ui.Text(c, locales.CurrentAt(versionTime(now, v.CreatedAt, loc))).FontSize(12.5).FontWeight(500)
			default:
				ui.Text(c, locales.CompareHead(versionTime(now, v.CreatedAt, loc), h.reasonLabel(h.sel, now, loc))).
					FontSize(12.5).FontWeight(500).SingleLine().Shrink(1)
				if h.diff != nil && h.mode == modeDiff {
					ui.Text(c, locales.ChangesCount(len(h.diff.Hunks()))).FontSize(12.5).TextColor(p.Muted).FontFeatures("tnum")
				}
			}
			ui.Spacer(c)
			if v != nil && h.sel > 0 {
				ui.Segmented(c.Key("mode"), &h.mode, locales.DiffMode(), locales.FullText())
			}
		})
		switch {
		case !h.loaded:
			ui.Spacer(c)
		case len(h.versions) == 0 || single:
			a.historyEmpty(c, len(h.versions) == 0)
		case h.sel == 0 || h.mode == modeFull:
			h.full.Theme = a.tab().ed.Theme
			editor.View(c.Key("full-"+v.ID), h.full).Grow(1).FillWidth()
		default:
			th := editor.DefaultTheme(c.Theme())
			th.Background, th.Text = p.Bg, p.Text
			th.Added, th.Deleted = p.AddText, p.DelText
			h.diff.Theme = th
			editor.DiffView(c.Key("diff-"+v.ID), h.diff).Grow(1).FillWidth()
		}
	})
}

func (a *App) tab() *Tab { return a.history.tab }

func (a *App) historyEmpty(c *ui.Context, none bool) {
	p := a.pal
	ui.Column(c).Grow(1).Gap(8).Padding(32).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
		ui.Icon(c, iconHistory).Size(28, 28).TextColor(p.Faint)
		title := locales.SingleTitle()
		if none {
			title = locales.NoVersionsTitle()
		}
		ui.Text(c, title).FontSize(14).FontWeight(600)
		ui.Text(c, locales.SingleBody()).FontSize(12.5).LineHeight(1.5).TextColor(p.Muted).MaxWidth(360).TextAlign(ui.Center)
	})
}

// historyDrawer is the list of versions on the right, newest first.
func (a *App) historyDrawer(c *ui.Context) {
	h := &a.history
	p := a.pal
	now := time.Now()
	loc := i18n.Locale()
	single := len(h.versions) <= 1
	ui.Column(c.Key("history")).Width(HistoryWidth).AlignItems(ui.Stretch).
		Background(p.Side).BorderWidth(0, 0, 0, 1).BorderColor(p.Line).Role(ui.RoleDialog).Label(locales.HistoryTitle()).
		Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.Start).Padding(12, 16).BorderWidth(0, 0, 1, 0).BorderColor(p.Line).Children(func() {
				ui.Column(c).Grow(1).Shrink(1).Gap(4).Children(func() {
					ui.Text(c, locales.HistoryTitle()).FontSize(13.5).FontWeight(600).Role(ui.RoleHeading)
					ui.Text(c, locales.HistorySub()).FontSize(12).LineHeight(17.0 / 12).TextColor(p.Muted)
				})
				// Pulled up so the × lines up with the title rather than below it.
				if a.iconButton(c, "close", iconClose, locales.CloseHistory(), "").Margin(-5, -6, 0, 0).Clicked() {
					a.CloseHistory()
				}
			})
			ui.Scroll(c.Key("versions")).Column().Grow(1).Shrink(1).AlignItems(ui.Stretch).Gap(4).Padding(8).
				Role(ui.RoleListBox).Label(locales.VersionsList()).Children(func() {
				for i := range h.versions {
					a.versionItem(c, i, now, loc)
				}
			})
			ui.Column(c).Gap(8).Padding(12, 16, 16, 16).BorderWidth(1, 0, 0, 0).BorderColor(p.Line).Children(func() {
				ui.Row(c).Gap(8).Children(func() {
					canRestore := !single && h.sel > 0
					rb := ui.ButtonBase(c.Key("restore")).Row().AlignItems(ui.Center).Justify(ui.Center).Grow(1).Height(32).Radius(8).
						Label(locales.RestoreVersion()).Disabled(!canRestore).DimDisabled(false)
					fg := a.acc.OnAccent
					if canRestore {
						rb.Background(a.acc.Accent)
						if rb.Hovered() {
							rb.Background(c.Theme().AccentHover)
						}
					} else {
						rb.Background(p.Badge)
						fg = p.Faint
					}
					rb.Children(func() {
						ui.Text(c, locales.RestoreVersion()).FontSize(13).FontWeight(500).TextColor(fg)
					})
					if rb.Clicked() {
						a.RestoreSelected(c.Now())
					}
					cb := ui.ButtonBase(c.Key("copyv")).Row().AlignItems(ui.Center).Justify(ui.Center).Grow(1).Height(32).Radius(8).Border(1, p.BtnLine).Background(p.Btn).
						Label(locales.CopyVersion()).Disabled(h.selected() == nil).DimDisabled(false)
					if cb.Hovered() {
						cb.Background(p.Chrome)
					}
					cfg := p.Text
					if h.selected() == nil {
						cfg = p.Faint
					}
					cb.Children(func() {
						ui.Text(c, locales.CopyVersion()).FontSize(13).TextColor(cfg)
					})
					if cb.Clicked() {
						a.CopyVersion()
						c.Announce(locales.CopiedAnnounce())
					}
				})
				ui.Text(c, locales.RestoreNote()).FontSize(11.5).LineHeight(17.0 / 11.5).TextColor(p.Muted)
			})
		})
}

func (a *App) versionItem(c *ui.Context, i int, now time.Time, loc string) {
	h := &a.history
	p := a.pal
	v := h.versions[i]
	sel := i == h.sel
	row := ui.ButtonBase(c.Key(v.ID)).Column().AlignItems(ui.Stretch).Gap(4).Padding(8, 12).Radius(8).
		Border(1, ui.Transparent).Role(ui.RoleOption).Selected(sel).PositionInSet(i+1, len(h.versions))
	switch {
	case sel:
		row.Background(p.Bg).Border(1, a.acc.SelLine)
	case row.Hovered():
		row.Background(p.Chrome)
	}
	when := versionTime(now, v.CreatedAt, loc)
	if i == 0 {
		when = locales.CurrentAt(when)
	}
	reason := h.reasonLabel(i, now, loc)
	delta := locales.LinesChanged(v.LinesChanged)
	if i == len(h.versions)-1 {
		delta = locales.FirstLines(lineCount(v.Body))
	}
	row.Label(when + ", " + reason + ", " + delta)
	row.Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, when).FontSize(13).FontWeight(500).TextColor(p.Text).FontFeatures("tnum").NoWrap()
			ui.Text(c, reason).FontSize(11).TextColor(p.Muted).Background(p.Badge).Radius(4).Padding(1, 4).NoWrap()
			ui.Spacer(c)
			ui.Text(c, delta).FontSize(11.5).TextColor(p.Muted).NoWrap()
		})
		if v.peek != "" {
			ui.Text(c, v.peek).FontSize(11.5).Font(monoFont).FontFeatures(monoFeatures...).TextColor(p.Muted).SingleLine()
		}
	})
	if row.Clicked() {
		h.sel = i
	}
}
