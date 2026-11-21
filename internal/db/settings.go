package db

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Known persistent setting keys stored in SQLite.
const (
	KeyVaultPath = "vault_path"
)

// Setting represents a single key-value entry in the settings table.
type Setting struct {
	Key       string
	Value     string
	UpdatedAt string
}

// AppSettings is the typed snapshot of all application settings.
type AppSettings struct {
	VaultPath string
}

// GetSetting fetches a setting value by key.
// If the key is absent, it returns ("", false, nil).
func (s *Store) GetSetting(key string) (string, bool, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return "", false, fmt.Errorf("%w: setting key is required", ErrInvalidInput)
	}
	var val string
	err := s.db.QueryRow(`SELECT value FROM settings WHERE key = ?`, key).Scan(&val)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("query setting %q: %w", key, err)
	}
	return val, true, nil
}

// SetSetting upserts a key-value pair in the settings table.
func (s *Store) SetSetting(key, value string) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return fmt.Errorf("%w: setting key is required", ErrInvalidInput)
	}
	now := nowUTC()
	if _, err := s.db.Exec(
		`INSERT INTO settings (key, value, updated_at) VALUES (?, ?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
		key, value, now,
	); err != nil {
		return fmt.Errorf("save setting %q: %w", key, err)
	}
	return nil
}

// DeleteSetting removes a setting key from SQLite.
func (s *Store) DeleteSetting(key string) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return fmt.Errorf("%w: setting key is required", ErrInvalidInput)
	}
	res, err := s.db.Exec(`DELETE FROM settings WHERE key = ?`, key)
	if err != nil {
		return fmt.Errorf("delete setting %q: %w", key, err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("%w: %q", ErrNotFound, key)
	}
	return nil
}

// ListSettings returns all stored settings ordered by key ascending.
func (s *Store) ListSettings() ([]Setting, error) {
	rows, err := s.db.Query(`SELECT key, value, updated_at FROM settings ORDER BY key ASC`)
	if err != nil {
		return nil, fmt.Errorf("list settings: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []Setting
	for rows.Next() {
		var item Setting
		if err := rows.Scan(&item.Key, &item.Value, &item.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan setting: %w", err)
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

// GetAppSettings loads the typed application settings snapshot.
func (s *Store) GetAppSettings() (AppSettings, error) {
	vp, _, err := s.GetSetting(KeyVaultPath)
	if err != nil {
		return AppSettings{}, err
	}
	return AppSettings{VaultPath: vp}, nil
}

// GetVaultPath returns the configured vault path, or "" if not yet configured.
func (s *Store) GetVaultPath() (string, error) {
	vp, _, err := s.GetSetting(KeyVaultPath)
	return vp, err
}

// SetVaultPath normalizes rawPath (expanding ~ and resolving absolute path)
// and persists it under KeyVaultPath.
func (s *Store) SetVaultPath(rawPath string) (string, error) {
	abs, err := NormalizePath(rawPath)
	if err != nil {
		return "", err
	}
	if err := s.SetSetting(KeyVaultPath, abs); err != nil {
		return "", err
	}
	return abs, nil
}

// NormalizePath expands a leading ~/ (or bare ~) and returns an absolute clean path.
func NormalizePath(raw string) (string, error) {
	p := strings.TrimSpace(raw)
	if p == "" {
		return "", fmt.Errorf("%w: vault path cannot be empty", ErrInvalidInput)
	}
	if p == "~" || strings.HasPrefix(p, "~/") || strings.HasPrefix(p, `~\`) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home directory: %w", err)
		}
		if p == "~" {
			p = home
		} else {
			p = filepath.Join(home, p[2:])
		}
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", fmt.Errorf("resolve absolute path: %w", err)
	}
	return filepath.Clean(abs), nil
}
