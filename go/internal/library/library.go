// Package library lists directory contents filtered to supported audio
// files/cue sheets and subdirectories, mirroring musipal.library.
package library

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var supportedAudioExts = map[string]bool{
	".mp3":  true,
	".ogg":  true,
	".flac": true,
}

const supportedCueExt = ".cue"

// Item is a single library entry (directory or audio/cue file).
type Item struct {
	Path  string
	IsDir bool
}

func isSupported(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	return supportedAudioExts[ext] || ext == supportedCueExt
}

// ListDir lists the contents of path, returning directories first then
// supported audio/cue files, both sorted case-insensitively by name.
func ListDir(path string) []Item {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil
	}

	items := make([]Item, 0, len(entries))
	for _, e := range entries {
		full := filepath.Join(path, e.Name())
		if e.IsDir() {
			items = append(items, Item{Path: full, IsDir: true})
			continue
		}
		if isSupported(e.Name()) {
			items = append(items, Item{Path: full, IsDir: false})
		}
	}

	sort.SliceStable(items, func(i, j int) bool {
		if items[i].IsDir != items[j].IsDir {
			return items[i].IsDir
		}
		return strings.ToLower(filepath.Base(items[i].Path)) < strings.ToLower(filepath.Base(items[j].Path))
	})
	return items
}

// IterAudioFilesRecursive returns all supported audio/cue files under path
// (or just path itself if it's a file), sorted by full path.
func IterAudioFilesRecursive(path string) []string {
	info, err := os.Stat(path)
	if err != nil {
		return nil
	}
	if !info.IsDir() {
		return []string{path}
	}

	var results []string
	_ = filepath.Walk(path, func(p string, fi os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if fi.IsDir() {
			return nil
		}
		if isSupported(fi.Name()) {
			results = append(results, p)
		}
		return nil
	})
	sort.Strings(results)
	return results
}
