package shell

import (
	_ "embed"
	"runtime"
	"time"

	"github.com/egoist/mygo"

	"github.com/momaek/lastab/internal/ui/keys"
	"github.com/momaek/lastab/locales"
)

//go:embed assets/tray-template.png
var trayTemplate []byte // macOS: black on clear, tinted by the menu bar

//go:embed assets/tray-win.png
var trayWin []byte

// SetTray shows or hides the icon in the menu bar (macOS) or the
// notification area (Windows). On Windows, a click shows the window, which
// closing hides; on macOS the icon opens its menu.
func (s *Shell) SetTray(on bool) {
	if !on {
		if s.tray != nil {
			s.tray.Destroy()
			s.tray = nil
		}
		return
	}
	if s.tray != nil {
		return
	}
	icon, template := trayWin, false
	if runtime.GOOS == "darwin" {
		icon, template = trayTemplate, true
	}
	t, err := mygo.NewTray(mygo.TrayOptions{
		ID: "lastab", Icon: icon, IconIsTemplate: template,
		ToolTip: "Lastab", Menu: s.trayMenu(),
	})
	if err != nil {
		s.Log.Warn("tray icon", "err", err)
		return
	}
	if runtime.GOOS == "windows" {
		t.OnClick(s.Show)
	}
	s.tray = t
}

func (s *Shell) trayMenu() *mygo.Menu {
	run := func(label string, a keys.Action) *mygo.MenuItem {
		return &mygo.MenuItem{Label: label, Click: func(*mygo.MenuItem, *mygo.Window) {
			s.do(func() { s.App.Run(a, time.Now()) })
			s.Show()
		}}
	}
	return mygo.NewMenu([]*mygo.MenuItem{
		{Label: locales.TrayShow(), Click: func(*mygo.MenuItem, *mygo.Window) { s.Show() }},
		run(locales.NewSnippetMenu(), keys.NewText),
		run(locales.AllSnippetsMenu(), keys.AllSnippets),
		mygo.Separator(),
		run(locales.SettingsMenu(), keys.Settings),
		mygo.Separator(),
		{Role: mygo.RoleQuit},
	})
}
