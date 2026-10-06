// Package icecast streams a local/remote audio source to an Icecast server
// using ffmpeg's built-in icecast:// output protocol, mirroring
// musipal.icecast.
package icecast

import (
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"sync"
)

// Streamer manages a single ffmpeg encode-and-push process.
type Streamer struct {
	mu        sync.Mutex
	cmd       *exec.Cmd
	lastError string
}

// Available reports whether ffmpeg is on PATH.
func (s *Streamer) Available() bool {
	_, err := exec.LookPath("ffmpeg")
	return err == nil
}

// IsStreaming reports whether ffmpeg is currently running.
func (s *Streamer) IsStreaming() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cmd != nil
}

// LastError returns the most recent ffmpeg stderr output, if any.
func (s *Streamer) LastError() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastError
}

// StartOptions configures a streaming session.
type StartOptions struct {
	Bitrate      int
	StartSeconds *float64
	Duration     *float64
	Title        string
	Name         string
	Description  string
	Genre        string
	ContentType  string
}

// Start begins streaming inputURI to destURL (an icecast:// ffmpeg URL).
func (s *Streamer) Start(inputURI, destURL string, opts StartOptions) error {
	if !s.Available() {
		return fmt.Errorf("ffmpeg not found on PATH")
	}
	if s.IsStreaming() {
		return fmt.Errorf("already streaming")
	}
	if !strings.HasPrefix(destURL, "icecast://") {
		return fmt.Errorf("invalid Icecast destination URL (expected icecast://...): %s", destURL)
	}

	bitrate := opts.Bitrate
	if bitrate == 0 {
		bitrate = 128
	}
	contentType := opts.ContentType
	if contentType == "" {
		contentType = "audio/mpeg"
	}
	name := opts.Name
	if name == "" {
		name = "Musipal"
	}

	args := []string{"-hide_banner", "-loglevel", "error", "-re"}
	if opts.StartSeconds != nil && *opts.StartSeconds > 0 {
		args = append(args, "-ss", strconv.FormatFloat(*opts.StartSeconds, 'f', -1, 64))
	}
	args = append(args, "-i", inputURI)
	if opts.Duration != nil && *opts.Duration > 0 {
		args = append(args, "-t", strconv.FormatFloat(*opts.Duration, 'f', -1, 64))
	}
	args = append(args, "-c:a", "libmp3lame", "-b:a", fmt.Sprintf("%dk", bitrate), "-f", "mp3")
	if opts.Title != "" {
		args = append(args, "-metadata", "title="+opts.Title)
	}
	args = append(args, "-ice_name", name)
	if opts.Description != "" {
		args = append(args, "-ice_description", opts.Description)
	}
	if opts.Genre != "" {
		args = append(args, "-ice_genre", opts.Genre)
	}
	args = append(args, "-ice_public", "0", "-content_type", contentType, "-legacy_icecast", "1", destURL)

	cmd := exec.Command("ffmpeg", args...)
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to start ffmpeg: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start ffmpeg: %w", err)
	}

	s.mu.Lock()
	s.cmd = cmd
	s.lastError = ""
	s.mu.Unlock()

	go func() {
		errBytes, _ := io.ReadAll(stderr)
		_ = cmd.Wait()
		errText := strings.TrimSpace(string(errBytes))

		s.mu.Lock()
		if errText != "" {
			s.lastError = errText
		}
		if s.cmd == cmd {
			s.cmd = nil
		}
		s.mu.Unlock()
	}()

	return nil
}

// Stop terminates the ffmpeg process, if running.
func (s *Streamer) Stop() {
	s.mu.Lock()
	cmd := s.cmd
	s.cmd = nil
	s.mu.Unlock()
	if cmd == nil || cmd.Process == nil {
		return
	}
	_ = cmd.Process.Kill()
}

var (
	defaultStreamer     *Streamer
	defaultStreamerOnce sync.Once
)

// GetStreamer returns the process-wide default streamer instance.
func GetStreamer() *Streamer {
	defaultStreamerOnce.Do(func() {
		defaultStreamer = &Streamer{}
	})
	return defaultStreamer
}
