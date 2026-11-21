package filestore

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

const appDirName = "vault-note-cli"

// ConfigDir resolves the platform-standard user configuration directory for vault-note-cli:
//   - Linux/BSD: $XDG_CONFIG_HOME/vault-note-cli (fallback: ~/.config/vault-note-cli)
//   - macOS:     ~/Library/Application Support/vault-note-cli
//   - Windows:   %APPDATA%\vault-note-cli (fallback: os.UserConfigDir)
func ConfigDir() (string, error) {
	return configDirForOS(runtime.GOOS)
}

func configDirForOS(goos string) (string, error) {
	switch goos {
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home dir: %w", err)
		}
		return filepath.Join(home, "Library", "Application Support", appDirName), nil
	case "windows":
		if appData := os.Getenv("APPDATA"); appData != "" {
			return filepath.Join(appData, appDirName), nil
		}
		cfg, err := os.UserConfigDir()
		if err != nil {
			return "", fmt.Errorf("resolve windows appdata dir: %w", err)
		}
		return filepath.Join(cfg, appDirName), nil
	default:
		if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
			return filepath.Join(xdg, appDirName), nil
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home dir: %w", err)
		}
		return filepath.Join(home, ".config", appDirName), nil
	}
}

// DefaultConfigPath returns <ConfigDir>/config.yaml.
func DefaultConfigPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.yaml"), nil
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
