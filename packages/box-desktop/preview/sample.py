"""The sample box: stand-ins for the agent and ZOOMRIYA ID, so Box Desktop
runs on a development machine (scripts/preview-desktop.sh; fakebox.py serves
them as the box does). Carried over from the native desktop's fake.py.

Signs in "amma" with any password of 8 characters or more; "kid" isn't an admin;
"new" has a temporary password, so it's asked for its own first.
People shows a family of four, one of them a child and one turned off.
"""

from __future__ import annotations

import math
from pathlib import Path
import random
import threading
import time
from datetime import datetime, timedelta, timezone
from collections.abc import Callable
from typing import Any

from dataclasses import dataclass

from setupfake import AgentError, FakeAgent

PACKAGES = Path(__file__).resolve().parents[2]
#: The OS's name and version, set in one place: zoomriya-base's os-release
#: template, read here as the box reads its os-release.
TEMPLATE = PACKAGES / "zoomriya-base/usr/share/zoomriya-base/os-release.in"
#: Our pictures, as the page has them.
ASSETS = PACKAGES / "box-desktop/web/public/backgrounds"


def os_name(path: Path) -> str:
    """The OS's name, as os-release gives it (its PRETTY_NAME)."""
    try:
        fields = dict(line.split("=", 1) for line in path.read_text().splitlines() if "=" in line)
    except OSError:
        return "ZOOMRIYA OS"
    return fields.get("PRETTY_NAME", "").strip().strip('"') or "ZOOMRIYA OS"


@dataclass
class Member:
    id: int
    username: str
    name: str
    token: str


class SignInError(Exception):
    code = "wrong_password"


class NewPasswordNeeded(SignInError):
    pass


class SecondStepNeeded(SignInError):
    def __init__(self, message: str, ticket: str) -> None:
        super().__init__(message)
        self.ticket = ticket


class SecondStepSetupNeeded(SecondStepNeeded):
    pass


class PeopleError(Exception):
    pass


class FakeBoxAgent(FakeAgent):
    def __init__(self) -> None:
        super().__init__()
        # Internet access: the street map declared, nothing switched off.
        self.internet_state: dict[str, Any] = {"apps_offline": False, "all_offline": False, "off": []}
        self.started -= 10  # the cable is in
        self.locale, self.region = "en_CA.UTF-8", "CA"  # as the first setup left it
        self.check: dict[str, Any] = {"never_run": False, "errors": 0}
        self.fixes_until = 0.0  # Check now's security fixes, for a few seconds
        # Advanced access: off, with one key added.
        self.access_on = False
        self.keys: list[dict[str, Any]] = [
            {"name": "Anna's laptop", "type": "ED25519", "bits": 256,
             "fingerprint": "SHA256:q8fV2mH0bWQyZ3Rk9cNnXlP1sE4uJ7aD6tY5rB2oKiM",
             "added": "2026-11-02T18:20:00Z", "by": "amma", "last_used": "2026-11-05T21:12:00Z"}]
        self.bundles: list[dict[str, Any]] = [
            {"name": "zoomriya-diagnostics-20261103-091500.tar.gz", "size_bytes": 3_400_000,
             "made": "2026-11-03T09:15:00Z", "errors": 0, "warnings": 1}]
        self.apps: list[dict[str, Any]] = [
            {"id": "photos", "name": "Photos", "description": "The family's photos and videos", "version": "1.4.2",
             "previous": "1.4.1", "installed": "2026-11-04T10:00:00Z", "by": "amma", "health": "running", "data": True},
            {"id": "hub", "name": "Family Hub", "description": "The family's home page", "version": "1.1.0",
             "installed": "2026-11-04T10:00:00Z", "by": "amma", "health": "running", "data": True}]
        self.job: dict[str, Any] | None = None
        self.job_started = 0.0
        self.sticks = [{"device": "/dev/sdb1", "label": "ANNA", "fstype": "vfat", "size_bytes": 32_000_000_000}]

    def access(self) -> dict[str, Any]:
        return {"on": self.access_on, "since": "2026-11-08T09:00:00Z" if self.access_on else None,
                "by": "amma" if self.access_on else None, "keys": list(self.keys), "user": "admin",
                "address": "zoomriya-7k2q.local", "host_key": "SHA256:Hk3vYt1Qm9wLx2Rb7cPz0sN4uE6aJ8dF5gT2yK1oWiQ"}

    def _internet(self, op: str, fields: dict[str, Any]) -> dict[str, Any]:
        state = self.internet_state
        if op == "internet-set":
            action, on = fields.get("action"), fields.get("on")
            if action == "apps-offline":
                state["apps_offline"] = bool(on)
            elif action == "all-offline":
                state["all_offline"] = bool(on)
            elif action in ("allow", "stop"):
                state["off"] = [i for i in state["off"] if i != fields.get("name")]
                if action == "stop":
                    state["off"].append(fields.get("name"))
        blocked = ("Everything offline is on" if state["all_offline"] else "Apps offline is on"
                   if state["apps_offline"] else "Switched off" if "street-map" in state["off"] else "")
        return {
            "apps_offline": state["apps_offline"], "all_offline": state["all_offline"], "days": 7,
            "declared": [{
                "id": "street-map", "account": "zoomriya-maps", "name": "Street map downloads",
                "feature": "Family Tracker's map, and place names in Photos",
                "what": "Only which province or state's map to fetch. Never a position, a name or anything "
                        "about the family.",
                "why": "The map is drawn from files on the box, so nobody learns where the family looks.",
                "when": "Only when an admin downloads or updates a map in Family Hub",
                "optional": True, "allowed": not blocked, "blocked": blocked,
            "off": "street-map" in state["off"]}],
            "blocked": [{"account": "zoomriya", "who": "Family Photos", "destination": "142.250.80.46",
                         "protocol": "tcp", "port": 443, "count": 3, "last": "2026-11-09T18:20:00Z"}],
        }

    def _backups(self, fields: dict[str, Any]) -> dict[str, Any]:
        """The box's backup (the agent's backups.go): last night's failed for Photos."""
        state = self.__dict__.setdefault("backup_state", {"nightly": True, "started": 0.0})
        action = fields.get("action")
        if action == "now":
            state["started"] = time.time()
        elif action == "nightly":
            state["nightly"] = bool(fields.get("on"))
        answer: dict[str, Any] = {"set_up": True, "nightly": state["nightly"], "where": "/data/zoomriya/volumes/backup"}
        if state["started"] and time.time() - state["started"] < 12:
            return answer | {"running": True, "last_start": datetime.now(timezone.utc).isoformat()}
        if state["started"]:
            return answer | {"running": False, "last_ok": True, "last_start": "2026-11-10T20:00:00Z",
                             "last_end": datetime.now(timezone.utc).isoformat(), "next": "2026-11-11T04:00:00Z"}
        return answer | {"running": False, "last_ok": False, "last_start": "2026-11-10T04:00:00Z",
                         "last_end": "2026-11-10T04:21:00Z", "next": "2026-11-11T04:00:00Z",
                         "problems": ["photos: /data/apps/photos/originals can't be read"]}

    def app_page(self, app: dict[str, Any]) -> str:
        """A page for an app's window, as the box would serve it (appview.py)."""
        name = app.get("name", "")
        return f"""<!doctype html><meta charset="utf-8"><title>{name}</title>
<style>body{{margin:0;font:15px Inter,sans-serif;background:#f6f7fb;color:#1b2733}}
header{{padding:18px 28px;background:#fff;border-bottom:1px solid #e5eaf0;font-weight:600;font-size:18px}}
main{{padding:28px;display:grid;grid-template-columns:repeat(auto-fill,minmax(180px,1fr));gap:16px}}
.c{{height:140px;border-radius:14px;background:linear-gradient(135deg,#7cb6ff,#a49bff)}}
.c:nth-child(3n){{background:linear-gradient(135deg,#6ee29a,#5fd8ea)}}
.c:nth-child(4n){{background:linear-gradient(135deg,#ffb565,#ff8fb1)}}</style>
<header>{name}</header><main>{"<div class=c></div>" * 12}</main>"""

    def _pictures(self, fields: dict[str, Any]) -> dict[str, Any]:
        """Two of our own pictures, as if on a stick (the agent's pictures.go)."""
        import base64

        assets = ASSETS
        on_stick = {"Holiday/lake.jpg": "lakeside.jpg", "hills.jpg": "dusk.jpg"}
        time.sleep(0.6)
        if fields.get("action") == "read":
            file = on_stick.get(fields.get("name", ""))
            if file is None:
                raise AgentError("That picture isn't on the USB stick any more.")
            return {"jpeg": base64.b64encode((assets / file).read_bytes()).decode()}
        found = []
        for name, file in on_stick.items():
            thumb = (assets / file).read_bytes()  # the agent's is smaller; the page shows it the same
            found.append({"stick": "ANNA", "device": "/dev/sdb1", "file": name,
                          "size_bytes": (assets / file).stat().st_size, "width": 2560, "height": 1440,
                          "thumb": base64.b64encode(thumb).decode()})
        return {"found": found}

    def ask(self, op: str, **fields: Any) -> dict[str, Any]:
        if op in ("internet", "internet-set"):
            return self._internet(op, fields)
        if op == "admin-recover":
            # The sample box's setup code.
            time.sleep(0.3)
            if fields.get("setup_code", "").replace(" ", "").upper() not in ("K7QM-4XRT", "K7QM4XRT"):
                raise AgentError("That isn't the box's setup code. It's on the sticker, or the one "
                                 "written down at the first setup.")
            return {"username": fields.get("username")}
        if op == "status":
            return {"os": os_name(TEMPLATE), "image_version": "2026.11.4", "support_end": "2030-06-30",
                    "uptime_seconds": 3 * 86400 + 4 * 3600 + 120, "restart_needed": True,
                    "restart_reasons": ["update: linux-image-6.12.116+deb13.1-amd64"], "maintenance_hour": "03:30", "always_on": True,
                    "last_security_update": "2026-11-03T03:31:00Z",
                    "last_update_check": "2026-11-03T03:30:00Z", "model": "ASUS NUC 14 Pro",
                    "serial": "G6RVH40712", "debian": "13.2", "base": "13.0.0",
                    "kernel": "6.12.115+deb13.1-amd64",
                    "advanced_access": self.access_on}
        if op == "resources":
            return {"cpu_percent": 6.5, "memory_total_bytes": 16 << 30, "memory_used_bytes": 5 << 30,
                    # Rising and falling, so the preview's graphs move.
                    "network": {"in_bps": 182_000 * (1.2 + math.sin(time.time() / 9)) + random.uniform(0, 90_000),
                                "out_bps": 24_000 * (1.2 + math.cos(time.time() / 13)) + random.uniform(0, 20_000)},
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
        if op == "network":
            # The box's Wi-Fi chip, which stays off (OS-14).
            answer = super().ask(op, **fields)
            answer["links"].append({"name": "wlp2s0", "kind": "wifi", "connected": False,
                                    "mac": "f4:4d:30:6e:21:0b", "ipv4": []})
            return answer
        if op == "security-check-now":
            self.fixes_until = time.time() + 12
            return {"started": True}
        if op == "held":
            return {"held": [{"package": "openssl", "version": "3.5.1-1",
                              "taken_back": "3.5.1-1+deb13u1"}]}
        if op == "held-release":
            return {"released": fields.get("package")}
        if op == "restore-point-list":
            return {"restore_points": [
                {"number": 1, "date": "2026-11-01T09:00:00Z", "description": "factory", "kind": "factory"},
                {"number": 7, "date": "2026-11-03T03:31:00Z", "description": "apt", "kind": "before-update"},
                {"number": 8, "date": "2026-11-03T03:33:00Z", "description": "apt", "kind": "after-update"}]}
        if op == "storage":
            return {"system": {"size_bytes": 250e9, "used_bytes": 41e9, "free_bytes": 209e9},
                    "system_parts": [{"kind": "app-data", "bytes": 18e9}, {"kind": "app-images", "bytes": 9e9},
                                     {"kind": "models-and-maps", "bytes": 7e9},
                                     {"kind": "databases", "bytes": 2e9}],
                    # One library drive, as on today's box (OS-4: a mirror when there are two).
                    "library": {"path": "/srv/library", "mirror": False, "size_bytes": 4e12,
                                "used_bytes": 1.3e12, "free_bytes": 2.7e12,
                                "drives": [{"name": "ZLIB0001"}],
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
            return {"updates": {"automatic": True, "running": time.time() < self.fixes_until,
                                "last_installed": "2026-11-03T03:31:00Z"},
                    "firewall": {"on": True, "open": [
                        {"port": 443, "protocol": "tcp", "what": "Local Web Access (e.g., Family Hub, apps)",
                         "detail": "Secure Web", "icon": "web"},
                        {"port": 5353, "protocol": "udp", "what": "Network Discovery (mDNS)",
                         "detail": "Multicast DNS", "icon": "discovery"},
                        {"port": 68, "protocol": "udp", "what": "Network Address Configuration (DHCP)",
                         "detail": "Client DHCP", "icon": "address"}]},
                    "integrity": {"checked": "2026-11-03T04:00:00Z", "clean": True, "changes": []},
                    "certificate": {"fingerprint": "3F:A2:9C:01:7B:E4:58:D0:22:6A:91:CF:4E:B7:05:83:1D:F6:62:A9:0C:7E:B3:54:98:E1:2F:6D:C0:47:8B:15",
                                    "trust_at": "http://zoomriya-7k2q.local/zoomriya-box.crt",
                                    "until": "2027-12-10T10:00:00Z", "problems": []},
                    "apparmor": {"on": True, "enforced": 4, "profiles": [
                        {"name": "box-desktop", "enforced": True}, {"name": "zoomriya-id", "enforced": True},
                        {"name": "avahi-daemon", "enforced": True}, {"name": "nvidia_modprobe", "enforced": True},
                        {"name": "lsb_release", "enforced": False}]}, "secure_boot": "on",
                    "connections": [
                        {"host": "deb.debian.org", "title": "OS Package Server",
                         "what": "System updates & security fixes", "icon": "package",
                         "when": "Every night, and when an admin presses Check now"},
                        {"host": "security.debian.org", "title": "OS Security Fix Server", "what": "Security fixes",
                         "icon": "package", "when": "Every night, and when an admin presses Check now"},
                        {"host": "0.debian.pool.ntp.org", "title": "Time Server",
                         "what": "The time, which certificates and updates depend on", "icon": "clock",
                         "when": "Every so often, to keep the clock right"}]}
        if op == "outbound":
            return {"days": 7, "connections": [
                {"account": "_apt", "who": "OS updates", "destination": "151.101.2.132",
                 "protocol": "tcp", "port": 80, "count": 14, "last": "2026-11-03T03:30:00Z"},
                {"account": "systemd-timesync", "who": "The clock", "destination": "162.159.200.1",
                 "protocol": "udp", "port": 123, "count": 3, "last": "2026-11-02T22:05:00Z"},
                {"account": "zoomriya-maps", "who": "Street map", "destination": "2606:4700::6810:84e5",
                 "protocol": "tcp", "port": 443, "count": 1, "last": "2026-10-31T19:12:00Z"}]}
        if op == "notifications":
            return {"notifications": [
                {"time": "2026-11-03T02:10:00Z", "level": "warning", "kind": "drive",
                 "message": "A drive needs checking: sda."},
                {"time": "2026-11-03T03:33:00Z", "level": "info", "kind": "update",
                 "message": "3 security fixes were installed."}],
                "kinds": ["drive", "update", "restart", "check", "network", "secure", "signin", "access", "apps"]}
        if op == "advanced-access":
            action = fields.get("action")
            if action in ("on", "off"):
                self.access_on = action == "on"
            elif action == "key-add":
                if "PRIVATE" in fields.get("key", ""):
                    raise AgentError("That's a private key. Paste the .pub file instead.")
                self.keys.append({"name": fields.get("name") or "key", "type": "ED25519", "bits": 256,
                                  "fingerprint": "SHA256:Zx9" + str(len(self.keys)) + "aB3kLm7Qp2Wr5Ty8Uv1Io4Pd6Fg",
                                  "added": datetime.now(timezone.utc).isoformat(), "by": fields.get("by", "")})
            elif action == "key-remove":
                self.keys = [k for k in self.keys if k["fingerprint"] != fields.get("fingerprint")]
            elif action == "usb-keys":
                return {"found": [{"name": "ravi-desktop", "type": "ED25519", "bits": 256, "stick": "ANNA",
                                   "fingerprint": "SHA256:Rv7kQ2mLp9Xw4Tz1Bc8Nd5Hf3Gj6Ys0Ua2Ei4Ol7Pq",
                                   "key": "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIRavi ravi@desktop",
                                   "file": "ravi-desktop.pub"}]}
            return self.access()
        if op == "pictures":
            return self._pictures(fields)
        if op == "backups":
            return self._backups(fields)
        if op == "apps":
            action = fields.get("action")
            if action == "usb":
                return {"keys": 1, "found": [
                    {"device": "/dev/sdb1", "stick": "ANNA", "file": "notes-1.2.0.zapp", "size_bytes": 182_000_000,
                     "id": "notes", "name": "Notes", "version": "1.2.0", "description": "The family's notes",
                     "ok": True, "kind": "new"},
                    {"device": "/dev/sdb1", "stick": "ANNA", "file": "photos-1.5.0.zapp", "size_bytes": 1_400_000_000,
                     "id": "photos", "name": "Photos", "version": "1.5.0", "description": "The family's photos and videos",
                     "ok": True, "kind": "update", "installed": "1.4.2"},
                    {"device": "/dev/sdb1", "stick": "ANNA", "file": "photos-1.3.9.zapp", "size_bytes": 1_300_000_000,
                     "id": "photos", "name": "Photos", "version": "1.3.9", "description": "The family's photos and videos",
                     "ok": False, "kind": "older", "newest": "1.4.2",
                     "problem": "Older than 1.4.2, which this box has had."},
                    {"device": "/dev/sdb1", "stick": "ANNA", "file": "games.zapp", "size_bytes": 90_000_000,
                     "ok": False, "problem": "Not a ZOOMRIYA app."}]}
            if action == "install":
                self.job = {"action": "install", "name": "Notes", "version": "1.2.0", "step": "Starting",
                            "done": False, "ok": False}
                self.job_started = time.time()
            elif action == "restore":
                for app in self.apps:
                    if app["id"] == fields.get("name"):
                        app["version"], app["previous"] = app["previous"], app["version"]
            elif action == "remove":
                self.apps = [a for a in self.apps if a["id"] != fields.get("name")]
            if self.job and not self.job["done"]:
                steps = ["Copying from the USB stick", "Checking it's a ZOOMRIYA app", "Adding it to the box",
                         "Starting it"]
                n = int((time.time() - self.job_started) / 2)
                if n < len(steps):
                    self.job["step"] = steps[n]
                else:
                    self.job.update(done=True, ok=True)
                    self.apps.append({"id": "notes", "name": "Notes", "description": "The family's notes",
                                      "version": "1.2.0", "previous": "", "health": "running", "by": "amma",
                                      "installed": datetime.now(timezone.utc).isoformat(), "data": True})
            return {"apps": sorted(self.apps, key=lambda a: a["name"]), "job": self.job, "keys": 1}
        if op == "diagnostics":
            action = fields.get("action")
            answer: dict[str, Any] = {}
            if action == "make":
                time.sleep(1.5)
                now = datetime.now(timezone.utc).replace(microsecond=0)
                name = f"zoomriya-diagnostics-{now:%Y%m%d-%H%M%S}.tar.gz"
                self.bundles.insert(0, {"name": name, "size_bytes": 3_600_000,
                                        "made": now.isoformat().replace("+00:00", "Z"), "errors": 1, "warnings": 1})
                del self.bundles[5:]
                answer["made"] = name
            elif action == "delete":
                self.bundles = [b for b in self.bundles if b["name"] != fields.get("name")]
            elif action == "show":
                return {"name": fields.get("name"), "manifest": {
                    "format": 1, "note": "Photos won't upload since Tuesday.", "redacted": 2,
                    "checks": [{"id": "failed-unit", "severity": "error", "summary": "zoomriya-photos.service failed."},
                               {"id": "drive", "severity": "warning", "summary": "WD Red Plus should be checked soon."},
                               {"id": "restart", "severity": "info", "summary": "A restart is waiting, for the maintenance hour."}],
                    "parts": [{"file": "manifest.json", "what": "What's in the bundle, and the checks' results", "bytes": 4200},
                              {"file": "versions.txt", "what": "ZOOMRIYA OS's release and every package's version", "bytes": 38000},
                              {"file": "status.json", "what": "The OS's version, uptime, restarts waiting", "bytes": 1300},
                              {"file": "disk-health.json", "what": "Each drive's health (SMART)", "bytes": 2100},
                              {"file": "network.json", "what": "The box's network: its ports, addresses, router and name servers", "bytes": 1800, "sensitive": True},
                              {"file": "failed-units.txt", "what": "Services that failed, each with its last log lines", "bytes": 3100},
                              {"file": "journal.txt", "what": "The system log of the last 7 days", "bytes": 9_200_000, "sensitive": True}]}}
            elif action == "read":
                return {"part": fields.get("part"), "cut": False,
                        "text": "2026-11-08T09:59:00+0000 zoomriya-7k2q systemd[1]: zoomriya-photos.service: Failed with result 'exit-code'.\n"
                                "2026-11-08T09:59:01+0000 zoomriya-7k2q photos[812]: connecting with token=[removed]\n"}
            elif action == "export":
                time.sleep(1)
                return {"saved": "ZOOMRIYA/" + fields.get("name", ""), "device": fields.get("device")}
            answer["bundles"] = list(self.bundles)
            answer["usb"] = list(self.sticks)
            return answer
        if op == "display-set":
            return {"output": fields.get("output"), "saved": True}
        if op in ("restart", "shutdown", "restore-point-restore", "maintenance-hour", "factory-reset"):
            return {"ok": True, "maintenance_hour": fields.get("hour", "03:30")}
        return super().ask(op, **fields)


class FakeID:
    """The family as ZOOMRIYA ID keeps it, for People; its rules too (an admin
    can't remove their own role or turn themselves off, a child is never one)."""

    def __init__(self) -> None:
        year = datetime.now().year
        self.family: list[dict[str, Any]] = [
            {"id": 1, "username": "amma", "display_name": "Amma", "role": "admin", "kind": "adult",
             "level": "adult", "birth_year": None, "guardians": [], "is_active": True},
            {"id": 2, "username": "appa", "display_name": "Appa", "role": "family", "kind": "adult",
             "level": "adult", "birth_year": None, "guardians": [], "is_active": True},
            {"id": 3, "username": "kid", "display_name": "Nila", "role": "family", "kind": "child",
             "level": "child", "birth_year": year - 9, "is_active": True,
             "guardians": [{"id": 1, "display_name": "Amma", "is_active": True},
                           {"id": 2, "display_name": "Appa", "is_active": True}]},
            {"id": 4, "username": "paati", "display_name": "Paati", "role": "family", "kind": "adult",
             "level": "adult", "birth_year": None, "guardians": [], "is_active": False},
        ]

    def sign_in(self, username: str, password: str) -> Member:
        time.sleep(0.3)
        if username.strip() == "new" and len(password) >= 8:
            raise NewPasswordNeeded("Choose your own password to finish signing in.")
        # "code" signs in with a second step (123456, or a backup code);
        # "setup" is an admin setting one up at their first sign-in.
        if username.strip() == "code" and len(password) >= 8:
            raise SecondStepNeeded("Enter the code from your authenticator app.", "ticket")
        if username.strip() == "setup" and len(password) >= 8:
            raise SecondStepSetupNeeded("Set up a second sign-in step first.", "ticket")
        if username.strip() == "kid":
            raise SignInError("Box Desktop is for the box's admins.")
        if len(password) < 8:
            raise SignInError("That username and password don't match.")
        return Member(1, username.strip(), username.strip().title(), "token")

    def sign_in_with_new_password(self, username: str, password: str, new_password: str) -> Member:
        time.sleep(0.3)
        if new_password == password:
            raise SignInError("Choose a password you aren't using now.")
        return Member(5, username.strip(), username.strip().title(), "token")

    def sign_in_with_code(self, ticket: str, code: str) -> Member:
        time.sleep(0.3)
        if code.strip() not in ("123456", "ABCDE-FGHJK"):
            raise SignInError("That code isn't right. Check the time on your phone, or use a backup code.")
        return Member(1, "code", "Code", "token")

    def second_step_start(self, ticket: str) -> dict[str, str]:
        time.sleep(0.2)
        # A made-up pattern in a QR code's place: dark on white, with a border.
        cells = "".join(f'<rect x="{4 + x}" y="{4 + y}" width="1" height="1"/>'
                        for y in range(25) for x in range(25)
                        if (x * 7 + y * 13 + x * y) % 3 == 0 or (x < 7 and y < 7) or (x > 17 and y < 7)
                        or (x < 7 and y > 17))
        svg = ('<svg xmlns="http://www.w3.org/2000/svg" width="198" height="198" viewBox="0 0 33 33">'
               f'<rect width="33" height="33" fill="#fff"/><g fill="#000">{cells}</g></svg>')
        return {"qr_svg": svg, "uri": "otpauth://totp/ZOOMRIYA:setup", "key": "JBSW Y3DP EHPK 3PXP JBSW Y3DP EHPK 3PXP"}

    def second_step_finish(self, ticket: str, code: str) -> tuple[Member, list[str]]:
        time.sleep(0.3)
        if code.strip() != "123456":
            raise SignInError("That code isn't right. Check the time on your phone, or use a backup code.")
        codes = ["ABCDE-FGHJK", "LMNPQ-RSTUV", "WXYZ2-34567", "89ABC-DEFGH", "JKLMN-PQRST",
                 "UVWXY-Z2345", "6789A-BCDEF", "GHJKL-MNPQR", "STUVW-XYZ23", "45678-9ABCD"]
        return Member(1, "setup", "Setup", "token"), codes

    def password_is(self, member: Member, password: str) -> bool:
        return len(password) >= 8

    def sign_out(self, member: Member) -> None:
        pass

    def family_activity(self, member: Member, query: str = "") -> dict[str, Any]:
        time.sleep(0.2)
        return {"more": False, "activity": [
            {"id": 3, "at": "2026-11-09T21:05:00Z", "kind": "sign-in-failed", "what": "Wrong password",
             "member": "Appa", "by": None, "source": "ZOOMRIYA ID", "address": "192.168.1.31"},
            {"id": 2, "at": "2026-11-09T19:40:00Z", "kind": "share-link", "what": "Made a guest link: Trip",
             "member": "Amma", "by": None, "source": "Family Photos", "address": "192.168.1.20"},
            {"id": 1, "at": "2026-11-09T08:12:00Z", "kind": "sign-in", "what": "Signed in",
             "member": "Nila", "by": None, "source": "ZOOMRIYA ID", "address": "192.168.1.44"}]}

    def people(self, member: Member) -> list[dict[str, Any]]:
        time.sleep(0.2)
        return [dict(p) for p in self.family]

    def _guardians(self, ids: list[int]) -> list[dict[str, Any]]:
        return [{"id": p["id"], "display_name": p["display_name"], "is_active": p["is_active"]}
                for p in self.family if p["id"] in ids]

    def add_person(self, member: Member, fields: dict[str, Any]) -> dict[str, Any]:
        time.sleep(0.3)
        if any(p["username"].lower() == fields["username"].lower() for p in self.family):
            raise PeopleError("That username is taken. Try another.")
        child = fields.get("kind") == "child"
        person = {"id": max(p["id"] for p in self.family) + 1, "username": fields["username"],
                  "display_name": fields.get("display_name") or fields["username"], "role": "family",
                  "kind": fields.get("kind", "adult"), "level": "child" if child else "adult",
                  "birth_year": fields.get("birth_year"), "is_active": True,
                  "guardians": self._guardians(fields.get("guardian_ids") or []) if child else []}
        self.family.append(person)
        return dict(person)

    def change_person(self, member: Member, person_id: int, fields: dict[str, Any]) -> dict[str, Any]:
        time.sleep(0.3)
        person = next(p for p in self.family if p["id"] == person_id)
        if person_id == member.id and (fields.get("role") == "family" or fields.get("is_active") is False):
            raise PeopleError("You can't do that to your own account. Ask another admin")
        if fields.get("role") == "admin" and person["kind"] == "child":
            raise PeopleError("A child's account can't be an admin")
        for key in ("display_name", "role", "is_active", "birth_year"):
            if key in fields:
                person[key] = fields[key] if key != "display_name" else (fields[key] or person["username"])
        if "guardian_ids" in fields:
            person["guardians"] = self._guardians(fields["guardian_ids"])
        return dict(person)
