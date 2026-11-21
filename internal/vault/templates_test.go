package vault

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newFixedClockVault(t *testing.T) *Vault {
	t.Helper()
	v := newTestVault(t)
	fixed := time.Date(2026, 5, 12, 9, 30, 0, 0, time.Local)
	v.now = func() time.Time { return fixed }
	return v
}

func TestListTemplatesEmptyAndOrdered(t *testing.T) {
	v := newTestVault(t)

	tpls, err := v.ListTemplates()
	if err != nil {
		t.Fatalf("ListTemplates on missing dir: %v", err)
	}
	if len(tpls) != 0 {
		t.Fatalf("expected no templates, got %+v", tpls)
	}

	for _, name := range []string{"zebra", "Alfa", "meio"} {
		if _, err := v.SaveTemplate(name, "corpo "+name); err != nil {
			t.Fatalf("SaveTemplate(%q): %v", name, err)
		}
	}
	// Noise that must be ignored: non-.txt file, hidden file, subdirectory.
	dir := filepath.Join(v.Root(), TemplatesDirName)
	if err := os.WriteFile(filepath.Join(dir, "notas.md"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".oculto.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write hidden: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatalf("mkdir sub: %v", err)
	}

	tpls, err = v.ListTemplates()
	if err != nil {
		t.Fatalf("ListTemplates: %v", err)
	}
	got := make([]string, len(tpls))
	for i, tpl := range tpls {
		got[i] = tpl.Name
	}
	want := []string{"Alfa", "meio", "zebra"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("template order = %v, want %v", got, want)
	}
	if tpls[0].Body != "corpo Alfa" {
		t.Fatalf("unexpected body: %q", tpls[0].Body)
	}
}

func TestSaveAndGetTemplate(t *testing.T) {
	v := newTestVault(t)

	saved, err := v.SaveTemplate("reuniao.txt", "v1")
	if err != nil {
		t.Fatalf("SaveTemplate: %v", err)
	}
	if saved.Name != "reuniao" {
		t.Fatalf("expected .txt suffix stripped, got %q", saved.Name)
	}
	if _, err := v.SaveTemplate("reuniao", "v2"); err != nil {
		t.Fatalf("SaveTemplate overwrite: %v", err)
	}
	got, err := v.GetTemplate("REUNIAO")
	if err != nil {
		t.Fatalf("GetTemplate: %v", err)
	}
	if got.Body != "v2" {
		t.Fatalf("expected overwritten body, got %q", got.Body)
	}

	if _, err := v.GetTemplate("inexistente"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestNormalizeTemplateNameRejectsUnsafeNames(t *testing.T) {
	for _, bad := range []string{"", "  ", "../x", ".hidden", "a/b", `a\b`, "x:y", ".."} {
		if _, err := normalizeTemplateName(bad); err == nil {
			t.Fatalf("expected error for %q", bad)
		}
	}
	got, err := normalizeTemplateName("reuniao.TXT")
	if err != nil || got != "reuniao" {
		t.Fatalf("normalizeTemplateName(reuniao.TXT) = (%q, %v)", got, err)
	}
}

func TestRenderTemplate(t *testing.T) {
	vars := map[string]string{
		"title":    "Reunião",
		"notebook": "WORK",
		"date":     "2026-05-12",
		"datetime": "2026-05-12T09:30:00-03:00",
		"template": "reuniao",
	}

	body := "{{title}} | {{ notebook }} | {{date}} | {{datetime}} | {{template}}"
	want := "Reunião | WORK | 2026-05-12 | 2026-05-12T09:30:00-03:00 | reuniao"
	if got := RenderTemplate(body, vars); got != want {
		t.Fatalf("RenderTemplate = %q, want %q", got, want)
	}

	// Unknown and malformed tokens stay literal.
	if got := RenderTemplate("a {{nope}} b {{ }} c {{title", vars); got != "a {{nope}} b {{ }} c {{title" {
		t.Fatalf("unexpected literal handling: %q", got)
	}

	// Substituted values are not re-scanned.
	nested := map[string]string{"title": "{{date}}", "date": "2026-05-12"}
	if got := RenderTemplate("{{title}}", nested); got != "{{date}}" {
		t.Fatalf("expected no recursion, got %q", got)
	}

	// No tokens: returned unchanged.
	if got := RenderTemplate("sem placeholders", vars); got != "sem placeholders" {
		t.Fatalf("unexpected passthrough: %q", got)
	}
}

func TestCreateNoteFromTemplateRendersAndPlacesNote(t *testing.T) {
	v := newFixedClockVault(t)
	if _, err := v.CreateNotebook("WORK"); err != nil {
		t.Fatalf("CreateNotebook: %v", err)
	}
	if _, err := v.SaveTemplate("reuniao", "Nota: {{title}} ({{template}})\nData: {{date}}\nCaderno: {{notebook}}\n"); err != nil {
		t.Fatalf("SaveTemplate: %v", err)
	}

	note, err := v.CreateNoteFromTemplate("WORK", "Reuniao 12-05", "reuniao")
	if err != nil {
		t.Fatalf("CreateNoteFromTemplate: %v", err)
	}
	if note.ID != "WORK/Reuniao 12-05.txt" {
		t.Fatalf("unexpected note ID: %q", note.ID)
	}
	want := "Nota: Reuniao 12-05 (reuniao)\nData: 2026-05-12\nCaderno: WORK\n"
	if note.Content != want {
		t.Fatalf("content = %q, want %q", note.Content, want)
	}

	root, err := v.CreateNoteFromTemplate("", "Solta", "reuniao")
	if err != nil {
		t.Fatalf("CreateNoteFromTemplate root: %v", err)
	}
	if root.Notebook != "" || !strings.Contains(root.Content, "Caderno: \n") {
		t.Fatalf("unexpected root note: %+v", root)
	}
}

func TestCreateNoteFromTemplateMissingTemplateWritesNothing(t *testing.T) {
	v := newTestVault(t)
	if _, err := v.CreateNotebook("WORK"); err != nil {
		t.Fatalf("CreateNotebook: %v", err)
	}

	_, err := v.CreateNoteFromTemplate("WORK", "Fantasma", "inexistente")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
	entries, err := os.ReadDir(filepath.Join(v.Root(), "WORK"))
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected no note created, found %d entries", len(entries))
	}
}

func TestDeleteTemplateMovesToTrash(t *testing.T) {
	v := newTestVault(t)
	if _, err := v.SaveTemplate("rascunho", "x"); err != nil {
		t.Fatalf("SaveTemplate: %v", err)
	}

	res, err := v.DeleteTemplate("rascunho")
	if err != nil {
		t.Fatalf("DeleteTemplate: %v", err)
	}
	if res.OriginalID != "rascunho" {
		t.Fatalf("unexpected OriginalID: %q", res.OriginalID)
	}
	if !strings.Contains(filepath.ToSlash(res.TrashPath), "/.trash/") ||
		!strings.HasSuffix(filepath.ToSlash(res.TrashPath), "/.templates/rascunho.txt") {
		t.Fatalf("unexpected trash path: %q", res.TrashPath)
	}
	if _, err := os.Stat(res.TrashPath); err != nil {
		t.Fatalf("expected trashed file to exist: %v", err)
	}
	if _, err := v.GetTemplate("rascunho"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected template gone, got %v", err)
	}
}

func TestTemplatesAreNotNotebooksOrNotes(t *testing.T) {
	v := newTestVault(t)
	if _, err := v.SaveTemplate("reuniao", "x"); err != nil {
		t.Fatalf("SaveTemplate: %v", err)
	}

	nbs, err := v.ListNotebooks()
	if err != nil {
		t.Fatalf("ListNotebooks: %v", err)
	}
	if len(nbs) != 0 {
		t.Fatalf("expected .templates hidden from notebooks, got %+v", nbs)
	}
	notes, err := v.ListNotes(NoteFilter{})
	if err != nil {
		t.Fatalf("ListNotes: %v", err)
	}
	if len(notes) != 0 {
		t.Fatalf("expected no notes, got %+v", notes)
	}
}

func TestCreateTemplateNewAndDuplicate(t *testing.T) {
	v := newTestVault(t)

	tpl, err := v.CreateTemplate("rascunho", "corpo {{title}}", false)
	if err != nil {
		t.Fatalf("CreateTemplate new: %v", err)
	}
	if tpl.Name != "rascunho" || tpl.Body != "corpo {{title}}" {
		t.Fatalf("unexpected template: %+v", tpl)
	}

	if _, err := v.CreateTemplate("rascunho", "outro", false); !errors.Is(err, ErrAlreadyExists) {
		t.Fatalf("expected ErrAlreadyExists, got %v", err)
	}
	got, err := v.GetTemplate("rascunho")
	if err != nil {
		t.Fatalf("GetTemplate: %v", err)
	}
	if got.Body != "corpo {{title}}" {
		t.Fatalf("duplicate create overwrote body: %q", got.Body)
	}

	if _, err := v.CreateTemplate("rascunho", "forcado", true); err != nil {
		t.Fatalf("CreateTemplate force: %v", err)
	}
	got, err = v.GetTemplate("rascunho")
	if err != nil {
		t.Fatalf("GetTemplate after force: %v", err)
	}
	if got.Body != "forcado" {
		t.Fatalf("force did not overwrite body: %q", got.Body)
	}
}

func TestCreateTemplateCaseFoldCollision(t *testing.T) {
	v := newTestVault(t)
	if _, err := v.SaveTemplate("Daily", "original"); err != nil {
		t.Fatalf("SaveTemplate Daily: %v", err)
	}

	if _, err := v.CreateTemplate("daily", "novo", false); !errors.Is(err, ErrAlreadyExists) {
		t.Fatalf("expected ErrAlreadyExists for case-fold collision, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(v.Root(), TemplatesDirName, "daily.txt")); !os.IsNotExist(err) {
		t.Fatalf("case-variant duplicate was created")
	}

	created, err := v.CreateTemplate("daily", "substituido", true)
	if err != nil {
		t.Fatalf("CreateTemplate force case-fold: %v", err)
	}
	if created.Name != "Daily" {
		t.Fatalf("force should preserve existing casing, got %q", created.Name)
	}
	if _, err := os.Stat(filepath.Join(v.Root(), TemplatesDirName, "daily.txt")); !os.IsNotExist(err) {
		t.Fatalf("force created a case-variant duplicate")
	}
	got, err := v.GetTemplate("Daily")
	if err != nil {
		t.Fatalf("GetTemplate Daily: %v", err)
	}
	if got.Body != "substituido" {
		t.Fatalf("force did not overwrite existing casing: %q", got.Body)
	}
}

func TestCreateTemplateFromNoteLeavesNoteUntouched(t *testing.T) {
	v := newTestVault(t)
	if _, err := v.CreateNotebook("WORK"); err != nil {
		t.Fatalf("CreateNotebook: %v", err)
	}
	body := "Titulo: {{title}}\nData: {{date}}\n\n- [ ] item\n"
	note, err := v.CreateNote("WORK", "Modelo", body)
	if err != nil {
		t.Fatalf("CreateNote: %v", err)
	}

	tpl, err := v.CreateTemplateFromNote("WORK/Modelo", "", "modelo", false)
	if err != nil {
		t.Fatalf("CreateTemplateFromNote: %v", err)
	}
	if tpl.Body != body {
		t.Fatalf("template body = %q, want verbatim %q", tpl.Body, body)
	}
	after, err := v.GetNote(note.ID)
	if err != nil {
		t.Fatalf("GetNote after: %v", err)
	}
	if after.Content != body {
		t.Fatalf("original note mutated: %q", after.Content)
	}
	if after.UpdatedAt != note.UpdatedAt {
		t.Fatalf("original note timestamp changed: %q -> %q", note.UpdatedAt, after.UpdatedAt)
	}
}

func TestCreateTemplateFromNoteResolvesNotebookAndEmptyBody(t *testing.T) {
	v := newTestVault(t)
	if _, err := v.CreateNotebook("WORK"); err != nil {
		t.Fatalf("CreateNotebook: %v", err)
	}
	if _, err := v.CreateNote("WORK", "Vazia", ""); err != nil {
		t.Fatalf("CreateNote: %v", err)
	}

	tpl, err := v.CreateTemplateFromNote("Vazia", "WORK", "vazia", false)
	if err != nil {
		t.Fatalf("CreateTemplateFromNote with notebook: %v", err)
	}
	if tpl.Body != "" {
		t.Fatalf("empty note produced non-empty template: %q", tpl.Body)
	}
}

func TestCreateTemplateFromNoteErrorsBeforeWrite(t *testing.T) {
	v := newTestVault(t)
	if _, err := v.CreateNotebook("A"); err != nil {
		t.Fatalf("CreateNotebook A: %v", err)
	}
	if _, err := v.CreateNotebook("B"); err != nil {
		t.Fatalf("CreateNotebook B: %v", err)
	}
	if _, err := v.CreateNote("A", "Dup", "a"); err != nil {
		t.Fatalf("CreateNote A: %v", err)
	}
	if _, err := v.CreateNote("B", "Dup", "b"); err != nil {
		t.Fatalf("CreateNote B: %v", err)
	}

	if _, err := v.CreateTemplateFromNote("Inexistente", "", "t", false); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
	if _, err := v.CreateTemplateFromNote("Dup", "", "t", false); !errors.Is(err, ErrAmbiguous) {
		t.Fatalf("expected ErrAmbiguous, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(v.Root(), TemplatesDirName)); !os.IsNotExist(err) {
		t.Fatalf("failed note resolution created .templates/")
	}
}
