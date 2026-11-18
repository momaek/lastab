package history

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/momaek/lastab/internal/store"
)

func TestLinesChanged(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"", "one\ntwo\n", 2},
		{"a\nb\nc\n", "a\nB\nc\n", 1},
		{"a\nb\nc\n", "a\nc\n", 1},
		{"a\nb", "a\nb", 0},
		{"a", "a\nb\nc", 3}, // "a" (no newline) becomes "a\n": changed, plus two added
	}
	for _, c := range cases {
		if got := LinesChanged(c.a, c.b); got != c.want {
			t.Errorf("LinesChanged(%q, %q) = %d; want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestSnapshotDedupes(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(filepath.Join(t.TempDir(), "lastab.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	sn, _ := s.CreateSnippet(ctx, store.KindText, "a", 0)
	r := Recorder{Store: s}
	for _, step := range []struct {
		body string
		want bool
	}{{"a", true}, {"a", false}, {"a\nb", true}, {"a\nb", false}} {
		got, err := r.Snapshot(ctx, sn.ID, step.body, store.ReasonPaused)
		if err != nil || got != step.want {
			t.Fatalf("Snapshot(%q) = %v, %v", step.body, got, err)
		}
	}
	vs, _ := s.Versions(ctx, sn.ID)
	if len(vs) != 2 || vs[0].LinesChanged != 2 {
		t.Fatalf("versions %+v", vs)
	}
}
