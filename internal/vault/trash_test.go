package vault

import (
	"os"
	"path/filepath"
	"testing"
)

func TestVaultListAndEmptyTrash(t *testing.T) {
	v, err := Open(filepath.Join(t.TempDir(), "vault"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	// Missing .trash/ -> empty list and idempotent EmptyTrash.
	items, err := v.ListTrash()
	if err != nil || len(items) != 0 {
		t.Fatalf("ListTrash (missing) = (%v, %v), want empty", items, err)
	}
	res, err := v.EmptyTrash()
	if err != nil || res.RemovedItems != 0 || res.RemovedBytes != 0 {
		t.Fatalf("EmptyTrash (missing) = (%+v, %v), want 0", res, err)
	}

	// Create active items that must survive EmptyTrash.
	if _, err := v.CreateNotebook("WORK"); err != nil {
		t.Fatalf("CreateNotebook WORK: %v", err)
	}
	if _, err := v.CreateNote("WORK", "keep", "keep me"); err != nil {
		t.Fatalf("CreateNote keep: %v", err)
	}
	if _, err := v.CreateTemplate("keep-tpl", "template body", false); err != nil {
		t.Fatalf("CreateTemplate keep: %v", err)
	}

	// Create and delete a root note, a notebook note, an empty notebook, and a template.
	if _, err := v.CreateNote("", "loose", "hello"); err != nil {
		t.Fatalf("CreateNote loose: %v", err)
	}
	if _, err := v.DeleteNote("loose.txt"); err != nil {
		t.Fatalf("DeleteNote loose: %v", err)
	}

	if _, err := v.CreateNote("WORK", "old", "1234567890"); err != nil {
		t.Fatalf("CreateNote old: %v", err)
	}
	if _, err := v.DeleteNote("WORK/old.txt"); err != nil {
		t.Fatalf("DeleteNote old: %v", err)
	}

	if _, err := v.CreateNotebook("EMPTY"); err != nil {
		t.Fatalf("CreateNotebook EMPTY: %v", err)
	}
	if _, err := v.DeleteNotebook("EMPTY", false); err != nil {
		t.Fatalf("DeleteNotebook EMPTY: %v", err)
	}

	if _, err := v.CreateTemplate("temp", "abc", false); err != nil {
		t.Fatalf("CreateTemplate temp: %v", err)
	}
	if _, err := v.DeleteTemplate("temp"); err != nil {
		t.Fatalf("DeleteTemplate temp: %v", err)
	}

	items, err = v.ListTrash()
	if err != nil {
		t.Fatalf("ListTrash: %v", err)
	}
	if len(items) != 4 {
		t.Fatalf("expected 4 trashed items, got %d: %+v", len(items), items)
	}

	var totalBytes int64
	foundDir := false
	for _, it := range items {
		if it.TrashedAt == "" {
			t.Errorf("expected non-empty TrashedAt for %+v", it)
		}
		if it.IsDir && it.RelPath == "EMPTY" {
			foundDir = true
		}
		totalBytes += it.SizeBytes
	}
	if !foundDir {
		t.Errorf("expected empty trashed notebook EMPTY in ListTrash, got %+v", items)
	}
	expectedBytes := int64(len("hello") + len("1234567890") + len("abc"))
	if totalBytes != expectedBytes {
		t.Fatalf("totalBytes = %d, want %d", totalBytes, expectedBytes)
	}

	// EmptyTrash permanently deletes .trash/ and reports counts.
	res, err = v.EmptyTrash()
	if err != nil {
		t.Fatalf("EmptyTrash: %v", err)
	}
	if res.RemovedItems != 4 || res.RemovedBytes != expectedBytes {
		t.Fatalf("EmptyTrash = %+v, want items=4 bytes=%d", res, expectedBytes)
	}
	if _, err := os.Stat(v.TrashDir()); !os.IsNotExist(err) {
		t.Fatalf("expected .trash dir to be removed, stat err = %v", err)
	}

	// Active notes and templates are untouched.
	kept, err := v.GetNote("WORK/keep.txt")
	if err != nil || kept.Content != "keep me" {
		t.Fatalf("kept note damaged: (%+v, %v)", kept, err)
	}
	keptTpl, err := v.GetTemplate("keep-tpl")
	if err != nil || keptTpl.Body != "template body" {
		t.Fatalf("kept template damaged: (%+v, %v)", keptTpl, err)
	}

	// Second EmptyTrash is idempotent.
	res2, err := v.EmptyTrash()
	if err != nil || res2.RemovedItems != 0 {
		t.Fatalf("second EmptyTrash = (%+v, %v), want 0", res2, err)
	}
}
