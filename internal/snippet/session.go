package snippet

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Session marks a running app with session.lock, holding its PID. A lock
// left by a process no longer running means the last run ended abnormally.
type Session struct {
	path string
}

// StartSession writes the lock in dir and reports whether the previous run
// crashed.
func StartSession(dir string) (*Session, bool, error) {
	path := filepath.Join(dir, "session.lock")
	crashed := false
	if b, err := os.ReadFile(path); err == nil {
		pid, _ := strconv.Atoi(strings.TrimSpace(string(b)))
		crashed = pid > 0 && pid != os.Getpid() && !processAlive(pid)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil, false, err
	}
	if err := os.WriteFile(path, []byte(strconv.Itoa(os.Getpid())), 0o600); err != nil {
		return nil, false, err
	}
	return &Session{path: path}, crashed, nil
}

// End removes the lock, on a normal quit.
func (s *Session) End() error {
	if s == nil {
		return nil
	}
	err := os.Remove(s.path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}
