// Command lastab reads Lastab's snippets for agents, read-only, as JSON.
// It works whether or not the app runs, and records each read in
// agent_log.db. The user turns access on and off in the app's settings.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/momaek/lastab/internal/agentlog"
	"github.com/momaek/lastab/internal/paths"
	"github.com/momaek/lastab/internal/redact"
	"github.com/momaek/lastab/internal/search"
	"github.com/momaek/lastab/internal/store"
	"github.com/momaek/lastab/internal/todo"
)

// version is set at build time: -ldflags "-X main.version=0.1.0".
var version = "dev"

// Exit codes.
const (
	exitOK       = 0
	exitUsage    = 2
	exitDenied   = 3
	exitNotFound = 4
	exitNoDB     = 5
	exitInternal = 1
)

type cliError struct {
	code    int
	kind    string
	message string
}

func (e *cliError) Error() string { return e.message }

func fail(code int, kind, format string, args ...any) error {
	return &cliError{code, kind, fmt.Sprintf(format, args...)}
}

type env struct {
	out     io.Writer
	dataDir string
	client  string
	now     func() time.Time
	st      *store.Store
	raw     bool // agent.allowRaw: output bodies unmasked
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	e := &env{out: stdout, now: time.Now}
	root := newRoot(e)
	root.SetArgs(args)
	root.SetOut(stdout)
	root.SetErr(stderr)
	err := root.Execute()
	if e.st != nil {
		e.st.Close()
	}
	if err == nil {
		return exitOK
	}
	var ce *cliError
	if !errors.As(err, &ce) {
		// cobra's own errors: unknown commands and flags.
		ce = &cliError{exitUsage, "usage", err.Error()}
	}
	writeJSON(stdout, map[string]any{"error": map[string]any{"kind": ce.kind, "message": ce.message}})
	return ce.code
}

func writeJSON(w io.Writer, v any) {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	// A failed write to stdout leaves nowhere to report the failure.
	_ = enc.Encode(v)
}

func newRoot(e *env) *cobra.Command {
	root := &cobra.Command{
		Use:           "lastab",
		Short:         "Read Lastab snippets as JSON (read-only)",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().StringVar(&e.dataDir, "data-dir", "", "data directory (default: the app's; or $LASTAB_DATA_DIR)")
	root.PersistentFlags().StringVar(&e.client, "client", "", "name of the calling agent, for the access log")
	root.PersistentFlags().Bool("json", true, "output JSON (the only format)")

	root.AddCommand(listCmd(e), searchCmd(e), readCmd(e), statusCmd(e), versionCmd(e))
	return root
}

func (e *env) dbPath() (string, error) {
	dir, err := paths.DataDir(e.dataDir, false)
	if err != nil {
		return "", fail(exitNoDB, "no_database", "%v", err)
	}
	return filepath.Join(dir, paths.DBFile), nil
}

// open opens the database read-only and checks access is allowed.
func (e *env) open(ctx context.Context) error {
	path, err := e.dbPath()
	if err != nil {
		return err
	}
	st, err := store.OpenReadOnly(path)
	switch {
	case errors.Is(err, store.ErrNoDatabase):
		return fail(exitNoDB, "no_database", "no Lastab database at %s", path)
	case errors.Is(err, store.ErrTooNew):
		return fail(exitNoDB, "database_too_new", "%v: update lastab", err)
	case err != nil:
		return fail(exitInternal, "internal", "%v", err)
	}
	e.st = st
	var access bool
	if _, err := st.GetSetting(ctx, store.SettingAgentAccess, &access); err != nil {
		return fail(exitInternal, "internal", "%v", err)
	}
	if !access {
		return fail(exitDenied, "access_denied", "agent access is turned off in Lastab's settings")
	}
	if _, err := st.GetSetting(ctx, store.SettingAgentRaw, &e.raw); err != nil {
		return fail(exitInternal, "internal", "%v", err)
	}
	return nil
}

func (e *env) clientName() string {
	if e.client != "" {
		return e.client
	}
	if os.Getenv("CLAUDECODE") == "1" {
		return "claude-code"
	}
	return "unknown"
}

func (e *env) record(ctx context.Context, op string, args map[string]any, ids []string) {
	path, err := e.dbPath()
	if err != nil {
		return
	}
	l, err := agentlog.Open(filepath.Join(filepath.Dir(path), paths.AgentLogFile))
	if err != nil {
		return
	}
	defer l.Close()
	// The access record is best effort: it never fails the command.
	_ = l.Add(ctx, agentlog.Entry{At: e.now(), Client: e.clientName(), Op: op, Args: args, SnippetIDs: ids})
}

func (e *env) mask(s string) (string, bool) {
	if e.raw {
		return s, false
	}
	return redact.Text(s)
}

// summary is a snippet in list and search results.
type summary struct {
	ID         string         `json:"id"`
	Kind       store.Kind     `json:"kind"`
	Title      string         `json:"title"`
	CreatedAt  string         `json:"created_at"`
	ModifiedAt string         `json:"modified_at"`
	Open       bool           `json:"open"`
	Preview    string         `json:"preview"`
	Progress   *todo.Progress `json:"progress,omitempty"`
	Matches    []search.Range `json:"matches,omitempty"`
}

func ts(t time.Time) string { return t.UTC().Format(time.RFC3339) }

func (e *env) summarize(sn store.Snippet, preview string, hs []search.Range) summary {
	s := summary{
		ID: sn.ID, Kind: sn.Kind, Title: redact.String(sn.Title),
		CreatedAt: ts(sn.CreatedAt), ModifiedAt: ts(sn.ModifiedAt), Open: sn.Open,
		Preview: preview, Matches: hs,
	}
	if sn.Kind == store.KindTodo {
		p := todo.Count(todo.Parse(sn.Body))
		s.Progress = &p
	}
	return s
}

type filterFlags struct {
	limit int
	kind  string
	since string
	until string
	open  bool
}

func (f *filterFlags) add(cmd *cobra.Command, withOpen bool) {
	cmd.Flags().IntVar(&f.limit, "limit", 20, "most results")
	cmd.Flags().StringVar(&f.kind, "kind", "", "text or todo")
	cmd.Flags().StringVar(&f.since, "since", "", "modified since: a duration (30m, 2h, 1d, 1w) or a date (2026-11-18)")
	cmd.Flags().StringVar(&f.until, "until", "", "modified before: a date (2026-11-18) or a duration ago")
	if withOpen {
		cmd.Flags().BoolVar(&f.open, "open", false, "only snippets open as tabs")
	}
}

func (f *filterFlags) options(now time.Time) (store.ListOptions, error) {
	o := store.ListOptions{Limit: f.limit, OpenOnly: f.open}
	switch f.kind {
	case "":
	case "text", "todo":
		o.Kind = store.Kind(f.kind)
	default:
		return o, fail(exitUsage, "usage", "--kind must be text or todo")
	}
	if f.limit < 1 || f.limit > 500 {
		return o, fail(exitUsage, "usage", "--limit must be between 1 and 500")
	}
	var err error
	if o.Since, err = parseWhen(f.since, now, false); err != nil {
		return o, fail(exitUsage, "usage", "--since: %v", err)
	}
	if o.Until, err = parseWhen(f.until, now, true); err != nil {
		return o, fail(exitUsage, "usage", "--until: %v", err)
	}
	return o, nil
}

func (f *filterFlags) args() map[string]any {
	m := map[string]any{"limit": f.limit}
	for k, v := range map[string]string{"kind": f.kind, "since": f.since, "until": f.until} {
		if v != "" {
			m[k] = v
		}
	}
	if f.open {
		m["open"] = true
	}
	return m
}

// parseWhen reads a duration ago (30m, 2h, 1d, 1w) or a local date. An
// --until date means the end of that day.
func parseWhen(s string, now time.Time, endOfDay bool) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, nil
	}
	if t, err := time.ParseInLocation("2006-01-02", s, time.Local); err == nil {
		if endOfDay {
			t = t.AddDate(0, 0, 1)
		}
		return t, nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	if len(s) >= 2 {
		n, err := strconv.Atoi(s[:len(s)-1])
		if err == nil && n >= 0 {
			unit := map[byte]time.Duration{'m': time.Minute, 'h': time.Hour, 'd': 24 * time.Hour, 'w': 7 * 24 * time.Hour}[s[len(s)-1]]
			if unit != 0 {
				return now.Add(-time.Duration(n) * unit), nil
			}
		}
	}
	return time.Time{}, fmt.Errorf("%q is not a duration (30m, 2h, 1d, 1w) or a date (2006-01-02)", s)
}

func listCmd(e *env) *cobra.Command {
	var f filterFlags
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List snippets, most recently edited first",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			o, err := f.options(e.now())
			if err != nil {
				return err
			}
			if err := e.open(ctx); err != nil {
				return err
			}
			sns, err := e.st.List(ctx, o)
			if err != nil {
				return fail(exitInternal, "internal", "%v", err)
			}
			out := []summary{}
			var ids []string
			for _, sn := range sns {
				p, _ := search.Preview(sn.Body, "", 160)
				if e.raw {
					p = firstChars(sn.Body, 160)
				}
				out = append(out, e.summarize(sn, p, nil))
				ids = append(ids, sn.ID)
			}
			e.record(ctx, "list", f.args(), ids)
			writeJSON(e.out, map[string]any{"snippets": out})
			return nil
		},
	}
	f.add(cmd, true)
	return cmd
}

func searchCmd(e *env) *cobra.Command {
	var f filterFlags
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Find snippets whose title or text contains the query",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			q := strings.Join(args, " ")
			o, err := f.options(e.now())
			if err != nil {
				return err
			}
			if err := e.open(ctx); err != nil {
				return err
			}
			res, err := search.Searcher{Store: e.st}.Query(ctx, q, o)
			if err != nil {
				return fail(exitInternal, "internal", "%v", err)
			}
			out := []summary{}
			var ids []string
			for _, r := range res {
				out = append(out, e.summarize(r.Snippet, r.Preview, r.Highlights))
				ids = append(ids, r.ID)
			}
			a := f.args()
			a["query"] = q
			e.record(ctx, "search", a, ids)
			writeJSON(e.out, map[string]any{"query": q, "snippets": out})
			return nil
		},
	}
	f.add(cmd, true)
	return cmd
}

func readCmd(e *env) *cobra.Command {
	return &cobra.Command{
		Use:   "read <id or unique prefix>",
		Short: "Print one snippet in full",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if err := e.open(ctx); err != nil {
				return err
			}
			sn, err := e.st.GetByPrefix(ctx, args[0])
			switch {
			case errors.Is(err, store.ErrNotFound):
				return fail(exitNotFound, "not_found", "no snippet with ID %q", args[0])
			case errors.Is(err, store.ErrAmbiguous):
				return fail(exitNotFound, "ambiguous", "ID prefix %q matches several snippets", args[0])
			case err != nil:
				return fail(exitInternal, "internal", "%v", err)
			}
			body, masked := e.mask(sn.Body)
			out := map[string]any{
				"id": sn.ID, "kind": sn.Kind, "title": redact.String(sn.Title),
				"created_at": ts(sn.CreatedAt), "modified_at": ts(sn.ModifiedAt),
				"open": sn.Open, "redacted": masked, "body": body,
			}
			if sn.Kind == store.KindTodo {
				items := todo.Parse(body)
				out["items"] = items
				out["progress"] = todo.Count(items)
			}
			e.record(ctx, "read", map[string]any{"id": args[0]}, []string{sn.ID})
			writeJSON(e.out, out)
			return nil
		},
	}
}

func statusCmd(e *env) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Whether agent access is on, the database version and the count of snippets",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			path, err := e.dbPath()
			if err != nil {
				return err
			}
			st, err := store.OpenReadOnly(path)
			switch {
			case errors.Is(err, store.ErrNoDatabase):
				return fail(exitNoDB, "no_database", "no Lastab database at %s", path)
			case errors.Is(err, store.ErrTooNew):
				return fail(exitNoDB, "database_too_new", "%v: update lastab", err)
			case err != nil:
				return fail(exitInternal, "internal", "%v", err)
			}
			e.st = st
			var access bool
			if _, err := st.GetSetting(ctx, store.SettingAgentAccess, &access); err != nil {
				return fail(exitInternal, "internal", "%v", err)
			}
			v, _ := st.UserVersion(ctx)
			out := map[string]any{"access": access, "database": path, "schema_version": v, "cli_version": version}
			if access {
				n, _ := st.Count(ctx)
				out["snippets"] = n
			}
			writeJSON(e.out, out)
			return nil
		},
	}
}

func versionCmd(e *env) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version",
		Args:  cobra.NoArgs,
		Run: func(*cobra.Command, []string) {
			writeJSON(e.out, map[string]any{"version": version, "schema_version": store.LatestVersion()})
		},
	}
}

func firstChars(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
