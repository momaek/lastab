// Package paths computes Lastab's data directory. The desktop app and the
// lastab CLI share it, so it must not depend on MyGo.
package paths

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
)

// EnvDataDir overrides the data directory, for development and tests.
const EnvDataDir = "LASTAB_DATA_DIR"

const (
	AppName    = "Lastab"
	DevAppName = "Lastab Dev"

	DBFile       = "lastab.db"
	AgentLogFile = "agent_log.db"
	SessionLock  = "session.lock"
)

// DataDir returns the directory holding lastab.db. override wins over
// LASTAB_DATA_DIR, which wins over the platform default. dev selects the
// "Lastab Dev" directory, kept apart from an installed app's.
func DataDir(override string, dev bool) (string, error) {
	if override != "" {
		return filepath.Abs(override)
	}
	if env := os.Getenv(EnvDataDir); env != "" {
		return filepath.Abs(env)
	}
	name := AppName
	if dev {
		name = DevAppName
	}
	switch runtime.GOOS {
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, "Library", "Application Support", name), nil
	case "windows":
		// Local, not Roaming: snippets are not synced and must not follow a
		// domain account.
		if dir := os.Getenv("LOCALAPPDATA"); dir != "" {
			return filepath.Join(dir, name), nil
		}
		return "", errors.New("paths: LOCALAPPDATA is not set")
	default:
		dir, err := os.UserConfigDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(dir, name), nil
	}
}

// Ensure creates dir if needed and returns it.
func Ensure(dir string) (string, error) {
	return dir, os.MkdirAll(dir, 0o700)
}
