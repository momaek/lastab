package main

import (
	"encoding/json"
	"testing"
)

const base = `{"name": "Lastab", "macos": {"minimumSystemVersion": "12.0"}}`

func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func TestWithSigningUnchangedWithoutSecrets(t *testing.T) {
	out, notes, err := withSigning([]byte(base), env(nil))
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != base {
		t.Errorf("changed without secrets: %s", out)
	}
	if len(notes) != 2 {
		t.Errorf("notes %q", notes)
	}
}

func TestWithSigningFromEnvironment(t *testing.T) {
	out, _, err := withSigning([]byte(base), env(map[string]string{
		"MACOS_SIGNING_IDENTITY":   "Developer ID Application: A (TEAM)",
		"NOTARY_KEYCHAIN_PROFILE":  "notary",
		"NOTARY_KEYCHAIN":          "/tmp/k.keychain-db",
		"WINDOWS_CERTIFICATE_FILE": "/tmp/c.pfx",
	}))
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		Name  string
		MacOS struct {
			MinimumSystemVersion string
			SigningIdentity      string
			Notarize             struct{ KeychainProfile, Keychain string }
		}
		Windows struct{ Certificate string }
	}
	if err := json.Unmarshal(out, &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Name != "Lastab" || cfg.MacOS.MinimumSystemVersion != "12.0" {
		t.Errorf("lost settings: %s", out)
	}
	if cfg.MacOS.SigningIdentity != "Developer ID Application: A (TEAM)" ||
		cfg.MacOS.Notarize.KeychainProfile != "notary" || cfg.MacOS.Notarize.Keychain != "/tmp/k.keychain-db" {
		t.Errorf("macos %+v", cfg.MacOS)
	}
	if cfg.Windows.Certificate != "/tmp/c.pfx" {
		t.Errorf("windows %+v", cfg.Windows)
	}
}

func TestNotarizeNeedsIdentity(t *testing.T) {
	out, _, err := withSigning([]byte(base), env(map[string]string{"NOTARY_KEYCHAIN_PROFILE": "notary"}))
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != base {
		t.Errorf("notarizing an ad hoc build: %s", out)
	}
}
