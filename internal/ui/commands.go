package appui

import (
	"time"

	"github.com/momaek/lastab/internal/store"
	"github.com/momaek/lastab/internal/ui/keys"
)

// Run performs a shortcut's action for what the window shows: menus and
// window shortcuts both come here, so a key means the same either way
// (menus take keys before the window does).
func (a *App) Run(action keys.Action, now time.Time) {
	switch {
	case action == keys.ToggleSidebar:
		a.ToggleSidebar()
		return
	case action == keys.Settings:
		if a.prefs.open {
			a.CloseSettings()
		} else {
			a.OpenSettings(-1)
		}
		return
	case a.prefs.open:
		switch action {
		case keys.CloseTab:
			a.CloseSettings()
			return
		case keys.NewText, keys.NewTodo, keys.AllSnippets, keys.Recent, keys.History:
			a.CloseSettings()
		case keys.CopyAll:
			return
		}
	}
	if a.history.open {
		switch action {
		case keys.History, keys.CloseTab:
			a.CloseHistory()
			return
		case keys.NewText, keys.NewTodo, keys.AllSnippets, keys.Recent:
			a.CloseHistory()
		}
	}
	if a.lib.open {
		switch action {
		case keys.NewText:
			a.CloseLibrary()
			a.NewTab(store.KindText)
		case keys.NewTodo:
			a.CloseLibrary()
			a.NewTab(store.KindTodo)
		case keys.AllSnippets, keys.CloseTab:
			a.CloseLibrary()
		case keys.CopyAll:
			a.CopySelected(now)
		case keys.Recent:
			a.CloseLibrary()
			a.OpenSwitcher()
		case keys.History:
			a.HistoryOfSelected()
		}
		return
	}
	switch action {
	case keys.NewText:
		a.NewTab(store.KindText)
	case keys.NewTodo:
		a.NewTab(store.KindTodo)
	case keys.CloseTab:
		a.CloseActive()
	case keys.CopyAll:
		a.CopyAll(now)
	case keys.Recent:
		if !a.switcher.open {
			a.OpenSwitcher()
		}
	case keys.AllSnippets:
		a.OpenLibrary()
	case keys.History:
		a.OpenHistory()
	}
}
