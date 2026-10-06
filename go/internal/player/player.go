// Package player implements an ffplay-backed audio player, mirroring
// musipal.ffmpeg_player (the ffmpeg/ffplay-based backend), since there are
// no good pure-Go libvlc bindings without cgo.
//
// Pause/resume uses SIGSTOP/SIGCONT on the process group; seeking restarts
// ffplay with -ss (best-effort), matching the Python implementation.
package player

import (
	"os/exec"
	"sync"
	"syscall"
	"time"
)

// QueueItem is a single queued track (local path or URL), optionally scoped
// to a CUE start/stop time range.
type QueueItem struct {
	URI     string
	Display string
	Start   *float64
	Stop    *float64
}

// Player manages a queue of items and plays them one at a time via ffplay.
type Player struct {
	mu           sync.Mutex
	queue        []QueueItem
	currentIndex int // -1 means none
	cmd          *exec.Cmd
	startTime    time.Time
	startOffset  float64
	paused       bool
	generation   int // bumped whenever we intentionally stop/replace the process
}

// New creates an empty player.
func New() *Player {
	return &Player{currentIndex: -1}
}

// Queue returns a copy of the current queue.
func (p *Player) Queue() []QueueItem {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]QueueItem, len(p.queue))
	copy(out, p.queue)
	return out
}

// Clear stops playback and empties the queue.
func (p *Player) Clear() {
	p.Stop()
	p.mu.Lock()
	p.queue = nil
	p.currentIndex = -1
	p.mu.Unlock()
}

// Add appends an item to the queue.
func (p *Player) Add(item QueueItem) {
	p.mu.Lock()
	p.queue = append(p.queue, item)
	if p.currentIndex < 0 {
		p.currentIndex = 0
	}
	p.mu.Unlock()
}

// AddPath appends a local file path to the queue.
func (p *Player) AddPath(path string, display string) {
	if display == "" {
		display = path
	}
	p.Add(QueueItem{URI: path, Display: display})
}

// AddURL appends a URL to the queue.
func (p *Player) AddURL(url string, display string) {
	if display == "" {
		display = url
	}
	p.Add(QueueItem{URI: url, Display: display})
}

// Remove removes and returns the item at index, adjusting currentIndex.
func (p *Player) Remove(index int) (QueueItem, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if index < 0 || index >= len(p.queue) {
		return QueueItem{}, false
	}
	removed := p.queue[index]
	p.queue = append(p.queue[:index:index], p.queue[index+1:]...)

	if len(p.queue) == 0 {
		p.currentIndex = -1
	} else if p.currentIndex >= 0 {
		if index < p.currentIndex {
			p.currentIndex--
		} else if index == p.currentIndex {
			if p.currentIndex >= len(p.queue) {
				p.currentIndex = len(p.queue) - 1
			}
		}
	}
	return removed, true
}

func ffplayCmd(uri string, start *float64) []string {
	cmd := []string{"ffplay", "-nodisp", "-autoexit", "-hide_banner", "-loglevel", "error"}
	if start != nil && *start > 0 {
		cmd = append(cmd, "-ss", ftoa(*start))
	}
	cmd = append(cmd, uri)
	return cmd
}

func ftoa(f float64) string {
	return timeFmt(f)
}

// startProcessLocked starts ffplay for uri/start. Caller must hold p.mu.
func (p *Player) startProcessLocked(uri string, start *float64) error {
	p.stopLocked()

	argv := ffplayCmd(uri, start)
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		return err
	}

	p.cmd = cmd
	p.startTime = time.Now()
	if start != nil {
		p.startOffset = *start
	} else {
		p.startOffset = 0
	}
	p.paused = false
	p.generation++
	gen := p.generation

	go p.watch(cmd, gen)
	return nil
}

// watch waits for the ffplay process to exit and advances the queue if it
// exited on its own (not via an intentional stop, detected via generation).
func (p *Player) watch(cmd *exec.Cmd, gen int) {
	_ = cmd.Wait()

	p.mu.Lock()
	defer p.mu.Unlock()
	if p.generation != gen {
		// A newer process has since started/stopped; nothing to do.
		return
	}
	p.cmd = nil

	if p.currentIndex >= 0 && p.currentIndex+1 < len(p.queue) {
		p.currentIndex++
		next := p.queue[p.currentIndex]
		_ = p.startProcessLocked(next.URI, next.Start)
	}
}

// Play plays the current queue item from its configured start (or 0).
func (p *Player) Play() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.currentIndex < 0 || p.currentIndex >= len(p.queue) {
		return
	}
	item := p.queue[p.currentIndex]
	_ = p.startProcessLocked(item.URI, item.Start)
}

// PlayIndex plays the queue item at index.
func (p *Player) PlayIndex(index int) {
	p.mu.Lock()
	if index < 0 || index >= len(p.queue) {
		p.mu.Unlock()
		return
	}
	p.currentIndex = index
	item := p.queue[index]
	_ = p.startProcessLocked(item.URI, item.Start)
	p.mu.Unlock()
}

// Pause sends SIGSTOP to the playing process group.
func (p *Player) Pause() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cmd == nil || p.cmd.Process == nil {
		return
	}
	_ = syscall.Kill(-p.cmd.Process.Pid, syscall.SIGSTOP)
	p.paused = true
}

// SeekSeconds restarts the current item at an absolute position.
func (p *Player) SeekSeconds(seconds float64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.currentIndex < 0 || p.currentIndex >= len(p.queue) {
		return
	}
	item := p.queue[p.currentIndex]
	_ = p.startProcessLocked(item.URI, &seconds)
}

// TogglePause toggles play/pause via SIGCONT/SIGSTOP.
func (p *Player) TogglePause() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cmd == nil || p.cmd.Process == nil {
		return
	}
	pid := p.cmd.Process.Pid
	if p.paused {
		_ = syscall.Kill(-pid, syscall.SIGCONT)
		p.startTime = time.Now().Add(-time.Duration(p.startOffset * float64(time.Second)))
		p.paused = false
	} else {
		_ = syscall.Kill(-pid, syscall.SIGSTOP)
		p.paused = true
	}
}

// stopLocked terminates the current process (if any). Caller must hold p.mu.
func (p *Player) stopLocked() {
	if p.cmd == nil || p.cmd.Process == nil {
		return
	}
	pid := p.cmd.Process.Pid
	p.generation++ // invalidate the watcher for this process
	_ = syscall.Kill(-pid, syscall.SIGTERM)

	done := make(chan struct{})
	cmd := p.cmd
	go func() {
		_ = cmd.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		_ = syscall.Kill(-pid, syscall.SIGKILL)
	}

	p.cmd = nil
	p.startTime = time.Time{}
	p.startOffset = 0
	p.paused = false
}

// Stop terminates playback.
func (p *Player) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.stopLocked()
}

// Next advances to and plays the next queue item.
func (p *Player) Next() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.queue) == 0 {
		return
	}
	if p.currentIndex < 0 {
		p.currentIndex = 0
	} else if p.currentIndex < len(p.queue)-1 {
		p.currentIndex++
	}
	item := p.queue[p.currentIndex]
	_ = p.startProcessLocked(item.URI, item.Start)
}

// Previous moves to and plays the previous queue item.
func (p *Player) Previous() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.queue) == 0 {
		return
	}
	if p.currentIndex < 0 {
		p.currentIndex = 0
	} else if p.currentIndex > 0 {
		p.currentIndex--
	}
	item := p.queue[p.currentIndex]
	_ = p.startProcessLocked(item.URI, item.Start)
}

// IsPlaying reports whether a process is running and not paused.
func (p *Player) IsPlaying() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.cmd != nil && !p.paused
}

// NowPlaying returns the URI of the current item, or "".
func (p *Player) NowPlaying() string {
	item := p.CurrentItem()
	if item == nil {
		return ""
	}
	return item.URI
}

// CurrentItem returns the currently selected queue item, or nil.
func (p *Player) CurrentItem() *QueueItem {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.currentIndex < 0 || p.currentIndex >= len(p.queue) {
		return nil
	}
	item := p.queue[p.currentIndex]
	return &item
}

// CurrentIndex returns the current queue index, or -1 if none.
func (p *Player) CurrentIndex() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.currentIndex
}

// SyncCurrentIndex is a no-op (kept for API parity with the VLC-backed
// player, which needed to resync from external media-change events).
func (p *Player) SyncCurrentIndex() {}

// PlaybackTimes returns (elapsed, total) seconds if available.
func (p *Player) PlaybackTimes() (*float64, *float64) {
	p.mu.Lock()
	cmdRunning := p.cmd != nil
	paused := p.paused
	startOffset := p.startOffset
	startTime := p.startTime
	item := (*QueueItem)(nil)
	if p.currentIndex >= 0 && p.currentIndex < len(p.queue) {
		it := p.queue[p.currentIndex]
		item = &it
	}
	p.mu.Unlock()

	if !cmdRunning && startTime.IsZero() {
		return nil, nil
	}

	var elapsed float64
	if paused {
		elapsed = startOffset
	} else {
		elapsed = time.Since(startTime).Seconds() + startOffset
	}

	var total *float64
	if item != nil {
		if d := probeDuration(item.URI); d != nil {
			total = d
		}
	}
	return &elapsed, total
}

// BitrateKbps returns a best-effort bitrate for the current item via ffprobe.
func (p *Player) BitrateKbps() *int {
	item := p.CurrentItem()
	if item == nil {
		return nil
	}
	return probeBitrate(item.URI)
}
