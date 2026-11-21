package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/hmanduca/vault-note-cli/internal/db"
	"github.com/hmanduca/vault-note-cli/internal/vault"
)

type focusPane int

const (
	focusList focusPane = iota
	focusEditor
)

// Options configures TUI initialization.
type Options struct {
	InitialNotebook    string
	DisableCursorBlink bool
}

// Model holds all state for the Elm-architecture Bubble Tea TUI.
type Model struct {
	settingsStore *db.Store
	vault         *vault.Vault
	vaultPath     string

	opts   Options
	styles styles
	layout layoutMetrics

	focus focusPane

	// Data and active filters.
	notes          []vault.Note
	notebooks      []vault.Notebook
	selected       int
	listOffset     int
	filterNotebook string
	filterQuery    string
	searchActive   bool
	searchInput    textinput.Model

	// Right-pane direct TUI editor.
	editor        textarea.Model
	loadedNoteID  string
	loadedContent string
	dirty         bool

	// Encapsulated modal state.
	modal modalState

	// Notifications & errors.
	statusMsg string
	statusErr bool
	quitting  bool
}

// NewModel constructs a fully initialized TUI model.
// If v is nil (no vault configured yet), it automatically opens the Settings
// modal in onboarding mode so the user can configure the vault path immediately.
func NewModel(settingsStore *db.Store, v *vault.Vault, opts Options) Model {
	p := graphitePalette()
	st := newStyles(p)

	ed := textarea.New()
	ed.Placeholder = "Comece a digitar sua anotação... (Ctrl+S salva · Ctrl+E abre $EDITOR · Esc volta à lista)"
	ed.ShowLineNumbers = false
	ed.CharLimit = 0
	ed.Prompt = ""
	applyTextareaTheme(&ed, p)
	if opts.DisableCursorBlink {
		ed.Cursor.SetMode(cursor.CursorStatic)
	}
	ed.Blur()

	search := textinput.New()
	search.Prompt = "/ "
	search.Placeholder = "filtrar..."
	applyTextInputTheme(&search, p)
	if opts.DisableCursorBlink {
		search.Cursor.SetMode(cursor.CursorStatic)
	}

	var vp string
	if v != nil {
		vp = v.Root()
	}

	m := Model{
		settingsStore:  settingsStore,
		vault:          v,
		vaultPath:      vp,
		opts:           opts,
		styles:         st,
		layout:         computeLayout(100, 30),
		focus:          focusList,
		filterNotebook: strings.TrimSpace(opts.InitialNotebook),
		searchInput:    search,
		editor:         ed,
		modal:          modalState{kind: modalNone},
	}
	m.syncEditorSize()
	if m.vault == nil {
		m.openSettingsModal(true)
	}
	return m
}

func (m Model) Init() tea.Cmd {
	if m.vault == nil {
		return nil
	}
	return m.reloadAllCmd("")
}

func (m *Model) currentNote() *vault.Note {
	if m.selected < 0 || m.selected >= len(m.notes) {
		return nil
	}
	return &m.notes[m.selected]
}

func (m *Model) syncEditorToSelected() {
	n := m.currentNote()
	if n == nil {
		m.loadedNoteID = ""
		m.loadedContent = ""
		m.editor.SetValue("")
		m.dirty = false
		return
	}
	m.loadedNoteID = n.ID
	m.loadedContent = n.Content
	m.editor.SetValue(n.Content)
	m.editor.CursorEnd()
	m.dirty = false
}

func (m *Model) syncEditorSize() {
	innerW := m.layout.rightWidth - 4 // border (2) + horizontal padding (2)
	if innerW < 10 {
		innerW = 10
	}
	// Pane border (2) + note metadata header (2) + divider (1) = 5 lines.
	innerH := m.layout.bodyHeight - 5
	if innerH < 3 {
		innerH = 3
	}
	m.editor.SetWidth(innerW)
	m.editor.SetHeight(innerH)

	searchW := m.layout.leftWidth - 7 // border (2) + horizontal padding (2) + prompt (2) + cursor (1)
	if searchW < 6 {
		searchW = 6
	}
	m.searchInput.Width = searchW
}

func (m *Model) clampListScroll() {
	visible := m.listVisibleRows()
	if visible <= 0 {
		m.listOffset = 0
		return
	}
	if m.selected < 0 {
		m.selected = 0
	}
	if len(m.notes) == 0 {
		m.selected = 0
		m.listOffset = 0
		return
	}
	if m.selected >= len(m.notes) {
		m.selected = len(m.notes) - 1
	}
	if m.selected < m.listOffset {
		m.listOffset = m.selected
	}
	if m.selected >= m.listOffset+visible {
		m.listOffset = m.selected - visible + 1
	}
	if m.listOffset < 0 {
		m.listOffset = 0
	}
}

func (m *Model) listVisibleRows() int {
	// border (2) + title (1) + search bar (1) = 4 lines.
	rows := m.layout.bodyHeight - 4
	if rows < 1 {
		return 1
	}
	return rows
}

func (m *Model) setStatus(msg string, isErr bool) {
	m.statusMsg = msg
	m.statusErr = isErr
}

func (m *Model) flushDirtyCmd() tea.Cmd {
	if !m.dirty || m.loadedNoteID == "" || m.vault == nil {
		return nil
	}
	content := m.editor.Value()
	noteID := m.loadedNoteID
	m.loadedContent = content
	m.dirty = false
	return saveNoteContentCmd(m.vault, noteID, content)
}

func (m *Model) formatHeaderBadges() string {
	var parts []string
	if m.filterNotebook != "" {
		parts = append(parts, fmt.Sprintf("[NB: %s]", m.filterNotebook))
	}
	if m.filterQuery != "" {
		parts = append(parts, fmt.Sprintf("[/%s]", m.filterQuery))
	}
	return strings.Join(parts, " ")
}
