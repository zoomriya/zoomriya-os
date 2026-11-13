"""The installer's server (design §10): the page the screen shows
(installer-screen, as its own account) and what it asks of the computer,
on 127.0.0.1 only. It runs as root, since it writes the chosen drive; it
lists the drives, checks the computer, installs, and restarts or switches off.
Nothing reaches it from the network: the stick's own system listens on
nothing else, and every change needs the page's exact address as its Origin.
"""

from __future__ import annotations

import json
import mimetypes
import os
import threading
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from typing import Any

from . import i18n
from .disks import MIN_BYTES, Disk, layout_after, size_text
from .i18n import _

ADDRESS = os.environ.get("ZOOMRIYA_INSTALLER_ADDRESS", "127.0.0.1:7445")
#: The page, as built (box-desktop's web/, installer.html).
WEB = Path(os.environ.get("ZOOMRIYA_INSTALLER_WEB", "/usr/share/zoomriya-installer/web"))
MIN_MEMORY = 4 * 1024**3
#: The bars' colours: the drive's partitions now, then ZOOMRIYA OS's.
NOW_COLOURS = ["#8e7cc3", "#e69138", "#6aa84f", "#c27ba0", "#45818e", "#a64d79"]
FREE_COLOUR = "#dfe5ec"
AFTER_COLOURS = {"EFI": "#7fb2ea", "Recovery": "#f0a95b", "ZOOMRIYA OS": "#1765c1"}


def os_name(path: Path = Path("/usr/share/zoomriya-installer/os-release")) -> str:
    """The OS's name as the box will show it, from the os-release the stick
    carries (its PRETTY_NAME); just "ZOOMRIYA OS" without it."""
    try:
        for line in path.read_text().splitlines():
            if line.startswith("PRETTY_NAME="):
                return line.split("=", 1)[1].strip().strip('"') or "ZOOMRIYA OS"
    except OSError:
        pass
    return "ZOOMRIYA OS"


def memory() -> int:
    try:
        for line in Path("/proc/meminfo").read_text().splitlines():
            if line.startswith("MemTotal:"):
                return int(line.split()[1]) * 1024
    except (OSError, ValueError):
        pass
    return 0


def memory_text(size: int) -> str:
    """Memory as it's sold: in whole GB of 1024³ bytes, less what the firmware keeps."""
    return f"{round(size / 1024**3)} GB"


def segments(listed: list[tuple[str, int, str]]) -> list[dict[str, Any]]:
    return [{"name": _(name), "size": size, "size_text": size_text(size), "colour": colour}
            for name, size, colour in listed]


def disk_json(disk: Disk) -> dict[str, Any]:
    now = [(part.what, part.size, NOW_COLOURS[i % len(NOW_COLOURS)]) for i, part in enumerate(disk.parts)]
    free = disk.size - sum(part.size for part in disk.parts)
    if free > disk.size * 0.01:
        now.append((_("Free space") if disk.parts else _("Empty"), free, FREE_COLOUR))
    after = [(name, size, AFTER_COLOURS[name]) for name, size in layout_after(disk.size)]
    return {
        "path": disk.path, "title": disk.title, "kind": disk.kind, "bus": disk.bus,
        "size": disk.size, "size_text": size_text(disk.size), "usable": disk.usable, "reason": disk.reason,
        "parts": [part.what for part in disk.parts], "now": segments(now), "after": segments(after),
    }


class Installer:
    """What the page sees: the drives, the internet, and the install."""

    def __init__(self, backend: Any) -> None:
        self.backend = backend
        self.lock = threading.Lock()
        self.disks: list[Disk] = []
        self.looking = False
        self.online: bool | None = None
        self.install: dict[str, Any] = {"running": False, "done": False, "problem": "", "stage": 0, "fraction": 0.0}
        self.look()
        threading.Thread(target=self.watch_internet, daemon=True).start()

    def look(self) -> None:
        """The drives again (a stick plugged in later)."""
        def run() -> None:
            try:
                found = self.backend.list_disks()
            except Exception:  # noqa: BLE001 - shown as no drives
                found = []
            with self.lock:
                self.disks, self.looking = found, False

        with self.lock:
            self.looking = True
        threading.Thread(target=run, daemon=True).start()

    def watch_internet(self) -> None:
        """Looks for the internet, again every few seconds until it's there (a
        cable plugged in later). It never stops the install."""
        import time

        while True:
            self.online = bool(self.backend.online())
            if self.online:
                return
            time.sleep(5)

    def state(self) -> dict[str, Any]:
        from .install import LOG, STAGES, overall

        fake = bool(os.environ.get("ZOOMRIYA_INSTALLER_FAKE"))
        ram = memory()
        with self.lock:
            disks = list(self.disks)
            install = dict(self.install)
            looking = self.looking
        stage = int(install["stage"])
        install["overall"] = overall(stage, float(install["fraction"]))
        install["stage_name"] = _(STAGES[stage][0])
        return {
            "language": i18n.language, "os": os_name(), "looking": looking, "online": self.online,
            "log": str(LOG),
            "requirements": [
                {"words": _("Started in UEFI mode"), "ok": fake or Path("/sys/firmware/efi").is_dir()},
                {"words": _("Memory: {have} ({need} needed)").format(have=memory_text(ram), need=memory_text(MIN_MEMORY)),
                 "ok": fake or ram >= MIN_MEMORY * 0.95},
                {"words": _("A drive of {size} or more").format(size=size_text(MIN_BYTES)), "ok": any(d.usable for d in disks)},
            ],
            "disks": [disk_json(d) for d in disks],
            "install": install,
        }

    def start(self, path: str) -> str:
        """Erases the drive and installs: "" when started, else why not."""
        from .install import InstallError

        with self.lock:
            disk = next((d for d in self.disks if d.path == path), None)
            if self.install["running"]:
                return _("ZOOMRIYA OS is being installed already.")
            if disk is None or not disk.usable:
                return disk.reason if disk else _("That drive isn't here any more. Press Look again.")
            self.install = {"running": True, "done": False, "problem": "", "stage": 0, "fraction": 0.0}

        def progress(stage: int, fraction: float) -> None:
            with self.lock:
                self.install.update(stage=stage, fraction=fraction)

        def run() -> None:
            try:
                self.backend.install(path, progress)
                problem = ""
            except InstallError as exc:
                problem = str(exc)
            except Exception as exc:  # noqa: BLE001 - shown to the person
                problem = _("The install stopped: {problem}").format(problem=exc)
            with self.lock:
                self.install.update(running=False, done=True, problem=problem)

        threading.Thread(target=run, daemon=True).start()
        return ""


def handler(installer: Installer) -> type[BaseHTTPRequestHandler]:
    origin = f"http://{ADDRESS}"

    class Handler(BaseHTTPRequestHandler):
        def log_message(self, *_a: Any) -> None:
            pass

        def send(self, status: int, body: bytes, kind: str, cache: str = "no-store") -> None:
            self.send_response(status)
            self.send_header("Content-Type", kind)
            self.send_header("Content-Length", str(len(body)))
            self.send_header("Cache-Control", cache)
            self.send_header("X-Content-Type-Options", "nosniff")
            self.send_header("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; "
                             "img-src 'self' data:; font-src 'self'; connect-src 'self'; frame-ancestors 'none'")
            self.end_headers()
            self.wfile.write(body)

        def json(self, status: int, value: Any) -> None:
            self.send(status, json.dumps(value).encode(), "application/json")

        def allowed(self) -> bool:
            if self.headers.get("Host") != ADDRESS:
                self.send(421, b"", "text/plain")
                return False
            return True

        def do_GET(self) -> None:  # noqa: N802
            if not self.allowed():
                return
            path = self.path.split("?")[0]
            if path == "/api/state":
                self.json(200, installer.state())
                return
            name = "installer.html" if path in ("/", "/index.html") else path.lstrip("/")
            file = (WEB / name).resolve()
            if not str(file).startswith(str(WEB.resolve())) or not file.is_file():
                self.send(404, b"", "text/plain")
                return
            kind = mimetypes.guess_type(file.name)[0] or "application/octet-stream"
            cache = "public, max-age=31536000, immutable" if name.startswith("assets/") else "no-cache"
            self.send(200, file.read_bytes(), kind, cache)

        def do_POST(self) -> None:  # noqa: N802
            if not self.allowed():
                return
            if self.headers.get("Origin") != origin or not (self.headers.get("Content-Type") or "").startswith("application/json"):
                self.json(403, {"code": "wrong_origin"})
                return
            length = min(int(self.headers.get("Content-Length") or 0), 65536)
            try:
                body = json.loads(self.rfile.read(length) or b"{}")
            except ValueError:
                body = {}
            if self.path == "/api/language":
                code = str(body.get("code", ""))
                if code in dict(i18n.LANGUAGES):
                    i18n.use(code)
                    # The drives' words (why one can't be chosen) in it too.
                    installer.look()
                self.json(200, installer.state())
            elif self.path == "/api/look":
                installer.look()
                self.json(200, {"ok": True})
            elif self.path == "/api/install":
                problem = installer.start(str(body.get("disk", "")))
                self.json(409 if problem else 200, {"error": problem} if problem else {"ok": True})
            elif self.path == "/api/restart":
                self.json(200, {"ok": True})
                installer.backend.restart()
            elif self.path == "/api/power-off":
                self.json(200, {"ok": True})
                installer.backend.power_off()
            else:
                self.json(404, {"code": "not_found"})

    return Handler


def serve(backend: Any) -> None:
    host, port = ADDRESS.rsplit(":", 1)
    ThreadingHTTPServer((host, int(port)), handler(Installer(backend))).serve_forever()
