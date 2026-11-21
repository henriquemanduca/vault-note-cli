package cmd

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runCLI(t *testing.T, configPath string, stdin string, args ...string) (int, string, string) {
	t.Helper()
	var out, errBuf bytes.Buffer
	fullArgs := append([]string{"--config", configPath}, args...)
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
	cfgPath := filepath.Join(tmp, "config.yaml")

	code, _, errOut := runCLI(t, cfgPath, "", "list")
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
	cfgPath := filepath.Join(tmp, "config.yaml")
	vaultDir := filepath.Join(tmp, "meu-vault")

	// Empty config list produces zero output (no TSV header).
	code, out, errOut := runCLI(t, cfgPath, "", "config", "get")
	if code != ExitOK || out != "" {
		t.Fatalf("empty config get = (%d, %q, %q)", code, out, errOut)
	}

	// Configure vault_path via CLI.
	code, out, errOut = runCLI(t, cfgPath, "", "config", "set", "vault_path", vaultDir)
	if code != ExitOK || !strings.Contains(out, vaultDir) {
		t.Fatalf("config set failed: (%d, %q, %q)", code, out, errOut)
	}

	rawYAML, err := os.ReadFile(cfgPath)
	if err != nil || !strings.Contains(string(rawYAML), "vault_path:") {
		t.Fatalf("expected YAML file at %s, got (%q, %v)", cfgPath, string(rawYAML), err)
	}

	code, out, errOut = runCLI(t, cfgPath, "", "config", "path")
	if code != ExitOK || strings.TrimSpace(out) != vaultDir {
		t.Fatalf("config path = (%d, %q, %q)", code, out, errOut)
	}

	// Empty note list produces zero output (no TSV header).
	code, out, errOut = runCLI(t, cfgPath, "", "list")
	if code != ExitOK || out != "" {
		t.Fatalf("empty list = (%d, %q, %q)", code, out, errOut)
	}

	// Create notebook folder.
	code, _, errOut = runCLI(t, cfgPath, "", "nn", "WORK")
	if code != ExitOK {
		t.Fatalf("nn WORK failed (%d): %s", code, errOut)
	}

	// Add note via stdin into WORK.
	code, out, errOut = runCLI(t, cfgPath, "Linha 1\nLinha 2", "add", "Arquitetura", "-b", "WORK", "--stdin")
	if code != ExitOK || !strings.Contains(out, "WORK/Arquitetura.txt") {
		t.Fatalf("add note failed: (%d, %q, %q)", code, out, errOut)
	}

	// Add standalone note in vault root.
	code, _, errOut = runCLI(t, cfgPath, "", "add", "Solta", "Texto rapido")
	if code != ExitOK {
		t.Fatalf("add root note failed (%d): %s", code, errOut)
	}

	// View raw content without .txt suffix.
	code, out, errOut = runCLI(t, cfgPath, "", "view", "WORK/Arquitetura", "--raw")
	if code != ExitOK || strings.TrimSpace(out) != "Linha 1\nLinha 2" {
		t.Fatalf("view raw = (%d, %q, %q)", code, out, errOut)
	}

	// List in table format.
	code, out, errOut = runCLI(t, cfgPath, "", "list", "-F", "table")
	if code != ExitOK || !strings.Contains(out, "WORK/Arquitetura.txt") || !strings.Contains(out, "Solta.txt") {
		t.Fatalf("list table = (%d, %q, %q)", code, out, errOut)
	}

	// Search by content.
	code, out, errOut = runCLI(t, cfgPath, "", "search", "Linha 2", "-n")
	if code != ExitOK || !strings.Contains(out, "WORK/Arquitetura.txt") || strings.Contains(out, "Solta.txt") {
		t.Fatalf("search = (%d, %q, %q)", code, out, errOut)
	}

	// Rename & move note to root.
	code, out, errOut = runCLI(t, cfgPath, "", "update", "WORK/Arquitetura", "Design", "-b", "none")
	if code != ExitOK || !strings.Contains(out, "Design.txt") {
		t.Fatalf("update failed: (%d, %q, %q)", code, out, errOut)
	}

	// Delete note -> moves to .trash/.
	code, out, errOut = runCLI(t, cfgPath, "", "rm", "Design")
	if code != ExitOK || !strings.Contains(out, ".trash") {
		t.Fatalf("rm failed: (%d, %q, %q)", code, out, errOut)
	}

	// Rename notebook and delete empty notebook.
	code, _, errOut = runCLI(t, cfgPath, "", "notebook", "rename", "WORK", "ARCHIVE")
	if code != ExitOK {
		t.Fatalf("notebook rename failed (%d): %s", code, errOut)
	}
	code, out, errOut = runCLI(t, cfgPath, "", "notebook", "delete", "ARCHIVE")
	if code != ExitOK || !strings.Contains(out, ".trash") {
		t.Fatalf("notebook delete failed: (%d, %q, %q)", code, out, errOut)
	}
}

func TestWorkspaceConfigAndVaultFlagPrecedence(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("VAULT_NOTE_CLI_VAULT", "")
	cfgPath := filepath.Join(tmp, "config.yaml")
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
	if code, _, errOut := runCLI(t, cfgPath, "", "nn", "WORK"); code != ExitOK {
		t.Fatalf("nn WORK: %s", errOut)
	}
	if code, out, errOut := runCLI(t, cfgPath, "", "add", "DefaultNB", "body"); code != ExitOK || !strings.Contains(out, "WORK/DefaultNB.txt") {
		t.Fatalf("expected default WORK from .vaultnotecli: (%d, %q, %q)", code, out, errOut)
	}

	// Escape hatch -b "" creates note in vault root.
	if code, out, errOut := runCLI(t, cfgPath, "", "add", "RootNote", "body", "-b", ""); code != ExitOK || !strings.Contains(out, "Created note RootNote.txt") {
		t.Fatalf("expected root note with -b '': (%d, %q, %q)", code, out, errOut)
	}

	// --vault flag overrides .vaultnotecli VAULT_PATH.
	code, out, errOut := runCLI(t, cfgPath, "", "--vault", vaultB, "config", "path")
	if code != ExitOK || strings.TrimSpace(out) != vaultB {
		t.Fatalf("--vault override = (%d, %q, %q)", code, out, errOut)
	}
}

func TestConfigShowNotebookPrefix(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("VAULT_NOTE_CLI_VAULT", "")
	cfgPath := filepath.Join(tmp, "config.yaml")
	vaultDir := filepath.Join(tmp, "vault")

	// Default is enabled even before anything is saved.
	code, out, errOut := runCLI(t, cfgPath, "", "config", "get", "show_notebook_prefix")
	if code != ExitOK || strings.TrimSpace(out) != "true" {
		t.Fatalf("default config get = (%d, %q, %q)", code, out, errOut)
	}

	if code, _, errOut := runCLI(t, cfgPath, "", "config", "set", "vault_path", vaultDir); code != ExitOK {
		t.Fatalf("config set vault_path = (%d, %q)", code, errOut)
	}

	code, out, errOut = runCLI(t, cfgPath, "", "config", "set", "show_notebook_prefix", "false")
	if code != ExitOK || !strings.Contains(out, "show_notebook_prefix = false") {
		t.Fatalf("config set false = (%d, %q, %q)", code, out, errOut)
	}
	code, out, _ = runCLI(t, cfgPath, "", "config", "get", "show_notebook_prefix")
	if code != ExitOK || strings.TrimSpace(out) != "false" {
		t.Fatalf("config get false = (%d, %q)", code, out)
	}

	code, out, errOut = runCLI(t, cfgPath, "", "config", "set", "show_notebook_prefix", "true")
	if code != ExitOK || !strings.Contains(out, "show_notebook_prefix = true") {
		t.Fatalf("config set true = (%d, %q, %q)", code, out, errOut)
	}
	code, out, _ = runCLI(t, cfgPath, "", "config", "get", "show_notebook_prefix")
	if code != ExitOK || strings.TrimSpace(out) != "true" {
		t.Fatalf("config get true = (%d, %q)", code, out)
	}

	code, _, errOut = runCLI(t, cfgPath, "", "config", "set", "show_notebook_prefix", "maybe")
	if code != ExitUsage || !strings.Contains(errOut, "expected true/false") {
		t.Fatalf("invalid bool = (%d, %q), want exit 2", code, errOut)
	}

	// List keeps ascending key order.
	code, out, _ = runCLI(t, cfgPath, "", "config", "get")
	if code != ExitOK || !strings.HasPrefix(out, "key\tvalue\n") {
		t.Fatalf("config list = (%d, %q)", code, out)
	}
	prefixIdx := strings.Index(out, "show_notebook_prefix")
	vaultIdx := strings.Index(out, "vault_path")
	if prefixIdx < 0 || vaultIdx < 0 || prefixIdx > vaultIdx {
		t.Fatalf("config list order = %q", out)
	}
}

func TestConfigShowLineNumbers(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("VAULT_NOTE_CLI_VAULT", "")
	cfgPath := filepath.Join(tmp, "config.yaml")

	code, out, errOut := runCLI(t, cfgPath, "", "config", "get", "show_line_numbers")
	if code != ExitOK || strings.TrimSpace(out) != "false" {
		t.Fatalf("default config get = (%d, %q, %q)", code, out, errOut)
	}

	code, out, errOut = runCLI(t, cfgPath, "", "config", "set", "show_line_numbers", "true")
	if code != ExitOK || !strings.Contains(out, "show_line_numbers = true") {
		t.Fatalf("config set true = (%d, %q, %q)", code, out, errOut)
	}
	code, out, _ = runCLI(t, cfgPath, "", "config", "get", "show_line_numbers")
	if code != ExitOK || strings.TrimSpace(out) != "true" {
		t.Fatalf("config get true = (%d, %q)", code, out)
	}

	code, _, errOut = runCLI(t, cfgPath, "", "config", "set", "show_line_numbers", "nope")
	if code != ExitUsage || !strings.Contains(errOut, "expected true/false") {
		t.Fatalf("invalid bool = (%d, %q), want exit 2", code, errOut)
	}
}

func TestNotebookCLICreateRenameUppercaseAndSingleWord(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("VAULT_NOTE_CLI_VAULT", "")
	cfgPath := filepath.Join(tmp, "config.yaml")
	vaultDir := filepath.Join(tmp, "vault")

	if code, _, errOut := runCLI(t, cfgPath, "", "config", "set", "vault_path", vaultDir); code != ExitOK {
		t.Fatalf("config set vault_path = (%d, %q)", code, errOut)
	}

	// Lowercase with leading/trailing spaces -> UPPERCASE.
	code, out, errOut := runCLI(t, cfgPath, "", "notebook", "add", "  estudos  ")
	if code != ExitOK || !strings.Contains(out, "Created notebook ESTUDOS") {
		t.Fatalf("notebook add lowercase = (%d, %q, %q)", code, out, errOut)
	}

	// Multi-word rejected on create.
	code, _, errOut = runCLI(t, cfgPath, "", "notebook", "add", "meu caderno")
	if code != ExitRuntime || !strings.Contains(errOut, "single word") {
		t.Fatalf("notebook add multi-word = (%d, %q), want exit 1", code, errOut)
	}

	// Rename trims and uppercases; multi-word rejected.
	code, _, errOut = runCLI(t, cfgPath, "", "notebook", "rename", "estudos", "duas palavras")
	if code != ExitRuntime || !strings.Contains(errOut, "single word") {
		t.Fatalf("notebook rename multi-word = (%d, %q), want exit 1", code, errOut)
	}
	code, out, errOut = runCLI(t, cfgPath, "", "notebook", "rename", "estudos", "  projetos  ")
	if code != ExitOK || !strings.Contains(out, "Renamed notebook to PROJETOS") {
		t.Fatalf("notebook rename lowercase = (%d, %q, %q)", code, out, errOut)
	}
}
