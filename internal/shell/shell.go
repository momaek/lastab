// Package shell connects Lastab to the desktop through MyGo: the menu bar,
// the global shortcut, staying open with no window, and a single instance.
package shell

import (
	"log/slog"

	"github.com/egoist/mygo"

	"github.com/momaek/lastab/internal/store"
	appui "github.com/momaek/lastab/internal/ui"
	"github.com/momaek/lastab/internal/ui/keys"
	"github.com/momaek/lastab/locales"
)

// Shell holds the main window and the app state it shows.
type Shell struct {
	App *appui.App
	Win *mygo.Window
	Log *slog.Logger

	quitting bool
}

// do runs fn on the main thread against the window's state, then draws.
func (s *Shell) do(fn func()) {
	if s.Win == nil {
		return
	}
	s.Win.Update(fn)
}

// Show brings the window forward.
func (s *Shell) Show() {
	if s.Win == nil {
		return
	}
	s.Win.Show()
	s.Win.Focus()
	mygo.App.Focus()
}

// Summon shows the window with a new blank snippet, for the global
// shortcut: the app is already running, so it is instant.
func (s *Shell) Summon() {
	s.do(func() { s.App.NewTab(store.KindText) })
	s.Show()
}

// Menu returns the menu bar. The Window menu holds only roles that leave
// Cmd+` free, which the recent switcher takes.
func (s *Shell) Menu() *mygo.Menu {
	item := func(label string, a keys.Action, fn func()) *mygo.MenuItem {
		return &mygo.MenuItem{Label: label, Accelerator: keys.Default(a), Click: func(*mygo.MenuItem, *mygo.Window) {
			s.do(fn)
		}}
	}
	return mygo.NewMenu([]*mygo.MenuItem{
		{Role: mygo.RoleAppMenu},
		{Label: locales.FileMenu(), Submenu: []*mygo.MenuItem{
			item(locales.NewSnippetMenu(), keys.NewText, func() { s.App.NewTab(store.KindText) }),
			item(locales.NewTodoMenu(), keys.NewTodo, func() { s.App.NewTab(store.KindTodo) }),
			mygo.Separator(),
			item(locales.CopyAllMenu(), keys.CopyAll, func() { s.App.CopyAll(nowFrame()) }),
			mygo.Separator(),
			item(locales.CloseSnippetMenu(), keys.CloseTab, s.App.CloseActive),
		}},
		{Role: mygo.RoleEditMenu},
		{Label: locales.GoMenu(), Submenu: []*mygo.MenuItem{
			item(locales.RecentMenu(), keys.Recent, s.App.OpenSwitcher),
		}},
		{Label: "Window", Submenu: []*mygo.MenuItem{
			{Role: mygo.RoleMinimize},
			{Role: mygo.RoleZoom},
			{Role: mygo.RoleToggleFullScreen},
		}},
	})
}

// Install sets the menu, the global shortcut and the app's lifecycle:
// closing the window hides it, the app stays open, a second launch shows
// the window, and quitting writes every unsaved body first.
func (s *Shell) Install(onQuit func()) {
	mygo.App.SetMenu(s.Menu())
	mygo.App.OnLocaleChanged(func(string) { mygo.App.SetMenu(s.Menu()) })

	if err := mygo.GlobalShortcut.Register(keys.Default(keys.Summon), s.Summon); err != nil {
		s.Log.Warn("global shortcut unavailable", "accelerator", keys.Default(keys.Summon), "err", err)
	}
	mygo.App.OnWindowAllClosed(func() {}) // stay resident
	mygo.App.OnSecondInstance(func([]string, string) { s.Show() })
	mygo.App.OnActivate(func(hasVisible bool) {
		if !hasVisible {
			s.Show()
		}
	})
	s.Win.OnClose(func(e *mygo.CloseEvent) {
		if s.quitting {
			return
		}
		e.PreventDefault()
		s.App.FlushAll()
		s.Win.Hide()
	})
	s.Win.OnBlur(func() { s.App.FlushAll() })
	mygo.App.OnBeforeQuit(func(*mygo.QuitEvent) {
		s.quitting = true
		s.App.FlushAll()
		if onQuit != nil {
			onQuit()
		}
	})
}
