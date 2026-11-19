package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
)

// Setting keys.
const (
	SettingAppearance  = "appearance"  // "system" | "light" | "dark"
	SettingLanguage    = "language"    // "system" | "zh-Hans" | "en"
	SettingAccent      = "accent"      // "#2F5BD3"
	SettingShortcuts   = "shortcuts"   // map[action]accelerator
	SettingEditorFont  = "editor.font" // font family
	SettingEditorSize  = "editor.fontSize"
	SettingHoldHint    = "holdHint"
	SettingTray        = "tray"
	SettingOpenAtLogin = "openAtLogin"
	SettingAgentAccess = "agent.access"
	SettingAgentMask   = "agent.mask"
	SettingAgentRaw    = "agent.allowRaw"
	SettingAgentRename = "agent.rename" // agents may rename titles: lastab retitle
)

// GetSetting decodes a setting's JSON value into v and reports whether it
// was set.
func (s *Store) GetSetting(ctx context.Context, key string, v any) (bool, error) {
	var raw string
	err := s.r.QueryRowContext(ctx, `SELECT value FROM setting WHERE key = ?`, key).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, json.Unmarshal([]byte(raw), v)
}

// SetSetting stores v as a setting's JSON value.
func (s *Store) SetSetting(ctx context.Context, key string, v any) error {
	w, err := s.writer()
	if err != nil {
		return err
	}
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = w.ExecContext(ctx,
		`INSERT INTO setting (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, string(b))
	return err
}
