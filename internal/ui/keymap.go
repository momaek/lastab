package appui

import (
	"runtime"
	"strings"

	"github.com/egoist/mygo/ui"
)

var keyNames = map[string]ui.Key{
	"ENTER": ui.KeyEnter, "RETURN": ui.KeyEnter, "ESC": ui.KeyEscape, "ESCAPE": ui.KeyEscape,
	"BACKSPACE": ui.KeyBackspace, "TAB": ui.KeyTab, "SPACE": ui.KeySpace, "DELETE": ui.KeyDelete,
	"INSERT": ui.KeyInsert, "HOME": ui.KeyHome, "END": ui.KeyEnd, "PAGEUP": ui.KeyPageUp, "PAGEDOWN": ui.KeyPageDown,
	"LEFT": ui.KeyLeft, "RIGHT": ui.KeyRight, "UP": ui.KeyUp, "DOWN": ui.KeyDown,
	"F1": ui.KeyF1, "F2": ui.KeyF2, "F3": ui.KeyF3, "F4": ui.KeyF4, "F5": ui.KeyF5, "F6": ui.KeyF6,
	"F7": ui.KeyF7, "F8": ui.KeyF8, "F9": ui.KeyF9, "F10": ui.KeyF10, "F11": ui.KeyF11, "F12": ui.KeyF12,
	"-": ui.KeyMinus, "=": ui.KeyEqual, ",": ui.KeyComma, ".": ui.KeyPeriod, "/": ui.KeySlash,
	";": ui.KeySemicolon, "'": ui.KeyQuote, "[": ui.KeyBracketLeft, "]": ui.KeyBracketRight,
	"\\": ui.KeyBackslash, "`": ui.KeyBackquote,
	"0": ui.Key0, "1": ui.Key1, "2": ui.Key2, "3": ui.Key3, "4": ui.Key4,
	"5": ui.Key5, "6": ui.Key6, "7": ui.Key7, "8": ui.Key8, "9": ui.Key9,
	"A": ui.KeyA, "B": ui.KeyB, "C": ui.KeyC, "D": ui.KeyD, "E": ui.KeyE, "F": ui.KeyF, "G": ui.KeyG,
	"H": ui.KeyH, "I": ui.KeyI, "J": ui.KeyJ, "K": ui.KeyK, "L": ui.KeyL, "M": ui.KeyM, "N": ui.KeyN,
	"O": ui.KeyO, "P": ui.KeyP, "Q": ui.KeyQ, "R": ui.KeyR, "S": ui.KeyS, "T": ui.KeyT, "U": ui.KeyU,
	"V": ui.KeyV, "W": ui.KeyW, "X": ui.KeyX, "Y": ui.KeyY, "Z": ui.KeyZ,
}

var keyByCode = func() map[ui.Key]string {
	m := map[ui.Key]string{}
	for name, k := range keyNames {
		if prev, ok := m[k]; !ok || len(name) < len(prev) || (len(name) == len(prev) && name < prev) {
			m[k] = name
		}
	}
	return m
}()

// parseAccel turns an accelerator into the modifiers and key the window's
// shortcuts take. CmdOrCtrl is ui.Cmd: Command on macOS, Ctrl elsewhere.
func parseAccel(acc string) (ui.Modifiers, ui.Key, bool) {
	parts := strings.Split(acc, "+")
	if len(parts) < 1 {
		return 0, 0, false
	}
	name := strings.ToUpper(parts[len(parts)-1])
	k, ok := keyNames[name]
	if !ok {
		return 0, 0, false
	}
	var m ui.Modifiers
	for _, p := range parts[:len(parts)-1] {
		switch strings.ToLower(p) {
		case "cmdorctrl", "commandorcontrol":
			m |= ui.Cmd
		case "cmd", "command", "super", "meta":
			m |= ui.Super
		case "ctrl", "control":
			m |= ui.Ctrl
		case "alt", "option":
			m |= ui.Alt
		case "shift":
			m |= ui.Shift
		default:
			return 0, 0, false
		}
	}
	return m, k, true
}

// formatAccel writes a key pressed with modifiers as an accelerator. The
// platform's primary modifier is written CmdOrCtrl, so a table moves
// between macOS and Windows.
func formatAccel(m ui.Modifiers, k ui.Key) (string, bool) {
	name, ok := keyByCode[k]
	if !ok {
		return "", false
	}
	var parts []string
	primary := ui.Cmd
	if m&primary != 0 {
		parts = append(parts, "CmdOrCtrl")
		m &^= primary
	}
	if m&ui.Ctrl != 0 {
		parts = append(parts, "Ctrl")
	}
	if m&ui.Super != 0 {
		if runtime.GOOS == "darwin" {
			parts = append(parts, "Cmd")
		} else {
			parts = append(parts, "Super")
		}
	}
	if m&ui.Alt != 0 {
		parts = append(parts, "Alt")
	}
	if m&ui.Shift != 0 {
		parts = append(parts, "Shift")
	}
	return strings.Join(append(parts, name), "+"), true
}
