package cmd

import (
	"fmt"
	"io"
)

func printHelp(w io.Writer, topic []string) error {
	if len(topic) > 0 {
		switch topic[0] {
		case "note":
			_, err := fmt.Fprint(w, noteHelpText)
			return err
		case "notebook", "nb":
			_, err := fmt.Fprint(w, notebookHelpText)
			return err
		case "config", "cfg":
			_, err := fmt.Fprint(w, configHelpText)
			return err
		}
	}
	_, err := fmt.Fprint(w, rootHelpText)
	return err
}

const rootHelpText = `vault-note-cli — Anotações em arquivos .txt organizados em pastas (TUI + CLI)

Uso:
  vault-note-cli                          Abre a TUI interativa (20% lista / 80% editor)
  vault-note-cli <recurso> <ação> [flags] Executa comando CLI scriptável

Recursos e Comandos:
  tui, ui [-b <caderno>]                  Abre a interface interativa (abre Configurações no 1º uso)
  note <ação>                             Gerencia notas .txt (add, list, view, update, edit, delete, search)
  notebook <ação>                         Gerencia cadernos/pastas (add, list, view, rename, delete)
  config <ação>                           Gerencia configurações salvas no SQLite (get, set, path, open)
  version                                 Exibe versão e metadados de build
  help [tópico]                           Exibe ajuda geral ou por recurso

Aliases Planos Frequentes:
  add (new, n), list (ls), view (show, get), update (mod, mv), edit,
  delete (del, rm), search (find), nb, nn, ln (nbs), cfg, ui

Flags Globais:
  -F, --format <tsv|table>                Formato de listagens (padrão: tsv)
      --vault <diretório>                 Sobrescreve temporariamente o caminho do vault
  -h, --help                              Exibe ajuda contextual

Workspace (.vaultnotecli):
  Coloque NOTEBOOK=WORK (e opcionalmente VAULT_PATH=~/meu-vault) em um arquivo
  .vaultnotecli no diretório do projeto (use -b "" para ignorar em uma execução).
`

const noteHelpText = `Uso: vault-note-cli note <ação> [argumentos]

Ações:
  add <título> [conteúdo] [-b <caderno>] [-c <conteúdo>] [--stdin]
  list [-b <caderno>] [-q <busca>] [-n] [--timestamps]
  view <ref> [-b <caderno>] [--raw] [-t]
  update <ref> [novo-título] [-c <conteúdo>] [-b <caderno|none>] [--stdin]
  edit <ref> [-b <caderno>]               Abre o arquivo .txt diretamente no $EDITOR
  delete <ref> [-b <caderno>]             Move o arquivo .txt para <vault>/.trash/
  search <termo> [-b <caderno>] [-n] [--timestamps]

Referência (<ref>):
  Caminho relativo (ex.: WORK/reuniao.txt ou WORK/reuniao) ou nome da nota
  quando não houver ambiguidade (ou combinado com -b <caderno>).
`

const notebookHelpText = `Uso: vault-note-cli notebook <ação> [argumentos]

Ações:
  add <nome>                              Cria pasta de caderno no vault
  list [-t] [-n]                          Lista pastas do vault com contagem de arquivos .txt
  view <nome> [-t]                        Exibe detalhes da pasta do caderno
  rename <atual> <novo>                   Renomeia a pasta do caderno
  delete <nome> [--force]                 Move a pasta para <vault>/.trash/ (--force se não vazia)
`

const configHelpText = `Uso: vault-note-cli config <ação> [argumentos]

Ações:
  get [chave] [-n]                        Lista configurações salvas no SQLite ou exibe uma chave
  set vault_path <diretório>              Define o caminho do vault (cria a pasta se não existir)
  path                                    Imprime o caminho ativo do vault
  open                                    Abre a pasta do vault no gerenciador de arquivos do SO
`
