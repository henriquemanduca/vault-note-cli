package cmd

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runCLI(t *testing.T, dbPath string, stdin string, args ...string) (int, string, string) {
	t.Helper()
	var out, errBuf bytes.Buffer
	fullArgs := append([]string{"--db", dbPath}, args...)
	code := Run(context.Background(), fullArgs, strings.NewReader(stdin), &out, &errBuf)
	return code, out.String(), errBuf.String()
}

func TestVersionAndHelpShortCircuitWithoutDBOrValidWorkspaceConfig(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	if err := os.WriteFile(filepath.Join(tmp, ".vaultnotecli"), []byte("BROKEN LINE\n"), 0o644); err != nil {
		t.Fatalf("write broken .vaultnotecli: %v", err)
	}
	origWD, _ := os.Getwd()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(origWD) })

	var out, errBuf bytes.Buffer
	if code := Run(context.Background(), []string{"version"}, strings.NewReader(""), &out, &errBuf); code != ExitOK {
		t.Fatalf("version failed (%d): %s", code, errBuf.String())
	}
	if !strings.Contains(out.String(), "vault-note-cli ") {
		t.Fatalf("unexpected version output: %q", out.String())
	}

	out.Reset()
	errBuf.Reset()
	if code := Run(context.Background(), []string{"note", "--help"}, strings.NewReader(""), &out, &errBuf); code != ExitOK {
		t.Fatalf("help failed (%d): %s", code, errBuf.String())
	}
	if !strings.Contains(out.String(), "vault-note-cli note") {
		t.Fatalf("unexpected help output: %q", out.String())
	}
}

func TestUnconfiguredVaultReturnsClearError(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("VAULT_NOTE_CLI_VAULT", "")
	dbPath := filepath.Join(tmp, "settings.db")

	code, _, errOut := runCLI(t, dbPath, "", "list")
	if code != ExitRuntime {
		t.Fatalf("expected ExitRuntime when vault unconfigured, got %d", code)
	}
	if !strings.Contains(errOut, "vault path not configured") {
		t.Fatalf("unexpected stderr: %q", errOut)
	}
}

func TestConfigNotebooksNotesEndToEnd(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("VAULT_NOTE_CLI_VAULT", "")
	dbPath := filepath.Join(tmp, "settings.db")
	vaultDir := filepath.Join(tmp, "meu-vault")

	// Empty config list produces zero output (no TSV header).
	code, out, errOut := runCLI(t, dbPath, "", "config", "get")
	if code != ExitOK || out != "" {
		t.Fatalf("empty config get = (%d, %q, %q)", code, out, errOut)
	}

	// Configure vault_path via CLI.
	code, out, errOut = runCLI(t, dbPath, "", "config", "set", "vault_path", vaultDir)
	if code != ExitOK || !strings.Contains(out, vaultDir) {
		t.Fatalf("config set failed: (%d, %q, %q)", code, out, errOut)
	}

	code, out, errOut = runCLI(t, dbPath, "", "config", "path")
	if code != ExitOK || strings.TrimSpace(out) != vaultDir {
		t.Fatalf("config path = (%d, %q, %q)", code, out, errOut)
	}

	// Empty note list produces zero output (no TSV header).
	code, out, errOut = runCLI(t, dbPath, "", "list")
	if code != ExitOK || out != "" {
		t.Fatalf("empty list = (%d, %q, %q)", code, out, errOut)
	}

	// Create notebook folder.
	code, _, errOut = runCLI(t, dbPath, "", "nn", "WORK")
	if code != ExitOK {
		t.Fatalf("nn WORK failed (%d): %s", code, errOut)
	}

	// Add note via stdin into WORK.
	code, out, errOut = runCLI(t, dbPath, "Linha 1\nLinha 2", "add", "Arquitetura", "-b", "WORK", "--stdin")
	if code != ExitOK || !strings.Contains(out, "WORK/Arquitetura.txt") {
		t.Fatalf("add note failed: (%d, %q, %q)", code, out, errOut)
	}

	// Add standalone note in vault root.
	code, _, errOut = runCLI(t, dbPath, "", "add", "Solta", "Texto rapido")
	if code != ExitOK {
		t.Fatalf("add root note failed (%d): %s", code, errOut)
	}

	// View raw content without .txt suffix.
	code, out, errOut = runCLI(t, dbPath, "", "view", "WORK/Arquitetura", "--raw")
	if code != ExitOK || strings.TrimSpace(out) != "Linha 1\nLinha 2" {
		t.Fatalf("view raw = (%d, %q, %q)", code, out, errOut)
	}

	// List in table format.
	code, out, errOut = runCLI(t, dbPath, "", "list", "-F", "table")
	if code != ExitOK || !strings.Contains(out, "WORK/Arquitetura.txt") || !strings.Contains(out, "Solta.txt") {
		t.Fatalf("list table = (%d, %q, %q)", code, out, errOut)
	}

	// Search by content.
	code, out, errOut = runCLI(t, dbPath, "", "search", "Linha 2", "-n")
	if code != ExitOK || !strings.Contains(out, "WORK/Arquitetura.txt") || strings.Contains(out, "Solta.txt") {
		t.Fatalf("search = (%d, %q, %q)", code, out, errOut)
	}

	// Rename & move note to root.
	code, out, errOut = runCLI(t, dbPath, "", "update", "WORK/Arquitetura", "Design", "-b", "none")
	if code != ExitOK || !strings.Contains(out, "Design.txt") {
		t.Fatalf("update failed: (%d, %q, %q)", code, out, errOut)
	}

	// Delete note -> moves to .trash/.
	code, out, errOut = runCLI(t, dbPath, "", "rm", "Design")
	if code != ExitOK || !strings.Contains(out, ".trash") {
		t.Fatalf("rm failed: (%d, %q, %q)", code, out, errOut)
	}

	// Rename notebook and delete empty notebook.
	code, _, errOut = runCLI(t, dbPath, "", "notebook", "rename", "WORK", "ARCHIVE")
	if code != ExitOK {
		t.Fatalf("notebook rename failed (%d): %s", code, errOut)
	}
	code, out, errOut = runCLI(t, dbPath, "", "notebook", "delete", "ARCHIVE")
	if code != ExitOK || !strings.Contains(out, ".trash") {
		t.Fatalf("notebook delete failed: (%d, %q, %q)", code, out, errOut)
	}
}

func TestWorkspaceConfigAndVaultFlagPrecedence(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("VAULT_NOTE_CLI_VAULT", "")
	dbPath := filepath.Join(tmp, "settings.db")
	vaultA := filepath.Join(tmp, "vaultA")
	vaultB := filepath.Join(tmp, "vaultB")

	// Workspace .vaultnotecli defines NOTEBOOK=WORK and VAULT_PATH=vaultA.
	ws := filepath.Join(tmp, "proj")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatalf("mkdir ws: %v", err)
	}
	cfgContent := "NOTEBOOK=WORK\nVAULT_PATH=" + vaultA + "\n"
	if err := os.WriteFile(filepath.Join(ws, ".vaultnotecli"), []byte(cfgContent), 0o644); err != nil {
		t.Fatalf("write .vaultnotecli: %v", err)
	}
	origWD, _ := os.Getwd()
	if err := os.Chdir(ws); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(origWD) })

	// Uses VAULT_PATH from .vaultnotecli automatically.
	if code, _, errOut := runCLI(t, dbPath, "", "nn", "WORK"); code != ExitOK {
		t.Fatalf("nn WORK: %s", errOut)
	}
	if code, out, errOut := runCLI(t, dbPath, "", "add", "DefaultNB", "body"); code != ExitOK || !strings.Contains(out, "WORK/DefaultNB.txt") {
		t.Fatalf("expected default WORK from .vaultnotecli: (%d, %q, %q)", code, out, errOut)
	}

	// Escape hatch -b "" creates note in vault root.
	if code, out, errOut := runCLI(t, dbPath, "", "add", "RootNote", "body", "-b", ""); code != ExitOK || !strings.Contains(out, "Created note RootNote.txt") {
		t.Fatalf("expected root note with -b '': (%d, %q, %q)", code, out, errOut)
	}

	// --vault flag overrides .vaultnotecli VAULT_PATH.
	code, out, errOut := runCLI(t, dbPath, "", "--vault", vaultB, "config", "path")
	if code != ExitOK || strings.TrimSpace(out) != vaultB {
		t.Fatalf("--vault override = (%d, %q, %q)", code, out, errOut)
	}
}
