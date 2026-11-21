package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadDiscoversNearestUpwardAndStopsAtHome(t *testing.T) {
	root := t.TempDir()
	fakeHome := filepath.Join(root, "home")
	proj := filepath.Join(fakeHome, "work", "app")
	nested := filepath.Join(proj, "sub", "deep")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	t.Setenv("HOME", fakeHome)

	// Config outside HOME should not be read when cwd is inside HOME.
	if err := os.WriteFile(filepath.Join(root, FileName), []byte("NOTEBOOK=OUTSIDE\n"), 0o644); err != nil {
		t.Fatalf("write root cfg: %v", err)
	}
	cfg, err := Load(nested)
	if err != nil {
		t.Fatalf("Load without home cfg: %v", err)
	}
	if cfg.Notebook != "" || cfg.Path != "" {
		t.Fatalf("expected empty config when stopping at HOME, got %+v", cfg)
	}

	// Home config discovered.
	homeCfg := filepath.Join(fakeHome, FileName)
	if err := os.WriteFile(homeCfg, []byte("NOTEBOOK=HOME_NB\n"), 0o644); err != nil {
		t.Fatalf("write home cfg: %v", err)
	}
	cfg, err = Load(nested)
	if err != nil {
		t.Fatalf("Load home cfg: %v", err)
	}
	if cfg.Notebook != "HOME_NB" || cfg.Path != homeCfg {
		t.Fatalf("got %+v, want HOME_NB from %s", cfg, homeCfg)
	}

	// Closer project config wins and supports quotes, export prefix, VAULT_PATH, and comments.
	projCfg := filepath.Join(proj, FileName)
	content := "# workspace settings\nexport NOTEBOOK=\"WORK\"\nVAULT_PATH='/tmp/my-vault'\n"
	if err := os.WriteFile(projCfg, []byte(content), 0o644); err != nil {
		t.Fatalf("write proj cfg: %v", err)
	}
	cfg, err = Load(nested)
	if err != nil {
		t.Fatalf("Load proj cfg: %v", err)
	}
	if cfg.Notebook != "WORK" || cfg.VaultPath != "/tmp/my-vault" || cfg.Path != projCfg {
		t.Fatalf("got %+v, want WORK & /tmp/my-vault from %s", cfg, projCfg)
	}
}

func TestLoadAcceptsNotebookCodeAlias(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte("NOTEBOOK_CODE=LEGACY\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Notebook != "LEGACY" {
		t.Fatalf("Notebook = %q, want LEGACY", cfg.Notebook)
	}
}

func TestLoadStrictValidation(t *testing.T) {
	cases := []struct {
		name    string
		content string
		wantErr string
	}{
		{"unknown key", "FOO=bar\n", "unknown configuration key"},
		{"invalid syntax", "NOT_A_PAIR\n", "invalid syntax"},
		{"unterminated quote", "NOTEBOOK=\"WORK\n", "unterminated quoted value"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("HOME", dir)
			p := filepath.Join(dir, FileName)
			if err := os.WriteFile(p, []byte(tc.content), 0o644); err != nil {
				t.Fatalf("write: %v", err)
			}
			_, err := Load(dir)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) || !strings.Contains(err.Error(), p) {
				t.Fatalf("err = %v, want substring %q and path %q", err, tc.wantErr, p)
			}
		})
	}
}
