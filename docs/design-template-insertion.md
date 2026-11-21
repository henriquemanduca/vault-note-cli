# Design — Inserir template em nota existente (`note insert` / `Ctrl+T`)

Status: **aprovado pelo usuário** (append no fim, `Ctrl+T`+`T`, `--at N`, guarda de duplicidade). Não implementado. Depende de `docs/design-note-templates.md`
(templates em `<vault>/.templates/`, `RenderTemplate`, `Template`/`CRUD`), que já está implementado e não commitado.

## 1. Objetivo e escopo

Reaproveitar um template dentro de uma nota que já existe, sem recriar a nota:

```bash
# acrescenta o template ao final da nota
vault-note-cli note insert WORK/"Projeto X" -t checklist

# pré-visualiza o resultado sem gravar
vault-note-cli note insert WORK/"Projeto X" -t checklist --print
```

Na TUI: `Ctrl+T` no editor insere o template **na posição do cursor**; `T` na lista insere **no final** da nota selecionada.

Escopo v1:

- Uma operação de composição (posição) + o renderer já existente. Nenhuma engine nova.
- CLI com `--prepend`, `--at <N>`, `--print`, `--open`.
- TUI com seletor de template reutilizando o padrão de modal-lista (`modalNotebook`).
- Nenhuma dependência nova, sem mudança de schema/config, escrita atômica.

Fora de escopo: inserir múltiplos templates numa chamada, histórico/undo próprio (o undo é o `.git` do vault ou `Ctrl+Z` do editor externo), guarda de duplicidade automática, variáveis novas além das cinco atuais.

## 2. Contexto

- Repo: `/home/henrique/Projetos/vault-note-cli`, Go 1.27.4, branch `master`, árvore suja com a feature de templates (uncommitted).
- Já existe: `internal/vault/templates.go` (`ListTemplates`, `GetTemplate` com fallback case-insensitive, `SaveTemplate`, `DeleteTemplate`, `CreateNoteFromTemplate`, `RenderTemplate`, relógio injetável `v.now`); `internal/cmd/template.go` (5 actions); `note add -t/--template`; seletor de template no modal de nova nota.
- A TUI já tem `modalNewNote`, `modalEditMeta`, `modalNotebook`, `modalSettings` etc. em `internal/tui/modal.go`; teclas do editor (`internal/tui/update.go`, `updateEditor`): `Ctrl+C`, `Esc`/`Shift+Tab`, `Ctrl+S`, `Ctrl+E`. **`Ctrl+T` está livre.** Na lista, `T` (shift) está livre.
- `bubbles/textarea v1.0.0` oferece `InsertString(s)` (insere no cursor, respeita undo) e `LineInfo()` (`CharOffset` + `ColumnOffset` = offset em runas). Isso é o que a TUI deve usar.

## 3. Decisões

| Tema | Decisão | Motivo |
| :--- | :--- | :--- |
| Posição padrão na CLI | **append no final** | Caso de uso dominante: acrescentar um checklist/seção no fim. `--prepend` e `--at` cobrem o resto. |
| Offset da CLI | em **runas**, não bytes | Nunca quebra UTF-8 em `ç`/emoji; `textarea` também pensa em runas. |
| Inserção na TUI no cursor | `m.editor.InsertString(rendered)` | O textarea é dono do cursor/linha; reimplementar isso no domínio seria pior. O que precisa ser comum é o **render**, e é (`RenderTemplateFor`). |
| Render por nota | `v.RenderTemplateFor(note, tpl)` | Uma fonte única para CLI (append/prepend/at) e TUI (cursor). |
| Variáveis numa nota existente | `{{title}}` = título atual, `{{notebook}}`, `{{date}}`/`{{datetime}}` = hoje local, `{{template}}` | Mesmo contrato do `add -t`; sem surpresa. |
| Separador no append/prepend | uma linha em branco entre conteúdo e snippet | Leitura previsível; snippet em si é inserido **verbatim** (nunca reindentado). |
| Duplicidade | **sem guarda**: reinserir o mesmo template é sempre permitido | Sem validação, flag `--force` ou modal de confirmação; cada inserção acrescenta outra cópia. |
| `--print` | compõe e imprime, **não grava** | Automação e diff seguro, mesmo padrão de `view --raw`. |
| Falha de template/nota | `ErrNotFound` **antes** de qualquer escrita | Nunca deixa nota meio-modificada. |
| Alias | `ins`, `apply` → `note insert` | Curto para uso diário. |

## 4. Passos

### Passo 1 — `internal/vault/templates.go` (estender)

```go
type InsertMode int

const (
    InsertAppend  InsertMode = iota // fim do conteúdo (padrão)
    InsertPrepend                   // início do conteúdo
    InsertAtRune                    // offset em runas
)

type InsertOptions struct {
    Mode       InsertMode
    RuneOffset int // usado quando Mode == InsertAtRune
}

// InsertSnippet compõe content + snippet. Pura, sem I/O.
func InsertSnippet(content, snippet string, opts InsertOptions) string

// RenderTemplateFor renderiza tpl para a nota alvo (não grava).
func (v *Vault) RenderTemplateFor(note Note, templateName string) (string, error)

// InsertTemplate renderiza e grava a nota atomicamente.
func (v *Vault) InsertTemplate(noteID, templateName string, opts InsertOptions) (Note, error)
```

Regras de `InsertSnippet` (tudo em runas):

- `InsertAtRune`: clamp `RuneOffset` em `[0, len([]rune(content))]`; insere verbatim; se o resultado não terminar em `\n`, acrescenta um.
- `InsertAppend`: se `content != ""`: garante terminar em `\n` e acrescenta **mais um** `\n` (linha em branco); depois o snippet; garante `\n` final.
- `InsertPrepend`: snippet + (se `content != ""`) linha em branco + content; snippet normalizado para terminar em `\n`.
- Snippet vazio ou conteúdo vazio: sem separadores extras; conteúdo vazio + append = snippet normalizado.
- Nenhuma normalização de espaços internos, tabs ou CRLF dentro do snippet.

`RenderTemplateFor`: `GetTemplate` → `RenderTemplate(tpl.Body, vars)` com as cinco variáveis derivadas da nota e de `v.clock()`.

`InsertTemplate`: `v.GetNote(noteID)` → `RenderTemplateFor` → `InsertSnippet(...)` → `v.UpdateNoteContent(noteID, composed)` (escrita atômica) → retorna a `Note` recarregada. Reinserir o mesmo template é sempre permitido.


### Passo 2 — `internal/cmd/note.go`: ação `note insert`

```bash
vault-note-cli note insert <ref> -t <template> [--prepend] [--at <N>] [--print] [--open] [-b <caderno>]
```

- `noteInsertArgs{ref, template string, prepend bool, at int+set, print bool, open bool, notebook string}`.
- Flags: `-t/--template` (obrigatória; ausente → `UsageError`), `--prepend`, `--at` (`optionalIntFlag`: `set` + valor), `--print`, `--open`, `-b/--notebook` (via `resolveNotebookDefault`, igual às outras ações).
- Validações no `Parse`: `--at` junto com `--prepend` → `UsageError`; `--at` negativo → `UsageError`; posicional `<ref>` obrigatório.
- `--open` + `--print` → `UsageError` (dry-run não abre editor).
- `Execute`: `RequireVault` → `ResolveNote(ref, notebook)` (para obter o ID canônico) → `InsertTemplate(id, tpl, opts)`; com `--print`, usar `RenderTemplateFor` + `InsertSnippet` e **não** gravar (retornar struct com `printed=true` e o texto).
- `Format`: `Inserted template <template> into <noteID>`; com `--print`, imprime o conteúdo composto e uma linha em `stderr`: `(dry-run, nota não modificada)`.
- `--open`: `tea.ExecProcess` já é usado no TUI; na CLI, reutilizar o caminho de `note edit` (mesma função de abrir `$EDITOR` no arquivo) — sem duplicar `os/exec`.

### Passo 3 — Roteamento, aliases, help

- `internal/cmd/note.go` `dispatchNote`: case `"insert", "ins"`.
- `internal/cmd/alias.go`: `"insert": {"note", "insert"}`, `"ins": {"note", "insert"}`, `"apply": {"note", "insert"}`.
- `internal/cmd/help.go`: linha nova em `noteHelpText`; `rootHelpText` acrescenta `insert` à lista de ações de `note` e os aliases novos.

### Passo 4 — TUI: modal seletor + duas teclas

`internal/tui/modal.go`, `internal/tui/update.go`, `internal/tui/commands.go`, `internal/tui/view.go`:

- Novo `modalKind`: `modalInsertTemplate` (depois de `modalNotebook`).
- Estado: reaproveita `listCursor`; adiciona `templateNames []string` (já existe no `modalState` desde a feature anterior) e `targetNoteID string`.
- `openInsertTemplateModal(noteNoteID string)`: carrega `m.vault.ListTemplates()` (erro → lista vazia, modal abre com mensagem), `listCursor = 0`.
- Teclas no editor (`updateEditor`): `case "ctrl+t"` → se `loadedNoteID != ""` e `!dirty`? **Não** exige `!dirty`: insere no buffer atual e mantém `dirty` — o render usa o conteúdo da nota em disco para as variáveis, o texto vai para o buffer. Abre o modal.
- Teclas na lista (`updateList`): `case "T"` → `openInsertTemplateModal(selectedNoteID)`, marcando modo "append".
- `updateInsertTemplateModal`: `esc`/`q` fecha; `j/k/↓/↑` navega; `enter` insere imediatamente, sem confirmação de duplicidade;
  - modo cursor: `rendered, err := m.vault.RenderTemplateFor(note, tpl)` → `m.editor.InsertString(rendered)` → `m.dirty = m.editor.Value() != m.loadedContent` → status `Template <nome> inserido (Ctrl+S para salvar)`; foco continua no editor.
  - modo append (vindo da lista): `insertTemplateCmd(m.vault, noteID, tpl)` → nova msg `templateInsertedMsg{note, err}`; no `Update`, se a nota é a carregada, recarregar `m.editor.SetValue(note.Content)` e `m.loadedContent = note.Content`, `m.dirty = false`; selecionar a nota e status `Template <nome> acrescentado`.
- Render (`renderModal`): título `Inserir Template`, lista com `listCursor` destacado, dica `j/k: navegar  Enter: inserir  Esc: cancelar`; linha de preview com a primeira linha não vazia do template renderizado (truncada em `m.modalBoxWidth()`), em estilo `muted`. Lista vazia: `Nenhum template em <vault>/.templates/ (crie com 'vault-note-cli template add')`.
- Tabela de atalhos (`modalHelp`): `{"Ctrl+T (Editor)", "Inserir template na posição do cursor"}`, `{"T (Lista)", "Acrescentar template ao final da nota selecionada"}`.

### Passo 5 — Documentação (invariante 6 de `AGENTS.md`)

- `README.md`: seção "Inserir template em nota existente" (CLI + teclas da TUI + nota sobre `--print` e ausência de guarda de duplicidade); tabela de atalhos.
- `skill/vault-note-cli.md`: `note insert` e `Ctrl+T`/`T`.
- `AGENTS.md`: linha no mapa (`internal/vault`: composição de inserção) e gotcha: "inserção na TUI usa `textarea.InsertString`; o domínio só compõe — não reimplemente cursor no `vault`".

### Passo 6 — Testes

`internal/vault/templates_test.go` (ou novo `insert_test.go`):

- `InsertSnippet`: append em notícia vazia, append com/sem `\n` final, append preservando linha em branco única, prepend, `InsertAtRune` em 0 / meio / fim / offset maior que o conteúdo (clamp) / offset negativo (clamp em 0), snippet vazio, conteúdo só com `\n`.
- **UTF-8**: conteúdo `"ação çã"`, inset em cada offset de runa, resultado sempre `utf8.ValidString`.
- `InsertTemplate`: append/prepend/at; variáveis `{{title}}`/`{{notebook}}` da nota alvo; template inexistente → `ErrNotFound` e **conteúdo do arquivo inalterado**; nota inexistente → `ErrNotFound`; data determinística com `now` fixo.
- `InsertTemplate` reinserido → sempre sucede e grava duas cópias.

`internal/cmd/template_test.go` (ou `note_test.go`):

- `note insert X -t tpl` → grava e imprime `Inserted template tpl into X`; `view --raw` mostra ordem correta.
- `--prepend`, `--at 0`, `--at` além do tamanho (clamp, exit 0), `--at` negativo (exit 2), `--prepend --at 1` (exit 2), `-t` ausente (exit 2), `--print` (arquivo inalterado, exit 0), `--print --open` (exit 2).
- Reinserção: segundo `note insert` → exit 0 e grava; `--print` de duplicata → exit 0 sem aviso e arquivo inalterado.
- Alias `apply` e `ins` roteiam para `note insert`.
- Regressão: `note add`, `note update`, `template *` inalterados.

`internal/tui/modal_test.go` (com `Options{DisableCursorBlink: true}`):

- `Ctrl+T` no editor abre `modalInsertTemplate` com nomes carregados; `Enter` insere no cursor do textarea, `dirty == true`, sem gravar arquivo.
- `T` na lista acrescenta ao final do arquivo e recarrega o buffer limpo.
- `Esc` fecha sem mudar conteúdo; lista vazia mostra mensagem.
- Reinserção na TUI: nota que já tem a primeira linha do template → `Enter` insere imediatamente (buffer sujo) / grava no modo append.

## 5. Critérios de pronto e verificação

```bash
cd /home/henrique/Projetos/vault-note-cli
make check      # fmt + vet + go test -race -count=1 ./...
make build      # CGO_ENABLED=0
```

Smoke (vault e config descartáveis; títulos sem `/`, que `validateName` rejeita):

```bash
export VAULT_NOTE_CLI_CONFIG=/tmp/vnc-insert.yaml
V=/tmp/vnc-insert-vault; rm -rf "$V" "$VAULT_NOTE_CLI_CONFIG"
C="bin/vault-note-cli --config $VAULT_NOTE_CLI_CONFIG"

$C config set vault_path "$V"
printf '== {{title}} — checklist ==\n- [ ] item 1\n- [ ] item 2\n' | $C template add checklist --stdin
$C notebook add WORK
$C add "Projeto X" "Corpo existente" -b WORK

$C note insert "WORK/Projeto X" -t checklist --print     # dry-run: corpo + branco + checklist
$C note insert "WORK/Projeto X" -t checklist              # grava
$C note insert "WORK/Projeto X" -t checklist --prepend # checklist no topo
$C view "WORK/Projeto X" --raw
$C note insert "WORK/Projeto X" -t inexistente; echo $?   # exit 1, arquivo intacto
$C note insert "WORK/Projeto X" -t checklist; echo $?      # exit 0: reinserção permitida
$C note insert "WORK/Projeto X" -t checklist               # grava a terceira cópia
```

Esperado: `--print` não altera o arquivo; após o append e o `--prepend` o `.txt` tem o corpo original entre duas cópias do checklist, com uma linha em branco separando; `view --raw` mostra `== Projeto X — checklist ==` nos dois pontos; template inexistente sai com `error: record not found: template "inexistente"` e exit 1; a repetição é sempre permitida e grava outra cópia.

TUI, manual: abrir a nota, posicionar o cursor no meio, `Ctrl+T`, escolher, `Enter` → texto entra no cursor, status pede `Ctrl+S`; `T` na lista → texto vai para o fim e o buffer recarrega sem `*` de alteração pendente.

## 6. Esforço

~650 linhas (vault ~90 + testes 150; cmd ~140 + testes 120; TUI ~180 + testes 90; docs). Uma PR, sem dependências novas.

## 7. Decisões do usuário (confirmadas — implementar exatamente assim)

1. **Posição padrão na CLI**: append no final. `--prepend` e `--at N` continuam disponíveis.
2. **Teclas da TUI**: `Ctrl+T` no editor (insere no cursor) e `T` na lista (insere no fim).
3. **`--at <N>`**: mantido, offset em runas.
4. **Duplicidade**: sem guarda — reinserir o mesmo template é sempre permitido, sem flag `--force` e sem modal de confirmação.
