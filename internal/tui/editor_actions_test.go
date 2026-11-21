package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestToggleCheckboxOnLine(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		line    int
		col     int
		want    string
		wantCol int
	}{
		{"plain to unchecked", "Comprar café", 0, 3, "- [ ] Comprar café", 9},
		{"bullet to unchecked", "  - item", 0, 4, "  - [ ] item", 8},
		{"unchecked to checked", "- [ ] tarefa", 0, 6, "- [x] tarefa", 6},
		{"checked to unchecked", "- [x] tarefa", 0, 6, "- [ ] tarefa", 6},
		{"upper X to unchecked", "  * [X] revisão", 0, 8, "  * [ ] revisão", 8},
		{"bare checkbox", "- [ ]", 0, 5, "- [x]", 5},
		{"second line only", "um\ndois\ntrês", 1, 2, "um\n- [ ] dois\ntrês", 8},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, gotLine, gotCol := toggleCheckboxOnLine(tc.in, tc.line, tc.col)
			if got != tc.want || gotLine != tc.line || gotCol != tc.wantCol {
				t.Fatalf("got (%q, %d, %d), want (%q, %d, %d)", got, gotLine, gotCol, tc.want, tc.line, tc.wantCol)
			}
		})
	}
}

func TestContinueListOnEnter(t *testing.T) {
	cases := []struct {
		name        string
		in          string
		line, col   int
		want        string
		wantL, wCol int
		wantHandled bool
	}{
		{"plain text ignored", "olá mundo", 0, 9, "olá mundo", 0, 9, false},
		{"dash bullet", "- primeiro", 0, 10, "- primeiro\n- ", 1, 2, true},
		{"indented checklist checked resets to unchecked", "  - [x] feito", 0, 13, "  - [x] feito\n  - [ ] ", 1, 8, true},
		{"numbered list increments", "1. passo um", 0, 11, "1. passo um\n2. ", 1, 3, true},
		{"empty bullet exits list", "  - ", 0, 4, "", 0, 0, true},
		{"empty checklist exits list", "- [ ] ", 0, 6, "", 0, 0, true},
		{"mid-line split with utf8", "- ação rápida", 0, 6, "- ação\n-  rápida", 1, 2, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, nl, nc, handled := continueListOnEnter(tc.in, tc.line, tc.col)
			if handled != tc.wantHandled || got != tc.want || nl != tc.wantL || nc != tc.wCol {
				t.Fatalf("got (%q, %d, %d, %v), want (%q, %d, %d, %v)", got, nl, nc, handled, tc.want, tc.wantL, tc.wCol, tc.wantHandled)
			}
		})
	}
}

func TestLineManipulationAndStats(t *testing.T) {
	// moveLine up/down and bounds.
	got, nl, nc, moved := moveLine("a\nb\nc", 1, 1, -1)
	if !moved || got != "b\na\nc" || nl != 0 || nc != 1 {
		t.Fatalf("moveLine up = (%q, %d, %d, %v)", got, nl, nc, moved)
	}
	if _, _, _, moved := moveLine("a\nb", 0, 0, -1); moved {
		t.Fatalf("moveLine at top should not move")
	}

	// duplicateLine.
	got, nl, nc = duplicateLine("a\nb", 0, 1)
	if got != "a\na\nb" || nl != 1 || nc != 1 {
		t.Fatalf("duplicateLine = (%q, %d, %d)", got, nl, nc)
	}

	// deleteLine.
	got, nl, nc = deleteLine("a\nb\nc", 1, 0)
	if got != "a\nc" || nl != 1 || nc != 0 {
		t.Fatalf("deleteLine = (%q, %d, %d)", got, nl, nc)
	}
	got, nl, nc = deleteLine("única", 0, 3)
	if got != "" || nl != 0 || nc != 0 {
		t.Fatalf("deleteLine single = (%q, %d, %d)", got, nl, nc)
	}

	// indent / dedent.
	got, nl, nc = indentLine("texto", 0, 2, 2)
	if got != "  texto" || nl != 0 || nc != 4 {
		t.Fatalf("indentLine = (%q, %d, %d)", got, nl, nc)
	}
	got, nl, nc = dedentLine("  texto", 0, 4, 2)
	if got != "texto" || nl != 0 || nc != 2 {
		t.Fatalf("dedentLine = (%q, %d, %d)", got, nl, nc)
	}
	got, nl, nc = dedentLine("\ttexto", 0, 2, 2)
	if got != "texto" || nl != 0 || nc != 1 {
		t.Fatalf("dedentLine tab = (%q, %d, %d)", got, nl, nc)
	}

	// countEditorStats.
	words, runes, lines := countEditorStats("olá ação\nsegunda linha")
	if words != 4 || runes != 22 || lines != 2 {
		t.Fatalf("countEditorStats = (%d, %d, %d), want (4, 22, 2)", words, runes, lines)
	}
}

func TestTUIEditorErgonomicsKeybindings(t *testing.T) {
	store, v := newTestEnv(t)
	if _, err := v.CreateNote("", "Checklist", "um\ndois"); err != nil {
		t.Fatalf("CreateNote: %v", err)
	}
	m := NewModel(store, v, Options{DisableCursorBlink: true})
	m = runCmd(m, m.Init())
	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = next.(Model)

	m = sendKey(t, m, "tab")
	// Cursor is at end of line 1 ("dois"). Toggle checkbox with Alt+X.
	m = sendKey(t, m, "alt+x")
	if m.editor.Value() != "um\n- [ ] dois" {
		t.Fatalf("after Alt+X = %q", m.editor.Value())
	}

	// Enter continues checklist.
	m = sendKey(t, m, "enter")
	if m.editor.Value() != "um\n- [ ] dois\n- [ ] " {
		t.Fatalf("after Enter on checklist = %q", m.editor.Value())
	}

	// Enter on empty checklist clears marker.
	m = sendKey(t, m, "enter")
	if m.editor.Value() != "um\n- [ ] dois\n" {
		t.Fatalf("after Enter on empty checklist = %q", m.editor.Value())
	}

	// Move line up / duplicate / indent / dedent / delete line.
	m = sendKey(t, m, "alt+up")
	if m.editor.Value() != "um\n\n- [ ] dois" {
		t.Fatalf("after Alt+Up = %q", m.editor.Value())
	}
	setEditorCursor(&m.editor, 0, 2)
	m = sendKey(t, m, "alt+p")
	if m.editor.Value() != "um\num\n\n- [ ] dois" {
		t.Fatalf("after Alt+P = %q", m.editor.Value())
	}
	m = sendKey(t, m, "tab")
	if m.editor.Value() != "um\n  um\n\n- [ ] dois" {
		t.Fatalf("after Tab indent = %q", m.editor.Value())
	}
	m = sendKey(t, m, "alt+[")
	if m.editor.Value() != "um\num\n\n- [ ] dois" {
		t.Fatalf("after Alt+[ dedent = %q", m.editor.Value())
	}
	m = sendKey(t, m, "alt+k")
	if m.editor.Value() != "um\n\n- [ ] dois" {
		t.Fatalf("after Alt+K delete = %q", m.editor.Value())
	}
}

func TestTUIEditorEnterPastDefaultMaxHeight(t *testing.T) {
	store, v := newTestEnv(t)
	lines := make([]string, 200)
	for i := range lines {
		lines[i] = fmt.Sprintf("linha %d", i+1)
	}
	if _, err := v.CreateNote("", "Longa", strings.Join(lines, "\n")); err != nil {
		t.Fatalf("CreateNote: %v", err)
	}

	m := NewModel(store, v, Options{DisableCursorBlink: true})
	m = runCmd(m, m.Init())
	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = next.(Model)

	// Focus editor and split a plain line past bubbles/textarea's defaultMaxHeight (99).
	m = sendKey(t, m, "tab")
	setEditorCursor(&m.editor, 150, 5)
	m = sendKey(t, m, "enter")

	got := strings.Split(m.editor.Value(), "\n")
	if len(got) != 201 {
		t.Fatalf("line count after Enter = %d, want 201", len(got))
	}
	if got[150] != "linha" || got[151] != " 151" {
		t.Fatalf("split lines = (%q, %q), want (%q, %q)", got[150], got[151], "linha", " 151")
	}
	if !m.dirty {
		t.Fatalf("expected dirty = true after Enter")
	}
}
