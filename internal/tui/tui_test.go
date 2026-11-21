package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/hmanduca/vault-note-cli/internal/db"
	"github.com/hmanduca/vault-note-cli/internal/vault"
)

func newTestEnv(t *testing.T) (*db.Store, *vault.Vault) {
	t.Helper()
	tmp := t.TempDir()
	store, err := db.NewStore(filepath.Join(tmp, "settings.db"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	vaultDir := filepath.Join(tmp, "vault")
	v, err := vault.Open(vaultDir)
	if err != nil {
		t.Fatalf("vault.Open: %v", err)
	}
	if _, err := store.SetVaultPath(v.Root()); err != nil {
		t.Fatalf("SetVaultPath: %v", err)
	}
	return store, v
}

func runCmd(m Model, cmd tea.Cmd) Model {
	if cmd == nil {
		return m
	}
	msg := cmd()
	if msg == nil {
		return m
	}
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, sub := range batch {
			m = runCmd(m, sub)
		}
		return m
	}
	next, nextCmd := m.Update(msg)
	m = next.(Model)
	if nextCmd != nil {
		return runCmd(m, nextCmd)
	}
	return m
}

func sendKey(t *testing.T, m Model, key string) Model {
	t.Helper()
	var km tea.KeyMsg
	switch key {
	case "enter":
		km = tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		km = tea.KeyMsg{Type: tea.KeyEsc}
	case "tab":
		km = tea.KeyMsg{Type: tea.KeyTab}
	case "shift+tab":
		km = tea.KeyMsg{Type: tea.KeyShiftTab}
	case "up":
		km = tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		km = tea.KeyMsg{Type: tea.KeyDown}
	case "left":
		km = tea.KeyMsg{Type: tea.KeyLeft}
	case "right":
		km = tea.KeyMsg{Type: tea.KeyRight}
	case "ctrl+s":
		km = tea.KeyMsg{Type: tea.KeyCtrlS}
	case "ctrl+c":
		km = tea.KeyMsg{Type: tea.KeyCtrlC}
	default:
		km = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
	}
	next, cmd := m.Update(km)
	m = next.(Model)
	return runCmd(m, cmd)
}

func typeText(t *testing.T, m Model, text string) Model {
	t.Helper()
	for _, r := range text {
		m = sendKey(t, m, string(r))
	}
	return m
}

func TestOnboardingOpensSettingsModalAndPersistsVaultPath(t *testing.T) {
	tmp := t.TempDir()
	store, err := db.NewStore(filepath.Join(tmp, "settings.db"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	// No vault configured -> modalSettings opens automatically.
	m := NewModel(store, nil, Options{DisableCursorBlink: true})
	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = next.(Model)

	if m.modal.kind != modalSettings || !m.modal.onboarding {
		t.Fatalf("expected onboarding modalSettings, got kind=%v onboarding=%v", m.modal.kind, m.modal.onboarding)
	}
	view := ansi.Strip(m.View())
	if !strings.Contains(view, "Configuração Inicial — vault-note-cli") {
		t.Fatalf("expected onboarding title in view, got:\n%s", view)
	}

	// Pre-populate a vault directory on disk with a notebook & .txt note.
	vaultDir := filepath.Join(tmp, "meu-vault")
	if err := os.MkdirAll(filepath.Join(vaultDir, "WORK"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(vaultDir, "WORK", "boas-vindas.txt"), []byte("Olá vault"), 0o644); err != nil {
		t.Fatalf("write note: %v", err)
	}

	m = typeText(t, m, vaultDir)
	m = sendKey(t, m, "enter")

	if m.modal.kind != modalNone {
		t.Fatalf("expected modal closed after saving vault path, got kind=%v err=%q", m.modal.kind, m.modal.errText)
	}
	if m.vault == nil || m.vault.Root() != vaultDir {
		t.Fatalf("expected vault initialized to %q, got %v", vaultDir, m.vault)
	}
	saved, err := store.GetVaultPath()
	if err != nil || saved != vaultDir {
		t.Fatalf("SQLite vault_path = (%q, %v), want %q", saved, err, vaultDir)
	}
	if len(m.notes) != 1 || m.notes[0].ID != "WORK/boas-vindas.txt" {
		t.Fatalf("expected loaded notes from configured vault, got %+v", m.notes)
	}
}

func TestTUIDirectEditingSaveAndTrash(t *testing.T) {
	store, v := newTestEnv(t)
	if _, err := v.CreateNotebook("WORK"); err != nil {
		t.Fatalf("CreateNotebook: %v", err)
	}
	n, err := v.CreateNote("WORK", "Sprint", "Inicial")
	if err != nil {
		t.Fatalf("CreateNote: %v", err)
	}

	m := NewModel(store, v, Options{DisableCursorBlink: true})
	m = runCmd(m, m.Init())
	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = next.(Model)

	if len(m.notes) != 1 || m.currentNote().ID != n.ID {
		t.Fatalf("expected 1 loaded note, got %+v", m.notes)
	}

	// Focus right-pane editor, type text, save with Ctrl+S, return to list with Esc.
	m = sendKey(t, m, "tab")
	if m.focus != focusEditor {
		t.Fatalf("expected focusEditor after Tab")
	}
	m = typeText(t, m, " atualizado")
	if !m.dirty {
		t.Fatalf("expected dirty=true after typing")
	}
	m = sendKey(t, m, "ctrl+s")
	if m.dirty {
		t.Fatalf("expected dirty=false after Ctrl+S")
	}
	m = sendKey(t, m, "esc")
	if m.focus != focusList {
		t.Fatalf("expected focusList after Esc")
	}

	// Verify content persisted to physical .txt file.
	raw, err := os.ReadFile(n.Path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(raw) != "Inicial atualizado" {
		t.Fatalf("file content = %q, want %q", string(raw), "Inicial atualizado")
	}

	// Delete note -> moves to .trash/.
	m = sendKey(t, m, "d")
	if m.modal.kind != modalDeleteConfirm {
		t.Fatalf("expected modalDeleteConfirm, got %v", m.modal.kind)
	}
	m = sendKey(t, m, "y")
	if len(m.notes) != 0 {
		t.Fatalf("expected 0 notes after trash, got %d", len(m.notes))
	}
	if _, err := os.Stat(n.Path); !os.IsNotExist(err) {
		t.Fatalf("expected original .txt removed from notebook")
	}
}

func TestLayout2080AndTopBarBranding(t *testing.T) {
	store, v := newTestEnv(t)
	if _, err := v.CreateNote("", "Raiz", "Corpo"); err != nil {
		t.Fatalf("CreateNote: %v", err)
	}

	m := NewModel(store, v, Options{DisableCursorBlink: true})
	m = runCmd(m, m.Init())
	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = next.(Model)

	if m.layout.mode != layoutMasterDetail {
		t.Fatalf("expected layoutMasterDetail, got %v", m.layout.mode)
	}
	if m.layout.leftWidth != 20 || m.layout.rightWidth != 80 {
		t.Fatalf("expected 20/80 split, got %d/%d", m.layout.leftWidth, m.layout.rightWidth)
	}

	view := ansi.Strip(m.View())
	lines := strings.Split(view, "\n")
	if len(lines) < 3 {
		t.Fatalf("unexpected short view: %d lines", len(lines))
	}
	topRow := lines[1] // line 0 is MarginTop(1)
	if !strings.Contains(topRow, "vault-note-cli") || !strings.Contains(topRow, "1 nota") {
		t.Fatalf("unexpected top bar %q", topRow)
	}
}
