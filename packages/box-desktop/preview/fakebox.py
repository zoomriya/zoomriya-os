#!/usr/bin/python3
"""The sample box for Box Desktop's preview (scripts/preview-desktop.sh): the
agent on a Unix socket (one JSON line in, one out, as zoomriya-agent answers)
and ZOOMRIYA ID's API over HTTP, from sample.py, and the apps' pages for
their windows. Nothing on this computer changes.

    fakebox.py SOCKET PORT [HOST]   (HOST: 127.0.0.1, or 0.0.0.0 for --lan)

Sign in with any username and a password of 8 characters or more; "kid" isn't
an admin; "new" has a temporary password; "code" asks for a code (123456);
"setup" sets one up. The setup code for Can't sign in? is K7QM-4XRT.
"""

from __future__ import annotations

import json
import os
import re
import secrets
import socketserver
import sys
import threading
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from typing import Any

from sample import (AgentError, FakeBoxAgent, FakeID, Member, NewPasswordNeeded, PeopleError,
                    SecondStepNeeded, SecondStepSetupNeeded, SignInError)

AGENT = FakeBoxAgent()
ID = FakeID()
SESSION_COOKIE = "zoomriya.auth.session"
#: Each token and session cookie, and who it is.
TOKENS: dict[str, Member] = {}
SESSIONS: dict[str, Member] = {}
LOCK = threading.Lock()
#: Whether the first setup's admin has set up their phone.
PHONE_SET: dict[str, bool] = {}


class AgentHandler(socketserver.StreamRequestHandler):
    def handle(self) -> None:
        line = self.rfile.readline()
        try:
            request = json.loads(line)
            op = request.pop("op")
            request.pop("by", None)
            with LOCK:
                answer = {"ok": True, **AGENT.ask(op, **request)}
            if op == "setup-finish" and os.environ.get("FAKEBOX_SETUP_DONE"):
                # As the agent marks the box set up.
                open(os.environ["FAKEBOX_SETUP_DONE"], "w").close()
        except AgentError as exc:
            answer = {"ok": False, "error": str(exc)}
        except Exception as exc:  # noqa: BLE001 - the sample box answers, whatever it was asked
            answer = {"ok": False, "error": f"The sample box can't do that ({exc})."}
        self.wfile.write(json.dumps(answer).encode() + b"\n")


class UnixServer(socketserver.ThreadingMixIn, socketserver.UnixStreamServer):
    daemon_threads = True


class IDHandler(BaseHTTPRequestHandler):
    def log_message(self, *_a: Any) -> None:
        pass

    def answer(self, status: int, body: Any = None, cookie: str = "") -> None:
        raw = json.dumps(body if body is not None else {}).encode()
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(raw)))
        if cookie:
            self.send_header("Set-Cookie", f"{SESSION_COOKIE}={cookie}; Path=/; HttpOnly; SameSite=Lax")
            self.send_header("Set-Cookie", f"zoomriya.auth.token=t{cookie}; Path=/; HttpOnly; SameSite=Lax")
        self.end_headers()
        self.wfile.write(raw)

    def body(self) -> dict[str, Any]:
        length = int(self.headers.get("Content-Length") or 0)
        return json.loads(self.rfile.read(length) or b"{}") if length else {}

    def me(self) -> Member | None:
        auth = self.headers.get("Authorization", "")
        return TOKENS.get(auth.removeprefix("Bearer "))

    def signed_in(self, member: Member, extra: dict[str, Any] | None = None) -> None:
        token, session = secrets.token_hex(16), secrets.token_hex(16)
        TOKENS[token] = SESSIONS[session] = member
        self.answer(200, {"access_token": token, **(extra or {})}, cookie=session)

    def refused(self, exc: Exception) -> None:
        if isinstance(exc, NewPasswordNeeded):
            self.answer(403, {"code": "new_password_needed"})
        elif isinstance(exc, SecondStepSetupNeeded):
            self.answer(403, {"code": "second_step_setup_needed", "ticket": exc.ticket})
        elif isinstance(exc, SecondStepNeeded):
            self.answer(403, {"code": "second_step_needed", "ticket": exc.ticket})
        elif "admins" in str(exc):
            # A family member: signed in, but /auth/me says they aren't an admin.
            self.signed_in(Member(9, "kid", "Nila", ""))
        elif "aren't using now" in str(exc):
            self.answer(400, {"detail": str(exc)})
        elif "code isn't right" in str(exc):
            self.answer(400, {"detail": str(exc)})
        else:
            self.answer(401, {"detail": str(exc)})

    def do_POST(self) -> None:  # noqa: N802
        path, b = self.path.removeprefix("/id/api/v1"), self.body()
        try:
            if path == "/auth/login":
                if AGENT.admin and b.get("username") == AGENT.admin and not PHONE_SET.get("done"):
                    # The admin the first setup just made: their phone comes next.
                    return self.answer(403, {"code": "second_step_setup_needed", "ticket": "setup"})
                return self.signed_in(ID.sign_in(b.get("username", ""), b.get("password", "")))
            if path == "/auth/login/new-password":
                return self.signed_in(ID.sign_in_with_new_password(b["username"], b["password"], b["new_password"]))
            if path == "/auth/login/second-step":
                return self.signed_in(ID.sign_in_with_code(b["ticket"], b["code"]))
            if path == "/auth/second-step/setup":
                return self.answer(200, ID.second_step_start(b["ticket"]))
            if path == "/auth/second-step/setup/finish":
                member, codes = ID.second_step_finish(b["ticket"], b["code"])
                PHONE_SET["done"] = True
                return self.signed_in(member, {"backup_codes": codes})
        except SignInError as exc:
            return self.refused(exc)
        if path == "/auth/refresh":
            cookie = (self.headers.get("Cookie") or "").partition(f"{SESSION_COOKIE}=")[2].split(";")[0]
            member = SESSIONS.get(cookie)
            if member is None:
                return self.answer(401)
            token = secrets.token_hex(16)
            TOKENS[token] = member
            return self.answer(200, {"access_token": token})
        member = self.me()
        if member is None:
            return self.answer(401)
        if path == "/auth/me/confirm-password":
            return self.answer(200 if ID.password_is(member, b.get("password", "")) else 401)
        if path == "/auth/logout":
            return self.answer(200)
        if path == "/users":
            try:
                return self.answer(201, ID.add_person(member, b))
            except PeopleError as exc:
                return self.answer(409 if "taken" in str(exc) else 400, {"detail": str(exc)})
        self.answer(404)

    def do_GET(self) -> None:  # noqa: N802
        if self.path.startswith("/app/"):
            # An app's pages, as the box would serve them.
            app_id = self.path.split("/")[2]
            app = next((a for a in AGENT.apps if a["id"] == app_id), {"name": app_id})
            raw = AGENT.app_page(app).encode()
            self.send_response(200)
            self.send_header("Content-Type", "text/html; charset=utf-8")
            self.send_header("Content-Length", str(len(raw)))
            self.end_headers()
            self.wfile.write(raw)
            return
        path = self.path.removeprefix("/id/api/v1")
        member = self.me()
        if member is None:
            return self.answer(401)
        if path == "/auth/me":
            admin = member.username != "kid"
            return self.answer(200, {"id": member.id, "username": member.username, "display_name": member.name,
                                     "role": "admin" if admin else "family"})
        if path == "/users":
            return self.answer(200, ID.people(member))
        if path.startswith("/users/activity"):
            return self.answer(200, ID.family_activity(member, path.removeprefix("/users/activity")))
        self.answer(404)

    def do_PATCH(self) -> None:  # noqa: N802
        member = self.me()
        found = re.fullmatch(r"/id/api/v1/users/(\d+)", self.path)
        if member is None or not found:
            return self.answer(401 if member is None else 404)
        try:
            self.answer(200, ID.change_person(member, int(found.group(1)), self.body()))
        except PeopleError as exc:
            self.answer(400, {"detail": str(exc)})


def main() -> int:
    socket_path, port = sys.argv[1], int(sys.argv[2])
    if os.path.exists(socket_path):
        os.unlink(socket_path)
    agent = UnixServer(socket_path, AgentHandler)
    threading.Thread(target=agent.serve_forever, daemon=True).start()
    host = sys.argv[3] if len(sys.argv) > 3 else "127.0.0.1"
    ThreadingHTTPServer((host, port), IDHandler).serve_forever()
    return 0


if __name__ == "__main__":
    sys.exit(main())
