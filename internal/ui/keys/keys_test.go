package keys

import "testing"

func TestDisplay(t *testing.T) {
	cases := []struct{ acc, goos, want string }{
		{"CmdOrCtrl+Shift+C", "darwin", "⇧⌘C"},
		{"CmdOrCtrl+Shift+C", "windows", "Ctrl+Shift+C"},
		{"Ctrl+Alt+N", "darwin", "⌃⌥N"},
		{"CmdOrCtrl+`", "darwin", "⌘`"},
		{"Ctrl+Shift+Y", "windows", "Ctrl+Shift+Y"},
	}
	for _, c := range cases {
		if got := Display(c.acc, c.goos); got != c.want {
			t.Errorf("Display(%q, %s) = %q; want %q", c.acc, c.goos, got, c.want)
		}
	}
	if Defaults("windows")[History] != "Ctrl+Shift+Y" || Defaults("darwin")[History] != "Cmd+Y" {
		t.Error("history defaults")
	}
}
