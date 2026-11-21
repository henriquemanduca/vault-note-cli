package vault

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Vault manages notebooks (1st-level folders) and notes (.txt files) under a root directory.
type Vault struct {
	root string
}

// ExpandPath expands leading ~/ (or ~) and returns a clean absolute path.
func ExpandPath(raw string) (string, error) {
	p := strings.TrimSpace(raw)
	if p == "" {
		return "", fmt.Errorf("%w: vault path is required", ErrInvalidInput)
	}
	if p == "~" || strings.HasPrefix(p, "~/") || strings.HasPrefix(p, `~\`) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home dir: %w", err)
		}
		if p == "~" {
			p = home
		} else {
			p = filepath.Join(home, p[2:])
		}
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", fmt.Errorf("resolve absolute vault path: %w", err)
	}
	return filepath.Clean(abs), nil
}

// EnsureDir validates and creates the vault directory if it does not yet exist.
func EnsureDir(raw string) (string, error) {
	abs, err := ExpandPath(raw)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(abs)
	if err == nil {
		if !info.IsDir() {
			return "", fmt.Errorf("%w: %s is a file, expected a directory", ErrInvalidInput, abs)
		}
		return abs, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("stat vault path %s: %w", abs, err)
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return "", fmt.Errorf("create vault directory %s: %w", abs, err)
	}
	return abs, nil
}

// Open validates/creates root and returns a Vault bound to that directory.
func Open(root string) (*Vault, error) {
	abs, err := EnsureDir(root)
	if err != nil {
		return nil, err
	}
	return &Vault{root: abs}, nil
}

// Root returns the absolute path of the vault root directory.
func (v *Vault) Root() string {
	if v == nil {
		return ""
	}
	return v.root
}

func (v *Vault) ensureInsideRoot(target string) error {
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return fmt.Errorf("%w: resolve path: %v", ErrInvalidInput, err)
	}
	rel, err := filepath.Rel(v.root, absTarget)
	if err != nil {
		return fmt.Errorf("%w: path outside vault", ErrInvalidInput)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return fmt.Errorf("%w: path escapes vault root", ErrInvalidInput)
	}
	return nil
}

// validateName ensures a folder or note base name is portable and cannot traverse paths.
func validateName(raw, label string) (string, error) {
	name := strings.TrimSpace(raw)
	if name == "" {
		return "", fmt.Errorf("%w: %s cannot be empty", ErrInvalidInput, label)
	}
	if strings.HasPrefix(name, ".") {
		return "", fmt.Errorf("%w: %s cannot start with '.'", ErrInvalidInput, label)
	}
	if name == "." || name == ".." || strings.Contains(name, "..") {
		return "", fmt.Errorf("%w: %s cannot contain '..'", ErrInvalidInput, label)
	}
	if strings.ContainsAny(name, `/\`) {
		return "", fmt.Errorf("%w: %s cannot contain path separators", ErrInvalidInput, label)
	}
	if strings.ContainsAny(name, `<>:"|?*`) {
		return "", fmt.Errorf("%w: %s contains invalid filename characters", ErrInvalidInput, label)
	}
	for _, r := range name {
		if r < 0x20 || r == 0x7f {
			return "", fmt.Errorf("%w: %s contains control characters", ErrInvalidInput, label)
		}
	}
	return name, nil
}

func normalizeNoteTitle(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if len(s) > len(NoteExt) && strings.EqualFold(s[len(s)-len(NoteExt):], NoteExt) {
		s = s[:len(s)-len(NoteExt)]
	}
	return validateName(s, "note title")
}

func isTxtFile(name string) bool {
	if strings.HasPrefix(name, ".") {
		return false
	}
	ext := filepath.Ext(name)
	return len(name) > len(ext) && strings.EqualFold(ext, NoteExt)
}

func stripNoteExt(name string) string {
	ext := filepath.Ext(name)
	if strings.EqualFold(ext, NoteExt) {
		return name[:len(name)-len(ext)]
	}
	return name
}

func formatModTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

func writeFileAtomic(path, content string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("ensure parent dir: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".vault-tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := tmp.WriteString(content); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("commit file %s: %w", path, err)
	}
	cleanup = false
	return nil
}

// moveToTrash moves absPath into <vault>/.trash/<timestamp>/<relPath>, avoiding collisions.
func (v *Vault) moveToTrash(relPath, absPath string) (string, error) {
	if err := v.ensureInsideRoot(absPath); err != nil {
		return "", err
	}
	ts := time.Now().UTC().Format("20060102T150405Z")
	trashRoot := filepath.Join(v.root, TrashDirName)

	var dest string
	for attempt := 0; ; attempt++ {
		bucket := ts
		if attempt > 0 {
			bucket = fmt.Sprintf("%s-%d", ts, attempt)
		}
		candidate := filepath.Join(trashRoot, bucket, filepath.FromSlash(relPath))
		if _, err := os.Stat(candidate); errors.Is(err, os.ErrNotExist) {
			dest = candidate
			break
		}
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return "", fmt.Errorf("create trash directory: %w", err)
	}
	if err := os.Rename(absPath, dest); err != nil {
		return "", fmt.Errorf("move to trash: %w", err)
	}
	return dest, nil
}
