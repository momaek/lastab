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
	if _, err := s.SaveBody(ctx, sn.ID, "something else", sn.Rev); err != nil {
		t.Fatal(err)
	}
	if got, _, _ := s.UpdateTitle(ctx, sn.ID); got != "curl · GET /a" {
		t.Errorf("title changed to %q", got)
	}
	// A to-do list's follows its progress.
	td, _ := s.Create(ctx, store.KindTodo, "- [ ] a\n- [ ] b", 1)
	if _, _, err := s.UpdateTitle(ctx, td.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SaveBody(ctx, td.ID, "- [x] a\n- [ ] b", td.Rev); err != nil {
		t.Fatal(err)
	}
	if got, _, _ := s.UpdateTitle(ctx, td.ID); got != "a · 1/2" {
		t.Errorf("todo title %q", got)
	}
	// Manual titles stick; clearing goes back to automatic.
	if err := s.Rename(ctx, td.ID, "Release"); err != nil {
		t.Fatal(err)
	}
	if got, src, _ := s.UpdateTitle(ctx, td.ID); got != "Release" || src != store.TitleManual {
		t.Errorf("manual %q %q", got, src)
	}
	if err := s.Rename(ctx, td.ID, "  "); err != nil {
		t.Fatal(err)
	}
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
	if err := s.End(); err != nil {
		t.Fatal(err)
	}
	if _, crashed, _ = StartSession(dir); crashed {
		t.Error("normal quit seen as a crash")
	}
	// A lock left by a dead process.
	if err := os.WriteFile(filepath.Join(dir, "session.lock"), []byte("999999"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, crashed, _ = StartSession(dir); !crashed {
		t.Error("crash not detected")
	}
}

func TestRecordRestore(t *testing.T) {
	ctx := context.Background()
	st, err := store.Open(filepath.Join(t.TempDir(), "lastab.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	s := New(st)
	sn, _ := s.Create(ctx, store.KindText, "v1", 0)
	if _, err := s.Snapshot(ctx, sn.ID, "v1", store.ReasonPaused); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Snapshot(ctx, sn.ID, "v2", store.ReasonPaused); err != nil {
		t.Fatal(err)
	}
	vs, _ := s.Versions(ctx, sn.ID)
	old := vs[1]
	// Current text "v2 edited" was never recorded: it is kept before the restore.
	if err := s.RecordRestore(ctx, sn.ID, "v2 edited", "v1", old.ID); err != nil {
		t.Fatal(err)
	}
	vs, _ = s.Versions(ctx, sn.ID)
	if len(vs) != 4 || vs[0].Body != "v1" || vs[0].RestoredFrom != old.ID || vs[1].Body != "v2 edited" || vs[1].RestoredFrom != "" {
		t.Fatalf("versions %+v", vs)
	}
}
