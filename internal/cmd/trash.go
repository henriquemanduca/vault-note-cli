package cmd

import (
	"flag"
	"fmt"
	"io"
	"strconv"

	"github.com/hmanduca/vault-note-cli/internal/vault"
)

func dispatchTrash(ctx *Context, args []string) error {
	if len(args) == 0 {
		return printHelp(ctx.Out, []string{"trash"})
	}
	switch args[0] {
	case "list", "ls":
		return RunAction(ctx, trashListAction{}, args[1:])
	case "empty", "clean", "clear", "purge":
		return RunAction(ctx, trashEmptyAction{}, args[1:])
	default:
		return UsageError("unknown trash subcommand %q (run 'vault-note-cli help trash')", args[0])
	}
}

// --- trash list ---

type trashListArgs struct {
	noHeader   bool
	timestamps bool
}

type trashListResult struct {
	items      []vault.TrashItem
	noHeader   bool
	timestamps bool
}

type trashListAction struct{}

func (trashListAction) Parse(_ *Context, args []string) (trashListArgs, error) {
	flagArgs, posArgs := splitFlagsAndPosArgs(args, nil)
	fs := flag.NewFlagSet("trash list", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var out trashListArgs
	fs.BoolVar(&out.noHeader, "n", false, "no header")
	fs.BoolVar(&out.noHeader, "no-header", false, "no header")
	fs.BoolVar(&out.timestamps, "t", false, "include trashed_at timestamp")
	fs.BoolVar(&out.timestamps, "timestamps", false, "include trashed_at timestamp")
	if err := fs.Parse(flagArgs); err != nil {
		return trashListArgs{}, UsageError("%v", err)
	}
	if len(posArgs) > 0 {
		return trashListArgs{}, UsageError("usage: vault-note-cli trash list [-t] [-n]")
	}
	return out, nil
}

func (trashListAction) Execute(ctx *Context, in trashListArgs) (trashListResult, error) {
	v, err := ctx.RequireVault()
	if err != nil {
		return trashListResult{}, err
	}
	items, err := v.ListTrash()
	if err != nil {
		return trashListResult{}, err
	}
	return trashListResult{
		items:      items,
		noHeader:   in.noHeader,
		timestamps: in.timestamps,
	}, nil
}

func (trashListAction) Format(ctx *Context, w io.Writer, res trashListResult) error {
	headers := []string{"id", "path", "size"}
	if res.timestamps {
		headers = append(headers, "trashed_at")
	}
	rows := make([][]string, 0, len(res.items))
	for _, it := range res.items {
		row := []string{it.ID, it.RelPath, strconv.FormatInt(it.SizeBytes, 10)}
		if res.timestamps {
			row = append(row, it.TrashedAt)
		}
		rows = append(rows, row)
	}
	return writeTabular(w, ctx.Format, headers, rows, res.noHeader)
}

// --- trash empty ---

type trashEmptyArgs struct {
	force bool
}

type trashEmptyAction struct{}

func (trashEmptyAction) Parse(_ *Context, args []string) (trashEmptyArgs, error) {
	flagArgs, posArgs := splitFlagsAndPosArgs(args, nil)
	fs := flag.NewFlagSet("trash empty", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var force bool
	fs.BoolVar(&force, "f", false, "permanently delete all items in .trash/")
	fs.BoolVar(&force, "force", false, "permanently delete all items in .trash/")
	if err := fs.Parse(flagArgs); err != nil {
		return trashEmptyArgs{}, UsageError("%v", err)
	}
	if len(posArgs) > 0 {
		return trashEmptyArgs{}, UsageError("usage: vault-note-cli trash empty --force")
	}
	return trashEmptyArgs{force: force}, nil
}

func (trashEmptyAction) Execute(ctx *Context, in trashEmptyArgs) (vault.TrashEmptyResult, error) {
	v, err := ctx.RequireVault()
	if err != nil {
		return vault.TrashEmptyResult{}, err
	}
	items, err := v.ListTrash()
	if err != nil {
		return vault.TrashEmptyResult{}, err
	}
	if len(items) == 0 {
		return v.EmptyTrash()
	}
	if !in.force {
		return vault.TrashEmptyResult{}, fmt.Errorf(
			"trash contains %d item(s); pass --force (-f) to permanently delete (run 'vault-note-cli trash list' to inspect)",
			len(items),
		)
	}
	return v.EmptyTrash()
}

func (trashEmptyAction) Format(_ *Context, w io.Writer, res vault.TrashEmptyResult) error {
	if res.RemovedItems == 0 {
		_, err := fmt.Fprintln(w, "Trash is already empty")
		return err
	}
	_, err := fmt.Fprintf(w, "Emptied trash: removed %d item(s) (%d bytes)\n", res.RemovedItems, res.RemovedBytes)
	return err
}
