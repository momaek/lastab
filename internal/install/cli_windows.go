//go:build windows

package install

import (
	"errors"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// CLIStatus says whether the app's bin folder, which holds lastab.exe, is
// on the user's PATH.
func (e Env) CLIStatus() Status {
	st := Status{State: Missing, Path: e.CLI}
	if e.CLI == "" {
		return st
	}
	if list, _, err := readUserPath(); err == nil && pathListHas(list, filepath.Dir(e.CLI), expandEnv) {
		st.State = Installed
	}
	return st
}

// InstallCLI adds the app's bin folder to the user's PATH (HKCU\Environment)
// and tells running programs; terminals opened from then on find lastab.
// admin and prompt are for macOS: the user's PATH needs no administrator.
func (e Env) InstallCLI(admin bool, prompt string) (Status, error) {
	if err := e.cliExists(); err != nil {
		return e.CLIStatus(), err
	}
	dir := filepath.Dir(e.CLI)
	k, err := registry.OpenKey(registry.CURRENT_USER, "Environment", registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return e.CLIStatus(), &DirError{`HKEY_CURRENT_USER\Environment`, err}
	}
	defer k.Close()
	list, typ, err := k.GetStringValue("Path")
	if err != nil && !errors.Is(err, registry.ErrNotExist) {
		return e.CLIStatus(), &DirError{`HKEY_CURRENT_USER\Environment`, err}
	}
	if next, changed := pathListAdd(list, dir, expandEnv); changed {
		if typ == registry.SZ {
			err = k.SetStringValue("Path", next)
		} else {
			err = k.SetExpandStringValue("Path", next)
		}
		if err != nil {
			return e.CLIStatus(), &DirError{`HKEY_CURRENT_USER\Environment`, err}
		}
	}
	broadcastEnvironment()
	return e.CLIStatus(), nil
}

func readUserPath() (string, uint32, error) {
	k, err := registry.OpenKey(registry.CURRENT_USER, "Environment", registry.QUERY_VALUE)
	if err != nil {
		return "", 0, err
	}
	defer k.Close()
	return k.GetStringValue("Path")
}

func expandEnv(s string) string {
	if v, err := registry.ExpandString(s); err == nil {
		return v
	}
	return s
}

// broadcastEnvironment tells top-level windows, Explorer among them, that
// the environment changed, so programs started from then on see the PATH.
func broadcastEnvironment() {
	const (
		hwndBroadcast    = 0xFFFF
		wmSettingChange  = 0x001A
		smtoAbortIfHung  = 0x0002
		broadcastTimeout = 5000
	)
	env, err := windows.UTF16PtrFromString("Environment")
	if err != nil {
		return
	}
	var result uintptr
	windows.NewLazySystemDLL("user32.dll").NewProc("SendMessageTimeoutW").Call(
		hwndBroadcast, wmSettingChange, 0, uintptr(unsafe.Pointer(env)),
		smtoAbortIfHung, broadcastTimeout, uintptr(unsafe.Pointer(&result)))
}
