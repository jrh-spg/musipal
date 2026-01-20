from __future__ import annotations

from dataclasses import dataclass
from pathlib import Path
import os
import sys
from ctypes.util import find_library

_VLC = None


def _choose_existing(paths: list[str]) -> str | None:
    for p in paths:
        if p and os.path.exists(p):
            return p
    return None


def _ensure_vlc_env() -> None:
    # In PyInstaller onefile, environment/library discovery can differ.
    # Force python-vlc to use system libvlc and plugins.
    if os.environ.get("PYTHON_VLC_LIB_PATH"):
        return

    candidates: list[str] = []
    try:
        p = find_library("vlc")
        if p:
            candidates.append(p)
    except Exception:
        pass
    candidates += [
        "/lib/x86_64-linux-gnu/libvlc.so.5",
        "/usr/lib/x86_64-linux-gnu/libvlc.so.5",
        "/usr/lib/libvlc.so.5",
        "/usr/local/lib/libvlc.so.5",
    ]

    libvlc = _choose_existing(candidates)
    if libvlc:
        os.environ["PYTHON_VLC_LIB_PATH"] = libvlc

    if not os.environ.get("PYTHON_VLC_MODULE_PATH"):
        plugin_candidates = [
            "/usr/lib/x86_64-linux-gnu/vlc/plugins",
            "/usr/lib/vlc/plugins",
            "/usr/lib64/vlc/plugins",
            "/usr/local/lib/vlc/plugins",
        ]
        plugins = _choose_existing(plugin_candidates)
        if plugins:
            os.environ["PYTHON_VLC_MODULE_PATH"] = plugins
            # libvlc also recognizes VLC_PLUGIN_PATH.
            os.environ.setdefault("VLC_PLUGIN_PATH", plugins)


def _import_vlc():
    global _VLC
    if _VLC is not None:
        return _VLC
    # Only do env forcing for frozen binaries (PyInstaller).
    if getattr(sys, "frozen", False) or getattr(sys, "_MEIPASS", None):
        _ensure_vlc_env()
    import vlc as _vlc

    _VLC = _vlc
    return _vlc


@dataclass(frozen=True)
class QueueItem:
    uri: str
    display: str
    start: float | None = None
    stop: float | None = None


class Player:
    def __init__(self) -> None:
        self._vlc = _import_vlc()

        # Pass conservative defaults.
        args: list[str] = ["--no-video", "--quiet"]
        plugins = os.environ.get("PYTHON_VLC_MODULE_PATH") or os.environ.get("VLC_PLUGIN_PATH")
        self._instance = self._vlc.Instance(args)
        if self._instance is None:
            lib = os.environ.get("PYTHON_VLC_LIB_PATH")
            raise RuntimeError(
                "Failed to initialize VLC (libvlc_new returned NULL). "
                f"PYTHON_VLC_LIB_PATH={lib!r} PYTHON_VLC_MODULE_PATH={plugins!r}. "
                "Ensure system VLC/libVLC is installed and VLC plugins are available."
            )

        self._player = self._instance.media_list_player_new()
        self._media_player = self._instance.media_player_new()
        self._player.set_media_player(self._media_player)
        self._media_list = self._instance.media_list_new([])
        self._player.set_media_list(self._media_list)
        self._queue: list[QueueItem] = []
        self._queue_mrls: list[str] = []
        self._current_index: int | None = None

        em = self._player.event_manager()
        em.event_attach(self._vlc.EventType.MediaListPlayerPlayed, self._on_played)
        em.event_attach(self._vlc.EventType.MediaListPlayerStopped, self._on_stopped)

        # Track the *actual* currently playing media. Using MediaListPlayerNextItemSet
        # is off-by-one because it refers to the upcoming item.
        pem = self._media_player.event_manager()
        pem.event_attach(self._vlc.EventType.MediaPlayerMediaChanged, self._on_media_changed)
        pem.event_attach(self._vlc.EventType.MediaPlayerPlaying, self._on_media_changed)

    @property
    def queue(self) -> list[QueueItem]:
        return list(self._queue)

    def clear(self) -> None:
        self.stop()
        self._queue.clear()
        self._queue_mrls.clear()
        self._current_index = None
        self._media_list = self._instance.media_list_new([])
        self._player.set_media_list(self._media_list)

    def add(self, item: QueueItem) -> None:
        media = self._instance.media_new(item.uri)
        # Store a stable identifier to help deduce "what's playing".
        try:
            media.set_meta(self._vlc.Meta.Title, item.display)
        except Exception:
            pass
        options: list[str] = []
        if item.start is not None:
            options.append(f":start-time={item.start}")
        if item.stop is not None:
            options.append(f":stop-time={item.stop}")
        for opt in options:
            media.add_option(opt)
        self._media_list.add_media(media)
        self._queue.append(item)
        try:
            self._queue_mrls.append(media.get_mrl() or "")
        except Exception:
            self._queue_mrls.append("")

        if self._current_index is None:
            self._current_index = 0

    def add_path(self, path: Path, display: str | None = None) -> None:
        self.add(QueueItem(uri=str(path), display=display or path.name))

    def add_url(self, url: str, display: str | None = None) -> None:
        self.add(QueueItem(uri=url, display=display or url))

    def play(self) -> None:
        self._player.play()

    def play_index(self, index: int) -> None:
        if index < 0 or index >= len(self._queue):
            return
        try:
            self._current_index = index
            self._player.play_item_at_index(index)
        except Exception:
            # Fallback to regular play.
            self._player.play()

    def pause(self) -> None:
        self._player.pause()

    def seek_seconds(self, seconds: float) -> None:
        """Seek to an absolute time in seconds (best-effort)."""
        try:
            ms = int(max(0.0, float(seconds)) * 1000.0)
        except Exception:
            return
        try:
            self._media_player.set_time(ms)
        except Exception:
            return

    def toggle_pause(self) -> None:
        # VLC's pause toggles
        self._player.pause()

    def stop(self) -> None:
        self._player.stop()

    def next(self) -> None:
        self._player.next()

    def previous(self) -> None:
        self._player.previous()

    def is_playing(self) -> bool:
        return bool(self._media_player.is_playing())

    def now_playing(self) -> str:
        media = self._media_player.get_media()
        if not media:
            return ""
        mrl = media.get_mrl() or ""
        return mrl

    def current_item(self) -> QueueItem | None:
        if self._current_index is None:
            return None
        if self._current_index < 0 or self._current_index >= len(self._queue):
            return None
        return self._queue[self._current_index]

    def current_index(self) -> int | None:
        return self._current_index

    def _on_played(self, event) -> None:
        # Ensure we have a sane current index on first start.
        if not self._queue:
            self._current_index = None
            return
        if self._current_index is None or self._current_index >= len(self._queue):
            self._current_index = 0

    def _on_stopped(self, event) -> None:
        # Keep index (so UI can still show last selection), but validate range.
        if self._current_index is not None and self._current_index >= len(self._queue):
            self._current_index = None

    def _on_media_changed(self, event) -> None:
        """Sync current index from the media that is actually set on the MediaPlayer."""
        self.sync_current_index()

    def sync_current_index(self) -> None:
        """Best-effort: recompute current queue index from current media.

        This keeps UI state (now playing/highlight) correct even if VLC events
        are delayed or missed.
        """
        if not self._queue:
            self._current_index = None
            return

        media = None
        try:
            media = self._media_player.get_media()
        except Exception:
            media = None
        if media is None:
            return

        try:
            mrl = media.get_mrl() or ""
        except Exception:
            mrl = ""
        try:
            title = media.get_meta(self._vlc.Meta.Title) or ""
        except Exception:
            title = ""

        if title:
            for idx, qi in enumerate(self._queue):
                if qi.display == title:
                    self._current_index = idx
                    return

        if mrl:
            for idx, qmrl in enumerate(self._queue_mrls):
                if qmrl == mrl:
                    self._current_index = idx
                    return

        # If we can't match, don't guess (avoids drifting).

    def playback_times(self) -> tuple[float | None, float | None]:
        """Return (elapsed_seconds, total_seconds) if available."""
        try:
            t_ms = int(self._media_player.get_time())
        except Exception:
            t_ms = -1
        try:
            l_ms = int(self._media_player.get_length())
        except Exception:
            l_ms = -1

        elapsed = (t_ms / 1000.0) if t_ms is not None and t_ms >= 0 else None
        total = (l_ms / 1000.0) if l_ms is not None and l_ms > 0 else None
        return elapsed, total

    def bitrate_kbps(self) -> int | None:
        """Best-effort bitrate in kbps for the currently playing media."""
        try:
            media = self._media_player.get_media()
        except Exception:
            media = None
        if media is None:
            return None

        try:
            stats = media.get_stats()
        except Exception:
            stats = None
        if not stats:
            return None

        # VLC reports these as kb/s (floats). Prefer demux bitrate when present.
        for attr in ("demux_bitrate", "input_bitrate"):
            try:
                v = getattr(stats, attr, None)
            except Exception:
                v = None
            if v is None:
                continue
            try:
                fv = float(v)
            except Exception:
                continue
            if fv > 0:
                return int(round(fv))

        return None
