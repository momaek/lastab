//go:build !windows

package install

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
)

func (e Env) globalLink() string { return filepath.Join(e.GlobalBin, "lastab") }
func (e Env) localLink() string  { return filepath.Join(e.Home, ".local", "bin", "lastab") }

// CLIStatus finds the link to the command: in /usr/local/bin, or else in
// ~/.local/bin. One to another copy of Lastab, as one moved or replaced,
// is Outdated.
func (e Env) CLIStatus() Status {
	var outdated *Status
	for _, p := range []string{e.globalLink(), e.localLink()} {
		target, err := os.Readlink(p)
		if err != nil {
			continue
		}
		st := Status{Path: p, NeedsPath: p == e.localLink()}
		switch {
		case e.isCLI(target):
			st.State = Installed
			return st
		case isLastab(target) && outdated == nil:
			st.State = Outdated
			outdated = &st
		}
	}
	if outdated != nil {
		return *outdated
	}
	return Status{State: Missing, Path: e.globalLink(), Elevate: !writable(e.GlobalBin)}
}

// InstallCLI links the command into /usr/local/bin. Without the right to
// write there it links it into ~/.local/bin instead (NeedsPath), unless
// admin is set: then the system asks for an administrator's password, on
// the user's click, and the link goes in /usr/local/bin. prompt is the
// message of that dialog.
func (e Env) InstallCLI(admin bool, prompt string) (Status, error) {
	if err := e.cliExists(); err != nil {
		return e.CLIStatus(), err
	}
	global := e.globalLink()
	if admin {
		if err := e.checkReplaceable(global); err != nil {
			return e.CLIStatus(), err
		}
		if err := adminLink(e.CLI, global, prompt); err != nil {
			return e.CLIStatus(), err
		}
		return e.CLIStatus(), nil
	}
	err := e.link(global)
	if err == nil {
		return e.CLIStatus(), nil
	}
	if !errors.Is(err, fs.ErrPermission) && !errors.Is(err, syscall.EROFS) {
		return e.CLIStatus(), err
	}
	if err := e.link(e.localLink()); err != nil {
		return e.CLIStatus(), err
	}
	st := e.CLIStatus()
	st.Elevate = true
	return st, nil
}

// link points dst at the command, through a link made beside it and
// renamed over it, so dst is never missing.
func (e Env) link(dst string) error {
	if err := e.checkReplaceable(dst); err != nil {
		return err
	}
	dir := filepath.Dir(dst)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return &DirError{dir, cause(err)}
	}
	tmp := filepath.Join(dir, fmt.Sprintf(".lastab-%d", os.Getpid()))
	os.Remove(tmp)
	if err := os.Symlink(e.CLI, tmp); err != nil {
		return &DirError{dir, cause(err)}
	}
	if err := os.Rename(tmp, dst); err != nil {
		os.Remove(tmp)
		return &DirError{dir, cause(err)}
	}
	return nil
}

// checkReplaceable refuses to replace anything at dst but a link to a
// copy of Lastab.
func (e Env) checkReplaceable(dst string) error {
	fi, err := os.Lstat(dst)
	if err != nil {
		return nil // nothing there
	}
	if fi.Mode()&fs.ModeSymlink != 0 {
		if target, err := os.Readlink(dst); err == nil && (e.isCLI(target) || isLastab(target)) {
			return nil
		}
	}
	return &ExistsError{dst}
}

func (e Env) isCLI(target string) bool {
	if target == e.CLI {
		return true
	}
	a, err1 := filepath.EvalSymlinks(target)
	b, err2 := filepath.EvalSymlinks(e.CLI)
	return err1 == nil && err2 == nil && a == b
}

// isLastab says whether a path is the command inside some copy of the app,
// or of a development build.
func isLastab(path string) bool {
	return strings.HasSuffix(path, ".app/Contents/Resources/bin/lastab") ||
		(filepath.Base(path) == "lastab" && strings.Contains(path, "/resources/"))
}

// writable says whether the user can make files in dir, or make dir.
func writable(dir string) bool {
	for d := dir; ; d = filepath.Dir(d) {
		if _, err := os.Stat(d); err == nil {
			return syscall.Access(d, 2 /* W_OK */) == nil
		}
		if d == filepath.Dir(d) {
			return false
		}
	}
}

// adminLink makes the link as an administrator, through the system's
// password dialog.
func adminLink(src, dst, prompt string) error {
	if runtime.GOOS != "darwin" {
		return errors.New("installing as an administrator needs macOS")
	}
	sh := fmt.Sprintf("/bin/mkdir -p %s && /bin/ln -sfh %s %s", shellQuote(filepath.Dir(dst)), shellQuote(src), shellQuote(dst))
	script := fmt.Sprintf("do shell script %s with prompt %s with administrator privileges", appleString(sh), appleString(prompt))
	out, err := exec.Command("/usr/bin/osascript", "-e", script).CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if strings.Contains(msg, "-128") {
			return ErrCanceled
		}
		if msg == "" {
			msg = err.Error()
		}
		return errors.New(msg)
	}
	return nil
}

func shellQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

func appleString(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`
}
