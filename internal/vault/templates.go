package vault

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// TemplatesDirName is the hidden directory inside the vault root that stores
// note templates. Leading dot keeps it out of notebooks, note listings and
// trash; templates are not notes.
const TemplatesDirName = ".templates"

// Template is a reusable plain-text skeleton stored as <vault>/.templates/<name>.txt.
type Template struct {
	Name string // Basename without .txt extension
	Path string // Absolute path on disk
	Body string // Raw UTF-8 template body with {{placeholder}} tokens
}

func (v *Vault) templatesDir() string {
	return filepath.Join(v.root, TemplatesDirName)
}

// normalizeTemplateName accepts "reuniao" or "reuniao.txt" and validates the
// result against the same portable-name rules used for notebooks and notes.
func normalizeTemplateName(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if len(s) > len(NoteExt) && strings.EqualFold(s[len(s)-len(NoteExt):], NoteExt) {
		s = s[:len(s)-len(NoteExt)]
	}
	return validateName(s, "template name")
}

func (v *Vault) templatePath(name string) string {
	return filepath.Join(v.templatesDir(), name+NoteExt)
}

// ListTemplates returns all templates sorted by name (case-insensitive).
// A missing .templates directory is an empty list, not an error.
func (v *Vault) ListTemplates() ([]Template, error) {
	dir := v.templatesDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read templates directory %s: %w", dir, err)
	}

	out := make([]Template, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || strings.HasPrefix(name, ".") {
			continue
		}
		ext := filepath.Ext(name)
		if len(name) <= len(ext) || !strings.EqualFold(ext, NoteExt) {
			continue
		}
		absPath := filepath.Join(dir, name)
		raw, err := os.ReadFile(absPath)
		if err != nil {
			return nil, fmt.Errorf("read template %q: %w", name, err)
		}
		out = append(out, Template{
			Name: strings.TrimSuffix(name, ext),
			Path: absPath,
			Body: string(raw),
		})
	}

	sort.Slice(out, func(i, j int) bool {
		li := strings.ToLower(out[i].Name)
		lj := strings.ToLower(out[j].Name)
		if li != lj {
			return li < lj
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

// GetTemplate loads one template by name, with or without the .txt suffix.
func (v *Vault) GetTemplate(name string) (Template, error) {
	clean, err := normalizeTemplateName(name)
	if err != nil {
		return Template{}, err
	}
	absPath := v.templatePath(clean)
	if err := v.ensureInsideRoot(absPath); err != nil {
		return Template{}, err
	}
	raw, err := os.ReadFile(absPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// Case-insensitive fallback, mirroring GetNotebook behavior.
			if folded, foldErr := v.findTemplateFold(clean); foldErr == nil {
				return folded, nil
			}
			return Template{}, fmt.Errorf("%w: template %q", ErrNotFound, clean)
		}
		return Template{}, fmt.Errorf("read template %q: %w", clean, err)
	}
	return Template{Name: clean, Path: absPath, Body: string(raw)}, nil
}

// findTemplateFold resolves a template name ignoring ASCII case.
func (v *Vault) findTemplateFold(name string) (Template, error) {
	tpls, err := v.ListTemplates()
	if err != nil {
		return Template{}, err
	}
	for _, tpl := range tpls {
		if strings.EqualFold(tpl.Name, name) {
			return tpl, nil
		}
	}
	return Template{}, fmt.Errorf("%w: template %q", ErrNotFound, name)
}

// SaveTemplate creates or overwrites a template atomically.
func (v *Vault) SaveTemplate(name, body string) (Template, error) {
	clean, err := normalizeTemplateName(name)
	if err != nil {
		return Template{}, err
	}
	absPath := v.templatePath(clean)
	if err := v.ensureInsideRoot(absPath); err != nil {
		return Template{}, err
	}
	if err := os.MkdirAll(v.templatesDir(), 0o755); err != nil {
		return Template{}, fmt.Errorf("create templates directory: %w", err)
	}
	if err := writeFileAtomic(absPath, body); err != nil {
		return Template{}, fmt.Errorf("save template %q: %w", clean, err)
	}
	return v.GetTemplate(clean)
}

// CreateTemplate creates a template only when the name is free. A duplicate
// name (exact or case-insensitive) returns ErrAlreadyExists unless force is set,
// in which case the existing template is overwritten without creating a
// case-variant duplicate. The body is stored verbatim.
func (v *Vault) CreateTemplate(name, body string, force bool) (Template, error) {
	clean, err := normalizeTemplateName(name)
	if err != nil {
		return Template{}, err
	}
	target := clean
	if existing, err := v.GetTemplate(clean); err == nil {
		if !force {
			return Template{}, fmt.Errorf("%w: template %q", ErrAlreadyExists, existing.Name)
		}
		target = existing.Name
	} else if !errors.Is(err, ErrNotFound) {
		return Template{}, err
	}
	return v.SaveTemplate(target, body)
}

// CreateTemplateFromNote copies a note's current content verbatim into a new
// template. The note is resolved before any template write and is never
// mutated or placeholder-rewritten.
func (v *Vault) CreateTemplateFromNote(noteRef, notebook, templateName string, force bool) (Template, error) {
	note, err := v.ResolveNote(noteRef, notebook)
	if err != nil {
		return Template{}, err
	}
	return v.CreateTemplate(templateName, note.Content, force)
}

// DeleteTemplate moves a template file into <vault>/.trash/.
func (v *Vault) DeleteTemplate(name string) (TrashResult, error) {
	tpl, err := v.GetTemplate(name)
	if err != nil {
		return TrashResult{}, err
	}
	trashPath, err := v.moveToTrash(TemplatesDirName+"/"+tpl.Name+NoteExt, tpl.Path)
	if err != nil {
		return TrashResult{}, err
	}
	return TrashResult{OriginalID: tpl.Name, TrashPath: trashPath}, nil
}

// CreateNoteFromTemplate renders the named template and creates the note.
// The template is resolved before any write, so a missing template never
// leaves a half-created note behind.
func (v *Vault) CreateNoteFromTemplate(notebook, title, templateName string) (Note, error) {
	tpl, err := v.GetTemplate(templateName)
	if err != nil {
		return Note{}, err
	}
	vars := map[string]string{
		"title":    title,
		"notebook": notebook,
		"date":     v.clock().Format("2006-01-02"),
		"datetime": v.clock().Format(time.RFC3339),
		"template": tpl.Name,
	}
	return v.CreateNote(notebook, title, RenderTemplate(tpl.Body, vars))
}

// RenderTemplate replaces {{name}} tokens found in vars with their values.
// Unknown or empty tokens are kept literally; substituted values are never
// re-scanned, so values may safely contain braces.
func RenderTemplate(body string, vars map[string]string) string {
	if !strings.Contains(body, "{{") {
		return body
	}
	var b strings.Builder
	b.Grow(len(body))
	rest := body
	for {
		start := strings.Index(rest, "{{")
		if start < 0 {
			b.WriteString(rest)
			break
		}
		b.WriteString(rest[:start])
		after := rest[start+2:]
		end := strings.Index(after, "}}")
		if end < 0 {
			b.WriteString(rest[start:])
			break
		}
		key := strings.TrimSpace(after[:end])
		if val, ok := vars[key]; ok && key != "" {
			b.WriteString(val)
		} else {
			b.WriteString(rest[start : start+2+end+2])
		}
		rest = after[end+2:]
	}
	return b.String()
}
