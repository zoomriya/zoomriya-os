"""Box Desktop's own lines in the box's journal: admins' sign-ins, for Log
Center's Sign-ins (the agent's logs, kind "signins"). Written with the
journal's own fields (SYSLOG_IDENTIFIER=box-desktop, ZOOMRIYA_SIGNIN=1,
ZOOMRIYA_BY), over its socket; away from the box, nowhere."""

from __future__ import annotations

import socket

JOURNAL = "/run/systemd/journal/socket"


def _clean(text: str) -> str:
    return " ".join(str(text).split())[:200]


def note_sign_in(username: str, signed_in: bool, why: str = "") -> None:
    who = _clean(username) or "someone"
    message = (f"{who} signed in on the box's screen" if signed_in
               else f"A sign-in as {who} on the box's screen was refused: {_clean(why)}")
    fields = {
        "MESSAGE": message,
        "PRIORITY": "6" if signed_in else "5",
        "SYSLOG_IDENTIFIER": "box-desktop",
        "ZOOMRIYA_SIGNIN": "1",
        "ZOOMRIYA_BY": who,
    }
    data = "".join(f"{key}={value}\n" for key, value in fields.items()).encode()
    try:
        with socket.socket(socket.AF_UNIX, socket.SOCK_DGRAM) as journal:
            journal.sendto(data, JOURNAL)
    except OSError:
        pass
