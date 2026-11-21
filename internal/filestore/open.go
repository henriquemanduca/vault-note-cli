package filestore

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// OpenResult indicates whether a desktop file manager was launched or if the
// caller should display the resolved path (headless fallback).
type OpenResult struct {
	Path   string
	Opened bool
}

// OpenFolder opens a directory in the OS file manager, falling back cleanly
// when no GUI session is available.
func OpenFolder(path string) (OpenResult, error) {
	return openTarget(path, false)
}

// OpenFile reveals or opens a file in the OS file manager, falling back
// cleanly when no GUI session is available.
func OpenFile(path string) (OpenResult, error) {
	return openTarget(path, true)
}

func openTarget(path string, isFile bool) (OpenResult, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return OpenResult{}, fmt.Errorf("resolve path: %w", err)
	}
	if _, err := os.Stat(absPath); err != nil {
		return OpenResult{}, fmt.Errorf("path does not exist: %w", err)
	}

	if isHeadless(runtime.GOOS) {
		return OpenResult{Path: absPath, Opened: false}, nil
	}

	cmd, err := openerCommand(runtime.GOOS, absPath, isFile)
	if err != nil {
		return OpenResult{Path: absPath, Opened: false}, nil
	}
	if err := cmd.Start(); err != nil {
		return OpenResult{Path: absPath, Opened: false}, nil
	}
	return OpenResult{Path: absPath, Opened: true}, nil
}

func isHeadless(goos string) bool {
	if goos == "darwin" || goos == "windows" {
		return false
	}
	return os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == ""
}

func openerCommand(goos, absPath string, isFile bool) (*exec.Cmd, error) {
	switch goos {
	case "darwin":
		if _, err := exec.LookPath("open"); err != nil {
			return nil, err
		}
		if isFile {
			return exec.Command("open", "-R", absPath), nil
		}
		return exec.Command("open", absPath), nil
	case "windows":
		if _, err := exec.LookPath("explorer.exe"); err != nil {
			return nil, err
		}
		if isFile {
			return exec.Command("explorer.exe", "/select,"+absPath), nil
		}
		return exec.Command("explorer.exe", absPath), nil
	default:
		if _, err := exec.LookPath("xdg-open"); err != nil {
			return nil, err
		}
		target := absPath
		if isFile {
			target = filepath.Dir(absPath)
		}
		return exec.Command("xdg-open", target), nil
	}
}
