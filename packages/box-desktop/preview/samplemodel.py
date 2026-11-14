"""A stand-in for the box's model service (zoomriya-model) in Box Desktop's
preview, so Myira's panel can be tried with no model and no ZOOMRIYA ID keys:
picks by a few words, and a fixed sentence for each answer, said to be the
preview's. FAKEBOX_NO_MODEL=1: no model yet, to try Download, which "fetches"
it in about twenty seconds."""

from __future__ import annotations

import json
import os
import re
import threading
import time
from typing import Any

#: The model is there unless the preview starts without one.
STATE: dict[str, Any] = {"loaded": os.environ.get("FAKEBOX_NO_MODEL") != "1", "started": 0.0, "busy": ""}
LOCK = threading.Lock()
TOTAL = 3427880384 + 986833248

#: The question's words, and what the box's model would pick (Support Center's
#: Help topics by their place in the page's list).
PICKS: list[tuple[str, dict[str, Any]]] = [
    (r"erase|wipe|reset|efface", {"pick": "open:0"}),
    (r"shut|reboot|restart (?:it|the box)? ?now|éteins|redémarre", {"pick": "open:1"}),
    (r"\bssh\b", {"pick": "open:2"}),
    (r"admin", {"pick": "open:3"}),
    (r"full|disk|storage|space|plein|disque|stockage", {"pick": "box.storage"}),
    (r"backup|sauvegarde", {"pick": "box.backups"}),
    (r"restart (?:time|hour)|maintenance|heure", {"pick": "box.set_maintenance_hour", "arguments": {"hour": "04:30"}}),
    (r"volume|louder|quieter", {"pick": "box.set_volume", "arguments": {"percent": 30}}),
    (r"slow|lent", {"pick": "help:2"}),
    (r"restarted|redémarré", {"pick": "help:3"}),
    (r"update|mise à jour", {"pick": "help:0"}),
    (r"drive", {"pick": "help:1"}),
    (r"sign in|connecter", {"pick": "help:5"}),
    (r"network|address|ip\b|réseau|adresse", {"pick": "box.network"}),
]


def loaded() -> bool:
    with LOCK:
        if not STATE["loaded"] and STATE["busy"] and time.time() - STATE["started"] > 20:
            STATE["loaded"], STATE["busy"] = True, ""
        return STATE["loaded"]


def current() -> dict[str, Any]:
    on = loaded()
    return {"file": "gemma-4-E2B-it-Q4_K_M.gguf", "loaded": "gemma-4-E2B-it-Q4_K_M.gguf" if on else None, "sees": on}


def pick(question: str) -> str:
    for words, chosen in PICKS:
        if re.search(words, question, re.IGNORECASE):
            return json.dumps(chosen)
    return json.dumps({"pick": "other"})


def answer_lines(system: str) -> list[bytes]:
    title = (re.search(r'tool "([^"]+)"', system) or [None, "the box"])[1]
    french = "Reply in French" in system
    sentence = (f"Exemple de réponse de {title} : dans l'aperçu, cette phrase est fixe."
                if french else f"A sample answer from {title}: in the preview, this sentence is fixed.")
    lines = [json.dumps({"text": word + " "}).encode() + b"\n" for word in sentence.split(" ")]
    return lines + [b'{"done": true}\n']


def agent_model(fields: dict[str, Any]) -> dict[str, Any]:
    """The agent's model request, as the sample box answers it."""
    action = fields.get("action")
    with LOCK:
        if action == "download" and not STATE["loaded"] and not STATE["busy"]:
            STATE["busy"], STATE["started"] = "download", time.time()
        if action == "usb":
            return {"found": [{"stick": "ANNA", "device": "/dev/sdb1", "name": "e2b"}]}
        if action == "copy" and not STATE["loaded"] and not STATE["busy"]:
            STATE["busy"], STATE["started"] = "copy", time.time()
    on = loaded()
    with LOCK:
        part = min(1.0, (time.time() - STATE["started"]) / 20) if STATE["busy"] else (1.0 if on else 0.0)
        return {"models": [{"name": "e2b", "complete": on, "bytes": int(TOTAL * part), "total": TOTAL},
                           {"name": "e4b", "complete": False, "bytes": 0, "total": 5335291936 + 991551840}],
                "busy": STATE["busy"]}
