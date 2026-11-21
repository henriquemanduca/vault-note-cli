package settings

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/hmanduca/vault-note-cli/internal/filestore"
)

// Known persistent setting keys stored in config.yaml.
const (
	KeyVaultPath          = "vault_path"
	KeyShowNotebookPrefix = "show_notebook_prefix"
	KeyShowLineNumbers    = "show_line_numbers"
)

// Domain sentinel errors.
var (
	ErrNotFound     = errors.New("setting not found")
	ErrInvalidInput = errors.New("invalid input")
)

// AppSettings is the typed YAML schema persisted in config.yaml.
type AppSettings struct {
	VaultPath          string `yaml:"vault_path,omitempty"`
	ShowNotebookPrefix *bool  `yaml:"show_notebook_prefix,omitempty"`
	ShowLineNumbers    *bool  `yaml:"show_line_numbers,omitempty"`
}

// ShowNotebookPrefixEnabled reports the effective sidebar prefix setting.
// An unset value defaults to enabled.
func (c AppSettings) ShowNotebookPrefixEnabled() bool {
	if c.ShowNotebookPrefix == nil {
		return true
	}
	return *c.ShowNotebookPrefix
}

// ShowLineNumbersEnabled reports the effective editor line-number setting.
// An unset value defaults to disabled.
func (c AppSettings) ShowLineNumbersEnabled() bool {
	if c.ShowLineNumbers == nil {
		return false
	}
	return *c.ShowLineNumbers
}

// ParseBool parses the boolean forms accepted by config set.
func ParseBool(raw string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "true", "1", "on":
		return true, nil
	case "false", "0", "off":
		return false, nil
	default:
		return false, fmt.Errorf("%w: expected true/false/1/0/on/off", ErrInvalidInput)
	}
}

// Setting represents a single key-value entry exposed to listings.
type Setting struct {
	Key   string
	Value string
}

// Store persists application settings in a YAML file with atomic writes.
type Store struct {
	path string
}

// DefaultPath returns the active config YAML path, honoring VAULT_NOTE_CLI_CONFIG
// before falling back to the OS-standard configuration directory.
func DefaultPath() (string, error) {
	if envPath := strings.TrimSpace(os.Getenv("VAULT_NOTE_CLI_CONFIG")); envPath != "" {
		return NormalizePath(envPath)
	}
	return filestore.DefaultConfigPath()
}

// NewStore creates a YAML settings store bound to path (or DefaultPath if empty).
// The underlying file does not need to exist yet; missing files load as empty settings.
func NewStore(path string) (*Store, error) {
	if strings.TrimSpace(path) == "" {
		var err error
		path, err = DefaultPath()
		if err != nil {
			return nil, fmt.Errorf("resolve default config path: %w", err)
		}
	} else {
		var err error
		path, err = NormalizePath(path)
		if err != nil {
			return nil, err
		}
	}
	return &Store{path: path}, nil
}

// Path returns the resolved YAML file path.
func (s *Store) Path() string {
	if s == nil {
		return ""
	}
	return s.path
}

// Load reads and parses the YAML settings file.
// If the file does not exist yet, it returns an empty AppSettings and nil error.
func (s *Store) Load() (AppSettings, error) {
	raw, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return AppSettings{}, nil
		}
		return AppSettings{}, fmt.Errorf("read config %s: %w", s.path, err)
	}
	var cfg AppSettings
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return AppSettings{}, fmt.Errorf("parse YAML config %s: %w", s.path, err)
	}
	cfg.VaultPath = strings.TrimSpace(cfg.VaultPath)
	return cfg, nil
}

// Save atomically writes cfg to the YAML file, creating parent directories if needed.
func (s *Store) Save(cfg AppSettings) error {
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create config directory %s: %w", dir, err)
	}

	raw, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal YAML config: %w", err)
	}

	tmp, err := os.CreateTemp(dir, ".config-tmp-*")
	if err != nil {
		return fmt.Errorf("create temp config file: %w", err)
	}
	tmpPath := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := tmp.Write(raw); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp config file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync temp config file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp config file: %w", err)
	}
	if err := os.Rename(tmpPath, s.path); err != nil {
		return fmt.Errorf("commit config %s: %w", s.path, err)
	}
	cleanup = false
	return nil
}

// GetVaultPath returns the configured vault path, or "" if not yet configured.
func (s *Store) GetVaultPath() (string, error) {
	cfg, err := s.Load()
	if err != nil {
		return "", err
	}
	return cfg.VaultPath, nil
}

// SetVaultPath normalizes rawPath (expanding ~/ and resolving absolute path)
// and persists it to config.yaml.
func (s *Store) SetVaultPath(rawPath string) (string, error) {
	abs, err := NormalizePath(rawPath)
	if err != nil {
		return "", err
	}
	cfg, err := s.Load()
	if err != nil {
		return "", err
	}
	cfg.VaultPath = abs
	if err := s.Save(cfg); err != nil {
		return "", err
	}
	return abs, nil
}

// GetShowNotebookPrefix returns the effective sidebar prefix setting.
func (s *Store) GetShowNotebookPrefix() (bool, error) {
	cfg, err := s.Load()
	if err != nil {
		return false, err
	}
	return cfg.ShowNotebookPrefixEnabled(), nil
}

// SetShowNotebookPrefix persists the sidebar prefix setting, preserving other settings.
func (s *Store) SetShowNotebookPrefix(enabled bool) error {
	cfg, err := s.Load()
	if err != nil {
		return err
	}
	cfg.ShowNotebookPrefix = &enabled
	return s.Save(cfg)
}

// GetShowLineNumbers returns the effective editor line-number setting.
func (s *Store) GetShowLineNumbers() (bool, error) {
	cfg, err := s.Load()
	if err != nil {
		return false, err
	}
	return cfg.ShowLineNumbersEnabled(), nil
}

// SetShowLineNumbers persists the editor line-number setting, preserving other settings.
func (s *Store) SetShowLineNumbers(enabled bool) error {
	cfg, err := s.Load()
	if err != nil {
		return err
	}
	cfg.ShowLineNumbers = &enabled
	return s.Save(cfg)
}

// GetSetting fetches a known setting value by key.
// If unset, returns ("", false, nil).
func (s *Store) GetSetting(key string) (string, bool, error) {
	norm := strings.ToLower(strings.TrimSpace(key))
	if norm == "" {
		return "", false, fmt.Errorf("%w: setting key is required", ErrInvalidInput)
	}
	cfg, err := s.Load()
	if err != nil {
		return "", false, err
	}
	switch norm {
	case KeyVaultPath:
		if cfg.VaultPath == "" {
			return "", false, nil
		}
		return cfg.VaultPath, true, nil
	case KeyShowNotebookPrefix:
		return strconv.FormatBool(cfg.ShowNotebookPrefixEnabled()), true, nil
	case KeyShowLineNumbers:
		return strconv.FormatBool(cfg.ShowLineNumbersEnabled()), true, nil
	default:
		return "", false, nil
	}
}

// SetSetting updates a known setting key in config.yaml.
func (s *Store) SetSetting(key, value string) error {
	norm := strings.ToLower(strings.TrimSpace(key))
	if norm == "" {
		return fmt.Errorf("%w: setting key is required", ErrInvalidInput)
	}
	switch norm {
	case KeyVaultPath:
		_, err := s.SetVaultPath(value)
		return err
	case KeyShowNotebookPrefix:
		enabled, err := ParseBool(value)
		if err != nil {
			return err
		}
		return s.SetShowNotebookPrefix(enabled)
	case KeyShowLineNumbers:
		enabled, err := ParseBool(value)
		if err != nil {
			return err
		}
		return s.SetShowLineNumbers(enabled)
	default:
		return fmt.Errorf("%w: unknown setting key %q", ErrInvalidInput, key)
	}
}

// DeleteSetting clears a known setting key from config.yaml.
func (s *Store) DeleteSetting(key string) error {
	norm := strings.ToLower(strings.TrimSpace(key))
	if norm == "" {
		return fmt.Errorf("%w: setting key is required", ErrInvalidInput)
	}
	cfg, err := s.Load()
	if err != nil {
		return err
	}
	switch norm {
	case KeyVaultPath:
		if cfg.VaultPath == "" {
			return fmt.Errorf("%w: %q", ErrNotFound, key)
		}
		cfg.VaultPath = ""
		return s.Save(cfg)
	case KeyShowNotebookPrefix:
		if cfg.ShowNotebookPrefix == nil {
			return fmt.Errorf("%w: %q", ErrNotFound, key)
		}
		cfg.ShowNotebookPrefix = nil
		return s.Save(cfg)
	case KeyShowLineNumbers:
		if cfg.ShowLineNumbers == nil {
			return fmt.Errorf("%w: %q", ErrNotFound, key)
		}
		cfg.ShowLineNumbers = nil
		return s.Save(cfg)
	default:
		return fmt.Errorf("%w: %q", ErrNotFound, key)
	}
}

// ListSettings returns all configured settings ordered by key ascending.
func (s *Store) ListSettings() ([]Setting, error) {
	cfg, err := s.Load()
	if err != nil {
		return nil, err
	}
	var out []Setting
	if cfg.ShowLineNumbers != nil {
		out = append(out, Setting{Key: KeyShowLineNumbers, Value: strconv.FormatBool(*cfg.ShowLineNumbers)})
	}
	if cfg.ShowNotebookPrefix != nil {
		out = append(out, Setting{Key: KeyShowNotebookPrefix, Value: strconv.FormatBool(*cfg.ShowNotebookPrefix)})
	}
	if cfg.VaultPath != "" {
		out = append(out, Setting{Key: KeyVaultPath, Value: cfg.VaultPath})
	}
	return out, nil
}

// NormalizePath expands a leading ~/ (or bare ~) and returns a clean absolute path.
func NormalizePath(raw string) (string, error) {
	p := strings.TrimSpace(raw)
	if p == "" {
		return "", fmt.Errorf("%w: path cannot be empty", ErrInvalidInput)
	}
	if p == "~" || strings.HasPrefix(p, "~/") || strings.HasPrefix(p, `~\`) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home directory: %w", err)
		}
		if p == "~" {
			p = home
		} else {
			p = filepath.Join(home, p[2:])
		}
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", fmt.Errorf("resolve absolute path: %w", err)
	}
	return filepath.Clean(abs), nil
}
