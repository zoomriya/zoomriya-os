"""Stand-ins for the agent, ZOOMRIYA ID and the sound, to run Box Desktop on a
development machine:

    BOX_DESKTOP_FAKE=1 python3 -m box_desktop

Signs in "amma" with any password of 8 characters or more; "kid" isn't an admin.
"""

from __future__ import annotations

import math
import threading
import time
from collections.abc import Callable
from typing import Any

from zoomriya_setup.fake import FakeAgent

from .display import Mode, Output
from .signin import Member, SignInError


class FakeBoxAgent(FakeAgent):
    def __init__(self) -> None:
        super().__init__()
        self.started -= 10  # the cable is in
        self.locale, self.region = "en_CA.UTF-8", "CA"  # as the first setup left it
        self.check: dict[str, Any] = {"never_run": False, "errors": 0}

    def ask(self, op: str, **fields: Any) -> dict[str, Any]:
        if op == "status":
            return {"os": "ZOOMRIYA OS 13", "image_version": "2026.11.4",
                    "uptime_seconds": 3 * 86400 + 4 * 3600 + 120, "restart_needed": True,
                    "restart_reasons": ["a new kernel"], "maintenance_hour": "03:30",
                    "last_security_update": "2026-11-03T03:31:00Z",
                    "last_update_check": "2026-11-03T03:30:00Z", "model": "ASUS NUC 14 Pro",
                    "serial": "G6RVH40712", "debian": "13.2", "base": "13.0.0",
                    "kernel": "6.12.115+deb13.1-amd64"}
        if op == "resources":
            return {"cpu_percent": 6.5, "memory_total_bytes": 16 << 30, "memory_used_bytes": 5 << 30,
                    "network": {"in_bps": 182_000, "out_bps": 24_000},
                    "services": [
                        {"name": "ZOOMRIYA ID", "unit": "zoomriya-id.service", "kind": "system",
                         "cpu_percent": 1.2, "memory_bytes": 82_000_000},
                        {"name": "Box Desktop", "unit": "box-desktop-app.service", "kind": "system",
                         "cpu_percent": 0.8, "memory_bytes": 64_000_000},
                        {"name": "Databases", "unit": "postgresql@17-main.service", "kind": "system",
                         "cpu_percent": 0.4, "memory_bytes": 120_000_000}],
                    "fans": [{"sensor": "nct6775", "label": "CPU fan", "rpm": 1150}],
                    "disks": {"read_bps": 1_200_000, "write_bps": 340_000},
                    "temperatures": [{"sensor": "coretemp", "label": "Package id 0", "celsius": 47.0}]}
        if op == "restore-point-list":
            return {"restore_points": [
                {"number": 1, "date": "2026-11-01T09:00:00Z", "description": "factory", "kind": "factory"},
                {"number": 7, "date": "2026-11-03T03:31:00Z", "description": "before apt", "kind": "pre"},
                {"number": 8, "date": "2026-11-03T03:33:00Z", "description": "after apt", "kind": "post"}]}
        if op == "storage":
            return {"system": {"size_bytes": 250e9, "used_bytes": 41e9, "free_bytes": 209e9},
                    "system_parts": [{"kind": "app-data", "bytes": 18e9}, {"kind": "app-images", "bytes": 9e9},
                                     {"kind": "models-and-maps", "bytes": 7e9},
                                     {"kind": "databases", "bytes": 2e9}],
                    "library": {"path": "/srv/library", "mirror": True, "size_bytes": 4e12,
                                "used_bytes": 1.3e12, "free_bytes": 2.7e12,
                                "drives": [{"name": "ZLIB0001"}, {"name": "ZLIB0002"}],
                                "check": self.check}}
        if op == "storage-check":
            action = fields.get("action")
            self.check = {"running": action != "pause", "paused": action == "pause",
                          "percent": self.check.get("percent", 0) or 42.0,
                          "started": "2026-11-03T02:00:00Z"}
            return {"check": self.check}
        if op == "disk-health":
            return {"drives": [
                {"name": "nvme0n1", "model": "Samsung 980", "size_bytes": 250e9, "system": True,
                 "health": "healthy", "temperature_c": 38, "reasons": []},
                {"name": "sda", "model": "WD Red Plus", "size_bytes": 4e12, "health": "warning",
                 "temperature_c": 41, "reasons": ["12 sectors were moved aside; keep an eye on it."]}]}
        if op == "logs":
            return {"entries": [
                {"time": "2026-11-03T03:31:00Z", "level": "info", "source": "Updates",
                 "message": "Installed 3 security fixes."},
                {"time": "2026-11-03T02:10:00Z", "level": "warning", "source": "Drives",
                 "message": "sda moved 12 sectors aside."}], "more": False}
        if op == "security":
            return {"updates": {"automatic": True}, "firewall": {"on": True, "open": []},
                    "integrity": {"checked": "2026-11-03T04:00:00Z", "clean": True, "changes": []},
                    "apparmor": {"on": True, "enforced": 12}, "secure_boot": "on",
                    "connections": [{"host": "deb.debian.org", "what": "Security fixes"}]}
        if op == "notifications":
            return {"notifications": [
                {"time": "2026-11-03T02:10:00Z", "level": "warning", "kind": "drive",
                 "message": "A drive needs checking: sda."},
                {"time": "2026-11-03T03:33:00Z", "level": "info", "kind": "update",
                 "message": "3 security fixes were installed."}],
                "kinds": ["drive", "update", "restart", "check", "network"]}
        if op == "display-set":
            return {"output": fields.get("output"), "saved": True}
        if op in ("restart", "shutdown", "restore-point-restore", "maintenance-hour", "factory-reset"):
            return {"ok": True, "maintenance_hour": fields.get("hour", "03:30")}
        return super().ask(op, **fields)


class FakeID:
    def sign_in(self, username: str, password: str) -> Member:
        time.sleep(0.3)
        if username.strip() == "kid":
            raise SignInError("Box Desktop is for the box's admins.")
        if len(password) < 8:
            raise SignInError("That username and password don't match.")
        return Member(1, username.strip(), username.strip().title(), "token")

    def sign_out(self, member: Member) -> None:
        pass


class FakeSound:
    def __init__(self) -> None:
        self.levels = {"@DEFAULT_AUDIO_SINK@": [0.6, False], "@DEFAULT_AUDIO_SOURCE@": [0.8, False]}
        self.listed = {"@DEFAULT_AUDIO_SINK@": [[41, "HDMI / DisplayPort, built-in audio", True],
                                                [42, "Headphones, built-in audio", False]],
                       "@DEFAULT_AUDIO_SOURCE@": [[51, "Internal microphone, built-in audio", True]]}

    def devices(self, device: str) -> list[tuple[int, str, bool]]:
        return [tuple(d) for d in self.listed[device]]

    def use(self, node: int) -> None:
        for listed in self.listed.values():
            if any(d[0] == node for d in listed):
                for d in listed:
                    d[2] = d[0] == node

    def volume(self, device: str) -> tuple[float, bool]:
        level, muted = self.levels[device]
        return level, muted

    def set_volume(self, device: str, level: float) -> None:
        self.levels[device][0] = level

    def set_muted(self, device: str, muted: bool) -> None:
        self.levels[device][1] = muted

    def play_test(self) -> None:
        pass

    def listen(self, level: Callable[[float], None]) -> Callable[[], None]:
        running = [True]

        def run() -> None:
            t = 0.0
            while running[0]:
                level(abs(math.sin(t)) * 0.7)
                t += 0.4
                time.sleep(0.1)

        threading.Thread(target=run, daemon=True).start()
        return lambda: running.__setitem__(0, False)


class FakeDisplay:
    """A 27-inch monitor on HDMI, as sway would list it."""

    def __init__(self) -> None:
        modes = [Mode(w, h, r) for w, h, rates in (
            (2560, 1440, (59951, 74971)), (1920, 1080, (60000, 59940, 50000)),
            (1680, 1050, (59883,)), (1280, 720, (60000, 50000)), (1024, 768, (60004,)))
            for r in rates]
        self.screen = Output("HDMI-A-1", "Dell U2724D", modes, modes[0])

    def outputs(self) -> list[Output]:
        return [self.screen]

    def apply(self, name: str, mode: Mode, scale: float, transform: str) -> bool:
        self.screen.mode, self.screen.scale, self.screen.transform = mode, scale, transform
        return True

    def pointer(self, speed: float, accel: bool, natural: bool) -> bool:
        return True
