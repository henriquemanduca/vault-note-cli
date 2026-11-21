package tui

import (
	"errors"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/hmanduca/vault-note-cli/internal/settings"
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
	note             vault.Note
	noteID           string
	attemptedContent string
	conflict         bool
	err              error
}

type conflictCopySavedMsg struct {
	origID string
	copy   vault.Note
	err    error
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

type templateInsertedMsg struct {
	note     vault.Note
	template string
	err      error
}

type templateSavedMsg struct {
	name      string
	body      string
	overwrite bool
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

type trashEmptiedMsg struct {
	res vault.TrashEmptyResult
	err error
}

type settingsSavedMsg struct {
	vaultPath          string
	vault              *vault.Vault
	seededTemplates    []string
	showNotebookPrefix bool
	showLineNumbers    bool
	err                error
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
		return noteSavedMsg{
			note:             note,
			noteID:           noteID,
			attemptedContent: content,
			err:              err,
		}
	}
}

func saveNoteContentCheckedCmd(v *vault.Vault, noteID, content, expectedHash string) tea.Cmd {
	return func() tea.Msg {
		note, err := v.UpdateNoteContentChecked(noteID, content, expectedHash)
		return noteSavedMsg{
			note:             note,
			noteID:           noteID,
			attemptedContent: content,
			conflict:         errors.Is(err, vault.ErrConflict),
			err:              err,
		}
	}
}

func createConflictCopyCmd(v *vault.Vault, noteID, content string) tea.Cmd {
	return func() tea.Msg {
		copied, err := v.CreateConflictCopy(noteID, content)
		return conflictCopySavedMsg{origID: noteID, copy: copied, err: err}
	}
}

func createNoteCmd(v *vault.Vault, notebook, title, template string) tea.Cmd {
	return func() tea.Msg {
		var note vault.Note
		var err error
		if template != "" {
			note, err = v.CreateNoteFromTemplate(notebook, title, template)
		} else {
			note, err = v.CreateNote(notebook, title, "")
		}
		return noteCreatedMsg{note: note, err: err}
	}
}

// insertTemplateCmd appends a template to an existing note on disk.
func insertTemplateCmd(v *vault.Vault, noteID, template string) tea.Cmd {
	return func() tea.Msg {
		note, err := v.InsertTemplate(noteID, template, vault.InsertOptions{})
		return templateInsertedMsg{note: note, template: template, err: err}
	}
}

// saveAsTemplateCmd snapshots a note body into a template asynchronously.
func saveAsTemplateCmd(v *vault.Vault, name, body string, force bool) tea.Cmd {
	return func() tea.Msg {
		tpl, err := v.CreateTemplate(name, body, force)
		saved := name
		if err == nil {
			saved = tpl.Name
		}
		return templateSavedMsg{name: saved, body: body, overwrite: force, err: err}
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

func emptyTrashCmd(v *vault.Vault) tea.Cmd {
	return func() tea.Msg {
		res, err := v.EmptyTrash()
		return trashEmptiedMsg{res: res, err: err}
	}
}

func saveSettingsCmd(store *settings.Store, rawVaultPath string, showNotebookPrefix, showLineNumbers bool) tea.Cmd {
	return func() tea.Msg {
		v, err := vault.Open(rawVaultPath)
		if err != nil {
			return settingsSavedMsg{
				vaultPath:          rawVaultPath,
				showNotebookPrefix: showNotebookPrefix,
				showLineNumbers:    showLineNumbers,
				err:                err,
			}
		}
		seeded, err := v.SeedDefaultTemplatesIfAbsent()
		if err != nil {
			return settingsSavedMsg{
				vaultPath:          rawVaultPath,
				showNotebookPrefix: showNotebookPrefix,
				showLineNumbers:    showLineNumbers,
				err:                err,
			}
		}
		absPath := v.Root()
		if store != nil {
			cfg, err := store.Load()
			if err != nil {
				return settingsSavedMsg{
					vaultPath:          absPath,
					showNotebookPrefix: showNotebookPrefix,
					showLineNumbers:    showLineNumbers,
					err:                err,
				}
			}
			cfg.VaultPath = absPath
			cfg.ShowNotebookPrefix = &showNotebookPrefix
			cfg.ShowLineNumbers = &showLineNumbers
			if err := store.Save(cfg); err != nil {
				return settingsSavedMsg{
					vaultPath:          absPath,
					showNotebookPrefix: showNotebookPrefix,
					showLineNumbers:    showLineNumbers,
					err:                err,
				}
			}
		}
		return settingsSavedMsg{
			vaultPath:          absPath,
			vault:              v,
			seededTemplates:    seeded,
			showNotebookPrefix: showNotebookPrefix,
			showLineNumbers:    showLineNumbers,
		}
	}
}
