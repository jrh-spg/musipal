// Package metadata extracts a best-effort display name for audio files.
// Uses dhowden/tag (pure Go) for ID3/Vorbis/FLAC tags, with an mtime-keyed
// cache to avoid re-reading unchanged files.
package metadata

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/dhowden/tag"
)

var (
	cacheMu sync.Mutex
	cache   = map[string]cacheEntry{}
)

type cacheEntry struct {
	mtime int64
	name  string
}

var leadingTrackNum = regexp.MustCompile(`^\s*\d+\s*[-_.]?\s*`)
var multiSpace = regexp.MustCompile(`\s+`)

func cleanupStem(stem string) string {
	s := leadingTrackNum.ReplaceAllString(stem, "")
	s = strings.ReplaceAll(s, "_", " ")
	s = strings.ReplaceAll(s, "-", " ")
	s = multiSpace.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

// PrettyName returns a best-effort pretty display name for an audio file,
// preferring artist/title tags over the cleaned filename stem.
func PrettyName(path string) string {
	info, err := os.Stat(path)
	if err != nil {
		return filepath.Base(path)
	}
	if info.IsDir() {
		return filepath.Base(path)
	}

	mtime := info.ModTime().UnixNano()

	cacheMu.Lock()
	if e, ok := cache[path]; ok && e.mtime == mtime {
		cacheMu.Unlock()
		return e.name
	}
	cacheMu.Unlock()

	name := ""
	if f, err := os.Open(path); err == nil {
		if m, err := tag.ReadFrom(f); err == nil {
			title := strings.TrimSpace(m.Title())
			artist := strings.TrimSpace(m.Artist())
			if title != "" && artist != "" {
				name = artist + " — " + title
			} else if title != "" {
				name = title
			} else if artist != "" {
				stem := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
				name = artist + " — " + stem
			}
		}
		f.Close()
	}

	if name == "" {
		stem := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		name = cleanupStem(stem)
	}

	cacheMu.Lock()
	cache[path] = cacheEntry{mtime: mtime, name: name}
	cacheMu.Unlock()
	return name
}
