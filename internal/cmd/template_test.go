package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newTemplateCLIEnv configures a throwaway config + vault and returns both paths.
func newTemplateCLIEnv(t *testing.T) (cfgPath, vaultDir string) {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("VAULT_NOTE_CLI_VAULT", "")
	cfgPath = filepath.Join(tmp, "config.yaml")
	vaultDir = filepath.Join(tmp, "vault")

	code, _, errOut := runCLI(t, cfgPath, "", "config", "set", "vault_path", vaultDir)
	if code != ExitOK {
		t.Fatalf("config set vault_path failed (%d): %s", code, errOut)
	}
	return cfgPath, vaultDir
}

func TestTemplateCLIListAddViewDelete(t *testing.T) {
	cfgPath, vaultDir := newTemplateCLIEnv(t)

	// First vault setup seeds the 4 starter templates.
	code, out, errOut := runCLI(t, cfgPath, "", "template", "list")
	if code != ExitOK {
		t.Fatalf("template list failed (%d): %s", code, errOut)
	}
	for _, name := range []string{"checklist", "daily", "decision", "meeting"} {
		if !strings.Contains(out, name+"\t") {
			t.Fatalf("seeded default %q missing from list output: %q", name, out)
		}
	}

	// Invariant 3: an unconfigured --vault is read-only and lists no templates.
	emptyVault := filepath.Join(t.TempDir(), "vault-ro")
	if err := os.MkdirAll(emptyVault, 0o755); err != nil {
		t.Fatalf("mkdir vault-ro: %v", err)
	}
	code, out, errOut = runCLI(t, cfgPath, "", "template", "list", "--vault", emptyVault)
	if code != ExitOK || out != "" {
		t.Fatalf("empty template list = (%d, %q, %q)", code, out, errOut)
	}
	if _, err := os.Stat(filepath.Join(emptyVault, ".templates")); !os.IsNotExist(err) {
		t.Fatalf("read-only template list seeded .templates/ (stat err = %v)", err)
	}

	// Add via stdin.
	code, out, errOut = runCLI(t, cfgPath, "Nota: {{title}}\nData: {{date}}\n", "template", "add", "reuniao", "--stdin")
	if code != ExitOK || !strings.Contains(out, "Created template reuniao") {
		t.Fatalf("template add = (%d, %q, %q)", code, out, errOut)
	}
	if _, err := os.Stat(filepath.Join(vaultDir, ".templates", "reuniao.txt")); err != nil {
		t.Fatalf("expected template file on disk: %v", err)
	}

	// List shows name + path with header.
	code, out, errOut = runCLI(t, cfgPath, "", "template", "list")
	if code != ExitOK {
		t.Fatalf("template list failed (%d): %s", code, errOut)
	}
	if !strings.HasPrefix(out, "name\tpath\n") || !strings.Contains(out, "reuniao") {
		t.Fatalf("unexpected template list output: %q", out)
	}

	// Table format.
	code, out, _ = runCLI(t, cfgPath, "", "template", "list", "-F", "table")
	if code != ExitOK || !strings.Contains(out, "reuniao") {
		t.Fatalf("template list -F table = (%d, %q)", code, out)
	}

	// View raw.
	code, out, _ = runCLI(t, cfgPath, "", "template", "view", "reuniao", "--raw")
	if code != ExitOK || !strings.HasPrefix(out, "Nota: {{title}}") {
		t.Fatalf("template view --raw = (%d, %q)", code, out)
	}

	// Update via positional body.
	code, out, _ = runCLI(t, cfgPath, "", "template", "update", "reuniao", "novo corpo")
	if code != ExitOK || !strings.Contains(out, "Updated template reuniao") {
		t.Fatalf("template update = (%d, %q)", code, out)
	}

	// Delete moves to .trash/.
	code, out, _ = runCLI(t, cfgPath, "", "template", "delete", "reuniao")
	if code != ExitOK || !strings.Contains(out, "Moved template reuniao to") {
		t.Fatalf("template delete = (%d, %q)", code, out)
	}
	if _, err := os.Stat(filepath.Join(vaultDir, ".templates", "reuniao.txt")); !os.IsNotExist(err) {
		t.Fatalf("expected template removed from .templates/")
	}
}

func TestTemplateCLIRequiresBodyAndValidName(t *testing.T) {
	cfgPath, _ := newTemplateCLIEnv(t)

	code, _, errOut := runCLI(t, cfgPath, "", "template", "add", "vazio")
	if code != ExitUsage || !strings.Contains(errOut, "template body is required") {
		t.Fatalf("expected usage error for missing body, got (%d, %q)", code, errOut)
	}

	code, _, errOut = runCLI(t, cfgPath, "", "template", "add", "../escape", "x")
	if code != ExitRuntime || !strings.Contains(errOut, "invalid input") {
		t.Fatalf("expected invalid input for traversal name, got (%d, %q)", code, errOut)
	}

	code, _, errOut = runCLI(t, cfgPath, "", "template", "update", "inexistente", "x")
	if code != ExitRuntime || !strings.Contains(errOut, "record not found") {
		t.Fatalf("expected not found on update, got (%d, %q)", code, errOut)
	}
}

func TestNoteAddFromTemplate(t *testing.T) {
	cfgPath, vaultDir := newTemplateCLIEnv(t)

	if code, _, errOut := runCLI(t, cfgPath, "", "nn", "WORK"); code != ExitOK {
		t.Fatalf("nn WORK failed: %s", errOut)
	}
	if code, _, errOut := runCLI(t, cfgPath, "Nota: {{title}} ({{template}})\nCaderno: {{notebook}}\n", "template", "add", "reuniao", "--stdin"); code != ExitOK {
		t.Fatalf("template add failed: %s", errOut)
	}

	code, out, errOut := runCLI(t, cfgPath, "", "add", "Reuniao 12-05", "-b", "WORK", "-t", "reuniao")
	if code != ExitOK || !strings.Contains(out, "Created note WORK/Reuniao 12-05.txt") {
		t.Fatalf("add -t = (%d, %q, %q)", code, out, errOut)
	}

	raw, err := os.ReadFile(filepath.Join(vaultDir, "WORK", "Reuniao 12-05.txt"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	want := "Nota: Reuniao 12-05 (reuniao)\nCaderno: WORK\n"
	if string(raw) != want {
		t.Fatalf("rendered note = %q, want %q", string(raw), want)
	}

	// Template + inline content is a usage error.
	code, _, errOut = runCLI(t, cfgPath, "", "add", "Dup", "-t", "reuniao", "-c", "y")
	if code != ExitUsage || !strings.Contains(errOut, "cannot be combined") {
		t.Fatalf("expected usage error, got (%d, %q)", code, errOut)
	}

	// Missing template: runtime error, no note created.
	code, _, errOut = runCLI(t, cfgPath, "", "add", "Fantasma", "-t", "inexistente")
	if code != ExitRuntime || !strings.Contains(errOut, "record not found") {
		t.Fatalf("expected not found, got (%d, %q)", code, errOut)
	}
	if _, err := os.Stat(filepath.Join(vaultDir, "Fantasma.txt")); !os.IsNotExist(err) {
		t.Fatalf("expected no note for missing template")
	}

	// Regression: plain add still works and keeps its output.
	code, out, _ = runCLI(t, cfgPath, "", "add", "Simples", "corpo")
	if code != ExitOK || !strings.Contains(out, "Created note Simples.txt") {
		t.Fatalf("plain add = (%d, %q)", code, out)
	}
}

func TestTemplateAliasesAndHelp(t *testing.T) {
	cfgPath, _ := newTemplateCLIEnv(t)

	if code, _, errOut := runCLI(t, cfgPath, "corpo", "tpl-add", "ata", "--stdin"); code != ExitOK {
		t.Fatalf("tpl-add alias failed: %s", errOut)
	}
	code, out, _ := runCLI(t, cfgPath, "", "tpls")
	if code != ExitOK || !strings.Contains(out, "ata") {
		t.Fatalf("tpls alias = (%d, %q)", code, out)
	}

	code, out, _ = runCLI(t, cfgPath, "", "help", "template")
	if code != ExitOK || !strings.Contains(out, "{{title}}") {
		t.Fatalf("help template = (%d, %q)", code, out)
	}
	code, out, _ = runCLI(t, cfgPath, "", "help")
	if code != ExitOK || !strings.Contains(out, "template <ação>") {
		t.Fatalf("root help missing template line: %q", out)
	}
}

func TestTemplateCLIInitAndSetupSeeding(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("VAULT_NOTE_CLI_VAULT", "")
	cfgPath := filepath.Join(tmp, "config.yaml")
	vaultDir := filepath.Join(tmp, "vault")

	code, out, errOut := runCLI(t, cfgPath, "", "config", "set", "vault_path", vaultDir)
	if code != ExitOK {
		t.Fatalf("config set vault_path = (%d, %q)", code, errOut)
	}
	if !strings.Contains(out, "Saved vault_path = "+vaultDir) {
		t.Fatalf("config set stdout changed: %q", out)
	}
	for _, name := range []string{"checklist", "daily", "decision", "meeting"} {
		if _, err := os.Stat(filepath.Join(vaultDir, ".templates", name+".txt")); err != nil {
			t.Fatalf("default %q not seeded: %v", name, err)
		}
	}

	// Deleting a default and re-saving vault_path must not resurrect it.
	if code, _, errOut := runCLI(t, cfgPath, "", "template", "delete", "daily"); code != ExitOK {
		t.Fatalf("template delete daily failed: %s", errOut)
	}
	if code, _, errOut := runCLI(t, cfgPath, "", "config", "set", "vault_path", vaultDir); code != ExitOK {
		t.Fatalf("second config set failed: %s", errOut)
	}
	if _, err := os.Stat(filepath.Join(vaultDir, ".templates", "daily.txt")); !os.IsNotExist(err) {
		t.Fatalf("config set resurrected daily.txt (stat err = %v)", err)
	}

	// template init recreates only the missing default.
	code, out, errOut = runCLI(t, cfgPath, "", "template", "init")
	if code != ExitOK || !strings.Contains(out, "Initialized 1 default template(s): daily") {
		t.Fatalf("template init = (%d, %q, %q)", code, out, errOut)
	}
	if _, err := os.Stat(filepath.Join(vaultDir, ".templates", "daily.txt")); err != nil {
		t.Fatalf("daily.txt not restored: %v", err)
	}

	// Idempotent second run.
	code, out, errOut = runCLI(t, cfgPath, "", "template", "init")
	if code != ExitOK || !strings.Contains(out, "Default templates already present (0 created)") {
		t.Fatalf("second template init = (%d, %q, %q)", code, out, errOut)
	}

	// Alias seed works.
	code, out, errOut = runCLI(t, cfgPath, "", "template", "seed")
	if code != ExitOK || !strings.Contains(out, "Default templates already present (0 created)") {
		t.Fatalf("template seed alias = (%d, %q, %q)", code, out, errOut)
	}

	// Positional argument is a usage error.
	code, _, errOut = runCLI(t, cfgPath, "", "template", "init", "extra-arg")
	if code != ExitUsage {
		t.Fatalf("template init extra-arg = (%d, %q), want exit 2", code, errOut)
	}
}

// newInsertCLIEnv sets up a vault with a "checklist" template and a note "Projeto X".
func newInsertCLIEnv(t *testing.T) (cfgPath, notePath string) {
	t.Helper()
	cfgPath, vaultDir := newTemplateCLIEnv(t)
	if code, _, errOut := runCLI(t, cfgPath, "", "nn", "WORK"); code != ExitOK {
		t.Fatalf("nn WORK failed: %s", errOut)
	}
	if code, _, errOut := runCLI(t, cfgPath, "== {{title}} — checklist ==\n- [ ] item 1\n", "template", "add", "checklist", "--stdin"); code != ExitOK {
		t.Fatalf("template add failed: %s", errOut)
	}
	if code, _, errOut := runCLI(t, cfgPath, "", "add", "Projeto X", "Corpo existente", "-b", "WORK"); code != ExitOK {
		t.Fatalf("add note failed: %s", errOut)
	}
	return cfgPath, filepath.Join(vaultDir, "WORK", "Projeto X.txt")
}

func readFileString(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", path, err)
	}
	return string(raw)
}

func TestNoteInsertAppendPrependAndAt(t *testing.T) {
	cfgPath, notePath := newInsertCLIEnv(t)

	code, out, errOut := runCLI(t, cfgPath, "", "note", "insert", "WORK/Projeto X", "-t", "checklist")
	if code != ExitOK || !strings.Contains(out, "Inserted template checklist into WORK/Projeto X.txt") {
		t.Fatalf("insert append = (%d, %q, %q)", code, out, errOut)
	}
	want := "Corpo existente\n\n== Projeto X — checklist ==\n- [ ] item 1\n"
	if got := readFileString(t, notePath); got != want {
		t.Fatalf("append content = %q, want %q", got, want)
	}

	// Prepend (force: the first line is already present).
	code, _, errOut = runCLI(t, cfgPath, "", "note", "insert", "WORK/Projeto X", "-t", "checklist", "--prepend")
	if code != ExitOK {
		t.Fatalf("insert prepend = (%d, %q)", code, errOut)
	}
	got := readFileString(t, notePath)
	if !strings.HasPrefix(got, "== Projeto X — checklist ==\n- [ ] item 1\n\nCorpo existente") {
		t.Fatalf("prepend content = %q", got)
	}

	// --at 0 with force.
	code, _, errOut = runCLI(t, cfgPath, "", "note", "insert", "WORK/Projeto X", "-t", "checklist", "--at", "0")
	if code != ExitOK {
		t.Fatalf("insert --at 0 = (%d, %q)", code, errOut)
	}
	if !strings.HasPrefix(readFileString(t, notePath), "== Projeto X — checklist ==") {
		t.Fatalf("--at 0 did not insert at the start")
	}

	// --at beyond the content is clamped, not an error.
	code, _, errOut = runCLI(t, cfgPath, "", "note", "insert", "WORK/Projeto X", "-t", "checklist", "--at", "9999")
	if code != ExitOK {
		t.Fatalf("insert --at clamp = (%d, %q)", code, errOut)
	}
	if !strings.HasSuffix(readFileString(t, notePath), "- [ ] item 1\n") {
		t.Fatalf("--at clamp did not append at the end")
	}
}

func TestNoteInsertUsageErrors(t *testing.T) {
	cfgPath, _ := newInsertCLIEnv(t)

	cases := []struct {
		name string
		args []string
		want string
	}{
		{"missing template", []string{"note", "insert", "WORK/Projeto X"}, "--template is required"},
		{"negative at", []string{"note", "insert", "WORK/Projeto X", "-t", "checklist", "--at", "-1"}, "non-negative"},
		{"prepend with at", []string{"note", "insert", "WORK/Projeto X", "-t", "checklist", "--prepend", "--at", "1"}, "cannot be combined"},
		{"print with open", []string{"note", "insert", "WORK/Projeto X", "-t", "checklist", "--print", "--open"}, "cannot be combined"},
		{"missing ref", []string{"note", "insert", "-t", "checklist"}, "usage:"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, _, errOut := runCLI(t, cfgPath, "", tc.args...)
			if code != ExitUsage {
				t.Fatalf("expected exit 2, got %d (%q)", code, errOut)
			}
			if !strings.Contains(errOut, tc.want) {
				t.Fatalf("stderr = %q, want substring %q", errOut, tc.want)
			}
		})
	}
}

func TestNoteInsertPrintDoesNotWrite(t *testing.T) {
	cfgPath, notePath := newInsertCLIEnv(t)
	before := readFileString(t, notePath)

	code, out, errOut := runCLI(t, cfgPath, "", "note", "insert", "WORK/Projeto X", "-t", "checklist", "--print")
	if code != ExitOK {
		t.Fatalf("--print = (%d, %q)", code, errOut)
	}
	want := "Corpo existente\n\n== Projeto X — checklist ==\n- [ ] item 1\n"
	if out != want {
		t.Fatalf("--print output = %q, want %q", out, want)
	}
	if !strings.Contains(errOut, "dry-run") {
		t.Fatalf("expected dry-run notice on stderr, got %q", errOut)
	}
	if got := readFileString(t, notePath); got != before {
		t.Fatalf("--print modified the file: %q", got)
	}

	// Re-inserting the same template via --print is allowed and file untouched.
	if code, _, errOut := runCLI(t, cfgPath, "", "note", "insert", "WORK/Projeto X", "-t", "checklist"); code != ExitOK {
		t.Fatalf("first insert failed: %s", errOut)
	}
	afterInsert := readFileString(t, notePath)
	code, out, errOut = runCLI(t, cfgPath, "", "note", "insert", "WORK/Projeto X", "-t", "checklist", "--print")
	if code != ExitOK {
		t.Fatalf("re-insert --print should exit 0, got %d (%q)", code, errOut)
	}
	if !strings.Contains(out, "== Projeto X — checklist ==") {
		t.Fatalf("expected composed preview, got %q", out)
	}
	if got := readFileString(t, notePath); got != afterInsert {
		t.Fatalf("re-insert --print modified the file")
	}
}

func TestNoteInsertReinsertSucceeds(t *testing.T) {
	cfgPath, notePath := newInsertCLIEnv(t)

	if code, _, errOut := runCLI(t, cfgPath, "", "note", "insert", "WORK/Projeto X", "-t", "checklist"); code != ExitOK {
		t.Fatalf("first insert failed: %s", errOut)
	}
	code, _, errOut := runCLI(t, cfgPath, "", "note", "insert", "WORK/Projeto X", "-t", "checklist")
	if code != ExitOK {
		t.Fatalf("re-insert should exit 0, got %d (%q)", code, errOut)
	}
	if strings.Contains(errOut, "aviso") {
		t.Fatalf("re-insert should not warn: %q", errOut)
	}
	if strings.Count(readFileString(t, notePath), "== Projeto X — checklist ==") != 2 {
		t.Fatalf("expected two copies after re-insert")
	}

	// Missing template: exit 1, file untouched.
	before := readFileString(t, notePath)
	code, _, errOut = runCLI(t, cfgPath, "", "note", "insert", "WORK/Projeto X", "-t", "inexistente")
	if code != ExitRuntime || !strings.Contains(errOut, "record not found") {
		t.Fatalf("missing template = (%d, %q)", code, errOut)
	}
	if got := readFileString(t, notePath); got != before {
		t.Fatalf("missing template modified the file")
	}
}

func TestNoteInsertAliasesAndHelp(t *testing.T) {
	cfgPath, notePath := newInsertCLIEnv(t)

	if code, _, errOut := runCLI(t, cfgPath, "", "apply", "WORK/Projeto X", "-t", "checklist"); code != ExitOK {
		t.Fatalf("apply alias failed: %s", errOut)
	}
	if !strings.Contains(readFileString(t, notePath), "== Projeto X — checklist ==") {
		t.Fatalf("apply alias did not insert")
	}
	if code, _, errOut := runCLI(t, cfgPath, "", "ins", "WORK/Projeto X", "-t", "checklist"); code != ExitOK {
		t.Fatalf("ins alias failed: %s", errOut)
	}

	code, out, _ := runCLI(t, cfgPath, "", "help", "note")
	if code != ExitOK || !strings.Contains(out, "insert <ref> -t <template>") {
		t.Fatalf("note help missing insert: %q", out)
	}
	code, out, _ = runCLI(t, cfgPath, "", "help")
	if code != ExitOK || !strings.Contains(out, "insert (ins, apply)") {
		t.Fatalf("root help missing insert aliases: %q", out)
	}
}

func TestTemplateAddFromNote(t *testing.T) {
	cfgPath, vaultDir := newTemplateCLIEnv(t)
	if code, _, errOut := runCLI(t, cfgPath, "", "nn", "WORK"); code != ExitOK {
		t.Fatalf("nn WORK failed: %s", errOut)
	}
	body := "Titulo: {{title}}\nData: {{date}}\n"
	if code, _, errOut := runCLI(t, cfgPath, "", "add", "Modelo", body, "-b", "WORK"); code != ExitOK {
		t.Fatalf("add Modelo failed: %s", errOut)
	}
	if code, _, errOut := runCLI(t, cfgPath, "", "add", "Solta", "corpo raiz"); code != ExitOK {
		t.Fatalf("add Solta failed: %s", errOut)
	}

	// From a notebook note with -b.
	code, out, errOut := runCLI(t, cfgPath, "", "template", "add", "modelo", "--from", "Modelo", "-b", "WORK")
	if code != ExitOK || !strings.Contains(out, "Created template modelo") {
		t.Fatalf("template add --from = (%d, %q, %q)", code, out, errOut)
	}
	code, out, _ = runCLI(t, cfgPath, "", "template", "view", "modelo", "--raw")
	if code != ExitOK || out != body {
		t.Fatalf("template view --raw = (%d, %q), want %q", code, out, body)
	}

	// Root note without -b.
	code, out, errOut = runCLI(t, cfgPath, "", "template", "add", "solta", "--from", "Solta")
	if code != ExitOK || !strings.Contains(out, "Created template solta") {
		t.Fatalf("root --from = (%d, %q, %q)", code, out, errOut)
	}

	// Duplicate without force: exit 1, hint, file untouched.
	before, _ := os.ReadFile(filepath.Join(vaultDir, ".templates", "modelo.txt"))
	code, _, errOut = runCLI(t, cfgPath, "", "template", "add", "modelo", "--from", "Modelo", "-b", "WORK")
	if code != ExitRuntime || !strings.Contains(errOut, "already exists") || !strings.Contains(errOut, "--force") {
		t.Fatalf("duplicate --from = (%d, %q)", code, errOut)
	}
	after, _ := os.ReadFile(filepath.Join(vaultDir, ".templates", "modelo.txt"))
	if string(after) != string(before) {
		t.Fatalf("duplicate --from modified template")
	}

	// --force overwrites.
	code, _, errOut = runCLI(t, cfgPath, "", "template", "add", "modelo", "--from", "Solta", "-f")
	if code != ExitOK {
		t.Fatalf("force --from = (%d, %q)", code, errOut)
	}
	code, out, _ = runCLI(t, cfgPath, "", "template", "view", "modelo", "--raw")
	if code != ExitOK || out != "corpo raiz\n" {
		t.Fatalf("force did not overwrite: (%d, %q)", code, out)
	}

	// Missing note: exit 1 and no template created.
	code, _, errOut = runCLI(t, cfgPath, "", "template", "add", "fantasma", "--from", "NaoExiste")
	if code != ExitRuntime || !strings.Contains(errOut, "record not found") {
		t.Fatalf("missing note = (%d, %q)", code, errOut)
	}
	if _, err := os.Stat(filepath.Join(vaultDir, ".templates", "fantasma.txt")); !os.IsNotExist(err) {
		t.Fatalf("missing note created a template")
	}
}

func TestTemplateAddFromNoteUsageErrors(t *testing.T) {
	cfgPath, _ := newTemplateCLIEnv(t)
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"body with from", []string{"template", "add", "x", "corpo", "--from", "n"}, "cannot be combined"},
		{"stdin with from", []string{"template", "add", "x", "--from", "n", "--stdin"}, "cannot be combined"},
		{"edit with from", []string{"template", "add", "x", "--from", "n", "-e"}, "cannot be combined"},
		{"notebook without from", []string{"template", "add", "x", "corpo", "-b", "WORK"}, "requires --from"},
		{"open with edit", []string{"template", "add", "x", "corpo", "-e", "--open"}, "cannot be combined"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, _, errOut := runCLI(t, cfgPath, "", tc.args...)
			if code != ExitUsage {
				t.Fatalf("expected exit 2, got %d (%q)", code, errOut)
			}
			if !strings.Contains(errOut, tc.want) {
				t.Fatalf("stderr = %q, want %q", errOut, tc.want)
			}
		})
	}
}

func TestTemplateAddFromNoteOpen(t *testing.T) {
	cfgPath, _ := newTemplateCLIEnv(t)
	tmp := t.TempDir()
	marker := filepath.Join(tmp, "opened.txt")
	editor := filepath.Join(tmp, "editor.sh")
	script := "#!/bin/sh\nprintf '%s\\n' \"$1\" > " + marker + "\n"
	if err := os.WriteFile(editor, []byte(script), 0o755); err != nil {
		t.Fatalf("write editor script: %v", err)
	}
	t.Setenv("EDITOR", editor)

	if code, _, errOut := runCLI(t, cfgPath, "", "add", "Modelo", "corpo"); code != ExitOK {
		t.Fatalf("add Modelo failed: %s", errOut)
	}
	code, out, errOut := runCLI(t, cfgPath, "", "template", "add", "aberto", "--from", "Modelo", "--open")
	if code != ExitOK || !strings.Contains(out, "Created template aberto") {
		t.Fatalf("--open = (%d, %q, %q)", code, out, errOut)
	}
	opened, err := os.ReadFile(marker)
	if err != nil {
		t.Fatalf("editor was not invoked: %v", err)
	}
	if !strings.HasSuffix(strings.TrimSpace(string(opened)), filepath.Join(".templates", "aberto.txt")) {
		t.Fatalf("editor opened %q", string(opened))
	}
}
