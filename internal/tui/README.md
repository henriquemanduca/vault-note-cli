# internal/tui

## Overview

Interface interativa de terminal (Bubble Tea + Bubbles + Lipgloss + `charmbracelet/x/ansi`) com layout Mestre-Detalhe 20%/80%, editor direto `textarea`, integração com `$EDITOR`, paleta Graphite adaptativa e modais flutuantes ANSI-aware (incluindo Configurações salvas no SQLite).

## Interface

- `Options` (`InitialNotebook`, `DisableCursorBlink`)
- `NewModel(settingsStore *db.Store, v *vault.Vault, opts Options) Model`
- `Run(settingsStore *db.Store, v *vault.Vault, opts Options) error`
- `ResolveEditor() []string` / `IsInteractiveTTY() bool`

## Invariants

- Guarda de TTY (`ErrNotTTY`) antes de iniciar `AltScreen`.
- Quando `v == nil` (nenhum `vault_path` salvo no SQLite), o modal `modalSettings` abre automaticamente em modo de configuração inicial.
- Todo I/O de disco (arquivos `.txt`, pastas e SQLite) roda em `tea.Cmd` assíncronos.
- Modais compostos por `compositeFloatingModal` preservam exatamente largura e altura da viewport mesmo com caracteres Unicode largos.

## Side Effects

- Lê e modifica arquivos `.txt` e pastas do vault via `internal/vault` e persiste `vault_path` no SQLite via `internal/db`.
- Suspende a TUI temporariamente ao abrir `$EDITOR` via `tea.ExecProcess`.

## Verification

- `go test -race -count=1 ./internal/tui/...`
