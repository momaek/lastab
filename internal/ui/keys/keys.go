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
