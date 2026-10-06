package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/rivo/tview"
	"golang.org/x/term"

	"musipal-go/internal/colors"
	"musipal-go/internal/durations"
	"musipal-go/internal/metadata"
)

func termWidth() int {
	w, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || w <= 0 {
		return 100
	}
	return w
}

// rerender updates all three panes and refreshes pane title colors to
// reflect which one is focused.
func (a *App) rerender() {
	a.mu.Lock()
	focused := a.focusedPane
	a.mu.Unlock()

	if focused == "library" {
		a.libraryView.SetTitle(" [::b]Library[-:-:-] ")
		a.queueView.SetTitle(" Queue ")
	} else {
		a.libraryView.SetTitle(" Library ")
		a.queueView.SetTitle(" [::b]Queue[-:-:-] ")
	}

	a.renderLibrary()
	a.renderQueue()
	a.renderStatus()
}

func extColor(name string, ext string) string {
	switch ext {
	case ".flac":
		return colors.C(name, "green", false)
	case ".mp3":
		return colors.C(name, "yellow", false)
	case ".ogg":
		return colors.C(name, "blue", false)
	case ".cue":
		return colors.C(name, "magenta", true)
	default:
		return name
	}
}

func (a *App) renderLibrary() {
	a.mu.Lock()
	cwd := a.cwd
	items := a.libraryItems
	selected := a.selectedIndex
	a.mu.Unlock()

	var b strings.Builder
	b.WriteString(colors.C(tview.Escape("Library"), "white", true))
	b.WriteString("  ")
	b.WriteString(colors.C(tview.Escape(cwd), "white", false))
	b.WriteString("\n")
	b.WriteString(colors.C(strings.Repeat("─", 200), "white", false))
	b.WriteString("\n")

	if len(items) == 0 {
		b.WriteString(colors.StatusWarn("(empty)"))
	}

	for i, it := range items {
		prefix := "  "
		if i == selected {
			prefix = "> "
		}

		var name string
		if it.IsDir {
			name = filepath.Base(it.Path) + "/"
		} else {
			name = metadata.PrettyName(it.Path)
		}
		nameEsc := tview.Escape(name)

		var nameColored string
		if it.IsDir {
			nameColored = colors.C(nameEsc, "cyan", true)
		} else {
			nameColored = extColor(nameEsc, strings.ToLower(filepath.Ext(it.Path)))
		}

		prefixColored := prefix
		if i == selected {
			prefixColored = colors.C(prefix, "white", true)
		}

		line := prefixColored + nameColored

		if !it.IsDir {
			a.mu.Lock()
			dur := a.cacheDurationLocked(it.Path)
			a.mu.Unlock()
			line += "  " + colors.C(fmt.Sprintf("[%s]", durations.FormatDuration(dur)), "white", false)
		}

		fmt.Fprintf(&b, "\n[\"%d\"]%s[\"\"]", i, line)
	}

	a.libraryView.SetText(b.String())
	if len(items) > 0 {
		a.libraryView.Highlight(strconv.Itoa(selected))
		a.libraryView.ScrollToHighlight()
	}
}

func (a *App) renderQueue() {
	q := a.player.Queue()
	curIdx := a.player.CurrentIndex()

	a.mu.Lock()
	selected := a.queueSelectedIndex
	if len(q) > 0 {
		if selected > len(q)-1 {
			selected = len(q) - 1
		}
		if selected < 0 {
			selected = 0
		}
		a.queueSelectedIndex = selected
	} else {
		selected = 0
		a.queueSelectedIndex = 0
	}
	a.mu.Unlock()

	var b strings.Builder
	b.WriteString(colors.C(tview.Escape("Queue / Playlist"), "white", true))
	b.WriteString("\n")
	b.WriteString(colors.C(strings.Repeat("─", 40), "white", false))

	if len(q) == 0 {
		b.WriteString("\n")
		b.WriteString(colors.StatusWarn("(queue empty)"))
	}

	limit := len(q)
	if limit > 500 {
		limit = 500
	}
	for i := 0; i < limit; i++ {
		item := q[i]
		label := tview.Escape(item.Display)
		if curIdx == i {
			label = colors.C(label, "cyan", true)
		} else {
			label = colors.C(label, "lightblue", false)
		}

		times := ""
		if item.Start != nil {
			times = fmt.Sprintf("[%s]", durations.FormatDuration(item.Start))
		}
		if item.Stop != nil {
			if times != "" {
				times += " "
			}
			times += fmt.Sprintf("→ %s", durations.FormatDuration(item.Stop))
		}

		odd := (i+1)%2 == 1
		numColor := "white"
		timesColor := "white"
		if odd {
			numColor = "gray"
			timesColor = "gray"
		}

		prefix := "  "
		if i == selected {
			prefix = colors.C("> ", "white", true)
		}
		num := colors.C(fmt.Sprintf("%3d.", i+1), numColor, false)

		line := prefix + num + " " + label
		if times != "" {
			line += "  " + colors.C(times, timesColor, false)
		}

		fmt.Fprintf(&b, "\n[\"%d\"]%s[\"\"]", i, line)
	}

	a.queueView.SetText(b.String())
	if len(q) > 0 {
		a.queueView.Highlight(strconv.Itoa(selected))
		a.queueView.ScrollToHighlight()
	}
}

func (a *App) playbackBar(elapsed, total *float64) string {
	w := termWidth()
	barWidth := w - 25
	if barWidth > 40 {
		barWidth = 40
	}
	if barWidth < 10 {
		barWidth = 10
	}

	elapsedS := durations.FormatDuration(elapsed)
	totalS := durations.FormatDuration(total)

	fill := 0
	if total != nil && *total > 0 && elapsed != nil && *elapsed >= 0 {
		frac := *elapsed / *total
		if frac > 1 {
			frac = 1
		}
		if frac < 0 {
			frac = 0
		}
		fill = int(frac * float64(barWidth))
	}

	filled := colors.C(strings.Repeat("=", fill), "green", false)
	empty := colors.C(strings.Repeat("-", barWidth-fill), "white", false)
	return fmt.Sprintf("[%s%s] %s / %s", filled, empty, colors.C(elapsedS, "white", false), colors.C(totalS, "white", false))
}

func (a *App) currentBitrateKbps() *int {
	item := a.player.CurrentItem()
	if item != nil {
		if p, ok := uriToPath(item.URI); ok && fileExists(p) {
			info, err := os.Stat(p)
			var mtime int64
			if err == nil {
				mtime = info.ModTime().UnixNano()
			}
			a.mu.Lock()
			cached, ok2 := a.bitrateCache[p]
			a.mu.Unlock()
			if ok2 && cached.mtime == mtime {
				return cached.kbps
			}
			kbps := durations.GetBitrateKbps(p)
			a.mu.Lock()
			a.bitrateCache[p] = bitrateCacheEntry{mtime: mtime, kbps: kbps}
			a.mu.Unlock()
			return kbps
		}
	}
	return a.player.BitrateKbps()
}

func (a *App) friendlyFilename() string {
	item := a.player.CurrentItem()
	if item != nil {
		if p, ok := uriToPath(item.URI); ok {
			return baseName(p)
		}
		if item.Display != "" {
			return item.Display
		}
	}
	mrl := a.player.NowPlaying()
	if p, ok := uriToPath(mrl); ok && p != "" {
		return baseName(p)
	}
	return ""
}

func (a *App) renderStatus() {
	isPlaying := a.player.IsPlaying()
	elapsed, total := a.player.PlaybackTimes()
	bar := a.playbackBar(elapsed, total)

	currentItem := a.player.CurrentItem()
	title := a.player.NowPlaying()
	if currentItem != nil && currentItem.Display != "" {
		title = currentItem.Display
	}

	kbps := a.currentBitrateKbps()
	bitrateS := "-- kbps"
	if kbps != nil {
		bitrateS = fmt.Sprintf("%d kbps", *kbps)
	}
	fname := a.friendlyFilename()

	var b strings.Builder
	b.WriteString(colors.C("Space", "white", true) + " play/pause  " +
		colors.C("a", "white", true) + " add  " +
		colors.C("n/p", "white", true) + " next/prev  " +
		colors.C("q", "white", true) + " quit")

	stateWord := colors.StatusWarn("Paused")
	if isPlaying {
		stateWord = colors.StatusOK("Playing")
	}
	if title != "" {
		b.WriteString("\n" + stateWord + " " + colors.C(tview.Escape(title), "white", false) + "  " + colors.C(bitrateS, "white", false))
	} else {
		b.WriteString("\n" + stateWord + "  " + colors.C(bitrateS, "white", false))
	}

	if fname != "" {
		b.WriteString("\n" + colors.C("File:", "white", true) + " " + colors.C(tview.Escape(fname), "white", false))
	}

	b.WriteString("\n" + bar)

	a.mu.Lock()
	inputMode := a.inputMode
	inputBuffer := a.inputBuffer
	a.mu.Unlock()
	if inputMode {
		b.WriteString("\n" + colors.C("Icecast server URL:", "white", true) + " " + colors.C(tview.Escape(inputBuffer)+"_", "white", false))
	}

	a.statusView.SetText(b.String())
}
