package player

import (
	"os/exec"
	"strconv"
	"strings"
)

// timeFmt formats a float seconds value for passing to ffplay/ffmpeg -ss args.
func timeFmt(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}

func probeDuration(uri string) *float64 {
	if _, err := exec.LookPath("ffprobe"); err != nil {
		return nil
	}
	out, err := exec.Command("ffprobe", "-v", "error", "-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1", uri).Output()
	if err != nil {
		return nil
	}
	s := strings.TrimSpace(string(out))
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil
	}
	return &v
}

func probeBitrate(uri string) *int {
	if _, err := exec.LookPath("ffprobe"); err != nil {
		return nil
	}
	out, err := exec.Command("ffprobe", "-v", "error", "-select_streams", "a:0",
		"-show_entries", "stream=bit_rate", "-of", "default=noprint_wrappers=1:nokey=1", uri).Output()
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
	kbps := int(v/1000.0 + 0.5)
	return &kbps
}
