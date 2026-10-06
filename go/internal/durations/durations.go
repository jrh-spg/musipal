// Package durations computes/formats audio durations and bitrates,
// mirroring musipal.durations. Duration/bitrate are obtained via ffprobe
// since Go has no mainstream pure dependency-free audio decoder for all of
// mp3/ogg/flac.
package durations

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// GetDurationSeconds returns the duration of a local audio file in seconds,
// or nil if unavailable.
func GetDurationSeconds(path string) *float64 {
	out, err := exec.Command("ffprobe", "-v", "error", "-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1", path).Output()
	if err != nil {
		return nil
	}
	s := strings.TrimSpace(string(out))
	if s == "" {
		return nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil
	}
	return &v
}

// GetBitrateKbps returns the nominal bitrate in kbps for a local audio file.
func GetBitrateKbps(path string) *int {
	out, err := exec.Command("ffprobe", "-v", "error", "-select_streams", "a:0",
		"-show_entries", "stream=bit_rate", "-of", "default=noprint_wrappers=1:nokey=1", path).Output()
	if err != nil {
		return nil
	}
	s := strings.TrimSpace(string(out))
	if s == "" {
		return nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || v <= 0 {
		return nil
	}
	kbps := int(v/1000.0 + 0.5)
	return &kbps
}

// FormatDuration formats seconds as h:mm:ss or m:ss, or "--:--" if nil/negative.
func FormatDuration(seconds *float64) string {
	if seconds == nil || *seconds < 0 {
		return "--:--"
	}
	total := int(*seconds + 0.5)
	h := total / 3600
	m := (total % 3600) / 60
	s := total % 60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}
