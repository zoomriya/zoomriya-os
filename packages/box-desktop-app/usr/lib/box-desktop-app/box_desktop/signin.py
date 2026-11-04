"""Signing in with the box's ZOOMRIYA ID, as every app does: its own login,
then who the member is. Box Desktop is for the box's admins only."""

from __future__ import annotations

import json
import urllib.error
import urllib.request
from dataclasses import dataclass

ID_API = "http://127.0.0.1:8050/id/api/v1"


class SignInError(Exception):
    """Worded for the person at the box."""


@dataclass
class Member:
    id: int
    username: str
    name: str
    token: str


def _post(path: str, body: dict) -> dict:
    request = urllib.request.Request(
        ID_API + path, data=json.dumps(body).encode(), method="POST",
        headers={"Content-Type": "application/json"},
    )
    return _send(request)


def _get(path: str, token: str) -> dict:
    request = urllib.request.Request(ID_API + path, headers={"Authorization": f"Bearer {token}"})
    return _send(request)


def _send(request: urllib.request.Request) -> dict:
    try:
        with urllib.request.urlopen(request, timeout=15) as response:
            return json.load(response)
    except urllib.error.HTTPError as exc:
        if exc.code == 401:
            raise SignInError("That username and password don't match.") from exc
        if exc.code == 429:
            raise SignInError("Too many tries. Wait a minute, then try again.") from exc
        raise SignInError("ZOOMRIYA ID couldn't sign you in just now.") from exc
    except OSError as exc:
        raise SignInError("ZOOMRIYA ID isn't answering yet. Wait a moment and try again.") from exc


class ZoomriyaID:
    def sign_in(self, username: str, password: str) -> Member:
        token = _post("/auth/login", {"username": username.strip(), "password": password})["access_token"]
        me = _get("/auth/me", token)
        if me.get("role") != "admin":
            raise SignInError("Box Desktop is for the box's admins.")
        return Member(me["id"], me["username"], me.get("display_name") or me["username"], token)

    def sign_out(self, member: Member) -> None:
        try:
            request = urllib.request.Request(
                ID_API + "/auth/logout", method="POST",
                headers={"Authorization": f"Bearer {member.token}"},
            )
            urllib.request.urlopen(request, timeout=5).close()
        except OSError:
            pass  # signed out here either way
