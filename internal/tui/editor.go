package tui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type externalEditFinishedMsg struct {
	noteID  string
	content string
	err     error
}

// ResolveEditor selects the user's preferred external editor ($VISUAL -> $EDITOR
// -> nano -> vi).
func ResolveEditor() []string {
	for _, env := range []string{"VISUAL", "EDITOR"} {
		if val := strings.TrimSpace(os.Getenv(env)); val != "" {
			fields := strings.Fields(val)
			if len(fields) > 0 {
				return fields
			}
		}
	}
	for _, fallback := range []string{"nano", "vi"} {
		if _, err := exec.LookPath(fallback); err == nil {
			return []string{fallback}
		}
	}
	return []string{"vi"}
}

// openExternalEditorCmd writes the note's current content to a temporary .txt
// file, suspends the TUI to launch $EDITOR via tea.ExecProcess, reads the
// edited file back, and removes the temporary file.
func openExternalEditorCmd(noteID, initialContent string) tea.Cmd {
	tmpFile, err := os.CreateTemp("", "vault-note-cli-*.txt")
	if err != nil {
		return func() tea.Msg {
			return externalEditFinishedMsg{noteID: noteID, err: fmt.Errorf("create temp file: %w", err)}
		}
	}
	tmpPath := tmpFile.Name()
	if _, err := tmpFile.WriteString(initialContent); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
		return func() tea.Msg {
			return externalEditFinishedMsg{noteID: noteID, err: fmt.Errorf("write temp file: %w", err)}
		}
	}
	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return func() tea.Msg {
			return externalEditFinishedMsg{noteID: noteID, err: fmt.Errorf("close temp file: %w", err)}
		}
	}

	editorCmd := ResolveEditor()
	args := append(editorCmd[1:], tmpPath)
	c := exec.Command(editorCmd[0], args...)

	return tea.ExecProcess(c, func(execErr error) tea.Msg {
		defer func() { _ = os.Remove(tmpPath) }()
		if execErr != nil {
			return externalEditFinishedMsg{noteID: noteID, err: fmt.Errorf("editor failed: %w", execErr)}
		}
		raw, readErr := os.ReadFile(tmpPath)
		if readErr != nil {
			return externalEditFinishedMsg{noteID: noteID, err: fmt.Errorf("read edited file: %w", readErr)}
		}
		return externalEditFinishedMsg{
			noteID:  noteID,
			content: string(raw),
		}
	})
}
