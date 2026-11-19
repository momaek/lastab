package appui

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/momaek/lastab/internal/store"
	"github.com/momaek/lastab/locales"
)

// renaming is a title being renamed in place: where (the editor's header
// or the library's preview) and for which snippet.
type renaming struct {
	where   string
	id      string
	text    string
	focused bool // the field has had the focus, so losing it saves
}

// startRename edits the title of snippet id in place, starting from title.
func (a *App) startRename(where, id, title string) {
	a.rename = renaming{where: where, id: id, text: title}
}

// renameField builds the field, if this is where a rename goes on, and
// reports whether it did. Enter or leaving the field saves; Escape cancels.
func (a *App) renameField(c *ui.Context, where, id string, size float32) bool {
	r := &a.rename
	if r.where != where || r.id != id {
		return false
	}
	p := a.pal
	in := ui.TextInput(c.Key("rename"), &r.text).AutoFocus().Grow(1).Shrink(1).
		FontSize(size).FontWeight(600).Placeholder(locales.RenamePlaceholder()).Label(locales.RenameTitle()).
		Padding(2, 6).Radius(6).Border(1.5, a.acc.Accent).Background(p.Bg)
	switch {
	case in.Shortcut(0, ui.KeyEscape):
		a.rename = renaming{}
	case in.Submitted():
		a.commitRename()
	case in.Focused():
		r.focused = true
	case r.focused:
		a.commitRename() // clicked elsewhere
	}
	return true
}

// renameButton is the pencil beside a title.
func (a *App) renameButton(c *ui.Context, where, id, title string) {
	if id == "" || a.rename.where != "" {
		return
	}
	if a.iconButton(c, "rename", iconRename, locales.RenameTitle(), "").Clicked() {
		a.startRename(where, id, title)
	}
}

// commitRename saves the title being edited. An empty title goes back to
// the automatic one.
func (a *App) commitRename() {
	r := a.rename
	a.rename = renaming{}
	if r.id == "" {
		return
	}
	text := strings.TrimSpace(r.text)
	svc := a.d.Service
	a.d.Async(func() func() {
		if err := svc.Rename(a.ctx(), r.id, text); err != nil {
			a.d.Log.Error("rename", "err", err)
			return nil
		}
		title, src, err := svc.UpdateTitle(a.ctx(), r.id)
		if err != nil {
			return nil
		}
		return func() { a.applyTitle(r.id, title, src) }
	})
}

// RefreshTitles reads the stored titles of the open tabs again, and has All
// snippets query again, so that a rename made outside the window, by an
// agent through `lastab retitle`, shows. The shell calls it when the window
// gets the focus back.
func (a *App) RefreshTitles() {
	if a.lib.open {
		a.lib.stale = true
	}
	var ids []string
	for _, t := range a.tabs {
		if t.id != "" {
			ids = append(ids, t.id)
		}
	}
	if len(ids) == 0 {
		return
	}
	st := a.d.Service.Store
	a.d.Async(func() func() {
		var got []store.Snippet
		for _, id := range ids {
			if sn, err := st.Get(a.ctx(), id); err == nil {
				got = append(got, sn)
			}
		}
		return func() {
			for _, sn := range got {
				a.applyTitle(sn.ID, sn.Title, sn.TitleSource)
			}
		}
	})
}

// maskAutoTitles masks, off the main thread, secrets an older Lastab let
// into automatic titles, and shows the masked titles.
func (a *App) maskAutoTitles() {
	svc := a.d.Service
	a.d.Async(func() func() {
		changed, err := svc.MaskAutoTitles(a.ctx())
		if err != nil {
			a.d.Log.Error("mask titles", "err", err)
		}
		if len(changed) == 0 {
			return nil
		}
		return func() {
			for id, t := range changed {
				a.applyTitle(id, t, store.TitleAuto)
			}
		}
	})
}

// applyTitle shows a snippet's new title wherever it shows.
func (a *App) applyTitle(id, title string, src store.TitleSource) {
	for _, t := range a.tabs {
		if t.id == id {
			t.title, t.titleSource = title, src
		}
	}
	for i := range a.lib.items {
		if a.lib.items[i].ID == id {
			a.lib.items[i].Title, a.lib.items[i].TitleSource = title, src
		}
	}
}
