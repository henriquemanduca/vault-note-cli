package tui

import (
	"errors"
	"fmt"
	"strings"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/hmanduca/vault-note-cli/internal/vault"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.layout = computeLayoutWithZen(msg.Width, msg.Height, m.zenMode)
		m.syncEditorSize()
		m.clampListScroll()
		return m, nil

	case settingsSavedMsg:
		if msg.err != nil {
			m.showNotebookPrefix = msg.showNotebookPrefix
			m.showLineNumbers = msg.showLineNumbers
			m.editor.ShowLineNumbers = m.showLineNumbers
			m.openSettingsModalWithValue(msg.vaultPath, m.vault == nil, msg.err.Error())
			return m, nil
		}
		vaultChanged := m.vault == nil || m.vaultPath != msg.vaultPath
		m.showNotebookPrefix = msg.showNotebookPrefix
		m.showLineNumbers = msg.showLineNumbers
		m.editor.ShowLineNumbers = m.showLineNumbers
		m.vault = msg.vault
		m.vaultPath = msg.vaultPath
		m.closeModal()
		if vaultChanged {
			m.filterNotebook = ""
			m.filterQuery = ""
			m.searchInput.SetValue("")
			m.loadedNoteID = ""
			m.loadedContent = ""
			m.loadedHash = ""
			m.editor.SetValue("")
			m.dirty = false
			m.zenMode = false
			m.closeFindBar()
			m.history.Reset()
			m.focus = focusList
			m.editor.Blur()
			m.recomputeLayout()
			if len(msg.seededTemplates) > 0 {
				m.setStatus(fmt.Sprintf("Vault configurado: %s (%d templates padrão criados)", msg.vaultPath, len(msg.seededTemplates)), false)
			} else {
				m.setStatus(fmt.Sprintf("Vault configurado: %s", msg.vaultPath), false)
			}
			return m, m.reloadAllCmd("")
		}
		m.syncEditorSize()
		m.setStatus("Configurações salvas", false)
		return m, nil

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
		if msg.conflict {
			m.setStatus(fmt.Sprintf("Conflito externo detectado em %s", msg.noteID), true)
			m.openConflictModal(msg.noteID, msg.attemptedContent)
			return m, nil
		}
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
			m.loadedHash = vault.ContentHash(msg.note.Content)
			m.dirty = false
		}
		m.setStatus(fmt.Sprintf("Nota %s salva", msg.note.ID), false)
		return m, nil

	case conflictCopySavedMsg:
		if msg.err != nil {
			m.setStatus(fmt.Sprintf("Erro ao salvar cópia de conflito: %v", msg.err), true)
			return m, nil
		}
		if m.loadedNoteID == msg.origID {
			m.dirty = false
		}
		m.setStatus(fmt.Sprintf("Cópia de conflito salva: %s", msg.copy.ID), false)
		return m, m.reloadAllCmd(msg.copy.ID)

	case clipboardCopiedMsg:
		if msg.err != nil {
			m.setStatus(fmt.Sprintf("Erro ao copiar: %v", msg.err), true)
			return m, nil
		}
		m.setStatus(msg.label+" copiado para o clipboard", false)
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

	case trashEmptiedMsg:
		if msg.err != nil {
			m.setStatus(fmt.Sprintf("Erro ao esvaziar lixeira: %v", msg.err), true)
			return m, nil
		}
		if msg.res.RemovedItems == 0 {
			m.setStatus("A lixeira (.trash/) já está vazia", false)
			return m, nil
		}
		m.setStatus(fmt.Sprintf("Lixeira esvaziada: %d item(ns) removido(s) (%s)", msg.res.RemovedItems, formatByteSize(msg.res.RemovedBytes)), false)
		return m, nil

	case templateInsertedMsg:
		if msg.err != nil {
			m.setStatus(fmt.Sprintf("Erro ao inserir template: %v", msg.err), true)
			return m, nil
		}
		for i := range m.notes {
			if m.notes[i].ID == msg.note.ID {
				m.notes[i] = msg.note
				break
			}
		}
		if m.loadedNoteID == msg.note.ID {
			m.editor.SetValue(msg.note.Content)
			m.editor.CursorEnd()
			m.loadedContent = msg.note.Content
			m.loadedHash = vault.ContentHash(msg.note.Content)
			m.dirty = false
			m.history.Reset()
		}
		m.setStatus(fmt.Sprintf("Template %s acrescentado a %s", msg.template, msg.note.ID), false)
		return m, nil

	case templateSavedMsg:
		if msg.err != nil {
			if errors.Is(msg.err, vault.ErrAlreadyExists) && !msg.overwrite {
				m.modal.kind = modalConfirmOverwriteTemplate
				m.modal.errText = ""
				return m, nil
			}
			m.modal.errText = msg.err.Error()
			return m, nil
		}
		m.closeModal()
		m.setStatus("Template salvo: "+msg.name, false)
		return m, nil

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
			m.loadedHash = vault.ContentHash(msg.content)
			m.dirty = false
			m.history.Reset()
		}
		return m, saveNoteContentCmd(m.vault, msg.noteID, msg.content)

	case tea.MouseMsg:
		return m.handleMouse(msg)

	case tea.KeyMsg:
		if m.modal.kind != modalNone {
			return m.updateModal(msg)
		}
		if m.searchActive {
			return m.updateSearch(msg)
		}
		if m.find.active {
			return m.updateEditorFind(msg)
		}
		if m.focus == focusEditor {
			return m.updateEditor(msg)
		}
		return m.updateList(msg)
	}

	if m.focus == focusEditor && m.modal.kind == modalNone && !m.find.active {
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
		m.saveOnQuit()
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
		case "n", "b", "m", "d", "D", "E", "T", "S", "Y", "z", "/", "r", "enter":
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

	case "D":
		m.openEmptyTrashModal()
		return m, nil

	case "T":
		if n := m.currentNote(); n != nil {
			m.openInsertTemplateModal(n.ID, true)
		}
		return m, nil

	case "S":
		if n := m.currentNote(); n != nil {
			m.openSaveAsTemplateModal(n)
		}
		return m, nil

	case "Y":
		if n := m.currentNote(); n != nil {
			return m, copyClipboardCmd(m.opts.ClipboardWriter, "Nota "+n.ID, n.Content)
		}
		return m, nil

	case "z":
		if m.currentNote() != nil {
			m.zenMode = !m.zenMode
			if m.zenMode {
				m.focus = focusEditor
				m.editor.Focus()
				m.setStatus("Modo Zen ativado (Alt+Z ou Esc para sair)", false)
			} else {
				m.setStatus("Modo Zen desativado", false)
			}
			m.recomputeLayout()
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
		m.saveOnQuit()
		return m, tea.Quit

	case "esc", "shift+tab":
		m.closeFindBar()
		if m.zenMode {
			m.zenMode = false
			m.recomputeLayout()
		}
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
		return m, saveNoteContentCheckedCmd(m.vault, m.loadedNoteID, content, m.loadedHash)

	case "ctrl+e":
		if m.loadedNoteID == "" || m.vault == nil {
			return m, nil
		}
		return m, openExternalEditorCmd(m.loadedNoteID, m.editor.Value())

	case "ctrl+t":
		if m.loadedNoteID == "" || m.vault == nil {
			return m, nil
		}
		m.openInsertTemplateModal(m.loadedNoteID, false)
		return m, nil

	case "alt+t":
		if m.loadedNoteID == "" || m.vault == nil {
			return m, nil
		}
		m.openSaveAsTemplateModal(m.currentNote())
		return m, nil

	case "ctrl+z":
		prev, ok := m.history.Undo(captureEditorSnapshot(m.editor))
		if !ok {
			m.setStatus("Nada para desfazer", false)
			return m, nil
		}
		applyEditorSnapshot(&m.editor, prev)
		m.dirty = m.editor.Value() != m.loadedContent
		m.setStatus("Alteração desfeita", false)
		return m, nil

	case "ctrl+y":
		next, ok := m.history.Redo(captureEditorSnapshot(m.editor))
		if !ok {
			m.setStatus("Nada para refazer", false)
			return m, nil
		}
		applyEditorSnapshot(&m.editor, next)
		m.dirty = m.editor.Value() != m.loadedContent
		m.setStatus("Alteração refeita", false)
		return m, nil

	case "ctrl+f":
		m.openFindBar(false)
		return m, nil

	case "ctrl+r":
		m.openFindBar(true)
		return m, nil

	case "alt+x":
		m.applyLineTransform(func(text string, line, col int) (string, int, int, bool) {
			nt, nl, nc := toggleCheckboxOnLine(text, line, col)
			return nt, nl, nc, nt != text
		})
		return m, nil

	case "enter":
		if !msg.Alt {
			snap := captureEditorSnapshot(m.editor)
			if nt, nl, nc, handled := continueListOnEnter(snap.value, snap.line, snap.col); handled {
				m.history.RecordBeforeEdit(snap, editOpDiscrete)
				m.editor.SetValue(nt)
				setEditorCursor(&m.editor, nl, nc)
				m.dirty = m.editor.Value() != m.loadedContent
				return m, nil
			}
		}

	case "alt+up":
		m.applyLineTransform(func(text string, line, col int) (string, int, int, bool) {
			return moveLine(text, line, col, -1)
		})
		return m, nil

	case "alt+down":
		m.applyLineTransform(func(text string, line, col int) (string, int, int, bool) {
			return moveLine(text, line, col, 1)
		})
		return m, nil

	case "alt+p":
		m.applyLineTransform(func(text string, line, col int) (string, int, int, bool) {
			nt, nl, nc := duplicateLine(text, line, col)
			return nt, nl, nc, true
		})
		return m, nil

	case "alt+k":
		m.applyLineTransform(func(text string, line, col int) (string, int, int, bool) {
			nt, nl, nc := deleteLine(text, line, col)
			return nt, nl, nc, nt != text
		})
		return m, nil

	case "tab", "alt+]":
		m.applyLineTransform(func(text string, line, col int) (string, int, int, bool) {
			nt, nl, nc := indentLine(text, line, col, 2)
			return nt, nl, nc, true
		})
		return m, nil

	case "alt+[":
		m.applyLineTransform(func(text string, line, col int) (string, int, int, bool) {
			nt, nl, nc := dedentLine(text, line, col, 2)
			return nt, nl, nc, nt != text
		})
		return m, nil

	case "alt+z":
		m.zenMode = !m.zenMode
		m.recomputeLayout()
		if m.zenMode {
			m.setStatus("Modo Zen ativado (Alt+Z para sair)", false)
		} else {
			m.setStatus("Modo Zen desativado", false)
		}
		return m, nil

	case "alt+n":
		m.showLineNumbers = !m.showLineNumbers
		m.editor.ShowLineNumbers = m.showLineNumbers
		m.syncEditorSize()
		if m.showLineNumbers {
			m.setStatus("Números de linha ativados", false)
		} else {
			m.setStatus("Números de linha desativados", false)
		}
		return m, nil

	case "alt+y":
		line, _ := editorCursorPos(m.editor)
		return m, copyClipboardCmd(m.opts.ClipboardWriter, "Linha", lineAt(m.editor.Value(), line))

	case "alt+w":
		label := "Nota"
		if m.loadedNoteID != "" {
			label = "Nota " + m.loadedNoteID
		}
		return m, copyClipboardCmd(m.opts.ClipboardWriter, label, m.editor.Value())
	}

	before := captureEditorSnapshot(m.editor)
	var cmd tea.Cmd
	m.editor, cmd = m.editor.Update(msg)
	if m.editor.Value() != before.value {
		m.history.RecordBeforeEdit(before, classifyEditKey(msg))
	}
	m.dirty = m.editor.Value() != m.loadedContent
	return m, cmd
}

func (m *Model) applyLineTransform(fn func(text string, line, col int) (string, int, int, bool)) {
	snap := captureEditorSnapshot(m.editor)
	nt, nl, nc, changed := fn(snap.value, snap.line, snap.col)
	if !changed {
		return
	}
	m.history.RecordBeforeEdit(snap, editOpDiscrete)
	m.editor.SetValue(nt)
	setEditorCursor(&m.editor, nl, nc)
	m.dirty = m.editor.Value() != m.loadedContent
}

func classifyEditKey(msg tea.KeyMsg) editOpKind {
	if msg.Paste {
		return editOpDiscrete
	}
	if msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && !msg.Alt && !unicode.IsSpace(msg.Runes[0]) {
		return editOpTyping
	}
	return editOpDiscrete
}

func (m Model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if m.layout.mode == layoutTooSmall {
		return m, nil
	}

	// Wheel scrolling in list modals.
	if m.modal.kind != modalNone {
		switch m.modal.kind {
		case modalNotebook:
			total := len(m.notebooks) + 1
			if msg.Button == tea.MouseButtonWheelDown {
				m.modal.listCursor = (m.modal.listCursor + 1) % total
			} else if msg.Button == tea.MouseButtonWheelUp {
				m.modal.listCursor = (m.modal.listCursor - 1 + total) % total
			}
		case modalInsertTemplate:
			total := len(m.modal.templateNames)
			if total > 0 {
				if msg.Button == tea.MouseButtonWheelDown {
					m.modal.listCursor = (m.modal.listCursor + 1) % total
				} else if msg.Button == tea.MouseButtonWheelUp {
					m.modal.listCursor = (m.modal.listCursor - 1 + total) % total
				}
			}
		}
		return m, nil
	}

	// Wheel scrolling in main panes.
	if msg.Button == tea.MouseButtonWheelUp || msg.Button == tea.MouseButtonWheelDown {
		delta := 3
		if msg.Button == tea.MouseButtonWheelUp {
			delta = -3
		}
		inLeft := m.layout.mode == layoutMasterDetail && msg.X < m.layout.leftWidth
		if m.layout.mode == layoutNarrow && m.focus == focusList {
			inLeft = true
		}
		if inLeft {
			visible := m.listVisibleRows()
			maxOffset := len(m.notes) - visible
			if maxOffset < 0 {
				maxOffset = 0
			}
			m.listOffset += delta
			if m.listOffset < 0 {
				m.listOffset = 0
			}
			if m.listOffset > maxOffset {
				m.listOffset = maxOffset
			}
			return m, nil
		}
		if m.currentNote() != nil {
			steps := delta
			if steps < 0 {
				for i := 0; i < -steps; i++ {
					m.editor.CursorUp()
				}
			} else {
				for i := 0; i < steps; i++ {
					m.editor.CursorDown()
				}
			}
		}
		return m, nil
	}

	// Left-click focus and note selection.
	if msg.Button == tea.MouseButtonLeft && msg.Action == tea.MouseActionPress {
		// Narrow mode tab bar sits on row 2 (margin=0, topBar=1, tabs=2).
		if m.layout.mode == layoutNarrow && msg.Y == 2 {
			if msg.X < 12 {
				if m.focus == focusEditor {
					m.closeFindBar()
					m.focus = focusList
					m.editor.Blur()
					return m, m.flushDirtyCmd()
				}
			} else if m.currentNote() != nil {
				m.focus = focusEditor
				m.editor.Focus()
			}
			return m, nil
		}

		inLeft := (m.layout.mode == layoutMasterDetail && msg.X < m.layout.leftWidth) ||
			(m.layout.mode == layoutNarrow && m.focus == focusList)
		if inLeft {
			firstRowY := 5 // margin(0) + topBar(1) + border(2) + title(3) + search(4) -> row 5
			if m.layout.mode == layoutNarrow {
				firstRowY = 6
			}
			idx := m.listOffset + (msg.Y - firstRowY)
			if idx >= 0 && idx < len(m.notes) {
				if idx == m.selected && m.focus == focusList {
					m.focus = focusEditor
					m.editor.Focus()
					return m, nil
				}
				saveCmd := m.flushDirtyCmd()
				m.selected = idx
				m.clampListScroll()
				m.syncEditorToSelected()
				m.focus = focusList
				m.editor.Blur()
				return m, saveCmd
			}
			if m.focus == focusEditor {
				m.closeFindBar()
				m.focus = focusList
				m.editor.Blur()
				return m, m.flushDirtyCmd()
			}
			return m, nil
		}

		// Right pane (or Zen mode): focus editor if a note is loaded.
		if msg.Y >= 2 && m.currentNote() != nil {
			m.focus = focusEditor
			m.editor.Focus()
		}
	}
	return m, nil
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
