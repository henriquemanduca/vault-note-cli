# internal/db

## Overview

Persistência SQLite embarcada em Go puro (`modernc.org/sqlite`, `CGO_ENABLED=0`) dedicada exclusivamente às configurações persistentes do `vault-note-cli` (tabela `settings`, como `vault_path`).

## Interface

- `DefaultDBPath() (string, error)` (respeita `VAULT_NOTE_CLI_DB`)
- `NewStore(path string) (*Store, error)` / `(*Store).Close()` / `(*Store).Path()`
- `GetSetting` / `SetSetting` / `DeleteSetting` / `ListSettings`
- `GetAppSettings` / `GetVaultPath` / `SetVaultPath` / `NormalizePath`

## Invariants

- Zero CGO (`modernc.org/sqlite`), `MaxOpenConns(1)`, `WAL`, `foreign_keys=ON`, `busy_timeout=5000`.
- Schema aplicado de forma idempotente (`CREATE TABLE IF NOT EXISTS settings`).
- `SetVaultPath` rejeita caminhos vazios, expande `~/` e salva o caminho absoluto normalizado.

## Side Effects

- Cria o diretório pai e o arquivo SQLite em disco ao inicializar `NewStore`.

## Verification

- `go test -race -count=1 ./internal/db/...`
