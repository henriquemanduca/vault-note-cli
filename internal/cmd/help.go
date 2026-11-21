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
		case "template", "tpl":
			_, err := fmt.Fprint(w, templateHelpText)
			return err
		case "trash", "tr":
			_, err := fmt.Fprint(w, trashHelpText)
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
  note <ação>                             Gerencia notas .txt (add, list, view, update, insert, edit, delete, search)
  notebook <ação>                         Gerencia cadernos/pastas (add, list, view, rename, delete)
  template <ação>                         Gerencia templates de nota em <vault>/.templates/ (add, list, view, update, delete, init)
  trash <ação>                            Gerencia a lixeira interna <vault>/.trash/ (list, empty)
  config <ação>                           Gerencia configurações salvas em config.yaml (get, set, path, open)
  version                                 Exibe versão e metadados de build
  help [tópico]                           Exibe ajuda geral ou por recurso

Aliases Planos Frequentes:
  add (new, n), list (ls), view (show, get), update (mod, mv), insert (ins, apply), edit,
  delete (del, rm), search (find), nb, nn, ln (nbs), cfg, ui,
  tpl, tpls (lt), tpl-add, tr, empty-trash

Flags Globais:
  -F, --format <tsv|table>                Formato de listagens (padrão: tsv)
      --vault <diretório>                 Sobrescreve temporariamente o caminho do vault
      --config <arquivo.yaml>             Usa um arquivo YAML de configuração específico
  -h, --help                              Exibe ajuda contextual

Workspace (.vaultnotecli):
  Coloque NOTEBOOK=WORK (e opcionalmente VAULT_PATH=~/meu-vault) em um arquivo
  .vaultnotecli no diretório do projeto (use -b "" para ignorar em uma execução).
`

const noteHelpText = `Uso: vault-note-cli note <ação> [argumentos]

Ações:
  add <título> [conteúdo] [-b <caderno>] [-c <conteúdo>] [--stdin] [-t <template>] [--open]
  list [-b <caderno>] [-q <busca>] [-n] [--timestamps]
  view <ref> [-b <caderno>] [--raw] [-t]
  update <ref> [novo-título] [-c <conteúdo>] [-b <caderno|none>] [--stdin]
  insert <ref> -t <template> [--prepend|--at <N>] [--print] [--open] [-b <caderno>]
                                          Insere um template na nota (padrão: fim)
  edit <ref> [-b <caderno>]               Abre o arquivo .txt diretamente no $EDITOR
  delete <ref> [-b <caderno>]             Move o arquivo .txt para <vault>/.trash/
  search <termo> [-b <caderno>] [-n] [--timestamps]

Referência (<ref>):
  Caminho relativo (ex.: WORK/reuniao.txt ou WORK/reuniao) ou nome da nota
  quando não houver ambiguidade (ou combinado com -b <caderno>).
`

const notebookHelpText = `Uso: vault-note-cli notebook <ação> [argumentos]

Ações:
  add <nome>                              Cria pasta de caderno no vault (palavra única, convertido para MAIÚSCULAS)
  list [-t] [-n]                          Lista pastas do vault com contagem de arquivos .txt
  view <nome> [-t]                        Exibe detalhes da pasta do caderno
  rename <atual> <novo>                   Renomeia a pasta do caderno (palavra única, convertido para MAIÚSCULAS)
  delete <nome> [--force]                 Move a pasta para <vault>/.trash/ (--force se não vazia)
`

const templateHelpText = `Uso: vault-note-cli template <ação> [argumentos]

Ações:
  add <nome> [corpo] [--stdin] [-e] [--from <nota>] [-b <caderno>] [-f] [--open]
                                          Cria ou sobrescreve um template (--from copia o corpo de uma nota)
  list [-n]                              Lista templates com nome e caminho
  view <nome> [--raw]                    Exibe o template (--raw: só o corpo)
  update <nome> [corpo] [--stdin] [-e]   Atualiza o corpo de um template existente
  delete <nome>                          Move o template para <vault>/.trash/
  init                                   Cria templates padrão ausentes sem sobrescrever existentes

Templates:
  Arquivos .txt em <vault>/.templates/ (pasta oculta, nunca vira caderno).
  Placeholders: {{title}} {{notebook}} {{date}} {{datetime}} {{template}}.
  {{date}} usa a data local (2006-01-02); {{datetime}} usa RFC3339 local.
  Placeholders desconhecidos permanecem literais; não há escape.
  Templates padrão: checklist, daily, decision, meeting (criados no 1º setup ou via init).
  --from copia o conteúdo da nota sem alterá-la; use -f para sobrescrever um template existente.

Exemplo:
  vault-note-cli template add modelo --from "Reuniao 12-05" -b WORK
  vault-note-cli add "Reunião 12/05" -b WORK -t reuniao
`

const configHelpText = `Uso: vault-note-cli config <ação> [argumentos]

Ações:
  get [chave] [-n]                        Lista configurações salvas em config.yaml ou exibe uma chave
  set <chave> <valor>                     Define uma configuração (vault_path, show_notebook_prefix, show_line_numbers)
  set vault_path <diretório>              Define o caminho do vault (cria a pasta e os templates padrão no 1º setup)
  set show_notebook_prefix <bool>         Exibe prefixo de até 3 letras do caderno na lista da TUI (true/false)
  set show_line_numbers <bool>            Exibe números de linha no editor da TUI (true/false, padrão: false)
  path                                    Imprime o caminho ativo do vault
  open                                    Abre a pasta do vault no gerenciador de arquivos do SO
`

const trashHelpText = `Uso: vault-note-cli trash <ação> [argumentos]

Ações:
  list [-t] [-n]                          Lista itens atualmente em <vault>/.trash/ (aliases: ls)
  empty --force                           Apaga permanentemente todos os itens de <vault>/.trash/ (aliases: clean, clear, purge; -f)

Atenção:
  'trash empty' exige --force (-f) quando há itens na lixeira para evitar perda acidental de dados.
  Atalho plano: 'vault-note-cli empty-trash --force' (ou 'vault-note-cli tr list').
`
