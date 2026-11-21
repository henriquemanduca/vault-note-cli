package cmd

import (
	"flag"
	"fmt"
	"io"
	"strconv"

	"github.com/hmanduca/vault-note-cli/internal/vault"
)

func dispatchNotebook(ctx *Context, args []string) error {
	if len(args) == 0 {
		return printHelp(ctx.Out, []string{"notebook"})
	}
	switch args[0] {
	case "add", "create", "new":
		return RunAction(ctx, nbAddAction{}, args[1:])
	case "list", "ls":
		return RunAction(ctx, nbListAction{}, args[1:])
	case "view", "show", "get":
		return RunAction(ctx, nbViewAction{}, args[1:])
	case "rename", "update", "edit", "mod", "mv":
		return RunAction(ctx, nbRenameAction{}, args[1:])
	case "delete", "del", "rm":
		return RunAction(ctx, nbDeleteAction{}, args[1:])
	default:
		return UsageError("unknown notebook subcommand %q (run 'vault-note-cli help notebook')", args[0])
	}
}

type nbAddAction struct{}

func (nbAddAction) Parse(_ *Context, args []string) (string, error) {
	_, posArgs := splitFlagsAndPosArgs(args, nil)
	if len(posArgs) != 1 {
		return "", UsageError("usage: vault-note-cli notebook add <name>")
	}
	return posArgs[0], nil
}

func (nbAddAction) Execute(ctx *Context, name string) (vault.Notebook, error) {
	v, err := ctx.RequireVault()
	if err != nil {
		return vault.Notebook{}, err
	}
	return v.CreateNotebook(name)
}

func (nbAddAction) Format(_ *Context, w io.Writer, nb vault.Notebook) error {
	_, err := fmt.Fprintf(w, "Created notebook %s (%s)\n", nb.Name, nb.Path)
	return err
}

type nbListArgs struct {
	noHeader   bool
	timestamps bool
}

type nbListResult struct {
	notebooks  []vault.Notebook
	noHeader   bool
	timestamps bool
}

type nbListAction struct{}

func (nbListAction) Parse(_ *Context, args []string) (nbListArgs, error) {
	flagArgs, _ := splitFlagsAndPosArgs(args, nil)
	fs := flag.NewFlagSet("notebook list", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var out nbListArgs
	fs.BoolVar(&out.noHeader, "n", false, "no header")
	fs.BoolVar(&out.noHeader, "no-header", false, "no header")
	fs.BoolVar(&out.timestamps, "t", false, "include updated_at")
	fs.BoolVar(&out.timestamps, "timestamps", false, "include updated_at")
	if err := fs.Parse(flagArgs); err != nil {
		return nbListArgs{}, UsageError("%v", err)
	}
	return out, nil
}

func (nbListAction) Execute(ctx *Context, in nbListArgs) (nbListResult, error) {
	v, err := ctx.RequireVault()
	if err != nil {
		return nbListResult{}, err
	}
	nbs, err := v.ListNotebooks()
	if err != nil {
		return nbListResult{}, err
	}
	return nbListResult{
		notebooks:  nbs,
		noHeader:   in.noHeader,
		timestamps: in.timestamps,
	}, nil
}

func (nbListAction) Format(ctx *Context, w io.Writer, res nbListResult) error {
	headers := []string{"notebook", "notes"}
	if res.timestamps {
		headers = append(headers, "updated_at")
	}
	rows := make([][]string, 0, len(res.notebooks))
	for _, nb := range res.notebooks {
		row := []string{nb.Name, strconv.Itoa(nb.NoteCount)}
		if res.timestamps {
			row = append(row, nb.UpdatedAt)
		}
		rows = append(rows, row)
	}
	return writeTabular(w, ctx.Format, headers, rows, res.noHeader)
}

type nbViewArgs struct {
	ref        string
	timestamps bool
}

type nbViewResult struct {
	nb         vault.Notebook
	timestamps bool
}

type nbViewAction struct{}

func (nbViewAction) Parse(_ *Context, args []string) (nbViewArgs, error) {
	flagArgs, posArgs := splitFlagsAndPosArgs(args, nil)
	fs := flag.NewFlagSet("notebook view", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var ts bool
	fs.BoolVar(&ts, "t", false, "timestamps")
	fs.BoolVar(&ts, "timestamps", false, "timestamps")
	if err := fs.Parse(flagArgs); err != nil {
		return nbViewArgs{}, UsageError("%v", err)
	}
	if len(posArgs) != 1 {
		return nbViewArgs{}, UsageError("usage: vault-note-cli notebook view <name> [-t]")
	}
	return nbViewArgs{ref: posArgs[0], timestamps: ts}, nil
}

func (nbViewAction) Execute(ctx *Context, in nbViewArgs) (nbViewResult, error) {
	v, err := ctx.RequireVault()
	if err != nil {
		return nbViewResult{}, err
	}
	nb, err := v.GetNotebook(in.ref)
	if err != nil {
		return nbViewResult{}, err
	}
	return nbViewResult{nb: nb, timestamps: in.timestamps}, nil
}

func (nbViewAction) Format(_ *Context, w io.Writer, res nbViewResult) error {
	nb := res.nb
	_, _ = fmt.Fprintf(w, "Notebook: %s\n", nb.Name)
	_, _ = fmt.Fprintf(w, "Path:     %s\n", nb.Path)
	_, _ = fmt.Fprintf(w, "Notes:    %d\n", nb.NoteCount)
	if res.timestamps {
		_, _ = fmt.Fprintf(w, "Updated:  %s\n", nb.UpdatedAt)
	}
	return nil
}

type nbRenameArgs struct {
	oldName string
	newName string
}

type nbRenameAction struct{}

func (nbRenameAction) Parse(_ *Context, args []string) (nbRenameArgs, error) {
	flagArgs, posArgs := splitFlagsAndPosArgs(args, map[string]bool{"name": true})
	fs := flag.NewFlagSet("notebook rename", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var nameFlag string
	fs.StringVar(&nameFlag, "name", "", "new folder name")
	if err := fs.Parse(flagArgs); err != nil {
		return nbRenameArgs{}, UsageError("%v", err)
	}
	if len(posArgs) == 2 {
		return nbRenameArgs{oldName: posArgs[0], newName: posArgs[1]}, nil
	}
	if len(posArgs) == 1 && nameFlag != "" {
		return nbRenameArgs{oldName: posArgs[0], newName: nameFlag}, nil
	}
	return nbRenameArgs{}, UsageError("usage: vault-note-cli notebook rename <old-name> <new-name>")
}

func (nbRenameAction) Execute(ctx *Context, in nbRenameArgs) (vault.Notebook, error) {
	v, err := ctx.RequireVault()
	if err != nil {
		return vault.Notebook{}, err
	}
	return v.RenameNotebook(in.oldName, in.newName)
}

func (nbRenameAction) Format(_ *Context, w io.Writer, nb vault.Notebook) error {
	_, err := fmt.Fprintf(w, "Renamed notebook to %s (%s)\n", nb.Name, nb.Path)
	return err
}

type nbDeleteArgs struct {
	name  string
	force bool
}

type nbDeleteAction struct{}

func (nbDeleteAction) Parse(_ *Context, args []string) (nbDeleteArgs, error) {
	flagArgs, posArgs := splitFlagsAndPosArgs(args, nil)
	fs := flag.NewFlagSet("notebook delete", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var force bool
	fs.BoolVar(&force, "f", false, "move non-empty folder to .trash")
	fs.BoolVar(&force, "force", false, "move non-empty folder to .trash")
	if err := fs.Parse(flagArgs); err != nil {
		return nbDeleteArgs{}, UsageError("%v", err)
	}
	if len(posArgs) != 1 {
		return nbDeleteArgs{}, UsageError("usage: vault-note-cli notebook delete <name> [--force]")
	}
	return nbDeleteArgs{name: posArgs[0], force: force}, nil
}

func (nbDeleteAction) Execute(ctx *Context, in nbDeleteArgs) (vault.TrashResult, error) {
	v, err := ctx.RequireVault()
	if err != nil {
		return vault.TrashResult{}, err
	}
	return v.DeleteNotebook(in.name, in.force)
}

func (nbDeleteAction) Format(_ *Context, w io.Writer, res vault.TrashResult) error {
	_, err := fmt.Fprintf(w, "Moved notebook %s to %s\n", res.OriginalID, res.TrashPath)
	return err
}
