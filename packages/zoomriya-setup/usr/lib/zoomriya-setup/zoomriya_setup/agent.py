"""Asking zoomriya-agent: one JSON line out, one back, over its socket (design §7).

The setup app's account is in zoomriya-box, the group the agent answers.
"""

from __future__ import annotations

import json
import socket
from typing import Any

SOCKET = "/run/zoomriya/agent.sock"
BY = "first-setup"


class AgentError(Exception):
    """The agent's refusal or failure, worded for the person at the box."""


class Agent:
    def __init__(self, path: str = SOCKET) -> None:
        self.path = path

    def ask(self, op: str, **fields: Any) -> dict[str, Any]:
        request = {"op": op, "by": BY, **{k: v for k, v in fields.items() if v is not None}}
        try:
            with socket.socket(socket.AF_UNIX, socket.SOCK_STREAM) as conn:
                conn.settimeout(120)
                conn.connect(self.path)
                conn.sendall(json.dumps(request).encode() + b"\n")
                line = conn.makefile("rb").readline()
        except OSError as exc:
            raise AgentError("The box isn't answering yet. Wait a moment and try again.") from exc
        try:
            answer = json.loads(line)
        except ValueError as exc:
            raise AgentError("The box gave an answer the setup can't read.") from exc
        if not answer.get("ok"):
            raise AgentError(answer.get("error") or "The box couldn't do that.")
        return answer
