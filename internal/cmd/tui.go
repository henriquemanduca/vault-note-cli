package cmd

import (
	"flag"
	"io"

	"github.com/hmanduca/vault-note-cli/internal/tui"
	"github.com/hmanduca/vault-note-cli/internal/vault"
)

func handleTUI(ctx *Context, args []string) error {
	flagArgs, _ := splitFlagsAndPosArgs(args, map[string]bool{"b": true, "notebook": true})
	fs := flag.NewFlagSet("tui", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var nb optionalStringFlag
	fs.Var(&nb, "b", "initial notebook filter")
	fs.Var(&nb, "notebook", "initial notebook filter")
	if err := fs.Parse(flagArgs); err != nil {
		return UsageError("%v", err)
	}

	var v *vault.Vault
	if ctx.VaultPath != "" {
		opened, err := vault.Open(ctx.VaultPath)
		if err != nil {
			return err
		}
		v = opened
	}

	return tui.Run(ctx.Store, v, tui.Options{
		InitialNotebook: resolveNotebookDefault(ctx, nb),
	})
}
