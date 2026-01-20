from __future__ import annotations

from dataclasses import dataclass
from pathlib import Path
import json

from musipal.player import QueueItem


def _session_path() -> Path:
    return Path.home() / ".config" / "musipal" / "last_session.json"


@dataclass(frozen=True)
class SessionState:
    queue: list[QueueItem]
    index: int
    elapsed: float | None
    was_playing: bool
    queue_selected_index: int


def load_session(path: Path | None = None) -> SessionState | None:
    p = path or _session_path()
    try:
        if not p.exists():
            return None
        data = json.loads(p.read_text(encoding="utf-8"))
    except Exception:
        return None

    q: list[QueueItem] = []
    try:
        for it in list((data or {}).get("queue", [])):
            try:
                uri = str((it or {}).get("uri", ""))
                display = str((it or {}).get("display", ""))
                start = (it or {}).get("start", None)
                stop = (it or {}).get("stop", None)
                start_f = float(start) if start is not None else None
                stop_f = float(stop) if stop is not None else None
                if uri:
                    q.append(QueueItem(uri=uri, display=display or uri, start=start_f, stop=stop_f))
            except Exception:
                continue
    except Exception:
        q = []

    try:
        index = int((data or {}).get("index", 0))
    except Exception:
        index = 0

    elapsed_raw = (data or {}).get("elapsed", None)
    try:
        elapsed = float(elapsed_raw) if elapsed_raw is not None else None
    except Exception:
        elapsed = None

    try:
        was_playing = bool((data or {}).get("was_playing", False))
    except Exception:
        was_playing = False

    try:
        queue_selected_index = int((data or {}).get("queue_selected_index", index))
    except Exception:
        queue_selected_index = index

    if not q:
        return None

    index = max(0, min(index, len(q) - 1))
    queue_selected_index = max(0, min(queue_selected_index, len(q) - 1))

    return SessionState(
        queue=q,
        index=index,
        elapsed=elapsed,
        was_playing=was_playing,
        queue_selected_index=queue_selected_index,
    )


def save_session(state: SessionState, path: Path | None = None) -> None:
    p = path or _session_path()
    p.parent.mkdir(parents=True, exist_ok=True)

    payload = {
        "index": int(state.index),
        "elapsed": float(state.elapsed) if state.elapsed is not None else None,
        "was_playing": bool(state.was_playing),
        "queue_selected_index": int(state.queue_selected_index),
        "queue": [
            {
                "uri": it.uri,
                "display": it.display,
                "start": it.start,
                "stop": it.stop,
            }
            for it in state.queue
        ],
    }

    try:
        tmp = p.with_suffix(p.suffix + ".tmp")
        tmp.write_text(json.dumps(payload, ensure_ascii=False, indent=2), encoding="utf-8")
        tmp.replace(p)
    except Exception:
        # Best-effort; failing to save should not crash the app.
        return
