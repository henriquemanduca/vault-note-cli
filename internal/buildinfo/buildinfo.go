package buildinfo

import (
	"fmt"
	"runtime"
)

// Variables injected at build time via -ldflags.
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// Summary returns a single-line human-readable build identifier.
func Summary() string {
	return fmt.Sprintf("vault-note-cli %s (commit %s, built %s, %s %s/%s)",
		Version, Commit, Date, runtime.Version(), runtime.GOOS, runtime.GOARCH)
}
