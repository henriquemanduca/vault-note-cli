package vault

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// TrashDir returns the absolute path to <vault>/.trash.
func (v *Vault) TrashDir() string {
	if v == nil {
		return ""
	}
	return filepath.Join(v.root, TrashDirName)
}

// ListTrash returns all trashed files and empty leaf directories inside <vault>/.trash/,
// sorted deterministically by canonical ID. If <vault>/.trash/ does not exist, it returns
// an empty slice and nil error.
func (v *Vault) ListTrash() ([]TrashItem, error) {
	trashRoot := v.TrashDir()
	if err := v.ensureInsideRoot(trashRoot); err != nil {
		return nil, err
	}

	info, err := os.Lstat(trashRoot)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("stat trash directory: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%w: %s is not a directory", ErrInvalidInput, TrashDirName)
	}

	var items []TrashItem
	err = filepath.WalkDir(trashRoot, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == trashRoot {
			return nil
		}

		rel, err := filepath.Rel(trashRoot, path)
		if err != nil {
			return err
		}
		relSlash := filepath.ToSlash(rel)

		if d.IsDir() {
			// Skip non-empty directories (their files/leaves will be visited).
			// Record empty leaf directories (e.g. an empty trashed notebook) unless it is a bare timestamp bucket.
			entries, err := os.ReadDir(path)
			if err != nil {
				return err
			}
			if len(entries) > 0 {
				return nil
			}
			// Bare top-level bucket directory with nothing inside is not a trashed item.
			if !strings.Contains(relSlash, "/") {
				return nil
			}
			fi, err := d.Info()
			if err != nil {
				return err
			}
			bucket, origRel := splitTrashRel(relSlash)
			items = append(items, TrashItem{
				ID:        relSlash,
				Bucket:    bucket,
				RelPath:   origRel,
				IsDir:     true,
				SizeBytes: 0,
				TrashedAt: parseTrashTimestamp(bucket, fi.ModTime()),
				Path:      path,
			})
			return nil
		}

		fi, err := d.Info()
		if err != nil {
			return err
		}
		bucket, origRel := splitTrashRel(relSlash)
		items = append(items, TrashItem{
			ID:        relSlash,
			Bucket:    bucket,
			RelPath:   origRel,
			IsDir:     false,
			SizeBytes: fi.Size(),
			TrashedAt: parseTrashTimestamp(bucket, fi.ModTime()),
			Path:      path,
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk trash directory: %w", err)
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].ID < items[j].ID
	})
	return items, nil
}

// EmptyTrash permanently deletes all items inside <vault>/.trash/ and removes the
// empty .trash directory. Calling EmptyTrash when .trash/ does not exist or is
// already empty succeeds with RemovedItems == 0.
func (v *Vault) EmptyTrash() (TrashEmptyResult, error) {
	trashRoot := v.TrashDir()
	if err := v.ensureInsideRoot(trashRoot); err != nil {
		return TrashEmptyResult{}, err
	}

	res := TrashEmptyResult{TrashDir: trashRoot}
	info, err := os.Lstat(trashRoot)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return res, nil
		}
		return TrashEmptyResult{}, fmt.Errorf("stat trash directory: %w", err)
	}
	if !info.IsDir() {
		return TrashEmptyResult{}, fmt.Errorf("%w: %s is not a directory", ErrInvalidInput, TrashDirName)
	}

	items, err := v.ListTrash()
	if err != nil {
		return TrashEmptyResult{}, err
	}
	res.RemovedItems = len(items)
	for _, it := range items {
		res.RemovedBytes += it.SizeBytes
	}

	entries, err := os.ReadDir(trashRoot)
	if err != nil {
		return TrashEmptyResult{}, fmt.Errorf("read trash directory: %w", err)
	}
	for _, entry := range entries {
		target := filepath.Join(trashRoot, entry.Name())
		if err := v.ensureInsideRoot(target); err != nil {
			return TrashEmptyResult{}, err
		}
		if err := os.RemoveAll(target); err != nil {
			return TrashEmptyResult{}, fmt.Errorf("remove trash entry %s: %w", entry.Name(), err)
		}
	}
	if err := os.Remove(trashRoot); err != nil && !errors.Is(err, os.ErrNotExist) {
		return TrashEmptyResult{}, fmt.Errorf("remove trash directory: %w", err)
	}
	return res, nil
}

func splitTrashRel(relSlash string) (bucket, origRel string) {
	if idx := strings.IndexByte(relSlash, '/'); idx >= 0 {
		return relSlash[:idx], relSlash[idx+1:]
	}
	return "", relSlash
}

func parseTrashTimestamp(bucket string, fallback time.Time) string {
	if bucket != "" {
		base := bucket
		if idx := strings.IndexByte(base, '-'); idx >= 0 {
			base = base[:idx]
		}
		if t, err := time.Parse("20060102T150405Z", base); err == nil {
			return formatModTime(t)
		}
	}
	return formatModTime(fallback)
}
