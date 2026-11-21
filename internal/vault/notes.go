package vault

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ListNotes returns plain-text .txt notes matching filter, always sorted
// alphabetically by Title (case-insensitive), tie-broken by ID.
func (v *Vault) ListNotes(filter NoteFilter) ([]Note, error) {
	var collected []Note

	nbFilter := strings.TrimSpace(filter.Notebook)
	if nbFilter != "" {
		nb, err := v.GetNotebook(nbFilter)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return nil, nil
			}
			return nil, err
		}
		items, err := v.readNotesInDir(nb.Name, nb.Path)
		if err != nil {
			return nil, err
		}
		collected = append(collected, items...)
	} else {
		// Root standalone notes.
		rootItems, err := v.readNotesInDir("", v.root)
		if err != nil {
			return nil, err
		}
		collected = append(collected, rootItems...)

		// 1st-level notebook folders.
		nbs, err := v.ListNotebooks()
		if err != nil {
			return nil, err
		}
		for _, nb := range nbs {
			items, err := v.readNotesInDir(nb.Name, nb.Path)
			if err != nil {
				return nil, err
			}
			collected = append(collected, items...)
		}
	}

	q := strings.ToLower(strings.TrimSpace(filter.Query))
	var filtered []Note
	for _, note := range collected {
		if q != "" {
			if !strings.Contains(strings.ToLower(note.Title), q) &&
				!strings.Contains(strings.ToLower(note.Notebook), q) &&
				!strings.Contains(strings.ToLower(note.Content), q) {
				continue
			}
		}
		filtered = append(filtered, note)
	}

	sort.Slice(filtered, func(i, j int) bool {
		ti := strings.ToLower(filtered[i].Title)
		tj := strings.ToLower(filtered[j].Title)
		if ti != tj {
			return ti < tj
		}
		if filtered[i].Title != filtered[j].Title {
			return filtered[i].Title < filtered[j].Title
		}
		li := strings.ToLower(filtered[i].ID)
		lj := strings.ToLower(filtered[j].ID)
		if li != lj {
			return li < lj
		}
		return filtered[i].ID < filtered[j].ID
	})

	return filtered, nil
}

func (v *Vault) readNotesInDir(notebook, dir string) ([]Note, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read notes directory %s: %w", dir, err)
	}
	var out []Note
	for _, entry := range entries {
		if entry.IsDir() || !isTxtFile(entry.Name()) {
			continue
		}
		item, err := v.loadNoteFile(notebook, entry.Name())
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

func (v *Vault) loadNoteFile(notebook, fileName string) (Note, error) {
	var absPath, id string
	if notebook == "" {
		absPath = filepath.Join(v.root, fileName)
		id = fileName
	} else {
		absPath = filepath.Join(v.root, notebook, fileName)
		id = notebook + "/" + fileName
	}
	if err := v.ensureInsideRoot(absPath); err != nil {
		return Note{}, err
	}
	info, err := os.Stat(absPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Note{}, fmt.Errorf("%w: note %q", ErrNotFound, id)
		}
		return Note{}, fmt.Errorf("stat note %q: %w", id, err)
	}
	if info.IsDir() {
		return Note{}, fmt.Errorf("%w: %q is a directory", ErrInvalidInput, id)
	}
	raw, err := os.ReadFile(absPath)
	if err != nil {
		return Note{}, fmt.Errorf("read note %q: %w", id, err)
	}
	return Note{
		ID:        id,
		Notebook:  notebook,
		Title:     stripNoteExt(fileName),
		FileName:  fileName,
		Path:      absPath,
		Content:   string(raw),
		UpdatedAt: formatModTime(info.ModTime()),
	}, nil
}

// GetNote loads a note by its exact relative ID (e.g. "WORK/reuniao.txt" or "solta.txt").
func (v *Vault) GetNote(id string) (Note, error) {
	return v.ResolveNote(id, "")
}

// ResolveNote resolves a note reference from CLI/TUI:
//   - "WORK/reuniao.txt" or "WORK/reuniao"
//   - "reuniao.txt" or "reuniao" (checks defaultNotebook first when set, then searches vault unambiguously)
func (v *Vault) ResolveNote(rawRef, defaultNotebook string) (Note, error) {
	ref := strings.TrimSpace(strings.ReplaceAll(rawRef, `\`, "/"))
	ref = strings.TrimPrefix(ref, "./")
	ref = strings.TrimPrefix(ref, "/")
	if ref == "" {
		return Note{}, fmt.Errorf("%w: note reference is required", ErrInvalidInput)
	}

	if strings.Contains(ref, "/") {
		parts := strings.Split(ref, "/")
		if len(parts) != 2 {
			return Note{}, fmt.Errorf("%w: nested paths deeper than <notebook>/<note>.txt are not supported (%q)", ErrInvalidInput, rawRef)
		}
		nb, err := v.GetNotebook(parts[0])
		if err != nil {
			return Note{}, err
		}
		title, err := normalizeNoteTitle(parts[1])
		if err != nil {
			return Note{}, err
		}
		return v.findNoteInDir(nb.Name, nb.Path, title)
	}

	title, err := normalizeNoteTitle(ref)
	if err != nil {
		return Note{}, err
	}

	// If a default notebook is active, prefer matching inside it.
	if nbRef := strings.TrimSpace(defaultNotebook); nbRef != "" {
		nb, err := v.GetNotebook(nbRef)
		if err == nil {
			if note, findErr := v.findNoteInDir(nb.Name, nb.Path, title); findErr == nil {
				return note, nil
			}
		}
	}

	// Search root + all 1st-level notebooks.
	allNotes, err := v.ListNotes(NoteFilter{})
	if err != nil {
		return Note{}, err
	}

	var exactMatches []Note
	var foldMatches []Note
	for _, n := range allNotes {
		if n.Title == title || n.FileName == ref {
			exactMatches = append(exactMatches, n)
		} else if strings.EqualFold(n.Title, title) {
			foldMatches = append(foldMatches, n)
		}
	}

	matches := exactMatches
	if len(matches) == 0 {
		matches = foldMatches
	}
	if len(matches) == 0 {
		return Note{}, fmt.Errorf("%w: note %q", ErrNotFound, rawRef)
	}
	if len(matches) > 1 {
		ids := make([]string, len(matches))
		for i, m := range matches {
			ids[i] = m.ID
		}
		return Note{}, fmt.Errorf("%w: %q matches %s (specify <notebook>/<note>)", ErrAmbiguous, rawRef, strings.Join(ids, ", "))
	}
	return matches[0], nil
}

func (v *Vault) findNoteInDir(notebook, dir, title string) (Note, error) {
	items, err := v.readNotesInDir(notebook, dir)
	if err != nil {
		return Note{}, err
	}
	for _, it := range items {
		if it.Title == title {
			return it, nil
		}
	}
	for _, it := range items {
		if strings.EqualFold(it.Title, title) {
			return it, nil
		}
	}
	target := title + NoteExt
	if notebook != "" {
		target = notebook + "/" + target
	}
	return Note{}, fmt.Errorf("%w: note %q", ErrNotFound, target)
}

// CreateNote creates a new .txt file in the specified notebook (or vault root if notebook == "").
func (v *Vault) CreateNote(notebook, rawTitle, content string) (Note, error) {
	title, err := normalizeNoteTitle(rawTitle)
	if err != nil {
		return Note{}, err
	}

	nbName := ""
	targetDir := v.root
	if trimmedNB := strings.TrimSpace(notebook); trimmedNB != "" {
		nb, err := v.GetNotebook(trimmedNB)
		if err != nil {
			return Note{}, err
		}
		nbName = nb.Name
		targetDir = nb.Path
	}

	if existing, err := v.findNoteInDir(nbName, targetDir, title); err == nil {
		return Note{}, fmt.Errorf("%w: note %q", ErrAlreadyExists, existing.ID)
	} else if !errors.Is(err, ErrNotFound) {
		return Note{}, err
	}

	fileName := title + NoteExt
	absPath := filepath.Join(targetDir, fileName)
	if err := v.ensureInsideRoot(absPath); err != nil {
		return Note{}, err
	}
	if err := writeFileAtomic(absPath, content); err != nil {
		return Note{}, fmt.Errorf("create note %q: %w", fileName, err)
	}
	loaded, err := v.loadNoteFile(nbName, fileName)
	if err != nil {
		return Note{}, err
	}
	return loaded, nil
}

// ContentHash returns a deterministic SHA-256 hex digest of a note's body.
// Used for optimistic concurrency checks against external disk edits.
func ContentHash(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

// UpdateNoteContent atomically overwrites the body of an existing note .txt file.
func (v *Vault) UpdateNoteContent(id, content string) (Note, error) {
	return v.UpdateNote(id, UpdateNoteInput{Content: &content})
}

// UpdateNoteContentChecked atomically overwrites the body of an existing note
// only if the on-disk content still matches expectedHash (or already equals
// content). When expectedHash is non-empty and the file on disk diverged, it
// returns ErrConflict without modifying the file.
func (v *Vault) UpdateNoteContentChecked(id, content, expectedHash string) (Note, error) {
	current, err := v.GetNote(id)
	if err != nil {
		return Note{}, err
	}
	if expectedHash != "" && ContentHash(current.Content) != expectedHash && current.Content != content {
		return Note{}, fmt.Errorf("%w: note %q was modified externally", ErrConflict, current.ID)
	}
	if err := writeFileAtomic(current.Path, content); err != nil {
		return Note{}, fmt.Errorf("write note %q: %w", current.FileName, err)
	}
	return v.loadNoteFile(current.Notebook, current.FileName)
}

// CreateConflictCopy saves content as a sibling note next to id so conflicting
// edits are preserved without overwriting external changes.
func (v *Vault) CreateConflictCopy(id, content string) (Note, error) {
	orig, err := v.GetNote(id)
	if err != nil {
		return Note{}, err
	}
	stamp := v.clock().Format("20060102-150405")
	baseTitle := fmt.Sprintf("%s (conflito %s)", orig.Title, stamp)
	for attempt := 0; ; attempt++ {
		title := baseTitle
		if attempt > 0 {
			title = fmt.Sprintf("%s-%d", baseTitle, attempt)
		}
		created, createErr := v.CreateNote(orig.Notebook, title, content)
		if createErr == nil {
			return created, nil
		}
		if !errors.Is(createErr, ErrAlreadyExists) {
			return Note{}, createErr
		}
	}
}

// UpdateNote updates content, renames the .txt file, and/or moves it across notebooks.
func (v *Vault) UpdateNote(id string, in UpdateNoteInput) (Note, error) {
	note, err := v.GetNote(id)
	if err != nil {
		return Note{}, err
	}

	newTitle := note.Title
	if in.Title != nil {
		t, err := normalizeNoteTitle(*in.Title)
		if err != nil {
			return Note{}, err
		}
		newTitle = t
	}

	newNotebook := note.Notebook
	targetDir := v.root
	if newNotebook != "" {
		targetDir = filepath.Join(v.root, newNotebook)
	}
	if in.SetNotebook {
		trimmed := strings.TrimSpace(in.Notebook)
		if trimmed == "" || strings.EqualFold(trimmed, "none") {
			newNotebook = ""
			targetDir = v.root
		} else {
			nb, err := v.GetNotebook(trimmed)
			if err != nil {
				return Note{}, err
			}
			newNotebook = nb.Name
			targetDir = nb.Path
		}
	}

	newFileName := newTitle + NoteExt
	destPath := filepath.Join(targetDir, newFileName)
	if err := v.ensureInsideRoot(destPath); err != nil {
		return Note{}, err
	}

	sameLocation := note.Notebook == newNotebook && note.FileName == newFileName
	if !sameLocation {
		// Prevent overwriting a different existing note (case-insensitive collision check).
		if existing, err := v.findNoteInDir(newNotebook, targetDir, newTitle); err == nil && existing.Path != note.Path {
			return Note{}, fmt.Errorf("%w: note %q", ErrAlreadyExists, existing.ID)
		} else if err != nil && !errors.Is(err, ErrNotFound) {
			return Note{}, err
		}
		if err := os.Rename(note.Path, destPath); err != nil {
			return Note{}, fmt.Errorf("move/rename note %q: %w", note.ID, err)
		}
	}

	if in.Content != nil {
		if err := writeFileAtomic(destPath, *in.Content); err != nil {
			return Note{}, fmt.Errorf("write note %q: %w", newFileName, err)
		}
	}

	loaded, err := v.loadNoteFile(newNotebook, newFileName)
	if err != nil {
		return Note{}, err
	}
	return loaded, nil
}

// DeleteNote safely moves a note .txt file into <vault>/.trash/<timestamp>/<relpath>.
func (v *Vault) DeleteNote(id string) (TrashResult, error) {
	note, err := v.GetNote(id)
	if err != nil {
		return TrashResult{}, err
	}
	trashPath, err := v.moveToTrash(note.ID, note.Path)
	if err != nil {
		return TrashResult{}, err
	}
	return TrashResult{
		OriginalID: note.ID,
		TrashPath:  trashPath,
	}, nil
}
