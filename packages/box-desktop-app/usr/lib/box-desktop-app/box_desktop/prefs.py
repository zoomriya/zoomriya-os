"""Each admin's own Box Desktop, kept between sign-ins: the theme, the keyboard
and the mouse, the open windows and where they were, and which notifications
are read or turned off.
One file per admin, named by a hash of their ZOOMRIYA ID member id, in Box
Desktop's own state folder, readable by its account only."""

from __future__ import annotations

import hashlib
import json
import os
import tempfile
from pathlib import Path
from typing import Any

DEFAULTS: dict[str, Any] = {
    "version": 1,
    "theme": "daylight",
    # The keyboard's layout on the box's screen ("" for the box's own) and the mouse.
    "keyboard": "",
    "handed": "right",
    # The mouse's speed (-1 slow to 1 fast), acceleration and scrolling.
    "mouse": {"speed": 0.0, "accel": True, "natural": False},
    "windows": [],
    "notifications": {"off": [], "read_until": ""},
}


def state_dir() -> Path:
    """systemd's StateDirectory on the box; a folder of one's own elsewhere."""
    if os.environ.get("STATE_DIRECTORY"):
        return Path(os.environ["STATE_DIRECTORY"].split(":")[0])
    return Path(os.environ.get("XDG_STATE_HOME", Path.home() / ".local/state")) / "box-desktop-app"


class Prefs:
    def __init__(self, folder: Path | None = None) -> None:
        self.folder = (folder or state_dir()) / "admins"

    def path(self, member_id: int | str) -> Path:
        return self.folder / (hashlib.sha256(str(member_id).encode()).hexdigest() + ".json")

    def load(self, member_id: int | str) -> dict[str, Any]:
        prefs = json.loads(json.dumps(DEFAULTS))
        try:
            saved = json.loads(self.path(member_id).read_text())
        except (OSError, ValueError):
            return prefs
        if isinstance(saved, dict):
            prefs.update({k: v for k, v in saved.items() if k in DEFAULTS})
        if prefs["theme"] not in ("daylight", "midnight"):
            prefs["theme"] = "daylight"
        if prefs["handed"] not in ("right", "left"):
            prefs["handed"] = "right"
        mouse = prefs["mouse"] if isinstance(prefs["mouse"], dict) else {}
        speed = mouse.get("speed", 0.0)
        prefs["mouse"] = {
            "speed": max(-1.0, min(1.0, float(speed))) if isinstance(speed, (int, float)) else 0.0,
            "accel": mouse.get("accel", True) is not False,
            "natural": mouse.get("natural", False) is True,
        }
        return prefs

    def save(self, member_id: int | str, prefs: dict[str, Any]) -> None:
        self.folder.mkdir(parents=True, exist_ok=True, mode=0o700)
        os.chmod(self.folder, 0o700)
        fd, temp = tempfile.mkstemp(dir=self.folder, prefix=".new-")
        with os.fdopen(fd, "w") as out:
            json.dump(prefs, out)
        os.chmod(temp, 0o600)
        os.replace(temp, self.path(member_id))
