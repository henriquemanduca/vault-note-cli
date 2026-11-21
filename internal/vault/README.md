# internal/vault

## Overview

Camada de domínio baseada em sistema de arquivos do `vault-note-cli`: interpreta pastas de 1º nível do vault como cadernos (`Notebook`) e arquivos `.txt` puros (em cadernos ou na raiz do vault) como notas (`Note`).

## Interface

- `Open(root string) (*Vault, error)` / `EnsureDir(raw string) (string, error)` / `ExpandPath(raw string) (string, error)`
- Cadernos: `ListNotebooks`, `GetNotebook`, `CreateNotebook`, `RenameNotebook`, `DeleteNotebook`
- Notas: `ListNotes`, `GetNote`, `ResolveNote`, `CreateNote`, `UpdateNote`, `UpdateNoteContent`, `UpdateNoteContentChecked`, `CreateConflictCopy`, `ContentHash`, `DeleteNote`

## Invariants

- Apenas pastas visíveis de 1º nível (sem prefixo `.`) são tratadas como cadernos; `.trash`, `.git` e arquivos não-`.txt` são ignorados.
- Nomes de cadernos em `CreateNotebook` e `RenameNotebook` passam por trim, exigem palavra única (sem espaços internos) e são normalizados para `UPPERCASE`.
- Proteção contra path traversal: valida nomes (`..`, `/`, `\`, caracteres inválidos, prefixo `.`) e confirma que todo caminho permanece dentro da raiz do vault.
- Escrita atômica via arquivo temporário + `os.Rename` evita corrupção de `.txt`.
- Exclusões nunca removem definitivamente: movem o `.txt` ou pasta para `<vault>/.trash/<timestamp>/<caminho-relativo>`.

## Side Effects

- Cria, renomeia, sobrescreve atomicamente e move pastas/arquivos `.txt` dentro do diretório raiz do vault.

## Verification

- `go test -race -count=1 ./internal/vault/...`
