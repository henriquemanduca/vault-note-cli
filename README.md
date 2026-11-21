# vault-note-cli

A fast **plain `.txt` note-taking app** for the terminal written in Go, distributed as a **single static binary (`CGO_ENABLED=0`)**, with a **filesystem-based vault** (folders = notebooks, `.txt` = notes), **YAML configuration (`config.yaml`)**, and a dual interface (**interactive TUI** + **scriptable CLI**).

## Highlights

- **Plain `.txt` file vault:** each top-level folder inside the vault is a `notebook` and each `.txt` file is a note — 100% portable, readable by any editor, and ready for Git/Syncthing. Loose `.txt` files in the vault root are treated as notebook-less notes, always listed in alphabetical order.
- **Safe internal trash (`.trash/`):** deleting a note or notebook moves files to `<vault>/.trash/<timestamp>/...`, preventing accidental data loss.
- **YAML configuration (`config.yaml`):** no database; the vault path (`vault_path`) and preferences are stored in a simple YAML file, editable via the **floating Settings modal** in the TUI (opens automatically on first run or via `c`) and via the CLI (`config`).
- **Master-Detail TUI Layout (20% / 80%) + Zen Mode:** note list on the left (20%) and TUI editor (`textarea`) on the right (80%) ready for direct typing with Undo/Redo, inline Find & Replace, checklist/list ergonomics, line actions, cursor/word info bar, optional line numbers, mouse scroll/click support, OSC 52 clipboard copy (works over SSH/tmux/screen), external conflict protection, and `$EDITOR` / `$VISUAL` integration via `E` or `Ctrl+E`.
- **Adaptive Graphite Theme:** high contrast on both light and dark terminals (`lipgloss.AdaptiveColor`) with ANSI-aware floating modals.

## Installation & Build

```bash
# Build static binary in ./bin/vault-note-cli
make build

# Install to ~/.local/bin (or PREFIX=/usr/local)
make install

# Install AI agent skill
make install-skill
```

## Configuration Directories

- **Linux/BSD:** `$XDG_CONFIG_HOME/vault-note-cli/config.yaml` (fallback `~/.config/vault-note-cli/config.yaml`)
- **macOS:** `~/Library/Application Support/vault-note-cli/config.yaml`
- **Windows:** `%APPDATA%\vault-note-cli\config.yaml`
- **Notes vault (configurable):** set in `vault_path` inside `config.yaml` (via the TUI Settings modal or `vault-note-cli config set vault_path <dir>`).

Example `config.yaml`:

```yaml
vault_path: /home/user/my-vault
show_notebook_prefix: true
show_line_numbers: false
```

### Vault Structure

```text
<vault_path>/
├── loose.txt              # Note without a notebook (vault root)
├── WORK/                  # Notebook (top-level folder)
│   └── meeting.txt        # Note inside WORK (ID: WORK/meeting.txt)
├── PERSONAL/              # Notebook
│   └── groceries.txt      # Note inside PERSONAL (ID: PERSONAL/groceries.txt)
├── .templates/            # Note templates (hidden folder, not a notebook)
│   └── meeting.txt
└── .trash/                # Internal trash (ignored in listings)
    └── 20261120T210000Z/
        └── WORK/old.txt
```

## TUI Usage

Run without arguments in an interactive terminal (or `vault-note-cli tui` / `vault-note-cli ui`):

```bash
vault-note-cli
vault-note-cli tui -b WORK
```

On first run (when no `vault_path` is saved in `config.yaml`), the **Settings modal** opens automatically prompting for the vault path (creating the folder if it does not exist). The same modal lets you toggle the compact notebook prefix in the sidebar list (up to 3 characters, e.g., `WOR/meeting`; enabled by default) and editor line numbers (`show_line_numbers`; disabled by default).

If a note is modified externally on disk (e.g., via Syncthing, Git, or another editor) while open in the TUI, saving detects the SHA-256 content mismatch and opens a **Conflict modal** offering safe options: overwrite disk (`o`), reload from disk (`r`), save a timestamped conflict copy (`c`), or cancel (`Esc`).

### Keybindings

| Context | Key | Action |
| :--- | :--- | :--- |
| **List (left 20%)** | `j`/`k`, `↑`/`↓`, `g`/`G` | Navigate between `.txt` notes |
| | `Enter` / `Tab` / `l` | Focus the right-hand TUI editor ready to type |
| | `n` | New `.txt` note (with optional template; automatically focuses editor) |
| | `m` | Rename `.txt` file or move between folders/notebooks |
| | `E` | Open note in external editor (`$EDITOR`) |
| | `Y` | Copy selected note content to clipboard (OSC 52) |
| | `z` | Open selected note in Zen mode (100% editor width) |
| | `T` | Append a template to the end of the selected note |
| | `S` | Save the selected note as a new template |
| | `d` | Move note to `<vault>/.trash/` (with confirmation) |
| | `D` | Permanently empty `<vault>/.trash/` (with confirmation) |
| | `/` | Filter notes in real time (title, folder, and content) |
| | `b` | Notebooks/Folders modal (filter, `n` create, `u` rename, `d` trash) |
| | `c` / `,` | Settings modal (`vault_path`, sidebar prefix, and editor line numbers) |
| | `r` / `?` / `q` | Reload vault / Help / Save and quit |
| **Editor (right 80%)** | Free typing | Edit text directly in the TUI (footer shows `Ln, Col`, word & char counts) |
| | `Ctrl+S` | Save immediately to the `.txt` file (with external conflict check) |
| | `Ctrl+Z` / `Ctrl+Y` | Undo / Redo (with smart typing coalescing and cursor restoration) |
| | `Ctrl+F` / `Ctrl+R` | Inline Find / Find & Replace (`Enter` next, `Alt+C` case, `Alt+R` replace, `Alt+A` replace all) |
| | `Alt+X` | Toggle checkbox (`- [ ]` ↔ `- [x]`) on the current line |
| | `Enter` | Smart list continuation (`- `, `* `, `- [ ] `, `1. `; clears empty item) |
| | `Alt+↑` / `Alt+↓` | Move current line up / down |
| | `Alt+P` / `Alt+K` | Duplicate current line / Delete current line |
| | `Tab` / `Alt+]` / `Alt+[` | Indent / Dedent current line (2 spaces) |
| | `Alt+Z` | Toggle Zen mode (hide/show sidebar for 100% editor width) |
| | `Alt+N` | Toggle editor line numbers for the current session |
| | `Alt+Y` / `Alt+W` | Copy current line / full note buffer to clipboard (OSC 52) |
| | `Ctrl+E` | Open current buffer in `$EDITOR` |
| | `Ctrl+T` | Insert a template at cursor position (buffer only; save with `Ctrl+S`) |
| | `Alt+T` | Save current editor buffer as a template (without saving the note) |
| | `Esc` / `Shift+Tab` | Save pending changes, exit Zen mode if active, and return to the list |
| **Mouse** | Scroll / Click | Scroll list, editor, or list modals; click to switch panes, narrow tabs, or select notes |

## CLI Usage

```bash
# Settings (config.yaml)
vault-note-cli config set vault_path ~/my-vault
vault-note-cli config set show_notebook_prefix false
vault-note-cli config set show_line_numbers true
vault-note-cli config get
vault-note-cli config path
vault-note-cli config open

# Notebooks (top-level folders; trimmed, single-word, normalized to UPPERCASE)
vault-note-cli notebook add WORK                     # alias: nn
vault-note-cli notebook list                         # aliases: ln, nbs
vault-note-cli notebook view WORK -t
vault-note-cli notebook rename WORK PROJECTS
vault-note-cli notebook delete PROJECTS --force      # moves folder to .trash/

# Notes (.txt files)
vault-note-cli add "Ideas" "20/80 layout" -b WORK
echo "Piped text" | vault-note-cli add "Pipeline" -b WORK --stdin
vault-note-cli list -b WORK
vault-note-cli list -F table --timestamps
vault-note-cli view WORK/Ideas
vault-note-cli view WORK/Ideas --raw
vault-note-cli update WORK/Ideas "New Ideas" -b none
vault-note-cli edit "New Ideas"                      # opens .txt directly in $EDITOR
vault-note-cli search "layout"
vault-note-cli delete "New Ideas"                    # moves .txt to .trash/

# Notes from templates (reusable skeleton)
vault-note-cli add "Meeting 2026-05-12" -b WORK -t meeting
vault-note-cli add "Draft" -t meeting --open         # opens in $EDITOR after creation

# Trash (.trash/)
vault-note-cli trash list [-t] [-n]                  # aliases: tr ls
vault-note-cli trash empty --force                   # aliases: empty-trash -f, tr clean -f
```

## Note Templates

Templates are plain `.txt` files in `<vault>/.templates/<name>.txt` — the folder is intentionally hidden: it never becomes a notebook, never appears in `list`, and is excluded from `notebook list`.

| Placeholder | Value |
| :--- | :--- |
| `{{title}}` | Title of the note being created |
| `{{notebook}}` | Notebook of the note (empty when the note is in the root) |
| `{{date}}` | **Local** date in `2006-01-02` format |
| `{{datetime}}` | **Local** date/time in RFC3339 format |
| `{{template}}` | Name of the template used |

```bash
# Create/overwrite from a file
vault-note-cli template add meeting --stdin < meeting.txt
# or by editing in $EDITOR
vault-note-cli template add meeting -e

vault-note-cli template list [-n]                  # aliases: tpls, lt
vault-note-cli template view meeting [--raw]
vault-note-cli template update meeting "new body"
vault-note-cli template delete meeting             # moves to .trash/
vault-note-cli template init                       # restores missing defaults (alias: seed)
```

On the first `config set vault_path` (CLI) or initial TUI setup, four default English templates are created in `<vault>/.templates/`: `checklist`, `daily`, `decision`, and `meeting`. If `.templates/` already exists, nothing is created — even after you delete a template. To restore missing defaults without overwriting existing ones, run `template init`.

- `--template` cannot be combined with `-c`, `--stdin`, or a positional body (usage error, exit 2): single content source per invocation.
- Unknown placeholders remain literal (`{{foo}}` stays as-is); there is no escaping or recursion.
- A nonexistent template fails **before** any write: no partial note is created.

### Save a note as a template

```bash
# copy note content without altering the original
vault-note-cli template add tpl --from "WORK/Meeting 12-05"
vault-note-cli template add tpl --from "Meeting 12-05" -b WORK
# overwrite an existing template
vault-note-cli template add tpl --from "Meeting 12-05" -b WORK -f
# open the template in $EDITOR after saving
vault-note-cli template add tpl --from "Meeting 12-05" -b WORK --open
```

- `--from` copies the body **verbatim** (placeholders like `{{title}}` remain literal) and never modifies the original note.
- `--from` cannot be combined with a positional body, `--stdin`, or `-e/--edit` (usage error, exit 2).
- Without `--force`, an existing template blocks creation (exit 1, nothing written); `-f/--force` overwrites while preserving the existing name's casing.
- In the TUI: `S` in the list saves the selected note as a template; `Alt+T` in the editor saves the current buffer (including unsaved edits) without writing the note to disk.

### Insert a template into an existing note

```bash
# append to the end (default)
vault-note-cli note insert "WORK/Project X" -t checklist
# at the top, or at a rune offset
vault-note-cli note insert "WORK/Project X" -t checklist --prepend
vault-note-cli note insert "WORK/Project X" -t checklist --at 120
# preview without writing
vault-note-cli note insert "WORK/Project X" -t checklist --print
# open the note in $EDITOR after inserting
vault-note-cli note insert "WORK/Project X" -t checklist --open
```

- Aliases: `insert`, `ins`, `apply`.
- `--at N` counts **runes**, never bytes (accents and emoji never split); out-of-range values are clamped to start/end.
- `--print` composes and prints without writing (notice on `stderr`).
- In the TUI: `Ctrl+T` in the editor inserts at cursor (buffer only — `Ctrl+S` saves); `T` in the list appends to the end of the file.

Re-inserting the same template is always allowed: there is no duplicate guard, flag, or confirmation modal. Each insertion simply adds another copy.

## Workspace Configuration (`.vaultnotecli`)

Create a `.vaultnotecli` file in your project directory:

```bash
# .vaultnotecli
NOTEBOOK=WORK
# VAULT_PATH=~/my-vault
```

- Automatically discovered by walking upward to `$HOME` (nearest file wins; read-only).
- Notebook precedence: `-b` flag > `-b ""` (escape hatch) > `.vaultnotecli` > global.
- Vault precedence: `--vault` > `VAULT_NOTE_CLI_VAULT` > `.vaultnotecli` (`VAULT_PATH`) > `config.yaml` (`vault_path`).

## Quality & Testing

```bash
make check          # gofmt + go vet + go test -race -count=1 ./...
make build-all      # cross-compilation for Linux/macOS/Windows
make checksums      # generates dist/checksums.txt
```
