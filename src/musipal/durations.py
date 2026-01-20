from __future__ import annotations

from pathlib import Path

from mutagen import File as MutagenFile


def get_duration_seconds(path: Path) -> float | None:
    try:
        audio = MutagenFile(path)
        if audio is None or not hasattr(audio, "info") or audio.info is None:
            return None
        length = getattr(audio.info, "length", None)
        if length is None:
            return None
        return float(length)
    except Exception:
        return None


def get_bitrate_kbps(path: Path) -> int | None:
    """Return the nominal bitrate in kbps for a local audio file."""
    try:
        audio = MutagenFile(path)
        if audio is None or not hasattr(audio, "info") or audio.info is None:
            return None
        bitrate = getattr(audio.info, "bitrate", None)
        if bitrate is None:
            return None
        bps = float(bitrate)
        if bps <= 0:
            return None
        return int(round(bps / 1000.0))
    except Exception:
        return None


def format_duration(seconds: float | None) -> str:
    if seconds is None or seconds < 0:
        return "--:--"
    total = int(seconds + 0.5)
    h = total // 3600
    m = (total % 3600) // 60
    s = total % 60
    if h > 0:
        return f"{h}:{m:02d}:{s:02d}"
    return f"{m}:{s:02d}"
