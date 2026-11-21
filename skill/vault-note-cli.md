---
name: vault-note-cli
description: >-
  Manage plain-text .txt notes and folder notebooks in a local vault with the
  `vault-note-cli` binary (single-binary Go TUI/CLI with YAML config).
  Use when the user asks to configure a vault path, add, list, view, search,
  edit, rename, move, or trash .txt notes and notebook folders.
---

# vault-note-cli

Single-binary Go plain-text notes TUI & CLI (`CGO_ENABLED=0`, no database or server).
- **Vault on filesystem:** 1st-level directories = notebooks; `.txt` files (in folders or vault root) = notes (always sorted alphabetically); hidden entries starting with `.` are ignored.
- **Safe delete:** deleting notes or notebooks moves them to `<vault>/.trash/<timestamp>/...`.
- **YAML settings file:** stores app settings (`vault_path`, `show_notebook_prefix`, `show_line_numbers`) persistently in `config.yaml`.
- Running `vault-note-cli` with no arguments in a TTY launches the master-detail TUI (20% notes list / 80% direct editor + `$EDITOR`), auto-opening the Settings modal on first run if `vault_path` is not set. The editor supports Undo/Redo (`Ctrl+Z`/`Ctrl+Y`), inline Find & Replace (`Ctrl+F`/`Ctrl+R`), checklist toggle (`Alt+X`), smart list continuation (`Enter`), line actions (`Alt+↑/↓`, `Alt+P`, `Alt+K`, `Tab`/`Alt+[/]`), Zen mode (`Alt+Z`/`z`), line numbers (`Alt+N`), mouse navigation, OSC 52 clipboard copy (`Alt+Y`/`Alt+W`/`Y`), and SHA-256 external conflict detection. The list shows a compact up-to-3-rune notebook prefix (e.g. `WOR/reuniao`) unless disabled in Settings or filtered by notebook.

## Configuration & Testing Overrides

- Standard config file locations:
  - Linux/BSD: `$XDG_CONFIG_HOME/vault-note-cli/config.yaml` (default `~/.config/vault-note-cli/config.yaml`)
  - macOS: `~/Library/Application Support/vault-note-cli/config.yaml`
  - Windows: `%APPDATA%\vault-note-cli\config.yaml`
- Testing overrides: `VAULT_NOTE_CLI_CONFIG=/tmp/test.yaml` or `--config /tmp/test.yaml`; `VAULT_NOTE_CLI_VAULT=/tmp/vault` or `--vault /tmp/vault`
- Transversal format flag: `-F tsv` (default) or `-F table` (works anywhere in args)

## Workspace Config (`.vaultnotecli`)

Discovered upward from `cwd` to `$HOME` (nearest wins, read-only):

```bash
NOTEBOOK=WORK
# VAULT_PATH=~/meu-vault
```

- Applies default notebook to `note add`, `note list`, `note view`, `note edit`, `note delete`, and TUI startup.
- Override per call: `-b OTHER` or escape hatch `-b ""`.

## Config (`config.yaml`)

```bash
vault-note-cli config set vault_path ~/meu-vault
vault-note-cli config set show_notebook_prefix false # sidebar prefix toggle (default true)
vault-note-cli config set show_line_numbers true     # editor line numbers toggle (default false)
vault-note-cli config get [key] [-n]                 # alias: cfg
vault-note-cli config path
vault-note-cli config open                           # prints path in headless
```

## Notebooks (1st-Level Folders)

Notebook names created or renamed are trimmed, restricted to a single word (no internal whitespace), and normalized to `UPPERCASE`.

```bash
vault-note-cli notebook add WORK                     # alias: nn
vault-note-cli notebook list [-t] [-n]               # aliases: ln, nbs
vault-note-cli notebook view WORK [-t]
vault-note-cli notebook rename WORK PROJETOS         # aliases: update, mv
vault-note-cli notebook delete PROJETOS [--force]    # moves to <vault>/.trash/
```

## Notes (`.txt` Files)

Notes are referenced by relative path (`WORK/reuniao.txt` or `WORK/reuniao`) or unambiguous title (`reuniao`, optionally scoped with `-b WORK`).

```bash
vault-note-cli note add "Título" "Conteúdo" [-b WORK] [-c "Corpo"] [--stdin] [-t template] [--open]  # aliases: add, new, n
vault-note-cli note list [-b WORK] [-q termo] [-n] [--timestamps]              # aliases: list, ls
vault-note-cli note view WORK/Título [--raw] [-t]                              # aliases: view, show, get
vault-note-cli note update WORK/Título "Novo" [-c "Corpo"] [-b HOME|none] [--stdin]  # aliases: update, mod, mv
vault-note-cli note insert WORK/Título -t checklist [--prepend|--at N] [--print] [--open]  # aliases: insert, ins, apply
vault-note-cli note edit WORK/Título                                           # opens .txt in $EDITOR
vault-note-cli note search "termo" [-b WORK] [-n] [--timestamps]               # aliases: search, find
vault-note-cli note delete WORK/Título                                         # moves to .trash/; aliases: delete, del, rm
```

## Templates (`.templates/`)

Templates are plain `.txt` skeletons in `<vault>/.templates/<name>.txt` (hidden folder: never a notebook, never listed as a note). First vault setup seeds four English defaults — `checklist`, `daily`, `decision`, `meeting` — only when `.templates/` does not exist; `template init` restores missing defaults without overwriting existing files.

```bash
vault-note-cli template add reuniao --stdin < reuniao.txt   # or: template add reuniao -e
vault-note-cli template add modelo --from "Reuniao 12-05" -b WORK  # copy a note body verbatim
vault-note-cli template add modelo --from "Reuniao 12-05" -b WORK -f  # overwrite existing
vault-note-cli template list [-n]                           # aliases: tpls, lt
vault-note-cli template view reuniao [--raw]
vault-note-cli template update reuniao "novo corpo" [--stdin] [-e]
vault-note-cli template delete reuniao                      # moves to <vault>/.trash/
vault-note-cli template init                                # restore missing defaults (alias: seed)
vault-note-cli add "Reuniao 2026-05-12" -b WORK -t reuniao  # create note from template
vault-note-cli add "Rascunho" -t reuniao --open             # then open it in $EDITOR
```

Save a note as a template with `template add <name> --from <note-ref> [-b <notebook>] [-f] [--open]`. The note body is copied verbatim (no placeholder rewriting) and the original note is never modified. Without `-f`, an existing template returns exit 1 and writes nothing. In the TUI, `S` on the list snapshots the selected note, and `Alt+T` in the editor snapshots the current buffer without saving the note.

Insert an existing template into an existing note with `note insert` (default: append at the end; `--prepend`, `--at N` in runes, `--print` dry-run, `--open`). In the TUI, `Ctrl+T` in the editor inserts at the cursor (buffer only, save with `Ctrl+S`) and `T` on the list appends to the file. Re-inserting the same template is always allowed: there is no duplicate guard, flag, or confirmation modal.

Placeholders: `{{title}}`, `{{notebook}}` (empty at vault root), `{{date}}` (local `2006-01-02`), `{{datetime}}` (local RFC3339), `{{template}}`. Unknown placeholders stay literal; there is no escape syntax and substituted values are not re-scanned. `--template` cannot be combined with `-c`, `--stdin`, or a positional body (exit 2). A missing template fails before any write.

## Trash (`.trash/`)

```bash
vault-note-cli trash list [-t] [-n]                          # aliases: tr ls
vault-note-cli trash empty --force                           # permanently deletes .trash/; aliases: empty-trash -f, tr clean -f
```

- `trash empty` refuses to delete non-empty trash without `--force` / `-f` (exit 1).
- In the TUI, press `D` in the notes list to empty `<vault>/.trash/` with a confirmation modal.

## Gotchas

- Empty TSV listings output nothing (no header row).
- Deleting notes or folders never destroys files permanently — they are moved into `<vault>/.trash/<timestamp>/...`.
- Note titles cannot contain `/`, `\`, `..`, or start with `.` — so a title like `Reuniao 12/05` is rejected; use `Reuniao 12-05`.
- Exit codes: `0` success, `1` runtime error, `2` usage/argument error.
