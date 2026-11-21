package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/hmanduca/vault-note-cli/internal/settings"
	"github.com/hmanduca/vault-note-cli/internal/vault"
)

func newTestEnv(t *testing.T) (*settings.Store, *vault.Vault) {
	t.Helper()
	tmp := t.TempDir()
	store, err := settings.NewStore(filepath.Join(tmp, "config.yaml"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

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
	case "ctrl+z":
		km = tea.KeyMsg{Type: tea.KeyCtrlZ}
	case "ctrl+y":
		km = tea.KeyMsg{Type: tea.KeyCtrlY}
	case "ctrl+f":
		km = tea.KeyMsg{Type: tea.KeyCtrlF}
	case "ctrl+r":
		km = tea.KeyMsg{Type: tea.KeyCtrlR}
	case "ctrl+t":
		km = tea.KeyMsg{Type: tea.KeyCtrlT}
	case "alt+up":
		km = tea.KeyMsg{Type: tea.KeyUp, Alt: true}
	case "alt+down":
		km = tea.KeyMsg{Type: tea.KeyDown, Alt: true}
	case "alt+t", "alt+x", "alt+z", "alt+n", "alt+p", "alt+k", "alt+[", "alt+]", "alt+y", "alt+w", "alt+c", "alt+a", "alt+r":
		km = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(strings.TrimPrefix(key, "alt+")), Alt: true}
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
	store, err := settings.NewStore(filepath.Join(tmp, "config.yaml"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

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
		t.Fatalf("YAML config vault_path = (%q, %v), want %q", saved, err, vaultDir)
	}
	if len(m.notes) != 1 || m.notes[0].ID != "WORK/boas-vindas.txt" {
		t.Fatalf("expected loaded notes from configured vault, got %+v", m.notes)
	}

	// Onboarding seeds the 4 starter templates and reports it in the status bar.
	for _, name := range []string{"checklist", "daily", "decision", "meeting"} {
		if _, err := os.Stat(filepath.Join(vaultDir, ".templates", name+".txt")); err != nil {
			t.Fatalf("default %q not seeded during onboarding: %v", name, err)
		}
	}
	if names := m.loadTemplateNames(); len(names) != 4 {
		t.Fatalf("loadTemplateNames after onboarding = %v, want 4", names)
	}
	if !strings.Contains(m.statusMsg, "4 templates padrão criados") {
		t.Fatalf("status message missing seeded count: %q", m.statusMsg)
	}
}

func TestSettingsReSaveDoesNotResurrectDeletedDefaults(t *testing.T) {
	store, v := newTestEnv(t)
	if _, err := v.SeedDefaultTemplatesIfAbsent(); err != nil {
		t.Fatalf("SeedDefaultTemplatesIfAbsent: %v", err)
	}
	if _, err := v.DeleteTemplate("daily"); err != nil {
		t.Fatalf("DeleteTemplate daily: %v", err)
	}

	m := NewModel(store, v, Options{DisableCursorBlink: true})
	m = runCmd(m, m.Init())
	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = next.(Model)

	m = sendKey(t, m, "c")
	if m.modal.kind != modalSettings {
		t.Fatalf("expected modalSettings on 'c', got %v", m.modal.kind)
	}
	m.modal.inputs[0].SetValue(v.Root())
	m = sendKey(t, m, "enter")

	if _, err := os.Stat(filepath.Join(v.Root(), ".templates", "daily.txt")); !os.IsNotExist(err) {
		t.Fatalf("re-saving settings resurrected daily.txt (stat err = %v)", err)
	}
	if names := m.loadTemplateNames(); len(names) != 3 {
		t.Fatalf("loadTemplateNames after re-save = %v, want 3", names)
	}
	if strings.Contains(m.statusMsg, "templates padrão criados") {
		t.Fatalf("status should not report seeding on re-save: %q", m.statusMsg)
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
	if !strings.Contains(view, "Ln 1, Col 6") || !strings.Contains(view, "1 pal") {
		t.Fatalf("expected editor info footer with cursor and word stats:\n%s", view)
	}
}

func TestZenModeLineNumbersAndSettingsPersistence(t *testing.T) {
	store, v := newTestEnv(t)
	if _, err := v.CreateNote("", "Foco", "linha 1\nlinha 2"); err != nil {
		t.Fatalf("CreateNote: %v", err)
	}
	m := NewModel(store, v, Options{DisableCursorBlink: true})
	m = runCmd(m, m.Init())
	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = next.(Model)

	// Toggle Zen from list with 'z'.
	m = sendKey(t, m, "z")
	if !m.zenMode || m.layout.mode != layoutZen || m.layout.rightWidth != 100 || m.focus != focusEditor {
		t.Fatalf("expected Zen mode active at 100 cols in editor, got zen=%v mode=%v w=%d focus=%v",
			m.zenMode, m.layout.mode, m.layout.rightWidth, m.focus)
	}
	zenView := ansi.Strip(m.View())
	if lines := strings.Split(zenView, "\n"); len(lines) != 30 {
		t.Fatalf("Zen view height = %d, want 30", len(lines))
	}
	if !strings.Contains(zenView, "[ZEN]") {
		t.Fatalf("expected [ZEN] badge in top bar")
	}

	// Alt+N toggles line numbers on in-session.
	m = sendKey(t, m, "alt+n")
	if !m.showLineNumbers || !m.editor.ShowLineNumbers {
		t.Fatalf("expected line numbers enabled via Alt+N")
	}

	// Esc exits Zen mode and returns focus to list.
	m = sendKey(t, m, "esc")
	if m.zenMode || m.focus != focusList || m.layout.mode != layoutMasterDetail {
		t.Fatalf("expected Esc to exit Zen and return to list, got zen=%v focus=%v mode=%v", m.zenMode, m.focus, m.layout.mode)
	}

	// Persist show_line_numbers via Settings modal (field index 2).
	m.showLineNumbers = false
	m.editor.ShowLineNumbers = false
	m = sendKey(t, m, "c")
	m = sendKey(t, m, "tab") // field 1: show_notebook_prefix
	m = sendKey(t, m, "tab") // field 2: show_line_numbers
	if m.modal.focusIdx != 2 {
		t.Fatalf("expected focus on line-numbers field (2), got %d", m.modal.focusIdx)
	}
	m = sendKey(t, m, " ")
	m = sendKey(t, m, "enter")
	if !m.showLineNumbers || !m.editor.ShowLineNumbers {
		t.Fatalf("expected line numbers enabled after saving Settings")
	}
	persisted, err := store.GetShowLineNumbers()
	if err != nil || !persisted {
		t.Fatalf("store.GetShowLineNumbers = (%v, %v), want true", persisted, err)
	}
}

func TestMouseScrollAndClickNavigation(t *testing.T) {
	store, v := newTestEnv(t)
	if _, err := v.CreateNote("", "Alpha", "a1\na2\na3\na4\na5"); err != nil {
		t.Fatalf("CreateNote Alpha: %v", err)
	}
	if _, err := v.CreateNote("", "Beta", "b1\nb2"); err != nil {
		t.Fatalf("CreateNote Beta: %v", err)
	}
	m := NewModel(store, v, Options{DisableCursorBlink: true})
	m = runCmd(m, m.Init())
	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = next.(Model)

	// Left-click second row (row 6: 5 + 1) selects Beta.
	nm, cmd := m.Update(tea.MouseMsg{X: 5, Y: 6, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
	m = runCmd(nm.(Model), cmd)
	if m.selected != 1 || m.currentNote().Title != "Beta" {
		t.Fatalf("expected Beta selected on click, got idx=%d", m.selected)
	}

	// Left-click right pane focuses editor.
	nm, cmd = m.Update(tea.MouseMsg{X: 40, Y: 10, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
	m = runCmd(nm.(Model), cmd)
	if m.focus != focusEditor {
		t.Fatalf("expected editor focused after right-pane click")
	}

	// Mouse wheel up moves cursor up in editor.
	nm, cmd = m.Update(tea.MouseMsg{X: 40, Y: 10, Button: tea.MouseButtonWheelUp})
	m = runCmd(nm.(Model), cmd)
	if m.editor.Line() != 0 {
		t.Fatalf("expected cursor moved up to line 0, got %d", m.editor.Line())
	}
}

func TestExternalConflictModalOverwriteReloadAndCopy(t *testing.T) {
	store, v := newTestEnv(t)
	note, err := v.CreateNote("", "Sync", "versao-1")
	if err != nil {
		t.Fatalf("CreateNote: %v", err)
	}

	m := NewModel(store, v, Options{DisableCursorBlink: true})
	m = runCmd(m, m.Init())
	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = next.(Model)

	// Edit in TUI.
	m = sendKey(t, m, "tab")
	m = typeText(t, m, " local")

	// Simulate external modification (e.g., Syncthing/Git) on disk.
	if err := os.WriteFile(note.Path, []byte("versao-externa"), 0o644); err != nil {
		t.Fatalf("external write: %v", err)
	}

	// Ctrl+S detects conflict and opens modalConflict without overwriting disk.
	m = sendKey(t, m, "ctrl+s")
	if m.modal.kind != modalConflict {
		t.Fatalf("expected modalConflict, got %v", m.modal.kind)
	}
	raw, _ := os.ReadFile(note.Path)
	if string(raw) != "versao-externa" {
		t.Fatalf("disk file was overwritten despite conflict: %q", string(raw))
	}

	// Choice 'c' saves conflict copy and preserves external file untouched.
	m = sendKey(t, m, "c")
	if m.modal.kind != modalNone {
		t.Fatalf("expected conflict modal closed after 'c'")
	}
	if len(m.notes) != 2 {
		t.Fatalf("expected 2 notes (original + conflict copy), got %d", len(m.notes))
	}
	raw, _ = os.ReadFile(note.Path)
	if string(raw) != "versao-externa" {
		t.Fatalf("original note mutated during conflict copy: %q", string(raw))
	}

	// Select original Sync note and trigger conflict again -> choose 'r' (reload).
	m.focus = focusList
	m.editor.Blur()
	for i, n := range m.notes {
		if n.ID == note.ID {
			m.selected = i
			m.syncEditorToSelected()
			break
		}
	}
	m = sendKey(t, m, "tab")
	m = typeText(t, m, " descartavel")
	if err := os.WriteFile(note.Path, []byte("externa-2"), 0o644); err != nil {
		t.Fatalf("external write 2: %v", err)
	}
	m = sendKey(t, m, "ctrl+s")
	if m.modal.kind != modalConflict {
		t.Fatalf("expected modalConflict on second conflict")
	}
	m = sendKey(t, m, "r")
	if m.modal.kind != modalNone || m.dirty || m.editor.Value() != "externa-2" {
		t.Fatalf("reload failed: modal=%v dirty=%v val=%q", m.modal.kind, m.dirty, m.editor.Value())
	}

	// Trigger conflict a third time and choose 'o' (overwrite).
	m = typeText(t, m, " forcado")
	if err := os.WriteFile(note.Path, []byte("externa-3"), 0o644); err != nil {
		t.Fatalf("external write 3: %v", err)
	}
	m = sendKey(t, m, "ctrl+s")
	if m.modal.kind != modalConflict {
		t.Fatalf("expected modalConflict on third conflict")
	}
	m = sendKey(t, m, "o")
	raw, _ = os.ReadFile(note.Path)
	if string(raw) != "externa-2 forcado" {
		t.Fatalf("overwrite did not persist buffer: %q", string(raw))
	}
}
