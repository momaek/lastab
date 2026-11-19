//go:build !windows

package install

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func newEnv(t *testing.T) Env {
	t.Helper()
	root := t.TempDir()
	cli := filepath.Join(root, "Lastab.app", "Contents", "Resources", "bin", "lastab")
	os.MkdirAll(filepath.Dir(cli), 0o755)
	if err := os.WriteFile(cli, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return Env{Home: filepath.Join(root, "home"), CLI: cli, GlobalBin: filepath.Join(root, "usr", "local", "bin")}
}

func TestCLIInstallGlobal(t *testing.T) {
	e := newEnv(t)
	if st := e.CLIStatus(); st.State != Missing || st.Path != e.globalLink() || st.Elevate {
		t.Fatalf("fresh %+v", st)
	}
	st, err := e.InstallCLI(false, "")
	if err != nil || st.State != Installed || st.Path != e.globalLink() || st.NeedsPath {
		t.Fatalf("install %+v %v", st, err)
	}
	if target, _ := os.Readlink(e.globalLink()); target != e.CLI {
		t.Errorf("link to %q", target)
	}
	// Again: replaces its own link.
	if st, err := e.InstallCLI(false, ""); err != nil || st.State != Installed {
		t.Errorf("reinstall %+v %v", st, err)
	}
}

func TestCLIFallsBackToLocalBin(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root writes anywhere")
	}
	e := newEnv(t)
	parent := filepath.Dir(e.GlobalBin) // usr/local, without bin, read-only
	os.MkdirAll(parent, 0o755)
	os.Chmod(parent, 0o555)
	t.Cleanup(func() { os.Chmod(parent, 0o755) })
	if st := e.CLIStatus(); !st.Elevate {
		t.Errorf("Elevate not offered: %+v", st)
	}
	st, err := e.InstallCLI(false, "")
	if err != nil || st.State != Installed || st.Path != e.localLink() || !st.NeedsPath || !st.Elevate {
		t.Fatalf("fallback %+v %v", st, err)
	}
}

func TestCLILeavesOtherFilesAlone(t *testing.T) {
	e := newEnv(t)
	os.MkdirAll(e.GlobalBin, 0o755)
	os.WriteFile(e.globalLink(), []byte("someone else's"), 0o755)
	_, err := e.InstallCLI(false, "")
	var ee *ExistsError
	if !errors.As(err, &ee) || ee.Path != e.globalLink() {
		t.Errorf("regular file: %v", err)
	}
	os.Remove(e.globalLink())
	os.Symlink("/opt/other/lastab", e.globalLink())
	if _, err := e.InstallCLI(false, ""); !errors.As(err, &ee) {
		t.Errorf("foreign link: %v", err)
	}
}

func TestCLIOutdatedLink(t *testing.T) {
	e := newEnv(t)
	os.MkdirAll(e.GlobalBin, 0o755)
	os.Symlink("/Volumes/Old/Lastab.app/Contents/Resources/bin/lastab", e.globalLink())
	if st := e.CLIStatus(); st.State != Outdated || st.Path != e.globalLink() {
		t.Fatalf("status %+v", st)
	}
	if st, err := e.InstallCLI(false, ""); err != nil || st.State != Installed {
		t.Errorf("reinstall %+v %v", st, err)
	}
}

func TestCLIMissingFromApp(t *testing.T) {
	e := newEnv(t)
	os.Remove(e.CLI)
	if _, err := e.InstallCLI(false, ""); !errors.Is(err, ErrNoCLI) {
		t.Errorf("err %v", err)
	}
	if _, err := (Env{}).InstallCLI(false, ""); !errors.Is(err, ErrNoCLI) {
		t.Errorf("no path: %v", err)
	}
}

func TestCLIRefusesTranslocatedApp(t *testing.T) {
	e := newEnv(t)
	moved := filepath.Join(filepath.Dir(e.CLI), "AppTranslocation", "x")
	os.MkdirAll(moved, 0o755)
	cli := filepath.Join(moved, "lastab")
	os.WriteFile(cli, nil, 0o755)
	e.CLI = cli
	if _, err := e.InstallCLI(false, ""); !errors.Is(err, ErrNotInstalled) {
		t.Errorf("err %v", err)
	}
}
