# musipal (Go port)

A Go port of the Python `musipal` terminal music player. Same features,
config, and session file format — built with [tview](https://github.com/rivo/tview)
instead of prompt_toolkit, and `ffplay`/`ffmpeg`/`ffprobe` instead of
python-vlc (no cgo/libvlc dependency required).

## Requirements

- Go 1.24+
- `ffmpeg` (provides `ffplay`/`ffprobe`/`ffmpeg`) on `PATH` for playback,
  duration/bitrate probing, and Icecast streaming.

## Build & run

```bash
cd go
go build -o musipal ./cmd/musipal
./musipal
```

Or run directly:

```bash
go run ./cmd/musipal --library-root /path/to/music
```

## Config

Shares the same config/session paths as the Python version:

- `~/.config/musipal/config.toml`
- `~/.config/musipal/last_session.json`

## Keys

Same keybindings as the Python version — see the top-level [README](../README.md#keys).
