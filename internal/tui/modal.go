package tui

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/hmanduca/vault-note-cli/internal/filestore"
	"github.com/hmanduca/vault-note-cli/internal/vault"
)

type modalKind int

const (
	modalNone modalKind = iota
	modalNewNote
	modalEditMeta
	modalDeleteConfirm
	modalNotebook
	modalCreateNotebook
	modalEditNotebook
	modalDeleteNotebookConfirm
	modalEmptyTrashConfirm
	modalInsertTemplate
	modalSaveAsTemplate
	modalConfirmOverwriteTemplate
	modalSettings
	modalConflict
	modalConfirmQuit
	modalHelp
)

type modalState struct {
	kind        modalKind
	inputs      []textinput.Model
	focusIdx    int
	notebookIdx int // 0 = (Raiz / Nenhum), 1..len(notebooks) = m.notebooks[i-1]
	listCursor  int

	// templateNames mirrors vault template names; templateIdx 0 = (sem template).
	templateNames []string
	templateIdx   int

	// Template insertion state (modalInsertTemplate).
	targetNoteID        string
	insertAppend        bool
	templatePreview     []string
	pendingTemplateBody string

	// External conflict resolution state (modalConflict).
	conflictNoteID  string
	conflictContent string

	noteID             string
	notebookName       string
	noteCount          int
	trashCount         int
	trashBytes         int64
	targetLabel        string
	onboarding         bool
	showNotebookPrefix bool
	showLineNumbers    bool
	errText            string
}

func (m *Model) newModalInput(placeholder, value string, charLimit int) textinput.Model {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.SetValue(value)
	ti.CharLimit = charLimit
	inputW := 46
	if m.layout.width > 0 {
		maxW := m.modalBoxWidth() - 10
		if maxW < inputW {
			inputW = max(18, maxW)
		}
	}
	ti.Width = inputW
	applyTextInputTheme(&ti, m.styles.p)
	if m.opts.DisableCursorBlink {
		_ = ti.Cursor.SetMode(cursor.CursorStatic)
	}
	return ti
}

func (m *Model) closeModal() {
	m.modal = modalState{kind: modalNone}
}

func (m *Model) syncModalInputFocus() {
	for i := range m.modal.inputs {
		if i == m.modal.focusIdx {
			m.modal.inputs[i].Focus()
		} else {
			m.modal.inputs[i].Blur()
		}
	}
}

func (m *Model) openNewNoteModal() {
	titleIn := m.newModalInput("Nome da nota .txt (obrigatório)", "", 200)
	titleIn.Focus()

	nbIdx := 0
	if m.filterNotebook != "" {
		for i, nb := range m.notebooks {
			if strings.EqualFold(nb.Name, m.filterNotebook) {
				nbIdx = i + 1
				break
			}
		}
	}

	m.modal = modalState{
		kind:          modalNewNote,
		inputs:        []textinput.Model{titleIn},
		focusIdx:      0,
		notebookIdx:   nbIdx,
		templateNames: m.loadTemplateNames(),
	}
}

// loadTemplateNames fetches template names for the new-note modal, treating
// load failures as "no templates" so the modal still opens.
func (m *Model) loadTemplateNames() []string {
	if m.vault == nil {
		return nil
	}
	tpls, err := m.vault.ListTemplates()
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(tpls))
	for _, tpl := range tpls {
		names = append(names, tpl.Name)
	}
	return names
}

// openInsertTemplateModal opens the template picker for an existing note.
// appendMode inserts at the end of the file; otherwise the snippet goes into
// the editor buffer at the cursor.
func (m *Model) openInsertTemplateModal(noteID string, appendMode bool) {
	names := m.loadTemplateNames()
	previews := make([]string, len(names))
	if m.vault != nil {
		if note := m.noteByID(noteID); note != nil {
			for i, name := range names {
				rendered, err := m.vault.RenderTemplateFor(*note, name)
				if err != nil {
					continue
				}
				previews[i] = firstNonEmptyLine(rendered)
			}
		}
	}
	m.modal = modalState{
		kind:            modalInsertTemplate,
		templateNames:   names,
		templatePreview: previews,
		targetNoteID:    noteID,
		insertAppend:    appendMode,
		listCursor:      0,
	}
}

// currentNoteBody returns the in-memory editor buffer for the loaded note, or
// the on-disk content for another note.
func (m *Model) currentNoteBody(n *vault.Note) string {
	if n == nil {
		return ""
	}
	if m.loadedNoteID == n.ID {
		return m.editor.Value()
	}
	return n.Content
}

// openSaveAsTemplateModal snapshots the current note body into a template.
func (m *Model) openSaveAsTemplateModal(n *vault.Note) {
	if n == nil {
		return
	}
	nameIn := m.newModalInput("Nome do template (sem .txt)", n.Title, 200)
	nameIn.Focus()
	m.modal = modalState{
		kind:                modalSaveAsTemplate,
		inputs:              []textinput.Model{nameIn},
		focusIdx:            0,
		pendingTemplateBody: m.currentNoteBody(n),
	}
}

// firstNonEmptyLine returns the first trimmed non-empty line of s.
func firstNonEmptyLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

// noteByID returns the loaded note with the given ID, if any.
func (m *Model) noteByID(id string) *vault.Note {
	for i := range m.notes {
		if m.notes[i].ID == id {
			return &m.notes[i]
		}
	}
	return nil
}

func (m *Model) openEditMetaModal() {
	n := m.currentNote()
	if n == nil {
		return
	}
	titleIn := m.newModalInput("Nome da nota .txt (obrigatório)", n.Title, 200)
	titleIn.Focus()

	nbIdx := 0
	if n.Notebook != "" {
		for i, nb := range m.notebooks {
			if strings.EqualFold(nb.Name, n.Notebook) {
				nbIdx = i + 1
				break
			}
		}
	}

	m.modal = modalState{
		kind:        modalEditMeta,
		inputs:      []textinput.Model{titleIn},
		focusIdx:    0,
		notebookIdx: nbIdx,
		noteID:      n.ID,
		targetLabel: n.ID,
	}
}

func (m *Model) openDeleteNoteModal() {
	n := m.currentNote()
	if n == nil {
		return
	}
	m.modal = modalState{
		kind:        modalDeleteConfirm,
		noteID:      n.ID,
		targetLabel: n.ID,
	}
}

func (m *Model) openNotebookModal() {
	idx := 0
	for i, nb := range m.notebooks {
		if strings.EqualFold(nb.Name, m.filterNotebook) {
			idx = i + 1
			break
		}
	}
	m.modal = modalState{
		kind:       modalNotebook,
		listCursor: idx,
	}
}

func (m *Model) openCreateNotebookModal() {
	nameIn := m.newModalInput("Nome da pasta / caderno (ex: WORK)", "", 120)
	nameIn.Focus()

	m.modal = modalState{
		kind:     modalCreateNotebook,
		inputs:   []textinput.Model{nameIn},
		focusIdx: 0,
	}
}

func (m *Model) openEditNotebookModal(nb vault.Notebook) {
	nameIn := m.newModalInput("Novo nome da pasta", nb.Name, 120)
	nameIn.Focus()

	m.modal = modalState{
		kind:         modalEditNotebook,
		inputs:       []textinput.Model{nameIn},
		focusIdx:     0,
		notebookName: nb.Name,
		targetLabel:  nb.Name,
	}
}

func (m *Model) openDeleteNotebookModal(nb vault.Notebook) {
	m.modal = modalState{
		kind:         modalDeleteNotebookConfirm,
		notebookName: nb.Name,
		noteCount:    nb.NoteCount,
		targetLabel:  nb.Name,
	}
}

func (m *Model) openEmptyTrashModal() {
	if m.vault == nil {
		return
	}
	items, err := m.vault.ListTrash()
	if err != nil {
		m.setStatus(fmt.Sprintf("Erro ao ler lixeira: %v", err), true)
		return
	}
	if len(items) == 0 {
		m.setStatus("A lixeira (.trash/) já está vazia", false)
		return
	}
	var totalBytes int64
	for _, it := range items {
		totalBytes += it.SizeBytes
	}
	m.modal = modalState{
		kind:       modalEmptyTrashConfirm,
		trashCount: len(items),
		trashBytes: totalBytes,
	}
}

func formatByteSize(n int64) string {
	switch {
	case n >= 1024*1024:
		return fmt.Sprintf("%.1f MB", float64(n)/(1024*1024))
	case n >= 1024:
		return fmt.Sprintf("%.1f KB", float64(n)/1024)
	default:
		return fmt.Sprintf("%d B", n)
	}
}

func (m *Model) openSettingsModal(onboarding bool) {
	m.openSettingsModalWithValue(m.vaultPath, onboarding, "")
}

func (m *Model) openSettingsModalWithValue(initialPath string, onboarding bool, errText string) {
	placeholder := filestore.DefaultVaultSuggestion()
	vaultIn := m.newModalInput(placeholder, initialPath, 500)
	vaultIn.Focus()

	m.modal = modalState{
		kind:               modalSettings,
		inputs:             []textinput.Model{vaultIn},
		focusIdx:           0,
		onboarding:         onboarding,
		showNotebookPrefix: m.showNotebookPrefix,
		showLineNumbers:    m.showLineNumbers,
		errText:            errText,
	}
}

func (m *Model) openConflictModal(noteID, content string) {
	m.modal = modalState{
		kind:            modalConflict,
		conflictNoteID:  noteID,
		conflictContent: content,
		targetLabel:     noteID,
	}
}

func (m *Model) openQuitConfirmModal() {
	m.modal = modalState{kind: modalConfirmQuit}
}

func (m *Model) openHelpModal() {
	m.modal = modalState{kind: modalHelp}
}

// --- Modal Updates ---

func (m Model) updateModal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	k := msg.String()
	if k == "ctrl+c" {
		m.quitting = true
		return m, tea.Quit
	}

	switch m.modal.kind {
	case modalHelp:
		if k == "esc" || k == "?" || k == "q" || k == "enter" {
			m.closeModal()
		}
		return m, nil

	case modalDeleteConfirm:
		switch k {
		case "y", "Y", "enter":
			noteID := m.modal.noteID
			m.closeModal()
			return m, deleteNoteCmd(m.vault, noteID)
		case "n", "N", "esc", "q":
			m.closeModal()
			return m, nil
		}
		return m, nil

	case modalDeleteNotebookConfirm:
		switch k {
		case "y", "Y", "enter":
			name := m.modal.notebookName
			m.closeModal()
			return m, deleteNotebookCmd(m.vault, name)
		case "n", "N", "esc", "q":
			m.closeModal()
			return m, nil
		}
		return m, nil

	case modalEmptyTrashConfirm:
		switch k {
		case "y", "Y", "enter":
			m.closeModal()
			return m, emptyTrashCmd(m.vault)
		case "n", "N", "esc", "q":
			m.closeModal()
			return m, nil
		}
		return m, nil

	case modalInsertTemplate:
		return m.updateInsertTemplateModal(msg)

	case modalSaveAsTemplate:
		return m.updateSaveAsTemplateModal(msg)

	case modalConfirmOverwriteTemplate:
		return m.updateConfirmOverwriteTemplateModal(msg)

	case modalConfirmQuit:
		switch k {
		case "y", "Y", "enter":
			m.closeModal()
			m.quitting = true
			m.saveOnQuit()
			return m, tea.Quit
		case "n", "N", "esc":
			m.closeModal()
			return m, nil
		}
		return m, nil

	case modalConflict:
		return m.updateConflictModal(msg)

	case modalNewNote, modalEditMeta:
		return m.updateNoteFormModal(msg)

	case modalNotebook:
		return m.updateNotebookModal(msg)

	case modalCreateNotebook, modalEditNotebook:
		return m.updateNotebookFormModal(msg)

	case modalSettings:
		return m.updateSettingsModal(msg)
	}
	return m, nil
}

func (m Model) updateNoteFormModal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Fields: 0=Título (input[0]), 1=Caderno (selector), 2=Template (selector, new note only)
	totalFields := 2
	if m.modal.kind == modalNewNote {
		totalFields = 3
	}
	k := msg.String()

	switch k {
	case "esc":
		m.closeModal()
		return m, nil
	case "tab", "down":
		m.modal.focusIdx = (m.modal.focusIdx + 1) % totalFields
		m.syncModalInputFocus()
		return m, nil
	case "shift+tab", "up":
		m.modal.focusIdx = (m.modal.focusIdx - 1 + totalFields) % totalFields
		m.syncModalInputFocus()
		return m, nil
	case "left", "h":
		if m.modal.focusIdx == 1 {
			total := len(m.notebooks) + 1
			m.modal.notebookIdx = (m.modal.notebookIdx - 1 + total) % total
			return m, nil
		}
		if m.modal.focusIdx == 2 && m.modal.kind == modalNewNote {
			total := len(m.modal.templateNames) + 1
			m.modal.templateIdx = (m.modal.templateIdx - 1 + total) % total
			return m, nil
		}
	case "right", "l", " ":
		if m.modal.focusIdx == 1 {
			total := len(m.notebooks) + 1
			m.modal.notebookIdx = (m.modal.notebookIdx + 1) % total
			return m, nil
		}
		if m.modal.focusIdx == 2 && m.modal.kind == modalNewNote {
			total := len(m.modal.templateNames) + 1
			m.modal.templateIdx = (m.modal.templateIdx + 1) % total
			return m, nil
		}
	case "enter":
		title := strings.TrimSpace(m.modal.inputs[0].Value())
		if title == "" {
			m.modal.errText = "Título é obrigatório"
			m.modal.focusIdx = 0
			m.syncModalInputFocus()
			return m, nil
		}
		nbName := ""
		if m.modal.notebookIdx > 0 && m.modal.notebookIdx-1 < len(m.notebooks) {
			nbName = m.notebooks[m.modal.notebookIdx-1].Name
		}

		if m.modal.kind == modalNewNote {
			tplName := ""
			if m.modal.templateIdx > 0 && m.modal.templateIdx-1 < len(m.modal.templateNames) {
				tplName = m.modal.templateNames[m.modal.templateIdx-1]
			}
			m.closeModal()
			return m, createNoteCmd(m.vault, nbName, title, tplName)
		}
		noteID := m.modal.noteID
		m.closeModal()
		return m, updateNoteMetaCmd(m.vault, noteID, title, nbName)
	}

	if m.modal.focusIdx < len(m.modal.inputs) {
		var cmd tea.Cmd
		m.modal.inputs[m.modal.focusIdx], cmd = m.modal.inputs[m.modal.focusIdx].Update(msg)
		m.modal.errText = ""
		return m, cmd
	}
	return m, nil
}

// updateInsertTemplateModal navigates the template picker and inserts on Enter.
func (m Model) updateInsertTemplateModal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	total := len(m.modal.templateNames)
	switch msg.String() {
	case "esc", "q":
		m.closeModal()
		return m, nil
	case "j", "down":
		if total > 0 {
			m.modal.listCursor = (m.modal.listCursor + 1) % total
		}
		return m, nil
	case "k", "up":
		if total > 0 {
			m.modal.listCursor = (m.modal.listCursor - 1 + total) % total
		}
		return m, nil
	case "enter":
		if total == 0 || m.vault == nil {
			return m, nil
		}
		tplName := m.modal.templateNames[m.modal.listCursor]
		note := m.noteByID(m.modal.targetNoteID)
		if note == nil {
			m.modal.errText = "Nota alvo não encontrada"
			return m, nil
		}
		rendered, err := m.vault.RenderTemplateFor(*note, tplName)
		if err != nil {
			m.modal.errText = err.Error()
			return m, nil
		}
		return m.performInsertTemplate(tplName, rendered)
	}
	return m, nil
}

// updateSaveAsTemplateModal asks for a name and snapshots the note body.
func (m Model) updateSaveAsTemplateModal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.closeModal()
		return m, nil
	case "enter":
		name := strings.TrimSpace(m.modal.inputs[0].Value())
		if name == "" {
			m.modal.errText = "informe um nome para o template"
			return m, nil
		}
		m.modal.errText = ""
		return m, saveAsTemplateCmd(m.vault, name, m.modal.pendingTemplateBody, false)
	}

	var cmd tea.Cmd
	m.modal.inputs[0], cmd = m.modal.inputs[0].Update(msg)
	return m, cmd
}

// updateConfirmOverwriteTemplateModal confirms overwriting an existing template.
func (m Model) updateConfirmOverwriteTemplateModal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y", "enter":
		name := strings.TrimSpace(m.modal.inputs[0].Value())
		return m, saveAsTemplateCmd(m.vault, name, m.modal.pendingTemplateBody, true)
	case "n", "N", "esc", "q":
		m.modal.kind = modalSaveAsTemplate
		m.modal.errText = ""
		m.syncModalInputFocus()
		return m, nil
	}
	return m, nil
}

// performInsertTemplate inserts into the editor buffer at the cursor, or appends
// to the note on disk when the picker was opened from the list.
func (m Model) performInsertTemplate(tplName, rendered string) (tea.Model, tea.Cmd) {
	noteID := m.modal.targetNoteID
	appendMode := m.modal.insertAppend
	m.closeModal()

	if appendMode {
		m.focus = focusList
		m.editor.Blur()
		return m, insertTemplateCmd(m.vault, noteID, tplName)
	}

	m.history.RecordBeforeEdit(captureEditorSnapshot(m.editor), editOpDiscrete)
	m.editor.InsertString(rendered)
	m.dirty = m.editor.Value() != m.loadedContent
	m.focus = focusEditor
	m.editor.Focus()
	m.setStatus(fmt.Sprintf("Template %s inserido (Ctrl+S para salvar)", tplName), false)
	return m, nil
}

func (m Model) updateNotebookModal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	total := len(m.notebooks) + 1 // 0 = Todos
	k := msg.String()

	switch k {
	case "esc", "q":
		m.closeModal()
		return m, nil
	case "j", "down":
		m.modal.listCursor = (m.modal.listCursor + 1) % total
		return m, nil
	case "k", "up":
		m.modal.listCursor = (m.modal.listCursor - 1 + total) % total
		return m, nil
	case "n", "c":
		m.openCreateNotebookModal()
		return m, nil
	case "u", "e":
		if m.modal.listCursor > 0 && m.modal.listCursor-1 < len(m.notebooks) {
			m.openEditNotebookModal(m.notebooks[m.modal.listCursor-1])
		}
		return m, nil
	case "d", "x":
		if m.modal.listCursor > 0 && m.modal.listCursor-1 < len(m.notebooks) {
			m.openDeleteNotebookModal(m.notebooks[m.modal.listCursor-1])
		}
		return m, nil
	case "enter":
		if m.modal.listCursor == 0 {
			m.filterNotebook = ""
		} else if m.modal.listCursor-1 < len(m.notebooks) {
			m.filterNotebook = m.notebooks[m.modal.listCursor-1].Name
		}
		m.closeModal()
		return m, m.reloadAllCmd("")
	}

	if len(k) == 1 && k[0] >= '1' && k[0] <= '9' {
		idx := int(k[0] - '1')
		if idx < total {
			m.modal.listCursor = idx
			if idx == 0 {
				m.filterNotebook = ""
			} else {
				m.filterNotebook = m.notebooks[idx-1].Name
			}
			m.closeModal()
			return m, m.reloadAllCmd("")
		}
	}
	return m, nil
}

func (m Model) updateNotebookFormModal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	k := msg.String()

	switch k {
	case "esc":
		m.closeModal()
		return m, nil
	case "enter":
		name := strings.TrimSpace(m.modal.inputs[0].Value())
		if name == "" {
			m.modal.errText = "Nome da pasta é obrigatório"
			m.modal.focusIdx = 0
			m.syncModalInputFocus()
			return m, nil
		}
		if strings.HasPrefix(name, ".") || strings.ContainsAny(name, `/\`) {
			m.modal.errText = "Nome inválido (não pode iniciar com '.' ou conter '/')"
			m.modal.focusIdx = 0
			m.syncModalInputFocus()
			return m, nil
		}
		if strings.IndexFunc(name, unicode.IsSpace) >= 0 {
			m.modal.errText = "Nome do caderno deve ter apenas uma palavra (sem espaços)"
			m.modal.focusIdx = 0
			m.syncModalInputFocus()
			return m, nil
		}
		name = strings.ToUpper(name)
		if m.modal.kind == modalCreateNotebook {
			m.closeModal()
			return m, createNotebookCmd(m.vault, name)
		}
		oldName := m.modal.notebookName
		m.closeModal()
		return m, renameNotebookCmd(m.vault, oldName, name)
	}

	if len(m.modal.inputs) > 0 {
		var cmd tea.Cmd
		m.modal.inputs[0], cmd = m.modal.inputs[0].Update(msg)
		m.modal.errText = ""
		return m, cmd
	}
	return m, nil
}

func (m Model) updateSettingsModal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Fields: 0=vault_path, 1=show_notebook_prefix, 2=show_line_numbers
	const totalFields = 3
	k := msg.String()

	switch k {
	case "esc":
		m.closeModal()
		return m, nil
	case "tab", "down":
		m.modal.focusIdx = (m.modal.focusIdx + 1) % totalFields
		m.syncModalInputFocus()
		return m, nil
	case "shift+tab", "up":
		m.modal.focusIdx = (m.modal.focusIdx - 1 + totalFields) % totalFields
		m.syncModalInputFocus()
		return m, nil
	case "enter":
		rawPath := strings.TrimSpace(m.modal.inputs[0].Value())
		if rawPath == "" {
			m.modal.errText = "Caminho do vault é obrigatório"
			m.modal.focusIdx = 0
			m.syncModalInputFocus()
			return m, nil
		}
		return m, saveSettingsCmd(m.settingsStore, rawPath, m.modal.showNotebookPrefix, m.modal.showLineNumbers)
	}

	if m.modal.focusIdx == 1 {
		switch k {
		case " ", "left", "right", "h", "l":
			m.modal.showNotebookPrefix = !m.modal.showNotebookPrefix
			return m, nil
		}
		return m, nil
	}

	if m.modal.focusIdx == 2 {
		switch k {
		case " ", "left", "right", "h", "l":
			m.modal.showLineNumbers = !m.modal.showLineNumbers
			return m, nil
		}
		return m, nil
	}

	if len(m.modal.inputs) > 0 {
		var cmd tea.Cmd
		m.modal.inputs[0], cmd = m.modal.inputs[0].Update(msg)
		m.modal.errText = ""
		return m, cmd
	}
	return m, nil
}

func (m Model) updateConflictModal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	noteID := m.modal.conflictNoteID
	content := m.modal.conflictContent
	switch msg.String() {
	case "o", "O":
		m.closeModal()
		return m, saveNoteContentCmd(m.vault, noteID, content)
	case "r", "R":
		m.closeModal()
		if m.loadedNoteID == noteID {
			m.dirty = false
		}
		m.setStatus("Recarregado do disco: "+noteID, false)
		return m, m.reloadAllCmd(noteID)
	case "c", "C":
		m.closeModal()
		return m, createConflictCopyCmd(m.vault, noteID, content)
	case "esc":
		m.closeModal()
		return m, nil
	}
	return m, nil
}

// --- Modal Rendering ---

func (m Model) renderModal() string {
	var b strings.Builder

	switch m.modal.kind {
	case modalNewNote, modalEditMeta:
		title := "Nova Nota (.txt)"
		if m.modal.kind == modalEditMeta {
			title = fmt.Sprintf("Mover / Renomear %s", truncateDisplay(m.modal.targetLabel, 34))
		}
		b.WriteString(m.styles.modalTitle.Render(title))
		b.WriteString("\n\n")

		b.WriteString(m.renderModalFieldLabel("Título (arquivo .txt)", m.modal.focusIdx == 0))
		b.WriteString("\n")
		b.WriteString(m.modal.inputs[0].View())
		b.WriteString("\n\n")

		nbLabel := "(Raiz / Sem caderno)"
		if m.modal.notebookIdx > 0 && m.modal.notebookIdx-1 < len(m.notebooks) {
			nbLabel = m.notebooks[m.modal.notebookIdx-1].Name
		}
		b.WriteString(m.renderModalFieldLabel("Caderno (pasta)", m.modal.focusIdx == 1))
		b.WriteString("\n")
		b.WriteString(m.renderSelectorValue(nbLabel, m.modal.focusIdx == 1))

		if m.modal.kind == modalNewNote {
			tplLabel := "(sem template)"
			if m.modal.templateIdx > 0 && m.modal.templateIdx-1 < len(m.modal.templateNames) {
				tplLabel = m.modal.templateNames[m.modal.templateIdx-1]
			}
			b.WriteString("\n\n")
			b.WriteString(m.renderModalFieldLabel("Template", m.modal.focusIdx == 2))
			b.WriteString("\n")
			b.WriteString(m.renderSelectorValue(tplLabel, m.modal.focusIdx == 2))
		}

		if m.modal.errText != "" {
			b.WriteString("\n\n")
			b.WriteString(m.styles.errText.Render("Erro: " + m.modal.errText))
		}
		b.WriteString("\n\n")
		b.WriteString(m.styles.modalHint.Render("Tab/↑↓: campo  ←/→: opção  Enter: salvar  Esc: cancelar"))

	case modalDeleteConfirm:
		b.WriteString(m.styles.abortBanner.Render("Mover Nota para Lixeira (.trash/)"))
		b.WriteString("\n\n")
		b.WriteString(fmt.Sprintf("Mover %s para .trash/?\n", truncateDisplay(m.modal.targetLabel, 48)))
		b.WriteString(m.styles.muted.Render("O arquivo .txt será preservado em <vault>/.trash/ para recuperação manual."))
		b.WriteString("\n\n")
		b.WriteString(m.styles.modalHint.Render("y/Enter: mover para lixeira  n/Esc: cancelar"))

	case modalNotebook:
		b.WriteString(m.styles.modalTitle.Render("Cadernos (Pastas do Vault)"))
		b.WriteString("\n\n")

		items := make([]string, 0, len(m.notebooks)+1)
		items = append(items, "[1] Todos os cadernos")
		for i, nb := range m.notebooks {
			prefix := "   "
			if i+2 <= 9 {
				prefix = fmt.Sprintf("[%d]", i+2)
			}
			items = append(items, fmt.Sprintf("%s %-24s (%d .txt)", prefix, nb.Name, nb.NoteCount))
		}
		for i, item := range items {
			row := truncateDisplay(item, m.modalBoxWidth()-8)
			if i == m.modal.listCursor {
				b.WriteString(m.styles.itemSelect.Render("▸ "+row) + "\n")
			} else {
				b.WriteString("  " + row + "\n")
			}
		}
		b.WriteString("\n")
		b.WriteString(m.styles.modalHint.Render("j/k ou 1-9: filtrar  n: nova pasta  u: renomear  d: lixeira  Esc: fechar"))

	case modalCreateNotebook:
		b.WriteString(m.styles.modalTitle.Render("Novo Caderno (Pasta)"))
		b.WriteString("\n\n")
		b.WriteString(m.renderModalFieldLabel("Nome da Pasta", m.modal.focusIdx == 0) + "\n" + m.modal.inputs[0].View())
		if m.modal.errText != "" {
			b.WriteString("\n\n" + m.styles.errText.Render("Erro: "+m.modal.errText))
		}
		b.WriteString("\n\n" + m.styles.modalHint.Render("Enter: criar pasta  Esc: cancelar"))

	case modalEditNotebook:
		b.WriteString(m.styles.modalTitle.Render(fmt.Sprintf("Renomear Caderno %s", m.modal.targetLabel)))
		b.WriteString("\n\n")
		b.WriteString(m.renderModalFieldLabel("Novo Nome da Pasta", m.modal.focusIdx == 0) + "\n" + m.modal.inputs[0].View())
		if m.modal.errText != "" {
			b.WriteString("\n\n" + m.styles.errText.Render("Erro: "+m.modal.errText))
		}
		b.WriteString("\n\n" + m.styles.modalHint.Render("Enter: renomear  Esc: cancelar"))

	case modalInsertTemplate:
		b.WriteString(m.styles.modalTitle.Render("Inserir Template"))
		b.WriteString("\n\n")
		if len(m.modal.templateNames) == 0 {
			b.WriteString(m.styles.muted.Render("Nenhum template em <vault>/.templates/"))
			b.WriteString("\n")
			b.WriteString(m.styles.muted.Render("Crie com 'vault-note-cli template add <nome>'"))
			b.WriteString("\n\n")
			b.WriteString(m.styles.modalHint.Render("Esc: cancelar"))
			break
		}
		for i, name := range m.modal.templateNames {
			row := truncateDisplay(name, m.modalBoxWidth()-8)
			if i == m.modal.listCursor {
				b.WriteString(m.styles.itemSelect.Render("▸ "+row) + "\n")
			} else {
				b.WriteString("  " + row + "\n")
			}
		}
		if m.modal.listCursor < len(m.modal.templatePreview) && m.modal.templatePreview[m.modal.listCursor] != "" {
			b.WriteString("\n")
			b.WriteString(m.styles.muted.Render(truncateDisplay(m.modal.templatePreview[m.modal.listCursor], m.modalBoxWidth()-8)))
		}
		if m.modal.errText != "" {
			b.WriteString("\n\n")
			b.WriteString(m.styles.errText.Render("Erro: " + m.modal.errText))
		}
		b.WriteString("\n\n")
		b.WriteString(m.styles.modalHint.Render("j/k: navegar  Enter: inserir  Esc: cancelar"))

	case modalSaveAsTemplate:
		b.WriteString(m.styles.modalTitle.Render("Salvar Nota como Template"))
		b.WriteString("\n\n")
		b.WriteString(m.styles.muted.Render("Copia o conteúdo atual sem alterar a nota original."))
		b.WriteString("\n\n")
		b.WriteString(m.renderModalFieldLabel("Nome do template", m.modal.focusIdx == 0) + "\n" + m.modal.inputs[0].View())
		if m.modal.errText != "" {
			b.WriteString("\n\n" + m.styles.errText.Render("Erro: "+m.modal.errText))
		}
		b.WriteString("\n\n" + m.styles.modalHint.Render("Enter salvar  Esc cancelar"))

	case modalConfirmOverwriteTemplate:
		b.WriteString(m.styles.abortBanner.Render("Template já existe"))
		b.WriteString("\n\n")
		b.WriteString("Sobrescrever o template " + truncateDisplay(m.modal.inputs[0].Value(), 36) + "?\n")
		b.WriteString(m.styles.muted.Render("O conteúdo anterior será substituído."))
		b.WriteString("\n\n")
		b.WriteString(m.styles.modalHint.Render("y/Enter sobrescrever  n/Esc voltar"))

	case modalDeleteNotebookConfirm:
		b.WriteString(m.styles.abortBanner.Render("Mover Caderno para Lixeira (.trash/)"))
		b.WriteString("\n\n")
		b.WriteString(fmt.Sprintf("Mover pasta %s (%d nota(s)) para .trash/?\n", truncateDisplay(m.modal.targetLabel, 36), m.modal.noteCount))
		b.WriteString(m.styles.muted.Render("A pasta e seus arquivos serão preservados dentro de <vault>/.trash/."))
		b.WriteString("\n\n")
		b.WriteString(m.styles.modalHint.Render("y/Enter: mover para lixeira  n/Esc: cancelar"))

	case modalEmptyTrashConfirm:
		b.WriteString(m.styles.abortBanner.Render("Esvaziar Lixeira (.trash/)"))
		b.WriteString("\n\n")
		b.WriteString(fmt.Sprintf("Apagar permanentemente %d item(ns) (%s) em .trash/?\n", m.modal.trashCount, formatByteSize(m.modal.trashBytes)))
		b.WriteString(m.styles.muted.Render("Esta ação é irreversível — os arquivos em <vault>/.trash/ não poderão ser recuperados."))
		b.WriteString("\n\n")
		b.WriteString(m.styles.modalHint.Render("y/Enter: esvaziar lixeira  n/Esc: cancelar"))

	case modalSettings:
		title := "Configurações — vault-note-cli"
		if m.modal.onboarding {
			title = "Configuração Inicial — vault-note-cli"
		}
		b.WriteString(m.styles.modalTitle.Render(title))
		b.WriteString("\n\n")
		b.WriteString(m.styles.muted.Render("Pastas de 1º nível = cadernos · Arquivos .txt = notas."))
		b.WriteString("\n")
		b.WriteString(m.styles.muted.Render("A pasta será criada automaticamente se não existir."))
		b.WriteString("\n\n")

		b.WriteString(m.renderModalFieldLabel("Caminho do Vault", m.modal.focusIdx == 0))
		b.WriteString("\n")
		b.WriteString(m.modal.inputs[0].View())
		b.WriteString("\n\n")
		b.WriteString(m.renderModalFieldLabel("Prefixo do caderno na lista", m.modal.focusIdx == 1))
		b.WriteString("\n")
		prefixState := "[ ] Desativado"
		if m.modal.showNotebookPrefix {
			prefixState = "[✓] Ativado (máx. 3 letras)"
		}
		b.WriteString(prefixState)
		b.WriteString("\n\n")
		b.WriteString(m.renderModalFieldLabel("Números de linha no editor", m.modal.focusIdx == 2))
		b.WriteString("\n")
		lineNumState := "[ ] Desativado"
		if m.modal.showLineNumbers {
			lineNumState = "[✓] Ativado"
		}
		b.WriteString(lineNumState)

		if m.settingsStore != nil && m.settingsStore.Path() != "" {
			b.WriteString("\n\n")
			b.WriteString(m.styles.subtle.Render(truncateDisplay("Arquivo: "+m.settingsStore.Path(), m.modalBoxWidth()-8)))
		}
		if m.modal.errText != "" {
			b.WriteString("\n\n")
			b.WriteString(m.styles.errText.Render("Erro: " + m.modal.errText))
		}
		b.WriteString("\n\n")
		b.WriteString(m.styles.modalHint.Render("Tab/↑↓: campo  ←/→/Space: alternar  Enter: salvar  Esc: fechar"))

	case modalConflict:
		b.WriteString(m.styles.abortBanner.Render("Conflito Externo Detectado"))
		b.WriteString("\n\n")
		b.WriteString(fmt.Sprintf("O arquivo %s foi alterado fora do app.\n", truncateDisplay(m.modal.targetLabel, 42)))
		b.WriteString(m.styles.muted.Render("Escolha como resolver sem perder alterações:"))
		b.WriteString("\n\n")
		b.WriteString("  " + m.styles.accent.Render("[o]") + " Sobrescrever arquivo no disco com o buffer atual\n")
		b.WriteString("  " + m.styles.accent.Render("[r]") + " Recarregar versão do disco (descarta buffer local)\n")
		b.WriteString("  " + m.styles.accent.Render("[c]") + " Salvar cópia de conflito e manter original intacto\n")
		b.WriteString("\n")
		b.WriteString(m.styles.modalHint.Render("o: sobrescrever  r: recarregar  c: salvar cópia  Esc: cancelar"))

	case modalConfirmQuit:
		b.WriteString(m.styles.modalTitle.Render("Sair do vault-note-cli"))
		b.WriteString("\n\n")
		b.WriteString("Deseja realmente encerrar o aplicativo?\n")
		if m.dirty {
			b.WriteString(m.styles.dirty.Render("Alterações pendentes serão salvas no arquivo .txt ao sair."))
		} else {
			b.WriteString(m.styles.muted.Render("Todas as notas já estão salvas no vault."))
		}
		b.WriteString("\n\n")
		b.WriteString(m.styles.modalHint.Render("y/Enter: sair  n/Esc: cancelar"))

	case modalHelp:
		b.WriteString(m.styles.modalTitle.Render("Atalhos de Teclado — vault-note-cli"))
		b.WriteString("\n\n")
		rows := [][2]string{
			{"j / k / ↑ / ↓ / g / G", "Navegar na lista de notas (20% esquerda)"},
			{"Enter / Tab / l", "Focar editor TUI (80% direita) pronto para digitar"},
			{"n / m / d / D", "Nova nota / Mover / Lixeira / Esvaziar .trash/"},
			{"E / Y / z", "Lista: $EDITOR / Copiar nota / Modo Zen (100%)"},
			{"/ / b / c / r", "Filtrar / Cadernos / Configurações / Recarregar"},
			{"Ctrl+S / Ctrl+E", "Editor: salvar no .txt / abrir buffer no $EDITOR"},
			{"Ctrl+Z / Ctrl+Y", "Editor: desfazer / refazer"},
			{"Ctrl+F / Ctrl+R", "Editor: buscar / buscar e substituir na nota"},
			{"Alt+X", "Editor: alternar checkbox (- [ ] ↔ - [x])"},
			{"Alt+↑ / Alt+↓", "Editor: mover linha para cima / baixo"},
			{"Alt+P / Alt+K", "Editor: duplicar linha / excluir linha"},
			{"Tab / Alt+[ / Alt+]", "Editor: indentar / desindentar linha"},
			{"Alt+Z / Alt+N", "Editor: Modo Zen (100%) / números de linha"},
			{"Alt+Y / Alt+W", "Editor: copiar linha / nota inteira (clipboard)"},
			{"Ctrl+T / T", "Inserir template (cursor / fim)"},
			{"S / Alt+T", "Salvar nota ou buffer como template"},
			{"Esc / Shift+Tab", "Salvar alterações pendentes e voltar à lista"},
			{"? / q / Ctrl+C", "Abrir ajuda / Salvar e sair"},
		}
		for _, r := range rows {
			b.WriteString(fmt.Sprintf("%s  %s\n",
				m.styles.accent.Render(fmt.Sprintf("%-24s", r[0])),
				r[1],
			))
		}
		b.WriteString("\n" + m.styles.modalHint.Render("Pressione Esc, ? ou Enter para fechar"))
	}

	return m.styles.modalBox.Width(m.modalBoxWidth()).Render(b.String())
}

func (m Model) modalBoxWidth() int {
	boxWidth := 62
	if m.modal.kind == modalHelp {
		boxWidth = 78
	}
	if m.layout.width > 0 && m.layout.width-6 < boxWidth {
		boxWidth = max(32, m.layout.width-6)
	}
	return boxWidth
}

func (m Model) renderModalFieldLabel(label string, focused bool) string {
	if focused {
		return m.styles.accent.Render("▸ " + label)
	}
	return m.styles.modalLabel.Render("  " + label)
}

func (m Model) renderSelectorValue(val string, focused bool) string {
	text := fmt.Sprintf("◀ %s ▶", val)
	if focused {
		return m.styles.itemSelect.Render(" " + text + " ")
	}
	return m.styles.normalRow.Render(" " + text + " ")
}

// overlayModal floats the active modal centered over a dimmed rendering of the base workspace.
func (m Model) overlayModal(base string) string {
	modal := m.renderModal()
	return compositeFloatingModal(base, modal, m.layout.width, m.layout.height, m.styles.scrim)
}

// compositeFloatingModal overlays modal centered on top of base, dimming the visible
// background workspace around the floating dialog while preserving exact visual alignment
// even across wide Unicode graphemes (CJK/emojis).
func compositeFloatingModal(base, modal string, width, height int, backdrop lipgloss.Style) string {
	bgLines := strings.Split(base, "\n")
	fgLines := strings.Split(modal, "\n")

	modalW := lipgloss.Width(modal)
	modalH := len(fgLines)

	w := width
	if w <= 0 {
		for _, line := range bgLines {
			if lw := ansi.StringWidth(line); lw > w {
				w = lw
			}
		}
		if modalW > w {
			w = modalW
		}
		if w <= 0 {
			w = 80
		}
	}

	h := height
	if h <= 0 {
		h = max(len(bgLines), modalH)
		if h <= 0 {
			h = 24
		}
	}

	if w < modalW || h < modalH {
		return lipgloss.Place(max(w, modalW), max(h, modalH), lipgloss.Center, lipgloss.Center, modal)
	}

	if len(bgLines) > h {
		bgLines = bgLines[:h]
	}
	for len(bgLines) < h {
		bgLines = append(bgLines, "")
	}

	startX := (w - modalW) / 2
	startY := (h - modalH) / 2

	out := make([]string, h)
	for row := 0; row < h; row++ {
		plainBg := ansi.Strip(bgLines[row])
		if bgW := ansi.StringWidth(plainBg); bgW < w {
			plainBg += strings.Repeat(" ", w-bgW)
		} else if bgW > w {
			plainBg = ansi.Cut(plainBg, 0, w)
			if cutW := ansi.StringWidth(plainBg); cutW < w {
				plainBg += strings.Repeat(" ", w-cutW)
			}
		}

		if row < startY || row >= startY+modalH {
			out[row] = backdrop.Render(plainBg)
			continue
		}

		fgLine := fgLines[row-startY]
		fgW := ansi.StringWidth(fgLine)
		if fgW < modalW {
			fgLine += strings.Repeat(" ", modalW-fgW)
			fgW = modalW
		}

		left := ansi.Cut(plainBg, 0, startX)
		if lw := ansi.StringWidth(left); lw < startX {
			left += strings.Repeat(" ", startX-lw)
		}

		rightStart := startX + fgW
		right := ""
		if rightStart < w {
			right = ansi.Cut(plainBg, rightStart, w)
			if rw := ansi.StringWidth(right); rw < w-rightStart {
				right = strings.Repeat(" ", (w-rightStart)-rw) + right
			}
		}

		var lineBuilder strings.Builder
		if startX > 0 {
			lineBuilder.WriteString(backdrop.Render(left))
		}
		lineBuilder.WriteString("\x1b[0m")
		lineBuilder.WriteString(fgLine)
		lineBuilder.WriteString("\x1b[0m")
		if rightStart < w {
			lineBuilder.WriteString(backdrop.Render(right))
		}
		out[row] = lineBuilder.String()
	}

	return strings.Join(out, "\n")
}

func padOrTruncatePlain(s string, width int) string {
	w := ansi.StringWidth(s)
	if w > width {
		s = ansi.Truncate(s, width, "")
		w = ansi.StringWidth(s)
	}
	if w < width {
		s += strings.Repeat(" ", width-w)
	}
	return s
}

func truncateDisplay(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	if ansi.StringWidth(s) <= maxWidth {
		return s
	}
	if maxWidth == 1 {
		return "…"
	}
	return ansi.Truncate(s, maxWidth, "…")
}
