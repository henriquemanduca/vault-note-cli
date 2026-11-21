package filestore

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

const appDirName = "vault-note-cli"

// BaseDir resolves the platform-standard user data directory for vault-note-cli:
//   - Linux/BSD: $XDG_DATA_HOME/vault-note-cli (fallback: ~/.local/share/vault-note-cli)
//   - macOS:     ~/Library/Application Support/vault-note-cli
//   - Windows:   %LOCALAPPDATA%\vault-note-cli
func BaseDir() (string, error) {
	return baseDirForOS(runtime.GOOS)
}

func baseDirForOS(goos string) (string, error) {
	switch goos {
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home dir: %w", err)
		}
		return filepath.Join(home, "Library", "Application Support", appDirName), nil
	case "windows":
		if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
			return filepath.Join(localAppData, appDirName), nil
		}
		cfg, err := os.UserConfigDir()
		if err != nil {
			return "", fmt.Errorf("resolve windows appdata dir: %w", err)
		}
		return filepath.Join(cfg, appDirName), nil
	default:
		if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
			return filepath.Join(xdg, appDirName), nil
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home dir: %w", err)
		}
		return filepath.Join(home, ".local", "share", appDirName), nil
	}
}

// DefaultDBPath returns <BaseDir>/vault-note-cli.db.
func DefaultDBPath() (string, error) {
	base, err := BaseDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "vault-note-cli.db"), nil
}

// DefaultVaultSuggestion returns a suggested path displayed as placeholder in
// the Settings modal when no vault path has been configured yet.
func DefaultVaultSuggestion() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "~/vault-notes"
	}
	return filepath.Join(home, "vault-notes")
}
