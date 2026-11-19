// Package shell connects Lastab to the desktop through MyGo: the menu bar,
// the global shortcut, staying open with no window, a single instance, and
// the settings that act outside the window (appearance, language, login).
package shell

import (
	"fmt"
	"log/slog"
	"runtime"
	"time"

	"github.com/egoist/mygo"

	appui "github.com/momaek/lastab/internal/ui"
	"github.com/momaek/lastab/internal/ui/keys"
	"github.com/momaek/lastab/locales"
)

// Shell holds the main window and the app state it shows. It is the
// app's appui.Platform.
type Shell struct {
	App *appui.App
	Win *mygo.Window
	Log *slog.Logger

	quitting  bool
	table     map[keys.Action]string // in the menus
	global    string                 // the summon shortcut registered
	suspended bool
}

var _ appui.Platform = (*Shell)(nil)

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
	s.do(func() { s.App.Run(keys.NewText, time.Now()) })
	s.Show()
}

// Menu returns the menu bar for a shortcut table. The Window menu holds
// only roles that leave Cmd+` free, which the recent switcher takes.
func (s *Shell) Menu(table map[keys.Action]string) *mygo.Menu {
	item := func(label string, a keys.Action) *mygo.MenuItem {
		acc := table[a]
		if s.suspended {
			acc = ""
		}
		return &mygo.MenuItem{Label: label, Accelerator: acc, Click: func(*mygo.MenuItem, *mygo.Window) {
			s.do(func() { s.App.Run(a, time.Now()) })
		}}
	}
	settings := item(locales.SettingsMenu(), keys.Settings)
	file := []*mygo.MenuItem{
		item(locales.NewSnippetMenu(), keys.NewText),
		item(locales.NewTodoMenu(), keys.NewTodo),
		mygo.Separator(),
		item(locales.CopyAllMenu(), keys.CopyAll),
		mygo.Separator(),
		item(locales.CloseSnippetMenu(), keys.CloseTab),
	}
	var app *mygo.MenuItem
	if runtime.GOOS == "darwin" {
		// The app menu, with Settings… where Mac apps have it.
		app = &mygo.MenuItem{Label: "Lastab", Submenu: []*mygo.MenuItem{
			{Role: mygo.RoleAbout}, mygo.Separator(), settings, mygo.Separator(),
			{Role: mygo.RoleServices}, mygo.Separator(),
			{Role: mygo.RoleHide}, {Role: mygo.RoleHideOthers}, {Role: mygo.RoleUnhide}, mygo.Separator(),
			{Role: mygo.RoleQuit},
		}}
	} else {
		file = append(file, mygo.Separator(), settings, mygo.Separator(), &mygo.MenuItem{Role: mygo.RoleQuit})
	}
	items := []*mygo.MenuItem{}
	if app != nil {
		items = append(items, app)
	}
	items = append(items,
		&mygo.MenuItem{Label: locales.FileMenu(), Submenu: file},
		&mygo.MenuItem{Role: mygo.RoleEditMenu},
		&mygo.MenuItem{Label: locales.ViewMenu(), Submenu: []*mygo.MenuItem{
			item(locales.ToggleSidebarMenu(), keys.ToggleSidebar),
		}},
		&mygo.MenuItem{Label: locales.GoMenu(), Submenu: []*mygo.MenuItem{
			item(locales.RecentMenu(), keys.Recent),
			item(locales.AllSnippetsMenu(), keys.AllSnippets),
			item(locales.HistoryMenu(), keys.History),
		}},
		// Not RoleWindowMenu: its "cycle through windows" item takes Cmd+`.
		&mygo.MenuItem{Label: locales.WindowMenu(), Submenu: []*mygo.MenuItem{
			{Role: mygo.RoleMinimize},
			{Role: mygo.RoleZoom},
			{Role: mygo.RoleToggleFullScreen},
		}},
	)
	return mygo.NewMenu(items)
}

// ApplyShortcuts puts a table in the menus and registers its global
// shortcut, keeping the one before when the system refuses the new one.
func (s *Shell) ApplyShortcuts(table map[keys.Action]string) error {
	s.table = table
	mygo.App.SetMenu(s.Menu(table))
	want := table[keys.Summon]
	if s.suspended || want == s.global {
		return nil
	}
	prev := s.global
	if prev != "" {
		mygo.GlobalShortcut.Unregister(prev)
		s.global = ""
	}
	if err := mygo.GlobalShortcut.Register(want, s.Summon); err != nil {
		if prev != "" && mygo.GlobalShortcut.Register(prev, s.Summon) == nil {
			s.global = prev
		}
		return fmt.Errorf("global shortcut %s: %w", want, err)
	}
	s.global = want
	return nil
}

// SuspendShortcuts takes the menus' and the global shortcut's keys away
// while Settings records a new shortcut, so the recorder gets them.
func (s *Shell) SuspendShortcuts(suspend bool) {
	if s.suspended == suspend {
		return
	}
	s.suspended = suspend
	if suspend {
		mygo.App.SetMenu(s.Menu(s.table))
		if s.global != "" {
			mygo.GlobalShortcut.Unregister(s.global)
			s.global = ""
		}
		return
	}
	if err := s.ApplyShortcuts(s.table); err != nil {
		s.Log.Warn("shortcuts", "err", err)
	}
}

// SetAppearance follows the system, or fixes light or dark.
func (s *Shell) SetAppearance(mode string) {
	mygo.Theme.SetSource(themeSource(mode))
}

func themeSource(mode string) mygo.ThemeSource {
	switch mode {
	case "light":
		return mygo.ThemeLight
	case "dark":
		return mygo.ThemeDark
	}
	return mygo.ThemeSystem
}

// SetLanguage shows a language, or follows the user's with "". MyGo keeps
// the choice for the next launches.
func (s *Shell) SetLanguage(tag string) { mygo.App.SetLocale(tag) }

// OpenAtLogin reports whether the system starts Lastab at login.
func (s *Shell) OpenAtLogin() bool { return mygo.App.OpenAtLogin() }

// SetOpenAtLogin has the system start Lastab at login, or not.
func (s *Shell) SetOpenAtLogin(on bool) error { return mygo.App.SetOpenAtLogin(on) }

// ShowInFolder reveals a file in Finder or Explorer.
func (s *Shell) ShowInFolder(path string) { mygo.Shell.ShowItemInFolder(path) }

// Version is Lastab's version and the framework's.
func (s *Shell) Version() string {
	return fmt.Sprintf("%s · MyGo %s", mygo.App.Version(), mygo.Version)
}

// Install sets the menu, the global shortcut and the app's lifecycle:
// closing the window hides it, the app stays open, a second launch shows
// the window, and quitting writes every unsaved body first.
func (s *Shell) Install(onQuit func()) {
	if err := s.ApplyShortcuts(s.App.Shortcuts()); err != nil {
		s.Log.Warn("global shortcut unavailable", "err", err)
	}
	mygo.App.OnLocaleChanged(func(string) { mygo.App.SetMenu(s.Menu(s.table)) })

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
	// Coming back to the window is a settle point: the sidebar reorders.
	s.Win.OnFocus(func() { s.do(s.App.Resort) })
	mygo.App.OnBeforeQuit(func(*mygo.QuitEvent) {
		s.quitting = true
		s.App.FlushAll()
		if onQuit != nil {
			onQuit()
		}
	})
}

// ThemeSource is the source to set before the app runs, for a saved
// appearance, so the window opens in it without a flash.
func ThemeSource(mode string) mygo.ThemeSource { return themeSource(mode) }
