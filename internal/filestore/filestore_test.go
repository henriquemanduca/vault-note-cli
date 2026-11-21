package filestore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBaseDirPerOS(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmp)
	t.Setenv("LOCALAPPDATA", tmp)

	linuxDir, err := baseDirForOS("linux")
	if err != nil {
		t.Fatalf("linux BaseDir: %v", err)
	}
	if want := filepath.Join(tmp, "vault-note-cli"); linuxDir != want {
		t.Fatalf("linux dir = %q, want %q", linuxDir, want)
	}

	winDir, err := baseDirForOS("windows")
	if err != nil {
		t.Fatalf("windows BaseDir: %v", err)
	}
	if want := filepath.Join(tmp, "vault-note-cli"); winDir != want {
		t.Fatalf("windows dir = %q, want %q", winDir, want)
	}

	macDir, err := baseDirForOS("darwin")
	if err != nil {
		t.Fatalf("darwin BaseDir: %v", err)
	}
	if !strings.HasSuffix(macDir, filepath.Join("Library", "Application Support", "vault-note-cli")) {
		t.Fatalf("unexpected darwin dir %q", macDir)
	}

	dbPath, err := DefaultDBPath()
	if err != nil {
		t.Fatalf("DefaultDBPath: %v", err)
	}
	if !strings.HasSuffix(dbPath, filepath.Join("vault-note-cli", "vault-note-cli.db")) {
		t.Fatalf("unexpected dbPath %q", dbPath)
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
