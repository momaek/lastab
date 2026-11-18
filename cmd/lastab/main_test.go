package main

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/momaek/lastab/internal/agentlog"
	"github.com/momaek/lastab/internal/store"
)

func setup(t *testing.T, access bool) (string, *store.Store) {
	t.Helper()
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "lastab.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	if access {
		st.SetSetting(context.Background(), store.SettingAgentAccess, true)
	}
	return dir, st
}

func cli(t *testing.T, dir string, args ...string) (int, map[string]any) {
	t.Helper()
	var out, errOut bytes.Buffer
	code := run(append([]string{"--data-dir", dir, "--client", "test"}, args...), &out, &errOut)
	var m map[string]any
	if out.Len() > 0 {
		if err := json.Unmarshal(out.Bytes(), &m); err != nil {
			t.Fatalf("not JSON: %q", out.String())
		}
	}
	return code, m
}

func TestAccessOffPrintsNothingButTheError(t *testing.T) {
	dir, st := setup(t, false)
	st.CreateSnippet(context.Background(), store.KindText, "secret plans", 0)
	code, m := cli(t, dir, "list")
	if code != exitDenied {
		t.Fatalf("code %d", code)
	}
	if _, ok := m["snippets"]; ok || m["error"] == nil {
		t.Fatalf("output %v", m)
	}
}

func TestNoDatabase(t *testing.T) {
	code, m := cli(t, t.TempDir(), "list")
	if code != exitNoDB || m["error"].(map[string]any)["kind"] != "no_database" {
		t.Fatalf("%d %v", code, m)
	}
}

func TestListSearchRead(t *testing.T) {
	dir, st := setup(t, true)
	ctx := context.Background()
	curl, _ := st.CreateSnippet(ctx, store.KindText, "curl -H 'Authorization: Bearer abcdefgh12345678' https://x.dev/v2/orders", 0)
	st.SetTitle(ctx, curl.ID, "curl · GET /v2/orders", store.TitleAuto)
	td, _ := st.CreateSnippet(ctx, store.KindTodo, "- [x] a\n- [ ] b", 1)

	code, m := cli(t, dir, "list")
	if code != 0 || len(m["snippets"].([]any)) != 2 {
		t.Fatalf("list %d %v", code, m)
	}
	code, m = cli(t, dir, "search", "/v2/")
	sn := m["snippets"].([]any)
	if code != 0 || len(sn) != 1 || sn[0].(map[string]any)["id"] != curl.ID {
		t.Fatalf("search %d %v", code, m)
	}
	if strings.Contains(sn[0].(map[string]any)["preview"].(string), "abcdefgh") {
		t.Error("secret in preview")
	}

	code, m = cli(t, dir, "read", strings.ToLower(curl.ID[:22]))
	if code != 0 || m["redacted"] != true || strings.Contains(m["body"].(string), "abcdefgh") {
		t.Fatalf("read %d %v", code, m)
	}
	code, m = cli(t, dir, "read", td.ID)
	if code != 0 || m["progress"].(map[string]any)["done"] != 1.0 || len(m["items"].([]any)) != 2 {
		t.Fatalf("read todo %d %v", code, m)
	}
	if code, _ = cli(t, dir, "read", "ZZZZ"); code != exitNotFound {
		t.Errorf("missing id: %d", code)
	}
	if code, _ = cli(t, dir, "list", "--since", "yesterday"); code != exitUsage {
		t.Errorf("bad --since: %d", code)
	}
	if code, _ = cli(t, dir, "nope"); code != exitUsage {
		t.Errorf("unknown command: %d", code)
	}

	// Raw output when the user allows it.
	st.SetSetting(ctx, store.SettingAgentRaw, true)
	_, m = cli(t, dir, "read", curl.ID)
	if m["redacted"] != false || !strings.Contains(m["body"].(string), "abcdefgh") {
		t.Errorf("raw read %v", m)
	}

	l, err := agentlog.Open(filepath.Join(dir, "agent_log.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	es, _ := l.Recent(ctx, 50)
	if len(es) != 5 || es[0].Client != "test" || es[0].Op != "read" {
		t.Errorf("log %+v", es)
	}
}
