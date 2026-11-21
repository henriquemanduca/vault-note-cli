package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/hmanduca/vault-note-cli/internal/db"
	"github.com/hmanduca/vault-note-cli/internal/vault"
)

type dataLoadedMsg struct {
	notes          []vault.Note
	notebooks      []vault.Notebook
	selectNoteID   string
	focusEditorNow bool
	err            error
}

type noteSavedMsg struct {
	note vault.Note
	err  error
}

type noteCreatedMsg struct {
	note vault.Note
	err  error
}

type noteUpdatedMsg struct {
	note vault.Note
	err  error
}

type noteDeletedMsg struct {
	noteID    string
	trashPath string
	err       error
}

type notebookCreatedMsg struct {
	nb  vault.Notebook
	err error
}

type notebookUpdatedMsg struct {
	oldName string
	nb      vault.Notebook
	err     error
}

type notebookDeletedMsg struct {
	name      string
	trashPath string
	err       error
}

type settingsSavedMsg struct {
	vaultPath string
	vault     *vault.Vault
	err       error
}

func (m Model) reloadAllCmd(selectNoteID string) tea.Cmd {
	return m.reloadAllWithFocusCmd(selectNoteID, false)
}

func (m Model) reloadAllWithFocusCmd(selectNoteID string, focusEditorNow bool) tea.Cmd {
	v := m.vault
	if v == nil {
		return nil
	}
	filter := vault.NoteFilter{
		Notebook: m.filterNotebook,
		Query:    m.filterQuery,
	}
	return func() tea.Msg {
		notes, err := v.ListNotes(filter)
		if err != nil {
			return dataLoadedMsg{err: err}
		}
		nbs, err := v.ListNotebooks()
		if err != nil {
			return dataLoadedMsg{err: err}
		}
		return dataLoadedMsg{
			notes:          notes,
			notebooks:      nbs,
			selectNoteID:   selectNoteID,
			focusEditorNow: focusEditorNow,
		}
	}
}

func saveNoteContentCmd(v *vault.Vault, noteID, content string) tea.Cmd {
	return func() tea.Msg {
		note, err := v.UpdateNoteContent(noteID, content)
		return noteSavedMsg{note: note, err: err}
	}
}

func createNoteCmd(v *vault.Vault, notebook, title string) tea.Cmd {
	return func() tea.Msg {
		note, err := v.CreateNote(notebook, title, "")
		return noteCreatedMsg{note: note, err: err}
	}
}

func updateNoteMetaCmd(v *vault.Vault, noteID, title, notebook string) tea.Cmd {
	return func() tea.Msg {
		note, err := v.UpdateNote(noteID, vault.UpdateNoteInput{
			Title:       &title,
			SetNotebook: true,
			Notebook:    notebook,
		})
		return noteUpdatedMsg{note: note, err: err}
	}
}

func deleteNoteCmd(v *vault.Vault, noteID string) tea.Cmd {
	return func() tea.Msg {
		res, err := v.DeleteNote(noteID)
		return noteDeletedMsg{noteID: noteID, trashPath: res.TrashPath, err: err}
	}
}

func createNotebookCmd(v *vault.Vault, name string) tea.Cmd {
	return func() tea.Msg {
		nb, err := v.CreateNotebook(name)
		return notebookCreatedMsg{nb: nb, err: err}
	}
}

func renameNotebookCmd(v *vault.Vault, oldName, newName string) tea.Cmd {
	return func() tea.Msg {
		nb, err := v.RenameNotebook(oldName, newName)
		return notebookUpdatedMsg{oldName: oldName, nb: nb, err: err}
	}
}

func deleteNotebookCmd(v *vault.Vault, name string) tea.Cmd {
	return func() tea.Msg {
		res, err := v.DeleteNotebook(name, true)
		return notebookDeletedMsg{name: name, trashPath: res.TrashPath, err: err}
	}
}

func saveSettingsCmd(store *db.Store, rawVaultPath string) tea.Cmd {
	return func() tea.Msg {
		v, err := vault.Open(rawVaultPath)
		if err != nil {
			return settingsSavedMsg{vaultPath: rawVaultPath, err: err}
		}
		absPath := v.Root()
		if store != nil {
			saved, err := store.SetVaultPath(absPath)
			if err != nil {
				return settingsSavedMsg{vaultPath: absPath, err: err}
			}
			absPath = saved
		}
		return settingsSavedMsg{vaultPath: absPath, vault: v}
	}
}
