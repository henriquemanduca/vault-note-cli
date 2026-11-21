package db

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/hmanduca/vault-note-cli/internal/filestore"
)

// Domain sentinel errors.
var (
	ErrNotFound     = errors.New("setting not found")
	ErrInvalidInput = errors.New("invalid input")
)

const schemaSQL = `
CREATE TABLE IF NOT EXISTS settings (
    key        TEXT PRIMARY KEY,
    value      TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
`

// Store wraps the embedded SQLite connection dedicated to persistent app settings.
type Store struct {
	db   *sql.DB
	path string
}

// DefaultDBPath returns the active database path, honoring VAULT_NOTE_CLI_DB
// override before falling back to the OS-standard data directory.
func DefaultDBPath() (string, error) {
	if envPath := strings.TrimSpace(os.Getenv("VAULT_NOTE_CLI_DB")); envPath != "" {
		return envPath, nil
	}
	return filestore.DefaultDBPath()
}

// NewStore opens (and initializes if needed) the SQLite settings database at path.
func NewStore(path string) (*Store, error) {
	if path == "" {
		var err error
		path, err = DefaultDBPath()
		if err != nil {
			return nil, fmt.Errorf("resolve default db path: %w", err)
		}
	}

	if path != ":memory:" && !strings.HasPrefix(path, "file:") {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, fmt.Errorf("create db directory: %w", err)
		}
	}

	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite db: %w", err)
	}
	// Serialize access within a single process to prevent SQLITE_BUSY under -race.
	conn.SetMaxOpenConns(1)

	pragmas := []string{
		"PRAGMA journal_mode = WAL;",
		"PRAGMA foreign_keys = ON;",
		"PRAGMA busy_timeout = 5000;",
	}
	for _, p := range pragmas {
		if _, err := conn.Exec(p); err != nil {
			_ = conn.Close()
			return nil, fmt.Errorf("apply %s: %w", p, err)
		}
	}

	if _, err := conn.Exec(schemaSQL); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}

	return &Store{db: conn, path: path}, nil
}

// Path returns the configured database path.
func (s *Store) Path() string {
	return s.path
}

// Close closes the underlying SQLite connection.
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func nowUTC() string {
	return time.Now().UTC().Format(time.RFC3339)
}
