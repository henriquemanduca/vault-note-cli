package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func TestFindAndReplacePureUTF8(t *testing.T) {
	content := "Ação rápida\nOutra ação e AÇÃO 🚀"

	matches := findMatchesInText(content, "ação", false)
	if len(matches) != 3 {
		t.Fatalf("expected 3 case-insensitive matches, got %d: %+v", len(matches), matches)
	}
	if matches[0].line != 0 || matches[0].startCol != 0 || matches[1].line != 1 || matches[1].startCol != 6 {
		t.Fatalf("unexpected match coordinates: %+v", matches)
	}

	sensitive := findMatchesInText(content, "ação", true)
	if len(sensitive) != 1 || sensitive[0].line != 1 || sensitive[0].startCol != 6 {
		t.Fatalf("unexpected case-sensitive matches: %+v", sensitive)
	}

	single := replaceMatchAt(content, matches[1], "tarefa")
	if single != "Ação rápida\nOutra tarefa e AÇÃO 🚀" {
		t.Fatalf("replaceMatchAt = %q", single)
	}

	all, count := replaceAllMatches(content, "ação", "passo", false)
	if count != 3 || all != "passo rápida\nOutra passo e passo 🚀" {
		t.Fatalf("replaceAllMatches = (%q, %d)", all, count)
	}
}

func TestTUIFindAndReplaceBarFlow(t *testing.T) {
	store, v := newTestEnv(t)
	if _, err := v.CreateNote("", "Busca", "alpha beta\nAlpha gamma\nalpha"); err != nil {
		t.Fatalf("CreateNote: %v", err)
	}
	m := NewModel(store, v, Options{DisableCursorBlink: true})
	m = runCmd(m, m.Init())
	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = next.(Model)

	// Focus editor and open Find bar (Ctrl+F).
	m = sendKey(t, m, "tab")
	m = sendKey(t, m, "ctrl+f")
	if !m.find.active || m.find.replaceMode {
		t.Fatalf("expected find bar active (replaceMode=false)")
	}
	m = typeText(t, m, "alpha")
	if len(m.find.matches) != 3 || m.find.current != 0 {
		t.Fatalf("expected 3 matches at index 0, got %d (%d)", len(m.find.matches), m.find.current)
	}
	if line, col := editorCursorPos(m.editor); line != 0 || col != 0 {
		t.Fatalf("cursor after find = (%d,%d), want (0,0)", line, col)
	}

	// View still maintains exact viewport height (30 lines).
	if lines := strings.Split(m.View(), "\n"); len(lines) != 30 {
		t.Fatalf("view height with find bar = %d, want 30", len(lines))
	}
	if !strings.Contains(ansi.Strip(m.View()), "1/3") {
		t.Fatalf("expected match counter 1/3 in view")
	}

	// Enter advances to next match (line 1).
	m = sendKey(t, m, "enter")
	if m.find.current != 1 {
		t.Fatalf("expected match index 1, got %d", m.find.current)
	}
	if line, _ := editorCursorPos(m.editor); line != 1 {
		t.Fatalf("expected cursor on line 1, got %d", line)
	}

	// Alt+C toggles case-sensitive -> only 2 lowercase matches remain.
	m = sendKey(t, m, "alt+c")
	if !m.find.caseSensitive || len(m.find.matches) != 2 {
		t.Fatalf("expected 2 case-sensitive matches, got %d (case=%v)", len(m.find.matches), m.find.caseSensitive)
	}

	// Ctrl+R expands Replace bar; type replacement and Alt+A replaces all.
	m = sendKey(t, m, "ctrl+r")
	if !m.find.replaceMode || !m.find.focusReplace {
		t.Fatalf("expected replace mode focused")
	}
	if lines := strings.Split(m.View(), "\n"); len(lines) != 30 {
		t.Fatalf("view height with find+replace bar = %d, want 30", len(lines))
	}
	m = typeText(t, m, "OMEGA")
	m = sendKey(t, m, "alt+a")
	if m.editor.Value() != "OMEGA beta\nAlpha gamma\nOMEGA" {
		t.Fatalf("after Alt+A replace all = %q", m.editor.Value())
	}

	// Esc closes bar and Ctrl+Z undoes Replace All atomically.
	m = sendKey(t, m, "esc")
	if m.find.active {
		t.Fatalf("expected find bar closed on Esc")
	}
	m = sendKey(t, m, "ctrl+z")
	if m.editor.Value() != "alpha beta\nAlpha gamma\nalpha" {
		t.Fatalf("expected atomic undo of Replace All, got %q", m.editor.Value())
	}
}
