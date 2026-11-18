// Package store opens lastab.db and reads and writes snippets, versions,
// the workspace and settings. It does not depend on MyGo: the lastab CLI
// uses it too, read-only.
package store

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// ErrTooNew means the database was written by a newer Lastab.
var ErrTooNew = errors.New("store: database is newer than this program")

// ErrNotFound means no snippet has the ID.
var ErrNotFound = errors.New("store: not found")

// ErrConflict means a save was based on an older revision.
var ErrConflict = errors.New("store: revision conflict")

// ErrNoDatabase means a read-only open found no database file.
var ErrNoDatabase = errors.New("store: database does not exist")

// Store is an open lastab.db. The app opens it read-write: writes go
// through one connection, so they are serialized, and reads use a pool.
// The CLI opens it read-only.
type Store struct {
	path     string
	w        *sql.DB // nil when read-only
	r        *sql.DB
	readOnly bool
	now      func() time.Time
}

// Open opens (creating and migrating) the database at path for the app.
func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	w, err := sql.Open("sqlite", dsn(path, false))
	if err != nil {
		return nil, err
	}
	w.SetMaxOpenConns(1)
	s := &Store{path: path, w: w, now: time.Now}
	if err := s.migrate(context.Background()); err != nil {
		w.Close()
		return nil, err
	}
	r, err := sql.Open("sqlite", dsn(path, true))
	if err != nil {
		w.Close()
		return nil, err
	}
	r.SetMaxOpenConns(4)
	s.r = r
	return s, nil
}

// OpenReadOnly opens an existing database for reading, as the CLI does. It
// never creates or migrates, and refuses a database newer than it knows.
func OpenReadOnly(path string) (*Store, error) {
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, ErrNoDatabase
		}
		return nil, err
	}
	r, err := sql.Open("sqlite", dsn(path, true))
	if err != nil {
		return nil, err
	}
	s := &Store{path: path, r: r, readOnly: true, now: time.Now}
	v, err := s.UserVersion(context.Background())
	if err != nil {
		r.Close()
		return nil, err
	}
	if v > LatestVersion() {
		r.Close()
		return nil, fmt.Errorf("%w (database v%d, supported v%d)", ErrTooNew, v, LatestVersion())
	}
	return s, nil
}

func dsn(path string, readOnly bool) string {
	q := url.Values{}
	q.Add("_pragma", "foreign_keys(1)")
	if readOnly {
		q.Set("mode", "ro")
		q.Add("_pragma", "busy_timeout(2000)")
		q.Add("_pragma", "query_only(1)")
	} else {
		q.Add("_pragma", "busy_timeout(5000)")
		q.Add("_pragma", "journal_mode(WAL)")
		q.Add("_pragma", "synchronous(NORMAL)")
	}
	return "file:" + filepath.ToSlash(path) + "?" + q.Encode()
}

// Path returns the database file.
func (s *Store) Path() string { return s.path }

// SetClock replaces the clock, for tests.
func (s *Store) SetClock(now func() time.Time) { s.now = now }

func (s *Store) nowMs() int64 { return s.now().UnixMilli() }

// Close closes the database.
func (s *Store) Close() error {
	var err error
	if s.w != nil {
		err = s.w.Close()
	}
	if s.r != nil {
		err = errors.Join(err, s.r.Close())
	}
	return err
}

func (s *Store) writer() (*sql.DB, error) {
	if s.readOnly {
		return nil, errors.New("store: opened read-only")
	}
	return s.w, nil
}

// UserVersion returns PRAGMA user_version.
func (s *Store) UserVersion(ctx context.Context) (int, error) {
	db := s.r
	if db == nil {
		db = s.w
	}
	var v int
	err := db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&v)
	return v, err
}

type migration struct {
	version int
	name    string
	sql     string
}

func migrations() []migration {
	entries, _ := migrationFS.ReadDir("migrations")
	var ms []migration
	for _, e := range entries {
		name := e.Name()
		n, err := strconv.Atoi(strings.SplitN(name, "_", 2)[0])
		if err != nil {
			panic("store: bad migration name " + name)
		}
		b, _ := migrationFS.ReadFile("migrations/" + name)
		ms = append(ms, migration{n, name, string(b)})
	}
	sort.Slice(ms, func(i, j int) bool { return ms[i].version < ms[j].version })
	return ms
}

// LatestVersion is the schema version this build migrates to.
func LatestVersion() int {
	ms := migrations()
	return ms[len(ms)-1].version
}

func (s *Store) migrate(ctx context.Context) error {
	var cur int
	if err := s.w.QueryRowContext(ctx, "PRAGMA user_version").Scan(&cur); err != nil {
		return err
	}
	if cur > LatestVersion() {
		return fmt.Errorf("%w (database v%d, supported v%d)", ErrTooNew, cur, LatestVersion())
	}
	for _, m := range migrations() {
		if m.version <= cur {
			continue
		}
		tx, err := s.w.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, m.sql); err != nil {
			tx.Rollback()
			return fmt.Errorf("store: migration %s: %w", m.name, err)
		}
		if _, err := tx.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d", m.version)); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}
