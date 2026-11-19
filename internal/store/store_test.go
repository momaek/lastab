package store

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func openTest(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "lastab.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestCreateSaveGet(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)
	body := "line1\r\nline2\rline3\n"
	sn, err := s.CreateSnippet(ctx, KindText, body, 0)
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.Get(ctx, sn.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Body != body || !got.Open || got.TitleSource != TitlePending {
		t.Fatalf("got %+v", got)
	}
	rev, _, err := s.SaveBody(ctx, sn.ID, "changed", sn.Rev)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.SaveBody(ctx, sn.ID, "stale", sn.Rev); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale save: %v", err)
	}
	if _, _, err := s.SaveBody(ctx, sn.ID, "newer", rev); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.SaveBody(ctx, "nope", "x", 1); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing: %v", err)
	}
}

func TestTitleManualWins(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)
	sn, _ := s.CreateSnippet(ctx, KindText, "hello", 0)
	if err := s.SetTitle(ctx, sn.ID, "mine", TitleManual); err != nil {
		t.Fatal(err)
	}
	if err := s.SetTitle(ctx, sn.ID, "auto", TitleAuto); err != nil {
		t.Fatal(err)
	}
	got, _ := s.Get(ctx, sn.ID)
	if got.Title != "mine" || got.TitleSource != TitleManual {
		t.Fatalf("got %q %q", got.Title, got.TitleSource)
	}
}

func TestWorkspaceCloseIsNotDelete(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)
	a, _ := s.CreateSnippet(ctx, KindText, "a", 0)
	b, _ := s.CreateSnippet(ctx, KindTodo, "- [ ] b", 1)
	if err := s.SetTabOrder(ctx, []string{b.ID, a.ID}, a.ID); err != nil {
		t.Fatal(err)
	}
	tabs, active, err := s.Workspace(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(tabs) != 2 || tabs[0].ID != b.ID || active != a.ID {
		t.Fatalf("tabs %v active %s", tabs, active)
	}
	if err := s.CloseTab(ctx, a.ID); err != nil {
		t.Fatal(err)
	}
	tabs, _, _ = s.Workspace(ctx)
	if len(tabs) != 1 {
		t.Fatalf("tabs after close: %d", len(tabs))
	}
	got, err := s.Get(ctx, a.ID)
	if err != nil || got.Open {
		t.Fatalf("closed snippet: %+v %v", got, err)
	}
	if err := s.Delete(ctx, a.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(ctx, a.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("after delete: %v", err)
	}
}

func TestSearch(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)
	mk := func(body string) Snippet {
		sn, err := s.CreateSnippet(ctx, KindText, body, 0)
		if err != nil {
			t.Fatal(err)
		}
		return sn
	}
	curl := mk(`curl -X POST 'https://api.example.com/v2/orders/search' -d '{"order_id":"A-20391"}'`)
	meet := mk("周会纪要：发版前检查清单")
	cli := mk("lastab search --since 1d")
	closed := mk("Closed but findable TOKEN")
	if err := s.CloseTab(ctx, closed.ID); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		q    string
		want string
	}{
		{"/v2/", curl.ID},
		{`{"`, curl.ID},
		{"order_ID", curl.ID}, // case-insensitive
		{"周会", meet.ID},       // 2 characters: LIKE fallback
		{"发版前", meet.ID},      // trigram
		{"--since", cli.ID},
		{"findable token", closed.ID},
		{"%", ""},
	}
	for _, c := range cases {
		got, err := s.Search(ctx, c.q, ListOptions{})
		if err != nil {
			t.Fatalf("%q: %v", c.q, err)
		}
		if c.want == "" {
			if len(got) != 0 {
				t.Errorf("%q: want none, got %d", c.q, len(got))
			}
			continue
		}
		if len(got) != 1 || got[0].ID != c.want {
			t.Errorf("%q: got %v", c.q, ids(got))
		}
	}
	got, _ := s.Search(ctx, "e", ListOptions{OpenOnly: true})
	for _, sn := range got {
		if sn.ID == closed.ID {
			t.Error("OpenOnly returned a closed snippet")
		}
	}
	got, _ = s.Search(ctx, "search", ListOptions{Since: time.Now().Add(time.Hour)})
	if len(got) != 0 {
		t.Errorf("Since in the future: %v", ids(got))
	}
}

// TestShortSearch checks the short-query scan, instr or LIKE, against a
// plain LIKE over title and body, and the open and closed filters against
// the workspace.
func TestShortSearch(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)
	bodies := []string{
		"周会纪要", "下周会议", "{\"id\": 1}", "100% done", "a_b c", `C:\\path`, "Café", "CAFÉ", "naïve",
		"周A计划", "周a计划", "Weekly sync", "WEEKLY", "x", "", "会", "emoji 🙂 here", "tab\there",
	}
	var open []string
	for i, b := range bodies {
		sn, err := s.CreateSnippet(ctx, KindText, b, i)
		if err != nil {
			t.Fatal(err)
		}
		if err := s.SetTitle(ctx, sn.ID, fmt.Sprintf("title %d 周", i), TitleAuto); err != nil {
			t.Fatal(err)
		}
		if i%3 == 0 {
			open = append(open, sn.ID)
		} else {
			if err := s.CloseTab(ctx, sn.ID); err != nil {
				t.Fatal(err)
			}
		}
	}
	isOpen := map[string]bool{}
	for _, id := range open {
		isOpen[id] = true
	}
	for _, q := range []string{"周会", "周", "会", `{"`, "%", "_", `\`, "é", "É", "ï", "周A", "周a", "we", "WE", "x", "🙂", "1", "99"} {
		rows, err := s.r.QueryContext(ctx, `SELECT id FROM snippet WHERE title LIKE ? ESCAPE '\' OR body LIKE ? ESCAPE '\'`,
			"%"+likeEscape(q)+"%", "%"+likeEscape(q)+"%")
		if err != nil {
			t.Fatal(err)
		}
		want := map[string]bool{}
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				t.Fatal(err)
			}
			want[id] = true
		}
		rows.Close()
		for _, o := range []ListOptions{{}, {OpenOnly: true}, {ClosedOnly: true}} {
			got, err := s.Search(ctx, q, o)
			if err != nil {
				t.Fatalf("%q: %v", q, err)
			}
			n := 0
			for id := range want {
				if (!o.OpenOnly || isOpen[id]) && (!o.ClosedOnly || !isOpen[id]) {
					n++
				}
			}
			if len(got) != n {
				t.Errorf("%q %+v: got %d, want %d", q, o, len(got), n)
			}
			for _, sn := range got {
				if !want[sn.ID] || o.OpenOnly && !isOpen[sn.ID] || o.ClosedOnly && isOpen[sn.ID] || sn.Open != isOpen[sn.ID] {
					t.Errorf("%q %+v: unexpected %q", q, o, sn.Body)
				}
			}
		}
	}
}

func ids(sn []Snippet) []string {
	var out []string
	for _, s := range sn {
		out = append(out, s.ID)
	}
	return out
}

func TestVersionsAndSettings(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)
	sn, _ := s.CreateSnippet(ctx, KindText, "a", 0)
	if _, err := s.LatestVersion(ctx, sn.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if _, err := s.AddVersion(ctx, sn.ID, "a", ReasonPaused, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddVersion(ctx, sn.ID, "ab", ReasonSwitch, 1); err != nil {
		t.Fatal(err)
	}
	v, err := s.LatestVersion(ctx, sn.ID)
	if err != nil || v.Body != "ab" {
		t.Fatalf("%+v %v", v, err)
	}
	if err := s.SetSetting(ctx, SettingAppearance, "dark"); err != nil {
		t.Fatal(err)
	}
	var app string
	ok, err := s.GetSetting(ctx, SettingAppearance, &app)
	if !ok || err != nil || app != "dark" {
		t.Fatalf("%v %v %q", ok, err, app)
	}
}

// The CLI reads while the app writes: no errors, no blocking.
func TestReadOnlyWhileWriting(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "lastab.db")
	app, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	sn, _ := app.CreateSnippet(ctx, KindText, "seed text", 0)

	cli, err := OpenReadOnly(path)
	if err != nil {
		t.Fatal(err)
	}
	defer cli.Close()

	var wg sync.WaitGroup
	wg.Add(1)
	errs := make(chan error, 1000)
	go func() {
		defer wg.Done()
		rev := sn.Rev
		for i := range 200 {
			r, _, err := app.SaveBody(ctx, sn.ID, "seed text "+string(rune('a'+i%26)), rev)
			if err != nil {
				errs <- err
				return
			}
			rev = r
		}
	}()
	for range 200 {
		if _, err := cli.Search(ctx, "seed", ListOptions{}); err != nil {
			errs <- err
			break
		}
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	if _, err := cli.CreateSnippet(ctx, KindText, "x", 0); err == nil {
		t.Error("read-only store wrote")
	}
}

func TestOpenReadOnlyMissing(t *testing.T) {
	if _, err := OpenReadOnly(filepath.Join(t.TempDir(), "none.db")); !errors.Is(err, ErrNoDatabase) {
		t.Fatal(err)
	}
}

func TestGetByPrefix(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)
	a, _ := s.CreateSnippet(ctx, KindText, "a", 0)
	got, err := s.GetByPrefix(ctx, a.ID[:20])
	if err != nil || got.ID != a.ID {
		t.Fatalf("%v %v", got.ID, err)
	}
	if _, err := s.CreateSnippet(ctx, KindText, "b", 0); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetByPrefix(ctx, a.ID[:2]); !errors.Is(err, ErrAmbiguous) {
		t.Fatalf("short prefix: %v", err)
	}
}

func TestClosedOnlyAndVersionCounts(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)
	a, _ := s.CreateSnippet(ctx, KindText, "open one", 0)
	b, _ := s.CreateSnippet(ctx, KindText, "closed one", 1)
	if err := s.CloseTab(ctx, b.ID); err != nil {
		t.Fatal(err)
	}
	got, _ := s.Search(ctx, "one", ListOptions{ClosedOnly: true})
	if len(got) != 1 || got[0].ID != b.ID {
		t.Fatalf("closed only: %v", ids(got))
	}
	if _, err := s.AddVersion(ctx, a.ID, "x", ReasonPaused, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddVersion(ctx, a.ID, "y", ReasonPaused, 1); err != nil {
		t.Fatal(err)
	}
	n, err := s.VersionCounts(ctx, []string{a.ID, b.ID})
	if err != nil || n[a.ID] != 2 || n[b.ID] != 0 {
		t.Fatalf("%v %v", n, err)
	}
}

func TestAgentRetitle(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)
	sn, _ := s.CreateSnippet(ctx, KindText, "curl https://x.dev", 0)
	if err := s.SetTitle(ctx, sn.ID, "curl · x.dev", TitleAuto); err != nil {
		t.Fatal(err)
	}
	before, _ := s.Get(ctx, sn.ID)
	old, err := s.AgentRetitle(ctx, sn.ID, "staging orders", "claude-code")
	if err != nil {
		t.Fatal(err)
	}
	if old.Title != "curl · x.dev" || old.TitleSource != TitleAuto {
		t.Fatalf("old %+v", old)
	}
	got, _ := s.Get(ctx, sn.ID)
	if got.Title != "staging orders" || got.TitleSource != TitleManual || got.TitleBy != "claude-code" || got.UserNamed() {
		t.Fatalf("got %+v", got)
	}
	if !got.ModifiedAt.Equal(before.ModifiedAt) || got.Rev != before.Rev {
		t.Fatalf("a rename changed modified_at or rev: %+v", got)
	}
	// An automatic title doesn't replace an agent's.
	if err := s.SetTitle(ctx, sn.ID, "auto", TitleAuto); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.Get(ctx, sn.ID); got.Title != "staging orders" {
		t.Fatalf("auto replaced the agent's title: %q", got.Title)
	}
	// Another agent may rename what an agent named.
	if _, err := s.AgentRetitle(ctx, sn.ID, "orders repro", "codex"); err != nil {
		t.Fatal(err)
	}
	// The user's own name is final.
	if err := s.SetTitle(ctx, sn.ID, "mine", TitleManual); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.Get(ctx, sn.ID); !got.UserNamed() {
		t.Fatalf("not the user's: %+v", got)
	}
	if _, err := s.AgentRetitle(ctx, sn.ID, "theirs", "claude-code"); !errors.Is(err, ErrUserTitle) {
		t.Fatalf("renamed the user's title: %v", err)
	}
	if got, _ := s.Get(ctx, sn.ID); got.Title != "mine" {
		t.Fatalf("title %q", got.Title)
	}
	if _, err := s.AgentRetitle(ctx, "nope", "x", "claude-code"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing: %v", err)
	}
}

func TestUndoAgentTitle(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)
	sn, _ := s.CreateSnippet(ctx, KindText, "hello", 0)
	must(t, s.SetTitle(ctx, sn.ID, "hello", TitleAuto))
	old, _ := s.AgentRetitle(ctx, sn.ID, "greeting", "claude-code")
	ok, err := s.UndoAgentTitle(ctx, sn.ID, "greeting", "claude-code", old)
	if err != nil || !ok {
		t.Fatalf("undo %v %v", ok, err)
	}
	if got, _ := s.Get(ctx, sn.ID); got.Title != "hello" || got.TitleSource != TitleAuto || got.TitleBy != "" {
		t.Fatalf("got %+v", got)
	}
	// After the user renames it, an old agent rename no longer undoes.
	old, _ = s.AgentRetitle(ctx, sn.ID, "greeting", "claude-code")
	must(t, s.SetTitle(ctx, sn.ID, "mine", TitleManual))
	if ok, _ := s.UndoAgentTitle(ctx, sn.ID, "greeting", "claude-code", old); ok {
		t.Fatal("undid over the user's title")
	}
	if got, _ := s.Get(ctx, sn.ID); got.Title != "mine" {
		t.Fatalf("title %q", got.Title)
	}
}

func TestOpenAgentWriter(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "lastab.db")
	if _, err := OpenAgentWriter(path); !errors.Is(err, ErrNoDatabase) {
		t.Fatalf("no db: %v", err)
	}
	app, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	sn, _ := app.CreateSnippet(ctx, KindText, "hello", 0)
	cli, err := OpenAgentWriter(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cli.AgentRetitle(ctx, sn.ID, "greeting", "test"); err != nil {
		t.Fatal(err)
	}
	cli.Close()
	if got, _ := app.Get(ctx, sn.ID); got.Title != "greeting" {
		t.Fatalf("the app doesn't see it: %q", got.Title)
	}
	if _, err := app.w.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d", LatestVersion()-1)); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenAgentWriter(path); !errors.Is(err, ErrTooOld) {
		t.Fatalf("old db: %v", err)
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
