# internal/buildinfo

## Overview

Centraliza os metadados de build (`Version`, `Commit`, `Date`) injetados em tempo de compilação via `-ldflags` no `Makefile`.

## Interface

- `Version`, `Commit`, `Date` (variáveis de pacote)
- `Summary() string` — retorna linha única formatada `vault-note-cli <version> (commit <commit>, built <date>, <go> <os>/<arch>)`.

## Invariants

- Valores padrão seguros (`dev`, `none`, `unknown`) quando compilado sem `-ldflags`.
- Nenhuma dependência externa ou I/O de disco.

## Side Effects

Nenhum.

## Verification

- `go test -race -count=1 ./internal/buildinfo/...` e `./bin/vault-note-cli --version`.
