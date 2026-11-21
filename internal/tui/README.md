# internal/tui

## Overview

Interface interativa de terminal (Bubble Tea + Bubbles + Lipgloss + `charmbracelet/x/ansi` + `go-osc52/v2`) com layout Mestre-Detalhe 20%/80% e Modo Zen (100%), editor direto `textarea` (Undo/Redo, busca/substituição inline, checklists/listas/ações de linha, rodapé de métricas e números de linha configuráveis), mouse (`WithMouseCellMotion`), clipboard OSC 52, detecção de conflito externo (`modalConflict`), integração com `$EDITOR`, paleta Graphite adaptativa e modais flutuantes ANSI-aware (incluindo Configurações salvas em `config.yaml`).

## Interface

- `Options` (`InitialNotebook`, `DisableCursorBlink`, `ClipboardWriter`)
- `NewModel(settingsStore *settings.Store, v *vault.Vault, opts Options) Model`
- `Run(settingsStore *settings.Store, v *vault.Vault, opts Options) error`
- `ResolveEditor() []string` / `IsInteractiveTTY() bool`

## Invariants

- Guarda de TTY (`ErrNotTTY`) antes de iniciar `AltScreen`.
- Quando `v == nil` (nenhum `vault_path` salvo em `config.yaml`), o modal `modalSettings` abre automaticamente em modo de configuração inicial.
- Todo I/O de disco (arquivos `.txt`, pastas e `config.yaml`) roda em `tea.Cmd` assíncronos.
- Modais compostos por `compositeFloatingModal` preservam exatamente largura e altura da viewport mesmo com caracteres Unicode largos.

## Side Effects

- Lê e modifica arquivos `.txt` e pastas do vault via `internal/vault` e persiste `vault_path` em `config.yaml` via `internal/settings`.
- Suspende a TUI temporariamente ao abrir `$EDITOR` via `tea.ExecProcess`.

## Verification

- `go test -race -count=1 ./internal/tui/...`
