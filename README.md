# musipal

Terminal (CLI/TUI) music player:
- Local library browser (file-manager-like)
- Playlists (M3U)
- Playback via VLC (mp3/ogg/flac + more)
- CUE sheets supported (tracks mapped to start/stop times)
- Icecast streams (play by URL)

## Install (dev)

```bash
python -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt
pip install -e .
```

## Run

```bash
musipal
```

## Build a Linux binary

This app uses `python-vlc`, which depends on the system VLC/libVLC runtime.
On Ubuntu/Debian you typically need `vlc` installed.

Build with PyInstaller:

```bash
./packaging/build.sh            # one-folder build
./dist/musipal
```

If you want to copy it into `~/bin`, either copy the whole folder:

```bash
cp -av dist/musipal ~/bin/
~/bin/musipal
```

Or build a single-file executable:

```bash
./packaging/build.sh --onefile
cp dist/musipal ~/bin/musipal
~/bin/musipal --help
```

## Keys

- `↑/↓` move
- `Enter` open directory / play file
- `Backspace` go up
- `a` add selected item to queue
- `A` add directory recursively to queue
- `s` add Icecast/stream URL to queue
- `Space` play/pause
- `n` next, `p` previous
- `q` quit
- `w` write current queue to an M3U playlist
- `l` load an M3U playlist into queue

## Config

Config file: `~/.config/musipal/config.toml`

```toml
library_root = "/path/to/music"
playlists_dir = "/path/to/playlists"
```
