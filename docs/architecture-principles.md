# Princípios de Arquitetura e Requisitos de Engenharia

Este documento consolida os princípios arquiteturais, decisões técnicas e requisitos não-funcionais aplicados no `task-cli`, estruturados como um **guia reutilizável** para orientar o desenvolvimento de novas aplicações CLI/TUI em Go.

---

## Sumário

1. [Filosofia e Requisitos Fundamentais](#1-filosofia-e-requisitos-fundamentais)
2. [Estrutura de Projeto e Fronteiras de Pacotes](#2-estrutura-de-projeto-e-fronteiras-de-pacotes)
3. [Design de CLI (Linha de Comando)](#3-design-de-cli-linha-de-comando)
4. [Configuração de Workspace](#4-configuração-de-workspace)
5. [Persistência, Domínio e Armazenamento de Arquivos](#5-persistência-domínio-e-armazenamento-de-arquivos)
6. [Design de TUI (Interface Interativa de Terminal)](#6-design-de-tui-interface-interativa-de-terminal)
7. [Testabilidade, Qualidade e Observabilidade Local](#7-testabilidade-qualidade-e-observabilidade-local)
8. [Build, Distribuição e Contrato Operacional](#8-build-distribuição-e-contrato-operacional)
9. [Documentação Viva e Suporte a Agentes](#9-documentação-viva-e-suporte-a-agentes)
10. [Checklist para Novas Aplicações](#10-checklist-para-novas-aplicações)

---

## 1. Filosofia e Requisitos Fundamentais

Toda aplicação derivada deste padrão deve priorizar simplicidade operacional, previsibilidade e longevidade:

- **Binário único e estático (`CGO_ENABLED=0`):** nenhuma dependência de bibliotecas C, servidor em background ou runtime externo. A instalação resume-se a copiar um único executável.
- **Local-first e portátil:** dados e anexos vivem em diretórios padrão do sistema operacional, sem exigir conectividade de rede, e funcionam consistentemente em Linux, macOS e Windows.
- **Interface dupla sobre o mesmo núcleo:** a aplicação oferece uma **CLI scriptável** (focada em automação, pipes Unix e agentes) e, quando fizer sentido, uma **TUI interativa** consumindo exatamente os mesmos pacotes de domínio (`internal/db`, `internal/filestore`, `internal/config`).
- **Segurança de dados por padrão:** operações destrutivas preferem *soft delete*, estados terminais são imutáveis, importações são transacionais e arquivos físicos não são removidos silenciosamente sem auditoria explícita.
- **Zero surpresas em automação:** saída limpa em `stdout`, mensagens de erro em `stderr`, códigos de saída padronizados e detecção de TTY antes de acionar modos interativos.

---

## 2. Estrutura de Projeto e Fronteiras de Pacotes

### Layout de diretórios

```text
<app>/
├── cmd/<app>/main.go        # Entrypoint mínimo: sinais, contexto, dispatch e os.Exit
├── internal/
│   ├── buildinfo/           # Metadados de versão/commit/data injetados via -ldflags
│   ├── config/              # Descoberta e parsing de configuração do workspace
│   ├── cmd/                 # Roteador CLI, aliases, flags globais, handlers e formatação
│   ├── db/                  # Persistência embarcada (SQLite), schema, CRUD e import/export
│   ├── filestore/           # Diretórios padrão por SO, cópia/hash/abertura/poda de arquivos
│   └── tui/                 # Interface interativa de terminal (Bubble Tea + Lipgloss)
├── docs/                    # Manuais, ADRs/designs e guias de arquitetura
├── skill/                   # Definição de skill para uso por agentes de IA
├── Makefile                 # Contrato padronizado de build, testes, lint e instalação
├── .golangci.yml            # Baseline estrito de análise estática
├── AGENTS.md                # Contrato operacional e armadilhas conhecidas do repositório
└── README.md                # Documentação voltada ao usuário final
```

### Regras de acoplamento

- **`cmd/<app>/main.go` deve ser fino:** apenas inicializa contexto (incluindo `signal.NotifyContext` para `os.Interrupt`), despacha ao roteador e converte o resultado em código de saída via `os.Exit`. Nenhuma lógica de domínio deve residir em `main`.
- **Tudo privado fica em `internal/`:** impede acoplamento externo acidental e deixa explícita a fronteira de cada subsistema.
- **Injeção de dependências via `Context`:** evita variáveis globais de banco ou saída. O `Context` carrega `*db.Store`, `*config.Config`, `DBPath`, `Format`, `io.Writer` (`Out`) e `context.Context` (`Ctx`).
- **Fluxo acíclico de dependências:**

```text
cmd/<app>
   └── internal/cmd
          ├── internal/config
          ├── internal/db ──────► internal/filestore
          ├── internal/filestore
          └── internal/tui ─────► internal/db, internal/filestore
```

---

## 3. Design de CLI (Linha de Comando)

### Roteamento hierárquico com aliases ergonômicos

- **Estrutura canônica:** `<app> <recurso> <ação>` (ex.: `task-cli task add`, `task-cli project list`, `task-cli task doc open`).
- **Aliases planos de conveniência/retrocompatibilidade:** comandos frequentes ou legados (`add`, `list`, `done`, `lp`, `np`, `attach`) são expandidos de forma declarativa (tabela `FlatAliases`) para suas rotas canônicas antes da execução.
- **Ajuda em todos os níveis:** `<app> help <recurso> [ação]` e `-h/--help` em qualquer subcomando geram documentação contextual a partir da própria árvore de comandos.

### Contrato `Action[TArgs, TResult]` (SRP)

Cada comando deve seguir um pipeline desacoplado de três etapas (referência: [internal/cmd/handler.go](../internal/cmd/handler.go)):

1. **`Parse(ctx, args) (TArgs, error)`:** valida flags e argumentos posicionais, aplica defaults do workspace e devolve uma struct tipada.
2. **`Execute(ctx, TArgs) (TResult, error)`:** executa a regra de negócio usando apenas o `Store`/serviços do `Context`, sem escrever diretamente em `os.Stdout`.
3. **`Format(ctx, w, TResult) error`:** renderiza o resultado no `io.Writer` injetado.

Esse desenho permite testar parsing, regra de negócio e formatação de maneira independente.

### Flags e ergonomia de argumentos

- **Flags transversais:** flags globais (como `-F/--format` e `--db`) são extraídas antes do roteamento, podendo aparecer antes, entre ou depois dos subcomandos, respeitando o terminador `--`.
- **Flags e posicionais misturados:** a stdlib `flag.FlagSet` para no primeiro argumento posicional; por isso, um utilitário de particionamento (`splitFlagsAndPosArgs`) separa flags (conhecendo quais recebem valor) de posicionais, permitindo chamadas naturais como `task add "Título" -p 1`.
- **Short-circuit sem I/O:** comandos como `version` e `help` não devem abrir banco de dados nem exigir configuração válida de workspace.

### Formatação de saída e códigos de retorno

- **TSV como padrão em listagens:** fácil de consumir com `awk`, `cut`, `sort` e agentes; suporta `-n/--no-header` e **não imprime cabeçalho quando há zero registros**.
- **Sanitização de células:** campos de texto em linhas tabulares passam por normalização (`cleanOneLine`) removendo quebras de linha e tabs para nunca corromper o formato TSV.
- **Formato humano opcional (`-F table`):** usa `text/tabwriter` sobre os mesmos dados.
- **Códigos de saída padronizados:**
  - `0` — sucesso;
  - `1` — erro de execução/runtime;
  - `2` — erro de uso/argumentos inválidos.

---

## 4. Configuração de Workspace

Quando a aplicação atua sobre projetos locais, defaults por diretório eliminam repetição de flags (referência: [internal/config/README.md](../internal/config/README.md)):

- **Formato simples e legível:** arquivo `.env`-style (`KEY=VALUE`) com comentários `#`, prefixo opcional `export ` e valores entre aspas.
- **Descoberta ascendente limitada:** sobe do diretório atual (`cwd`) até `$HOME` (inclusive); o arquivo mais próximo vence. Se `cwd` estiver fora de `$HOME`, sobe até a raiz do sistema de arquivos.
- **Somente leitura:** o carregador nunca cria nem altera o arquivo de configuração automaticamente.
- **Precedência explícita e previsível:**
  1. Flag explícita na invocação (vence sempre);
  2. Escape hatch explícito com valor vazio (ex.: `-p ""`) para ignorar o default do workspace em uma única execução;
  3. Configuração do `.taskcli` (workspace);
  4. Comportamento padrão global.
- **Validação estrita:** chaves desconhecidas ou sintaxe inválida falham imediatamente informando **caminho do arquivo e número da linha**.

---

## 5. Persistência, Domínio e Armazenamento de Arquivos

### Banco de dados embarcado (SQLite em Go puro)

- **Driver pure Go:** usar `modernc.org/sqlite` (ou equivalente sem CGO) para preservar builds estáticos multiplataforma.
- **Inicialização defensiva:** criar diretórios automaticamente (`os.MkdirAll`), habilitar `PRAGMA journal_mode=WAL` e `PRAGMA foreign_keys=ON`, e aplicar schema de forma idempotente (`CREATE TABLE IF NOT EXISTS`).
- **Isolamento para testes e desenvolvimento:** aceitar override do caminho do banco via variável de ambiente (`<APP>_DB`) e flag interna (`--db`), evitando qualquer risco aos dados reais do usuário.

### Diretórios padrão por plataforma e migração segura

- **Respeitar convenções do SO:**
  - Linux/BSD: `$XDG_DATA_HOME/<app>` (fallback: `~/.local/share/<app>`);
  - macOS: `~/Library/Application Support/<app>`;
  - Windows: `%LOCALAPPDATA%\<app>`.
- **Migração de legado sem perda de dados:** ao mover arquivos de caminhos antigos (ex.: `~/.task-cli.db`), realizar cópia em stream com `fsync`, validar integridade (`PRAGMA quick_check`) no destino e somente então renomear a origem (`.migrated`) — nunca deletar o original antes da verificação.

### Integridade de domínio

- **Soft delete e imutabilidade:** registros removidos recebem `deleted_at` e estado terminal (`deleted`). Estados terminais (`deleted`, `aborted`) são imutáveis — qualquer tentativa de edição ou transição retorna erro de domínio explícito (`ErrTaskImmutable`), reforçado tanto na camada de banco quanto na CLI e TUI.
- **Constraints no banco:** unicidade case-insensitive (`COLLATE NOCASE`), chaves estrangeiras coerentes (`ON DELETE SET NULL` ou `ON DELETE CASCADE`) e defaults explícitos.

### Armazenamento gerenciado de arquivos (`filestore`)

Para aplicações que gerenciam anexos ou artefatos em disco:

- **Nomes livres de colisão:** `<data_root>/attachments/<entity_id>/<stem>_<sha256_prefix>.<ext>`.
- **Performance sem sacrificar portabilidade:** tentar cópia CoW (*reflink*) via chamadas específicas por OS (`reflink_linux.go`, `reflink_darwin.go`, `reflink_other.go` com build tags) e fazer fallback transparente para `io.Copy` em stream.
- **Proteção contra arquivos grandes:** definir limiar razoável (ex.: 100 MB) acima do qual vincula-se por referência por padrão, oferecendo flags explícitas (`--copy`, `--link`).
- **Consistência entre disco e banco (rollback compensatório):** se o registro no SQLite falhar após a cópia do arquivo, remover imediatamente o arquivo recém-copiado.
- **Retenção segura e limpeza explícita:** desvincular ou deletar logicamente um registro **não** apaga o arquivo físico; oferecer comando de auditoria/limpeza (`prune`) com suporte obrigatório a `--dry-run`.
- **Abertura nativa com fallback headless:** delegar ao gerenciador do SO (`xdg-open`, `open`, `explorer.exe`), mas detectar ausência de sessão gráfica (`DISPLAY`/`WAYLAND_DISPLAY`/SSH) e devolver o caminho resolvido sem quebrar a interface.

### Intercâmbio de dados (Import/Export JSON)

- **Schema versionado:** incluir campo `version` validado contra `CurrentSchemaVersion`.
- **Importação transacional e segura:** executar tudo dentro de uma transação SQL; suportar `--dry-run` (executa validações e faz rollback), `--strict` (aborta no primeiro erro em vez de pular com aviso), deduplicação previsível e leitura/escrita via `stdin`/`stdout` (`-`).

---

## 6. Design de TUI (Interface Interativa de Terminal)

Quando a aplicação oferecer modo interativo (`tui` / `ui`), seguir estes princípios (referência: [internal/tui/README.md](../internal/tui/README.md)):

- **Guarda de TTY:** validar `isatty` em `stdin` e `stdout` antes de iniciar o programa em `AltScreen`, retornando erro claro em ambientes não interativos (pipes/CI).
- **Arquitetura Elm (MVU) com Charm:** `bubbletea` + `bubbles` + `lipgloss` + `charmbracelet/x/ansi` (100% Go puro). Operações de I/O (leituras/mutações no banco, anexos, abertura de pastas) ocorrem via `tea.Cmd` assíncronos, nunca bloqueando `View()`.
- **Layout responsivo por breakpoints:** adaptar dinamicamente a quantidade de painéis conforme `tea.WindowSizeMsg` (ex.: amplo `>= 105` cols com 3 painéis, médio `70–104` cols com 2 painéis, estreito `< 70` cols com abas) e exibir aviso limpo abaixo do tamanho mínimo viável (`< 36x12`).
- **Tema visual estruturado e adaptativo (Graphite):**
  - Centralizar todos os tokens de cor em uma `palette` usando `lipgloss.AdaptiveColor` (suporte nativo a terminais claros e escuros).
  - Priorizar **chrome neutro/acromático** (tons grafite/carvão/prata para bordas, cabeçalhos, seleções e abas) combinado com **cores semânticas dessaturadas** para status, prioridades e alertas — garantindo alto contraste sem poluição visual.
  - Estilizar também componentes auxiliares (`textinput` prompt, placeholder e cursor) a partir da mesma paleta.
- **Modais flutuantes compostos sobre o workspace:**
  - Renderizar formulários, seletores, confirmações e ajuda como caixas flutuantes centralizadas sobre o layout principal esmaecido (*backdrop scrim*), preservando contexto visual.
  - Utilizar funções conscientes de ANSI e grafemas (`ansi.Strip`, `ansi.Cut`, `ansi.StringWidth`) para garantir alinhamento exato mesmo com caracteres Unicode largos (CJK/emojis) e fallback gracioso quando o terminal for menor que o modal.
- **Integração externa limpa:**
  - Edição de textos longos via `$EDITOR` usando `tea.ExecProcess` sobre arquivo temporário devidamente removido após o término.
  - Em ambientes headless, capturar caminhos de fallback na barra de notificações da TUI em vez de escrever em `os.Stdout`.
- **Navegação ergonômica:** atalhos consistentes inspirados em Vim/TUI (`j/k`, `g/G`, `Tab`/`Shift+Tab`, `/`, `Esc`, `?`, `q`, `Ctrl+C`), com ajuda contextual no rodapé e modal de atalhos.

---

## 7. Testabilidade, Qualidade e Observabilidade Local

### Estratégia de testes

- **Detector de race sempre ativo:** `go test -race -count=1 ./...` como padrão em `make test` e `make check` (`-count=1` evita mascarar falhas por cache).
- **Isolamento total:** todo teste usa `t.TempDir()`, `t.Setenv(...)` e bancos temporários; nenhum teste pode ler ou modificar `~/.local/share/<app>` ou o ambiente real do desenvolvedor.
- **Handlers e I/O testáveis:** injetar `bytes.Buffer` ou helpers (`captureStdout`, `runWithStdin`) para validar saídas exatas e comportamento de pipes.
- **Testes determinísticos de TUI:** exercitar `Model.Init/Update/View` de forma síncrona/headless em testes, desativando blink timers de cursor (`cursor.CursorStatic`) e cobrindo múltiplos tamanhos de terminal, fluxos CRUD completos, invariantes de imutabilidade e propriedades visuais (ex.: validade hex e neutralidade dos tokens do tema, dimensões exatas dos modais flutuantes).

### Análise estática e formatação

- **Baseline `golangci-lint` v2** (referência: [.golangci.yml](../.golangci.yml)):
  - Linters obrigatórios: `errcheck`, `govet`, `ineffassign`, `staticcheck`, `unused`, `gocritic`, `misspell`, `revive`, `unconvert`, `unparam`, `copyloopvar`.
  - Formatadores: `gofmt` e `goimports` (com `local-prefixes` apontando para o módulo da aplicação).
- **Gate local obrigatório:** `make check` (`fmt` + `vet` + `test -race`) deve passar antes de qualquer commit.

---

## 8. Build, Distribuição e Contrato Operacional

### Metadados de build (`internal/buildinfo`)

Expor `Version`, `Commit` e `Date` como variáveis de pacote sobrescritas via `-ldflags` no `Makefile`, combinadas com `runtime.Version()`, `runtime.GOOS` e `runtime.GOARCH`. Isso elimina strings de versão duplicadas no código-fonte.

### Compilação estática e cross-compilation

```makefile
LDFLAGS := -s -w \
	-X '$(MODULE)/internal/buildinfo.Version=$(VERSION)' \
	-X '$(MODULE)/internal/buildinfo.Commit=$(COMMIT)' \
	-X '$(MODULE)/internal/buildinfo.Date=$(DATE)'
BUILD   := CGO_ENABLED=0 $(GO) build -trimpath -ldflags "$(LDFLAGS)"
```

- `-trimpath` remove caminhos locais da máquina de build (reprodutibilidade e privacidade).
- `-s -w` reduz o tamanho do binário final.
- `build-all` gera artefatos para `linux/{amd64,arm64,armv7}`, `darwin/{amd64,arm64}` e `windows/amd64`, acompanhados de `checksums.txt` (`sha256sum`).

### Alvos obrigatórios do `Makefile`

Todo projeto seguindo este padrão deve implementar os alvos mínimos (com `help` auto-documentado como `.DEFAULT_GOAL`):

| Alvo | Finalidade |
| :--- | :--- |
| `help` | Lista alvos disponíveis e descrições |
| `build` | Compila binário estático em `bin/<app>` (gitignored) |
| `build-all` / `checksums` | Cross-compilation de release e hashes SHA-256 |
| `run` | Executa a partir do código-fonte (sempre com DB descartável) |
| `test` / `test-cover` | Executa suíte com `-race -count=1` e relatório HTML de cobertura |
| `fmt` / `vet` / `lint` | Formatação, análise estática e linter |
| `check` | Gate completo pré-commit (`fmt` + `test`) |
| `clean` | Limpa artefatos de build e cobertura |
| `install` / `uninstall` | Instala/remove binário em `$(PREFIX)/bin` (padrão `~/.local/bin`) |

---

## 9. Documentação Viva e Suporte a Agentes

Projetos modernos são operados tanto por humanos quanto por agentes de código/automação. Para reduzir deriva de documentação:

- **README por pacote (`internal/<pkg>/README.md`):** estrutura enxuta e padronizada com 5 seções — *Overview*, *Interface*, *Invariants*, *Side Effects* e *Verification*. Facilita onboarding rápido e preserva invariantes arquiteturais.
- **`AGENTS.md` na raiz:** concentra comandos de build/teste, mapa de arquitetura, armadilhas conhecidas (*gotchas*) e regras de qualidade para assistentes de código.
- **Skill de uso (`skill/<app>.md`):** resumo orientado a tarefas reais e comandos exatos, instalável via `make install-skill`, permitindo que agentes operem a CLI corretamente sem adivinhar flags.
- **Sincronização em novos comandos:** ao adicionar ou alterar um comando, atualizar conjuntamente o roteador, o `printUsage`, o `README.md` e a skill.

---

## 10. Checklist para Novas Aplicações

Use esta lista ao iniciar uma nova aplicação Go CLI/TUI baseada nestes princípios:

- [ ] Criar módulo Go com layout `cmd/<app>/main.go` + `internal/{buildinfo,config,cmd,db,...}`.
- [ ] Adotar `Makefile` com alvos obrigatórios, `CGO_ENABLED=0`, `-trimpath` e injeção de `buildinfo`.
- [ ] Copiar `.golangci.yml` ajustando `goimports.local-prefixes` para o nome do novo módulo.
- [ ] Estruturar `cmd/<app>/main.go` como entrypoint fino com `signal.NotifyContext` e ownership exclusivo de `os.Exit`.
- [ ] Implementar roteador hierárquico com `Action[TArgs, TResult]` (`Parse -> Execute -> Format`), `Context` injetado e flags transversais.
- [ ] Definir TSV como saída padrão de listagens (sem cabeçalho para 0 linhas) + `-F table` opcional.
- [ ] Resolver caminhos de dados segundo XDG/macOS/Windows, com override via `--db` / `<APP>_DB` para testes.
- [ ] Se houver persistência SQLite, usar driver pure Go (`modernc.org/sqlite`), WAL, `foreign_keys=ON` e schema idempotente.
- [ ] Se houver anexos/arquivos, isolar em `internal/filestore` com hash de conteúdo, rollback compensatório, retenção segura e `prune --dry-run`.
- [ ] Se houver TUI, validar TTY, aplicar layout responsivo, paleta adaptativa (Light/Dark) e modais flutuantes ANSI-aware.
- [ ] Escrever testes isolados (`t.TempDir()`, `-race -count=1`) e validar com `make check`.
- [ ] Criar `README.md`, READMEs de pacote em `internal/`, `AGENTS.md` e `skill/<app>.md`.
