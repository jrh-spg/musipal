from __future__ import annotations

from dataclasses import dataclass
from pathlib import Path


@dataclass(frozen=True)
class CueTrack:
    title: str
    performer: str
    source_file: Path
    start_seconds: float
    end_seconds: float | None


def _mmssff_to_seconds(mm: int, ss: int, ff: int) -> float:
    # CUE frames are 75 frames/sec
    return mm * 60 + ss + (ff / 75.0)


def parse_cue(cue_path: Path) -> list[CueTrack]:
    """Minimal CUE parser: supports FILE + TRACK + TITLE + PERFORMER + INDEX 01."""
    text = cue_path.read_text(encoding="utf-8", errors="replace").splitlines()

    album_file: Path | None = None
    album_performer = ""

    tracks_raw: list[dict] = []
    current: dict | None = None

    for line in text:
        line = line.strip()
        if not line or line.startswith("REM"):
            continue

        if line.upper().startswith("PERFORMER") and current is None:
            val = line.split(" ", 1)[1].strip().strip('"')
            album_performer = val
            continue

        if line.upper().startswith("FILE"):
            # FILE "something.flac" WAVE
            try:
                rest = line.split(" ", 1)[1].strip()
                quoted = rest.split(" ", 1)[0]
                file_name = quoted.strip('"')
                album_file = (cue_path.parent / file_name).resolve()
            except Exception:
                album_file = None
            continue

        if line.upper().startswith("TRACK"):
            if current is not None:
                tracks_raw.append(current)
            current = {"title": "", "performer": "", "index": None}
            continue

        if current is not None and line.upper().startswith("TITLE"):
            current["title"] = line.split(" ", 1)[1].strip().strip('"')
            continue

        if current is not None and line.upper().startswith("PERFORMER"):
            current["performer"] = line.split(" ", 1)[1].strip().strip('"')
            continue

        if current is not None and line.upper().startswith("INDEX 01"):
            # INDEX 01 mm:ss:ff
            parts = line.split()
            if len(parts) >= 3:
                t = parts[2]
                mm, ss, ff = (int(x) for x in t.split(":"))
                current["index"] = _mmssff_to_seconds(mm, ss, ff)
            continue

    if current is not None:
        tracks_raw.append(current)

    if album_file is None:
        # If missing FILE, assume same stem with flac
        album_file = cue_path.with_suffix(".flac")

    # Build CueTrack list with computed end times
    tracks: list[CueTrack] = []
    indices = [t["index"] for t in tracks_raw if t.get("index") is not None]
    for i, tr in enumerate(tracks_raw):
        start = tr.get("index")
        if start is None:
            continue
        end = None
        # end is next track start
        next_start = None
        # find next with index
        for j in range(i + 1, len(tracks_raw)):
            ns = tracks_raw[j].get("index")
            if ns is not None:
                next_start = ns
                break
        if next_start is not None:
            end = float(next_start)

        title = tr.get("title") or f"Track {i+1:02d}"
        performer = tr.get("performer") or album_performer
        tracks.append(
            CueTrack(
                title=title,
                performer=performer,
                source_file=album_file,
                start_seconds=float(start),
                end_seconds=end,
            )
        )
    return tracks


def cue_source_file(cue_path: Path) -> Path | None:
    """Best-effort extraction of the referenced FILE path in a .cue."""
    try:
        for line in cue_path.read_text(encoding="utf-8", errors="replace").splitlines():
            s = line.strip()
            if not s:
                continue
            if s.upper().startswith("FILE"):
                rest = s.split(" ", 1)[1].strip()
                quoted = rest.split(" ", 1)[0]
                file_name = quoted.strip('"')
                return (cue_path.parent / file_name).resolve()
    except Exception:
        return None
    return None
