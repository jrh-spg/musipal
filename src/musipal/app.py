from __future__ import annotations

from dataclasses import dataclass
from pathlib import Path
import re
import asyncio
from urllib.parse import urlparse

from halo import Halo
from prompt_toolkit.application import Application
from prompt_toolkit.data_structures import Point
from prompt_toolkit.formatted_text import ANSI
from prompt_toolkit.key_binding import KeyBindings
from prompt_toolkit.layout import HSplit, Layout, VSplit
from prompt_toolkit.layout.controls import FormattedTextControl
from prompt_toolkit.layout.dimension import D
from prompt_toolkit.layout.containers import Window
from prompt_toolkit.widgets import Frame

from musipal.colors import c, status_err, status_ok, status_warn
from musipal.config import load_config
from musipal.cue import cue_source_file, parse_cue
from musipal.durations import format_duration, get_bitrate_kbps, get_duration_seconds
from musipal.library import iter_audio_files_recursive, list_dir
from musipal.player import Player, QueueItem
from musipal.playlists import PlaylistEntry, load_m3u, write_m3u
from musipal.session import SessionState, load_session, save_session


@dataclass
class State:
    cwd: Path
    library_items: list[Path]
    selected_index: int
    queue_selected_index: int
    focused_pane: str  # 'library' | 'queue'
    status: str
    durations_cache: dict[Path, tuple[int, float | None]]


def run_app(*, library_root_override: str | None = None) -> None:
    cfg = load_config(library_root_override)

    with Halo(text="Loading library...", spinner="dots") as spinner:
        root = cfg.library_root
        root.mkdir(parents=True, exist_ok=True)
        spinner.succeed(status_ok(f"Library root: {root}"))

    player = Player()

    saved_session = load_session()

    state = State(
        cwd=root,
        library_items=[],
        selected_index=0,
        queue_selected_index=0,
        focused_pane="library",
        status="",
        durations_cache={},
    )

    if saved_session is not None:
        player.clear()
        for qi in saved_session.queue:
            player.add(qi)
        state.queue_selected_index = saved_session.queue_selected_index

    _ansi_re = re.compile(r"\x1b\[[0-9;]*m")

    def _strip_ansi(s: str) -> str:
        return _ansi_re.sub("", s)

    bitrate_cache: dict[Path, tuple[int, int | None]] = {}

    def _uri_to_path(uri: str) -> Path | None:
        try:
            u = urlparse(uri)
            if u.scheme == "file":
                return Path(u.path)
        except Exception:
            pass
        if "://" in uri:
            return None
        try:
            return Path(uri)
        except Exception:
            return None

    def _current_bitrate_kbps() -> int | None:
        item = player.current_item()
        if item is not None:
            p = _uri_to_path(item.uri)
            if p is not None and p.exists():
                try:
                    mtime = int(p.stat().st_mtime_ns)
                except Exception:
                    mtime = 0

                cached = bitrate_cache.get(p)
                if cached is not None and cached[0] == mtime:
                    return cached[1]

                kbps = get_bitrate_kbps(p)
                bitrate_cache[p] = (mtime, kbps)
                return kbps

        # Streams/unknown sources: try VLC stats.
        return player.bitrate_kbps()

    def _friendly_filename() -> str:
        """Best-effort filename/identifier for what's currently playing."""
        item = player.current_item()
        if item is not None:
            p = _uri_to_path(item.uri)
            if p is not None:
                return p.name

            # If it's a URL, prefer the last path segment.
            try:
                u = urlparse(item.uri)
                if u.scheme and u.path:
                    tail = Path(u.path).name
                    if tail:
                        return tail
            except Exception:
                pass

            # Fallback to display.
            if item.display:
                return item.display

        # Last resort: use VLC's MRL.
        mrl = player.now_playing()
        p = _uri_to_path(mrl)
        if p is not None:
            return p.name
        try:
            u = urlparse(mrl)
            if u.scheme and u.path:
                tail = Path(u.path).name
                if tail:
                    return tail
        except Exception:
            pass
        return ""

    def _duration_text(p: Path) -> str:
        if p.is_dir():
            return ""
        seconds = state.durations_cache.get(p, (0, None))[1]
        return c(f"[{format_duration(seconds)}]", "white")

    def _render_name(p: Path, name: str) -> str:
        if p.is_dir():
            return c(name, "cyan", bold=True)
        suf = p.suffix.lower()
        if suf == ".cue":
            return c(name, "magenta", bold=True)
        if suf in {".flac"}:
            return c(name, "green")
        if suf in {".mp3"}:
            return c(name, "yellow")
        if suf in {".ogg"}:
            return c(name, "blue")
        return name

    def _truncate_plain(text: str, max_len: int) -> str:
        if max_len <= 0:
            return ""
        if len(text) <= max_len:
            return text
        if max_len <= 3:
            return text[:max_len]
        return text[: max_len - 3] + "..."

    def refresh_dir() -> None:
        items = list_dir(state.cwd)
        state.library_items = [it.path for it in items]
        state.selected_index = min(state.selected_index, max(0, len(state.library_items) - 1))

        # Cache durations for visible files (avoid re-reading unchanged files).
        for p in state.library_items:
            if p.is_dir():
                continue
            try:
                mtime = int(p.stat().st_mtime_ns)
            except Exception:
                mtime = 0

            cached = state.durations_cache.get(p)
            if cached is not None and cached[0] == mtime:
                continue

            duration: float | None = None
            if p.suffix.lower() == ".cue":
                src = cue_source_file(p)
                if src is not None and src.exists():
                    duration = get_duration_seconds(src)
            else:
                duration = get_duration_seconds(p)

            state.durations_cache[p] = (mtime, duration)

    refresh_dir()

    # We include a 2-line header above the list. The cursor position is used by
    # prompt_toolkit to keep the selection visible (auto-scrolling).
    library_header_lines = 2
    queue_header_lines = 2

    def _library_cursor_position() -> Point:
        row = library_header_lines
        if state.library_items:
            row += max(0, min(state.selected_index, len(state.library_items) - 1))
        return Point(x=0, y=row)

    library_control = FormattedTextControl(
        text="",
        focusable=True,
        show_cursor=False,
        get_cursor_position=_library_cursor_position,
    )

    def _queue_cursor_position() -> Point:
        row = queue_header_lines
        q = player.queue
        if q:
            row += max(0, min(state.queue_selected_index, len(q) - 1))
        return Point(x=0, y=row)

    queue_control = FormattedTextControl(
        text="",
        focusable=True,
        show_cursor=False,
        get_cursor_position=_queue_cursor_position,
    )
    status_control = FormattedTextControl(text="")

    def set_status(msg: str) -> None:
        state.status = msg
        status_control.text = ANSI(msg)

    def render_library() -> None:
        lines: list[str] = []
        header = c("Library", "white", bold=True) + "  " + c(str(state.cwd), "white")
        lines.append(header)
        # Keep the divider wide enough, but the window will crop as needed.
        lines.append(c("─" * 200, "white"))
        if not state.library_items:
            lines.append(status_warn("(empty)"))

        try:
            window_width = int(library_window.render_info.window_width) if library_window.render_info else 80
        except Exception:
            window_width = 80

        for i, p in enumerate(state.library_items):
            prefix = "> " if i == state.selected_index else "  "
            dur = _duration_text(p)
            dur_plain_len = len(_strip_ansi(dur))

            name_plain = p.name + ("/" if p.is_dir() else "")
            if dur:
                # Leave at least one space before duration.
                available = window_width - len(prefix) - dur_plain_len - 1
                name_plain = _truncate_plain(name_plain, max(1, available))

            name_rendered = _render_name(p, name_plain)
            prefix_rendered = c(prefix, "white", bold=True) if i == state.selected_index else prefix

            base_plain_len = len(_strip_ansi(prefix_rendered + name_rendered))
            if dur:
                spaces = max(1, window_width - base_plain_len - dur_plain_len)
                lines.append(prefix_rendered + name_rendered + (" " * spaces) + dur)
            else:
                lines.append(prefix_rendered + name_rendered)
        library_control.text = ANSI("\n".join(lines))

    def render_queue() -> None:
        lines: list[str] = []
        lines.append(c("Queue / Playlist", "white", bold=True))
        lines.append(c("─" * 40, "white"))
        q = player.queue
        if not q:
            lines.append(status_warn("(queue empty)"))

        def _queue_label(idx: int, item: QueueItem) -> str:
            # Per request: highlight the currently playing item in cyan.
            cur = player.current_index()
            if cur is not None and idx == cur:
                return c(item.display, "cyan", bold=True)
            # Otherwise: consistent light blue for queue track names.
            return c(item.display, "light_blue")

        try:
            queue_width = int(queue_window.render_info.window_width) if queue_window.render_info else 50
        except Exception:
            queue_width = 50

        # Clamp selection into range.
        if q:
            state.queue_selected_index = max(0, min(state.queue_selected_index, len(q) - 1))
        else:
            state.queue_selected_index = 0

        for i, item in enumerate(q[:500]):
            label = _queue_label(i, item)

            times = ""
            if item.start is not None:
                times = f"[{format_duration(item.start)}]"
            if item.stop is not None:
                times = (times + " " if times else "") + f"→ {format_duration(item.stop)}"

            odd = ((i + 1) % 2 == 1)
            prefix = "> " if i == state.queue_selected_index else "  "
            prefix_color = "white"
            num_color = "light_gray" if odd else "white"
            times_color = "light_gray" if odd else "white"
            prefix_rendered = c(prefix, prefix_color, bold=True) if i == state.queue_selected_index else prefix
            num_rendered = c(f"{i+1:>3}.", num_color)
            left = prefix_rendered + num_rendered + " " + label
            if times:
                times_rendered = c(times, times_color)
                times_len = len(_strip_ansi(times_rendered))

                # Leave at least one space between label and times.
                available = queue_width - times_len - 1
                left_plain = _strip_ansi(left)
                if len(left_plain) > available:
                    # Truncate only the display text portion.
                    plain_prefix = prefix + f"{i+1:>3}. "
                    prefix_len = len(plain_prefix)
                    label_plain = _strip_ansi(label)
                    trunc = _truncate_plain(label_plain, max(1, available - prefix_len))
                    # Recompute num_rendered for truncation branch
                    num_rendered = c(f"{i+1:>3}.", num_color)
                    left = prefix_rendered + num_rendered + " " + _queue_label(i, QueueItem(uri=item.uri, display=trunc, start=item.start, stop=item.stop))
                    left_plain = _strip_ansi(left)

                spaces = max(1, queue_width - len(left_plain) - times_len)
                lines.append(left + (" " * spaces) + times_rendered)
            else:
                lines.append(left)
        queue_control.text = ANSI("\n".join(lines))

    def _playback_bar(elapsed: float | None, total: float | None) -> str:
        # Use the status window width if available.
        try:
            w = int(status_window.render_info.window_width) if status_window.render_info else 80
        except Exception:
            w = 80

        # Keep it simple and predictable.
        bar_width = max(10, min(40, w - 25))

        if elapsed is None:
            elapsed_s = "--:--"
        else:
            elapsed_s = format_duration(elapsed)
        if total is None:
            total_s = "--:--"
        else:
            total_s = format_duration(total)

        if total is None or total <= 0 or elapsed is None or elapsed < 0:
            fill = 0
        else:
            fill = int(max(0.0, min(1.0, elapsed / total)) * bar_width)

        filled = c("=" * fill, "green")
        empty = c("-" * (bar_width - fill), "white")
        return f"[{filled}{empty}] {c(elapsed_s, 'white')} / {c(total_s, 'white')}"

    def render_status() -> None:
        is_playing = player.is_playing()
        now = player.now_playing()
        elapsed, total = player.playback_times()
        bar = _playback_bar(elapsed, total)

        # Deduce the CUE track title from playback time.
        current_item = player.current_item()
        cue_title = ""
        if current_item is not None:
            # If this queue item is itself a cue segment, prefer its display.
            if current_item.start is not None or current_item.stop is not None:
                cue_title = current_item.display
            else:
                # Otherwise, if the current media matches cue segments in the queue,
                # pick the segment that contains the current playback position.
                abs_seconds: float | None = None
                if elapsed is not None:
                    abs_seconds = elapsed

                # Try to match by local file path (handles file:// MRLs).
                def _mrl_to_path(mrl: str) -> Path | None:
                    try:
                        u = urlparse(mrl)
                        if u.scheme == "file":
                            return Path(u.path)
                    except Exception:
                        return None
                    return None

                def _uri_to_path(uri: str) -> Path | None:
                    if "://" in uri:
                        return None
                    try:
                        return Path(uri)
                    except Exception:
                        return None

                current_path = _mrl_to_path(now)
                if current_path is None:
                    current_path = _uri_to_path(current_item.uri)

                if abs_seconds is not None and current_path is not None:
                    try:
                        current_path = current_path.resolve()
                    except Exception:
                        pass

                    # Find best matching cue segment.
                    for qi in player.queue:
                        if qi.start is None:
                            continue
                        qpath = _uri_to_path(qi.uri)
                        if qpath is None:
                            continue
                        try:
                            qpath = qpath.resolve()
                        except Exception:
                            pass
                        if qpath != current_path:
                            continue
                        if abs_seconds < qi.start:
                            continue
                        if qi.stop is not None and abs_seconds >= qi.stop:
                            continue
                        cue_title = qi.display
                        break
        msg = (
            f"{c('Space', 'white', bold=True)} play/pause  {c('a', 'white', bold=True)} add  "
            f"{c('n/p', 'white', bold=True)} next/prev  {c('q', 'white', bold=True)} quit"
        )

        current_item = player.current_item()
        if current_item is not None and current_item.display:
            title = current_item.display
        else:
            title = now

        kbps = _current_bitrate_kbps()
        bitrate_s = f"{kbps} kbps" if kbps is not None else "-- kbps"
        fname = _friendly_filename()

        state_word = status_ok("Playing") if is_playing else status_warn("Paused")
        if title:
            msg += "\n" + state_word + " " + c(title, "white") + "  " + c(bitrate_s, "white")
        else:
            msg += "\n" + state_word + "  " + c(bitrate_s, "white")

        if fname:
            msg += "\n" + c("File:", "white", bold=True) + " " + c(fname, "white")

        msg += "\n" + bar
        status_control.text = ANSI(msg)

    def rerender() -> None:
        # Update pane titles to reflect focus.
        try:
            library_frame.title = ANSI(
                c("Library", "light_magenta", bold=True) if state.focused_pane == "library" else c("Library", "white")
            )
            queue_frame.title = ANSI(
                c("Queue", "light_magenta", bold=True) if state.focused_pane == "queue" else c("Queue", "white")
            )
        except Exception:
            pass
        render_library()
        render_queue()
        render_status()

    def selected_path() -> Path | None:
        if not state.library_items:
            return None
        if state.selected_index < 0 or state.selected_index >= len(state.library_items):
            return None
        return state.library_items[state.selected_index]

    kb = KeyBindings()

    session_saved = False

    def _save_current_session() -> None:
        nonlocal session_saved
        try:
            idx = player.current_index() if player.current_index() is not None else 0
            elapsed, _total = player.playback_times()
            ss = SessionState(
                queue=player.queue,
                index=int(idx),
                elapsed=elapsed,
                was_playing=bool(player.is_playing()),
                queue_selected_index=int(state.queue_selected_index),
            )
            save_session(ss)
            session_saved = True
        except Exception:
            return

    def _focus_library(event) -> None:
        state.focused_pane = "library"
        event.app.layout.focus(library_control)

    def _focus_queue(event) -> None:
        state.focused_pane = "queue"
        event.app.layout.focus(queue_control)

    @kb.add("tab")
    def _tab(event) -> None:
        if state.focused_pane == "library":
            _focus_queue(event)
        else:
            _focus_library(event)
        rerender()

    @kb.add("s-tab")
    def _shift_tab(event) -> None:
        if state.focused_pane == "queue":
            _focus_library(event)
        else:
            _focus_queue(event)
        rerender()

    def _page_step() -> int:
        # How many list rows to move for PageUp/PageDown.
        # Depends on current render height; fallback to a reasonable default.
        try:
            if library_window.render_info is None:
                return 10
            h = int(library_window.render_info.window_height)
            usable = max(1, h - library_header_lines)
            return max(1, usable - 1)
        except Exception:
            return 10

    def _queue_page_step() -> int:
        try:
            if queue_window.render_info is None:
                return 10
            h = int(queue_window.render_info.window_height)
            usable = max(1, h - queue_header_lines)
            return max(1, usable - 1)
        except Exception:
            # Always define these before any use
            odd = ((i + 1) % 2 == 1)
            prefix = "> " if i == state.queue_selected_index else "  "
            prefix_color = "white"
            num_color = "light_gray" if odd else "white"
            times_color = "light_gray" if odd else "white"
            prefix_rendered = c(prefix, prefix_color, bold=True) if i == state.queue_selected_index else prefix
            num_rendered = c(f"{i+1:>3}.", num_color)
            num_rendered = c(f"{i+1:>3}.", num_color)
            if q:
                state.queue_selected_index = max(0, state.queue_selected_index - 1)
        else:
            state.selected_index = max(0, state.selected_index - 1)
        rerender()

    @kb.add("down")
    def _down(event) -> None:
        if state.focused_pane == "queue":
            q = player.queue
            if q:
                state.queue_selected_index = min(len(q) - 1, state.queue_selected_index + 1)
        else:
            state.selected_index = min(len(state.library_items) - 1, state.selected_index + 1)
        rerender()

    @kb.add("pageup")
    def _page_up(event) -> None:
        if state.focused_pane == "queue":
            q = player.queue
            if q:
                state.queue_selected_index = max(0, state.queue_selected_index - _queue_page_step())
        else:
            state.selected_index = max(0, state.selected_index - _page_step())
        rerender()

    @kb.add("pagedown")
    def _page_down(event) -> None:
        if state.focused_pane == "queue":
            q = player.queue
            if q:
                state.queue_selected_index = min(len(q) - 1, state.queue_selected_index + _queue_page_step())
        else:
            state.selected_index = min(len(state.library_items) - 1, state.selected_index + _page_step())
        rerender()

    @kb.add("enter")
    def _enter(event) -> None:
        if state.focused_pane == "queue":
            q = player.queue
            if not q:
                return
            idx = max(0, min(state.queue_selected_index, len(q) - 1))
            player.play_index(idx)
            set_status(status_ok(f"Playing {q[idx].display}"))
            rerender()
            return

        p = selected_path()
        if not p:
            return
        if p.is_dir():
            state.cwd = p
            refresh_dir()
            set_status(status_ok(f"Opened {p}"))
            rerender()
            return
        # play file (and clear previous)
        player.clear()
        _enqueue_path(p)
        player.play()
        set_status(status_ok(f"Playing {p.name}"))
        rerender()

    @kb.add("backspace")
    def _back(event) -> None:
        if state.cwd == root:
            return
        state.cwd = state.cwd.parent
        refresh_dir()
        set_status(status_ok(f"Up to {state.cwd}"))
        rerender()

    def _enqueue_path(p: Path) -> None:
        if p.suffix.lower() == ".cue":
            try:
                tracks = parse_cue(p)
                if not tracks:
                    set_status(status_err("CUE has no tracks"))
                    return
                for t in tracks:
                    player.add(
                        QueueItem(
                            uri=str(t.source_file),
                            display=f"{t.performer} - {t.title}".strip(" -"),
                            start=t.start_seconds,
                            stop=t.end_seconds,
                        )
                    )
                set_status(status_ok(f"Added {len(tracks)} CUE tracks"))
            except Exception as e:
                set_status(status_err(f"Failed to parse CUE: {e}"))
            return
        player.add_path(p)
        set_status(status_ok(f"Added {p.name}"))

    @kb.add("a")
    def _add(event) -> None:
        p = selected_path()
        if not p or p.is_dir():
            return
        _enqueue_path(p)
        rerender()

    @kb.add("A")
    def _add_dir_recursive(event) -> None:
        p = selected_path()
        if not p:
            return
        target = p if p.is_dir() else p.parent
        with Halo(text="Adding directory...", spinner="dots") as spinner:
            files = iter_audio_files_recursive(target)
            for f in files:
                _enqueue_path(f)
            spinner.succeed(status_ok(f"Added {len(files)} items"))
        rerender()

    @kb.add(" ")
    def _space(event) -> None:
        player.toggle_pause()
        rerender()

    @kb.add("n")
    def _next(event) -> None:
        player.next()
        rerender()

    @kb.add("p")
    def _prev(event) -> None:
        player.previous()
        rerender()

    @kb.add("s")
    def _stream(event) -> None:
        from prompt_toolkit.shortcuts import input_dialog

        url = input_dialog(title="Icecast/Stream", text="Enter stream URL:").run()
        if not url:
            return
        player.add_url(url, display=f"Stream: {url}")
        set_status(status_ok("Added stream URL"))
        rerender()

    @kb.add("w")
    def _write_playlist(event) -> None:
        from prompt_toolkit.shortcuts import input_dialog

        name = input_dialog(title="Write playlist", text="Playlist name (no extension):").run()
        if not name:
            return
        path = (cfg.playlists_dir / f"{name}.m3u").resolve()
        entries = [PlaylistEntry(uri=i.uri, display=i.display) for i in player.queue]
        write_m3u(path, entries)
        set_status(status_ok(f"Wrote {path}"))
        rerender()

    @kb.add("l")
    def _load_playlist(event) -> None:
        from prompt_toolkit.shortcuts import input_dialog

        name = input_dialog(title="Load playlist", text="Playlist filename (e.g. my.m3u):").run()
        if not name:
            return
        path = (cfg.playlists_dir / name).resolve()
        uris = load_m3u(path)
        if not uris:
            set_status(status_warn("Playlist empty or not found"))
            rerender()
            return
        player.clear()
        state.queue_selected_index = 0
        for u in uris:
            player.add_url(u, display=Path(u).name if "://" not in u else f"Stream: {u}")
        player.play()
        set_status(status_ok(f"Loaded {len(uris)} items"))
        rerender()

    @kb.add("q")
    def _quit(event) -> None:
        _save_current_session()
        player.stop()
        event.app.exit()

    library_window = Window(content=library_control, wrap_lines=False)
    queue_window = Window(content=queue_control, wrap_lines=False)
    status_window = Window(content=status_control, height=5, wrap_lines=True)

    library_frame = Frame(library_window, title=ANSI(c("Library", "light_magenta", bold=True)), width=D(weight=1))
    queue_frame = Frame(queue_window, title=ANSI(c("Queue", "white")), width=D(weight=1))
    status_frame = Frame(status_window, title="Status")

    root_container = HSplit([
        VSplit([
            library_frame,
            queue_frame,
        ]),
        status_frame,
    ])

    app = Application(layout=Layout(root_container, focused_element=library_control), key_bindings=kb, full_screen=True)

    async def _ticker() -> None:
        # Periodically update the status bar so playback position advances.
        while True:
            # Keep the current track and highlight in sync as playback advances.
            try:
                player.sync_current_index()
            except Exception:
                pass

            render_queue()
            render_status()
            app.invalidate()
            await asyncio.sleep(0.25)

    def _pre_run() -> None:
        # At this point, prompt_toolkit has an active asyncio loop.
        app.create_background_task(_ticker())

        if saved_session is not None and saved_session.queue:
            async def _resume() -> None:
                # Start playback at the saved index, then seek once media is ready.
                try:
                    player.play_index(saved_session.index)
                    await asyncio.sleep(0.25)
                    if saved_session.elapsed is not None:
                        player.seek_seconds(saved_session.elapsed)
                    if not saved_session.was_playing:
                        # Ensure paused state after restoring position.
                        await asyncio.sleep(0.05)
                        if player.is_playing():
                            player.pause()
                    rerender()
                    app.invalidate()
                except Exception:
                    return

            app.create_background_task(_resume())

    rerender()
    try:
        app.run(pre_run=_pre_run)
    finally:
        if not session_saved:
            _save_current_session()
