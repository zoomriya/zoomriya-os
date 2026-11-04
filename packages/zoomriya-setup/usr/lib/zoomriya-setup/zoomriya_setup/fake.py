"""A stand-in for the agent, to run the setup app on a development machine.

    ZOOMRIYA_SETUP_FAKE=1 python3 -m zoomriya_setup

The cable is plugged in a few seconds after the network page opens; the
password "taken" plays an admin that already exists.
"""

from __future__ import annotations

import time
from typing import Any

from .agent import AgentError

LAYOUTS = [
    {"id": "us", "name": "English (US)"},
    {"id": "gb", "name": "English (UK)"},
    {"id": "ca", "name": "French (Canada)"},
    {"id": "fr", "name": "French"},
]


class FakeAgent:
    def __init__(self) -> None:
        self.started = time.monotonic()
        self.layout = "us"
        self.zone = "Etc/UTC"
        self.region = ""
        self.locale = ""
        self.mode = "dhcp"
        self.fixed: dict[str, Any] | None = None
        self.admin: str | None = None
        self.finished = False

    def ask(self, op: str, **fields: Any) -> dict[str, Any]:
        time.sleep(0.2)
        if op == "keyboard":
            return {"layout": self.layout, "layouts": LAYOUTS}
        if op == "keyboard-set":
            self.layout = fields["layout"]
            return {}
        if op == "network":
            return self._network()
        if op == "network-set":
            self.mode = fields["mode"]
            self.fixed = fields if self.mode == "fixed" else None
            return {"link": "enp1s0", "mode": self.mode}
        if op == "network-keep":
            return {"kept": ["enp1s0"]}
        if op == "setup-status":
            return {
                "finished": self.finished,
                "has_admin": self.admin is not None,
                "language": self.locale,
                "region": self.region,
                "zone": self.zone,
                "clock_synced": True,
            }
        if op == "language-set":
            self.locale = fields["locale"]
            return {"language": self.locale}
        if op == "region-set":
            self.region = fields["region"]
            return {"region": self.region}
        if op == "time-zone-set":
            self.zone = fields["zone"]
            return {"zone": self.zone}
        if op == "first-admin":
            if fields["password"] == "taken":
                raise AgentError("ZOOMRIYA ID already has an admin on this box.")
            self.admin = fields["username"].strip()
            return {"username": self.admin}
        if op == "setup-finish":
            self.finished = True
            return {"finished": True}
        raise AgentError(f"Unknown request: {op!r}.")

    def _network(self) -> dict[str, Any]:
        plugged = time.monotonic() - self.started > 4
        address = "192.168.1.57/24"
        if self.fixed:
            address = self.fixed["address"]
        link = {
            "name": "enp1s0",
            "kind": "ethernet",
            "connected": plugged,
            "mac": "52:54:00:12:34:56",
            "speed_mbps": 1000 if plugged else 0,
            "ipv4": [address] if plugged else [],
            "ipv6": [],
            "default": plugged,
            "gateway": "192.168.1.1" if plugged else "",
            "mode": self.mode,
        }
        return {
            "name": "zoomriya-7k2q.local",
            "online": plugged,
            "via": "ethernet" if plugged else "",
            "links": [link],
            "dns": ["192.168.1.1"] if plugged else [],
        }
