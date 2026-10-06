// Package cue implements a minimal CUE sheet parser, mirroring musipal.cue.
package cue

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Track is a single CUE track with its computed start/end offsets.
type Track struct {
	Title        string
	Performer    string
	SourceFile   string
	StartSeconds float64
	EndSeconds   *float64 // nil means "until end of file"
}

func mmssffToSeconds(mm, ss, ff int) float64 {
	return float64(mm*60+ss) + float64(ff)/75.0
}

type rawTrack struct {
	title     string
	performer string
	index     *float64
}

func stripQuotes(s string) string {
	return strings.Trim(s, `"`)
}

func fieldAfter(line string) string {
	parts := strings.SplitN(line, " ", 2)
	if len(parts) < 2 {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

// Parse reads a minimal CUE file: FILE + TRACK + TITLE + PERFORMER + INDEX 01.
func Parse(cuePath string) ([]Track, error) {
	data, err := os.ReadFile(cuePath)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(data), "\n")

	var albumFile string
	albumPerformer := ""

	var tracksRaw []rawTrack
	var current *rawTrack

	for _, raw := range lines {
		line := strings.TrimSpace(strings.TrimRight(raw, "\r"))
		if line == "" || strings.HasPrefix(line, "REM") {
			continue
		}
		upper := strings.ToUpper(line)

		if strings.HasPrefix(upper, "PERFORMER") && current == nil {
			albumPerformer = stripQuotes(fieldAfter(line))
			continue
		}

		if strings.HasPrefix(upper, "FILE") {
			rest := fieldAfter(line)
			quoted := strings.SplitN(rest, " ", 2)[0]
			fileName := stripQuotes(quoted)
			if fileName != "" {
				albumFile = filepath.Join(filepath.Dir(cuePath), fileName)
				if abs, err := filepath.Abs(albumFile); err == nil {
					albumFile = abs
				}
			}
			continue
		}

		if strings.HasPrefix(upper, "TRACK") {
			if current != nil {
				tracksRaw = append(tracksRaw, *current)
			}
			current = &rawTrack{}
			continue
		}

		if current != nil && strings.HasPrefix(upper, "TITLE") {
			current.title = stripQuotes(fieldAfter(line))
			continue
		}

		if current != nil && strings.HasPrefix(upper, "PERFORMER") {
			current.performer = stripQuotes(fieldAfter(line))
			continue
		}

		if current != nil && strings.HasPrefix(upper, "INDEX 01") {
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				t := parts[2]
				comps := strings.Split(t, ":")
				if len(comps) == 3 {
					mm, e1 := strconv.Atoi(comps[0])
					ss, e2 := strconv.Atoi(comps[1])
					ff, e3 := strconv.Atoi(comps[2])
					if e1 == nil && e2 == nil && e3 == nil {
						v := mmssffToSeconds(mm, ss, ff)
						current.index = &v
					}
				}
			}
			continue
		}
	}

	if current != nil {
		tracksRaw = append(tracksRaw, *current)
	}

	if albumFile == "" {
		ext := filepath.Ext(cuePath)
		albumFile = strings.TrimSuffix(cuePath, ext) + ".flac"
	}

	var tracks []Track
	for i, tr := range tracksRaw {
		if tr.index == nil {
			continue
		}
		start := *tr.index

		var end *float64
		for j := i + 1; j < len(tracksRaw); j++ {
			if tracksRaw[j].index != nil {
				v := *tracksRaw[j].index
				end = &v
				break
			}
		}

		title := tr.title
		if title == "" {
			title = "Track " + padLeft(strconv.Itoa(i+1), 2)
		}
		performer := tr.performer
		if performer == "" {
			performer = albumPerformer
		}

		tracks = append(tracks, Track{
			Title:        title,
			Performer:    performer,
			SourceFile:   albumFile,
			StartSeconds: start,
			EndSeconds:   end,
		})
	}

	return tracks, nil
}

func padLeft(s string, width int) string {
	for len(s) < width {
		s = "0" + s
	}
	return s
}

// SourceFile does a best-effort extraction of the referenced FILE path in a
// .cue file without fully parsing tracks.
func SourceFile(cuePath string) (string, bool) {
	data, err := os.ReadFile(cuePath)
	if err != nil {
		return "", false
	}
	for _, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(strings.TrimRight(raw, "\r"))
		if line == "" {
			continue
		}
		if strings.HasPrefix(strings.ToUpper(line), "FILE") {
			rest := fieldAfter(line)
			quoted := strings.SplitN(rest, " ", 2)[0]
			fileName := stripQuotes(quoted)
			if fileName == "" {
				return "", false
			}
			full := filepath.Join(filepath.Dir(cuePath), fileName)
			if abs, err := filepath.Abs(full); err == nil {
				full = abs
			}
			return full, true
		}
	}
	return "", false
}
