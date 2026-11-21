package tui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func TestEditorHistoryCoalesceCapAndUndoRedo(t *testing.T) {
	now := time.Date(2026, 11, 21, 15, 0, 0, 0, time.Local)
	h := newEditorHistory()
	h.maxEntries = 3
	h.now = func() time.Time { return now }

	// Rapid typing within 800ms coalesces into a single checkpoint.
	h.RecordBeforeEdit(editorSnapshot{value: "", line: 0, col: 0}, editOpTyping)
	now = now.Add(100 * time.Millisecond)
	h.RecordBeforeEdit(editorSnapshot{value: "a", line: 0, col: 1}, editOpTyping)
	now = now.Add(100 * time.Millisecond)
	h.RecordBeforeEdit(editorSnapshot{value: "ab", line: 0, col: 2}, editOpTyping)

	if len(h.undo) != 1 || h.undo[0].value != "" {
		t.Fatalf("expected 1 coalesced checkpoint, got %+v", h.undo)
	}

	// After window expires, typing creates a new checkpoint.
	now = now.Add(2 * time.Second)
	h.RecordBeforeEdit(editorSnapshot{value: "abc", line: 0, col: 3}, editOpTyping)
	if len(h.undo) != 2 {
		t.Fatalf("expected 2 checkpoints after pause, got %d", len(h.undo))
	}

	// Discrete edits always create checkpoints and respect maxEntries cap (3).
	h.RecordBeforeEdit(editorSnapshot{value: "abc ", line: 0, col: 4}, editOpDiscrete)
	h.RecordBeforeEdit(editorSnapshot{value: "abc def", line: 0, col: 7}, editOpDiscrete)
	if len(h.undo) != 3 || h.undo[0].value != "abc" {
		t.Fatalf("expected capped undo stack [abc, 'abc ', 'abc def'], got %+v", h.undo)
	}

	// Undo restores previous value and pushes current to redo.
	prev, ok := h.Undo(editorSnapshot{value: "abc def!", line: 0, col: 8})
	if !ok || prev.value != "abc def" || prev.col != 7 {
		t.Fatalf("Undo = (%+v, %v), want 'abc def'", prev, ok)
	}

	// Redo brings it back.
	next, ok := h.Redo(prev)
	if !ok || next.value != "abc def!" || next.col != 8 {
		t.Fatalf("Redo = (%+v, %v), want 'abc def!'", next, ok)
	}

	// New edit clears redo stack.
	_, _ = h.Undo(next)
	h.RecordBeforeEdit(editorSnapshot{value: "abc def", line: 0, col: 7}, editOpDiscrete)
	if len(h.redo) != 0 {
		t.Fatalf("expected redo cleared on new edit, got %+v", h.redo)
	}
}

func TestTUIUndoRedoAndCursorRestoration(t *testing.T) {
	store, v := newTestEnv(t)
	if _, err := v.CreateNote("", "Notas", "linha 1\nlinha 2"); err != nil {
		t.Fatalf("CreateNote: %v", err)
	}
	m := NewModel(store, v, Options{DisableCursorBlink: true})
	m = runCmd(m, m.Init())
	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = next.(Model)

	m = sendKey(t, m, "tab")
	// Inject fixed clock that advances per edit to guarantee distinct checkpoints.
	baseTime := time.Date(2026, 11, 21, 15, 0, 0, 0, time.Local)
	m.history.now = func() time.Time { return baseTime }

	// Paste acts as a single atomic undo step.
	pasteMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" colado"), Paste: true}
	nModel, cmd := m.Update(pasteMsg)
	m = runCmd(nModel.(Model), cmd)
	if m.editor.Value() != "linha 1\nlinha 2 colado" {
		t.Fatalf("after paste = %q", m.editor.Value())
	}
	if !m.dirty {
		t.Fatalf("expected dirty after paste")
	}

	// Ctrl+Z undoes the paste in one step and returns dirty to false.
	m = sendKey(t, m, "ctrl+z")
	if m.editor.Value() != "linha 1\nlinha 2" {
		t.Fatalf("after Ctrl+Z = %q", m.editor.Value())
	}
	if m.dirty {
		t.Fatalf("expected clean buffer after undoing back to loaded content")
	}

	// Ctrl+Y redoes the paste.
	m = sendKey(t, m, "ctrl+y")
	if m.editor.Value() != "linha 1\nlinha 2 colado" || !m.dirty {
		t.Fatalf("after Ctrl+Y = (%q, dirty=%v)", m.editor.Value(), m.dirty)
	}
}
