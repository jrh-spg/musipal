package tui

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"musipal-go/internal/cue"
	"musipal-go/internal/durations"
)

func durationSeconds(p string) *float64 {
	return durations.GetDurationSeconds(p)
}

func cueSourceFile(p string) (string, bool) {
	return cue.SourceFile(p)
}

// uriToPath returns the local filesystem path for a URI/path string, or
// ("", false) if it looks like a remote URL.
func uriToPath(uri string) (string, bool) {
	if u, err := url.Parse(uri); err == nil && u.Scheme == "file" {
		return u.Path, true
	}
	if strings.Contains(uri, "://") {
		return "", false
	}
	return uri, true
}

// trimDisplayDash mirrors Python's `"{a} - {b}".strip(" -")`.
func trimDisplayDash(s string) string {
	return strings.Trim(s, " -")
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func baseName(p string) string {
	return filepath.Base(p)
}
