package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/oklog/ulid/v2"
)

// Kind is a snippet's kind.
type Kind string

const (
	KindText Kind = "text"
	KindTodo Kind = "todo"
)

// TitleSource says where a snippet's title came from.
type TitleSource string

const (
	TitlePending TitleSource = "pending"
	TitleAuto    TitleSource = "auto"
	TitleManual  TitleSource = "manual"
)

// Snippet is one row of the snippet table.
type Snippet struct {
	ID          string
	Kind        Kind
	Body        string
	Title       string
	TitleSource TitleSource
	// TitleBy is who named a manual title: "" for the user, or the client
	// name of the agent that renamed it through `lastab retitle`.
	TitleBy    string
	CreatedAt  time.Time
	ModifiedAt time.Time
	Rev        int
	Open       bool // in the workspace
}

var (
	idMu      sync.Mutex
	idEntropy = ulid.Monotonic(rand.Reader, 0)
)

// NewID returns a new ULID, sortable by time.
func NewID(t time.Time) string {
	idMu.Lock()
	defer idMu.Unlock()
	return ulid.MustNew(ulid.Timestamp(t), idEntropy).String()
}

const snippetCols = `s.id, s.kind, s.body, s.title, s.title_source, s.title_by, s.created_at, s.modified_at, s.rev,
	EXISTS(SELECT 1 FROM workspace_tab w WHERE w.snippet_id = s.id)`

type scanner interface{ Scan(...any) error }

func scanSnippet(row scanner) (Snippet, error) {
	var sn Snippet
	var created, modified int64
	var open bool
	err := row.Scan(&sn.ID, &sn.Kind, &sn.Body, &sn.Title, &sn.TitleSource, &sn.TitleBy, &created, &modified, &sn.Rev, &open)
	if errors.Is(err, sql.ErrNoRows) {
		return sn, ErrNotFound
	}
	sn.CreatedAt = time.UnixMilli(created)
	sn.ModifiedAt = time.UnixMilli(modified)
	sn.Open = open
	return sn, err
}

// CreateSnippet inserts a snippet and opens it as a tab at position.
func (s *Store) CreateSnippet(ctx context.Context, kind Kind, body string, position int) (Snippet, error) {
	w, err := s.writer()
	if err != nil {
		return Snippet{}, err
	}
	now := s.now()
	sn := Snippet{
		ID: NewID(now), Kind: kind, Body: body, TitleSource: TitlePending,
		CreatedAt: time.UnixMilli(now.UnixMilli()), ModifiedAt: time.UnixMilli(now.UnixMilli()),
		Rev: 1, Open: true,
	}
	tx, err := w.BeginTx(ctx, nil)
	if err != nil {
		return Snippet{}, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO snippet (id, kind, body, title, title_source, created_at, modified_at, rev) VALUES (?, ?, ?, '', 'pending', ?, ?, 1)`,
		sn.ID, sn.Kind, sn.Body, now.UnixMilli(), now.UnixMilli()); err != nil {
		return Snippet{}, err
	}
	if err := openTabTx(ctx, tx, sn.ID, position, now.UnixMilli()); err != nil {
		return Snippet{}, err
	}
	return sn, tx.Commit()
}

// SaveBody writes a snippet's body if its revision is still baseRev, and
// returns the new revision. It returns ErrConflict otherwise.
func (s *Store) SaveBody(ctx context.Context, id, body string, baseRev int) (int, time.Time, error) {
	w, err := s.writer()
	if err != nil {
		return 0, time.Time{}, err
	}
	now := s.nowMs()
	res, err := w.ExecContext(ctx,
		`UPDATE snippet SET body = ?, modified_at = ?, rev = rev + 1 WHERE id = ? AND rev = ?`,
		body, now, id, baseRev)
	if err != nil {
		return 0, time.Time{}, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		var exists bool
		if err := w.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM snippet WHERE id = ?)`, id).Scan(&exists); err != nil {
			return 0, time.Time{}, err
		}
		if !exists {
			return 0, time.Time{}, ErrNotFound
		}
		return 0, time.Time{}, ErrConflict
	}
	return baseRev + 1, time.UnixMilli(now), nil
}

// SetTitle sets a snippet's title and its source, as the app does: a manual
// title is the user's own. An automatic title never replaces a manual one
// (the user's or an agent's); a manual or pending source always applies.
func (s *Store) SetTitle(ctx context.Context, id, title string, source TitleSource) error {
	w, err := s.writer()
	if err != nil {
		return err
	}
	q := `UPDATE snippet SET title = ?, title_source = ?, title_by = '' WHERE id = ?`
	if source == TitleAuto {
		q += ` AND title_source != 'manual'`
	}
	_, err = w.ExecContext(ctx, q, title, source, id)
	return err
}

// UserNamed reports whether the user named the snippet themselves: then
// nothing but the user renames it.
func (sn Snippet) UserNamed() bool { return sn.TitleSource == TitleManual && sn.TitleBy == "" }

// ErrUserTitle means the user named the snippet, so an agent may not rename it.
var ErrUserTitle = errors.New("store: the user named this snippet")

// AgentRetitle sets the title of snippet id on behalf of the agent client,
// as `lastab retitle` does, and returns the snippet as it was before. The
// title is stored as manual, so automatic titles no longer replace it, with
// client in title_by. A title the user named is never replaced: that
// returns ErrUserTitle. modified_at stays: a title is metadata, not an edit.
func (s *Store) AgentRetitle(ctx context.Context, id, title, client string) (Snippet, error) {
	w, err := s.writer()
	if err != nil {
		return Snippet{}, err
	}
	if client == "" {
		return Snippet{}, errors.New("store: an agent rename needs a client name")
	}
	tx, err := w.BeginTx(ctx, nil)
	if err != nil {
		return Snippet{}, err
	}
	defer tx.Rollback()
	old, err := scanSnippet(tx.QueryRowContext(ctx, `SELECT `+snippetCols+` FROM snippet s WHERE s.id = ?`, id))
	if err != nil {
		return Snippet{}, err
	}
	if old.UserNamed() {
		return old, ErrUserTitle
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE snippet SET title = ?, title_source = 'manual', title_by = ? WHERE id = ?`, title, client, id); err != nil {
		return Snippet{}, err
	}
	return old, tx.Commit()
}

// UndoAgentTitle puts back the title that agent client's rename to
// agentTitle replaced, if the snippet still has that agent's title, and
// reports whether it did. A later rename, by the user or anyone, wins.
func (s *Store) UndoAgentTitle(ctx context.Context, id, agentTitle, client string, prev Snippet) (bool, error) {
	w, err := s.writer()
	if err != nil {
		return false, err
	}
	switch prev.TitleSource {
	case TitlePending, TitleAuto:
		prev.TitleBy = ""
	case TitleManual:
	default:
		return false, fmt.Errorf("store: title source %q", prev.TitleSource)
	}
	if client == "" {
		return false, nil
	}
	res, err := w.ExecContext(ctx,
		`UPDATE snippet SET title = ?, title_source = ?, title_by = ?
		 WHERE id = ? AND title = ? AND title_source = 'manual' AND title_by = ?`,
		prev.Title, prev.TitleSource, prev.TitleBy, id, agentTitle, client)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// Get returns one snippet.
func (s *Store) Get(ctx context.Context, id string) (Snippet, error) {
	return scanSnippet(s.r.QueryRowContext(ctx, `SELECT `+snippetCols+` FROM snippet s WHERE s.id = ?`, id))
}

// ErrAmbiguous means an ID prefix matches several snippets.
var ErrAmbiguous = errors.New("store: ID prefix is ambiguous")

// GetByPrefix returns the one snippet whose ID starts with prefix (case
// insensitive, as ULIDs are).
func (s *Store) GetByPrefix(ctx context.Context, prefix string) (Snippet, error) {
	prefix = strings.ToUpper(strings.TrimSpace(prefix))
	if prefix == "" {
		return Snippet{}, ErrNotFound
	}
	rows, err := s.r.QueryContext(ctx,
		`SELECT `+snippetCols+` FROM snippet s WHERE s.id >= ? AND s.id < ? ORDER BY s.id LIMIT 2`,
		prefix, prefix+"\U0010FFFF")
	if err != nil {
		return Snippet{}, err
	}
	defer rows.Close()
	var found []Snippet
	for rows.Next() {
		sn, err := scanSnippet(rows)
		if err != nil {
			return Snippet{}, err
		}
		found = append(found, sn)
	}
	if err := rows.Err(); err != nil {
		return Snippet{}, err
	}
	switch len(found) {
	case 0:
		return Snippet{}, ErrNotFound
	case 1:
		return found[0], nil
	}
	return Snippet{}, ErrAmbiguous
}

// ListOptions filters List.
type ListOptions struct {
	Limit      int
	Kind       Kind      // "" for any
	Since      time.Time // zero for any
	Until      time.Time // zero for any
	OpenOnly   bool
	ClosedOnly bool
}

func (o ListOptions) where(args *[]any) string {
	var conds []string
	if o.Kind != "" {
		conds = append(conds, "s.kind = ?")
		*args = append(*args, string(o.Kind))
	}
	if !o.Since.IsZero() {
		conds = append(conds, "s.modified_at >= ?")
		*args = append(*args, o.Since.UnixMilli())
	}
	if !o.Until.IsZero() {
		conds = append(conds, "s.modified_at < ?")
		*args = append(*args, o.Until.UnixMilli())
	}
	if o.OpenOnly {
		conds = append(conds, "s.id IN (SELECT snippet_id FROM workspace_tab)")
	}
	if o.ClosedOnly {
		conds = append(conds, "NOT EXISTS(SELECT 1 FROM workspace_tab w WHERE w.snippet_id = s.id)")
	}
	if len(conds) == 0 {
		return ""
	}
	return " AND " + strings.Join(conds, " AND ")
}

// List returns snippets, most recently modified first.
func (s *Store) List(ctx context.Context, o ListOptions) ([]Snippet, error) {
	var args []any
	q := `SELECT ` + snippetCols + ` FROM snippet s WHERE 1=1` + o.where(&args) + ` ORDER BY s.modified_at DESC`
	if o.Limit > 0 {
		q += ` LIMIT ?`
		args = append(args, o.Limit)
	}
	return s.querySnippets(ctx, q, args...)
}

func (s *Store) querySnippets(ctx context.Context, q string, args ...any) ([]Snippet, error) {
	rows, err := s.r.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Snippet
	for rows.Next() {
		sn, err := scanSnippet(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, sn)
	}
	return out, rows.Err()
}

// Count returns the number of snippets.
func (s *Store) Count(ctx context.Context) (int, error) {
	var n int
	err := s.r.QueryRowContext(ctx, `SELECT count(*) FROM snippet`).Scan(&n)
	return n, err
}

// Delete removes a snippet with its versions, tab and index rows.
func (s *Store) Delete(ctx context.Context, id string) error {
	w, err := s.writer()
	if err != nil {
		return err
	}
	res, err := w.ExecContext(ctx, `DELETE FROM snippet WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}
