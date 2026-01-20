from __future__ import annotations

from dataclasses import dataclass
from pathlib import Path

SUPPORTED_AUDIO_EXTS = {".mp3", ".ogg", ".flac"}
SUPPORTED_CUE_EXT = ".cue"


@dataclass(frozen=True)
class LibraryItem:
    path: Path
    is_dir: bool


def list_dir(path: Path) -> list[LibraryItem]:
    items: list[LibraryItem] = []
    try:
        for child in sorted(path.iterdir(), key=lambda p: (not p.is_dir(), p.name.lower())):
            if child.is_dir():
                items.append(LibraryItem(path=child, is_dir=True))
                continue
            if child.suffix.lower() in SUPPORTED_AUDIO_EXTS or child.suffix.lower() == SUPPORTED_CUE_EXT:
                items.append(LibraryItem(path=child, is_dir=False))
    except FileNotFoundError:
        return []
    except PermissionError:
        return []
    return items


def iter_audio_files_recursive(path: Path) -> list[Path]:
    results: list[Path] = []
    if path.is_file():
        return [path]
    for p in path.rglob("*"):
        if p.is_file() and (p.suffix.lower() in SUPPORTED_AUDIO_EXTS or p.suffix.lower() == SUPPORTED_CUE_EXT):
            results.append(p)
    return sorted(results)
