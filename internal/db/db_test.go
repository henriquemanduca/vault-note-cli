package db

import (
	"errors"
	"path/filepath"
	"testing"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "vault-note-cli-test.db")
	s, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestDefaultDBPathHonorsEnvOverride(t *testing.T) {
	custom := filepath.Join(t.TempDir(), "custom.db")
	t.Setenv("VAULT_NOTE_CLI_DB", custom)

	got, err := DefaultDBPath()
	if err != nil {
		t.Fatalf("DefaultDBPath: %v", err)
	}
	if got != custom {
		t.Fatalf("DefaultDBPath = %q, want %q", got, custom)
	}
}

func TestSettingsCRUDAndVaultPath(t *testing.T) {
	s := newTestStore(t)

	// Unconfigured initially.
	vp, err := s.GetVaultPath()
	if err != nil {
		t.Fatalf("GetVaultPath: %v", err)
	}
	if vp != "" {
		t.Fatalf("expected empty initial vault path, got %q", vp)
	}

	// Empty path rejected.
	if _, err := s.SetVaultPath("   "); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for empty path, got %v", err)
	}

	// Tilde expansion.
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)
	saved, err := s.SetVaultPath("~/my-vault")
	if err != nil {
		t.Fatalf("SetVaultPath: %v", err)
	}
	want := filepath.Join(fakeHome, "my-vault")
	if saved != want {
		t.Fatalf("saved = %q, want %q", saved, want)
	}

	appCfg, err := s.GetAppSettings()
	if err != nil {
		t.Fatalf("GetAppSettings: %v", err)
	}
	if appCfg.VaultPath != want {
		t.Fatalf("AppSettings.VaultPath = %q, want %q", appCfg.VaultPath, want)
	}

	// Overwrite existing setting.
	other := filepath.Join(t.TempDir(), "vault-2")
	saved2, err := s.SetVaultPath(other)
	if err != nil {
		t.Fatalf("SetVaultPath overwrite: %v", err)
	}
	if saved2 != other {
		t.Fatalf("saved2 = %q, want %q", saved2, other)
	}

	// ListSettings returns saved entry.
	all, err := s.ListSettings()
	if err != nil {
		t.Fatalf("ListSettings: %v", err)
	}
	if len(all) != 1 || all[0].Key != KeyVaultPath || all[0].Value != other {
		t.Fatalf("unexpected settings list: %+v", all)
	}

	// DeleteSetting removes entry and returns ErrNotFound on second call.
	if err := s.DeleteSetting(KeyVaultPath); err != nil {
		t.Fatalf("DeleteSetting: %v", err)
	}
	if err := s.DeleteSetting(KeyVaultPath); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
