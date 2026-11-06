"""Box Desktop's sign-in with ZOOMRIYA ID, against a stand-in for its API.

    python3 -m unittest discover -s tests/box-desktop
"""

import json
import sys
import threading
import unittest
from http.server import BaseHTTPRequestHandler, HTTPServer
from pathlib import Path

PACKAGES = Path(__file__).resolve().parents[2] / "packages"
sys.path[:0] = [str(PACKAGES / "box-desktop-app/usr/lib/box-desktop-app"),
                str(PACKAGES / "zoomriya-setup/usr/lib/zoomriya-setup")]

from box_desktop import signin  # noqa: E402
from box_desktop.app import duration  # noqa: E402
from box_desktop.display import Mode, Output, ratio  # noqa: E402
from box_desktop.prefs import Prefs  # noqa: E402
from box_desktop.tools import rate, size, when  # noqa: E402

ACCOUNTS = {"amma": ("a long passphrase", "admin"), "kid": ("kid passphrase", "family")}


class FakeID(BaseHTTPRequestHandler):
    def log_message(self, *args):
        pass

    def reply(self, code, body):
        self.send_response(code)
        self.send_header("Content-Type", "application/json")
        self.end_headers()
        self.wfile.write(json.dumps(body).encode())

    def do_POST(self):
        body = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
        password, _role = ACCOUNTS.get(body["username"], (None, None))
        if self.path.endswith("/auth/login") and password == body["password"]:
            self.reply(200, {"access_token": body["username"], "expires_in": 900})
        else:
            self.reply(401, {"detail": "no"})

    def do_GET(self):
        name = self.headers["Authorization"].removeprefix("Bearer ")
        self.reply(200, {"id": 7, "username": name, "display_name": name.title(),
                         "role": ACCOUNTS[name][1]})


class SignIn(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.server = HTTPServer(("127.0.0.1", 0), FakeID)
        threading.Thread(target=cls.server.serve_forever, daemon=True).start()
        signin.ID_API = f"http://127.0.0.1:{cls.server.server_port}/id/api/v1"

    @classmethod
    def tearDownClass(cls):
        cls.server.shutdown()

    def test_an_admin_signs_in(self):
        member = signin.ZoomriyaID().sign_in(" amma ", "a long passphrase")
        self.assertEqual((member.id, member.username, member.name), (7, "amma", "Amma"))

    def test_a_wrong_password(self):
        with self.assertRaisesRegex(signin.SignInError, "don't match"):
            signin.ZoomriyaID().sign_in("amma", "not it at all")

    def test_only_admins(self):
        with self.assertRaisesRegex(signin.SignInError, "admins"):
            signin.ZoomriyaID().sign_in("kid", "kid passphrase")


class Words(unittest.TestCase):
    def test_how_long_it_has_run(self):
        self.assertEqual(duration(3 * 86400 + 4 * 3600 + 120), "3 days, 4 hours")
        self.assertEqual(duration(86400), "1 day, 0 hours")
        self.assertEqual(duration(600), "10 minutes")


class ToolWords(unittest.TestCase):
    def test_sizes(self):
        self.assertEqual(size(512), "512 bytes")
        self.assertEqual(size(41e9), "41.0 GB")
        self.assertEqual(size(4e12), "4.0 TB")
        self.assertEqual(rate(182_000), "182.0 KB/s")

    def test_times(self):
        self.assertEqual(when(""), "Never")
        self.assertEqual(when("not a time"), "not a time")
        self.assertRegex(when("2026-11-03T03:31:00Z"), r"^\w{3} \d{1,2} Nov 2026, \d{2}:\d{2}$")


class AdminsOwnDesktop(unittest.TestCase):
    def setUp(self):
        import tempfile

        self.folder = Path(tempfile.mkdtemp()) / "admin"
        self.prefs = Prefs(self.folder)

    def test_kept_between_sign_ins(self):
        saved = self.prefs.load(7)
        self.assertEqual(saved["theme"], "midnight")
        saved["theme"] = "daylight"
        saved["windows"] = [{"kind": "storage", "x": 150, "y": 14, "section": "drives"}]
        self.prefs.save(7, saved)
        again = self.prefs.load(7)
        self.assertEqual((again["theme"], again["windows"][0]["section"]), ("daylight", "drives"))
        self.assertEqual(self.prefs.load(8)["theme"], "midnight")

    def test_only_its_own_account_reads_it(self):
        self.prefs.save(7, self.prefs.load(7))
        self.assertEqual(self.prefs.path(7).stat().st_mode & 0o777, 0o600)
        self.assertEqual(self.folder.stat().st_mode & 0o777, 0o700)
        # Named by a hash of the member id, not the id itself.
        import hashlib

        self.assertEqual(self.prefs.path(7).name, hashlib.sha256(b"7").hexdigest() + ".json")

    def test_a_damaged_mouse_is_the_defaults(self):
        self.prefs.save(7, self.prefs.load(7))
        self.prefs.path(7).write_text('{"mouse": {"speed": 9, "accel": "x", "natural": 1}}')
        self.assertEqual(self.prefs.load(7)["mouse"], {"speed": 1.0, "accel": True, "natural": False})
        self.prefs.path(7).write_text('{"mouse": "fast"}')
        self.assertEqual(self.prefs.load(7)["mouse"]["speed"], 0.0)

    def test_a_damaged_file_is_the_defaults(self):
        self.prefs.save(7, self.prefs.load(7))
        self.prefs.path(7).write_text("{not json")
        self.assertEqual(self.prefs.load(7)["windows"], [])
        self.prefs.path(7).write_text('{"theme": "neon", "windows": []}')
        self.assertEqual(self.prefs.load(7)["theme"], "midnight")


class Screen(unittest.TestCase):
    def test_what_it_can_show(self):
        modes = [Mode(1920, 1080, 60000), Mode(2560, 1440, 59951), Mode(1920, 1080, 50000),
                 Mode(2560, 1440, 74971)]
        screen = Output("HDMI-A-1", "Dell", modes, modes[1])
        self.assertEqual(screen.sizes(), [(2560, 1440), (1920, 1080)])
        self.assertEqual(screen.rates(2560, 1440), [74971, 59951])
        self.assertEqual(modes[1].rate, "59.95 Hz")

    def test_ratios(self):
        self.assertEqual(ratio(2560, 1440), "16:9")
        self.assertEqual(ratio(1920, 1200), "16:10")
        self.assertEqual(ratio(1024, 768), "4:3")
        self.assertEqual(ratio(1366, 768), "16:9")
        self.assertEqual(ratio(1000, 999), "")


if __name__ == "__main__":
    unittest.main()
