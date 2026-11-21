package vault

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestDefaultTemplatesAreValidAndRenderCleanly(t *testing.T) {
	tpls, err := DefaultTemplates()
	if err != nil {
		t.Fatalf("DefaultTemplates: %v", err)
	}
	names := make([]string, 0, len(tpls))
	for _, tpl := range tpls {
		names = append(names, tpl.Name)
		if _, err := normalizeTemplateName(tpl.Name); err != nil {
			t.Fatalf("default template name %q invalid: %v", tpl.Name, err)
		}
		if !utf8.ValidString(tpl.Body) {
			t.Fatalf("default template %q is not valid UTF-8", tpl.Name)
		}
		if !strings.HasSuffix(tpl.Body, "\n") {
			t.Fatalf("default template %q does not end with a newline", tpl.Name)
		}
		rendered := RenderTemplate(tpl.Body, map[string]string{
			"title":    "Titulo",
			"notebook": "WORK",
			"date":     "2026-05-12",
			"datetime": "2026-05-12T10:00:00-03:00",
			"template": tpl.Name,
		})
		if strings.Contains(rendered, "{{") || strings.Contains(rendered, "}}") {
			t.Fatalf("default template %q left a placeholder after rendering: %q", tpl.Name, rendered)
		}
	}
	want := []string{"checklist", "daily", "decision", "meeting"}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("default template names = %v, want %v", names, want)
	}
}

func TestOpenDoesNotSeedDefaultTemplates(t *testing.T) {
	root := t.TempDir()
	v, err := Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if _, err := os.Stat(filepath.Join(v.Root(), TemplatesDirName)); !os.IsNotExist(err) {
		t.Fatalf("Open created .templates/ (stat err = %v)", err)
	}
}

func TestSeedDefaultTemplatesIfAbsent(t *testing.T) {
	v, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	created, err := v.SeedDefaultTemplatesIfAbsent()
	if err != nil {
		t.Fatalf("SeedDefaultTemplatesIfAbsent: %v", err)
	}
	want := []string{"checklist", "daily", "decision", "meeting"}
	if !reflect.DeepEqual(created, want) {
		t.Fatalf("created = %v, want %v", created, want)
	}
	tpls, err := v.ListTemplates()
	if err != nil {
		t.Fatalf("ListTemplates: %v", err)
	}
	if len(tpls) != 4 {
		t.Fatalf("ListTemplates returned %d templates, want 4", len(tpls))
	}

	// A custom edit survives a second absent-seed call.
	if _, err := v.SaveTemplate("daily", "custom daily\n"); err != nil {
		t.Fatalf("SaveTemplate: %v", err)
	}
	created, err = v.SeedDefaultTemplatesIfAbsent()
	if err != nil {
		t.Fatalf("second SeedDefaultTemplatesIfAbsent: %v", err)
	}
	if len(created) != 0 {
		t.Fatalf("second seed created %v, want none", created)
	}
	tpl, err := v.GetTemplate("daily")
	if err != nil {
		t.Fatalf("GetTemplate daily: %v", err)
	}
	if tpl.Body != "custom daily\n" {
		t.Fatalf("custom daily overwritten: %q", tpl.Body)
	}
}

func TestSeedDefaultTemplatesIfAbsentDoesNotResurrectDeletedTemplates(t *testing.T) {
	v, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if _, err := v.SeedDefaultTemplatesIfAbsent(); err != nil {
		t.Fatalf("SeedDefaultTemplatesIfAbsent: %v", err)
	}
	for _, name := range []string{"checklist", "daily", "decision", "meeting"} {
		if _, err := v.DeleteTemplate(name); err != nil {
			t.Fatalf("DeleteTemplate(%s): %v", name, err)
		}
	}
	tpls, err := v.ListTemplates()
	if err != nil {
		t.Fatalf("ListTemplates: %v", err)
	}
	if len(tpls) != 0 {
		t.Fatalf("expected empty templates after deleting all, got %v", tpls)
	}

	created, err := v.SeedDefaultTemplatesIfAbsent()
	if err != nil {
		t.Fatalf("SeedDefaultTemplatesIfAbsent after deletes: %v", err)
	}
	if len(created) != 0 {
		t.Fatalf("deleted defaults resurrected: %v", created)
	}
	tpls, err = v.ListTemplates()
	if err != nil {
		t.Fatalf("ListTemplates after deletes: %v", err)
	}
	if len(tpls) != 0 {
		t.Fatalf("expected still empty templates, got %v", tpls)
	}
}

func TestSeedDefaultTemplatesRestoresOnlyMissingAndPreservesEdits(t *testing.T) {
	v, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if _, err := v.SeedDefaultTemplatesIfAbsent(); err != nil {
		t.Fatalf("SeedDefaultTemplatesIfAbsent: %v", err)
	}
	if _, err := v.SaveTemplate("meeting", "edited meeting\n"); err != nil {
		t.Fatalf("SaveTemplate: %v", err)
	}
	for _, name := range []string{"daily", "decision"} {
		if _, err := v.DeleteTemplate(name); err != nil {
			t.Fatalf("DeleteTemplate(%s): %v", name, err)
		}
	}

	created, err := v.SeedDefaultTemplates()
	if err != nil {
		t.Fatalf("SeedDefaultTemplates: %v", err)
	}
	if !reflect.DeepEqual(created, []string{"daily", "decision"}) {
		t.Fatalf("created = %v, want [daily decision]", created)
	}
	meeting, err := v.GetTemplate("meeting")
	if err != nil {
		t.Fatalf("GetTemplate meeting: %v", err)
	}
	if meeting.Body != "edited meeting\n" {
		t.Fatalf("edited meeting overwritten: %q", meeting.Body)
	}
}

func TestSeedDefaultTemplatesCaseInsensitiveCollision(t *testing.T) {
	v, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if _, err := v.SaveTemplate("Daily", "custom case\n"); err != nil {
		t.Fatalf("SaveTemplate Daily: %v", err)
	}

	created, err := v.SeedDefaultTemplates()
	if err != nil {
		t.Fatalf("SeedDefaultTemplates: %v", err)
	}
	for _, name := range created {
		if strings.EqualFold(name, "daily") {
			t.Fatalf("SeedDefaultTemplates created %q despite Daily.txt", name)
		}
	}
	if _, err := os.Stat(filepath.Join(v.Root(), TemplatesDirName, "daily.txt")); !os.IsNotExist(err) {
		t.Fatalf("lowercase daily.txt was created (stat err = %v)", err)
	}
	tpl, err := v.GetTemplate("Daily")
	if err != nil {
		t.Fatalf("GetTemplate Daily: %v", err)
	}
	if tpl.Body != "custom case\n" {
		t.Fatalf("Daily.txt overwritten: %q", tpl.Body)
	}
}
