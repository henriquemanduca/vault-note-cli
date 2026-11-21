package cmd

import (
	"flag"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/hmanduca/vault-note-cli/internal/filestore"
	"github.com/hmanduca/vault-note-cli/internal/settings"
	"github.com/hmanduca/vault-note-cli/internal/vault"
)

func dispatchConfig(ctx *Context, args []string) error {
	if len(args) == 0 {
		return RunAction(ctx, configGetAction{}, nil)
	}
	switch args[0] {
	case "get", "list", "ls", "show":
		return RunAction(ctx, configGetAction{}, args[1:])
	case "set":
		return RunAction(ctx, configSetAction{}, args[1:])
	case "path":
		return RunAction(ctx, configPathAction{}, args[1:])
	case "open":
		return RunAction(ctx, configOpenAction{}, args[1:])
	default:
		return UsageError("unknown config subcommand %q (run 'vault-note-cli help config')", args[0])
	}
}

// --- config get ---

type configGetArgs struct {
	key      string
	noHeader bool
}

type configGetResult struct {
	singleKey string
	singleVal string
	items     []settings.Setting
	noHeader  bool
}

type configGetAction struct{}

func (configGetAction) Parse(_ *Context, args []string) (configGetArgs, error) {
	flagArgs, posArgs := splitFlagsAndPosArgs(args, nil)
	fs := flag.NewFlagSet("config get", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var noHeader bool
	fs.BoolVar(&noHeader, "n", false, "suppress header row")
	fs.BoolVar(&noHeader, "no-header", false, "suppress header row")
	if err := fs.Parse(flagArgs); err != nil {
		return configGetArgs{}, UsageError("%v", err)
	}
	if len(posArgs) > 1 {
		return configGetArgs{}, UsageError("usage: vault-note-cli config get [key] [-n]")
	}
	out := configGetArgs{noHeader: noHeader}
	if len(posArgs) == 1 {
		out.key = strings.TrimSpace(posArgs[0])
	}
	return out, nil
}

func (configGetAction) Execute(ctx *Context, in configGetArgs) (configGetResult, error) {
	if in.key != "" {
		val, ok, err := ctx.Store.GetSetting(in.key)
		if err != nil {
			return configGetResult{}, err
		}
		if !ok {
			return configGetResult{}, fmt.Errorf("%w: %q", settings.ErrNotFound, in.key)
		}
		return configGetResult{singleKey: in.key, singleVal: val}, nil
	}
	all, err := ctx.Store.ListSettings()
	if err != nil {
		return configGetResult{}, err
	}
	return configGetResult{items: all, noHeader: in.noHeader}, nil
}

func (configGetAction) Format(ctx *Context, w io.Writer, res configGetResult) error {
	if res.singleKey != "" {
		_, err := fmt.Fprintln(w, res.singleVal)
		return err
	}
	rows := make([][]string, 0, len(res.items))
	for _, s := range res.items {
		rows = append(rows, []string{s.Key, s.Value})
	}
	return writeTabular(w, ctx.Format, []string{"key", "value"}, rows, res.noHeader)
}

// --- config set ---

type configSetArgs struct {
	key   string
	value string
}

type configSetResult struct {
	key   string
	value string
}

type configSetAction struct{}

func (configSetAction) Parse(_ *Context, args []string) (configSetArgs, error) {
	_, posArgs := splitFlagsAndPosArgs(args, nil)
	if len(posArgs) != 2 {
		return configSetArgs{}, UsageError("usage: vault-note-cli config set <key> <value>")
	}
	return configSetArgs{
		key:   strings.TrimSpace(posArgs[0]),
		value: strings.TrimSpace(posArgs[1]),
	}, nil
}

func (configSetAction) Execute(ctx *Context, in configSetArgs) (configSetResult, error) {
	switch strings.ToLower(in.key) {
	case settings.KeyVaultPath:
		v, err := vault.Open(in.value)
		if err != nil {
			return configSetResult{}, err
		}
		if _, err := v.SeedDefaultTemplatesIfAbsent(); err != nil {
			return configSetResult{}, err
		}
		saved, err := ctx.Store.SetVaultPath(v.Root())
		if err != nil {
			return configSetResult{}, err
		}
		return configSetResult{key: settings.KeyVaultPath, value: saved}, nil
	case settings.KeyShowNotebookPrefix:
		enabled, err := settings.ParseBool(in.value)
		if err != nil {
			return configSetResult{}, UsageError("%v", err)
		}
		if err := ctx.Store.SetShowNotebookPrefix(enabled); err != nil {
			return configSetResult{}, err
		}
		return configSetResult{key: settings.KeyShowNotebookPrefix, value: strconv.FormatBool(enabled)}, nil
	case settings.KeyShowLineNumbers:
		enabled, err := settings.ParseBool(in.value)
		if err != nil {
			return configSetResult{}, UsageError("%v", err)
		}
		if err := ctx.Store.SetShowLineNumbers(enabled); err != nil {
			return configSetResult{}, err
		}
		return configSetResult{key: settings.KeyShowLineNumbers, value: strconv.FormatBool(enabled)}, nil
	default:
		return configSetResult{}, UsageError("unknown setting key %q (supported: %s, %s, %s)", in.key, settings.KeyVaultPath, settings.KeyShowNotebookPrefix, settings.KeyShowLineNumbers)
	}
}

func (configSetAction) Format(_ *Context, w io.Writer, res configSetResult) error {
	_, err := fmt.Fprintf(w, "Saved %s = %s\n", res.key, res.value)
	return err
}

// --- config path ---

type configPathAction struct{}

func (configPathAction) Parse(_ *Context, args []string) (struct{}, error) {
	if len(args) > 0 {
		return struct{}{}, UsageError("usage: vault-note-cli config path")
	}
	return struct{}{}, nil
}

func (configPathAction) Execute(ctx *Context, _ struct{}) (string, error) {
	v, err := ctx.RequireVault()
	if err != nil {
		return "", err
	}
	return v.Root(), nil
}

func (configPathAction) Format(_ *Context, w io.Writer, path string) error {
	_, err := fmt.Fprintln(w, path)
	return err
}

// --- config open ---

type configOpenAction struct{}

func (configOpenAction) Parse(_ *Context, args []string) (struct{}, error) {
	if len(args) > 0 {
		return struct{}{}, UsageError("usage: vault-note-cli config open")
	}
	return struct{}{}, nil
}

func (configOpenAction) Execute(ctx *Context, _ struct{}) (filestore.OpenResult, error) {
	v, err := ctx.RequireVault()
	if err != nil {
		return filestore.OpenResult{}, err
	}
	return filestore.OpenFolder(v.Root())
}

func (configOpenAction) Format(_ *Context, w io.Writer, res filestore.OpenResult) error {
	if !res.Opened {
		_, err := fmt.Fprintln(w, res.Path)
		return err
	}
	return nil
}
