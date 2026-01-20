from __future__ import annotations

from dataclasses import dataclass
from pathlib import Path

from tomlkit import dumps, parse


@dataclass(frozen=True)
class Config:
    library_root: Path
    playlists_dir: Path


def _config_path() -> Path:
    return Path.home() / ".config" / "musipal" / "config.toml"


def load_config(library_root_override: str | None = None) -> Config:
    cfg_path = _config_path()
    cfg_path.parent.mkdir(parents=True, exist_ok=True)

    if not cfg_path.exists():
        default_library = Path.home() / "Music"
        default_playlists = Path.home() / "Music" / "Playlists"
        default_playlists.mkdir(parents=True, exist_ok=True)
        cfg_path.write_text(
            dumps(
                {
                    "library_root": str(default_library),
                    "playlists_dir": str(default_playlists),
                }
            ),
            encoding="utf-8",
        )

    data = parse(cfg_path.read_text(encoding="utf-8"))
    library_root = Path(str(data.get("library_root", Path.home() / "Music"))).expanduser()
    playlists_dir = Path(str(data.get("playlists_dir", Path.home() / "Music" / "Playlists"))).expanduser()

    if library_root_override:
        library_root = Path(library_root_override).expanduser()

    playlists_dir.mkdir(parents=True, exist_ok=True)
    return Config(library_root=library_root, playlists_dir=playlists_dir)
