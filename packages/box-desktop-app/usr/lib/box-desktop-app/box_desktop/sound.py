"""The box's sound, through PipeWire (WirePlumber's wpctl, pw-dump, pw-play,
pw-record): which speakers and microphone it uses, their volume, a test sound,
and the microphone's level as it hears."""

from __future__ import annotations

import array
import json
import math
import os
import shutil
import subprocess
import tempfile
import threading
import wave
from collections.abc import Callable
from pathlib import Path

OUTPUT = "@DEFAULT_AUDIO_SINK@"
INPUT = "@DEFAULT_AUDIO_SOURCE@"


def has_hardware() -> bool:
    """A sound card the kernel found; the Sound tools show only then."""
    try:
        return "no soundcards" not in Path("/proc/asound/cards").read_text()
    except OSError:
        return False


def short_name(name: str) -> str:
    """A device's name as the sound system gives it, shorter: HDMI/DP for
    "HDMI / DisplayPort"."""
    return name.replace("HDMI / DisplayPort", "HDMI/DP")


class Sound:
    def devices(self, device: str) -> list[tuple[int, str, bool]]:
        """The speakers (OUTPUT) or microphones (INPUT) as (id, name, in use)."""
        wanted = "Audio/Sink" if device == OUTPUT else "Audio/Source"
        key = "default.audio.sink" if device == OUTPUT else "default.audio.source"
        try:
            objects = json.loads(subprocess.run(["pw-dump"], capture_output=True, text=True,
                                                timeout=5).stdout or "[]")
        except (OSError, ValueError, subprocess.TimeoutExpired):
            return []
        chosen = ""
        for o in objects:
            if o.get("type") == "PipeWire:Interface:Metadata" and \
                    (o.get("props") or {}).get("metadata.name") == "default":
                for entry in o.get("metadata") or []:
                    if entry.get("key") == key and isinstance(entry.get("value"), dict):
                        chosen = entry["value"].get("name", "")
        found = []
        for o in objects:
            props = (o.get("info") or {}).get("props") or {}
            if o.get("type") == "PipeWire:Interface:Node" and props.get("media.class") == wanted:
                found.append((o["id"], short_name(props.get("node.description") or props.get("node.name", "")),
                              props.get("node.name") == chosen))
        return found

    def use(self, node: int) -> None:
        """Makes the speakers or microphone with this id the ones the box uses."""
        subprocess.run(["wpctl", "set-default", str(node)], timeout=5)

    def volume(self, device: str) -> tuple[float, bool]:
        """(0.0 to 1.5, muted)."""
        out = subprocess.run(["wpctl", "get-volume", device], capture_output=True, text=True,
                             timeout=5).stdout
        # "Volume: 0.40" or "Volume: 0.40 [MUTED]"
        parts = out.split()
        level = float(parts[1]) if len(parts) > 1 else 0.0
        return level, "[MUTED]" in out

    def set_volume(self, device: str, level: float) -> None:
        subprocess.run(["wpctl", "set-volume", "-l", "1.0", device, f"{level:.2f}"], timeout=5)

    def set_muted(self, device: str, muted: bool) -> None:
        subprocess.run(["wpctl", "set-mute", device, "1" if muted else "0"], timeout=5)

    def play_test(self) -> None:
        """A short two-note chime, made here, so no sound files are needed."""
        rate = 48000
        samples = array.array("h")
        for freq, length in ((660, 0.18), (880, 0.28)):
            for i in range(int(rate * length)):
                fade = min(1.0, (rate * length - i) / (rate * 0.05))
                samples.append(int(9000 * fade * math.sin(2 * math.pi * freq * i / rate)))
        with tempfile.NamedTemporaryFile(suffix=".wav", delete=False) as file:
            path = file.name
        with wave.open(path, "wb") as out:
            out.setnchannels(1)
            out.setsampwidth(2)
            out.setframerate(rate)
            out.writeframes(samples.tobytes())
        threading.Thread(target=lambda: (subprocess.run(["pw-play", path], timeout=10),
                                         os.unlink(path)), daemon=True).start()

    def listen(self, level: Callable[[float], None]) -> Callable[[], None]:
        """Calls level(0.0 to 1.0) about ten times a second; returns stop."""
        if not shutil.which("pw-record"):
            return lambda: None
        process = subprocess.Popen(
            ["pw-record", "--rate", "16000", "--channels", "1", "--format", "s16", "-"],
            stdout=subprocess.PIPE, stderr=subprocess.DEVNULL,
        )

        def read() -> None:
            chunk = 3200  # a tenth of a second
            while process.poll() is None:
                data = process.stdout.read(chunk)
                if not data:
                    break
                values = array.array("h", data[: len(data) // 2 * 2])
                peak = max((abs(v) for v in values), default=0) / 32768
                level(min(1.0, peak * 1.6))

        threading.Thread(target=read, daemon=True).start()
        return process.terminate
