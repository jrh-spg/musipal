// Package session persists/restores the playback queue and position between
// runs, mirroring musipal.session (same JSON schema/path for compatibility).
package session

import (
	"encoding/json"
	"os"
	"path/filepath"

	"musipal/internal/player"
)

// State is the persisted session (queue + playback position).
type State struct {
	Queue              []player.QueueItem
	Index              int
	Elapsed            *float64
	WasPlaying         bool
	QueueSelectedIndex int
}

type jsonItem struct {
	URI     string   `json:"uri"`
	Display string   `json:"display"`
	Start   *float64 `json:"start"`
	Stop    *float64 `json:"stop"`
}

type jsonState struct {
	Index              int        `json:"index"`
	Elapsed            *float64   `json:"elapsed"`
	WasPlaying         bool       `json:"was_playing"`
	QueueSelectedIndex int        `json:"queue_selected_index"`
	Queue              []jsonItem `json:"queue"`
}

func sessionPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "musipal", "last_session.json")
}

// Load reads the saved session, returning nil if none exists or it's invalid.
func Load() *State {
	p := sessionPath()
	data, err := os.ReadFile(p)
	if err != nil {
		return nil
	}

	var js jsonState
	if err := json.Unmarshal(data, &js); err != nil {
		return nil
	}

	var q []player.QueueItem
	for _, it := range js.Queue {
		if it.URI == "" {
			continue
		}
		display := it.Display
		if display == "" {
			display = it.URI
		}
		q = append(q, player.QueueItem{URI: it.URI, Display: display, Start: it.Start, Stop: it.Stop})
	}
	if len(q) == 0 {
		return nil
	}

	index := js.Index
	if index < 0 {
		index = 0
	}
	if index > len(q)-1 {
		index = len(q) - 1
	}
	qsi := js.QueueSelectedIndex
	if qsi < 0 {
		qsi = 0
	}
	if qsi > len(q)-1 {
		qsi = len(q) - 1
	}

	return &State{
		Queue:              q,
		Index:              index,
		Elapsed:            js.Elapsed,
		WasPlaying:         js.WasPlaying,
		QueueSelectedIndex: qsi,
	}
}

// Save writes the session to disk, best-effort (errors are swallowed since
// failing to save shouldn't crash the app).
func Save(s State) {
	p := sessionPath()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return
	}

	items := make([]jsonItem, 0, len(s.Queue))
	for _, it := range s.Queue {
		items = append(items, jsonItem{URI: it.URI, Display: it.Display, Start: it.Start, Stop: it.Stop})
	}
	js := jsonState{
		Index:              s.Index,
		Elapsed:            s.Elapsed,
		WasPlaying:         s.WasPlaying,
		QueueSelectedIndex: s.QueueSelectedIndex,
		Queue:              items,
	}

	data, err := json.MarshalIndent(js, "", "  ")
	if err != nil {
		return
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return
	}
	_ = os.Rename(tmp, p)
}
