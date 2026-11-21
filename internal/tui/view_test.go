package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func TestFormatNotebookPrefix(t *testing.T) {
	cases := []struct {
		name     string
		notebook string
		want     string
	}{
		{"empty", "", ""},
		{"spaces", "   ", ""},
		{"short", "go", "go/"},
		{"three", "abc", "abc/"},
		{"long", "Pessoal", "Pes/"},
		{"trims", "  WORK  ", "WOR/"},
		{"multibyte", "Ação", "Açã/"},
		{"multibyte long", "AçãoLonga", "Açã/"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := formatNotebookPrefix(tc.notebook); got != tc.want {
				t.Fatalf("formatNotebookPrefix(%q) = %q, want %q", tc.notebook, got, tc.want)
			}
		})
	}
}

func TestSidebarNotebookPrefixVisibility(t *testing.T) {
	store, v := newTestEnv(t)
	if _, err := v.CreateNotebook("WORK"); err != nil {
		t.Fatalf("CreateNotebook WORK: %v", err)
	}
	if _, err := v.CreateNotebook("Pessoal"); err != nil {
		t.Fatalf("CreateNotebook Pessoal: %v", err)
	}
	if _, err := v.CreateNote("WORK", "reuniao", "corpo"); err != nil {
		t.Fatalf("CreateNote WORK: %v", err)
	}
	if _, err := v.CreateNote("Pessoal", "compras", "corpo"); err != nil {
		t.Fatalf("CreateNote Pessoal: %v", err)
	}
	if _, err := v.CreateNote("", "solta", "corpo"); err != nil {
		t.Fatalf("CreateNote root: %v", err)
	}

	m := NewModel(store, v, Options{DisableCursorBlink: true})
	m = runCmd(m, m.Init())
	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = next.(Model)

	render := func() string {
		return ansi.Strip(m.renderLeftPane(40, 20))
	}

	// Enabled and unfiltered: notebook notes get up to 3 runes; root does not.
	m.showNotebookPrefix = true
	m.filterNotebook = ""
	out := render()
	for _, want := range []string{"WOR/reuniao", "PES/compras", "  solta"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in sidebar:\n%s", want, out)
		}
	}
	if strings.Contains(out, "WOR/solta") {
		t.Fatalf("root note must not receive a notebook prefix:\n%s", out)
	}

	// Filtered view hides prefixes.
	m.filterNotebook = "WORK"
	out = render()
	if strings.Contains(out, "WOR/") || strings.Contains(out, "PES/") {
		t.Fatalf("filtered view must hide prefixes:\n%s", out)
	}

	// Disabled hides prefixes.
	m.filterNotebook = ""
	m.showNotebookPrefix = false
	out = render()
	if strings.Contains(out, "WOR/") || strings.Contains(out, "PES/") {
		t.Fatalf("disabled view must hide prefixes:\n%s", out)
	}

	// Narrow width must not panic and still truncate safely.
	m.showNotebookPrefix = true
	_ = render()
	_ = ansi.Strip(m.renderLeftPane(12, 10))
}
