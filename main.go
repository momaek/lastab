// Command Lastab is the desktop app: a resident window of plain-text
// snippets that keeps everything typed into it.
package main

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"gopkg.in/natefinch/lumberjack.v2"

	"github.com/momaek/lastab/internal/paths"
	"github.com/momaek/lastab/internal/shell"
	"github.com/momaek/lastab/internal/snippet"
	"github.com/momaek/lastab/internal/store"
	appui "github.com/momaek/lastab/internal/ui"
	_ "github.com/momaek/lastab/locales"
)

func main() {
	if !mygo.App.RequestSingleInstanceLock() {
		return // the running instance shows its window (OnSecondInstance)
	}

	dir, err := paths.DataDir("", mygo.IsDev())
	if err == nil {
		_, err = paths.Ensure(dir)
	}
	if err != nil {
		fatal(nil, "data directory", err)
	}
	log := newLogger(dir)

	st, err := store.Open(filepath.Join(dir, paths.DBFile))
	if err != nil {
		fatal(log, "open database", err)
	}
	session, crashed, err := snippet.StartSession(dir)
	if err != nil {
		log.Warn("session lock", "err", err)
	}
	svc := snippet.New(st)

	// Appearance is set before Run, so the window opens in it: no flash.
	var appearance, accent string
	st.GetSetting(context.Background(), store.SettingAppearance, &appearance)
	st.GetSetting(context.Background(), store.SettingAccent, &accent)
	switch appearance {
	case "light":
		mygo.Theme.SetSource(mygo.ThemeLight)
	case "dark":
		mygo.Theme.SetSource(mygo.ThemeDark)
	}

	sh := &shell.Shell{Log: log}
	app := appui.New(appui.Deps{
		Service:   svc,
		Clipboard: clipboard{},
		Recovered: crashed,
		Accent:    accent,
		Log:       log,
		Async: func(work func() func()) {
			go func() {
				apply := work()
				if apply != nil && sh.Win != nil {
					sh.Win.Update(apply)
				}
			}()
		},
	})
	if err := app.Load(); err != nil {
		log.Error("restore workspace", "err", err)
	}
	sh.App = app

	mygo.App.WhenReady(func() {
		sh.Win = mygo.NewWindow(mygo.WindowOptions{
			Title:         "Lastab",
			Width:         960,
			Height:        640,
			MinWidth:      520,
			MinHeight:     320,
			StateKey:      "main",
			TitleBarStyle: mygo.TitleBarHidden,
			Content:       ui.View(app.View),
		})
		sh.Install(func() {
			session.End()
			st.Close()
		})
	})
	if err := mygo.App.Run(); err != nil {
		fatal(log, "run", err)
	}
}

type clipboard struct{}

func (clipboard) WriteText(s string) { mygo.Clipboard.WriteText(s) }
func (clipboard) ReadText() string   { return mygo.Clipboard.ReadText() }

func newLogger(dataDir string) *slog.Logger {
	var w io.Writer = os.Stderr
	logDir := filepath.Join(dataDir, "logs")
	if os.MkdirAll(logDir, 0o700) == nil {
		w = io.MultiWriter(os.Stderr, &lumberjack.Logger{
			Filename: filepath.Join(logDir, "lastab.log"), MaxSize: 5, MaxBackups: 3,
		})
	}
	return slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{Level: slog.LevelInfo}))
}

func fatal(log *slog.Logger, what string, err error) {
	if log != nil {
		log.Error(what, "err", err)
	} else {
		slog.Error(what, "err", err)
	}
	os.Exit(1)
}
