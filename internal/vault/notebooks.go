package vault

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
)

// normalizeNotebookName trims whitespace, validates filename safety, rejects
// multi-word names (any internal whitespace), and converts to UPPERCASE.
func normalizeNotebookName(raw string) (string, error) {
	name, err := validateName(raw, "notebook name")
	if err != nil {
		return "", err
	}
	if strings.IndexFunc(name, unicode.IsSpace) >= 0 {
		return "", fmt.Errorf("%w: notebook name must be a single word (spaces are not allowed)", ErrInvalidInput)
	}
	return strings.ToUpper(name), nil
}

// ListNotebooks returns all visible 1st-level directories inside the vault root,
// sorted alphabetically (case-insensitive), along with their .txt note counts.
func (v *Vault) ListNotebooks() ([]Notebook, error) {
	entries, err := os.ReadDir(v.root)
	if err != nil {
		return nil, fmt.Errorf("read vault root: %w", err)
	}

	var out []Notebook
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") || !entry.IsDir() {
			continue
		}
		nbPath := filepath.Join(v.root, name)
		info, err := entry.Info()
		if err != nil {
			continue
		}
		count, err := countTxtFiles(nbPath)
		if err != nil {
			return nil, err
		}
		out = append(out, Notebook{
			Name:      name,
			Path:      nbPath,
			NoteCount: count,
			UpdatedAt: formatModTime(info.ModTime()),
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

// GetNotebook looks up a 1st-level notebook folder by exact name or case-insensitive fallback.
func (v *Vault) GetNotebook(ref string) (Notebook, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return Notebook{}, fmt.Errorf("%w: notebook name is required", ErrInvalidInput)
	}
	nbs, err := v.ListNotebooks()
	if err != nil {
		return Notebook{}, err
	}
	for _, nb := range nbs {
		if nb.Name == ref {
			return nb, nil
		}
	}
	for _, nb := range nbs {
		if strings.EqualFold(nb.Name, ref) {
			return nb, nil
		}
	}
	return Notebook{}, fmt.Errorf("%w: notebook %q", ErrNotFound, ref)
}

// CreateNotebook creates a new 1st-level folder inside the vault root.
// Names are trimmed, restricted to a single word, and normalized to UPPERCASE.
func (v *Vault) CreateNotebook(rawName string) (Notebook, error) {
	name, err := normalizeNotebookName(rawName)
	if err != nil {
		return Notebook{}, err
	}
	if _, err := v.GetNotebook(name); err == nil {
		return Notebook{}, fmt.Errorf("%w: notebook %q", ErrAlreadyExists, name)
	} else if !errors.Is(err, ErrNotFound) {
		return Notebook{}, err
	}

	target := filepath.Join(v.root, name)
	if err := v.ensureInsideRoot(target); err != nil {
		return Notebook{}, err
	}
	if err := os.Mkdir(target, 0o755); err != nil {
		if errors.Is(err, os.ErrExist) {
			return Notebook{}, fmt.Errorf("%w: notebook %q", ErrAlreadyExists, name)
		}
		return Notebook{}, fmt.Errorf("create notebook %q: %w", name, err)
	}
	return v.GetNotebook(name)
}

// RenameNotebook renames an existing 1st-level notebook folder.
// New names are trimmed, restricted to a single word, and normalized to UPPERCASE.
func (v *Vault) RenameNotebook(oldRef, rawNewName string) (Notebook, error) {
	nb, err := v.GetNotebook(oldRef)
	if err != nil {
		return Notebook{}, err
	}
	newName, err := normalizeNotebookName(rawNewName)
	if err != nil {
		return Notebook{}, err
	}
	if nb.Name == newName {
		return nb, nil
	}

	// Refuse collision with another existing notebook (case-insensitive check unless same folder casing change).
	if existing, err := v.GetNotebook(newName); err == nil && existing.Name != nb.Name {
		return Notebook{}, fmt.Errorf("%w: notebook %q", ErrAlreadyExists, newName)
	} else if err != nil && !errors.Is(err, ErrNotFound) {
		return Notebook{}, err
	}

	newPath := filepath.Join(v.root, newName)
	if err := v.ensureInsideRoot(newPath); err != nil {
		return Notebook{}, err
	}
	if err := os.Rename(nb.Path, newPath); err != nil {
		return Notebook{}, fmt.Errorf("rename notebook %q to %q: %w", nb.Name, newName, err)
	}
	return v.GetNotebook(newName)
}

// DeleteNotebook moves a notebook folder into <vault>/.trash/.
// If the folder is non-empty and force is false, it returns ErrNotEmpty.
func (v *Vault) DeleteNotebook(ref string, force bool) (TrashResult, error) {
	nb, err := v.GetNotebook(ref)
	if err != nil {
		return TrashResult{}, err
	}
	entries, err := os.ReadDir(nb.Path)
	if err != nil {
		return TrashResult{}, fmt.Errorf("inspect notebook %q: %w", nb.Name, err)
	}
	if len(entries) > 0 && !force {
		return TrashResult{}, fmt.Errorf("%w: notebook %q contains %d item(s) (pass --force to move to .trash)", ErrNotEmpty, nb.Name, len(entries))
	}
	trashPath, err := v.moveToTrash(nb.Name, nb.Path)
	if err != nil {
		return TrashResult{}, err
	}
	return TrashResult{
		OriginalID: nb.Name,
		TrashPath:  trashPath,
	}, nil
}

func countTxtFiles(dir string) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, fmt.Errorf("read notebook dir %s: %w", dir, err)
	}
	count := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if isTxtFile(entry.Name()) {
			count++
		}
	}
	return count, nil
}
