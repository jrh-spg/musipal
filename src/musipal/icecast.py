from __future__ import annotations

import shutil
import subprocess
import threading
from typing import Optional
from urllib.parse import urlparse, urlunparse


class IcecastStreamer:
    def __init__(self) -> None:
        self._proc: Optional[subprocess.Popen] = None
        self._thread: Optional[threading.Thread] = None

    def available(self) -> bool:
        return shutil.which("ffmpeg") is not None

    def is_streaming(self) -> bool:
        return self._proc is not None and self._proc.poll() is None

    def start(self, input_uri: str, dest_url: str, *, bitrate: int = 128, start_seconds: Optional[float] = None, duration: Optional[float] = None) -> None:
        """Start streaming `input_uri` to Icecast `dest_url` using ffmpeg.

        `dest_url` should be in the form icecast://user:pass@host:port/mount
        """
        if not self.available():
            raise RuntimeError("ffmpeg not found on PATH")
        if self.is_streaming():
            raise RuntimeError("Already streaming")

        # Normalize destination URL to use the icecast scheme which ffmpeg expects.
        try:
            parsed = urlparse(dest_url)
            if parsed.scheme != "icecast":
                new_parsed = parsed._replace(scheme="icecast")
                normalized = urlunparse(new_parsed)
                try:
                    with open("/tmp/musipal_icecast.log", "a") as fh:
                        fh.write(f"Normalizing dest_url: {dest_url!r} -> {normalized!r}\n")
                except Exception:
                    pass
                dest_url = normalized
        except Exception:
            # If parsing fails, continue with the original value and let ffmpeg report errors.
            pass

        cmd = ["ffmpeg", "-hide_banner", "-loglevel", "error", "-re"]
        # Seek if requested
        if start_seconds is not None:
            cmd += ["-ss", str(start_seconds)]

        cmd += ["-i", input_uri]

        # If a duration is provided, use -t to limit output duration.
        transcode = ["-c:a", "libmp3lame", "-b:a", f"{bitrate}k", "-f", "mp3"]
        cmd += transcode

        # Ensure content type for Icecast (explicit helps some servers/clients).
        cmd += ["-content_type", "audio/mpeg"]

        if duration is not None and duration > 0:
            cmd += ["-t", str(duration)]

        cmd += [dest_url]

        # Start ffmpeg in a background thread to avoid blocking the TUI.
        try:
            proc = subprocess.Popen(cmd, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
            import inspect
            if inspect.iscoroutine(proc) or inspect.isawaitable(proc):
                with open("/tmp/musipal_icecast.log", "a") as fh:
                    fh.write(f"ERROR: Attempted to assign coroutine/awaitable to self._proc: {proc}\n")
                raise RuntimeError("Attempted to assign coroutine/awaitable to self._proc")
            self._proc = proc

            def _wait_thread():
                try:
                    proc.wait()
                finally:
                    # Clear proc reference when process exits.
                    try:
                        self._proc = None
                    except Exception:
                        pass

            t = threading.Thread(target=_wait_thread, daemon=True)
            self._thread = t
            t.start()
            try:
                with open("/tmp/musipal_icecast.log", "a") as fh:
                    fh.write(f"Started ffmpeg proc: type={type(self._proc)!r} cmd={cmd}\n")
            except Exception:
                pass
        except Exception as e:
            raise RuntimeError(f"Failed to start ffmpeg: {e}")

    def stop(self) -> None:
        if self._proc is None:
            return
        try:
            import inspect, traceback

            # Log proc type for debugging.
            try:
                with open("/tmp/musipal_icecast.log", "a") as fh:
                    fh.write(f"Stopping ffmpeg proc: type={type(self._proc)!r}\n")
            except Exception:
                pass

            # If it's a coroutine (unexpected), just clear and return.
            if inspect.iscoroutine(self._proc) or inspect.isawaitable(self._proc):
                try:
                    with open("/tmp/musipal_icecast.log", "a") as fh:
                        fh.write("Proc is a coroutine/awaitable; cannot terminate synchronously\n")
                except Exception:
                    pass
                self._proc = None
                self._thread = None
                return

            try:
                try:
                    self._proc.terminate()
                except Exception:
                    pass
                # Prefer Popen.wait(timeout=...) when available.
                wait = getattr(self._proc, "wait", None)
                if callable(wait):
                    try:
                        wait(timeout=2.0)
                    except TypeError:
                        # wait() exists but doesn't accept timeout (unlikely); call without timeout.
                        try:
                            wait()
                        except Exception:
                            pass
                    except Exception:
                        try:
                            self._proc.kill()
                        except Exception:
                            pass
                else:
                    try:
                        self._proc.kill()
                    except Exception:
                        pass
            except Exception:
                try:
                    self._proc.kill()
                except Exception:
                    pass
        finally:
            self._proc = None
            self._thread = None


_default_streamer: Optional[IcecastStreamer] = None


def get_streamer() -> IcecastStreamer:
    global _default_streamer
    if _default_streamer is None:
        _default_streamer = IcecastStreamer()
    return _default_streamer
