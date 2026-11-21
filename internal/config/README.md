# internal/config

## Overview

Descobre e interpreta arquivos `.vaultnotecli` no estilo `.env` para definir defaults de workspace (`NOTEBOOK`, `VAULT_PATH`) sem exigir repetição de flags por diretório.

## Interface

- `FileName` (`".vaultnotecli"`)
- `Config` (`Path`, `Notebook`, `VaultPath`)
- `Load(cwd string) (*Config, error)`

## Invariants

- Somente leitura: nunca cria ou altera `.vaultnotecli`.
- Descoberta ascendente até `$HOME` (inclusive) quando `cwd` está dentro de `$HOME`; arquivo mais próximo vence.
- Validação estrita: sintaxe inválida, aspas não fechadas ou chaves desconhecidas falham imediatamente informando arquivo e número da linha.

## Side Effects

- Leitura de metadados e conteúdo do arquivo `.vaultnotecli` mais próximo.

## Verification

- `go test -race -count=1 ./internal/config/...`
