// Package playlists reads/writes simple M3U playlists, mirroring
// musipal.playlists.
package playlists

import (
	"os"
	"path/filepath"
	"strings"
)

// Entry is a single playlist entry (file path or URL).
type Entry struct {
	URI     string
	Display string
}

// WriteM3U writes entries to path as a simple #EXTM3U playlist (URIs only,
// matching the Python implementation).
func WriteM3U(path string, entries []Entry) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var b strings.Builder
	b.WriteString("#EXTM3U\n")
	for _, e := range entries {
		b.WriteString(e.URI)
		b.WriteString("\n")
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

// LoadM3U reads the URIs listed in an M3U playlist, skipping blank lines and
// comments. Returns nil if the file doesn't exist.
func LoadM3U(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var uris []string
	for _, line := range strings.Split(string(data), "\n") {
		s := strings.TrimSpace(line)
		if s == "" || strings.HasPrefix(s, "#") {
			continue
		}
		uris = append(uris, s)
	}
	return uris
}
