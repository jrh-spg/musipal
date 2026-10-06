# musipal

Terminal (CLI/TUI) music player, written in Go:
- Local library browser (file-manager-like)
- Playlists (M3U)
- Playback via ffplay (mp3/ogg/flac + more)
- CUE sheets supported (tracks mapped to start/stop times)
- Icecast streaming of the current track via ffmpeg

## Requirements

- Go 1.24+ (only needed to build from source)
- `ffmpeg` (provides `ffplay`/`ffprobe`/`ffmpeg`) on `PATH`, for playback,
  duration/bitrate probing, and Icecast streaming

## Prebuilt packages

Each [GitHub release](https://github.com/jrh-spg/musipal/releases) ships rpm/deb
packages:

| Distro       | Package                              | Install                              |
|--------------|---------------------------------------|---------------------------------------|
| RHEL 9       | `musipal-<version>.el9.x86_64.rpm`    | `sudo dnf install ./musipal-*.el9.x86_64.rpm` |
| RHEL 10      | `musipal-<version>.el10.x86_64.rpm`   | `sudo dnf install ./musipal-*.el10.x86_64.rpm` |
| Ubuntu 24.04 | `musipal_<version>_ubuntu24.04_amd64.deb` | `sudo apt install ./musipal_*_ubuntu24.04_amd64.deb` |
| Ubuntu 26.04 | `musipal_<version>_ubuntu26.04_amd64.deb` | `sudo apt install ./musipal_*_ubuntu26.04_amd64.deb` |

These packages install a single `/usr/bin/musipal` binary; `ffmpeg` must be
installed separately. Packages are built by
[.github/workflows/package.yml](.github/workflows/package.yml).

## Build from source

```bash
go build -o musipal ./cmd/musipal
./musipal
```

Or run directly without a separate build step:

```bash
go run ./cmd/musipal --library-root /path/to/music
```

## Keys

- `↑/↓` move
- `Enter` open directory / play file / activate selected queue item
- `Backspace` go up
- `b` / `B` back / forward through visited directories
- `PageUp`/`PageDown` move selection by page
- `Tab` / `Shift-Tab` switch focus between Library and Queue
- `a` add selected item to queue
- `A` add directory recursively to queue
- `Space` play/pause
- `n` next, `p` previous
- `w` write current queue to an M3U playlist
- `l` load an M3U playlist into queue
- `d` or `Delete` delete selected song from queue
- `D` delete playlist file
- `h` show help
- `q` quit

Streaming and Icecast
- `t` toggle streaming of the current track to the configured Icecast mount (starts/stops streaming)
- `T` set the Icecast server URL for this session (inline input mode)

When you press `T` an inline input prompt appears in the status area. Type the
server URL and press `Enter` to save (or `Esc` to cancel). The URL is stored
only for the running session unless you edit the config manually.

Example Icecast URL (with source password), replacing `SECRET_PASSWORD` and `mount`:

    icecast://source:SECRET_PASSWORD@icecast.example.com:8000/mount

Keep your source password private; do not commit real credentials to the
repository or share them publicly.

Streaming requires `ffmpeg` on your `PATH` (the app spawns `ffmpeg` to encode
and push to the Icecast mount).

## Config

Config file: `~/.config/musipal/config.toml`

```toml
library_root = "/path/to/music"
playlists_dir = "/path/to/playlists"
```

Session state (queue + playback position) is persisted to
`~/.config/musipal/last_session.json` and restored on the next launch.
