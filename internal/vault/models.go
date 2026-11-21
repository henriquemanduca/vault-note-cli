package vault

import "errors"

// Domain sentinel errors for filesystem vault operations.
var (
	ErrNotFound      = errors.New("record not found")
	ErrAlreadyExists = errors.New("already exists")
	ErrInvalidInput  = errors.New("invalid input")
	ErrNotEmpty      = errors.New("notebook is not empty")
	ErrAmbiguous     = errors.New("ambiguous note reference")
	ErrConflict      = errors.New("external modification conflict")
)

const (
	// TrashDirName is the hidden directory inside the vault root used for safe deletes.
	TrashDirName = ".trash"
	// NoteExt is the required file extension for plain-text notes in the vault.
	NoteExt = ".txt"
)

// Notebook represents a 1st-level directory inside the vault root.
type Notebook struct {
	Name      string
	Path      string
	NoteCount int
	UpdatedAt string
}

// Note represents a plain-text .txt file stored either in the vault root
// (Notebook == "") or directly inside a 1st-level notebook directory.
type Note struct {
	ID        string // Canonical relative path with forward slashes (e.g. "WORK/reuniao.txt" or "solta.txt")
	Notebook  string // 1st-level directory name, or "" for root notes
	Title     string // Filename without .txt extension
	FileName  string // Basename including .txt extension
	Path      string // Absolute path on disk
	Content   string // Plain UTF-8 file contents
	UpdatedAt string // RFC3339 UTC timestamp from file ModTime
}

// NoteFilter controls note listing and search across the vault.
type NoteFilter struct {
	Notebook string // Filter by 1st-level folder name (case-insensitive fallback)
	Query    string // Case-insensitive substring match against title, notebook, or content
}

// UpdateNoteInput configures partial updates (content, rename, and/or move) to a note.
type UpdateNoteInput struct {
	Title       *string
	Content     *string
	SetNotebook bool
	Notebook    string // "" moves the note to the vault root
}

// TrashResult reports where a deleted note or notebook was moved inside .trash/.
type TrashResult struct {
	OriginalID string
	TrashPath  string
}

// TrashItem represents a trashed file or empty directory inside <vault>/.trash/.
type TrashItem struct {
	ID        string // Canonical relative path inside .trash/ with forward slashes
	Bucket    string // Top-level entry under .trash/ (usually UTC timestamp bucket)
	RelPath   string // Relative path inside the bucket (or entry name if at .trash/ root)
	IsDir     bool   // True for empty trashed directories (e.g., an empty notebook)
	SizeBytes int64  // File size in bytes (0 for empty directories)
	TrashedAt string // RFC3339 UTC timestamp from bucket name or ModTime fallback
	Path      string // Absolute path on disk
}

// TrashEmptyResult summarizes a permanent cleanup of <vault>/.trash/.
type TrashEmptyResult struct {
	RemovedItems int
	RemovedBytes int64
	TrashDir     string
}
