# internal/cmd

## Overview

Camada CLI do `vault-note-cli`: roteador hierárquico (`<recurso> <ação>`), expansão declarativa de aliases planos, extração de flags transversais (`-F/--format`, `--vault` e `--db` oculta), pipeline `Action[TArgs, TResult]` (`Parse -> Execute -> Format`) e formatação tabular TSV/table.

## Interface

- `Run(ctx, args, stdin, stdout, stderr) int`
- `Context` (injeção de `*db.Store`, `*config.Config`, `VaultPath`, `RequireVault()`, `Format`, I/O)
- `Action[TArgs, TResult]` e `RunAction`
- `FlatAliases`

## Invariants

- `version` e `help` executam short-circuit sem abrir o SQLite nem exigir `.vaultnotecli` válido.
- Listagens TSV vazias não imprimem cabeçalho.
- Precedência de caderno: flag explícita `-b` > escape hatch `-b ""` > `.vaultnotecli` (`NOTEBOOK`) > global.
- Precedência de vault: `--vault` > `VAULT_NOTE_CLI_VAULT` > `.vaultnotecli` (`VAULT_PATH`) > SQLite (`vault_path`).
- Códigos de saída padronizados: `0` (sucesso), `1` (erro de execução), `2` (erro de uso).

## Side Effects

- Abre SQLite de configurações, carrega `.vaultnotecli`, opera sobre o vault em disco e escreve nos writers injetados.

## Verification

- `go test -race -count=1 ./internal/cmd/...`
