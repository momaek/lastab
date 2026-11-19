package appui

import (
	"math"
	"testing"
)

// TestToastCenteredFromFirstFrame checks that a toast shows centered in
// the frame it first appears in and stays there, so its action can be
// clicked at once.
func TestToastCenteredFromFirstFrame(t *testing.T) {
	h := newHarness(t, "")
	var mids []float32
	for _, text := range []string{"Short", "A much longer message than the one before it"} {
		undone := false
		h.app.showToast(text, UndoFor, "Undo", func() { undone = true }, h.tt.Now())
		h.tt.Frame()
		first, ok := h.tt.Find("Undo")
		if !ok {
			t.Fatalf("no toast %q", text)
		}
		r, _ := h.tt.Find(text)
		mids = append(mids, (r.X+first.X+first.W)/2)
		h.tt.Frame()
		if again, _ := h.tt.Find("Undo"); again != first {
			t.Errorf("%q: toast moved after its first frame: %v → %v", text, first, again)
		}
		if err := h.tt.Click("Undo"); err != nil {
			t.Fatal(err)
		}
		h.tt.Frame()
		if !undone {
			t.Errorf("%q: Undo did nothing", text)
		}
	}
	// Centered: the same middle whatever the width.
	if math.Abs(float64(mids[0]-mids[1])) > 1 {
		t.Errorf("middles %v", mids)
	}
}
