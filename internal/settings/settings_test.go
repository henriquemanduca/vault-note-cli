package settings

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	s, err := NewStore(cfgPath)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	return s
}

func TestDefaultPathHonorsEnvOverride(t *testing.T) {
	custom := filepath.Join(t.TempDir(), "custom.yaml")
	t.Setenv("VAULT_NOTE_CLI_CONFIG", custom)

	got, err := DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath: %v", err)
	}
	if got != custom {
		t.Fatalf("DefaultPath = %q, want %q", got, custom)
	}
}

func TestSettingsYAMLCRUDAndVaultPath(t *testing.T) {
	s := newTestStore(t)

	// Missing file loads cleanly as empty settings.
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

	// Tilde expansion and YAML serialization.
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

	raw, err := os.ReadFile(s.Path())
	if err != nil {
		t.Fatalf("read config.yaml: %v", err)
	}
	if !strings.Contains(string(raw), "vault_path:") || !strings.Contains(string(raw), want) {
		t.Fatalf("unexpected YAML content:\n%s", string(raw))
	}

	// Reload from disk.
	cfg, err := s.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.VaultPath != want {
		t.Fatalf("Load.VaultPath = %q, want %q", cfg.VaultPath, want)
	}

	// Overwrite with another path.
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

	// DeleteSetting clears entry and returns ErrNotFound on second call.
	if err := s.DeleteSetting(KeyVaultPath); err != nil {
		t.Fatalf("DeleteSetting: %v", err)
	}
	if err := s.DeleteSetting(KeyVaultPath); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestMalformedYAMLReturnsErrorWithPath(t *testing.T) {
	s := newTestStore(t)
	if err := os.WriteFile(s.Path(), []byte(":::bad: [yaml\n"), 0o644); err != nil {
		t.Fatalf("write malformed yaml: %v", err)
	}
	_, err := s.Load()
	if err == nil || !strings.Contains(err.Error(), s.Path()) {
		t.Fatalf("expected error mentioning config path %q, got %v", s.Path(), err)
	}
}

func TestShowNotebookPrefixDefaultsAndPersistence(t *testing.T) {
	s := newTestStore(t)

	enabled, err := s.GetShowNotebookPrefix()
	if err != nil || !enabled {
		t.Fatalf("default = (%v, %v), want true", enabled, err)
	}
	cfg, err := s.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.ShowNotebookPrefixEnabled() {
		t.Fatalf("nil pointer should default to enabled")
	}

	if err := s.SetShowNotebookPrefix(false); err != nil {
		t.Fatalf("SetShowNotebookPrefix(false): %v", err)
	}
	enabled, err = s.GetShowNotebookPrefix()
	if err != nil || enabled {
		t.Fatalf("persisted = (%v, %v), want false", enabled, err)
	}
	if _, err := s.SetVaultPath(filepath.Join(t.TempDir(), "vault")); err != nil {
		t.Fatalf("SetVaultPath: %v", err)
	}
	enabled, err = s.GetShowNotebookPrefix()
	if err != nil || enabled {
		t.Fatalf("SetVaultPath lost prefix setting: (%v, %v)", enabled, err)
	}

	if err := s.SetShowNotebookPrefix(true); err != nil {
		t.Fatalf("SetShowNotebookPrefix(true): %v", err)
	}
	enabled, err = s.GetShowNotebookPrefix()
	if err != nil || !enabled {
		t.Fatalf("persisted = (%v, %v), want true", enabled, err)
	}
}

func TestShowNotebookPrefixGetSetDeleteAndList(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.SetVaultPath(filepath.Join(t.TempDir(), "vault")); err != nil {
		t.Fatalf("SetVaultPath: %v", err)
	}

	all, err := s.ListSettings()
	if err != nil {
		t.Fatalf("ListSettings: %v", err)
	}
	if len(all) != 1 || all[0].Key != KeyVaultPath {
		t.Fatalf("default list = %+v, want only vault_path", all)
	}

	val, ok, err := s.GetSetting(KeyShowNotebookPrefix)
	if err != nil || !ok || val != "true" {
		t.Fatalf("GetSetting default = (%q, %v, %v), want true", val, ok, err)
	}
	if err := s.SetSetting(KeyShowNotebookPrefix, "false"); err != nil {
		t.Fatalf("SetSetting false: %v", err)
	}
	val, ok, err = s.GetSetting(KeyShowNotebookPrefix)
	if err != nil || !ok || val != "false" {
		t.Fatalf("GetSetting after false = (%q, %v, %v)", val, ok, err)
	}

	all, err = s.ListSettings()
	if err != nil {
		t.Fatalf("ListSettings: %v", err)
	}
	if len(all) != 2 || all[0].Key != KeyShowNotebookPrefix || all[1].Key != KeyVaultPath {
		t.Fatalf("list order = %+v, want show_notebook_prefix then vault_path", all)
	}

	if err := s.DeleteSetting(KeyShowNotebookPrefix); err != nil {
		t.Fatalf("DeleteSetting: %v", err)
	}
	enabled, err := s.GetShowNotebookPrefix()
	if err != nil || !enabled {
		t.Fatalf("after delete default = (%v, %v), want true", enabled, err)
	}
	if err := s.DeleteSetting(KeyShowNotebookPrefix); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second delete = %v, want ErrNotFound", err)
	}
	all, err = s.ListSettings()
	if err != nil {
		t.Fatalf("ListSettings after delete: %v", err)
	}
	if len(all) != 1 || all[0].Key != KeyVaultPath {
		t.Fatalf("after delete list = %+v, want only vault_path", all)
	}
}

func TestParseBool(t *testing.T) {
	for _, raw := range []string{"true", "TRUE", "1", "on", "On"} {
		got, err := ParseBool(raw)
		if err != nil || !got {
			t.Fatalf("ParseBool(%q) = (%v, %v), want true", raw, got, err)
		}
	}
	for _, raw := range []string{"false", "FALSE", "0", "off", "Off"} {
		got, err := ParseBool(raw)
		if err != nil || got {
			t.Fatalf("ParseBool(%q) = (%v, %v), want false", raw, got, err)
		}
	}
	if _, err := ParseBool("maybe"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("ParseBool(maybe) = %v, want ErrInvalidInput", err)
	}
}

func TestShowLineNumbersDefaultsPersistenceAndOrder(t *testing.T) {
	s := newTestStore(t)

	// Default is false when unset.
	enabled, err := s.GetShowLineNumbers()
	if err != nil || enabled {
		t.Fatalf("default = (%v, %v), want false", enabled, err)
	}
	val, ok, err := s.GetSetting(KeyShowLineNumbers)
	if err != nil || !ok || val != "false" {
		t.Fatalf("GetSetting default = (%q, %v, %v), want false", val, ok, err)
	}

	if _, err := s.SetVaultPath(filepath.Join(t.TempDir(), "vault")); err != nil {
		t.Fatalf("SetVaultPath: %v", err)
	}
	if err := s.SetShowNotebookPrefix(false); err != nil {
		t.Fatalf("SetShowNotebookPrefix: %v", err)
	}
	if err := s.SetSetting(KeyShowLineNumbers, "true"); err != nil {
		t.Fatalf("SetSetting true: %v", err)
	}
	enabled, err = s.GetShowLineNumbers()
	if err != nil || !enabled {
		t.Fatalf("persisted = (%v, %v), want true", enabled, err)
	}

	// ListSettings maintains ascending key order.
	all, err := s.ListSettings()
	if err != nil {
		t.Fatalf("ListSettings: %v", err)
	}
	if len(all) != 3 || all[0].Key != KeyShowLineNumbers || all[1].Key != KeyShowNotebookPrefix || all[2].Key != KeyVaultPath {
		t.Fatalf("unexpected settings order: %+v", all)
	}

	if err := s.DeleteSetting(KeyShowLineNumbers); err != nil {
		t.Fatalf("DeleteSetting: %v", err)
	}
	if err := s.DeleteSetting(KeyShowLineNumbers); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second delete = %v, want ErrNotFound", err)
	}
}
