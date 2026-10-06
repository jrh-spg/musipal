package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"

	"musipal-go/internal/cue"
	"musipal-go/internal/icecast"
	"musipal-go/internal/library"
	"musipal-go/internal/metadata"
	"musipal-go/internal/player"
	"musipal-go/internal/playlists"
)

// handleKey is the Application-level input capture: it fully replaces
// per-widget key handling (mirroring the global KeyBindings in app.py).
func (a *App) handleKey(event *tcell.EventKey) *tcell.EventKey {
	if a.pages.HasPage(dialogPageName) {
		// A modal dialog is showing; let tview's normal focus-based input
		// handling manage it instead of our global bindings.
		return event
	}

	a.mu.Lock()
	inputMode := a.inputMode
	a.mu.Unlock()

	if inputMode {
		a.captureInput(event)
		return nil
	}

	switch event.Key() {
	case tcell.KeyUp:
		a.moveUp()
		return nil
	case tcell.KeyDown:
		a.moveDown()
		return nil
	case tcell.KeyPgUp:
		a.pageUp()
		return nil
	case tcell.KeyPgDn:
		a.pageDown()
		return nil
	case tcell.KeyEnter:
		a.enter()
		return nil
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		a.goUp()
		return nil
	case tcell.KeyTab:
		a.focusNext()
		return nil
	case tcell.KeyBacktab:
		a.focusPrev()
		return nil
	case tcell.KeyDelete:
		a.deleteQueueSong()
		return nil
	}

	switch event.Rune() {
	case ' ':
		a.space()
		return nil
	case 'a':
		a.add()
		return nil
	case 'A':
		a.addDirRecursive()
		return nil
	case 'd':
		a.deleteQueueSong()
		return nil
	case 'D':
		a.deletePlaylistFile()
		return nil
	case 't':
		a.toggleStreaming()
		return nil
	case 'T':
		a.setStreamServer()
		return nil
	case 'n':
		a.next()
		return nil
	case 'p':
		a.previous()
		return nil
	case 'w':
		a.writePlaylist()
		return nil
	case 'l':
		a.loadPlaylist()
		return nil
	case 'b':
		a.navBack()
		return nil
	case 'B':
		a.navForward()
		return nil
	case 'h':
		a.showHelp()
		return nil
	case 'q':
		a.quit()
		return nil
	}

	return event
}

func (a *App) captureInput(event *tcell.EventKey) {
	switch event.Key() {
	case tcell.KeyEscape:
		a.mu.Lock()
		a.inputMode = false
		a.mu.Unlock()
		a.rerender()
		return
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		a.mu.Lock()
		if n := len(a.inputBuffer); n > 0 {
			a.inputBuffer = a.inputBuffer[:n-1]
		}
		a.mu.Unlock()
		a.rerender()
		return
	case tcell.KeyEnter:
		a.mu.Lock()
		a.icecastURL = strings.TrimSpace(a.inputBuffer)
		a.inputMode = false
		a.mu.Unlock()
		a.rerender()
		return
	}

	r := event.Rune()
	if r >= 0x20 && r != 0x7f {
		a.mu.Lock()
		a.inputBuffer += string(r)
		a.mu.Unlock()
		a.rerender()
	}
}

func (a *App) currentFocus() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.focusedPane
}

func (a *App) focusNext() {
	a.mu.Lock()
	if a.focusedPane == "library" {
		a.focusedPane = "queue"
	} else {
		a.focusedPane = "library"
	}
	a.mu.Unlock()
	a.rerender()
}

func (a *App) focusPrev() {
	a.focusNext() // only two panes; next/prev are equivalent
}

func (a *App) pageStep(v *App) int {
	_, _, _, h := a.libraryView.GetInnerRect()
	usable := h - 2
	if usable < 1 {
		usable = 1
	}
	step := usable - 1
	if step < 1 {
		step = 1
	}
	return step
}

func (a *App) queuePageStep() int {
	_, _, _, h := a.queueView.GetInnerRect()
	usable := h - 2
	if usable < 1 {
		usable = 1
	}
	step := usable - 1
	if step < 1 {
		step = 1
	}
	return step
}

func (a *App) moveUp() {
	a.mu.Lock()
	if a.focusedPane == "queue" {
		if a.queueSelectedIndex > 0 {
			a.queueSelectedIndex--
		}
	} else {
		if a.selectedIndex > 0 {
			a.selectedIndex--
		}
	}
	a.mu.Unlock()
	a.rerender()
}

func (a *App) moveDown() {
	a.mu.Lock()
	if a.focusedPane == "queue" {
		n := len(a.player.Queue())
		if n > 0 && a.queueSelectedIndex < n-1 {
			a.queueSelectedIndex++
		}
	} else {
		if len(a.libraryItems) > 0 && a.selectedIndex < len(a.libraryItems)-1 {
			a.selectedIndex++
		}
	}
	a.mu.Unlock()
	a.rerender()
}

func (a *App) pageUp() {
	a.mu.Lock()
	if a.focusedPane == "queue" {
		step := a.queuePageStep()
		a.queueSelectedIndex -= step
		if a.queueSelectedIndex < 0 {
			a.queueSelectedIndex = 0
		}
	} else {
		step := a.pageStep(a)
		a.selectedIndex -= step
		if a.selectedIndex < 0 {
			a.selectedIndex = 0
		}
	}
	a.mu.Unlock()
	a.rerender()
}

func (a *App) pageDown() {
	a.mu.Lock()
	if a.focusedPane == "queue" {
		n := len(a.player.Queue())
		step := a.queuePageStep()
		a.queueSelectedIndex += step
		if n > 0 && a.queueSelectedIndex > n-1 {
			a.queueSelectedIndex = n - 1
		}
	} else {
		step := a.pageStep(a)
		a.selectedIndex += step
		if len(a.libraryItems) > 0 && a.selectedIndex > len(a.libraryItems)-1 {
			a.selectedIndex = len(a.libraryItems) - 1
		}
	}
	a.mu.Unlock()
	a.rerender()
}

func (a *App) enter() {
	if a.currentFocus() == "queue" {
		q := a.player.Queue()
		if len(q) == 0 {
			return
		}
		a.mu.Lock()
		idx := a.queueSelectedIndex
		if idx < 0 {
			idx = 0
		}
		if idx > len(q)-1 {
			idx = len(q) - 1
		}
		a.mu.Unlock()
		a.player.PlayIndex(idx)
		a.rerender()
		return
	}

	p, ok := a.selectedPath()
	if !ok {
		return
	}
	info := isDirPath(p)
	if info {
		a.mu.Lock()
		a.backStack = append(a.backStack, a.cwd)
		a.forwardStack = nil
		a.cwd = p
		a.selectedIndex = 0
		a.mu.Unlock()
		a.refreshDir()
		a.rerender()
		return
	}

	a.player.Clear()
	a.enqueuePath(p)
	a.player.Play()
	a.rerender()
}

func isDirPath(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

func (a *App) goUp() {
	a.mu.Lock()
	if a.cwd == a.root {
		a.mu.Unlock()
		return
	}
	a.backStack = append(a.backStack, a.cwd)
	a.forwardStack = nil
	a.cwd = filepath.Dir(a.cwd)
	a.selectedIndex = 0
	a.mu.Unlock()
	a.refreshDir()
	a.rerender()
}

func (a *App) navBack() {
	a.mu.Lock()
	if len(a.backStack) == 0 {
		a.mu.Unlock()
		return
	}
	a.forwardStack = append(a.forwardStack, a.cwd)
	last := a.backStack[len(a.backStack)-1]
	a.backStack = a.backStack[:len(a.backStack)-1]
	a.cwd = last
	a.selectedIndex = 0
	a.mu.Unlock()
	a.refreshDir()
	a.rerender()
}

func (a *App) navForward() {
	a.mu.Lock()
	if len(a.forwardStack) == 0 {
		a.mu.Unlock()
		return
	}
	a.backStack = append(a.backStack, a.cwd)
	last := a.forwardStack[len(a.forwardStack)-1]
	a.forwardStack = a.forwardStack[:len(a.forwardStack)-1]
	a.cwd = last
	a.selectedIndex = 0
	a.mu.Unlock()
	a.refreshDir()
	a.rerender()
}

// enqueuePath adds a single library entry to the queue, expanding CUE
// sheets into one queue item per track.
func (a *App) enqueuePath(p string) {
	if strings.ToLower(filepath.Ext(p)) == ".cue" {
		tracks, err := cue.Parse(p)
		if err != nil || len(tracks) == 0 {
			return
		}
		for _, t := range tracks {
			display := trimDisplayDash(fmt.Sprintf("%s - %s", t.Performer, t.Title))
			start := t.StartSeconds
			a.player.Add(player.QueueItem{
				URI:     t.SourceFile,
				Display: display,
				Start:   &start,
				Stop:    t.EndSeconds,
			})
		}
		return
	}

	display := metadata.PrettyName(p)
	a.player.AddPath(p, display)
}

func (a *App) add() {
	p, ok := a.selectedPath()
	if !ok {
		return
	}
	if isDirPath(p) {
		return
	}
	a.enqueuePath(p)
	a.rerender()
}

func (a *App) addDirRecursive() {
	p, ok := a.selectedPath()
	if !ok {
		return
	}
	target := p
	if !isDirPath(p) {
		target = filepath.Dir(p)
	}
	files := library.IterAudioFilesRecursive(target)
	for _, f := range files {
		a.enqueuePath(f)
	}
	a.rerender()
}

func (a *App) deleteQueueSong() {
	if a.currentFocus() != "queue" {
		return
	}
	q := a.player.Queue()
	if len(q) == 0 {
		return
	}
	a.mu.Lock()
	idx := a.queueSelectedIndex
	if idx > len(q)-1 {
		idx = len(q) - 1
	}
	if idx < 0 {
		idx = 0
	}
	a.mu.Unlock()
	if _, ok := a.player.Remove(idx); !ok {
		return
	}
	a.mu.Lock()
	if n := len(a.player.Queue()); idx >= n && n > 0 {
		a.queueSelectedIndex = n - 1
	}
	a.mu.Unlock()
	a.rerender()
}

func (a *App) deletePlaylistFile() {
	a.showInputDialog("Delete playlist", "Playlist filename (e.g. my.m3u):", "", func(name string, ok bool) {
		if !ok || strings.TrimSpace(name) == "" {
			return
		}
		path := filepath.Join(a.cfg.PlaylistsDir, name)
		if !fileExists(path) {
			a.rerender()
			return
		}
		a.showConfirmDialog("Confirm delete", fmt.Sprintf("Delete playlist %s?", filepath.Base(path)), func(confirmed bool) {
			if confirmed {
				_ = os.Remove(path)
			}
			a.rerender()
		})
	})
}

func (a *App) toggleStreaming() {
	streamer := icecast.GetStreamer()
	if !streamer.Available() {
		a.rerender()
		return
	}
	q := a.player.Queue()
	if len(q) == 0 {
		a.rerender()
		return
	}
	current := a.player.CurrentItem()
	if current == nil {
		a.rerender()
		return
	}

	a.mu.Lock()
	active := a.streamingActive
	url := a.icecastURL
	a.mu.Unlock()

	if !active {
		elapsed, _ := a.player.PlaybackTimes()
		if err := streamer.Start(current.URI, url, icecast.StartOptions{StartSeconds: elapsed}); err == nil {
			a.mu.Lock()
			a.streamingActive = true
			a.streamStop = make(chan struct{})
			stop := a.streamStop
			a.mu.Unlock()
			go a.streamMonitor(stop)
		}
	} else {
		streamer.Stop()
		a.mu.Lock()
		if a.streamStop != nil {
			close(a.streamStop)
			a.streamStop = nil
		}
		a.streamingActive = false
		a.mu.Unlock()
	}
	a.rerender()
}

// streamMonitor mirrors _stream_monitor: restarts the ffmpeg encode when the
// playing track changes, and recovers if ffmpeg dies unexpectedly.
func (a *App) streamMonitor(stop chan struct{}) {
	streamer := icecast.GetStreamer()
	lastIndex := a.player.CurrentIndex()
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
		}

		a.mu.Lock()
		url := a.icecastURL
		a.mu.Unlock()

		curIdx := a.player.CurrentIndex()
		if curIdx != lastIndex {
			streamer.Stop()
			if ci := a.player.CurrentItem(); ci != nil {
				elapsed, _ := a.player.PlaybackTimes()
				_ = streamer.Start(ci.URI, url, icecast.StartOptions{StartSeconds: elapsed})
			}
			lastIndex = curIdx
		} else if !streamer.IsStreaming() {
			if ci := a.player.CurrentItem(); ci != nil {
				elapsed, _ := a.player.PlaybackTimes()
				_ = streamer.Start(ci.URI, url, icecast.StartOptions{StartSeconds: elapsed})
			}
		}
	}
}

func (a *App) setStreamServer() {
	a.mu.Lock()
	a.inputMode = true
	a.inputBuffer = a.icecastURL
	a.mu.Unlock()
	a.rerender()
}

func (a *App) space() {
	a.player.TogglePause()

	a.mu.Lock()
	active := a.streamingActive
	url := a.icecastURL
	a.mu.Unlock()

	if active {
		streamer := icecast.GetStreamer()
		if !a.player.IsPlaying() {
			if streamer.IsStreaming() {
				streamer.Stop()
			}
		} else if !streamer.IsStreaming() {
			if ci := a.player.CurrentItem(); ci != nil {
				elapsed, _ := a.player.PlaybackTimes()
				_ = streamer.Start(ci.URI, url, icecast.StartOptions{StartSeconds: elapsed})
			}
		}
	}
	a.rerender()
}

func (a *App) next() {
	a.player.Next()
	a.rerender()
}

func (a *App) previous() {
	a.player.Previous()
	a.rerender()
}

func (a *App) writePlaylist() {
	a.showInputDialog("Write playlist", "Playlist name (no extension):", "", func(name string, ok bool) {
		if !ok || strings.TrimSpace(name) == "" {
			a.rerender()
			return
		}
		path := filepath.Join(a.cfg.PlaylistsDir, name+".m3u")
		q := a.player.Queue()
		entries := make([]playlists.Entry, 0, len(q))
		for _, it := range q {
			entries = append(entries, playlists.Entry{URI: it.URI, Display: it.Display})
		}
		_ = playlists.WriteM3U(path, entries)
		a.rerender()
	})
}

func (a *App) loadPlaylist() {
	a.showInputDialog("Load playlist", "Playlist filename (e.g. my.m3u):", "", func(name string, ok bool) {
		if !ok || strings.TrimSpace(name) == "" {
			a.rerender()
			return
		}
		path := filepath.Join(a.cfg.PlaylistsDir, name)
		uris := playlists.LoadM3U(path)
		if len(uris) == 0 {
			a.rerender()
			return
		}
		a.player.Clear()
		a.mu.Lock()
		a.queueSelectedIndex = 0
		a.mu.Unlock()
		for _, u := range uris {
			display := filepath.Base(u)
			if strings.Contains(u, "://") {
				display = "Stream: " + u
			}
			a.player.AddURL(u, display)
		}
		a.player.Play()
		a.rerender()
	})
}

func (a *App) quit() {
	a.saveCurrentSession()
	a.player.Stop()
	a.tv.Stop()
}

func (a *App) showHelp() {
	helpText := "Keyboard commands:\n" +
		"  h        Show this help\n" +
		"  Space    Play/Pause\n" +
		"  Enter    Open directory / Play file / Activate selected queue item\n" +
		"  Backspace Up one directory\n" +
		"  b / B    Back / Forward navigation through visited directories\n" +
		"  Up/Down  Move selection\n" +
		"  PageUp/PageDown  Move selection by page\n" +
		"  Tab / Shift-Tab  Switch focus between Library and Queue\n" +
		"  a        Add selected file to queue\n" +
		"  A        Add selected directory (recursive) to queue\n" +
		"  d/Delete Delete selected song from queue\n" +
		"  D        Delete playlist file\n" +
		"  n / p    Next / Previous track\n" +
		"  w        Write playlist to file\n" +
		"  l        Load playlist from file\n" +
		"  t        Toggle Icecast streaming\n" +
		"  T        Set Icecast server URL (inline)\n" +
		"  q        Quit\n"
	a.showMessageDialog("Help — Keyboard Commands", helpText)
}
