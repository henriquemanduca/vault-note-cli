package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/hmanduca/vault-note-cli/internal/buildinfo"
	"github.com/hmanduca/vault-note-cli/internal/config"
	"github.com/hmanduca/vault-note-cli/internal/db"
)

// Run is the single CLI entrypoint invoked by cmd/vault-note-cli/main.go.
func Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	err := dispatch(ctx, args, stdin, stdout, stderr)
	if err != nil {
		if msg := err.Error(); msg != "" {
			_, _ = fmt.Fprintf(stderr, "error: %s\n", msg)
		}
		return ExitCode(err)
	}
	return ExitOK
}

func dispatch(stdCtx context.Context, rawArgs []string, stdin io.Reader, stdout, stderr io.Writer) error {
	gOpts, args, err := extractGlobalFlags(rawArgs)
	if err != nil {
		return err
	}

	// Short-circuit version and help without opening DB or parsing .vaultnotecli.
	if len(args) > 0 {
		switch args[0] {
		case "version", "--version", "-v":
			_, err := fmt.Fprintln(stdout, buildinfo.Summary())
			return err
		case "help", "--help", "-h":
			return printHelp(stdout, args[1:])
		}
	}
	if hasHelpFlag(args) {
		return printHelp(stdout, stripHelpFlags(expandAliases(args)))
	}

	cwd, _ := os.Getwd()
	cfg, err := config.Load(cwd)
	if err != nil {
		return fmt.Errorf("load workspace config: %w", err)
	}

	store, err := db.NewStore(gOpts.dbPath)
	if err != nil {
		return err
	}
	defer func() { _ = store.Close() }()

	vaultPath, err := resolveVaultPath(gOpts.vaultPath, cfg, store)
	if err != nil {
		return err
	}

	c := &Context{
		Ctx:       stdCtx,
		Store:     store,
		Config:    cfg,
		DBPath:    store.Path(),
		VaultPath: vaultPath,
		Format:    gOpts.format,
		Out:       stdout,
		Err:       stderr,
		In:        stdin,
		Cwd:       cwd,
	}

	// No subcommand -> launch interactive TUI directly.
	if len(args) == 0 {
		return handleTUI(c, nil)
	}

	expanded := expandAliases(args)
	switch expanded[0] {
	case "tui":
		return handleTUI(c, expanded[1:])
	case "note":
		return dispatchNote(c, expanded[1:])
	case "notebook":
		return dispatchNotebook(c, expanded[1:])
	case "config":
		return dispatchConfig(c, expanded[1:])
	default:
		return UsageError("unknown command %q (run 'vault-note-cli help' for usage)", args[0])
	}
}

// resolveVaultPath applies explicit precedence:
// --vault flag > VAULT_NOTE_CLI_VAULT env > .vaultnotecli VAULT_PATH > SQLite vault_path.
func resolveVaultPath(flagVault string, cfg *config.Config, store *db.Store) (string, error) {
	if vp := strings.TrimSpace(flagVault); vp != "" {
		return vp, nil
	}
	if vp := strings.TrimSpace(os.Getenv("VAULT_NOTE_CLI_VAULT")); vp != "" {
		return vp, nil
	}
	if cfg != nil && strings.TrimSpace(cfg.VaultPath) != "" {
		return strings.TrimSpace(cfg.VaultPath), nil
	}
	return store.GetVaultPath()
}

func stripHelpFlags(args []string) []string {
	var out []string
	for _, a := range args {
		if a == "-h" || a == "--help" {
			continue
		}
		out = append(out, a)
	}
	return out
}
