package cmd

import (
	"flag"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/hmanduca/vault-note-cli/internal/tui"
	"github.com/hmanduca/vault-note-cli/internal/vault"
)

func dispatchNote(ctx *Context, args []string) error {
	if len(args) == 0 {
		return printHelp(ctx.Out, []string{"note"})
	}
	switch args[0] {
	case "add", "create", "new":
		return RunAction(ctx, noteAddAction{}, args[1:])
	case "list", "ls":
		return RunAction(ctx, noteListAction{}, args[1:])
	case "view", "show", "get":
		return RunAction(ctx, noteViewAction{}, args[1:])
	case "update", "mod", "mv":
		return RunAction(ctx, noteUpdateAction{}, args[1:])
	case "edit":
		return RunAction(ctx, noteEditAction{}, args[1:])
	case "insert", "ins":
		return RunAction(ctx, noteInsertAction{}, args[1:])
	case "delete", "del", "rm", "remove":
		return RunAction(ctx, noteDeleteAction{}, args[1:])
	case "search", "find":
		return RunAction(ctx, noteSearchAction{}, args[1:])
	default:
		return UsageError("unknown note subcommand %q (run 'vault-note-cli help note')", args[0])
	}
}

// --- note add ---

type noteAddArgs struct {
	title    string
	content  string
	notebook string
	template string
	open     bool
}

type noteAddAction struct{}

func (noteAddAction) Parse(ctx *Context, args []string) (noteAddArgs, error) {
	flagArgs, posArgs := splitFlagsAndPosArgs(args, map[string]bool{
		"b": true, "notebook": true,
		"c": true, "content": true,
		"t": true, "template": true,
	})
	fs := flag.NewFlagSet("note add", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var nb optionalStringFlag
	var tpl optionalStringFlag
	var contentFlag string
	var fromStdin bool
	var openEditor bool

	fs.Var(&nb, "b", "notebook folder name")
	fs.Var(&nb, "notebook", "notebook folder name")
	fs.StringVar(&contentFlag, "c", "", "note content")
	fs.StringVar(&contentFlag, "content", "", "note content")
	fs.BoolVar(&fromStdin, "stdin", false, "read note content from stdin")
	fs.Var(&tpl, "t", "create the note from a template")
	fs.Var(&tpl, "template", "create the note from a template")
	fs.BoolVar(&openEditor, "o", false, "open the new note in $EDITOR")
	fs.BoolVar(&openEditor, "open", false, "open the new note in $EDITOR")

	if err := fs.Parse(flagArgs); err != nil {
		return noteAddArgs{}, UsageError("%v", err)
	}
	if len(posArgs) < 1 {
		return noteAddArgs{}, UsageError("usage: vault-note-cli note add <title> [content] [-b <notebook>] [-c <content>] [--stdin] [-t <template>] [--open]")
	}

	templateName := ""
	if tpl.set {
		templateName = strings.TrimSpace(tpl.value)
		switch {
		case templateName == "":
			return noteAddArgs{}, UsageError("flag --template requires a template name")
		case contentFlag != "" || len(posArgs) >= 2 || fromStdin:
			return noteAddArgs{}, UsageError("flag --template cannot be combined with inline content, --content or --stdin")
		}
	}

	content := contentFlag
	if len(posArgs) >= 2 && content == "" {
		content = strings.Join(posArgs[1:], " ")
	}
	if fromStdin {
		raw, err := io.ReadAll(ctx.In)
		if err != nil {
			return noteAddArgs{}, fmt.Errorf("read stdin: %w", err)
		}
		content = string(raw)
	}

	return noteAddArgs{
		title:    posArgs[0],
		content:  content,
		notebook: resolveNotebookDefault(ctx, nb),
		template: templateName,
		open:     openEditor,
	}, nil
}

func (noteAddAction) Execute(ctx *Context, in noteAddArgs) (vault.Note, error) {
	v, err := ctx.RequireVault()
	if err != nil {
		return vault.Note{}, err
	}

	var note vault.Note
	if in.template != "" {
		note, err = v.CreateNoteFromTemplate(in.notebook, in.title, in.template)
	} else {
		note, err = v.CreateNote(in.notebook, in.title, in.content)
	}
	if err != nil {
		return vault.Note{}, err
	}
	if !in.open {
		return note, nil
	}

	if err := openPathInEditor(ctx, note.Path); err != nil {
		return vault.Note{}, err
	}
	return v.GetNote(note.ID)
}

func (noteAddAction) Format(_ *Context, w io.Writer, n vault.Note) error {
	_, err := fmt.Fprintf(w, "Created note %s\n", n.ID)
	return err
}

// --- note list ---

type noteListArgs struct {
	filter     vault.NoteFilter
	noHeader   bool
	timestamps bool
}

type noteListResult struct {
	notes      []vault.Note
	noHeader   bool
	timestamps bool
}

type noteListAction struct{}

func (noteListAction) Parse(ctx *Context, args []string) (noteListArgs, error) {
	flagArgs, posArgs := splitFlagsAndPosArgs(args, map[string]bool{
		"b": true, "notebook": true,
		"q": true, "query": true,
	})
	fs := flag.NewFlagSet("note list", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var nb optionalStringFlag
	var query string
	var noHeader bool
	var timestamps bool

	fs.Var(&nb, "b", "filter by notebook folder")
	fs.Var(&nb, "notebook", "filter by notebook folder")
	fs.StringVar(&query, "q", "", "search query")
	fs.StringVar(&query, "query", "", "search query")
	fs.BoolVar(&noHeader, "n", false, "suppress header row")
	fs.BoolVar(&noHeader, "no-header", false, "suppress header row")
	fs.BoolVar(&timestamps, "timestamps", false, "show updated_at column")

	if err := fs.Parse(flagArgs); err != nil {
		return noteListArgs{}, UsageError("%v", err)
	}
	if query == "" && len(posArgs) > 0 {
		query = strings.Join(posArgs, " ")
	}

	return noteListArgs{
		filter: vault.NoteFilter{
			Notebook: resolveNotebookDefault(ctx, nb),
			Query:    query,
		},
		noHeader:   noHeader,
		timestamps: timestamps,
	}, nil
}

func (noteListAction) Execute(ctx *Context, in noteListArgs) (noteListResult, error) {
	v, err := ctx.RequireVault()
	if err != nil {
		return noteListResult{}, err
	}
	notes, err := v.ListNotes(in.filter)
	if err != nil {
		return noteListResult{}, err
	}
	return noteListResult{notes: notes, noHeader: in.noHeader, timestamps: in.timestamps}, nil
}

func (noteListAction) Format(ctx *Context, w io.Writer, res noteListResult) error {
	return formatNoteList(ctx, w, res.notes, res.noHeader, res.timestamps)
}

func formatNoteList(ctx *Context, w io.Writer, notes []vault.Note, noHeader, timestamps bool) error {
	headers := []string{"id", "notebook", "title"}
	if timestamps {
		headers = append(headers, "updated_at")
	}
	rows := make([][]string, 0, len(notes))
	for _, n := range notes {
		nb := "-"
		if n.Notebook != "" {
			nb = n.Notebook
		}
		row := []string{n.ID, nb, n.Title}
		if timestamps {
			row = append(row, n.UpdatedAt)
		}
		rows = append(rows, row)
	}
	return writeTabular(w, ctx.Format, headers, rows, noHeader)
}

// --- note search ---

type noteSearchAction struct{}

func (noteSearchAction) Parse(ctx *Context, args []string) (noteListArgs, error) {
	parsed, err := noteListAction{}.Parse(ctx, args)
	if err != nil {
		return noteListArgs{}, err
	}
	if parsed.filter.Query == "" {
		return noteListArgs{}, UsageError("usage: vault-note-cli note search <query> [-b <notebook>]")
	}
	return parsed, nil
}

func (noteSearchAction) Execute(ctx *Context, in noteListArgs) (noteListResult, error) {
	return noteListAction{}.Execute(ctx, in)
}

func (noteSearchAction) Format(ctx *Context, w io.Writer, res noteListResult) error {
	return noteListAction{}.Format(ctx, w, res)
}

// --- note view ---

type noteViewArgs struct {
	ref        string
	notebook   string
	raw        bool
	timestamps bool
}

type noteViewResult struct {
	note       vault.Note
	raw        bool
	timestamps bool
}

type noteViewAction struct{}

func (noteViewAction) Parse(ctx *Context, args []string) (noteViewArgs, error) {
	flagArgs, posArgs := splitFlagsAndPosArgs(args, map[string]bool{
		"b": true, "notebook": true,
	})
	fs := flag.NewFlagSet("note view", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var nb optionalStringFlag
	var raw bool
	var timestamps bool
	fs.Var(&nb, "b", "notebook context")
	fs.Var(&nb, "notebook", "notebook context")
	fs.BoolVar(&raw, "raw", false, "output only note content")
	fs.BoolVar(&timestamps, "t", false, "show timestamps")
	fs.BoolVar(&timestamps, "timestamps", false, "show timestamps")

	if err := fs.Parse(flagArgs); err != nil {
		return noteViewArgs{}, UsageError("%v", err)
	}
	if len(posArgs) != 1 {
		return noteViewArgs{}, UsageError("usage: vault-note-cli note view <ref> [-b <notebook>] [--raw] [-t]")
	}
	return noteViewArgs{
		ref:        posArgs[0],
		notebook:   resolveNotebookDefault(ctx, nb),
		raw:        raw,
		timestamps: timestamps,
	}, nil
}

func (noteViewAction) Execute(ctx *Context, in noteViewArgs) (noteViewResult, error) {
	v, err := ctx.RequireVault()
	if err != nil {
		return noteViewResult{}, err
	}
	note, err := v.ResolveNote(in.ref, in.notebook)
	if err != nil {
		return noteViewResult{}, err
	}
	return noteViewResult{note: note, raw: in.raw, timestamps: in.timestamps}, nil
}

func (noteViewAction) Format(_ *Context, w io.Writer, res noteViewResult) error {
	if res.raw {
		_, err := fmt.Fprint(w, res.note.Content)
		if err == nil && res.note.Content != "" && !strings.HasSuffix(res.note.Content, "\n") {
			_, err = fmt.Fprintln(w)
		}
		return err
	}

	n := res.note
	_, _ = fmt.Fprintf(w, "ID:       %s\n", n.ID)
	_, _ = fmt.Fprintf(w, "Title:    %s\n", n.Title)
	if n.Notebook != "" {
		_, _ = fmt.Fprintf(w, "Notebook: %s\n", n.Notebook)
	}
	_, _ = fmt.Fprintf(w, "Path:     %s\n", n.Path)
	if res.timestamps {
		_, _ = fmt.Fprintf(w, "Updated:  %s\n", n.UpdatedAt)
	}
	if n.Content != "" {
		_, _ = fmt.Fprintf(w, "\n%s", n.Content)
		if !strings.HasSuffix(n.Content, "\n") {
			_, _ = fmt.Fprintln(w)
		}
	}
	return nil
}

// --- note update ---

type noteUpdateArgs struct {
	ref         string
	defaultNB   string
	title       *string
	content     *string
	setNotebook bool
	notebook    string
}

type noteUpdateAction struct{}

func (noteUpdateAction) Parse(ctx *Context, args []string) (noteUpdateArgs, error) {
	flagArgs, posArgs := splitFlagsAndPosArgs(args, map[string]bool{
		"b": true, "notebook": true,
		"c": true, "content": true,
	})
	fs := flag.NewFlagSet("note update", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var nb optionalStringFlag
	var content optionalStringFlag
	var fromStdin bool

	fs.Var(&nb, "b", "move to notebook folder (or 'none' for vault root)")
	fs.Var(&nb, "notebook", "move to notebook folder (or 'none' for vault root)")
	fs.Var(&content, "c", "replace note content")
	fs.Var(&content, "content", "replace note content")
	fs.BoolVar(&fromStdin, "stdin", false, "read replacement content from stdin")

	if err := fs.Parse(flagArgs); err != nil {
		return noteUpdateArgs{}, UsageError("%v", err)
	}
	if len(posArgs) < 1 {
		return noteUpdateArgs{}, UsageError("usage: vault-note-cli note update <ref> [new-title] [-c <content>] [-b <notebook|none>] [--stdin]")
	}

	out := noteUpdateArgs{
		ref: posArgs[0],
	}
	if ctx.Config != nil {
		out.defaultNB = strings.TrimSpace(ctx.Config.Notebook)
	}
	if len(posArgs) >= 2 {
		t := strings.Join(posArgs[1:], " ")
		out.title = &t
	}
	if content.set {
		c := content.value
		out.content = &c
	}
	if fromStdin {
		raw, err := io.ReadAll(ctx.In)
		if err != nil {
			return noteUpdateArgs{}, fmt.Errorf("read stdin: %w", err)
		}
		c := string(raw)
		out.content = &c
	}
	if nb.set {
		out.setNotebook = true
		out.notebook = strings.TrimSpace(nb.value)
	}
	if out.title == nil && out.content == nil && !out.setNotebook {
		return noteUpdateArgs{}, UsageError("nothing to update: provide a new title, -c, --stdin, or -b")
	}
	return out, nil
}

func (noteUpdateAction) Execute(ctx *Context, in noteUpdateArgs) (vault.Note, error) {
	v, err := ctx.RequireVault()
	if err != nil {
		return vault.Note{}, err
	}
	note, err := v.ResolveNote(in.ref, in.defaultNB)
	if err != nil {
		return vault.Note{}, err
	}
	return v.UpdateNote(note.ID, vault.UpdateNoteInput{
		Title:       in.title,
		Content:     in.content,
		SetNotebook: in.setNotebook,
		Notebook:    in.notebook,
	})
}

func (noteUpdateAction) Format(_ *Context, w io.Writer, n vault.Note) error {
	_, err := fmt.Fprintf(w, "Updated note %s\n", n.ID)
	return err
}

// --- note edit ($EDITOR) ---

type noteEditArgs struct {
	ref      string
	notebook string
}

type noteEditAction struct{}

func (noteEditAction) Parse(ctx *Context, args []string) (noteEditArgs, error) {
	flagArgs, posArgs := splitFlagsAndPosArgs(args, map[string]bool{
		"b": true, "notebook": true,
	})
	fs := flag.NewFlagSet("note edit", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var nb optionalStringFlag
	fs.Var(&nb, "b", "notebook context")
	fs.Var(&nb, "notebook", "notebook context")
	if err := fs.Parse(flagArgs); err != nil {
		return noteEditArgs{}, UsageError("%v", err)
	}
	if len(posArgs) != 1 {
		return noteEditArgs{}, UsageError("usage: vault-note-cli note edit <ref> [-b <notebook>]")
	}
	return noteEditArgs{
		ref:      posArgs[0],
		notebook: resolveNotebookDefault(ctx, nb),
	}, nil
}

func (noteEditAction) Execute(ctx *Context, in noteEditArgs) (vault.Note, error) {
	v, err := ctx.RequireVault()
	if err != nil {
		return vault.Note{}, err
	}
	note, err := v.ResolveNote(in.ref, in.notebook)
	if err != nil {
		return vault.Note{}, err
	}

	if err := openPathInEditor(ctx, note.Path); err != nil {
		return vault.Note{}, err
	}
	return v.GetNote(note.ID)
}

func (noteEditAction) Format(_ *Context, w io.Writer, n vault.Note) error {
	_, err := fmt.Fprintf(w, "Saved note %s\n", n.ID)
	return err
}

// --- note delete (.trash/) ---

type noteDeleteArgs struct {
	ref      string
	notebook string
}

type noteDeleteAction struct{}

func (noteDeleteAction) Parse(ctx *Context, args []string) (noteDeleteArgs, error) {
	flagArgs, posArgs := splitFlagsAndPosArgs(args, map[string]bool{
		"b": true, "notebook": true,
	})
	fs := flag.NewFlagSet("note delete", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var nb optionalStringFlag
	fs.Var(&nb, "b", "notebook context")
	fs.Var(&nb, "notebook", "notebook context")
	if err := fs.Parse(flagArgs); err != nil {
		return noteDeleteArgs{}, UsageError("%v", err)
	}
	if len(posArgs) != 1 {
		return noteDeleteArgs{}, UsageError("usage: vault-note-cli note delete <ref> [-b <notebook>]")
	}
	return noteDeleteArgs{
		ref:      posArgs[0],
		notebook: resolveNotebookDefault(ctx, nb),
	}, nil
}

func (noteDeleteAction) Execute(ctx *Context, in noteDeleteArgs) (vault.TrashResult, error) {
	v, err := ctx.RequireVault()
	if err != nil {
		return vault.TrashResult{}, err
	}
	note, err := v.ResolveNote(in.ref, in.notebook)
	if err != nil {
		return vault.TrashResult{}, err
	}
	return v.DeleteNote(note.ID)
}

func (noteDeleteAction) Format(_ *Context, w io.Writer, res vault.TrashResult) error {
	_, err := fmt.Fprintf(w, "Moved note %s to %s\n", res.OriginalID, res.TrashPath)
	return err
}

// openPathInEditor launches the user's $EDITOR on a file path, wiring the CLI streams.
func openPathInEditor(ctx *Context, path string) error {
	ed := tui.ResolveEditor()
	cmd := exec.CommandContext(ctx.Ctx, ed[0], append(ed[1:], path)...)
	cmd.Stdin = ctx.In
	cmd.Stdout = ctx.Out
	cmd.Stderr = ctx.Err
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("editor failed: %w", err)
	}
	return nil
}

// --- note insert (template into an existing note) ---

type noteInsertArgs struct {
	ref      string
	notebook string
	template string
	prepend  bool
	at       optionalIntFlag
	print    bool
	open     bool
}

type noteInsertResult struct {
	note     vault.Note
	template string
	printed  bool
	content  string
}

type noteInsertAction struct{}

func (noteInsertAction) Parse(ctx *Context, args []string) (noteInsertArgs, error) {
	flagArgs, posArgs := splitFlagsAndPosArgs(args, map[string]bool{
		"b": true, "notebook": true,
		"t": true, "template": true,
		"at": true,
	})
	fs := flag.NewFlagSet("note insert", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var nb optionalStringFlag
	var tpl optionalStringFlag
	var out noteInsertArgs
	fs.Var(&nb, "b", "notebook context")
	fs.Var(&nb, "notebook", "notebook context")
	fs.Var(&tpl, "t", "template name")
	fs.Var(&tpl, "template", "template name")
	fs.BoolVar(&out.prepend, "prepend", false, "insert at the start of the note")
	fs.Var(&out.at, "at", "insert at rune offset (clamped)")
	fs.BoolVar(&out.print, "print", false, "print the composed note without writing")
	fs.BoolVar(&out.open, "o", false, "open the note in $EDITOR after inserting")
	fs.BoolVar(&out.open, "open", false, "open the note in $EDITOR after inserting")

	if err := fs.Parse(flagArgs); err != nil {
		return noteInsertArgs{}, UsageError("%v", err)
	}
	if len(posArgs) != 1 {
		return noteInsertArgs{}, UsageError("usage: vault-note-cli note insert <ref> -t <template> [--prepend|--at <N>] [--print] [--open] [-b <notebook>]")
	}
	if !tpl.set || strings.TrimSpace(tpl.value) == "" {
		return noteInsertArgs{}, UsageError("flag --template is required (e.g. -t checklist)")
	}
	if out.prepend && out.at.set {
		return noteInsertArgs{}, UsageError("flags --prepend and --at cannot be combined")
	}
	if out.at.set && out.at.value < 0 {
		return noteInsertArgs{}, UsageError("flag --at requires a non-negative rune offset")
	}
	if out.print && out.open {
		return noteInsertArgs{}, UsageError("flags --print and --open cannot be combined")
	}

	out.ref = posArgs[0]
	out.notebook = resolveNotebookDefault(ctx, nb)
	out.template = strings.TrimSpace(tpl.value)
	return out, nil
}

func (in noteInsertArgs) insertOptions() vault.InsertOptions {
	opts := vault.InsertOptions{Mode: vault.InsertAppend}
	switch {
	case in.at.set:
		opts.Mode = vault.InsertAtRune
		opts.RuneOffset = in.at.value
	case in.prepend:
		opts.Mode = vault.InsertPrepend
	}
	return opts
}

func (noteInsertAction) Execute(ctx *Context, in noteInsertArgs) (noteInsertResult, error) {
	v, err := ctx.RequireVault()
	if err != nil {
		return noteInsertResult{}, err
	}
	note, err := v.ResolveNote(in.ref, in.notebook)
	if err != nil {
		return noteInsertResult{}, err
	}

	if in.print {
		rendered, err := v.RenderTemplateFor(note, in.template)
		if err != nil {
			return noteInsertResult{}, err
		}
		return noteInsertResult{
			note:     note,
			template: in.template,
			printed:  true,
			content:  vault.InsertSnippet(note.Content, rendered, in.insertOptions()),
		}, nil
	}

	updated, err := v.InsertTemplate(note.ID, in.template, in.insertOptions())
	if err != nil {
		return noteInsertResult{}, err
	}
	if in.open {
		if err := openPathInEditor(ctx, updated.Path); err != nil {
			return noteInsertResult{}, err
		}
		if reopened, err := v.GetNote(updated.ID); err == nil {
			updated = reopened
		}
	}
	return noteInsertResult{note: updated, template: in.template}, nil
}

func (noteInsertAction) Format(ctx *Context, w io.Writer, res noteInsertResult) error {
	if res.printed {
		if _, err := fmt.Fprint(w, res.content); err != nil {
			return err
		}
		if res.content != "" && !strings.HasSuffix(res.content, "\n") {
			if _, err := fmt.Fprintln(w); err != nil {
				return err
			}
		}
		_, err := fmt.Fprintln(ctx.Err, "(dry-run, nota não modificada)")
		return err
	}
	_, err := fmt.Fprintf(w, "Inserted template %s into %s\n", res.template, res.note.ID)
	return err
}
