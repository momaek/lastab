package appui

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/momaek/lastab/internal/install"
	"github.com/momaek/lastab/locales"
)

// installRow is the state of an item of Settings > Agent access > Install:
// the lastab command or the Claude Code skill. Nothing is written until the
// user clicks; a failure stays on the row with its reason and Retry.
type installRow struct {
	st     install.Status
	busy   bool
	err    string // why the last install failed, shown with Retry
	admin  bool   // the last install asked for an administrator
	justOK bool   // installed by a click on this page
}

// Which items installItem draws.
const (
	itemCLI = iota
	itemSkill
)

func (a *App) loadInstallStatus() {
	plat := a.d.Platform
	if plat == nil {
		return
	}
	a.d.Async(func() func() {
		cli, sk := plat.CLIStatus(), plat.SkillStatus()
		return func() {
			if !a.prefs.cli.busy {
				a.prefs.cli.st = cli
			}
			if !a.prefs.skill.busy {
				a.prefs.skill.st = sk
			}
		}
	})
}

// InstallCLI puts the lastab command on the PATH, off the main thread; admin
// asks for an administrator's password to write /usr/local/bin (macOS).
func (a *App) InstallCLI(admin bool) {
	a.runInstall(itemCLI, admin)
}

// InstallSkill writes the Claude Code skill, off the main thread.
func (a *App) InstallSkill() {
	a.runInstall(itemSkill, false)
}

func (a *App) rowOf(item int) *installRow {
	if item == itemCLI {
		return &a.prefs.cli
	}
	return &a.prefs.skill
}

func (a *App) runInstall(item int, admin bool) {
	plat := a.d.Platform
	r := a.rowOf(item)
	if plat == nil || r.busy {
		return
	}
	r.busy, r.err, r.admin, r.justOK = true, "", admin, false
	a.d.Async(func() func() {
		var st install.Status
		var err error
		if item == itemCLI {
			st, err = plat.InstallCLI(admin)
		} else {
			st, err = plat.InstallSkill()
		}
		return func() {
			r := a.rowOf(item)
			r.busy, r.st = false, st
			switch {
			case err == nil:
				r.justOK = true
			case errors.Is(err, install.ErrCanceled):
				// The user closed the password dialog: nothing to report.
			default:
				r.err = installErrorText(err)
			}
		}
	})
}

// installErrorText says why an install failed, and what to do.
func installErrorText(err error) string {
	var de *install.DirError
	var ee *install.ExistsError
	switch {
	case errors.Is(err, install.ErrNoCLI):
		return locales.InstallNoCLI()
	case errors.Is(err, install.ErrNotInstalled):
		return locales.InstallMoveApp()
	case errors.As(err, &ee):
		return locales.InstallExists(displayPath(ee.Path))
	case errors.As(err, &de) && errors.Is(err, fs.ErrPermission):
		return locales.InstallNoPermission(displayPath(de.Dir))
	case errors.As(err, &de):
		return locales.InstallFailed(displayPath(de.Dir) + ": " + de.Err.Error())
	}
	return locales.InstallFailed(err.Error())
}

// displayPath writes a path as the user would: ~/.claude/… on macOS,
// %LOCALAPPDATA%\… or %USERPROFILE%\… on Windows.
func displayPath(p string) string {
	if p == "" {
		return ""
	}
	if runtime.GOOS == "windows" {
		for _, v := range []string{"LOCALAPPDATA", "USERPROFILE"} {
			if dir := os.Getenv(v); dir != "" && hasDirPrefix(p, dir) {
				return "%" + v + "%" + p[len(dir):]
			}
		}
		return p
	}
	if home, err := os.UserHomeDir(); err == nil && hasDirPrefix(p, home) {
		return "~" + p[len(home):]
	}
	return p
}

func hasDirPrefix(p, dir string) bool {
	dir = strings.TrimRight(dir, `\/`)
	if len(p) <= len(dir) || !strings.EqualFold(p[:len(dir)], dir) {
		return false
	}
	return p[len(dir)] == filepath.Separator || p[len(dir)] == '/'
}

// agentInstall is the Install group of Agent access: the command and the
// skill, each with its state and button, then how other agents call it.
func (a *App) agentInstall(c *ui.Context) {
	p := a.pal
	pg := &a.prefs
	if !pg.installLoaded {
		pg.installLoaded = true
		a.loadInstallStatus()
	}
	ui.Column(c.Key("install")).AlignItems(ui.Stretch).Gap(8).Margin(8, 0, 0, 0).Children(func() {
		ui.Text(c, locales.InstallHeading()).FontSize(13).FontWeight(600).Role(ui.RoleHeading)
		ui.Column(c.Key("items")).AlignItems(ui.Stretch).Radius(12).Border(1, p.Line).Children(func() {
			a.installItem(c, itemCLI)
			a.installItem(c, itemSkill)
		})
		ui.Column(c).Gap(4).Children(func() {
			ui.Row(c).Gap(4).AlignItems(ui.Center).Wrap().Children(func() {
				ui.Text(c, locales.AgentOther()).FontSize(12).TextColor(p.Muted)
				ui.Text(c, `lastab search "curl staging" --since 1d --json`).Font(monoFont).FontSize(12).
					TextColor(p.Text).Selectable()
			})
			ui.Text(c, locales.AgentJSONNote()).FontSize(12).LineHeight(1.5).TextColor(p.Muted)
		})
	})
}

func (a *App) installItem(c *ui.Context, item int) {
	p := a.pal
	r := a.rowOf(item)
	cli := item == itemCLI
	key, title, path := "skill", "Claude Code Skill", displayPath(r.st.Path)
	if cli {
		key, title = "cli", locales.CliTool()
		if v := a.appVersion(); v != "" && path != "" {
			path += " · v" + v
		}
	}
	retry := func() { a.runInstall(item, r.admin) }
	col := ui.Column(c.Key(key)).AlignItems(ui.Stretch).Gap(8).Padding(12, 16).Role(ui.RoleGroup).Label(title)
	if cli {
		col.BorderWidth(0, 0, 1, 0).BorderColor(p.LineSoft)
	}
	col.Children(func() {
		ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(2).Children(func() {
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					ui.Text(c, title).FontSize(13).FontWeight(500)
					if cli {
						ui.Text(c, "lastab").Font(monoFont).FontSize(13)
					}
				})
				ui.Text(c, path).Font(monoFont).FontSize(11.5).TextColor(p.Muted).Selectable()
			})
			switch {
			case r.busy:
				a.installButton(c, "busy", locales.Installing(), true)
			case r.err != "":
				// The alert below holds Retry.
			case r.st.State == install.Installed:
				ui.Text(c, locales.Installed()).FontSize(11.5).LineHeight(20.0/11.5).Padding(0, 8).Radius(4).
					Background(p.OkBg).TextColor(p.OkText).Role(ui.RoleStatus)
				if cli && a.outlineButton(c, "reinstall", locales.Reinstall(), "").Height(28).Clicked() {
					a.runInstall(item, false)
				}
			case r.st.State == install.Outdated:
				if a.outlineButton(c, "reinstall", locales.Reinstall(), "").Height(28).Clicked() {
					a.runInstall(item, false)
				}
			default:
				label := locales.InstallSkill()
				if cli {
					label = locales.InstallCLI()
				}
				if a.installButton(c, "install", label, false).Clicked() {
					a.runInstall(item, false)
				}
			}
		})
		if r.busy {
			return
		}
		note := func(s string) {
			ui.Text(c, s).FontSize(12).LineHeight(1.5).TextColor(p.Muted)
		}
		switch {
		case r.err != "":
			ui.Row(c.Key("failed")).Gap(8).AlignItems(ui.Center).Padding(8, 12).Radius(4).Background(p.DangerBg).
				Role(ui.RoleAlert).Children(func() {
				ui.Text(c, "ⓘ").FontSize(13).TextColor(p.Danger).AccessHidden(true)
				ui.Text(c, r.err).FontSize(12.5).LineHeight(1.5).TextColor(p.Danger).Grow(1).Shrink(1).MinWidth(0)
				if a.outlineButton(c, "retry", locales.Retry(), "").Height(28).Clicked() {
					retry()
				}
			})
		case r.st.State == install.Outdated && cli:
			note(locales.CliOutdated())
		case r.st.State == install.Outdated:
			note(locales.SkillOutdated())
		case cli && r.st.State == install.Installed && r.st.NeedsPath:
			ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
				ui.Text(c, locales.CliNeedsPath(displayPath(filepath.Dir(r.st.Path)))).FontSize(12).LineHeight(1.5).
					TextColor(p.Muted).Grow(1).Shrink(1).MinWidth(0)
				if runtime.GOOS == "darwin" && a.outlineButton(c, "admin", locales.InstallAsAdmin(), "").Height(28).Clicked() {
					a.runInstall(item, true)
				}
			})
		case cli && r.justOK && runtime.GOOS == "windows":
			note(locales.CliNewTerminal())
		}
	})
}

// installButton is the accent button of an item, with a spinner while it
// installs.
func (a *App) installButton(c *ui.Context, key, label string, busy bool) ui.Element {
	b := ui.ButtonBase(c.Key(key)).Row().Gap(8).AlignItems(ui.Center).Justify(ui.Center).MinWidth(120).Height(28).
		Padding(0, 12).Radius(8).Background(a.acc.Accent).Label(label).Busy(busy).Disabled(busy).DimDisabled(false)
	if busy {
		b.Opacity(0.8)
	}
	b.Children(func() {
		if busy {
			ui.Spinner(c).FontSize(12).TextColor(a.acc.OnAccent)
		}
		ui.Text(c, label).FontSize(12).FontWeight(500).TextColor(a.acc.OnAccent)
	})
	return b
}

// appVersion is the app's version, the first part of Platform.Version.
func (a *App) appVersion() string {
	if a.d.Platform == nil {
		return ""
	}
	v, _, _ := strings.Cut(a.d.Platform.Version(), " · ")
	return v
}
