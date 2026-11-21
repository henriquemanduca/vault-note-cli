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
		t.Fatalf("SQLite vault_path = (%q, %v), want %q", saved, err, vault2)
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
