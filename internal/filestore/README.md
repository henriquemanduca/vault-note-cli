# internal/filestore

## Overview

Resolve diretórios padrão por sistema operacional (XDG/macOS/Windows) para o banco SQLite de configurações (`vault-note-cli.db`) e oferece abertura nativa de pastas/arquivos com fallback seguro em ambientes headless/SSH.

## Interface

- `BaseDir() (string, error)`
- `DefaultDBPath() (string, error)`
- `DefaultVaultSuggestion() string`
- `OpenFolder(dir string) (OpenResult, error)` / `OpenFile(path string) (OpenResult, error)`

## Invariants

- Convenções nativas: `$XDG_DATA_HOME/vault-note-cli` (fallback `~/.local/share/vault-note-cli`), `~/Library/Application Support/vault-note-cli` (macOS), `%LOCALAPPDATA%\vault-note-cli` (Windows).
- Em sessões sem interface gráfica (`SSH_CONNECTION` ou sem `DISPLAY`/`WAYLAND_DISPLAY` no Linux), retorna `OpenResult{Path: ..., Opened: false}` sem falhar.

## Side Effects

- `OpenFolder`/`OpenFile` invocam o gerenciador de arquivos nativo do SO quando há sessão gráfica.

## Verification

- `go test -race -count=1 ./internal/filestore/...`
