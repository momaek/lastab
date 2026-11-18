// Package agentlog records each read of snippets by an agent through the
// lastab CLI, in agent_log.db, so the main database stays read-only to the
// CLI. Settings' access log reads it.
package agentlog

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS access (
  id          INTEGER PRIMARY KEY,
  at          INTEGER NOT NULL,       -- Unix ms
  client      TEXT NOT NULL,          -- claude-code, unknown, ...
  op          TEXT NOT NULL,          -- list | search | read
  args        TEXT NOT NULL,          -- JSON
  snippet_ids TEXT NOT NULL           -- JSON array
);
CREATE INDEX IF NOT EXISTS access_at ON access(at DESC);
`

// Entry is one access.
type Entry struct {
	At         time.Time
	Client     string
	Op         string
	Args       map[string]any
	SnippetIDs []string
}

// Log is an open agent_log.db.
type Log struct{ db *sql.DB }

// Open opens or creates the log at path.
func Open(path string) (*Log, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	q := url.Values{}
	q.Add("_pragma", "busy_timeout(2000)")
	q.Add("_pragma", "journal_mode(WAL)")
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(path)+"?"+q.Encode())
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}
	return &Log{db}, nil
}

// Close closes the log.
func (l *Log) Close() error { return l.db.Close() }

// Add records an access.
func (l *Log) Add(ctx context.Context, e Entry) error {
	args, _ := json.Marshal(e.Args)
	ids, _ := json.Marshal(e.SnippetIDs)
	if e.SnippetIDs == nil {
		ids = []byte("[]")
	}
	_, err := l.db.ExecContext(ctx, `INSERT INTO access (at, client, op, args, snippet_ids) VALUES (?, ?, ?, ?, ?)`,
		e.At.UnixMilli(), e.Client, e.Op, string(args), string(ids))
	return err
}

// Recent returns the newest entries.
func (l *Log) Recent(ctx context.Context, limit int) ([]Entry, error) {
	rows, err := l.db.QueryContext(ctx, `SELECT at, client, op, args, snippet_ids FROM access ORDER BY at DESC, id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Entry
	for rows.Next() {
		var e Entry
		var at int64
		var args, ids string
		if err := rows.Scan(&at, &e.Client, &e.Op, &args, &ids); err != nil {
			return nil, err
		}
		e.At = time.UnixMilli(at)
		json.Unmarshal([]byte(args), &e.Args)
		json.Unmarshal([]byte(ids), &e.SnippetIDs)
		out = append(out, e)
	}
	return out, rows.Err()
}
