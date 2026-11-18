package store

import (
	"context"
	"database/sql"
)

type execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func openTabTx(ctx context.Context, tx execer, id string, position int, now int64) error {
	_, err := tx.ExecContext(ctx,
		`INSERT INTO workspace_tab (snippet_id, position, opened_at, is_active) VALUES (?, ?, ?, 0)
		 ON CONFLICT(snippet_id) DO NOTHING`, id, position, now)
	return err
}

// Workspace returns the open tabs in tab-bar order, and the active one's ID.
func (s *Store) Workspace(ctx context.Context) ([]Snippet, string, error) {
	rows, err := s.r.QueryContext(ctx, `SELECT `+snippetCols+`, w.is_active FROM workspace_tab w
		JOIN snippet s ON s.id = w.snippet_id ORDER BY w.position, w.opened_at`)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()
	var tabs []Snippet
	var active string
	for rows.Next() {
		var isActive bool
		sn, err := scanSnippet(rowWithExtra{rows, &isActive})
		if err != nil {
			return nil, "", err
		}
		if isActive {
			active = sn.ID
		}
		tabs = append(tabs, sn)
	}
	return tabs, active, rows.Err()
}

type rowWithExtra struct {
	r     *sql.Rows
	extra any
}

func (r rowWithExtra) Scan(dest ...any) error { return r.r.Scan(append(dest, r.extra)...) }

// OpenTab adds a snippet to the workspace at the end.
func (s *Store) OpenTab(ctx context.Context, id string) error {
	w, err := s.writer()
	if err != nil {
		return err
	}
	_, err = w.ExecContext(ctx,
		`INSERT INTO workspace_tab (snippet_id, position, opened_at, is_active)
		 VALUES (?, (SELECT coalesce(max(position), -1) + 1 FROM workspace_tab), ?, 0)
		 ON CONFLICT(snippet_id) DO NOTHING`, id, s.nowMs())
	return err
}

// CloseTab removes a snippet from the workspace. The snippet stays.
func (s *Store) CloseTab(ctx context.Context, id string) error {
	w, err := s.writer()
	if err != nil {
		return err
	}
	_, err = w.ExecContext(ctx, `DELETE FROM workspace_tab WHERE snippet_id = ?`, id)
	return err
}

// SetTabOrder writes the tab bar's order and the active tab ("" for a
// blank tab that has no snippet yet).
func (s *Store) SetTabOrder(ctx context.Context, ids []string, active string) error {
	w, err := s.writer()
	if err != nil {
		return err
	}
	tx, err := w.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for i, id := range ids {
		if _, err := tx.ExecContext(ctx,
			`UPDATE workspace_tab SET position = ?, is_active = (snippet_id = ?) WHERE snippet_id = ?`, i, active, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}
