"""The box's own screen, changed while it runs (sway's swaymsg, screen-session):
the keyboard's layout and the mouse's handedness. Away from the box (no sway),
nothing happens."""

from __future__ import annotations

import os
import shutil
import subprocess


def swaymsg(*command: str) -> bool:
    if not os.environ.get("SWAYSOCK") or not shutil.which("swaymsg"):
        return False
    done = subprocess.run(["swaymsg", *command], capture_output=True, timeout=5)
    return done.returncode == 0


def set_keyboard(layout_id: str) -> bool:
    """A layout as the agent lists it: "fr", or "ch:fr" with its variant."""
    layout, _colon, variant = layout_id.partition(":")
    return (swaymsg("input", "type:keyboard", "xkb_layout", layout)
            and swaymsg("input", "type:keyboard", "xkb_variant", variant or '""'))


def set_left_handed(left: bool) -> bool:
    return swaymsg("input", "type:pointer", "left_handed", "enabled" if left else "disabled")
