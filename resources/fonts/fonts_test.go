package fonts

import (
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestRegister(t *testing.T) {
	if err := Register(); err != nil {
		t.Fatal(err)
	}
	var names []string
	mono := map[string]bool{}
	for _, f := range ui.FontFamilies() {
		names = append(names, f.Name)
		mono[f.Name] = f.Monospaced
	}
	if !slices.Contains(names, UI) || !slices.Contains(names, Mono) {
		t.Fatalf("families %v", names)
	}
	if !mono[Mono] || mono[UI] {
		t.Errorf("monospaced: %v", mono)
	}
}
