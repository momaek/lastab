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
	if action == keys.ToggleSidebar {
		a.ToggleSidebar()
		return
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
	}
}
