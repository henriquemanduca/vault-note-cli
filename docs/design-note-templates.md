# Design — Templates de Nota (`note templates`)

Status: **implementado** (branch `master`, base `b75260e v2`, ainda não commitado). `make check` e `make build` verdes.

## 1. Objetivo e escopo

Criar uma nota já preenchida a partir de um esqueleto reutilizável:

```bash
vault-note-cli add "Reunião 12/05" -b WORK -t reuniao
```

Escopo (v1):

- Templates são arquivos `.txt` puros em `<vault>/.templates/<nome>.txt`.
- Substituição de 5 placeholders (`{{title}}`, `{{notebook}}`, `{{date}}`, `{{datetime}}`, `{{template}}`).
- CRUD de templates só pela CLI (TUI apenas consome o template no modal de nova nota).
- Nenhuma dependência nova (somente stdlib), nenhuma mudança em `config.yaml`, `.vaultnotecli` ou schema.

Fora de escopo: engine de template (`text/template`), condicionais, includes, versionamento de templates, CRUD de templates na TUI.

## 2. Contexto do repositório

- Caminho: `/home/henrique/Projetos/vault-note-cli`
- Stack: Go 1.27.4, módulo `github.com/hmanduca/vault-note-cli`; Bubble Tea/Bubbles/Lipgloss na TUI; `gopkg.in/yaml.v3` só nas settings.
- Estado: implementação existente e testada (`internal/vault`, `internal/cmd`, `internal/tui`, `internal/settings`, `internal/config`, `internal/filestore`). Sem nenhuma referência a "template" no código hoje — esta feature começa do zero.
- Contrato operacional: `AGENTS.md` (invariantes abaixo) e `docs/architecture-principles.md`.

Invariantes que a implementação precisa respeitar:

1. Zero CGO (`CGO_ENABLED=0`).
2. Testes só com `t.TempDir()` / `t.Setenv` / configs temporárias; nunca tocar `~/.config/vault-note-cli`.
3. `writeTabular` não imprime cabeçalho com 0 registros.
4. Nomes validados contra path traversal; escrita atômica; exclusão vai para `.trash/`.
5. `version`/`help` não leem `config.yaml`.
6. Ao adicionar comandos/atalhos, atualizar juntos: `internal/cmd/router.go`, `internal/cmd/alias.go`, `internal/cmd/help.go`, `README.md`, `skill/vault-note-cli.md`.
7. Testes de TUI com `Options{DisableCursorBlink: true}`.

## 3. Decisões tomadas

| Tema | Decisão | Motivo |
| :--- | :--- | :--- |
| Local dos templates | `<vault>/.templates/*.txt` | Portátil com Git/Syncthing; diretórios com prefixo `.` já são ignorados por `ListNotebooks` e pela lixeira interna; precedente `.trash/`. |
| Config extra | nenhuma | Menos superfície; nenhuma migração em `settings`. |
| Engine | scan manual de `{{ nome }}` | `text/template` expõe pipelines e vaza erros internos; 5 variáveis não justificam engine. |
| Placeholder desconhecido | permanece literal | Erro de digitação aparece na nota; nada é apagado silenciosamente e a criação nunca falha por isso. |
| Escapes | não existem | `{{...}}` é sempre substituído; documentado no README. |
| `{{date}}` | hora **local** (`2006-01-02`) | É o que o usuário espera numa nota diária; divergência consciente do UTC RFC3339 usado em `UpdatedAt`, documentada. |
| `{{datetime}}` | RFC3339 local | idem. |
| Conteúdo + template | conflito é erro de uso | `--template` com `-c`, corpo posicional ou `--stdin` → `UsageError` (exit 2). Uma única fonte de conteúdo por chamada. |
| `template delete` | vai direto para `.trash/`; `--force` aceito como no-op | Template é arquivo único: nunca há conteúdo a proteger. O flag fica só por simetria com `notebook delete`. |
| `add --open` | incluído no v1 (com e sem `-t`) | Abrir no `$EDITOR` logo após criar é o fluxo natural de quem usa template. |
| `GetTemplate` | fallback case-insensitive | Espelha `GetNotebook`/`findNoteInDir`: `-t Reuniao` encontra `reuniao.txt`. |
| Saída do `add` | inalterada: `Created note <ID>` | Não quebrar scripts nem o `skill/`. |
| CRUD na TUI | fora do escopo v1 | Templates são `.txt`; `$EDITOR` já resolve edição. |

## 4. Passos de implementação

### Passo 1 — `internal/vault/templates.go` (novo)

```go
const TemplatesDirName = ".templates" // oculto, fora das listagens e nunca é caderno

type Template struct {
    Name string // "reuniao" (sem .txt)
    Path string // <vault>/.templates/reuniao.txt
    Body string
}

func (v *Vault) ListTemplates() ([]Template, error)          // ordenado por Name, case-insensitive
func (v *Vault) GetTemplate(name string) (Template, error)   // ErrNotFound quando ausente
func (v *Vault) SaveTemplate(name, body string) (Template, error) // cria/sobrescreve, atômico
func (v *Vault) DeleteTemplate(name string) (TrashResult, error)  // move para .trash
func (v *Vault) CreateNoteFromTemplate(notebook, title, templateName string) (Note, error)
func RenderTemplate(body string, vars map[string]string) string    // pura, sem I/O
```

Detalhes obrigatórios:

- Resolver nome com `normalizeTemplateName`: reutiliza `validateName` (mesmas regras de `normalizeNoteTitle`: sem `..`, sem `/` nem `\`, sem `.` inicial, sem caracteres inválidos) e remove sufixo `.txt` se presente (case-insensitive).
- `ListTemplates`: `os.ReadDir(<root>/.templates)`; ausência do diretório → lista vazia e `nil` (não é erro); ignora subdiretórios e arquivos que não terminam em `.txt` e nomes com prefixo `.`.
- `SaveTemplate`: `os.MkdirAll(dir, 0o755)` + `writeFileAtomic` (helper existente em `vault.go`).
- `DeleteTemplate`: `moveToTrash(filepath.ToSlash(TemplatesDirName+"/"+name+NoteExt), path)` → resultado em `.trash/<ts>/.templates/<nome>.txt`.
- Nenhum template novo pode escapar do root: `ensureInsideRoot`.
- `CreateNoteFromTemplate`: ordem **`GetTemplate` → render → `CreateNote`**. Template ausente deve retornar `ErrNotFound` **antes** de qualquer escrita (nunca deixar nota meio-criada).

Relógio injetável para `{{date}}`/`{{datetime}}` sem mexer na API pública:

```go
type Vault struct {
    root string
    now  func() time.Time // nil => time.Now(); inicializado em Open()
}

func (v *Vault) clock() time.Time {
    if v.now == nil {
        return time.Now()
    }
    return v.now()
}
```

Testes (mesmo pacote `vault`) montam `&Vault{root: dir, now: func() time.Time { return fixed }}`. `Open()` passa a preencher `now: time.Now`.

### Passo 2 — `RenderTemplate` (no mesmo arquivo, ~20 linhas)

- Percorre `body` buscando `{{`, depois `}}`; faz `strings.TrimSpace` no miolo; se a chave existe em `vars`, substitui; senão mantém o trecho original (incluindo espaços).
- `{{ }}` vazio ou `{{` sem fechamento: mantém literal.
- Sem regex, sem recursão (valor substituído não é re-escaneado), sem alocação além do builder.

Variáveis montadas por `CreateNoteFromTemplate`:

```go
vars := map[string]string{
    "title":     title,
    "notebook":  notebook,             // "" para nota na raiz
    "date":      v.clock().Format("2006-01-02"),
    "datetime":  v.clock().Format(time.RFC3339),
    "template":  tpl.Name,
}
```

### Passo 3 — `internal/cmd/template.go` (novo)

Cinco `Action[TArgs, TResult]`, no padrão de `internal/cmd/note.go` (`Parse` → `Execute` → `Format`, `splitFlagsAndPosArgs`, `flag.ContinueOnError` + `io.Discard`):

| Ação | Assinatura | Saída |
| :--- | :--- | :--- |
| `template list` | `[-n]` (sem cabeçalho) | TSV/tabela via `writeTabular`: `name`, `path` |
| `template view` | `<nome> [--raw]` | `--raw` imprime o corpo; sem `--raw`, `Name:`/`Path:` + corpo |
| `template add` | `<nome> [corpo] / --stdin / -e,--edit` | `Created template <nome>` |
| `template update` | `<nome> [corpo] / --stdin / -e,--edit` | `Updated template <nome>` |
| `template delete` | `<nome> [--force]` | `Moved template <nome> to <trashPath>` |

- `--force` existe só por consistência com `notebook delete`; `DeleteTemplate` vai sempre para `.trash/` e o flag é aceito como no-op (documentado no help).
- `template delete` não pede confirmação: a exclusão é reversível via `.trash/`.
- `--edit`/`-e`: escreve `tpl.Body` em arquivo temporário dentro de `filestore.ConfigDir()` (nunca no vault), executa `$EDITOR`/`$VISUAL` via `os/exec` usando `ctx.Ctx`, e salva o resultado com `SaveTemplate`.
- Corpo vazio e sem `--stdin`/`--edit` → `UsageError` com a linha de uso.

### Passo 4 — `note add` aceita `-t/--template`

Em `internal/cmd/note.go` (`noteAddAction`):

```go
type noteAddArgs struct {
    title    string
    content  string
    notebook string
    template string // novo
}
```

- Flag registrada: `fs.Var(&tpl, "t", ...)` e `fs.Var(&tpl, "template", ...)` (usar `optionalStringFlag` para distinguir ausente de `""`; `-t ""` também é `UsageError`).
- Validação no `Parse`: se `tpl.set` e (`contentFlag != ""` ou `len(posArgs) >= 2` ou `fromStdin`) → `UsageError("flag --template cannot be combined with inline content, --content or --stdin")`.
- `Execute`: se `in.template != ""` → `v.CreateNoteFromTemplate(in.notebook, in.title, in.template)`, senão o caminho atual `v.CreateNote(...)` **sem alteração**.
- `Format`: inalterado (`Created note %s\n`).
- `help note` (em `help.go`) ganha `[-t <template>]` na linha do `add`.

### Passo 5 — Roteamento, aliases, ajuda

- `internal/cmd/router.go`: case `"template": return dispatchTemplate(c, expanded[1:])`; `dispatchTemplate` faz o switch das 5 ações (espelhar `dispatchNotebook`).
- `internal/cmd/alias.go`: `"template": {"template"}`, `"tpl": {"template"}`, `"tpls": {"template", "list"}`, `"lt": {"template", "list"}`, `"tpl-add": {"template", "add"}`.
- `internal/cmd/help.go`: nova constante `templateHelpText` + case `"template", "tpl"` em `printHelp`; `rootHelpText` ganha a linha de `template <ação>` e os aliases novos.

### Passo 6 — TUI: seletor de template no modal de nova nota

Arquivos: `internal/tui/modal.go`, `internal/tui/commands.go`.

- `modalState` ganha `templateNames []string` e `templateIdx int` (`0 = (sem template)`).
- `openNewNoteModal`: carrega `m.vault.ListTemplates()` (erro → lista vazia, modal abre normalmente) e guarda só os nomes; `modalEditMeta` **não** usa template.
- `updateNoteFormModal`: `totalFields` 2 → 3 (`0=Título`, `1=Caderno`, `2=Template`); navegação `h/l/space` no campo 2 cicla `templateIdx` sobre `len(templateNames)+1`; `enter` resolve `tplName` e chama `createNoteCmd(m.vault, nbName, title, tplName)` (só em `modalNewNote`).
- Renderização do campo (`modal.go`, por volta da linha 486, junto de `renderModalFieldLabel("Caderno (pasta)")`): rótulo `Template`, valor `(sem template)` quando `templateIdx == 0`.
- `createNoteCmd(v *vault.Vault, notebook, title, tpl string) tea.Cmd`: template vazio → `v.CreateNote`, senão `v.CreateNoteFromTemplate`. Mensagem `noteCreatedMsg` e tratamento de erro existentes não mudam (erro mantém modal aberto via `errText`).
- Atualizar a tabela de atalhos no modal de ajuda (linha do `n`): mencionar "n = nova nota (com template opcional)".

### Passo 7 — Documentação (mesmo PR, invariante 6)

- `README.md`: seção "Templates de Nota" (estrutura do vault com `.templates/`, tabela de placeholders, exemplos CLI, aviso de `{{date}}` local e ausência de escape); tabela de atalhos da TUI com o seletor de template.
- `skill/vault-note-cli.md`: comandos `template *` e `add -t`.
- `AGENTS.md`: linha do `internal/vault` no mapa da arquitetura (templates + relógio injetável); gotcha novo: "templates vivem em `<vault>/.templates/`, oculto de propósito — não transforme em caderno nem em settings".

### Passo 8 — Testes

`internal/vault/templates_test.go`:

- `ListTemplates` ordenado, vazio sem diretório, ignora não-`.txt` e subpastas.
- `SaveTemplate` sobrescreve atomicamente; `GetTemplate` ausente → `ErrNotFound`.
- `normalizeTemplateName` rejeita `../x`, `.hidden`, `a/b`, `x:y`; aceita `reuniao.txt` → `reuniao`.
- `RenderTemplate`: todas as variáveis, `{{ title }}` com espaços, placeholder desconhecido literal, `{{` sem fechamento literal, sem recursão (`{{title}}` cujo valor contenha `{{date}}` não expande).
- `CreateNoteFromTemplate` com relógio fixo injetado (`now`), `{{title}}`/`{{date}}` corretos, nota criada no caderno e na raiz.
- **Template ausente → `ErrNotFound` e `<vault>` sem arquivo novo** (contagem de `.txt` inalterada).
- `DeleteTemplate` termina em `.trash/<ts>/.templates/<nome>.txt`.

`internal/cmd/cmd_test.go` (ou novo `template_test.go`):

- `template list` TSV com cabeçalho e, com 0 templates, **sem** cabeçalho.
- `template list -F table`.
- `add X -t tpl` cria conteúdo renderizado; `add X -t tpl -c "y"` → exit 2 (`UsageError`).
- `add X` sem `--template` mantém comportamento atual (regressão).
- `template add` via `--stdin`, `template view --raw`, `template delete`.

`internal/tui/modal_test.go`:

- `openNewNoteModal` popula `templateNames`; ciclar até o template e `enter` cria nota com o corpo renderizado; `Options{DisableCursorBlink: true}`.

## 5. Interfaces, dependências e estilo

- Dependências novas: nenhuma. `os/exec` e `time` da stdlib.
- Fronteiras: `vault` não importa `cmd` nem `tui`; `cmd` e `tui` só usam a API acima.
- Estilo: código/comentários em inglês, strings de UI/help em pt-BR (padrão do repo); `gofmt` + `go vet` limpos; erros com `fmt.Errorf("%w: ...", ErrXxx)`.
- Segurança: mesma validação de `validateName`, `ensureInsideRoot`, escrita atômica, lixeira interna; nenhum caminho absoluto ou `..` aceito como nome de template; `--edit` usa arquivo temporário fora do vault.

## 6. Critérios de pronto e verificação

```bash
cd /home/henrique/Projetos/vault-note-cli
make check        # gofmt + go vet + go test -race -count=1 ./...  (deve passar)
make build        # CGO_ENABLED=0 → bin/vault-note-cli
```

Smoke test manual com vault e config descartáveis:

```bash
export VAULT_NOTE_CLI_CONFIG=/tmp/vnc-check.yaml
V=/tmp/vnc-vault; rm -rf "$V"
bin/vault-note-cli --config "$VAULT_NOTE_CLI_CONFIG" config set vault_path "$V"
printf 'Nota: {{title}} ({{template}})\nData: {{date}}\n' \
  | bin/vault-note-cli --config "$VAULT_NOTE_CLI_CONFIG" template add reuniao --stdin
bin/vault-note-cli --config "$VAULT_NOTE_CLI_CONFIG" template list
bin/vault-note-cli --config "$VAULT_NOTE_CLI_CONFIG" notebook add WORK   # -b exige caderno existente
bin/vault-note-cli --config "$VAULT_NOTE_CLI_CONFIG" add "Reuniao 12-05" -b WORK -t reuniao
bin/vault-note-cli --config "$VAULT_NOTE_CLI_CONFIG" view "WORK/Reuniao 12-05" --raw
bin/vault-note-cli --config "$VAULT_NOTE_CLI_CONFIG" add Dup -t inexistente   # erro, exit 1
```

Esperado: `template list` mostra `reuniao`; `view --raw` imprime `Nota: Reuniao 12-05 (reuniao)` e `Data: <data local de hoje>`; template inexistente sai com erro e **não** cria nota; `.templates/` não aparece em `notebook list`.

Atenção ao título usado no smoke: `validateName` rejeita `/` (e `\`, `..`, `.` inicial, `<>:"|?*`), então `"Reuniao 12/05"` falha com `note title cannot contain path separators`. Use `-` ou `.` como separador de data.

Também aceitável como evidência: `git diff --stat` ~600–700 linhas adicionadas, sem alteração de `go.mod`/`go.sum`.

## 7. Decisões que ficaram em aberto no design — agora resolvidas

1. `template delete` vai direto para `.trash/`; `--force` aceito como no-op.
2. `{{date}}`/`{{datetime}}` em hora local (mantido).
3. `add --open` implementado, funcionando com e sem `-t`.

Pendência única: commit da implementação (árvore suja em `master`).
