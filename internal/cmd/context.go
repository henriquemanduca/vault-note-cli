package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/hmanduca/vault-note-cli/internal/config"
	"github.com/hmanduca/vault-note-cli/internal/db"
	"github.com/hmanduca/vault-note-cli/internal/vault"
)

// Standard CLI exit codes.
const (
	ExitOK      = 0
	ExitRuntime = 1
	ExitUsage   = 2
)

// ExitError wraps an error with a specific exit status code.
type ExitError struct {
	Code int
	Err  error
}

func (e *ExitError) Error() string {
	if e.Err == nil {
		return ""
	}
	return e.Err.Error()
}

func (e *ExitError) Unwrap() error {
	return e.Err
}

// UsageError creates an error mapped to exit code 2 (invalid CLI arguments).
func UsageError(format string, args ...any) error {
	return &ExitError{
		Code: ExitUsage,
		Err:  fmt.Errorf(format, args...),
	}
}

// ExitCode extracts the exit status code for err.
func ExitCode(err error) int {
	if err == nil {
		return ExitOK
	}
	var exitErr *ExitError
	if errors.As(err, &exitErr) {
		return exitErr.Code
	}
	return ExitRuntime
}

// OutputFormat controls how tabular listings are rendered.
type OutputFormat string

const (
	FormatTSV   OutputFormat = "tsv"
	FormatTable OutputFormat = "table"
)

// Context carries injected runtime dependencies across CLI handlers.
type Context struct {
	Ctx       context.Context
	Store     *db.Store
	Config    *config.Config
	DBPath    string
	VaultPath string
	vault     *vault.Vault
	Format    OutputFormat
	Out       io.Writer
	Err       io.Writer
	In        io.Reader
	Cwd       string
}

// RequireVault lazily opens and caches the configured filesystem vault.
// If no vault path is configured, it returns an actionable error.
func (c *Context) RequireVault() (*vault.Vault, error) {
	if c.vault != nil {
		return c.vault, nil
	}
	if c.VaultPath == "" {
		return nil, errors.New("vault path not configured (run 'vault-note-cli config set vault_path <dir>' or launch 'vault-note-cli' to configure)")
	}
	v, err := vault.Open(c.VaultPath)
	if err != nil {
		return nil, err
	}
	c.vault = v
	c.VaultPath = v.Root()
	return v, nil
}
