# AGENTS.md

Contrato operacional para agentes de IA e desenvolvedores trabalhando no repositório `vault-note-cli`.

## Comandos Principais

```bash
make check          # Gate obrigatório: fmt + vet + go test -race -count=1 ./...
make build          # Binário estático CGO_ENABLED=0 em bin/vault-note-cli
make test           # go test -race -count=1 ./...
make build-all      # Cross-compilation em dist/
make checksums      # Gera dist/checksums.txt
make run            # Executa com DB de configurações descartável (/tmp/vault-note-cli-dev.db)
```

## Mapa da Arquitetura

- `cmd/vault-note-cli/main.go`: entrypoint mínimo (`signal.NotifyContext`, `cmd.Run`, `os.Exit`). Nenhuma regra de domínio aqui.
- `internal/buildinfo`: metadados `Version`, `Commit`, `Date` injetados via `-ldflags`.
- `internal/filestore`: diretórios padrão XDG/macOS/Windows (`vault-note-cli.db`) e `OpenFolder`/`OpenFile` com fallback headless.
- `internal/config`: descoberta ascendente (até `$HOME`) e parsing estrito de `.vaultnotecli` (`NOTEBOOK`, `VAULT_PATH`).
- `internal/db`: persistência SQLite pure Go (`modernc.org/sqlite`), WAL, dedicada exclusivamente às configurações do aplicativo (`settings` — ex.: `vault_path`).
- `internal/vault`: persistência baseada em sistema de arquivos onde pastas de 1º nível = cadernos (`notebooks`), arquivos `.txt` puros = notas (incluindo raiz do vault), escrita atômica e lixeira interna `<vault>/.trash/`.
- `internal/tui`: interface Bubble Tea + Bubbles + Lipgloss + `charmbracelet/x/ansi` com layout mestre-detalhe 20%/80%, editor direto `textarea`, `$EDITOR` via `tea.ExecProcess`, paleta Graphite adaptativa, modais flutuantes ANSI-aware e modal de Configurações com onboarding automático quando `vault_path` não está definido.
- `internal/cmd`: roteador hierárquico + `FlatAliases`, `Action[TArgs, TResult]`, flags transversais (`-F/--format`, `--vault`, `--db` oculta) e saída TSV/table.

## Invariantes e Armadilhas (Gotchas)

1. **Zero CGO:** nunca introduza dependências C; mantenha `CGO_ENABLED=0` funcionando em todos os alvos de `make build-all`.
2. **Isolamento de testes:** todo teste deve usar `t.TempDir()`, `t.Setenv(...)`, bancos SQLite temporários e vaults temporários. Nunca toque em `~/.local/share/vault-note-cli`.
3. **TSV com 0 linhas:** `writeTabular` nunca imprime cabeçalho quando há zero registros.
4. **Segurança de arquivos no vault:** valide nomes contra path traversal (`..`, `/`, `\`, `.` inicial), use escrita atômica para `.txt` e mova exclusões para `<vault>/.trash/` sem apagar silenciosamente.
5. **Short-circuit:** `version` e `help` não podem abrir o banco nem falhar por `.vaultnotecli` malformado.
6. **Sincronização de comandos:** ao adicionar/alterar comandos ou atalhos, atualize em conjunto `internal/cmd/router.go`, `internal/cmd/alias.go`, `internal/cmd/help.go`, `README.md` e `skill/vault-note-cli.md`.
7. **Testes de TUI:** use `Options{DisableCursorBlink: true}` para execução headless determinística sem timers pendentes.
