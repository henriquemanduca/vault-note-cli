package vault

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newTestVault(t *testing.T) *Vault {
	t.Helper()
	v, err := Open(filepath.Join(t.TempDir(), "vault"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return v
}

func TestVaultNotebooksCRUDAndTrash(t *testing.T) {
	v := newTestVault(t)

	// Hidden folders and non-txt files should be ignored.
	if err := os.MkdirAll(filepath.Join(v.Root(), ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	if err := os.WriteFile(filepath.Join(v.Root(), "ignore.md"), []byte("# md"), 0o644); err != nil {
		t.Fatalf("write ignore.md: %v", err)
	}

	nb, err := v.CreateNotebook("WORK")
	if err != nil {
		t.Fatalf("CreateNotebook: %v", err)
	}
	if nb.Name != "WORK" || nb.NoteCount != 0 {
		t.Fatalf("unexpected notebook: %+v", nb)
	}

	// Duplicate case-insensitive rejected.
	if _, err := v.CreateNotebook("work"); !errors.Is(err, ErrAlreadyExists) {
		t.Fatalf("expected ErrAlreadyExists, got %v", err)
	}

	// Invalid names rejected (traversal / hidden / separators).
	for _, bad := range []string{"", "   ", ".hidden", "../escape", "a/b", `a\b`} {
		if _, err := v.CreateNotebook(bad); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("CreateNotebook(%q) err = %v, want ErrInvalidInput", bad, err)
		}
	}

	// Rename notebook.
	renamed, err := v.RenameNotebook("work", "Projects")
	if err != nil {
		t.Fatalf("RenameNotebook: %v", err)
	}
	if renamed.Name != "Projects" {
		t.Fatalf("renamed = %+v", renamed)
	}

	// Add a note inside notebook and verify non-empty delete requires force.
	if _, err := v.CreateNote("Projects", "spec", "body"); err != nil {
		t.Fatalf("CreateNote: %v", err)
	}
	if _, err := v.DeleteNotebook("Projects", false); !errors.Is(err, ErrNotEmpty) {
		t.Fatalf("expected ErrNotEmpty without force, got %v", err)
	}

	trashRes, err := v.DeleteNotebook("Projects", true)
	if err != nil {
		t.Fatalf("DeleteNotebook force: %v", err)
	}
	if !strings.Contains(trashRes.TrashPath, TrashDirName) {
		t.Fatalf("expected trash path under %s, got %q", TrashDirName, trashRes.TrashPath)
	}
	if _, err := os.Stat(filepath.Join(trashRes.TrashPath, "spec.txt")); err != nil {
		t.Fatalf("expected trashed note inside trashed folder: %v", err)
	}

	// Trashed notebook must not show up in ListNotebooks.
	nbs, err := v.ListNotebooks()
	if err != nil {
		t.Fatalf("ListNotebooks: %v", err)
	}
	if len(nbs) != 0 {
		t.Fatalf("expected 0 notebooks after delete, got %+v", nbs)
	}
}

func TestVaultNotesCRUDMoveResolveAndTrash(t *testing.T) {
	v := newTestVault(t)
	if _, err := v.CreateNotebook("WORK"); err != nil {
		t.Fatalf("CreateNotebook WORK: %v", err)
	}
	if _, err := v.CreateNotebook("HOME"); err != nil {
		t.Fatalf("CreateNotebook HOME: %v", err)
	}

	// Create root note and notebook notes (stripping redundant .txt suffix).
	nRoot, err := v.CreateNote("", "solta.txt", "conteudo raiz")
	if err != nil {
		t.Fatalf("CreateNote root: %v", err)
	}
	if nRoot.ID != "solta.txt" || nRoot.Notebook != "" || nRoot.Title != "solta" {
		t.Fatalf("unexpected root note: %+v", nRoot)
	}

	nWork, err := v.CreateNote("WORK", "reuniao", "pauta sprint")
	if err != nil {
		t.Fatalf("CreateNote WORK: %v", err)
	}
	if nWork.ID != "WORK/reuniao.txt" || nWork.Notebook != "WORK" {
		t.Fatalf("unexpected work note: %+v", nWork)
	}

	// Ambiguity check when same title exists in two notebooks.
	if _, err := v.CreateNote("HOME", "reuniao", "pauta familia"); err != nil {
		t.Fatalf("CreateNote HOME: %v", err)
	}
	if _, err := v.ResolveNote("reuniao", ""); !errors.Is(err, ErrAmbiguous) {
		t.Fatalf("expected ErrAmbiguous, got %v", err)
	}
	// Resolved unambiguously via defaultNotebook or explicit path.
	got, err := v.ResolveNote("reuniao", "WORK")
	if err != nil || got.ID != "WORK/reuniao.txt" {
		t.Fatalf("ResolveNote with defaultNotebook = (%+v, %v)", got, err)
	}
	got2, err := v.ResolveNote("HOME/reuniao", "")
	if err != nil || got2.Content != "pauta familia" {
		t.Fatalf("ResolveNote explicit = (%+v, %v)", got2, err)
	}

	// Search by content.
	found, err := v.ListNotes(NoteFilter{Query: "sprint"})
	if err != nil {
		t.Fatalf("ListNotes query: %v", err)
	}
	if len(found) != 1 || found[0].ID != "WORK/reuniao.txt" {
		t.Fatalf("search result = %+v", found)
	}

	// Update content + rename + move to root.
	newTitle := "ata-sprint"
	newBody := "decisoes finais"
	moved, err := v.UpdateNote(nWork.ID, UpdateNoteInput{
		Title:       &newTitle,
		Content:     &newBody,
		SetNotebook: true,
		Notebook:    "",
	})
	if err != nil {
		t.Fatalf("UpdateNote: %v", err)
	}
	if moved.ID != "ata-sprint.txt" || moved.Notebook != "" || moved.Content != newBody {
		t.Fatalf("unexpected moved note: %+v", moved)
	}
	if _, err := os.Stat(nWork.Path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("old note path should not exist after move")
	}

	// Safe delete moves file to .trash/ preserving content.
	res, err := v.DeleteNote(moved.ID)
	if err != nil {
		t.Fatalf("DeleteNote: %v", err)
	}
	raw, err := os.ReadFile(res.TrashPath)
	if err != nil || string(raw) != newBody {
		t.Fatalf("trashed note content = (%q, %v), want %q", string(raw), err, newBody)
	}

	// Trashed note excluded from ListNotes.
	remaining, err := v.ListNotes(NoteFilter{})
	if err != nil {
		t.Fatalf("ListNotes remaining: %v", err)
	}
	if len(remaining) != 2 {
		t.Fatalf("expected 2 remaining notes, got %d", len(remaining))
	}
}

func TestListNotesAlwaysAlphabeticalWithAndWithoutFilters(t *testing.T) {
	v := newTestVault(t)
	if _, err := v.CreateNotebook("WORK"); err != nil {
		t.Fatalf("CreateNotebook WORK: %v", err)
	}
	if _, err := v.CreateNotebook("HOME"); err != nil {
		t.Fatalf("CreateNotebook HOME: %v", err)
	}

	// Create notes in reverse alphabetical order and touch the last one to be newest.
	for _, spec := range []struct {
		nb, title, body string
	}{
		{"WORK", "Zebra", "item comum"},
		{"", "beta", "item comum"},
		{"HOME", "Alpha", "item comum"},
		{"WORK", "alpha-2", "item comum"},
		{"WORK", "Delta", "outro"},
	} {
		if _, err := v.CreateNote(spec.nb, spec.title, spec.body); err != nil {
			t.Fatalf("CreateNote %s/%s: %v", spec.nb, spec.title, err)
		}
	}

	// Without any filter: strictly alphabetical by Title (case-insensitive).
	all, err := v.ListNotes(NoteFilter{})
	if err != nil {
		t.Fatalf("ListNotes all: %v", err)
	}
	wantAll := []string{"HOME/Alpha.txt", "WORK/alpha-2.txt", "beta.txt", "WORK/Delta.txt", "WORK/Zebra.txt"}
	if len(all) != len(wantAll) {
		t.Fatalf("len(all) = %d, want %d", len(all), len(wantAll))
	}
	for i, want := range wantAll {
		if all[i].ID != want {
			t.Fatalf("all[%d] = %q, want %q", i, all[i].ID, want)
		}
	}

	// With notebook filter: alphabetical.
	work, err := v.ListNotes(NoteFilter{Notebook: "WORK"})
	if err != nil {
		t.Fatalf("ListNotes WORK: %v", err)
	}
	wantWork := []string{"WORK/alpha-2.txt", "WORK/Delta.txt", "WORK/Zebra.txt"}
	if len(work) != len(wantWork) {
		t.Fatalf("len(work) = %d, want %d", len(work), len(wantWork))
	}
	for i, want := range wantWork {
		if work[i].ID != want {
			t.Fatalf("work[%d] = %q, want %q", i, work[i].ID, want)
		}
	}

	// With search query filter: alphabetical.
	searched, err := v.ListNotes(NoteFilter{Query: "comum"})
	if err != nil {
		t.Fatalf("ListNotes query: %v", err)
	}
	wantSearched := []string{"HOME/Alpha.txt", "WORK/alpha-2.txt", "beta.txt", "WORK/Zebra.txt"}
	if len(searched) != len(wantSearched) {
		t.Fatalf("len(searched) = %d, want %d", len(searched), len(wantSearched))
	}
	for i, want := range wantSearched {
		if searched[i].ID != want {
			t.Fatalf("searched[%d] = %q, want %q", i, searched[i].ID, want)
		}
	}
}
