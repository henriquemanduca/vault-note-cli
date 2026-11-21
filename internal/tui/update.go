package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.layout = computeLayout(msg.Width, msg.Height)
		m.syncEditorSize()
		m.clampListScroll()
		return m, nil

	case settingsSavedMsg:
		if msg.err != nil {
			m.openSettingsModalWithValue(msg.vaultPath, m.vault == nil, msg.err.Error())
			return m, nil
		}
		m.vault = msg.vault
		m.vaultPath = msg.vaultPath
		m.filterNotebook = ""
		m.filterQuery = ""
		m.searchInput.SetValue("")
		m.loadedNoteID = ""
		m.loadedContent = ""
		m.editor.SetValue("")
		m.dirty = false
		m.focus = focusList
		m.editor.Blur()
		m.closeModal()
		m.setStatus(fmt.Sprintf("Vault configurado: %s", msg.vaultPath), false)
		return m, m.reloadAllCmd("")

	case dataLoadedMsg:
		if msg.err != nil {
			m.setStatus(fmt.Sprintf("Erro ao carregar vault: %v", msg.err), true)
			return m, nil
		}
		prevID := m.loadedNoteID
		if msg.selectNoteID != "" {
			prevID = msg.selectNoteID
		}
		m.notes = msg.notes
		m.notebooks = msg.notebooks

		// Restore cursor on previously selected or target note.
		m.selected = 0
		if prevID != "" {
			for i, n := range m.notes {
				if n.ID == prevID {
					m.selected = i
					break
				}
			}
		}
		m.clampListScroll()
		if !m.dirty || m.loadedNoteID != m.selectedNoteID() {
			m.syncEditorToSelected()
		}
		if msg.focusEditorNow && m.currentNote() != nil {
			m.focus = focusEditor
			m.editor.Focus()
		}
		return m, nil

	case noteSavedMsg:
		if msg.err != nil {
			m.setStatus(fmt.Sprintf("Erro ao salvar: %v", msg.err), true)
			return m, nil
		}
		for i := range m.notes {
			if m.notes[i].ID == msg.note.ID {
				m.notes[i] = msg.note
				break
			}
		}
		if m.loadedNoteID == msg.note.ID && m.editor.Value() == msg.note.Content {
			m.loadedContent = msg.note.Content
			m.dirty = false
		}
		m.setStatus(fmt.Sprintf("Nota %s salva", msg.note.ID), false)
		return m, nil

	case noteCreatedMsg:
		if msg.err != nil {
			m.setStatus(fmt.Sprintf("Erro ao criar nota: %v", msg.err), true)
			return m, nil
		}
		m.setStatus(fmt.Sprintf("Nota %s criada — editando", msg.note.ID), false)
		return m, m.reloadAllWithFocusCmd(msg.note.ID, true)

	case noteUpdatedMsg:
		if msg.err != nil {
			m.setStatus(fmt.Sprintf("Erro ao atualizar nota: %v", msg.err), true)
			return m, nil
		}
		m.loadedNoteID = msg.note.ID
		m.setStatus(fmt.Sprintf("Nota atualizada: %s", msg.note.ID), false)
		return m, m.reloadAllCmd(msg.note.ID)

	case noteDeletedMsg:
		if msg.err != nil {
			m.setStatus(fmt.Sprintf("Erro ao mover para lixeira: %v", msg.err), true)
			return m, nil
		}
		m.dirty = false
		m.loadedNoteID = ""
		m.setStatus(fmt.Sprintf("Nota %s movida para .trash/", msg.noteID), false)
		return m, m.reloadAllCmd("")

	case notebookCreatedMsg:
		if msg.err != nil {
			m.setStatus(fmt.Sprintf("Erro ao criar caderno: %v", msg.err), true)
			return m, nil
		}
		m.filterNotebook = msg.nb.Name
		m.setStatus(fmt.Sprintf("Caderno %s criado e selecionado", msg.nb.Name), false)
		return m, m.reloadAllCmd("")

	case notebookUpdatedMsg:
		if msg.err != nil {
			m.setStatus(fmt.Sprintf("Erro ao renomear caderno: %v", msg.err), true)
			return m, nil
		}
		if strings.EqualFold(m.filterNotebook, msg.oldName) {
			m.filterNotebook = msg.nb.Name
		}
		m.setStatus(fmt.Sprintf("Caderno renomeado para %s", msg.nb.Name), false)
		return m, m.reloadAllCmd("")

	case notebookDeletedMsg:
		if msg.err != nil {
			m.setStatus(fmt.Sprintf("Erro ao excluir caderno: %v", msg.err), true)
			return m, nil
		}
		if strings.EqualFold(m.filterNotebook, msg.name) {
			m.filterNotebook = ""
		}
		m.setStatus(fmt.Sprintf("Caderno %s movido para .trash/", msg.name), false)
		return m, m.reloadAllCmd("")

	case externalEditFinishedMsg:
		if msg.err != nil {
			m.setStatus(fmt.Sprintf("Editor externo: %v", msg.err), true)
			return m, nil
		}
		if m.vault == nil {
			return m, nil
		}
		if m.loadedNoteID == msg.noteID {
			m.editor.SetValue(msg.content)
			m.editor.CursorEnd()
			m.loadedContent = msg.content
			m.dirty = false
		}
		return m, saveNoteContentCmd(m.vault, msg.noteID, msg.content)

	case tea.KeyMsg:
		if m.modal.kind != modalNone {
			return m.updateModal(msg)
		}
		if m.searchActive {
			return m.updateSearch(msg)
		}
		if m.focus == focusEditor {
			return m.updateEditor(msg)
		}
		return m.updateList(msg)
	}

	if m.focus == focusEditor && m.modal.kind == modalNone {
		var cmd tea.Cmd
		m.editor, cmd = m.editor.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m Model) selectedNoteID() string {
	if n := m.currentNote(); n != nil {
		return n.ID
	}
	return ""
}

func (m Model) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	switch key {
	case "ctrl+c":
		m.quitting = true
		if m.dirty && m.loadedNoteID != "" && m.vault != nil {
			_, _ = m.vault.UpdateNoteContent(m.loadedNoteID, m.editor.Value())
		}
		return m, tea.Quit

	case "q":
		m.openQuitConfirmModal()
		return m, nil

	case "c", ",":
		saveCmd := m.flushDirtyCmd()
		m.openSettingsModal(m.vault == nil)
		return m, saveCmd

	case "?":
		m.openHelpModal()
		return m, nil
	}

	// All remaining actions require a configured vault.
	if m.vault == nil {
		switch key {
		case "n", "b", "m", "d", "E", "/", "r", "enter":
			m.setStatus("Configure o caminho do vault primeiro", false)
			m.openSettingsModal(true)
		}
		return m, nil
	}

	switch key {
	case "j", "down":
		if len(m.notes) > 0 && m.selected < len(m.notes)-1 {
			saveCmd := m.flushDirtyCmd()
			m.selected++
			m.clampListScroll()
			m.syncEditorToSelected()
			return m, saveCmd
		}
		return m, nil

	case "k", "up":
		if len(m.notes) > 0 && m.selected > 0 {
			saveCmd := m.flushDirtyCmd()
			m.selected--
			m.clampListScroll()
			m.syncEditorToSelected()
			return m, saveCmd
		}
		return m, nil

	case "g":
		if len(m.notes) > 0 && m.selected != 0 {
			saveCmd := m.flushDirtyCmd()
			m.selected = 0
			m.clampListScroll()
			m.syncEditorToSelected()
			return m, saveCmd
		}
		return m, nil

	case "G":
		if len(m.notes) > 0 && m.selected != len(m.notes)-1 {
			saveCmd := m.flushDirtyCmd()
			m.selected = len(m.notes) - 1
			m.clampListScroll()
			m.syncEditorToSelected()
			return m, saveCmd
		}
		return m, nil

	case "enter", "tab", "l", "2":
		if m.currentNote() != nil {
			m.focus = focusEditor
			m.editor.Focus()
			m.setStatus("Editando (Ctrl+S salvar · Ctrl+E $EDITOR · Esc lista)", false)
		}
		return m, nil

	case "n":
		m.openNewNoteModal()
		return m, nil

	case "m":
		if m.currentNote() != nil {
			m.openEditMetaModal()
		}
		return m, nil

	case "E":
		if n := m.currentNote(); n != nil {
			return m, openExternalEditorCmd(n.ID, n.Content)
		}
		return m, nil

	case "d":
		if m.currentNote() != nil {
			m.openDeleteNoteModal()
		}
		return m, nil

	case "/":
		m.searchActive = true
		m.searchInput.SetValue(m.filterQuery)
		m.searchInput.Focus()
		return m, nil

	case "b":
		m.openNotebookModal()
		return m, nil

	case "r":
		m.setStatus("Vault recarregado", false)
		return m, m.reloadAllCmd(m.selectedNoteID())
	}
	return m, nil
}

func (m Model) updateEditor(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		m.quitting = true
		if m.dirty && m.loadedNoteID != "" && m.vault != nil {
			_, _ = m.vault.UpdateNoteContent(m.loadedNoteID, m.editor.Value())
		}
		return m, tea.Quit

	case "esc", "shift+tab":
		m.focus = focusList
		m.editor.Blur()
		cmd := m.flushDirtyCmd()
		if cmd == nil {
			m.setStatus("", false)
		}
		return m, cmd

	case "ctrl+s":
		if m.loadedNoteID == "" || m.vault == nil {
			return m, nil
		}
		content := m.editor.Value()
		m.loadedContent = content
		m.dirty = false
		return m, saveNoteContentCmd(m.vault, m.loadedNoteID, content)

	case "ctrl+e":
		if m.loadedNoteID == "" || m.vault == nil {
			return m, nil
		}
		return m, openExternalEditorCmd(m.loadedNoteID, m.editor.Value())
	}

	var cmd tea.Cmd
	m.editor, cmd = m.editor.Update(msg)
	m.dirty = m.editor.Value() != m.loadedContent
	return m, cmd
}

func (m Model) updateSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.searchActive = false
		m.searchInput.Blur()
		if m.filterQuery != "" {
			m.filterQuery = ""
			m.searchInput.SetValue("")
			return m, m.reloadAllCmd("")
		}
		return m, nil
	case "enter":
		m.searchActive = false
		m.searchInput.Blur()
		m.filterQuery = strings.TrimSpace(m.searchInput.Value())
		return m, m.reloadAllCmd("")
	}

	var cmd tea.Cmd
	m.searchInput, cmd = m.searchInput.Update(msg)
	newQuery := strings.TrimSpace(m.searchInput.Value())
	if newQuery != m.filterQuery {
		m.filterQuery = newQuery
		return m, tea.Batch(cmd, m.reloadAllCmd(""))
	}
	return m, cmd
}
