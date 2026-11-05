"""The box's own screen and mouse, as sway runs them (swaymsg): the screen's
resolution, refresh rate, scale and orientation, and how the mouse moves and
scrolls. Each admin's own, like the keyboard (prefs.py), put back when they
sign in. Away from the box (no sway) there's no screen to list and nothing
changes."""

from __future__ import annotations

import json
import os
import shutil
import subprocess
from dataclasses import dataclass, field

from zoomriya_setup.screen import swaymsg

from .i18n import N_

#: sway's transforms, as the person reads them.
ORIENTATIONS = [("normal", N_("Landscape")), ("90", N_("Portrait, turned right")),
                ("270", N_("Portrait, turned left")), ("180", N_("Landscape, upside down"))]
SCALES = [1.0, 1.25, 1.5, 1.75, 2.0]


@dataclass
class Mode:
    width: int
    height: int
    refresh: int  # mHz, as sway counts it

    @property
    def size(self) -> str:
        return f"{self.width} × {self.height}"

    @property
    def rate(self) -> str:
        return f"{self.refresh / 1000:.2f} Hz"


@dataclass
class Output:
    name: str
    title: str
    modes: list[Mode]
    mode: Mode
    scale: float = 1.0
    transform: str = "normal"
    settings: dict = field(default_factory=dict)

    def sizes(self) -> list[tuple[int, int]]:
        """The resolutions it can show, biggest first, each once."""
        seen = sorted({(m.width, m.height) for m in self.modes}, key=lambda s: -s[0] * s[1])
        return seen

    def rates(self, width: int, height: int) -> list[int]:
        return sorted({m.refresh for m in self.modes if (m.width, m.height) == (width, height)},
                      reverse=True)


def ratio(width: int, height: int) -> str:
    """16:9, 16:10, 4:3; nothing for the odd ones."""
    for name, (w, h) in {"16:9": (16, 9), "16:10": (16, 10), "4:3": (4, 3), "21:9": (64, 27),
                         "5:4": (5, 4), "3:2": (3, 2)}.items():
        if abs(width / height - w / h) < 0.01:
            return name
    return ""


class Display:
    def outputs(self) -> list[Output]:
        if not os.environ.get("SWAYSOCK") or not shutil.which("swaymsg"):
            return []
        done = subprocess.run(["swaymsg", "-t", "get_outputs", "-r"], capture_output=True,
                              text=True, timeout=5)
        try:
            raw = json.loads(done.stdout or "[]")
        except ValueError:
            return []
        found = []
        for o in raw:
            if not o.get("active"):
                continue
            modes = [Mode(m["width"], m["height"], m["refresh"]) for m in o.get("modes") or []]
            now = o.get("current_mode") or {}
            mode = Mode(now.get("width", 0), now.get("height", 0), now.get("refresh", 0))
            title = " ".join(p for p in (o.get("make"), o.get("model")) if p and p != "Unknown")
            found.append(Output(o["name"], title or o["name"], modes or [mode], mode,
                                float(o.get("scale") or 1.0), str(o.get("transform") or "normal")))
        return found

    def apply(self, name: str, mode: Mode, scale: float, transform: str) -> bool:
        return swaymsg("output", name, "mode", f"{mode.width}x{mode.height}@{mode.refresh / 1000:.3f}Hz",
                       "scale", f"{scale:g}", "transform", transform)

    def pointer(self, speed: float, accel: bool, natural: bool) -> bool:
        """speed from -1 (slow) to 1 (fast); accel: faster as the mouse moves faster;
        natural: scrolling moves the page, as on a phone."""
        return (swaymsg("input", "type:pointer", "accel_speed", f"{speed:.2f}")
                and swaymsg("input", "type:pointer", "accel_profile", "adaptive" if accel else "flat")
                and swaymsg("input", "type:pointer", "natural_scroll", "enabled" if natural else "disabled"))
