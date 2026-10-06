// Package tui implements the terminal UI for musipal, using tview/tcell.
package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rivo/tview"

	"musipal/internal/config"
	"musipal/internal/library"
	"musipal/internal/player"
	"musipal/internal/session"
)

type durCacheEntry struct {
	mtime    int64
	duration *float64
}

type bitrateCacheEntry struct {
	mtime int64
	kbps  *int
}

// App holds all mutable state for the running TUI.
type App struct {
	cfg    config.Config
	root   string
	player *player.Player

	tv    *tview.Application
	pages *tview.Pages

	libraryView *tview.TextView
	queueView   *tview.TextView
	statusView  *tview.TextView

	mu sync.Mutex // guards the fields below (also touched from background goroutines)

	cwd                string
	libraryItems       []library.Item
	selectedIndex      int
	queueSelectedIndex int
	focusedPane        string // "library" | "queue"

	durationsCache map[string]durCacheEntry
	bitrateCache   map[string]bitrateCacheEntry

	backStack    []string
	forwardStack []string

	icecastURL      string
	streamingActive bool
	streamStop      chan struct{}

	inputMode   bool
	inputBuffer string

	savedSession *session.State
	sessionSaved bool
	tickerStop   chan struct{}
}

// New constructs an App, loading config and the saved session.
func New(libraryRootOverride string) (*App, error) {
	cfg, err := config.Load(libraryRootOverride)
	if err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}
	if err := os.MkdirAll(cfg.LibraryRoot, 0o755); err != nil {
		return nil, fmt.Errorf("creating library root: %w", err)
	}

	a := &App{
		cfg:            cfg,
		root:           cfg.LibraryRoot,
		player:         player.New(),
		cwd:            cfg.LibraryRoot,
		focusedPane:    "library",
		durationsCache: map[string]durCacheEntry{},
		bitrateCache:   map[string]bitrateCacheEntry{},
		icecastURL:     "http://localhost:8000/stream",
		tickerStop:     make(chan struct{}),
	}

	a.savedSession = session.Load()
	if a.savedSession != nil {
		a.player.Clear()
		for _, qi := range a.savedSession.Queue {
			a.player.Add(qi)
		}
		a.queueSelectedIndex = a.savedSession.QueueSelectedIndex
	}

	a.refreshDir()
	return a, nil
}

// refreshDir reloads state.libraryItems for state.cwd and updates the
// duration cache for newly visible files.
func (a *App) refreshDir() {
	a.mu.Lock()
	defer a.mu.Unlock()

	items := library.ListDir(a.cwd)
	a.libraryItems = items
	if a.selectedIndex > len(items)-1 {
		a.selectedIndex = len(items) - 1
	}
	if a.selectedIndex < 0 {
		a.selectedIndex = 0
	}

	for _, it := range items {
		if it.IsDir {
			continue
		}
		a.cacheDurationLocked(it.Path)
	}
}

func (a *App) selectedPath() (string, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.libraryItems) == 0 {
		return "", false
	}
	if a.selectedIndex < 0 || a.selectedIndex >= len(a.libraryItems) {
		return "", false
	}
	return a.libraryItems[a.selectedIndex].Path, true
}

func buildLayout(a *App) tview.Primitive {
	a.libraryView = tview.NewTextView().
		SetDynamicColors(true).
		SetRegions(true).
		SetWrap(false)
	a.libraryView.SetBorder(true).SetTitle(" Library ")

	a.queueView = tview.NewTextView().
		SetDynamicColors(true).
		SetRegions(true).
		SetWrap(false)
	a.queueView.SetBorder(true).SetTitle(" Queue ")

	a.statusView = tview.NewTextView().
		SetDynamicColors(true).
		SetWrap(true)
	a.statusView.SetBorder(true).SetTitle(" Status ")

	panes := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(a.libraryView, 0, 1, true).
		AddItem(a.queueView, 0, 1, false)

	root := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(panes, 0, 1, true).
		AddItem(a.statusView, 7, 0, false)

	return root
}

// Run starts the TUI event loop and blocks until the user quits.
func (a *App) Run() error {
	a.tv = tview.NewApplication()
	root := buildLayout(a)
	a.pages = tview.NewPages().AddPage("main", root, true, true)

	a.tv.SetInputCapture(a.handleKey)
	a.tv.SetRoot(a.pages, true).SetFocus(a.libraryView)

	go a.tickerLoop()

	if a.savedSession != nil && len(a.savedSession.Queue) > 0 {
		go a.resumeSavedSession()
	}

	a.rerender()

	err := a.tv.Run()

	close(a.tickerStop)
	a.mu.Lock()
	saved := a.sessionSaved
	a.mu.Unlock()
	if !saved {
		a.saveCurrentSession()
	}
	a.player.Stop()

	return err
}

func (a *App) resumeSavedSession() {
	ss := a.savedSession
	a.player.PlayIndex(ss.Index)
	time.Sleep(250 * time.Millisecond)
	if ss.Elapsed != nil {
		a.player.SeekSeconds(*ss.Elapsed)
	}
	if !ss.WasPlaying {
		time.Sleep(50 * time.Millisecond)
		if a.player.IsPlaying() {
			a.player.Pause()
		}
	}
	a.tv.QueueUpdateDraw(func() {
		a.rerender()
	})
}

func (a *App) tickerLoop() {
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-a.tickerStop:
			return
		case <-ticker.C:
			a.player.SyncCurrentIndex()
			a.tv.QueueUpdateDraw(func() {
				a.renderQueue()
				a.renderStatus()
			})
		}
	}
}

func (a *App) saveCurrentSession() {
	idx := a.player.CurrentIndex()
	if idx < 0 {
		idx = 0
	}
	elapsed, _ := a.player.PlaybackTimes()
	a.mu.Lock()
	qsi := a.queueSelectedIndex
	a.mu.Unlock()

	ss := session.State{
		Queue:              a.player.Queue(),
		Index:              idx,
		Elapsed:            elapsed,
		WasPlaying:         a.player.IsPlaying(),
		QueueSelectedIndex: qsi,
	}
	session.Save(ss)

	a.mu.Lock()
	a.sessionSaved = true
	a.mu.Unlock()
}

func (a *App) cacheDurationLocked(p string) *float64 {
	info, err := os.Stat(p)
	var mtime int64
	if err == nil {
		mtime = info.ModTime().UnixNano()
	}
	if cached, ok := a.durationsCache[p]; ok && cached.mtime == mtime {
		return cached.duration
	}

	var duration *float64
	if strings.ToLower(filepath.Ext(p)) == ".cue" {
		if src, ok := cueSourceFile(p); ok {
			if _, err := os.Stat(src); err == nil {
				duration = durationSeconds(src)
			}
		}
	} else {
		duration = durationSeconds(p)
	}
	a.durationsCache[p] = durCacheEntry{mtime: mtime, duration: duration}
	return duration
}
