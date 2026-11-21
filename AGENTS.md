# AGENTS.md

Operational contract for AI agents and developers working in the `vault-note-cli` repository.

## Memory

- MCP openviking

## Main Commands

```bash
make check          # Mandatory gate: fmt + vet + go test -race -count=1 ./...
make build          # Static CGO_ENABLED=0 binary in bin/vault-note-cli
make test           # go test -race -count=1 ./...
make build-all      # Cross-compilation in dist/
make checksums      # Generates dist/checksums.txt
make run            # Runs with a disposable YAML config (/tmp/vault-note-cli-dev.yaml)
```

## Architecture Map

- `cmd/vault-note-cli/main.go`: minimal entrypoint (`signal.NotifyContext`, `cmd.Run`, `os.Exit`). No domain rules here.
- `internal/buildinfo`: `Version`, `Commit`, `Date` metadata injected via `-ldflags`.
- `internal/filestore`: standard XDG/macOS/Windows directories (`config.yaml`) and `OpenFolder`/`OpenFile` with headless fallback.
- `internal/config`: upward discovery (up to `$HOME`) and strict parsing of `.vaultnotecli` (`NOTEBOOK`, `VAULT_PATH`).
- `internal/settings`: YAML file persistence (`gopkg.in/yaml.v3`) with atomic writes dedicated to application settings (`vault_path`, `show_notebook_prefix`, `show_line_numbers`).
- `internal/vault`: filesystem-based persistence where top-level folders = `notebooks`, plain `.txt` files = notes (including vault root, always sorted alphabetically), atomic writes, optimistic external conflict checks (`ContentHash`, `UpdateNoteContentChecked`, `CreateConflictCopy`, `ErrConflict`), internal trash `<vault>/.trash/`, note templates in `<vault>/.templates/` (`{{title}}`, `{{notebook}}`, `{{date}}`, `{{datetime}}`, `{{template}}`) with injectable clock for `{{date}}`/`{{datetime}}`, template insertion into existing notes (pure rune-safe `InsertSnippet`, `RenderTemplateFor`, `InsertTemplate`; re-insertion is always allowed), template creation from notes (`CreateTemplate`, `CreateTemplateFromNote` with case-insensitive check and `force`), embedded default templates in `internal/vault/defaults/*.txt`, and `SeedDefaultTemplates`/`SeedDefaultTemplatesIfAbsent` (seeded on vault setup, never in `vault.Open()`).
- `internal/tui`: Bubble Tea + Bubbles + Lipgloss + `charmbracelet/x/ansi` + `go-osc52/v2` interface with 20%/80% master-detail layout (plus Zen mode at 100% width), direct `textarea` editor with Undo/Redo (`editor_history.go`), checklist/list/line ergonomics (`editor_actions.go`), inline Find & Replace (`editor_find.go`), cursor/word footer, optional line numbers, mouse scroll/click (`WithMouseCellMotion`), OSC 52 clipboard (`clipboard.go`), external conflict resolution modal (`modalConflict`), `$EDITOR` via `tea.ExecProcess`, adaptive Graphite palette, ANSI-aware floating modals, and Settings modal with automatic onboarding when `vault_path` is not set.
- `internal/cmd`: hierarchical router + `FlatAliases`, `Action[TArgs, TResult]`, global flags (`-F/--format`, `--vault`, `--config`), and TSV/table output.

## Invariants & Gotchas

1. **Zero CGO:** never introduce C dependencies; keep `CGO_ENABLED=0` working across all `make build-all` targets.
2. **Test isolation:** every test must use `t.TempDir()`, `t.Setenv(...)`, temporary `config.yaml` files, and temporary vaults. Never touch `~/.config/vault-note-cli`.
3. **TSV with 0 rows:** `writeTabular` never prints a header when there are zero records.
4. **Vault file safety:** validate names against path traversal (`..`, `/`, `\`, leading `.`), use atomic writes for `.txt` and `config.yaml`, and move deletions to `<vault>/.trash/` without silently deleting.
5. **Short-circuit:** `version` and `help` must not read `config.yaml` or fail due to a malformed `.vaultnotecli`.
6. **Command synchronization:** when adding/changing commands or shortcuts, update `internal/cmd/router.go`, `internal/cmd/alias.go`, `internal/cmd/help.go`, `README.md`, and `skill/vault-note-cli.md` together.
7. **TUI tests:** use `Options{DisableCursorBlink: true}` for deterministic headless execution without pending timers.
8. **Templates:** live in `<vault>/.templates/` and are intentionally hidden — do not turn this folder into a notebook or move templates into `settings`/`config.yaml`. Placeholders use **local** time; substituted values are never re-scanned.
9. **Template from note:** `--from` / TUI `S` / `Alt+T` copy the body verbatim and never alter the note; without `force`, existing names (including case variants) return `ErrAlreadyExists`. In the TUI, `Alt+T` saves the dirty buffer without writing it to disk.
10. **TUI notebook prefix:** `show_notebook_prefix` (default `true`) shows up to 3 runes of the notebook in the list (`WOR/reuniao`); root notes receive no prefix and active notebook filters hide it. Changing the toggle must not save or discard a dirty editor or active filters.
11. **Default templates:** `internal/vault/defaults/*.txt` are embedded via `//go:embed`; `SeedDefaultTemplatesIfAbsent` only seeds when `<vault>/.templates/` does not exist (never in `vault.Open()`), and `SeedDefaultTemplates` (CLI `template init`) restores only missing ones without overwriting, using case-insensitive comparison.
12. **TUI insertion:** uses `textarea.InsertString` (the textarea owns the cursor) and only marks `dirty`; the domain only composes (`InsertSnippet`/`RenderTemplateFor`) — do not reimplement cursor logic in `vault`. `--at` offsets are in runes, never bytes. Re-inserting a template is always allowed (no duplicate guard, flag, or modal).
