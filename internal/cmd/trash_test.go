package cmd

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestTrashListAndEmptyCLI(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("VAULT_NOTE_CLI_VAULT", "")
	cfgPath := filepath.Join(tmp, "config.yaml")
	vaultDir := filepath.Join(tmp, "vault")

	code, _, errOut := runCLI(t, cfgPath, "", "config", "set", "vault_path", vaultDir)
	if code != ExitOK {
		t.Fatalf("config set vault_path failed (%d): %s", code, errOut)
	}

	// Help short-circuit for trash.
	code, out, errOut := runCLI(t, cfgPath, "", "help", "trash")
	if code != ExitOK || !strings.Contains(out, "vault-note-cli trash") {
		t.Fatalf("help trash = (%d, %q, %q)", code, out, errOut)
	}

	// Empty trash list produces zero output (no TSV header).
	code, out, errOut = runCLI(t, cfgPath, "", "trash", "list")
	if code != ExitOK || out != "" {
		t.Fatalf("empty trash list = (%d, %q, %q)", code, out, errOut)
	}

	// Empty trash when already empty succeeds without --force.
	code, out, errOut = runCLI(t, cfgPath, "", "trash", "empty")
	if code != ExitOK || !strings.Contains(out, "Trash is already empty") {
		t.Fatalf("empty trash (already empty) = (%d, %q, %q)", code, out, errOut)
	}

	// Create notebook and delete two notes.
	code, _, errOut = runCLI(t, cfgPath, "", "notebook", "add", "WORK")
	if code != ExitOK {
		t.Fatalf("notebook add WORK failed (%d): %s", code, errOut)
	}
	code, _, errOut = runCLI(t, cfgPath, "", "add", "Alpha", "content-a", "-b", "WORK")
	if code != ExitOK {
		t.Fatalf("add Alpha failed (%d): %s", code, errOut)
	}
	code, _, errOut = runCLI(t, cfgPath, "", "add", "Beta", "content-bb")
	if code != ExitOK {
		t.Fatalf("add Beta failed (%d): %s", code, errOut)
	}
	code, _, errOut = runCLI(t, cfgPath, "", "delete", "WORK/Alpha")
	if code != ExitOK {
		t.Fatalf("delete Alpha failed (%d): %s", code, errOut)
	}
	code, _, errOut = runCLI(t, cfgPath, "", "delete", "Beta")
	if code != ExitOK {
		t.Fatalf("delete Beta failed (%d): %s", code, errOut)
	}

	// trash list (alias tr ls) shows both items with header and timestamps.
	code, out, errOut = runCLI(t, cfgPath, "", "tr", "ls", "-t")
	if code != ExitOK {
		t.Fatalf("tr ls -t failed (%d): %s", code, errOut)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines (header + 2 rows), got %d:\n%s", len(lines), out)
	}
	if lines[0] != "id\tpath\tsize\ttrashed_at" {
		t.Fatalf("unexpected header: %q", lines[0])
	}
	if !strings.Contains(out, "WORK/Alpha.txt") || !strings.Contains(out, "Beta.txt") {
		t.Fatalf("expected deleted notes in trash list:\n%s", out)
	}

	// trash empty without --force refuses and keeps items intact.
	code, _, errOut = runCLI(t, cfgPath, "", "trash", "empty")
	if code != ExitRuntime || !strings.Contains(errOut, "--force") {
		t.Fatalf("trash empty without --force = (%d, %q), want ExitRuntime mentioning --force", code, errOut)
	}

	// Unexpected positional arg -> ExitUsage.
	code, _, _ = runCLI(t, cfgPath, "", "trash", "empty", "extra")
	if code != ExitUsage {
		t.Fatalf("expected ExitUsage for unexpected arg, got %d", code)
	}

	// Flat alias empty-trash -f permanently deletes all items.
	code, out, errOut = runCLI(t, cfgPath, "", "empty-trash", "-f")
	if code != ExitOK || !strings.Contains(out, "removed 2 item(s)") {
		t.Fatalf("empty-trash -f = (%d, %q, %q)", code, out, errOut)
	}

	// Trash is now empty.
	code, out, errOut = runCLI(t, cfgPath, "", "trash", "list")
	if code != ExitOK || out != "" {
		t.Fatalf("trash list after empty = (%d, %q, %q)", code, out, errOut)
	}
}
