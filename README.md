# vault-note-cli

Aplicativo de **anotações rápidas em arquivos `.txt`** para terminal escrito em Go, distribuído como **binário único estático (`CGO_ENABLED=0`)**, com **vault em sistema de arquivos** (pastas = cadernos, `.txt` = notas), **SQLite embarcado** exclusivo para configurações do app (`modernc.org/sqlite`) e interface dupla (**TUI interativa** + **CLI scriptável**).

## Destaques

- **Vault em arquivos `.txt` puros:** cada pasta de 1º nível dentro do vault é um caderno (`notebook`) e cada arquivo `.txt` é uma nota — 100% portável, legível por qualquer editor e pronto para Git/Syncthing. Arquivos `.txt` soltos na raiz do vault são tratados como notas sem caderno.
- **Lixeira interna segura (`.trash/`):** excluir uma nota ou caderno move os arquivos para `<vault>/.trash/<timestamp>/...`, evitando perda acidental.
- **SQLite exclusivo para configurações:** o caminho do vault (`vault_path`) e futuras preferências ficam persistidos em SQLite, gerenciáveis pela CLI (`config`) e pelo **modal flutuante de Configurações** na TUI (aberto automaticamente na primeira execução ou via `c`).
- **Layout TUI Mestre-Detalhe (20% / 80%):** lista de notas à esquerda (20%) e editor TUI (`textarea`) à direita (80%) pronto para digitação direta ao focar com `Tab`/`Enter`, além de abertura no editor externo (`$EDITOR` / `$VISUAL`) via `E` ou `Ctrl+E`.
- **Tema Adaptativo Graphite:** alto contraste em terminais claros e escuros (`lipgloss.AdaptiveColor`) e modais flutuantes ANSI-aware.

## Instalação e Build

```bash
# Compilar binário estático em ./bin/vault-note-cli
make build

# Instalar em ~/.local/bin (ou PREFIX=/usr/local)
make install

# Instalar skill para agentes de IA
make install-skill
```

## Diretórios de Dados e Configurações

- **Linux/BSD:** `$XDG_DATA_HOME/vault-note-cli` (fallback `~/.local/share/vault-note-cli`)
- **macOS:** `~/Library/Application Support/vault-note-cli`
- **Windows:** `%LOCALAPPDATA%\vault-note-cli`
- **Banco SQLite de configurações:** `<data_root>/vault-note-cli.db`
- **Vault de notas (configurável):** definido em `vault_path` no SQLite (via modal de Configurações na TUI ou `vault-note-cli config set vault_path <dir>`).

### Estrutura do Vault

```text
<vault_path>/
├── solta.txt              # Nota sem caderno (raiz do vault)
├── WORK/                  # Caderno (pasta de 1º nível)
│   └── reuniao.txt        # Nota dentro de WORK (ID: WORK/reuniao.txt)
├── PESSOAL/               # Caderno
│   └── compras.txt        # Nota dentro de PESSOAL (ID: PESSOAL/compras.txt)
└── .trash/                # Lixeira interna (ignorada nas listagens)
    └── 20261120T210000Z/
        └── WORK/antiga.txt
```

## Uso da TUI

Execute sem argumentos em um terminal interativo (ou `vault-note-cli tui` / `vault-note-cli ui`):

```bash
vault-note-cli
vault-note-cli tui -b WORK
```

Na primeira execução (sem `vault_path` salvo no SQLite), o **modal de Configurações** abre automaticamente solicitando o caminho do vault (criando a pasta caso não exista).

### Atalhos Principais

| Contexto | Tecla | Ação |
| :--- | :--- | :--- |
| **Lista (20% esquerda)** | `j`/`k`, `↑`/`↓`, `g`/`G` | Navega entre notas `.txt` |
| | `Enter` / `Tab` / `l` | Foca o editor TUI à direita pronto para digitar |
| | `n` | Nova nota `.txt` (foca o editor automaticamente) |
| | `m` | Renomear arquivo `.txt` ou mover entre pastas/cadernos |
| | `E` | Abrir nota no editor externo (`$EDITOR`) |
| | `d` | Mover nota para `<vault>/.trash/` (com confirmação) |
| | `/` | Filtrar notas em tempo real (título, pasta e conteúdo) |
| | `b` | Modal de Cadernos/Pastas (filtrar, `n` criar, `u` renomear, `d` lixeira) |
| | `c` / `,` | Modal de Configurações (alterar caminho do vault salvo no SQLite) |
| | `r` / `?` / `q` | Recarregar vault / Ajuda / Salvar e sair |
| **Editor (80% direita)** | Digitação livre | Edita o texto diretamente na TUI |
| | `Ctrl+S` | Salva imediatamente no arquivo `.txt` |
| | `Ctrl+E` | Abre o buffer atual no `$EDITOR` |
| | `Esc` / `Shift+Tab` | Salva alterações pendentes e retorna à lista |

## Uso da CLI

```bash
# Configurações (SQLite)
vault-note-cli config set vault_path ~/meu-vault
vault-note-cli config get
vault-note-cli config path
vault-note-cli config open

# Cadernos (pastas de 1º nível)
vault-note-cli notebook add WORK                     # alias: nn
vault-note-cli notebook list                         # aliases: ln, nbs
vault-note-cli notebook view WORK -t
vault-note-cli notebook rename WORK PROJETOS
vault-note-cli notebook delete PROJETOS --force      # move pasta para .trash/

# Notas (arquivos .txt)
vault-note-cli add "Ideias" "Layout 20/80" -b WORK
echo "Texto via pipe" | vault-note-cli add "Pipeline" -b WORK --stdin
vault-note-cli list -b WORK
vault-note-cli list -F table --timestamps
vault-note-cli view WORK/Ideias
vault-note-cli view WORK/Ideias --raw
vault-note-cli update WORK/Ideias "Novas Ideias" -b none
vault-note-cli edit "Novas Ideias"                   # abre .txt direto no $EDITOR
vault-note-cli search "Layout"
vault-note-cli delete "Novas Ideias"                 # move .txt para .trash/
```

## Configuração de Workspace (`.vaultnotecli`)

Crie um arquivo `.vaultnotecli` no diretório do projeto:

```bash
# .vaultnotecli
NOTEBOOK=WORK
# VAULT_PATH=~/meu-vault
```

- Descoberto automaticamente subindo até `$HOME` (arquivo mais próximo vence; somente leitura).
- Precedência de caderno: flag `-b` > `-b ""` (escape hatch) > `.vaultnotecli` > global.
- Precedência de vault: `--vault` > `VAULT_NOTE_CLI_VAULT` > `.vaultnotecli` (`VAULT_PATH`) > SQLite (`vault_path`).

## Qualidade e Testes

```bash
make check          # gofmt + go vet + go test -race -count=1 ./...
make build-all      # cross-compilation Linux/macOS/Windows
make checksums      # gera dist/checksums.txt
```
