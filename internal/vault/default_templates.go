package vault

import (
	"embed"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

//go:embed defaults/checklist.txt defaults/daily.txt defaults/decision.txt defaults/meeting.txt
var defaultTemplatesFS embed.FS

// DefaultTemplates returns the built-in starter templates sorted by name
// (case-insensitive). Returned copies are read-only snapshots from embed.FS.
func DefaultTemplates() ([]Template, error) {
	entries, err := defaultTemplatesFS.ReadDir("defaults")
	if err != nil {
		return nil, fmt.Errorf("read embedded default templates: %w", err)
	}

	out := make([]Template, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(name), NoteExt) {
			continue
		}
		raw, err := defaultTemplatesFS.ReadFile(path.Join("defaults", name))
		if err != nil {
			return nil, fmt.Errorf("read embedded default template %q: %w", name, err)
		}
		clean, err := normalizeTemplateName(strings.TrimSuffix(name, filepath.Ext(name)))
		if err != nil {
			return nil, fmt.Errorf("invalid embedded default template %q: %w", name, err)
		}
		out = append(out, Template{
			Name: clean,
			Path: path.Join("defaults", name),
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

// SeedDefaultTemplatesIfAbsent creates <vault>/.templates/ and writes all
// built-in starter templates ONLY when the .templates/ directory does not yet
// exist. If .templates/ already exists (even if empty after the user deleted
// templates via `template delete`), it returns (nil, nil) without writing.
func (v *Vault) SeedDefaultTemplatesIfAbsent() ([]string, error) {
	dir := v.templatesDir()
	if _, err := os.Stat(dir); err == nil {
		return nil, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("stat templates directory %s: %w", dir, err)
	}
	return v.SeedDefaultTemplates()
}

// SeedDefaultTemplates ensures <vault>/.templates/ exists and writes any
// built-in starter templates that are not already present (case-insensitive
// match against existing templates). Existing files are never overwritten.
// Returns the sorted names of templates created in this call.
func (v *Vault) SeedDefaultTemplates() ([]string, error) {
	defaults, err := DefaultTemplates()
	if err != nil {
		return nil, err
	}
	existing, err := v.ListTemplates()
	if err != nil {
		return nil, err
	}
	present := make(map[string]struct{}, len(existing))
	for _, tpl := range existing {
		present[strings.ToLower(tpl.Name)] = struct{}{}
	}

	created := make([]string, 0, len(defaults))
	for _, tpl := range defaults {
		if _, ok := present[strings.ToLower(tpl.Name)]; ok {
			continue
		}
		if _, err := v.SaveTemplate(tpl.Name, tpl.Body); err != nil {
			return created, err
		}
		created = append(created, tpl.Name)
	}
	return created, nil
}
