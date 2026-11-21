---
name: vault-note-cli
description: >-
  Manage plain-text .txt notes and folder notebooks in a local vault with the
  `vault-note-cli` binary (single-binary Go TUI/CLI with SQLite settings store).
  Use when the user asks to configure a vault path, add, list, view, search,
  edit, rename, move, or trash .txt notes and notebook folders.
---

# vault-note-cli

Single-binary Go plain-text notes TUI & CLI (`CGO_ENABLED=0`, no server).
- **Vault on filesystem:** 1st-level directories = notebooks; `.txt` files (in folders or vault root) = notes; hidden entries starting with `.` are ignored.
- **Safe delete:** deleting notes or notebooks moves them to `<vault>/.trash/<timestamp>/...`.
- **SQLite settings store:** stores app settings (`vault_path`) persistently.
- Running `vault-note-cli` with no arguments in a TTY launches the master-detail TUI (20% notes list / 80% direct editor + `$EDITOR`), auto-opening the Settings modal on first run if `vault_path` is not set.

## Storage & Testing Overrides

- Standard data roots (`<data_root>`):
  - Linux/BSD: `$XDG_DATA_HOME/vault-note-cli` (default `~/.local/share/vault-note-cli`)
  - macOS: `~/Library/Application Support/vault-note-cli`
  - Windows: `%LOCALAPPDATA%\vault-note-cli`
- Settings DB: `<data_root>/vault-note-cli.db`
- Testing overrides: `VAULT_NOTE_CLI_DB=/tmp/test.db` or `--db /tmp/test.db`; `VAULT_NOTE_CLI_VAULT=/tmp/vault` or `--vault /tmp/vault`
- Transversal format flag: `-F tsv` (default) or `-F table` (works anywhere in args)

## Workspace Config (`.vaultnotecli`)

Discovered upward from `cwd` to `$HOME` (nearest wins, read-only):

```bash
NOTEBOOK=WORK
# VAULT_PATH=~/meu-vault
```

- Applies default notebook to `note add`, `note list`, `note view`, `note edit`, `note delete`, and TUI startup.
- Override per call: `-b OTHER` or escape hatch `-b ""`.

## Config (SQLite Settings)

```bash
vault-note-cli config set vault_path ~/meu-vault
vault-note-cli config get [key] [-n]                 # alias: cfg
vault-note-cli config path
vault-note-cli config open                           # prints path in headless
```

## Notebooks (1st-Level Folders)

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
vault-note-cli note add "Título" "Conteúdo" [-b WORK] [-c "Corpo"] [--stdin]  # aliases: add, new, n
vault-note-cli note list [-b WORK] [-q termo] [-n] [--timestamps]              # aliases: list, ls
vault-note-cli note view WORK/Título [--raw] [-t]                              # aliases: view, show, get
vault-note-cli note update WORK/Título "Novo" [-c "Corpo"] [-b HOME|none] [--stdin]  # aliases: update, mod, mv
vault-note-cli note edit WORK/Título                                           # opens .txt in $EDITOR
vault-note-cli note search "termo" [-b WORK] [-n] [--timestamps]               # aliases: search, find
vault-note-cli note delete WORK/Título                                         # moves to .trash/; aliases: delete, del, rm
```

## Gotchas

- Empty TSV listings output nothing (no header row).
- Deleting notes or folders never destroys files permanently — they are moved into `<vault>/.trash/<timestamp>/...`.
- Exit codes: `0` success, `1` runtime error, `2` usage/argument error.
