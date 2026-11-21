package tui

import (
	"os"
	"strings"
	"testing"
)

func TestTUIEmptyTrashModalAndFlow(t *testing.T) {
	store, v := newTestEnv(t)
	if _, err := v.CreateNotebook("WORK"); err != nil {
		t.Fatalf("CreateNotebook: %v", err)
	}
	if _, err := v.CreateNote("WORK", "Temp", "discard me"); err != nil {
		t.Fatalf("CreateNote: %v", err)
	}

	m := NewModel(store, v, Options{DisableCursorBlink: true})
	m = runCmd(m, m.Init())

	// Pressing D when trash is already empty sets status and does not open a modal.
	m = sendKey(t, m, "D")
	if m.modal.kind != modalNone {
		t.Fatalf("expected no modal when trash is empty, got %v", m.modal.kind)
	}
	if !strings.Contains(m.statusMsg, "já está vazia") {
		t.Fatalf("unexpected statusMsg: %q", m.statusMsg)
	}

	// Delete note to populate .trash/.
	m = sendKey(t, m, "d")
	if m.modal.kind != modalDeleteConfirm {
		t.Fatalf("expected modalDeleteConfirm, got %v", m.modal.kind)
	}
	m = sendKey(t, m, "y")

	// Pressing D opens confirmation modal with item count; n cancels without deleting.
	m = sendKey(t, m, "D")
	if m.modal.kind != modalEmptyTrashConfirm {
		t.Fatalf("expected modalEmptyTrashConfirm, got %v", m.modal.kind)
	}
	if m.modal.trashCount != 1 {
		t.Fatalf("expected trashCount=1, got %d", m.modal.trashCount)
	}
	rendered := m.renderModal()
	if !strings.Contains(rendered, "Esvaziar Lixeira") || !strings.Contains(rendered, "1 item(ns)") {
		t.Fatalf("unexpected modal view:\n%s", rendered)
	}

	m = sendKey(t, m, "n")
	if m.modal.kind != modalNone {
		t.Fatalf("expected modal closed on cancel, got %v", m.modal.kind)
	}
	if items, err := v.ListTrash(); err != nil || len(items) != 1 {
		t.Fatalf("expected trash intact after cancel, got (%v, %v)", items, err)
	}

	// Pressing D -> y empties trash permanently.
	m = sendKey(t, m, "D")
	m = sendKey(t, m, "y")
	if m.modal.kind != modalNone {
		t.Fatalf("expected modal closed after confirm, got %v", m.modal.kind)
	}
	if !strings.Contains(m.statusMsg, "Lixeira esvaziada") {
		t.Fatalf("unexpected statusMsg after empty: %q", m.statusMsg)
	}
	if _, err := os.Stat(v.TrashDir()); !os.IsNotExist(err) {
		t.Fatalf("expected .trash dir removed, stat err = %v", err)
	}
}
