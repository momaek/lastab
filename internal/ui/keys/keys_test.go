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

func TestTableAndConflicts(t *testing.T) {
	tab := Table("darwin", map[Action]string{NewText: "Cmd+Alt+N"})
	if tab[NewText] != "Cmd+Alt+N" || tab[NewTodo] != "CmdOrCtrl+T" {
		t.Fatalf("table %v", tab)
	}
	if Canonical("CmdOrCtrl+Shift+c", "darwin") != "Shift+Cmd+C" || Canonical("CmdOrCtrl+Shift+c", "windows") != "Ctrl+Shift+C" {
		t.Error("canonical")
	}
	if a, ok := Conflict(tab, CopyAll, "Cmd+T", "darwin"); !ok || a != NewTodo {
		t.Errorf("conflict %v %v", a, ok)
	}
	if _, ok := Conflict(tab, NewTodo, "CmdOrCtrl+T", "darwin"); ok {
		t.Error("an action conflicts with itself")
	}
	if a, ok := Conflict(tab, NewText, "Cmd+,", "darwin"); !ok || a != Settings {
		t.Error("settings key not reserved")
	}
	if HasModifier("Shift+A", "darwin") || !HasModifier("Alt+A", "darwin") {
		t.Error("HasModifier")
	}
}
