# Design: Default Starter Templates on First Vault Setup

**Repository:** `/home/henrique/Projetos/vault-note-cli`
**Stack:** Go 1.27.4, static binary (`CGO_ENABLED=0`), stdlib only (no new dependencies)
**Base state:** `master` @ `b75260e` with uncommitted note-templates (`docs/design-note-templates.md`) and template-insertion (`docs/design-template-insertion.md`) changes already in the working tree. Build directly on top of them; do **not** revert or commit unless asked.

---

## 1. Goal & Decisions (User-Confirmed)

Seed a small set of starter `.txt` templates into `<vault>/.templates/` when a vault is configured for the first time, plus a manual recovery command to restore missing defaults without overwriting user edits.

Settled choices:
1. **Language & starter set (4 English templates):** `checklist`, `daily`, `decision`, `meeting`.
2. **Automatic seeding rule:** Trigger whenever `<vault>/.templates/` does not exist at vault setup time (CLI `config set vault_path <dir>` and TUI `saveSettingsCmd`).
3. **Manual recovery command:** Add `vault-note-cli template init` (subcommand alias: `seed`) to create any missing default templates without overwriting existing ones.
4. **Never seed inside `vault.Open()`:** `Open()` runs on every read-only command and in `t.TempDir()` unit tests. Keeping seeding in explicit setup/init paths prevents read-only invocations (`note list`, `--vault ...`) from mutating disk and keeps unit tests isolated.

---

## 2. Storage & Packaging (`//go:embed`)

Store the 4 default templates as plain UTF-8 `.txt` files embedded into the binary via Go's stdlib `embed` package:

```text
internal/vault/defaults/
├── checklist.txt
├── daily.txt
├── decision.txt
└── meeting.txt
```

Each file ends with a single trailing `\n`, uses only the 5 supported placeholders (`{{title}}`, `{{notebook}}`, `{{date}}`, `{{datetime}}`, `{{template}}`), and starts with a clear non-empty first line (works cleanly with `SnippetPresent` duplicate guard):

### `internal/vault/defaults/daily.txt`
```text
# Daily — {{date}}

## Focus
- [ ] 

## Notes
- 

## Wrap-up
- 
```

### `internal/vault/defaults/meeting.txt`
```text
# {{title}}
Date: {{date}}
Attendees: 

## Agenda
- 

## Notes
- 

## Decisions
- 

## Action Items
- [ ] 
```

### `internal/vault/defaults/checklist.txt`
```text
## Checklist
- [ ] 
- [ ] 
- [ ] 
```

### `internal/vault/defaults/decision.txt`
```text
# Decision: {{title}}
Date: {{date}}
Status: Proposed

## Context


## Decision


## Consequences
- 
```

---

## 3. Domain Layer (`internal/vault/default_templates.go`)

Add `internal/vault/default_templates.go`:

```go
//go:embed defaults/*.txt
var defaultTemplatesFS embed.FS

// DefaultTemplates returns the built-in starter templates sorted by name
// (case-insensitive). Returned copies are read-only snapshots from embed.FS.
func DefaultTemplates() ([]Template, error)

// SeedDefaultTemplatesIfAbsent creates <vault>/.templates/ and writes all
// built-in starter templates ONLY when the .templates/ directory does not yet
// exist. If .templates/ already exists (even if empty after the user deleted
// templates via `template delete`), it returns (nil, nil) without writing.
func (v *Vault) SeedDefaultTemplatesIfAbsent() ([]string, error)

// SeedDefaultTemplates ensures <vault>/.templates/ exists and writes any
// built-in starter templates that are not already present (case-insensitive
// match against existing templates). Existing files are never overwritten.
// Returns the sorted names of templates created in this call.
func (v *Vault) SeedDefaultTemplates() ([]string, error)
```

### Key behaviors & invariants
- **No zombie templates:** `DeleteTemplate` moves `.templates/<name>.txt` into `.trash/` and leaves `.templates/` intact. Because `SeedDefaultTemplatesIfAbsent()` checks `os.Stat(v.templatesDir())` and returns `(nil, nil)` whenever `.templates/` exists, deleting all defaults will never cause them to reappear when `vault_path` is saved again.
- **No overwrites, ever:** `SeedDefaultTemplates()` checks existing templates case-insensitively (using `v.ListTemplates()` or case-fold lookup) so an existing `Daily.txt` on a case-sensitive filesystem is treated as already present and is neither overwritten nor duplicated. To reset a customized default, the user runs `template delete <name>` (preserving their version in `.trash/`) followed by `template init`.
- **Safe writes:** Validate every embedded name with `normalizeTemplateName`, verify `v.ensureInsideRoot(absPath)`, create `.templates/` with `0o755`, and write each file via `writeFileAtomic`.
- **Deterministic order:** Return created names sorted alphabetically (`checklist`, `daily`, `decision`, `meeting`).

---

## 4. Integration Points

### Step 1: Domain (`internal/vault/defaults/*.txt` + `internal/vault/default_templates.go` + `internal/vault/default_templates_test.go`)
Create the 4 `.txt` files, implement `DefaultTemplates`, `SeedDefaultTemplatesIfAbsent`, and `SeedDefaultTemplates`, and unit-test them.

### Step 2: CLI vault setup (`internal/cmd/config.go`)
In `configSetAction.Execute` for `settings.KeyVaultPath`:
1. Open the vault via `v, err := vault.Open(in.value)` (validates & creates the dir via `EnsureDir`).
2. Call `if _, err := v.SeedDefaultTemplatesIfAbsent(); err != nil { return configSetResult{}, err }`.
3. Persist `saved, err := ctx.Store.SetVaultPath(v.Root())`.
4. Keep `Format` stdout **unchanged**: `Saved vault_path = <path>\n` so scripts parsing it remain intact.

### Step 3: CLI `template init` (`internal/cmd/template.go`)
In `dispatchTemplate`:
- Add `case "init", "seed": return RunAction(ctx, templateInitAction{}, args[1:])`.
- `templateInitAction`:
  - `Parse`: reject unknown flags or positional args with `UsageError("usage: vault-note-cli template init")`.
  - `Execute`: `v, err := ctx.RequireVault()`, then `created, err := v.SeedDefaultTemplates()`.
  - `Format`:
    - If `len(created) == 0`: `Default templates already present (0 created)\n`
    - Else: `Initialized %d default template(s): %s\n` (comma-space joined, e.g. `Initialized 4 default template(s): checklist, daily, decision, meeting`).
- Exit code 0 in both cases (idempotent). Note `tpl init` works automatically via existing `"tpl": {"template"}` in `FlatAliases`.

### Step 4: TUI vault setup (`internal/tui/commands.go` + `internal/tui/update.go`)
- In `settingsSavedMsg`, add `seededTemplates []string`.
- In `saveSettingsCmd(store, rawVaultPath)`:
  1. `v, err := vault.Open(rawVaultPath)`
  2. `seeded, err := v.SeedDefaultTemplatesIfAbsent()` (on error, return `settingsSavedMsg{vaultPath: rawVaultPath, err: err}` before saving `vault_path`)
  3. Persist `store.SetVaultPath(absPath)` as today.
  4. Return `settingsSavedMsg{vaultPath: absPath, vault: v, seededTemplates: seeded}`.
- In `internal/tui/update.go` (`case settingsSavedMsg:`):
  - If `len(msg.seededTemplates) > 0`:
    `m.setStatus(fmt.Sprintf("Vault configurado: %s (%d templates padrão criados)", msg.vaultPath, len(msg.seededTemplates)), false)`
  - Else:
    `m.setStatus(fmt.Sprintf("Vault configurado: %s", msg.vaultPath), false)`

### Step 5: Tests
- **`internal/vault/default_templates_test.go`** (all `t.TempDir()`):
  - `DefaultTemplates()` returns 4 valid UTF-8 templates (`checklist`, `daily`, `decision`, `meeting`) passing `normalizeTemplateName` and rendering cleanly with `RenderTemplate`.
  - `vault.Open()` alone does **not** create `.templates/`.
  - `SeedDefaultTemplatesIfAbsent()` seeds all 4 on a fresh vault, returns all 4 names.
  - Calling `SeedDefaultTemplatesIfAbsent()` a second time returns empty slice and leaves custom edits untouched.
  - Deleting all 4 via `DeleteTemplate` leaves `.templates/` empty; calling `SeedDefaultTemplatesIfAbsent()` again does **not** resurrect them (returns empty slice, `ListTemplates()` still empty).
  - `SeedDefaultTemplates()` after deleting 2 defaults (and editing 1) recreates only the 2 missing ones and preserves the edited one verbatim.
  - Case-insensitive collision guard: if `Daily.txt` exists, `SeedDefaultTemplates()` skips `daily` without creating a second file or overwriting `Daily.txt`.
- **`internal/cmd/template_test.go`**:
  - Update `TestTemplateCLIListAddViewDelete` (whose `newTemplateCLIEnv` runs `config set vault_path`): assert the 4 defaults are seeded on setup, and keep an explicit invariant-3 empty-list test (using a fresh `--vault <dir>` without `config set`, or after deleting all templates) verifying `template list` outputs `""` with no header.
  - Add `TestTemplateCLIInitAndSetupSeeding`:
    - `config set vault_path <newdir>` creates the 4 defaults while keeping stdout `Saved vault_path = ...`.
    - Deleting `daily` and re-running `config set vault_path <newdir>` does NOT resurrect `daily`.
    - Running `template init` recreates only `daily` (`Initialized 1 default template(s): daily`).
    - Running `template init` again prints `Default templates already present (0 created)` with exit 0.
    - Running `template init extra-arg` returns exit 2 (`ExitUsage`).
- **`internal/tui/tui_test.go` / `internal/tui/modal_test.go`**:
  - Verify onboarding (`modalSettings`) seeds the 4 defaults into `<vault>/.templates/` and reflects them in `m.loadTemplateNames()` and status message, while re-saving settings after deleting templates does not resurrect them.

### Step 6: Documentation Sync (Invariant 6)
Update in the same change:
- `internal/cmd/help.go`: add `init` to `rootHelpText` (`template <ação>`) and `templateHelpText` (`init   Cria templates padrão ausentes sem sobrescrever existentes`), and note in `configHelpText` that `set vault_path` creates starter templates on first setup.
- `README.md`: document starter defaults (`checklist`, `daily`, `decision`, `meeting`) and `template init`.
- `skill/vault-note-cli.md`: add `template init` and mention default template seeding.
- `AGENTS.md`: note embedded default templates (`internal/vault/defaults/*.txt`) and `SeedDefaultTemplates*` under `internal/vault`.

---

## 5. Verification ("Done" Criteria)

1. `make check` (`gofmt` + `go vet` + `go test -race -count=1 ./...`) passes with zero failures.
2. `make build` (`CGO_ENABLED=0`) succeeds; `go.mod` and `go.sum` are untouched.
3. Smoke test against `./bin/vault-note-cli` using a throwaway `/tmp` dir:

```bash
TMP=$(mktemp -d)
CFG="$TMP/config.yaml"
VLT="$TMP/vault"
BIN=./bin/vault-note-cli

# 1. First-time setup seeds 4 defaults, stdout unchanged
$BIN --config "$CFG" config set vault_path "$VLT"
$BIN --config "$CFG" template list

# 2. Delete one, re-run config set -> must NOT resurrect
$BIN --config "$CFG" template delete daily
$BIN --config "$CFG" config set vault_path "$VLT"
$BIN --config "$CFG" template list

# 3. Edit one, run template init -> restores only daily, preserves edit
printf "custom\n" | $BIN --config "$CFG" template update meeting --stdin
$BIN --config "$CFG" template init
$BIN --config "$CFG" template view meeting --raw
$BIN --config "$CFG" template init

# 4. --vault on an unconfigured dir must NOT seed on read
VLT2="$TMP/vault-ro"
mkdir -p "$VLT2"
$BIN --config "$CFG" --vault "$VLT2" template list
test ! -d "$VLT2/.templates"

rm -rf "$TMP"
```

Expected:
- Step 1 lists `checklist`, `daily`, `decision`, `meeting`.
- Step 2 lists 3 templates (`daily` absent).
- Step 3 restores `daily` (`Initialized 1 default template(s): daily`), `meeting --raw` still prints `custom`, second `template init` prints `Default templates already present (0 created)`.
- Step 4 outputs empty string (no header) and `$VLT2/.templates` does not exist.
