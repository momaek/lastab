package snippet

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/momaek/lastab/internal/store"
)

func TestIsBlank(t *testing.T) {
	for in, want := range map[string]bool{"": true, " \n\t\r\n　": true, " x ": false, "\n{": false} {
		if IsBlank(in) != want {
			t.Errorf("IsBlank(%q) != %v", in, want)
		}
	}
}

func TestTitlesAndClose(t *testing.T) {
	ctx := context.Background()
	st, err := store.Open(filepath.Join(t.TempDir(), "lastab.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	s := New(st)

	sn, _ := s.Create(ctx, store.KindText, "curl https://x.dev/a", 0)
	got, src, err := s.UpdateTitle(ctx, sn.ID)
	if err != nil || got != "curl · GET /a" || src != store.TitleAuto {
		t.Fatalf("%q %q %v", got, src, err)
	}
	// A text snippet keeps its first automatic title.
	s.SaveBody(ctx, sn.ID, "something else", sn.Rev)
	if got, _, _ := s.UpdateTitle(ctx, sn.ID); got != "curl · GET /a" {
		t.Errorf("title changed to %q", got)
	}
	// A to-do list's follows its progress.
	td, _ := s.Create(ctx, store.KindTodo, "- [ ] a\n- [ ] b", 1)
	s.UpdateTitle(ctx, td.ID)
	s.SaveBody(ctx, td.ID, "- [x] a\n- [ ] b", td.Rev)
	if got, _, _ := s.UpdateTitle(ctx, td.ID); got != "a · 1/2" {
		t.Errorf("todo title %q", got)
	}
	// Manual titles stick; clearing goes back to automatic.
	s.Rename(ctx, td.ID, "Release")
	if got, src, _ := s.UpdateTitle(ctx, td.ID); got != "Release" || src != store.TitleManual {
		t.Errorf("manual %q %q", got, src)
	}
	s.Rename(ctx, td.ID, "  ")
	if got, _, _ := s.UpdateTitle(ctx, td.ID); got != "a · 1/2" {
		t.Errorf("after clearing: %q", got)
	}

	if err := s.Close(ctx, sn.ID, "something else"); err != nil {
		t.Fatal(err)
	}
	vs, _ := st.Versions(ctx, sn.ID)
	if len(vs) != 1 || vs[0].Reason != store.ReasonClose {
		t.Errorf("versions %+v", vs)
	}
	if _, err := st.Get(ctx, sn.ID); err != nil {
		t.Errorf("closing deleted the snippet: %v", err)
	}
}

func TestSessionCrash(t *testing.T) {
	dir := t.TempDir()
	s, crashed, err := StartSession(dir)
	if err != nil || crashed {
		t.Fatalf("first start: %v %v", crashed, err)
	}
	s.End()
	if _, crashed, _ = StartSession(dir); crashed {
		t.Error("normal quit seen as a crash")
	}
	// A lock left by a dead process.
	os.WriteFile(filepath.Join(dir, "session.lock"), []byte("999999"), 0o600)
	if _, crashed, _ = StartSession(dir); !crashed {
		t.Error("crash not detected")
	}
}
