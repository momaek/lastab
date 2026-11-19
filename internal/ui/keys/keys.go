// Package keys holds Lastab's default shortcuts, in MyGo's accelerator
// syntax, and formats them for display: ⌘ ⌥ ⇧ ⌃ on macOS, Ctrl, Alt and
// Shift words on Windows. Menus, Settings and the hint overlay read this
// one table.
package keys

import (
	"runtime"
	"strings"
)

// Action names a command with a shortcut.
type Action string

const (
	NewText     Action = "newText"
	NewTodo     Action = "newTodo"
	Summon      Action = "summon" // global
	Recent      Action = "recent"
	AllSnippets Action = "allSnippets"
	CopyAll     Action = "copyAll"
	History     Action = "history"
	CloseTab    Action = "closeTab"
	// ToggleSidebar shows or hides the list of open snippets.
	ToggleSidebar Action = "toggleSidebar"
)

// Defaults returns the default accelerators for goos.
func Defaults(goos string) map[Action]string {
	m := map[Action]string{
		NewText:       "CmdOrCtrl+N",
		NewTodo:       "CmdOrCtrl+T",
		Summon:        "Ctrl+Alt+N",
		Recent:        "CmdOrCtrl+`",
		AllSnippets:   "CmdOrCtrl+K",
		CopyAll:       "CmdOrCtrl+Shift+C",
		History:       "Cmd+Y",
		CloseTab:      "CmdOrCtrl+W",
		ToggleSidebar: "CmdOrCtrl+\\",
	}
	if goos != "darwin" {
		// Ctrl+Y is the editor's Redo on Windows.
		m[History] = "Ctrl+Shift+Y"
	}
	return m
}

// Default is Defaults for this platform.
func Default(a Action) string { return Defaults(runtime.GOOS)[a] }

// Display formats an accelerator for goos: "⌘⇧C" on macOS, "Ctrl+Shift+C"
// elsewhere.
func Display(acc, goos string) string {
	parts := strings.Split(acc, "+")
	mac := goos == "darwin"
	var mods []string
	key := ""
	for i, p := range parts {
		if i == len(parts)-1 {
			key = p
			break
		}
		switch strings.ToLower(p) {
		case "cmdorctrl", "commandorcontrol":
			if mac {
				mods = append(mods, "⌘")
			} else {
				mods = append(mods, "Ctrl")
			}
		case "cmd", "command", "super", "meta":
			if mac {
				mods = append(mods, "⌘")
			} else {
				mods = append(mods, "Win")
			}
		case "ctrl", "control":
			if mac {
				mods = append(mods, "⌃")
			} else {
				mods = append(mods, "Ctrl")
			}
		case "alt", "option":
			if mac {
				mods = append(mods, "⌥")
			} else {
				mods = append(mods, "Alt")
			}
		case "shift":
			if mac {
				mods = append(mods, "⇧")
			} else {
				mods = append(mods, "Shift")
			}
		}
	}
	if mac {
		// macOS orders modifiers ⌃⌥⇧⌘.
		order := map[string]int{"⌃": 0, "⌥": 1, "⇧": 2, "⌘": 3}
		for i := 1; i < len(mods); i++ {
			for j := i; j > 0 && order[mods[j]] < order[mods[j-1]]; j-- {
				mods[j], mods[j-1] = mods[j-1], mods[j]
			}
		}
		switch strings.ToLower(key) {
		case "enter", "return":
			key = "↵"
		case "backspace":
			key = "⌫"
		case "up":
			key = "↑"
		case "down":
			key = "↓"
		}
		return strings.Join(mods, "") + strings.ToUpper(key)
	}
	return strings.Join(append(mods, strings.ToUpper(key)), "+")
}

// Settings opens the settings page. It is fixed, not customizable.
const Settings Action = "settings"

// Customizable lists the actions Settings shows, in its order.
var Customizable = []Action{NewText, NewTodo, Summon, Recent, AllSnippets, CopyAll, History, CloseTab, ToggleSidebar}

// IsGlobal reports whether an action's shortcut works from any app.
func IsGlobal(a Action) bool { return a == Summon }

// Table returns the accelerators in effect for goos: the defaults, with
// the user's own over them.
func Table(goos string, custom map[Action]string) map[Action]string {
	m := Defaults(goos)
	for a, acc := range custom {
		if _, ok := m[a]; ok && acc != "" {
			m[a] = acc
		}
	}
	m[Settings] = "CmdOrCtrl+,"
	return m
}

// Canonical writes an accelerator in one form for goos, so two that press
// the same keys compare equal: CmdOrCtrl resolves to Cmd on macOS and Ctrl
// elsewhere, modifiers come in a fixed order, the key in upper case.
func Canonical(acc, goos string) string {
	parts := strings.Split(acc, "+")
	if len(parts) == 0 {
		return ""
	}
	key := strings.ToUpper(parts[len(parts)-1])
	if parts[len(parts)-1] == "" && len(parts) >= 2 { // "Ctrl++"
		key = "+"
	}
	has := map[string]bool{}
	for _, p := range parts[:len(parts)-1] {
		switch strings.ToLower(p) {
		case "cmdorctrl", "commandorcontrol":
			if goos == "darwin" {
				has["Cmd"] = true
			} else {
				has["Ctrl"] = true
			}
		case "cmd", "command", "super", "meta":
			has["Cmd"] = true
		case "ctrl", "control":
			has["Ctrl"] = true
		case "alt", "option":
			has["Alt"] = true
		case "shift":
			has["Shift"] = true
		}
	}
	var out []string
	for _, m := range []string{"Ctrl", "Alt", "Shift", "Cmd"} {
		if has[m] {
			out = append(out, m)
		}
	}
	return strings.Join(append(out, key), "+")
}

// Conflict returns the action other than a whose accelerator in table
// presses the same keys as acc.
func Conflict(table map[Action]string, a Action, acc, goos string) (Action, bool) {
	want := Canonical(acc, goos)
	for _, other := range append(append([]Action{}, Customizable...), Settings) {
		if other != a && Canonical(table[other], goos) == want {
			return other, true
		}
	}
	return "", false
}

// HasModifier reports whether acc holds a modifier other than Shift, as an
// app's shortcut must, so it never takes plain typing.
func HasModifier(acc, goos string) bool {
	c := Canonical(acc, goos)
	return strings.Contains(c, "Ctrl+") || strings.Contains(c, "Alt+") || strings.Contains(c, "Cmd+")
}
