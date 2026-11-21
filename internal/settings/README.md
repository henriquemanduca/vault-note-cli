# internal/settings

## Overview

Persistência das configurações globais do `vault-note-cli` em arquivo YAML (`config.yaml`) usando `gopkg.in/yaml.v3` (100% Go puro, `CGO_ENABLED=0`) com escrita atômica.

## Interface

- `KeyVaultPath` (`"vault_path"`)
- `KeyShowNotebookPrefix` (`"show_notebook_prefix"`, padrão `true`)
- `KeyShowLineNumbers` (`"show_line_numbers"`, padrão `false`)
- `DefaultPath() (string, error)` (respeita `VAULT_NOTE_CLI_CONFIG`)
- `NewStore(path string) (*Store, error)` / `(*Store).Path()`
- `Load() (AppSettings, error)` / `Save(cfg AppSettings) error`
- `GetVaultPath` / `SetVaultPath` / `GetShowNotebookPrefix` / `SetShowNotebookPrefix` / `GetShowLineNumbers` / `SetShowLineNumbers` / `GetSetting` / `SetSetting` / `DeleteSetting` / `ListSettings` / `NormalizePath`
- `ParseBool` aceita `true`/`false`/`1`/`0`/`on`/`off` (case-insensitive).

## Invariants

- Arquivo inexistente é tratado como configuração vazia (`AppSettings{}`) sem criar lixo em disco durante leituras.
- Escrita atômica via arquivo temporário no mesmo diretório + `Sync` + `os.Rename`.
- `SetVaultPath` rejeita caminhos vazios, expande `~/` e grava o caminho absoluto normalizado em `config.yaml`.
- `SetVaultPath` e `SetShowNotebookPrefix` preservam as demais configurações em uma única escrita atômica.
- `show_notebook_prefix` ausente equivale a `true`; `DeleteSetting` volta ao padrão sem apagar `vault_path`.
- `ListSettings` ordena por chave; `show_notebook_prefix` aparece antes de `vault_path` quando definido. Sem a chave, a lista continua contendo apenas `vault_path`.
- YAML malformado falha explicitamente informando o caminho do arquivo.

## Side Effects

- Cria o diretório pai e grava `config.yaml` em disco ao chamar `Save` / `SetVaultPath` / `SetShowNotebookPrefix` / `SetSetting` / `DeleteSetting`.

## Verification

- `go test -race -count=1 ./internal/settings/...`
