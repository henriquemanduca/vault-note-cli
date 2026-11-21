package vault

import (
	"strings"
	"time"
)

// InsertMode selects where a rendered template snippet is placed in a note.
type InsertMode int

const (
	// InsertAppend places the snippet at the end of the note (CLI default).
	InsertAppend InsertMode = iota
	// InsertPrepend places the snippet at the start of the note.
	InsertPrepend
	// InsertAtRune places the snippet at a rune offset inside the note.
	InsertAtRune
)

// InsertOptions configures snippet composition.
type InsertOptions struct {
	Mode       InsertMode
	RuneOffset int // used when Mode == InsertAtRune; clamped to the content length
}

// InsertSnippet composes content and snippet according to opts. It is pure and
// rune-safe: offsets are counted in runes, never bytes, so UTF-8 stays valid.
func InsertSnippet(content, snippet string, opts InsertOptions) string {
	if snippet == "" {
		return content
	}

	switch opts.Mode {
	case InsertPrepend:
		head := ensureTrailingNewline(snippet)
		if content == "" {
			return head
		}
		return head + "\n" + content

	case InsertAtRune:
		runes := []rune(content)
		offset := opts.RuneOffset
		if offset < 0 {
			offset = 0
		}
		if offset > len(runes) {
			offset = len(runes)
		}
		out := string(runes[:offset]) + snippet + string(runes[offset:])
		return ensureTrailingNewline(out)

	default: // InsertAppend
		body := ensureTrailingNewline(snippet)
		if content == "" {
			return body
		}
		head := ensureTrailingNewline(content)
		return head + "\n" + body
	}
}

func ensureTrailingNewline(s string) string {
	if strings.HasSuffix(s, "\n") {
		return s
	}
	return s + "\n"
}

// RenderTemplateFor renders a template for an existing note without writing.
// Placeholders use the note's current title/notebook and the vault clock.
func (v *Vault) RenderTemplateFor(note Note, templateName string) (string, error) {
	tpl, err := v.GetTemplate(templateName)
	if err != nil {
		return "", err
	}
	return v.renderTemplateFor(note, tpl), nil
}

func (v *Vault) renderTemplateFor(note Note, tpl Template) string {
	vars := map[string]string{
		"title":    note.Title,
		"notebook": note.Notebook,
		"date":     v.clock().Format("2006-01-02"),
		"datetime": v.clock().Format(time.RFC3339),
		"template": tpl.Name,
	}
	return RenderTemplate(tpl.Body, vars)
}

// InsertTemplate renders a template into an existing note and writes it
// atomically. Missing notes/templates fail before any write, so the note is
// never left half-modified. Re-inserting the same template is allowed.
func (v *Vault) InsertTemplate(noteID, templateName string, opts InsertOptions) (Note, error) {
	note, err := v.GetNote(noteID)
	if err != nil {
		return Note{}, err
	}
	tpl, err := v.GetTemplate(templateName)
	if err != nil {
		return Note{}, err
	}
	rendered := v.renderTemplateFor(note, tpl)
	composed := InsertSnippet(note.Content, rendered, opts)
	return v.UpdateNoteContent(note.ID, composed)
}
