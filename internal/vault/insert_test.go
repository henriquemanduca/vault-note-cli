package vault

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func TestInsertSnippet(t *testing.T) {
	appendOpts := InsertOptions{Mode: InsertAppend}
	prependOpts := InsertOptions{Mode: InsertPrepend}

	cases := []struct {
		name    string
		content string
		snippet string
		opts    InsertOptions
		want    string
	}{
		{"append to empty content", "", "snip", appendOpts, "snip\n"},
		{"append content without newline", "corpo", "snip", appendOpts, "corpo\n\nsnip\n"},
		{"append content with newline", "corpo\n", "snip", appendOpts, "corpo\n\nsnip\n"},
		{"append snippet without newline", "corpo\n", "snip", appendOpts, "corpo\n\nsnip\n"},
		{"append keeps single blank line", "corpo\n\n", "snip", appendOpts, "corpo\n\n\nsnip\n"},
		{"prepend to content", "corpo", "snip", prependOpts, "snip\n\ncorpo"},
		{"prepend to empty content", "", "snip", prependOpts, "snip\n"},
		{"at 0", "corpo", "X", InsertOptions{Mode: InsertAtRune, RuneOffset: 0}, "Xcorpo\n"},
		{"at middle", "abcd", "X", InsertOptions{Mode: InsertAtRune, RuneOffset: 2}, "abXcd\n"},
		{"at end", "abcd", "X", InsertOptions{Mode: InsertAtRune, RuneOffset: 4}, "abcdX\n"},
		{"at clamps above length", "abcd", "X", InsertOptions{Mode: InsertAtRune, RuneOffset: 99}, "abcdX\n"},
		{"at clamps negative", "abcd", "X", InsertOptions{Mode: InsertAtRune, RuneOffset: -5}, "Xabcd\n"},
		{"at before trailing newline", "abcd\n", "X", InsertOptions{Mode: InsertAtRune, RuneOffset: 4}, "abcdX\n"},
		{"at after trailing newline", "abcd\n", "X", InsertOptions{Mode: InsertAtRune, RuneOffset: 5}, "abcd\nX\n"},
		{"empty snippet append", "corpo", "", appendOpts, "corpo"},
		{"empty snippet prepend", "corpo", "", prependOpts, "corpo"},
		{"empty snippet at", "corpo", "", InsertOptions{Mode: InsertAtRune, RuneOffset: 1}, "corpo"},
		{"content only newline append", "\n", "snip", appendOpts, "\n\nsnip\n"},
		{"snippet verbatim", "a\n", "- [ ] x\n\tindentado\n", appendOpts, "a\n\n- [ ] x\n\tindentado\n"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := InsertSnippet(tc.content, tc.snippet, tc.opts); got != tc.want {
				t.Fatalf("InsertSnippet(%q, %q) = %q, want %q", tc.content, tc.snippet, got, tc.want)
			}
		})
	}
}

func TestInsertSnippetAtEveryRuneOffsetKeepsValidUTF8(t *testing.T) {
	content := "ação çã 😀"
	snippet := "«x»"
	runeLen := len([]rune(content))

	for offset := -2; offset <= runeLen+2; offset++ {
		got := InsertSnippet(content, snippet, InsertOptions{Mode: InsertAtRune, RuneOffset: offset})
		if !utf8.ValidString(got) {
			t.Fatalf("offset %d produced invalid UTF-8: %q", offset, got)
		}
		clamped := offset
		if clamped < 0 {
			clamped = 0
		}
		if clamped > runeLen {
			clamped = runeLen
		}
		runes := []rune(content)
		want := string(runes[:clamped]) + snippet + string(runes[clamped:]) + "\n"
		if got != want {
			t.Fatalf("offset %d = %q, want %q", offset, got, want)
		}
	}
}

func TestInsertTemplateModesAndVariables(t *testing.T) {
	v := newFixedClockVault(t)
	if _, err := v.CreateNotebook("WORK"); err != nil {
		t.Fatalf("CreateNotebook: %v", err)
	}
	if _, err := v.SaveTemplate("checklist", "== {{title}} — checklist ==\n- {{notebook}} {{date}}\n"); err != nil {
		t.Fatalf("SaveTemplate: %v", err)
	}
	note, err := v.CreateNote("WORK", "Projeto X", "Corpo existente")
	if err != nil {
		t.Fatalf("CreateNote: %v", err)
	}

	got, err := v.InsertTemplate(note.ID, "checklist", InsertOptions{Mode: InsertAppend})
	if err != nil {
		t.Fatalf("InsertTemplate append: %v", err)
	}
	want := "Corpo existente\n\n== Projeto X — checklist ==\n- WORK 2026-05-12\n"
	if got.Content != want {
		t.Fatalf("append content = %q, want %q", got.Content, want)
	}

	got, err = v.InsertTemplate(note.ID, "checklist", InsertOptions{Mode: InsertPrepend})
	if err != nil {
		t.Fatalf("InsertTemplate prepend: %v", err)
	}
	if !strings.HasPrefix(got.Content, "== Projeto X — checklist ==\n- WORK 2026-05-12\n\n") {
		t.Fatalf("prepend content = %q", got.Content)
	}

	got, err = v.InsertTemplate(note.ID, "checklist", InsertOptions{Mode: InsertAtRune, RuneOffset: 0})
	if err != nil {
		t.Fatalf("InsertTemplate at 0: %v", err)
	}
	if !strings.HasPrefix(got.Content, "== Projeto X — checklist ==") {
		t.Fatalf("at-0 content = %q", got.Content)
	}

	// The file on disk matches the returned note.
	raw, err := os.ReadFile(got.Path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(raw) != got.Content {
		t.Fatalf("on-disk content differs from returned note")
	}
}

func TestInsertTemplateMissingTargetsWriteNothing(t *testing.T) {
	v := newTestVault(t)
	if _, err := v.SaveTemplate("checklist", "corpo\n"); err != nil {
		t.Fatalf("SaveTemplate: %v", err)
	}
	note, err := v.CreateNote("", "Alvo", "original")
	if err != nil {
		t.Fatalf("CreateNote: %v", err)
	}

	if _, err := v.InsertTemplate(note.ID, "inexistente", InsertOptions{}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound for missing template, got %v", err)
	}
	if _, err := v.InsertTemplate("inexistente.txt", "checklist", InsertOptions{}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound for missing note, got %v", err)
	}
	raw, err := os.ReadFile(note.Path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(raw) != "original" {
		t.Fatalf("note was modified on failure: %q", string(raw))
	}

	// The templates directory must not gain anything either.
	entries, err := os.ReadDir(filepath.Join(v.Root(), TemplatesDirName))
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 template file, got %d", len(entries))
	}
}

func TestInsertTemplateReinsertSucceeds(t *testing.T) {
	v := newTestVault(t)
	if _, err := v.SaveTemplate("checklist", "== checklist ==\n- item\n"); err != nil {
		t.Fatalf("SaveTemplate: %v", err)
	}
	note, err := v.CreateNote("", "Projeto X", "corpo")
	if err != nil {
		t.Fatalf("CreateNote: %v", err)
	}

	first, err := v.InsertTemplate(note.ID, "checklist", InsertOptions{})
	if err != nil {
		t.Fatalf("first insert: %v", err)
	}
	second, err := v.InsertTemplate(note.ID, "checklist", InsertOptions{})
	if err != nil {
		t.Fatalf("re-insert should succeed: %v", err)
	}
	if strings.Count(second.Content, "== checklist ==") != 2 {
		t.Fatalf("expected two copies after re-insert, got %q", second.Content)
	}
	if !strings.HasPrefix(second.Content, first.Content) {
		t.Fatalf("re-insert should preserve the first copy")
	}
	raw, err := os.ReadFile(first.Path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(raw) != second.Content {
		t.Fatalf("on-disk content differs from returned note")
	}
}

func TestInsertTemplateUsesVaultClock(t *testing.T) {
	v := newTestVault(t)
	if _, err := v.SaveTemplate("diario", "{{date}} {{datetime}}\n"); err != nil {
		t.Fatalf("SaveTemplate: %v", err)
	}
	note, err := v.CreateNote("", "Dia", "")
	if err != nil {
		t.Fatalf("CreateNote: %v", err)
	}
	fixed := time.Date(2030, 1, 2, 3, 4, 5, 0, time.Local)
	v.now = func() time.Time { return fixed }

	got, err := v.InsertTemplate(note.ID, "diario", InsertOptions{})
	if err != nil {
		t.Fatalf("InsertTemplate: %v", err)
	}
	want := "2030-01-02 " + fixed.Format(time.RFC3339) + "\n"
	if got.Content != want {
		t.Fatalf("content = %q, want %q", got.Content, want)
	}
}
