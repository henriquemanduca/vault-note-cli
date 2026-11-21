package tui

import (
	"fmt"
	"io"
	"os"

	osc52 "github.com/aymanbagabas/go-osc52/v2"
	tea "github.com/charmbracelet/bubbletea"
)

type clipboardCopiedMsg struct {
	label string
	err   error
}

// copyToClipboardOSC52 writes a terminal OSC 52 escape sequence to w, wrapping
// for tmux/screen passthrough when detected via environment variables.
func copyToClipboardOSC52(w io.Writer, text string) error {
	if w == nil {
		w = os.Stderr
	}
	seq := osc52.New(text)
	switch {
	case os.Getenv("TMUX") != "":
		seq = seq.Tmux()
	case os.Getenv("STY") != "":
		seq = seq.Screen()
	}
	if _, err := seq.WriteTo(w); err != nil {
		return fmt.Errorf("write OSC 52 clipboard sequence: %w", err)
	}
	return nil
}

func copyClipboardCmd(w io.Writer, label, text string) tea.Cmd {
	return func() tea.Msg {
		err := copyToClipboardOSC52(w, text)
		return clipboardCopiedMsg{label: label, err: err}
	}
}
