package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// Reason says why a version was recorded.
type Reason string

const (
	ReasonPaused  Reason = "paused"
	ReasonSwitch  Reason = "switch"
	ReasonClose   Reason = "close"
	ReasonCopy    Reason = "copy"
	ReasonRestore Reason = "restore"
)

// Version is a snapshot of a snippet's body.
type Version struct {
	ID           string
	SnippetID    string
	Body         string
	CreatedAt    time.Time
	Reason       Reason
	LinesChanged int
}

// LatestVersion returns the newest version of a snippet, or ErrNotFound.
func (s *Store) LatestVersion(ctx context.Context, snippetID string) (Version, error) {
	var v Version
	var created int64
	err := s.r.QueryRowContext(ctx,
		`SELECT id, snippet_id, body, created_at, reason, lines_changed FROM version
		 WHERE snippet_id = ? ORDER BY created_at DESC, id DESC LIMIT 1`, snippetID).
		Scan(&v.ID, &v.SnippetID, &v.Body, &created, &v.Reason, &v.LinesChanged)
	if errors.Is(err, sql.ErrNoRows) {
		return v, ErrNotFound
	}
	v.CreatedAt = time.UnixMilli(created)
	return v, err
}

// AddVersion records a version.
func (s *Store) AddVersion(ctx context.Context, snippetID, body string, reason Reason, linesChanged int) (Version, error) {
	w, err := s.writer()
	if err != nil {
		return Version{}, err
	}
	now := s.now()
	v := Version{ID: NewID(now), SnippetID: snippetID, Body: body, CreatedAt: time.UnixMilli(now.UnixMilli()), Reason: reason, LinesChanged: linesChanged}
	_, err = w.ExecContext(ctx,
		`INSERT INTO version (id, snippet_id, body, created_at, reason, lines_changed) VALUES (?, ?, ?, ?, ?, ?)`,
		v.ID, v.SnippetID, v.Body, now.UnixMilli(), v.Reason, v.LinesChanged)
	return v, err
}

// Versions returns a snippet's versions, newest first.
func (s *Store) Versions(ctx context.Context, snippetID string) ([]Version, error) {
	rows, err := s.r.QueryContext(ctx,
		`SELECT id, snippet_id, body, created_at, reason, lines_changed FROM version
		 WHERE snippet_id = ? ORDER BY created_at DESC, id DESC`, snippetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Version
	for rows.Next() {
		var v Version
		var created int64
		if err := rows.Scan(&v.ID, &v.SnippetID, &v.Body, &created, &v.Reason, &v.LinesChanged); err != nil {
			return nil, err
		}
		v.CreatedAt = time.UnixMilli(created)
		out = append(out, v)
	}
	return out, rows.Err()
}
