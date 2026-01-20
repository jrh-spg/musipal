from __future__ import annotations

from dataclasses import dataclass
from pathlib import Path


@dataclass(frozen=True)
class PlaylistEntry:
    # Either a file path or a URL
    uri: str
    display: str


def write_m3u(path: Path, entries: list[PlaylistEntry]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    lines = ["#EXTM3U"]
    for e in entries:
        lines.append(e.uri)
    path.write_text("\n".join(lines) + "\n", encoding="utf-8")


def load_m3u(path: Path) -> list[str]:
    if not path.exists():
        return []
    uris: list[str] = []
    for line in path.read_text(encoding="utf-8", errors="replace").splitlines():
        s = line.strip()
        if not s or s.startswith("#"):
            continue
        uris.append(s)
    return uris
