package tui

import (
	"errors"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/term"

	"github.com/hmanduca/vault-note-cli/internal/settings"
	"github.com/hmanduca/vault-note-cli/internal/vault"
)

// ErrNotTTY is returned when starting the interactive TUI without an attached terminal.
var ErrNotTTY = errors.New("interactive TUI requires a TTY on stdin and stdout (use CLI subcommands in pipes/scripts)")

// IsInteractiveTTY reports whether both stdin and stdout are attached to a terminal.
func IsInteractiveTTY() bool {
	return term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))
}

// Run starts the Bubble Tea TUI in AltScreen after validating TTY availability.
// If v is nil, the TUI automatically opens the Settings modal for initial vault setup.
func Run(settingsStore *settings.Store, v *vault.Vault, opts Options) error {
	if !IsInteractiveTTY() {
		return ErrNotTTY
	}
	m := NewModel(settingsStore, v, opts)
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("run TUI: %w", err)
	}
	return nil
}
