# internal/filestore

## Overview

Resolve diretórios padrão por sistema operacional (XDG/macOS/Windows) para o arquivo de configuração YAML (`config.yaml`) e oferece abertura nativa de pastas/arquivos com fallback seguro em ambientes headless/SSH.

## Interface

- `ConfigDir() (string, error)`
- `DefaultConfigPath() (string, error)`
- `DefaultVaultSuggestion() string`
- `OpenFolder(dir string) (OpenResult, error)` / `OpenFile(path string) (OpenResult, error)`

## Invariants

- Convenções nativas: `$XDG_CONFIG_HOME/vault-note-cli` (fallback `~/.config/vault-note-cli`), `~/Library/Application Support/vault-note-cli` (macOS), `%APPDATA%\vault-note-cli` (Windows).
- Em sessões sem interface gráfica (`SSH_CONNECTION` ou sem `DISPLAY`/`WAYLAND_DISPLAY` no Linux), retorna `OpenResult{Path: ..., Opened: false}` sem falhar.

## Side Effects

- `OpenFolder`/`OpenFile` invocam o gerenciador de arquivos nativo do SO quando há sessão gráfica.

## Verification

- `go test -race -count=1 ./internal/filestore/...`
