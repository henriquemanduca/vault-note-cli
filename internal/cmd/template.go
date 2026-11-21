package cmd

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/hmanduca/vault-note-cli/internal/filestore"
	"github.com/hmanduca/vault-note-cli/internal/tui"
	"github.com/hmanduca/vault-note-cli/internal/vault"
)

func dispatchTemplate(ctx *Context, args []string) error {
	if len(args) == 0 {
		return printHelp(ctx.Out, []string{"template"})
	}
	switch args[0] {
	case "add", "create", "new":
		return RunAction(ctx, templateAddAction{}, args[1:])
	case "list", "ls":
		return RunAction(ctx, templateListAction{}, args[1:])
	case "view", "show", "get":
		return RunAction(ctx, templateViewAction{}, args[1:])
	case "update", "mod", "edit", "mv":
		return RunAction(ctx, templateUpdateAction{}, args[1:])
	case "delete", "del", "rm", "remove":
		return RunAction(ctx, templateDeleteAction{}, args[1:])
	case "init", "seed":
		return RunAction(ctx, templateInitAction{}, args[1:])
	default:
		return UsageError("unknown template subcommand %q (run 'vault-note-cli help template')", args[0])
	}
}

// --- template list ---

type templateListArgs struct {
	noHeader bool
}

type templateListResult struct {
	templates []vault.Template
	noHeader  bool
}

type templateListAction struct{}

func (templateListAction) Parse(_ *Context, args []string) (templateListArgs, error) {
	flagArgs, _ := splitFlagsAndPosArgs(args, nil)
	fs := flag.NewFlagSet("template list", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var noHeader bool
	fs.BoolVar(&noHeader, "n", false, "suppress header row")
	fs.BoolVar(&noHeader, "no-header", false, "suppress header row")
	if err := fs.Parse(flagArgs); err != nil {
		return templateListArgs{}, UsageError("%v", err)
	}
	return templateListArgs{noHeader: noHeader}, nil
}

func (templateListAction) Execute(ctx *Context, in templateListArgs) (templateListResult, error) {
	v, err := ctx.RequireVault()
	if err != nil {
		return templateListResult{}, err
	}
	tpls, err := v.ListTemplates()
	if err != nil {
		return templateListResult{}, err
	}
	return templateListResult{templates: tpls, noHeader: in.noHeader}, nil
}

func (templateListAction) Format(ctx *Context, w io.Writer, res templateListResult) error {
	headers := []string{"name", "path"}
	rows := make([][]string, 0, len(res.templates))
	for _, tpl := range res.templates {
		rows = append(rows, []string{tpl.Name, tpl.Path})
	}
	return writeTabular(w, ctx.Format, headers, rows, res.noHeader)
}

// --- template view ---

type templateViewArgs struct {
	name string
	raw  bool
}

type templateViewResult struct {
	template vault.Template
	raw      bool
}

type templateViewAction struct{}

func (templateViewAction) Parse(_ *Context, args []string) (templateViewArgs, error) {
	flagArgs, posArgs := splitFlagsAndPosArgs(args, nil)
	fs := flag.NewFlagSet("template view", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var raw bool
	fs.BoolVar(&raw, "raw", false, "print only the template body")
	if err := fs.Parse(flagArgs); err != nil {
		return templateViewArgs{}, UsageError("%v", err)
	}
	if len(posArgs) != 1 {
		return templateViewArgs{}, UsageError("usage: vault-note-cli template view <nome> [--raw]")
	}
	return templateViewArgs{name: posArgs[0], raw: raw}, nil
}

func (templateViewAction) Execute(ctx *Context, in templateViewArgs) (templateViewResult, error) {
	v, err := ctx.RequireVault()
	if err != nil {
		return templateViewResult{}, err
	}
	tpl, err := v.GetTemplate(in.name)
	if err != nil {
		return templateViewResult{}, err
	}
	return templateViewResult{template: tpl, raw: in.raw}, nil
}

func (templateViewAction) Format(_ *Context, w io.Writer, res templateViewResult) error {
	if res.raw {
		_, err := fmt.Fprint(w, res.template.Body)
		if err == nil && res.template.Body != "" && !strings.HasSuffix(res.template.Body, "\n") {
			_, err = fmt.Fprintln(w)
		}
		return err
	}
	_, _ = fmt.Fprintf(w, "Name: %s\n", res.template.Name)
	_, _ = fmt.Fprintf(w, "Path: %s\n", res.template.Path)
	if res.template.Body != "" {
		_, _ = fmt.Fprintf(w, "\n%s", res.template.Body)
		if !strings.HasSuffix(res.template.Body, "\n") {
			_, _ = fmt.Fprintln(w)
		}
	}
	return nil
}

// --- shared body resolution for add/update ---

// templateBodyFlags holds the content-source flags shared by template add/update.
type templateBodyFlags struct {
	stdin bool
	edit  bool
}

// resolveTemplateBody reads the template body from positional args, --stdin,
// or $EDITOR (seeded with seed when editing an existing template).
func resolveTemplateBody(ctx *Context, posArgs []string, flags templateBodyFlags, seed string) (string, error) {
	sources := 0
	body := ""
	if len(posArgs) >= 2 {
		body = strings.Join(posArgs[1:], " ")
		sources++
	}
	if flags.stdin {
		raw, err := io.ReadAll(ctx.In)
		if err != nil {
			return "", fmt.Errorf("read stdin: %w", err)
		}
		body = string(raw)
		sources++
	}
	if flags.edit {
		edited, err := editTemplateBody(ctx, seed)
		if err != nil {
			return "", err
		}
		body = edited
		sources++
	}
	if sources == 0 {
		return "", UsageError("template body is required (pass it as an argument, use --stdin, or -e/--edit)")
	}
	if sources > 1 {
		return "", UsageError("provide only one source for the template body (argument, --stdin, or -e/--edit)")
	}
	return body, nil
}

// editTemplateBody opens $EDITOR on a temp file outside the vault and returns its contents.
func editTemplateBody(ctx *Context, seed string) (string, error) {
	dir, err := filestore.ConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve config dir: %w", err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create config dir: %w", err)
	}
	tmp, err := os.CreateTemp(dir, "template-*.txt")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()

	if _, err := tmp.WriteString(seed); err != nil {
		_ = tmp.Close()
		return "", fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("close temp file: %w", err)
	}

	ed := tui.ResolveEditor()
	cmd := exec.CommandContext(ctx.Ctx, ed[0], append(ed[1:], tmpPath)...)
	cmd.Stdin = ctx.In
	cmd.Stdout = ctx.Out
	cmd.Stderr = ctx.Err
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("editor failed: %w", err)
	}
	raw, err := os.ReadFile(tmpPath)
	if err != nil {
		return "", fmt.Errorf("read edited template: %w", err)
	}
	return string(raw), nil
}

// --- template add ---

type templateAddArgs struct {
	name     string
	body     string
	from     string
	notebook string
	force    bool
	open     bool
}

type templateAddAction struct{}

func (templateAddAction) Parse(ctx *Context, args []string) (templateAddArgs, error) {
	flagArgs, posArgs := splitFlagsAndPosArgs(args, map[string]bool{
		"from": true, "b": true, "notebook": true,
	})
	fs := flag.NewFlagSet("template add", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var flags templateBodyFlags
	var from string
	var nb optionalStringFlag
	var force bool
	var open bool
	fs.BoolVar(&flags.stdin, "stdin", false, "read template body from stdin")
	fs.BoolVar(&flags.edit, "e", false, "edit template body in $EDITOR")
	fs.BoolVar(&flags.edit, "edit", false, "edit template body in $EDITOR")
	fs.StringVar(&from, "from", "", "copy the body from an existing note")
	fs.Var(&nb, "b", "notebook context for --from")
	fs.Var(&nb, "notebook", "notebook context for --from")
	fs.BoolVar(&force, "f", false, "overwrite an existing template")
	fs.BoolVar(&force, "force", false, "overwrite an existing template")
	fs.BoolVar(&open, "open", false, "open the template in $EDITOR")
	if err := fs.Parse(flagArgs); err != nil {
		return templateAddArgs{}, UsageError("%v", err)
	}
	if len(posArgs) < 1 {
		return templateAddArgs{}, UsageError("usage: vault-note-cli template add <nome> [corpo] [--stdin] [-e] [--from <nota>] [-b <caderno>] [-f] [--open]")
	}
	if open && flags.edit {
		return templateAddArgs{}, UsageError("--open cannot be combined with -e/--edit")
	}

	if from != "" {
		if len(posArgs) >= 2 {
			return templateAddArgs{}, UsageError("--from cannot be combined with a positional body")
		}
		if flags.stdin || flags.edit {
			return templateAddArgs{}, UsageError("--from cannot be combined with --stdin or -e/--edit")
		}
		return templateAddArgs{
			name:     posArgs[0],
			from:     from,
			notebook: resolveNotebookDefault(ctx, nb),
			force:    force,
			open:     open,
		}, nil
	}

	if nb.set {
		return templateAddArgs{}, UsageError("-b/--notebook requires --from")
	}
	body, err := resolveTemplateBody(ctx, posArgs, flags, "")
	if err != nil {
		return templateAddArgs{}, err
	}
	return templateAddArgs{name: posArgs[0], body: body, force: force, open: open}, nil
}

func (templateAddAction) Execute(ctx *Context, in templateAddArgs) (vault.Template, error) {
	v, err := ctx.RequireVault()
	if err != nil {
		return vault.Template{}, err
	}

	var tpl vault.Template
	if in.from != "" {
		tpl, err = v.CreateTemplateFromNote(in.from, in.notebook, in.name, in.force)
		if errors.Is(err, vault.ErrAlreadyExists) {
			return vault.Template{}, fmt.Errorf("%w (use --force to overwrite)", err)
		}
	} else {
		tpl, err = v.SaveTemplate(in.name, in.body)
	}
	if err != nil {
		return vault.Template{}, err
	}
	if in.open {
		if err := openPathInEditor(ctx, tpl.Path); err != nil {
			return vault.Template{}, err
		}
	}
	return tpl, nil
}

func (templateAddAction) Format(_ *Context, w io.Writer, tpl vault.Template) error {
	_, err := fmt.Fprintf(w, "Created template %s\n", tpl.Name)
	return err
}

// --- template update ---

type templateUpdateArgs struct {
	name string
	body string
}

type templateUpdateAction struct{}

func (templateUpdateAction) Parse(ctx *Context, args []string) (templateUpdateArgs, error) {
	flagArgs, posArgs := splitFlagsAndPosArgs(args, nil)
	fs := flag.NewFlagSet("template update", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var flags templateBodyFlags
	fs.BoolVar(&flags.stdin, "stdin", false, "read template body from stdin")
	fs.BoolVar(&flags.edit, "e", false, "edit template body in $EDITOR")
	fs.BoolVar(&flags.edit, "edit", false, "edit template body in $EDITOR")
	if err := fs.Parse(flagArgs); err != nil {
		return templateUpdateArgs{}, UsageError("%v", err)
	}
	if len(posArgs) < 1 {
		return templateUpdateArgs{}, UsageError("usage: vault-note-cli template update <nome> [corpo] [--stdin] [-e]")
	}

	seed := ""
	if flags.edit && len(posArgs) < 2 && !flags.stdin {
		v, err := ctx.RequireVault()
		if err != nil {
			return templateUpdateArgs{}, err
		}
		tpl, err := v.GetTemplate(posArgs[0])
		if err != nil {
			return templateUpdateArgs{}, err
		}
		seed = tpl.Body
	}

	body, err := resolveTemplateBody(ctx, posArgs, flags, seed)
	if err != nil {
		return templateUpdateArgs{}, err
	}
	return templateUpdateArgs{name: posArgs[0], body: body}, nil
}

func (templateUpdateAction) Execute(ctx *Context, in templateUpdateArgs) (vault.Template, error) {
	v, err := ctx.RequireVault()
	if err != nil {
		return vault.Template{}, err
	}
	if _, err := v.GetTemplate(in.name); err != nil {
		return vault.Template{}, err
	}
	return v.SaveTemplate(in.name, in.body)
}

func (templateUpdateAction) Format(_ *Context, w io.Writer, tpl vault.Template) error {
	_, err := fmt.Fprintf(w, "Updated template %s\n", tpl.Name)
	return err
}

// --- template delete (.trash/) ---

type templateDeleteArgs struct {
	name string
}

type templateDeleteAction struct{}

func (templateDeleteAction) Parse(_ *Context, args []string) (templateDeleteArgs, error) {
	flagArgs, posArgs := splitFlagsAndPosArgs(args, nil)
	fs := flag.NewFlagSet("template delete", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	// Accepted for symmetry with 'notebook delete'; a template is a single file,
	// so deletion never needs extra confirmation.
	var force bool
	fs.BoolVar(&force, "f", false, "no-op, kept for symmetry")
	fs.BoolVar(&force, "force", false, "no-op, kept for symmetry")
	if err := fs.Parse(flagArgs); err != nil {
		return templateDeleteArgs{}, UsageError("%v", err)
	}
	if len(posArgs) != 1 {
		return templateDeleteArgs{}, UsageError("usage: vault-note-cli template delete <nome>")
	}
	return templateDeleteArgs{name: posArgs[0]}, nil
}

func (templateDeleteAction) Execute(ctx *Context, in templateDeleteArgs) (vault.TrashResult, error) {
	v, err := ctx.RequireVault()
	if err != nil {
		return vault.TrashResult{}, err
	}
	return v.DeleteTemplate(in.name)
}

func (templateDeleteAction) Format(_ *Context, w io.Writer, res vault.TrashResult) error {
	_, err := fmt.Fprintf(w, "Moved template %s to %s\n", res.OriginalID, res.TrashPath)
	return err
}

// --- template init ---

type templateInitAction struct{}

func (templateInitAction) Parse(_ *Context, args []string) (struct{}, error) {
	flagArgs, posArgs := splitFlagsAndPosArgs(args, nil)
	if len(flagArgs) > 0 || len(posArgs) > 0 {
		return struct{}{}, UsageError("usage: vault-note-cli template init")
	}
	return struct{}{}, nil
}

func (templateInitAction) Execute(ctx *Context, _ struct{}) ([]string, error) {
	v, err := ctx.RequireVault()
	if err != nil {
		return nil, err
	}
	return v.SeedDefaultTemplates()
}

func (templateInitAction) Format(_ *Context, w io.Writer, created []string) error {
	if len(created) == 0 {
		_, err := fmt.Fprintln(w, "Default templates already present (0 created)")
		return err
	}
	_, err := fmt.Fprintf(w, "Initialized %d default template(s): %s\n", len(created), strings.Join(created, ", "))
	return err
}
