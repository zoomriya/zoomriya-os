"""The first setup app's checks of what the person types (no screen needed).

    python3 -m unittest discover -s tests/setup
"""

import sys
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[2] / "packages/zoomriya-setup/usr/lib/zoomriya-setup"))

from zoomriya_setup import app  # noqa: E402
from zoomriya_setup.agent import AgentError  # noqa: E402
from zoomriya_setup.fake import FakeAgent  # noqa: E402


class FixedAddress(unittest.TestCase):
    def test_the_agents_fields(self):
        self.assertEqual(
            app.fixed_settings(" 192.168.1.40 ", "255.255.255.0", "192.168.1.1", "1.1.1.1, 9.9.9.9"),
            {"mode": "fixed", "address": "192.168.1.40/24", "gateway": "192.168.1.1",
             "dns": ["1.1.1.1", "9.9.9.9"]},
        )

    def test_the_router_names_by_default(self):
        self.assertEqual(app.fixed_settings("10.0.0.5", "255.0.0.0", "10.0.0.1", "")["dns"], ["10.0.0.1"])

    def test_mistakes_are_said_plainly(self):
        for args in [("192.168.1", "255.255.255.0", "192.168.1.1", ""),
                     ("192.168.1.40", "255.255.0.255", "192.168.1.1", "")]:
            with self.assertRaises(ValueError):
                app.fixed_settings(*args)


class Admin(unittest.TestCase):
    def test_a_good_admin(self):
        self.assertIsNone(app.password_problem("amma", "a long passphrase", "a long passphrase"))

    def test_problems(self):
        self.assertIn("username", app.password_problem("  ", "a long passphrase", "a long passphrase"))
        self.assertIn("Choose a password", app.password_problem("amma", "", ""))
        self.assertIn("again", app.password_problem("amma", "a long passphrase", ""))
        self.assertIn("8 characters", app.password_problem("amma", "short", "short"))
        self.assertIn("aren't the same", app.password_problem("amma", "a long passphrase", "a long passphrasf"))


class TimeZones(unittest.TestCase):
    def test_places_not_aliases(self):
        zones = app.time_zones()
        self.assertIn("America/Toronto", zones)
        self.assertIn("UTC", zones)
        self.assertFalse([z for z in zones if z.startswith(("Etc/", "posix/", "US/"))])


class Regions(unittest.TestCase):
    def test_countries_by_name(self):
        names = dict(app.regions())
        self.assertEqual(names["CA"], "Canada")
        self.assertEqual(names["LK"], "Sri Lanka")
        listed = [name for _, name in app.regions()]
        self.assertEqual(listed, sorted(listed))

    def test_a_regions_zones(self):
        self.assertIn("America/Toronto", app.zones_in("CA"))
        self.assertIn("America/Vancouver", app.zones_in("CA"))
        self.assertEqual(app.zones_in("LK"), ["Asia/Colombo"])
        self.assertEqual(app.zones_in("XX"), [])


class Languages(unittest.TestCase):
    def test_named_in_themselves(self):
        names = dict(app.languages())
        self.assertEqual(names["en"], "English")
        self.assertEqual(names["fr"], "Français · French")
        self.assertEqual(names["ta"], "தமிழ் · Tamil")
        self.assertEqual(names["es"], "Español · Spanish")

    def test_the_language_as_written_in_the_region(self):
        self.assertEqual(app.locale_for("ta", "LK"), "ta_LK")
        self.assertEqual(app.locale_for("en", "CA"), "en_CA.UTF-8")
        self.assertEqual(app.locale_for("fr", "CA"), "fr_CA.UTF-8")

    def test_else_its_home(self):
        self.assertEqual(app.locale_for("ta", "CA"), "ta_IN")
        self.assertEqual(app.locale_for("en", "LK"), "en_US.UTF-8")
        self.assertEqual(app.locale_for("de", ""), "de_DE.UTF-8")
        self.assertEqual(app.locale_for("xx", "CA"), "")


class Fake(unittest.TestCase):
    def test_no_region_until_chosen(self):
        agent = FakeAgent()
        self.assertEqual(agent.ask("setup-status")["region"], "")
        agent.ask("region-set", region="LK")
        self.assertEqual(agent.ask("setup-status")["region"], "LK")


    def test_an_admin_already_made(self):
        agent = FakeAgent()
        agent.ask("first-admin", username="amma", password="a long passphrase")
        self.assertTrue(agent.ask("setup-status")["has_admin"])
        with self.assertRaises(AgentError):
            agent.ask("first-admin", username="amma", password="taken")


if __name__ == "__main__":
    unittest.main()
