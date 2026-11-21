package filestore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigDirPerOS(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	t.Setenv("APPDATA", tmp)

	linuxDir, err := configDirForOS("linux")
	if err != nil {
		t.Fatalf("linux ConfigDir: %v", err)
	}
	if want := filepath.Join(tmp, "vault-note-cli"); linuxDir != want {
		t.Fatalf("linux dir = %q, want %q", linuxDir, want)
	}

	winDir, err := configDirForOS("windows")
	if err != nil {
		t.Fatalf("windows ConfigDir: %v", err)
	}
	if want := filepath.Join(tmp, "vault-note-cli"); winDir != want {
		t.Fatalf("windows dir = %q, want %q", winDir, want)
	}

	macDir, err := configDirForOS("darwin")
	if err != nil {
		t.Fatalf("darwin ConfigDir: %v", err)
	}
	if !strings.HasSuffix(macDir, filepath.Join("Library", "Application Support", "vault-note-cli")) {
		t.Fatalf("unexpected darwin dir %q", macDir)
	}

	cfgPath, err := DefaultConfigPath()
	if err != nil {
		t.Fatalf("DefaultConfigPath: %v", err)
	}
	if !strings.HasSuffix(cfgPath, filepath.Join("vault-note-cli", "config.yaml")) {
		t.Fatalf("unexpected cfgPath %q", cfgPath)
	}
}

func TestDefaultVaultSuggestion(t *testing.T) {
	got := DefaultVaultSuggestion()
	if strings.TrimSpace(got) == "" {
		t.Fatalf("expected non-empty vault suggestion")
	}
	if !strings.HasSuffix(got, "vault-notes") {
		t.Fatalf("unexpected suggestion %q", got)
	}
}

func TestOpenFolderHeadlessFallback(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("DISPLAY", "")
	t.Setenv("WAYLAND_DISPLAY", "")
	t.Setenv("SSH_CONNECTION", "1")

	res, err := OpenFolder(tmp)
	if err != nil {
		t.Fatalf("OpenFolder: %v", err)
	}
	if res.Opened {
		t.Fatalf("expected headless fallback, got Opened=true")
	}
	if res.Path != tmp {
		t.Fatalf("OpenFolder path = %q, want %q", res.Path, tmp)
	}

	file := filepath.Join(tmp, "hello.txt")
	if err := os.WriteFile(file, []byte("ok"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	fileRes, err := OpenFile(file)
	if err != nil {
		t.Fatalf("OpenFile: %v", err)
	}
	if fileRes.Opened || fileRes.Path != file {
		t.Fatalf("unexpected OpenFile result: %+v", fileRes)
	}
}
