package config

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FileName is the workspace configuration filename discovered upward from cwd.
const FileName = ".vaultnotecli"

// Config holds read-only workspace defaults loaded from .vaultnotecli.
type Config struct {
	Path      string
	Notebook  string
	VaultPath string
}

// Load searches upward from cwd for a .vaultnotecli file, stopping at $HOME
// (inclusive). If cwd is outside $HOME, it ascends to the filesystem root.
// If no config file is found, it returns an empty *Config and nil error.
func Load(cwd string) (*Config, error) {
	if cwd == "" {
		var err error
		cwd, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("get working directory: %w", err)
		}
	}
	absCwd, err := filepath.Abs(cwd)
	if err != nil {
		return nil, fmt.Errorf("resolve working directory: %w", err)
	}

	home, _ := os.UserHomeDir()
	var absHome string
	if home != "" {
		absHome, _ = filepath.Abs(home)
	}
	withinHome := absHome != "" && isSubpathOrEqual(absCwd, absHome)

	dir := absCwd
	for {
		candidate := filepath.Join(dir, FileName)
		info, err := os.Stat(candidate)
		if err == nil && !info.IsDir() {
			return parseFile(candidate)
		}
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("stat %s: %w", candidate, err)
		}

		if withinHome && dir == absHome {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return &Config{}, nil
}

func isSubpathOrEqual(path, base string) bool {
	rel, err := filepath.Rel(base, path)
	if err != nil {
		return false
	}
	return rel == "." || (!strings.HasPrefix(rel, ".."+string(os.PathSeparator)) && rel != "..")
}

func parseFile(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	cfg := &Config{Path: path}
	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		}
		eqIdx := strings.IndexByte(line, '=')
		if eqIdx <= 0 {
			return nil, fmt.Errorf("%s:%d: invalid syntax, expected KEY=VALUE", path, lineNum)
		}
		key := strings.TrimSpace(line[:eqIdx])
		val := strings.TrimSpace(line[eqIdx+1:])

		unquoted, err := unquoteValue(val)
		if err != nil {
			return nil, fmt.Errorf("%s:%d: %w", path, lineNum, err)
		}

		switch key {
		case "NOTEBOOK", "NOTEBOOK_CODE":
			cfg.Notebook = strings.TrimSpace(unquoted)
		case "VAULT_PATH":
			cfg.VaultPath = strings.TrimSpace(unquoted)
		default:
			return nil, fmt.Errorf("%s:%d: unknown configuration key %q", path, lineNum, key)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	return cfg, nil
}

func unquoteValue(raw string) (string, error) {
	if len(raw) == 0 {
		return "", nil
	}
	first := raw[0]
	if first == '"' || first == '\'' {
		if len(raw) < 2 || raw[len(raw)-1] != first {
			return "", errors.New("unterminated quoted value")
		}
		return raw[1 : len(raw)-1], nil
	}
	return raw, nil
}
