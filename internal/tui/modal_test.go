package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func TestNewNoteModalCreatesTxtAndFocusesEditor(t *testing.T) {
	store, v := newTestEnv(t)
	if _, err := v.CreateNotebook("WORK"); err != nil {
		t.Fatalf("CreateNotebook: %v", err)
	}

	m := NewModel(store, v, Options{DisableCursorBlink: true})
	m = runCmd(m, m.Init())
	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = next.(Model)

	m = sendKey(t, m, "n")
	if m.modal.kind != modalNewNote {
		t.Fatalf("expected modalNewNote, got %v", m.modal.kind)
	}
	m = typeText(t, m, "Plano")
	// Select WORK notebook (Tab -> Right)
	m = sendKey(t, m, "tab")
	m = sendKey(t, m, "right")
	m = sendKey(t, m, "enter")

	if m.modal.kind != modalNone {
		t.Fatalf("expected modal closed, got %v", m.modal.kind)
	}
	if m.focus != focusEditor {
		t.Fatalf("expected focusEditor after creating note")
	}
	if len(m.notes) != 1 || m.notes[0].ID != "WORK/Plano.txt" {
		t.Fatalf("unexpected notes: %+v", m.notes)
	}
}

func TestNotebookModalCreateRenameAndFilter(t *testing.T) {
	store, v := newTestEnv(t)
	m := NewModel(store, v, Options{DisableCursorBlink: true})
	m = runCmd(m, m.Init())
	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = next.(Model)

	// Open notebooks modal -> 'n' create -> "DEV" -> Enter.
	m = sendKey(t, m, "b")
	if m.modal.kind != modalNotebook {
		t.Fatalf("expected modalNotebook, got %v", m.modal.kind)
	}
	m = sendKey(t, m, "n")
	if m.modal.kind != modalCreateNotebook {
		t.Fatalf("expected modalCreateNotebook, got %v", m.modal.kind)
	}
	m = typeText(t, m, "DEV")
	m = sendKey(t, m, "enter")

	if m.filterNotebook != "DEV" || len(m.notebooks) != 1 || m.notebooks[0].Name != "DEV" {
		t.Fatalf("unexpected notebook state: filter=%q nbs=%+v", m.filterNotebook, m.notebooks)
	}

	// Open notebooks modal again -> cursor is on DEV (idx 1) -> 'u' rename -> "BACKEND".
	m = sendKey(t, m, "b")
	m = sendKey(t, m, "u")
	if m.modal.kind != modalEditNotebook {
		t.Fatalf("expected modalEditNotebook, got %v", m.modal.kind)
	}
	m.modal.inputs[0].SetValue("BACKEND")
	m = sendKey(t, m, "enter")

	if m.filterNotebook != "BACKEND" || len(m.notebooks) != 1 || m.notebooks[0].Name != "BACKEND" {
		t.Fatalf("expected renamed notebook BACKEND, got filter=%q nbs=%+v", m.filterNotebook, m.notebooks)
	}
}

func TestNotebookModalNormalizesUppercaseAndRejectsMultiWord(t *testing.T) {
	store, v := newTestEnv(t)
	m := NewModel(store, v, Options{DisableCursorBlink: true})
	m = runCmd(m, m.Init())
	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = next.(Model)

	// Open create notebook modal -> multi-word rejected inline.
	m = sendKey(t, m, "b")
	m = sendKey(t, m, "n")
	m.modal.inputs[0].SetValue("meu caderno")
	m = sendKey(t, m, "enter")
	if m.modal.kind != modalCreateNotebook || !strings.Contains(m.modal.errText, "uma palavra") {
		t.Fatalf("expected inline single-word error, got kind=%v err=%q", m.modal.kind, m.modal.errText)
	}

	// Trimmed lowercase creates UPPERCASE notebook.
	m.modal.inputs[0].SetValue("  financas  ")
	m = sendKey(t, m, "enter")
	if m.modal.kind != modalNone || len(m.notebooks) != 1 || m.notebooks[0].Name != "FINANCAS" {
		t.Fatalf("expected FINANCAS created, got modal=%v nbs=%+v", m.modal.kind, m.notebooks)
	}

	// Rename multi-word rejected inline; lowercase normalized to UPPERCASE.
	m = sendKey(t, m, "b")
	m = sendKey(t, m, "u")
	m.modal.inputs[0].SetValue("duas palavras")
	m = sendKey(t, m, "enter")
	if m.modal.kind != modalEditNotebook || !strings.Contains(m.modal.errText, "uma palavra") {
		t.Fatalf("expected inline single-word error on rename, got kind=%v err=%q", m.modal.kind, m.modal.errText)
	}
	m.modal.inputs[0].SetValue("  contas  ")
	m = sendKey(t, m, "enter")
	if m.modal.kind != modalNone || len(m.notebooks) != 1 || m.notebooks[0].Name != "CONTAS" {
		t.Fatalf("expected CONTAS after rename, got modal=%v nbs=%+v", m.modal.kind, m.notebooks)
	}
}

func TestSettingsModalSwitchVaultAtRuntime(t *testing.T) {
	store, v := newTestEnv(t)
	if _, err := v.CreateNote("", "antiga", "v1"); err != nil {
		t.Fatalf("CreateNote: %v", err)
	}

	m := NewModel(store, v, Options{DisableCursorBlink: true})
	m = runCmd(m, m.Init())
	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = next.(Model)

	// Create second vault directory with a distinct note.
	vault2 := filepath.Join(t.TempDir(), "vault-2")
	if err := os.MkdirAll(vault2, 0o755); err != nil {
		t.Fatalf("mkdir vault2: %v", err)
	}
	if err := os.WriteFile(filepath.Join(vault2, "nova.txt"), []byte("v2"), 0o644); err != nil {
		t.Fatalf("write nova.txt: %v", err)
	}

	m = sendKey(t, m, "c")
	if m.modal.kind != modalSettings {
		t.Fatalf("expected modalSettings on 'c', got %v", m.modal.kind)
	}
	m.modal.inputs[0].SetValue(vault2)
	m = sendKey(t, m, "enter")

	if m.modal.kind != modalNone {
		t.Fatalf("expected modalSettings closed, got %v", m.modal.kind)
	}
	if len(m.notes) != 1 || m.notes[0].ID != "nova.txt" {
		t.Fatalf("expected notes from vault2, got %+v", m.notes)
	}
	saved, err := store.GetVaultPath()
	if err != nil || saved != vault2 {
		t.Fatalf("YAML config vault_path = (%q, %v), want %q", saved, err, vault2)
	}
}

func TestAllModalsRenderWithinViewportDimensions(t *testing.T) {
	store, v := newTestEnv(t)
	if _, err := v.CreateNotebook("WORK"); err != nil {
		t.Fatalf("CreateNotebook: %v", err)
	}
	if _, err := v.CreateNote("WORK", "Doc", "Conteúdo"); err != nil {
		t.Fatalf("CreateNote: %v", err)
	}
	if _, err := v.SaveTemplate("existente", "antigo"); err != nil {
		t.Fatalf("SaveTemplate: %v", err)
	}

	const width, height = 100, 30
	base := NewModel(store, v, Options{DisableCursorBlink: true})
	base = runCmd(base, base.Init())
	next, _ := base.Update(tea.WindowSizeMsg{Width: width, Height: height})
	base = next.(Model)

	cases := []struct {
		name   string
		open   func(Model) Model
		marker string
	}{
		{"new note", func(m Model) Model { return sendKey(t, m, "n") }, "Nova Nota (.txt)"},
		{"edit meta", func(m Model) Model { return sendKey(t, m, "m") }, "Mover / Renomear"},
		{"delete confirm", func(m Model) Model { return sendKey(t, m, "d") }, "Mover Nota para Lixeira"},
		{"notebooks", func(m Model) Model { return sendKey(t, m, "b") }, "Cadernos (Pastas do Vault)"},
		{"settings", func(m Model) Model { return sendKey(t, m, "c") }, "Configurações — vault-note-cli"},
		{"help", func(m Model) Model { return sendKey(t, m, "?") }, "Atalhos de Teclado — vault-note-cli"},
		{"quit confirm", func(m Model) Model { return sendKey(t, m, "q") }, "Sair do vault-note-cli"},
		{"save as template", func(m Model) Model { return sendKey(t, m, "S") }, "Salvar Nota como Template"},
		{"overwrite template", func(m Model) Model {
			m = sendKey(t, m, "S")
			m.modal.inputs[0].SetValue("existente")
			return sendKey(t, m, "enter")
		}, "Template já existe"},
		{"external conflict", func(m Model) Model {
			m.openConflictModal("WORK/Doc.txt", "novo")
			return m
		}, "Conflito Externo Detectado"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := tc.open(base)
			out := m.View()
			lines := strings.Split(out, "\n")
			if len(lines) != height {
				t.Fatalf("height = %d, want %d", len(lines), height)
			}
			for i, line := range lines {
				if w := ansi.StringWidth(line); w != width {
					t.Fatalf("line %d width = %d, want %d", i, w, width)
				}
			}
			plain := ansi.Strip(out)
			if !strings.Contains(plain, tc.marker) {
				t.Fatalf("expected marker %q in rendered modal", tc.marker)
			}
			for _, bgMarker := range []string{"vault-note-cli", "Notas"} {
				if !strings.Contains(plain, bgMarker) {
					t.Fatalf("expected dimmed background marker %q around modal", bgMarker)
				}
			}
		})
	}
}

func TestNewNoteModalTemplateSelector(t *testing.T) {
	store, v := newTestEnv(t)
	if _, err := v.CreateNotebook("WORK"); err != nil {
		t.Fatalf("CreateNotebook: %v", err)
	}
	if _, err := v.SaveTemplate("reuniao", "Nota: {{title}} ({{template}})\n"); err != nil {
		t.Fatalf("SaveTemplate: %v", err)
	}

	m := NewModel(store, v, Options{DisableCursorBlink: true})
	m = runCmd(m, m.Init())
	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = next.(Model)

	m = sendKey(t, m, "n")
	if m.modal.kind != modalNewNote {
		t.Fatalf("expected modalNewNote, got %v", m.modal.kind)
	}
	if len(m.modal.templateNames) != 1 || m.modal.templateNames[0] != "reuniao" {
		t.Fatalf("expected template names loaded, got %+v", m.modal.templateNames)
	}
	if !strings.Contains(ansi.Strip(m.View()), "Template") {
		t.Fatalf("expected Template field in modal")
	}

	m = typeText(t, m, "Plano")
	// Tab -> notebook, Tab -> template, Right -> select "reuniao".
	m = sendKey(t, m, "tab")
	m = sendKey(t, m, "tab")
	if m.modal.focusIdx != 2 {
		t.Fatalf("expected focus on template field, got %d", m.modal.focusIdx)
	}
	m = sendKey(t, m, "right")
	if m.modal.templateIdx != 1 {
		t.Fatalf("expected templateIdx 1, got %d", m.modal.templateIdx)
	}
	m = sendKey(t, m, "enter")

	if m.modal.kind != modalNone {
		t.Fatalf("expected modal closed, got %v", m.modal.kind)
	}
	if len(m.notes) != 1 || m.notes[0].ID != "Plano.txt" {
		t.Fatalf("expected created note, got %+v", m.notes)
	}
	if m.notes[0].Content != "Nota: Plano (reuniao)\n" {
		t.Fatalf("unexpected rendered content: %q", m.notes[0].Content)
	}
}

func TestEditMetaModalHasNoTemplateField(t *testing.T) {
	store, v := newTestEnv(t)
	if _, err := v.CreateNote("", "Doc", "x"); err != nil {
		t.Fatalf("CreateNote: %v", err)
	}
	if _, err := v.SaveTemplate("reuniao", "x"); err != nil {
		t.Fatalf("SaveTemplate: %v", err)
	}

	m := NewModel(store, v, Options{DisableCursorBlink: true})
	m = runCmd(m, m.Init())
	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = next.(Model)

	m = sendKey(t, m, "m")
	if m.modal.kind != modalEditMeta {
		t.Fatalf("expected modalEditMeta, got %v", m.modal.kind)
	}
	if len(m.modal.templateNames) != 0 {
		t.Fatalf("edit-meta modal must not load templates, got %+v", m.modal.templateNames)
	}
	// Tab twice wraps back to the title field (only 2 fields).
	m = sendKey(t, m, "tab")
	m = sendKey(t, m, "tab")
	if m.modal.focusIdx != 0 {
		t.Fatalf("expected focus to wrap to 0, got %d", m.modal.focusIdx)
	}
}

func TestListSOpensSaveAsTemplateModal(t *testing.T) {
	store, v := newTestEnv(t)
	if _, err := v.CreateNote("", "Modelo", "corpo original"); err != nil {
		t.Fatalf("CreateNote: %v", err)
	}
	m := NewModel(store, v, Options{DisableCursorBlink: true})
	m = runCmd(m, m.Init())
	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = next.(Model)

	m = sendKey(t, m, "S")
	if m.modal.kind != modalSaveAsTemplate {
		t.Fatalf("expected modalSaveAsTemplate, got %v", m.modal.kind)
	}
	if m.modal.inputs[0].Value() != "Modelo" {
		t.Fatalf("expected title prefill, got %q", m.modal.inputs[0].Value())
	}
	m.modal.inputs[0].SetValue("modelo")
	m = sendKey(t, m, "enter")
	if m.modal.kind != modalNone {
		t.Fatalf("expected modal closed, got %v", m.modal.kind)
	}
	tpl, err := v.GetTemplate("modelo")
	if err != nil {
		t.Fatalf("GetTemplate: %v", err)
	}
	if tpl.Body != "corpo original" {
		t.Fatalf("template body = %q", tpl.Body)
	}
	note, err := v.GetNote("Modelo.txt")
	if err != nil {
		t.Fatalf("GetNote: %v", err)
	}
	if note.Content != "corpo original" {
		t.Fatalf("original note mutated: %q", note.Content)
	}
	if !strings.Contains(m.statusMsg, "Template salvo: modelo") {
		t.Fatalf("unexpected status: %q", m.statusMsg)
	}
}

func TestEditorAltTSavesDirtyBufferWithoutWritingNote(t *testing.T) {
	store, v := newTestEnv(t)
	note, err := v.CreateNote("", "Rascunho", "inicial")
	if err != nil {
		t.Fatalf("CreateNote: %v", err)
	}
	m := NewModel(store, v, Options{DisableCursorBlink: true})
	m = runCmd(m, m.Init())
	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = next.(Model)

	m = sendKey(t, m, "tab")
	m = typeText(t, m, " alterado")
	if !m.dirty {
		t.Fatalf("expected dirty buffer")
	}
	m = sendKey(t, m, "alt+t")
	if m.modal.kind != modalSaveAsTemplate {
		t.Fatalf("expected modalSaveAsTemplate, got %v", m.modal.kind)
	}
	m.modal.inputs[0].SetValue("instantaneo")
	m = sendKey(t, m, "enter")
	if m.modal.kind != modalNone {
		t.Fatalf("expected modal closed, got %v", m.modal.kind)
	}
	tpl, err := v.GetTemplate("instantaneo")
	if err != nil {
		t.Fatalf("GetTemplate: %v", err)
	}
	if tpl.Body != "inicial alterado" {
		t.Fatalf("template body = %q", tpl.Body)
	}
	after, err := os.ReadFile(note.Path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(after) != "inicial" {
		t.Fatalf("Alt+T wrote the dirty note to disk: %q", string(after))
	}
	if !m.dirty {
		t.Fatalf("Alt+T must keep the buffer dirty")
	}
}

func TestSaveAsTemplateOverwriteConfirmation(t *testing.T) {
	store, v := newTestEnv(t)
	if _, err := v.CreateNote("", "Nota", "corpo novo"); err != nil {
		t.Fatalf("CreateNote: %v", err)
	}
	if _, err := v.SaveTemplate("existente", "antigo"); err != nil {
		t.Fatalf("SaveTemplate: %v", err)
	}
	m := NewModel(store, v, Options{DisableCursorBlink: true})
	m = runCmd(m, m.Init())
	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = next.(Model)

	m = sendKey(t, m, "S")
	m.modal.inputs[0].SetValue("existente")
	m = sendKey(t, m, "enter")
	if m.modal.kind != modalConfirmOverwriteTemplate {
		t.Fatalf("expected modalConfirmOverwriteTemplate, got %v", m.modal.kind)
	}
	if m.modal.inputs[0].Value() != "existente" {
		t.Fatalf("typed name lost on confirmation: %q", m.modal.inputs[0].Value())
	}

	// n returns to the form with the name intact.
	m = sendKey(t, m, "n")
	if m.modal.kind != modalSaveAsTemplate {
		t.Fatalf("expected n to return to save form, got %v", m.modal.kind)
	}
	if m.modal.inputs[0].Value() != "existente" {
		t.Fatalf("name not preserved after n: %q", m.modal.inputs[0].Value())
	}

	// Enter then y overwrites.
	m = sendKey(t, m, "enter")
	if m.modal.kind != modalConfirmOverwriteTemplate {
		t.Fatalf("expected confirmation again, got %v", m.modal.kind)
	}
	m = sendKey(t, m, "y")
	if m.modal.kind != modalNone {
		t.Fatalf("expected modal closed after y, got %v", m.modal.kind)
	}
	tpl, err := v.GetTemplate("existente")
	if err != nil {
		t.Fatalf("GetTemplate: %v", err)
	}
	if tpl.Body != "corpo novo" {
		t.Fatalf("template not overwritten: %q", tpl.Body)
	}
}

// newInsertTUIEnv builds a TUI model over a vault with one template and one note.
func newInsertTUIEnv(t *testing.T) (Model, string) {
	t.Helper()
	store, v := newTestEnv(t)
	if _, err := v.SaveTemplate("checklist", "== {{title}} checklist ==\n- item\n"); err != nil {
		t.Fatalf("SaveTemplate: %v", err)
	}
	note, err := v.CreateNote("", "Projeto X", "Corpo existente")
	if err != nil {
		t.Fatalf("CreateNote: %v", err)
	}

	m := NewModel(store, v, Options{DisableCursorBlink: true})
	m = runCmd(m, m.Init())
	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	return next.(Model), note.Path
}

func TestEditorCtrlTOpensTemplatePicker(t *testing.T) {
	m, notePath := newInsertTUIEnv(t)
	before, err := os.ReadFile(notePath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	m = sendKey(t, m, "tab")
	if m.focus != focusEditor {
		t.Fatalf("expected focusEditor")
	}
	m = sendKey(t, m, "ctrl+t")
	if m.modal.kind != modalInsertTemplate {
		t.Fatalf("expected modalInsertTemplate, got %v", m.modal.kind)
	}
	if len(m.modal.templateNames) != 1 || m.modal.templateNames[0] != "checklist" {
		t.Fatalf("expected templates loaded, got %+v", m.modal.templateNames)
	}
	if m.modal.insertAppend {
		t.Fatalf("editor picker must not be in append mode")
	}
	if !strings.Contains(ansi.Strip(m.View()), "Inserir Template") {
		t.Fatalf("expected picker modal rendered")
	}

	// Move the cursor to the start so insertion happens at the cursor.
	m.editor.CursorStart()
	m = sendKey(t, m, "enter")
	if m.modal.kind != modalNone {
		t.Fatalf("expected modal closed, got %v", m.modal.kind)
	}
	if !m.dirty {
		t.Fatalf("expected dirty buffer after in-editor insert")
	}
	want := "== Projeto X checklist ==\n- item\nCorpo existente"
	if got := m.editor.Value(); got != want {
		t.Fatalf("editor value = %q, want %q", got, want)
	}
	// Buffer only: nothing written until Ctrl+S.
	after, err := os.ReadFile(notePath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(after) != string(before) {
		t.Fatalf("Ctrl+T wrote to disk without saving")
	}
	if m.focus != focusEditor {
		t.Fatalf("expected focus to stay in editor")
	}
}

func TestListTAppendsTemplateToFile(t *testing.T) {
	m, notePath := newInsertTUIEnv(t)

	m = sendKey(t, m, "T")
	if m.modal.kind != modalInsertTemplate {
		t.Fatalf("expected modalInsertTemplate, got %v", m.modal.kind)
	}
	if !m.modal.insertAppend {
		t.Fatalf("list picker must be in append mode")
	}

	m = sendKey(t, m, "enter")
	if m.modal.kind != modalNone {
		t.Fatalf("expected modal closed, got %v", m.modal.kind)
	}
	raw, err := os.ReadFile(notePath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	want := "Corpo existente\n\n== Projeto X checklist ==\n- item\n"
	if string(raw) != want {
		t.Fatalf("file = %q, want %q", string(raw), want)
	}
	if m.dirty {
		t.Fatalf("expected buffer reloaded clean after append")
	}
	if m.editor.Value() != want {
		t.Fatalf("editor value = %q, want %q", m.editor.Value(), want)
	}
	if !strings.Contains(m.statusMsg, "acrescentado") {
		t.Fatalf("unexpected status: %q", m.statusMsg)
	}
}

func TestInsertTemplatePickerEscAndEmptyList(t *testing.T) {
	m, _ := newInsertTUIEnv(t)
	m = sendKey(t, m, "T")
	m = sendKey(t, m, "esc")
	if m.modal.kind != modalNone {
		t.Fatalf("expected Esc to close picker")
	}

	store, v := newTestEnv(t)
	if _, err := v.CreateNote("", "Sem template", "corpo"); err != nil {
		t.Fatalf("CreateNote: %v", err)
	}
	empty := NewModel(store, v, Options{DisableCursorBlink: true})
	empty = runCmd(empty, empty.Init())
	next, _ := empty.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	empty = next.(Model)

	empty = sendKey(t, empty, "T")
	plain := ansi.Strip(empty.View())
	if !strings.Contains(plain, "Nenhum template") {
		t.Fatalf("expected empty-list message, got %q", plain)
	}
	// Enter on an empty list must not panic or close into a half state.
	empty = sendKey(t, empty, "enter")
	if empty.modal.kind != modalInsertTemplate {
		t.Fatalf("expected picker to stay open on empty list")
	}
}

func TestInsertTemplateReinsertSucceedsInTUI(t *testing.T) {
	// Editor path: note already contains the rendered first line.
	store, v := newTestEnv(t)
	if _, err := v.SaveTemplate("checklist", "== checklist ==\n- item\n"); err != nil {
		t.Fatalf("SaveTemplate: %v", err)
	}
	if _, err := v.CreateNote("", "Projeto X", "== checklist ==\n- item\n"); err != nil {
		t.Fatalf("CreateNote: %v", err)
	}
	m := NewModel(store, v, Options{DisableCursorBlink: true})
	m = runCmd(m, m.Init())
	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = next.(Model)

	m = sendKey(t, m, "tab")
	m = sendKey(t, m, "ctrl+t")
	m = sendKey(t, m, "enter")
	if m.modal.kind != modalNone {
		t.Fatalf("expected immediate insert with no confirmation, got %v", m.modal.kind)
	}
	if strings.Count(m.editor.Value(), "== checklist ==") != 2 {
		t.Fatalf("expected duplicate inserted, got %q", m.editor.Value())
	}
	if !m.dirty {
		t.Fatalf("expected dirty buffer")
	}

	// Append path: re-insert also succeeds immediately and writes a second copy.
	store2, v2 := newTestEnv(t)
	if _, err := v2.SaveTemplate("checklist", "== checklist ==\n- item\n"); err != nil {
		t.Fatalf("SaveTemplate: %v", err)
	}
	note2, err := v2.CreateNote("", "Projeto Y", "== checklist ==\n- item\n")
	if err != nil {
		t.Fatalf("CreateNote: %v", err)
	}
	m2 := NewModel(store2, v2, Options{DisableCursorBlink: true})
	m2 = runCmd(m2, m2.Init())
	next2, _ := m2.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m2 = next2.(Model)

	m2 = sendKey(t, m2, "T")
	m2 = sendKey(t, m2, "enter")
	if m2.modal.kind != modalNone {
		t.Fatalf("expected immediate append with no confirmation, got %v", m2.modal.kind)
	}
	raw, err := os.ReadFile(note2.Path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if strings.Count(string(raw), "== checklist ==") != 2 {
		t.Fatalf("expected second copy appended, got %q", string(raw))
	}
}

func TestSettingsTogglePreservesEditorAndFiltersWhenVaultUnchanged(t *testing.T) {
	store, v := newTestEnv(t)
	if _, err := v.CreateNotebook("WORK"); err != nil {
		t.Fatalf("CreateNotebook: %v", err)
	}
	note, err := v.CreateNote("WORK", "Doc", "conteúdo")
	if err != nil {
		t.Fatalf("CreateNote: %v", err)
	}
	m := NewModel(store, v, Options{DisableCursorBlink: true})
	m = runCmd(m, m.Init())
	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = next.(Model)

	m.filterNotebook = "WORK"
	m.filterQuery = "Doc"
	m.searchInput.SetValue("Doc")
	m.loadedNoteID = note.ID
	m.loadedContent = "conteúdo"
	m.editor.SetValue("alterado")
	m.dirty = true

	m.openSettingsModal(false)
	if m.modal.kind != modalSettings {
		t.Fatalf("expected settings modal")
	}
	m = sendKey(t, m, "tab")
	if m.modal.focusIdx != 1 {
		t.Fatalf("expected prefix field focused, got %d", m.modal.focusIdx)
	}
	m = sendKey(t, m, " ")
	if m.modal.showNotebookPrefix {
		t.Fatalf("expected toggle to turn prefix off")
	}
	m = sendKey(t, m, "enter")

	if m.modal.kind != modalNone {
		t.Fatalf("expected settings modal closed, got %v", m.modal.kind)
	}
	if m.showNotebookPrefix {
		t.Fatalf("expected model prefix disabled")
	}
	enabled, err := store.GetShowNotebookPrefix()
	if err != nil || enabled {
		t.Fatalf("persisted prefix = (%v, %v), want false", enabled, err)
	}
	if m.filterNotebook != "WORK" || m.filterQuery != "Doc" {
		t.Fatalf("filters lost: notebook=%q query=%q", m.filterNotebook, m.filterQuery)
	}
	if !m.dirty || m.editor.Value() != "alterado" {
		t.Fatalf("dirty editor lost: dirty=%v value=%q", m.dirty, m.editor.Value())
	}
}
