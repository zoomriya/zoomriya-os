"""The first setup, on the box's own screen (design §5).

Sitting in front of the box is what proves it's yours, so nothing here goes
over the network: the language, the keyboard, the Ethernet connection, the country or region
and the time zone, then the box's admin in ZOOMRIYA ID, which every ZOOMRIYA app on the box shares.
Each step is done on the box as soon as it's confirmed, and the setup asks the
agent where things stand when it starts, so a power cut loses nothing.
"""

from __future__ import annotations

import functools
import ipaddress
import json
import math
import os
import socket
import threading
import zoneinfo
from collections.abc import Callable
from datetime import datetime
from pathlib import Path
from typing import Any

import gi

gi.require_version("Gtk", "4.0")
gi.require_version("Gdk", "4.0")
gi.require_foreign("cairo")
from gi.repository import Gdk, Gio, GLib, Gtk  # noqa: E402

from .agent import Agent, AgentError  # noqa: E402
from . import i18n  # noqa: E402
from .brand import MARK, WORDMARK, logo  # noqa: E402
from .i18n import N_, _  # noqa: E402
from .screen import set_keyboard  # noqa: E402

STEPS = [N_("Language"), N_("Keyboard"), N_("Network"), N_("Region"), N_("Time zone"), N_("Admin"),
         N_("Done")]
LANGUAGE, KEYBOARD, NETWORK, REGION, ZONE, ADMIN, DONE = range(len(STEPS))
#: Reached to say whether the box is online; the setup goes on without it.
INTERNET_CHECK = ("deb.debian.org", 443)
#: tzdata's countries and regions, and the zones in each.
COUNTRIES = Path("/usr/share/zoneinfo/iso3166.tab")
#: The same by their ISO names, which iso-codes translates (i18n.country), and
#: their states and provinces.
ISO_COUNTRIES = Path("/usr/share/iso-codes/json/iso_3166-1.json")
ISO_SUBDIVISIONS = Path("/usr/share/iso-codes/json/iso_3166-2.json")
COUNTRY_ZONES = Path("/usr/share/zoneinfo/zone.tab")
#: Debian's locales (the locales package): those it can build.
SUPPORTED = Path("/usr/share/i18n/SUPPORTED")
#: Home locales of languages without an xx_XX one (fr_FR, de_DE), for a
#: language whose region has no locale of its own.
HOME_LOCALES = {
    "ar": "ar_EG", "bn": "bn_BD", "ca": "ca_ES", "cs": "cs_CZ", "da": "da_DK", "el": "el_GR",
    "en": "en_US", "et": "et_EE", "eu": "eu_ES", "fa": "fa_IR", "ga": "ga_IE", "gl": "gl_ES",
    "gu": "gu_IN", "he": "he_IL", "hi": "hi_IN", "hy": "hy_AM", "ja": "ja_JP", "ka": "ka_GE",
    "kk": "kk_KZ", "kn": "kn_IN", "ko": "ko_KR", "ml": "ml_IN", "mr": "mr_IN", "ms": "ms_MY",
    "nb": "nb_NO", "ne": "ne_NP", "nn": "nn_NO", "pa": "pa_IN", "si": "si_LK", "sl": "sl_SI",
    "sq": "sq_AL", "sv": "sv_SE", "sw": "sw_KE", "ta": "ta_IN", "te": "te_IN", "uk": "uk_UA",
    "ur": "ur_PK", "vi": "vi_VN", "zh": "zh_CN",
}
#: Steps done, so a setup cut short goes on from where it was (systemd's
#: StateDirectory).
PROGRESS = Path(os.environ.get("STATE_DIRECTORY", "/nonexistent").split(":")[0]) / "progress.json"


def progress() -> dict[str, Any]:
    try:
        return json.loads(PROGRESS.read_text())
    except (OSError, ValueError):
        return {}


def save_progress(**done: Any) -> None:
    try:
        PROGRESS.write_text(json.dumps({**progress(), **done}))
    except OSError:
        pass  # away from the box: nothing to keep
MIN_PASSWORD = 8
#: Narrower than this, the side shows the steps' numbers only.
COMPACT_WIDTH = 900


def internet_reachable() -> bool:
    try:
        socket.create_connection(INTERNET_CHECK, timeout=5).close()
        return True
    except OSError:
        return False


def tab_rows(path: Path) -> list[list[str]]:
    try:
        lines = path.read_text().splitlines()
    except OSError:
        return []
    return [line.split("\t") for line in lines if line and not line.startswith("#")]


@functools.cache
def iso_names() -> dict[str, str]:
    """Each country's ISO name by its code, as iso-codes' translations know it."""
    try:
        listed = json.loads(ISO_COUNTRIES.read_text()).get("3166-1", [])
    except (OSError, ValueError):
        return {}
    return {c["alpha_2"]: c["name"] for c in listed if "alpha_2" in c and "name" in c}


def regions() -> list[tuple[str, str]]:
    """Countries and regions as (code, name) in the language in use, by name;
    tzdata's English name where iso-codes has none."""
    names = iso_names()

    def name(code: str, english: str) -> str:
        iso = names.get(code)
        return i18n.country(iso) if iso else english

    return sorted(((row[0], name(row[0], row[1])) for row in tab_rows(COUNTRIES) if len(row) > 1),
                  key=lambda region: region[1].casefold())


@functools.cache
def all_subdivisions() -> list[dict[str, str]]:
    try:
        return json.loads(ISO_SUBDIVISIONS.read_text()).get("3166-2", [])
    except (OSError, ValueError):
        return []


def subdivisions(region: str) -> list[tuple[str, str]]:
    """A country's states or provinces as (code, name) in the language in use,
    by name: its top level only (France's regions, not its departments), as
    the agent's region-set takes them."""
    found = [(s["code"], i18n.subdivision(s["name"])) for s in all_subdivisions()
             if s.get("code", "").startswith(region + "-") and not s.get("parent")]
    return sorted(found, key=lambda sub: sub[1].casefold())


def zones_in(region: str) -> list[str]:
    return sorted(row[2] for row in tab_rows(COUNTRY_ZONES) if len(row) > 2 and row[0] == region)


@functools.cache
def utf8_locales() -> list[str]:
    """The locales Debian can build in UTF-8, without variants (@...), nor
    C.UTF-8, which is no language."""
    rows = [row[0].split() for row in tab_rows(SUPPORTED)]
    return [row[0] for row in rows if len(row) == 2 and row[1] == "UTF-8" and "@" not in row[0]
            and not row[0].startswith("C.")]


def languages() -> list[tuple[str, str]]:
    """The languages the box offers, as (Debian's code, name in itself): those
    our screens are translated into (i18n.OURS)."""
    return [("en", "English"), ("fr", "Français"), ("de", "Deutsch"), ("es", "Español"),
            ("ta", "தமிழ்"), ("ja", "日本語"), ("zh", "简体中文")]


def main_locale(code: str, locales: list[str]) -> str:
    """The language's home locale (fr_FR, de_DE), else its first."""
    home_name = HOME_LOCALES.get(code, f"{code}_{code.upper()}")
    home = [loc for loc in locales if loc.split(".")[0] == home_name]
    return (home or sorted(locales))[0]


def locale_for(language: str, region: str) -> str:
    """The language as written in the region (ta_LK), else its home locale."""
    locales = [loc for loc in utf8_locales() if loc.split("_")[0] == language]
    if not locales:
        return ""
    here = [loc for loc in locales if loc.split(".")[0] == f"{language}_{region}"]
    return here[0] if here else main_locale(language, locales)


def time_zones() -> list[str]:
    """Places people pick (Area/City), not the database's aliases."""
    areas = ("Africa", "America", "Antarctica", "Asia", "Atlantic", "Australia",
             "Europe", "Indian", "Pacific")
    zones = [z for z in zoneinfo.available_timezones() if z.split("/")[0] in areas]
    return sorted(zones) + ["UTC"]


def fixed_settings(address: str, mask: str, router: str, dns: str) -> dict[str, Any]:
    """The agent's network-set fields, or ValueError worded for the person."""
    try:
        bits = ipaddress.IPv4Network(f"0.0.0.0/{mask.strip()}").prefixlen
    except ValueError:
        raise ValueError(_("The subnet mask should look like 255.255.255.0.")) from None
    try:
        ipaddress.IPv4Address(address.strip())
    except ValueError:
        raise ValueError(_("The box's address should look like 192.168.1.40.")) from None
    servers = [s for s in dns.replace(",", " ").split() if s]
    return {
        "mode": "fixed",
        "address": f"{address.strip()}/{bits}",
        "gateway": router.strip(),
        "dns": servers or [router.strip()],
    }


def password_problem(username: str, password: str, again: str) -> str | None:
    if not username.strip():
        return _("Choose a username.")
    if not password:
        return _("Choose a password.")
    if len(username.strip()) > 128:
        return _("The username can be at most 128 characters.")
    if len(password) < MIN_PASSWORD:
        return _("The password needs at least {count} characters.").format(count=MIN_PASSWORD)
    if not again:
        return _("Type the password again.")
    if password != again:
        return _("The two passwords aren't the same.")
    return None


def label(text: str, *classes: str, wrap: bool = True) -> Gtk.Label:
    widget = Gtk.Label(label=text, xalign=0, wrap=wrap)
    if wrap:
        # Lines a person can read across, however wide the screen.
        widget.set_max_width_chars(64)
    for name in classes:
        widget.add_css_class(name)
    return widget


def draw_done(_area: Any, cr: Any, w: int, h: int) -> None:
    """A green tick in a circle, as the installer's Done."""
    green = (0x1f / 255, 0x9d / 255, 0x55 / 255)
    cr.arc(w / 2, h / 2, w / 2, 0, 2 * math.pi)
    cr.set_source_rgba(*green, 0.15)
    cr.fill()
    cr.arc(w / 2, h / 2, w / 2 - 12, 0, 2 * math.pi)
    cr.set_source_rgb(*green)
    cr.fill()
    cr.set_source_rgb(1, 1, 1)
    cr.set_line_width(6)
    cr.set_line_cap(1)  # round
    cr.set_line_join(1)
    cr.move_to(w * 0.33, h * 0.52)
    cr.line_to(w * 0.45, h * 0.64)
    cr.line_to(w * 0.68, h * 0.38)
    cr.stroke()


def field(caption: str, entry: Gtk.Widget) -> Gtk.Box:
    box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=6)
    box.append(label(caption, "field-label"))
    box.append(entry)
    return box


class SetupWindow(Gtk.ApplicationWindow):
    def __init__(self, app: Gtk.Application, agent: Any, chosen: str = "") -> None:
        """chosen: the language just picked, when the setup is drawn again in it."""
        super().__init__(application=app, title=_("ZOOMRIYA Box setup"))
        self.chosen = chosen
        self.agent = agent
        self.step = 0
        self.network: dict[str, Any] = {}
        self.online: bool | None = None
        self.admin = ""
        self.poll: int | None = None
        self.language_choice = ""
        self.region_choice = ""
        self.zone_choice = ""

        # The installer's layout (zoomriya-installer): the steps down a blue
        # side, the page beside it, the buttons along the bottom.
        self.compact = False
        self.whole = whole = Gtk.Box()
        whole.add_css_class("setup")
        self.set_child(whole)
        whole.append(self.sidebar())

        main = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, hexpand=True)
        whole.append(main)
        self.stack = Gtk.Stack(transition_type=Gtk.StackTransitionType.CROSSFADE, vexpand=True,
                               hhomogeneous=False, vhomogeneous=False)
        for step, page in enumerate([self.language_page(), self.keyboard_page(),
                                     self.network_page(), self.region_page(),
                                     self.zone_page(), self.admin_page(), self.done_page()]):
            page.add_css_class("content")
            self.stack.add_named(page, str(step))
        main.append(self.stack)

        buttons = Gtk.Box(spacing=10)
        buttons.add_css_class("buttons")
        self.error = label("", "error")
        self.error.set_hexpand(True)
        self.error.set_visible(False)
        buttons.append(self.error)
        buttons.append(Gtk.Box(hexpand=True))
        self.back = Gtk.Button(label=_("Back"))
        self.back.connect("clicked", lambda _b: self.go(self.step - 1))
        self.next = Gtk.Button(label=_("Next"))
        self.next.add_css_class("primary")
        self.next.connect("clicked", lambda _b: self.confirm())
        self.busy = Gtk.Spinner()
        buttons.append(self.busy)
        buttons.append(self.back)
        buttons.append(self.next)
        main.append(buttons)

        if chosen:
            # Drawn again in a language just picked: still on Language, with it chosen.
            self.select_language(chosen)
            self.go(LANGUAGE)
        else:
            self.call(lambda: self.agent.ask("setup-status"), self.resume, lambda _e: self.go(LANGUAGE))

    # --- the side and moving between steps -----------------------------------

    def sidebar(self) -> Gtk.Widget:
        self.side = side = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=6)
        side.add_css_class("sidebar")
        side.set_size_request(280, -1)
        #: Hidden on a small screen, leaving the mark and the steps' numbers.
        self.side_words: list[Gtk.Widget] = []

        brand = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=6)
        brand.add_css_class("brand-row")
        wordmark = logo(WORDMARK, 22)
        brand.append(wordmark)
        self.side_words.append(wordmark)
        #: The mark alone, where the wordmark doesn't fit.
        self.side_mark = logo(MARK, 26)
        self.side_mark.set_visible(False)
        brand.append(self.side_mark)
        sub = label(_("Box setup"), "brand-sub", wrap=False)
        brand.append(sub)
        self.side_words.append(sub)
        side.append(brand)

        self.step_rows: list[tuple[Gtk.Box, Gtk.Label]] = []
        for i, name in enumerate(STEPS):
            row = Gtk.Box(spacing=12)
            row.add_css_class("step")
            marker = label(str(i + 1), "step-marker", wrap=False)
            marker.set_xalign(0.5)
            row.append(marker)
            words = label(_(name), "step-name", wrap=False)
            row.append(words)
            self.side_words.append(words)
            side.append(row)
            self.step_rows.append((row, marker))
        return side

    def do_size_allocate(self, width: int, height: int, baseline: int) -> None:
        Gtk.ApplicationWindow.do_size_allocate(self, width, height, baseline)
        compact = width < COMPACT_WIDTH
        if compact != self.compact:
            self.compact = compact
            # Changed once this layout is done, not during it.
            GLib.idle_add(self.lay_out)

    def lay_out(self) -> bool:
        for widget in self.side_words:
            widget.set_visible(not self.compact)
        self.side_mark.set_visible(self.compact)
        self.side.set_size_request(76 if self.compact else 280, -1)
        if self.compact:
            self.whole.add_css_class("compact")
        else:
            self.whole.remove_css_class("compact")
        return False

    # --- moving between steps -------------------------------------------------

    def resume(self, status: dict[str, Any]) -> None:
        """Where a setup cut short (a power cut) left off. The language, the
        region and the time zone are chosen by the person, never assumed:
        until they've been chosen, nothing is selected."""
        done = progress()
        if done.get("language"):
            self.select_language(done["language"])
        if status.get("region"):
            self.select_region(status["region"])
            self.show_subdivisions(status["region"], status.get("subdivision", ""))
        if done.get("zone") and status.get("zone"):
            self.zone_choice = status["zone"]
        if status.get("has_admin"):
            self.go(DONE)
        else:
            self.go(NETWORK if done.get("keyboard") else KEYBOARD if done.get("language") else LANGUAGE)

    def go(self, step: int) -> None:
        self.step = max(0, min(step, len(STEPS) - 1))
        self.stack.set_visible_child_name(str(self.step))
        self.say("")
        for i, (row, marker) in enumerate(self.step_rows):
            for name in ("current", "done"):
                row.remove_css_class(name)
            if i == self.step:
                row.add_css_class("current")
            elif i < self.step:
                row.add_css_class("done")
            marker.set_text("✓" if i < self.step else str(i + 1))
        self.back.set_visible(LANGUAGE < self.step < DONE)
        self.next.set_label(_("Finish") if self.step == DONE else _("Next"))
        if self.poll is not None:
            GLib.source_remove(self.poll)
            self.poll = None
        if self.step == KEYBOARD:
            self.call(lambda: self.agent.ask("keyboard"), self.show_layouts)
        if self.step == NETWORK:
            self.refresh_network()
            self.poll = GLib.timeout_add_seconds(2, self.refresh_network)
        if self.step == REGION:
            self.region_search.grab_focus()
        if self.step == ZONE:
            self.show_zones()
        if self.step == ADMIN:
            self.username.grab_focus()
        if self.step == DONE:
            self.show_done()
        self.update_next()

    def confirm(self) -> None:
        if self.step == LANGUAGE:
            language, locale = self.language_choice, locale_for(self.language_choice, self.region_choice)

            def language_set(_r: Any) -> None:
                save_progress(language=language)
                self.go(KEYBOARD)

            self.call(lambda: self.agent.ask("language-set", locale=locale), language_set)
        elif self.step == KEYBOARD:
            layout = self.layout_ids[self.layouts.get_selected()] if self.layout_ids else None

            def done(_r: Any) -> None:
                save_progress(keyboard=True)
                # On the screen at once, so the admin's password is typed with it.
                if layout:
                    set_keyboard(layout)
                self.go(NETWORK)

            self.call(lambda: self.agent.ask("keyboard-set", layout=layout) if layout else {}, done)
        elif self.step == NETWORK:
            self.call(self.keep_network, lambda _r: self.go(REGION))
        elif self.step == REGION:
            region, sub = self.region_choice, self.subdivision_choice()
            # The language as written there, if the region has its own: Tamil
            # in Sri Lanka is ta_LK, with Sri Lanka's dates and numbers.
            locale = locale_for(self.language_choice, region)

            def region_set() -> dict[str, Any]:
                answer = self.agent.ask("region-set", region=region, subdivision=sub)
                if locale:
                    self.agent.ask("language-set", locale=locale)
                return answer

            self.call(region_set, lambda _r: self.go(ZONE))
        elif self.step == ZONE:
            zone = self.zone_choice

            def zone_set(_r: Any) -> None:
                save_progress(zone=True)
                self.go(ADMIN)

            self.call(lambda: self.agent.ask("time-zone-set", zone=zone), zone_set)
        elif self.step == ADMIN:
            self.make_admin()
        elif self.step == DONE:
            self.call(lambda: self.agent.ask("setup-finish"), lambda _r: self.get_application().quit())

    def update_next(self) -> None:
        ready = True
        if self.step == LANGUAGE:
            ready = bool(self.language_choice)
        elif self.step == NETWORK:
            ready = bool(self.address())
        elif self.step == REGION:
            ready = bool(self.region_choice)
        elif self.step == ZONE:
            ready = bool(self.zone_choice)
        elif self.step == ADMIN:
            problem = password_problem(self.username.get_text(), self.password.get_text(),
                                       self.again.get_text())
            # What's still missing, so a Next that won't press says why.
            self.admin_missing.set_text(problem or "")
            ready = problem is None
        self.next.set_sensitive(ready and not self.busy.get_spinning())

    def say(self, text: str) -> None:
        self.error.set_text(text)
        self.error.set_visible(bool(text))

    def call(self, work: Callable[[], Any], done: Callable[[Any], None],
             failed: Callable[[str], None] | None = None) -> None:
        """Runs work away from the screen, then done (or shows the error)."""
        self.busy.start()
        self.next.set_sensitive(False)

        def finish(result: Any, problem: str | None) -> bool:
            self.busy.stop()
            self.update_next()
            if problem is None:
                done(result)
            elif failed is not None:
                failed(problem)
            else:
                self.say(problem)
            return False

        def run() -> None:
            try:
                result, problem = work(), None
            except AgentError as exc:
                result, problem = None, str(exc)
            GLib.idle_add(finish, result, problem)

        threading.Thread(target=run, daemon=True).start()

    # --- Language -------------------------------------------------------------

    def language_page(self) -> Gtk.Widget:
        box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=14)
        box.append(label(_("Welcome to your ZOOMRIYA Box"), "title"))
        box.append(label(_("Let's set it up. It takes a few minutes: the language, the keyboard, "
                           "the network, the country or region, the time zone, and the box's "
                           "admin. First, the box's language."), "lead"))
        # Nothing selectable until a language is chosen (a click, or Enter): a
        # list selects whichever row it's focused on, and none is assumed.
        self.language_list = Gtk.ListBox(selection_mode=Gtk.SelectionMode.NONE)
        self.language_list.add_css_class("zones")
        for code, name in languages():
            row = Gtk.ListBoxRow()
            row.code, row.name = code, name
            row.set_child(label(name, "zone", wrap=False))
            self.language_list.append(row)
        self.language_list.connect("row-selected", self.language_selected)
        self.language_list.connect("row-activated", lambda _l, row: self.select_language(row.code))
        scroll = Gtk.ScrolledWindow(min_content_height=240, vexpand=True)
        scroll.set_child(self.language_list)
        box.append(scroll)
        self.language_state = label(_("Choose one to go on."), "state")
        box.append(self.language_state)
        return box

    def language_selected(self, _list: Gtk.ListBox, row: Gtk.ListBoxRow | None) -> None:
        if row is not None:
            self.language_choice = row.code
            self.language_state.set_text(_("Chosen: {name}").format(name=row.name))
            # The setup in that language at once, once this selection is done.
            if i18n.ours(row.code) != i18n.language:
                GLib.idle_add(lambda code=row.code: self.get_application().speak(code) and False)
        self.update_next()

    def select_language(self, code: str) -> None:
        for row in _rows(self.language_list):
            if row.code == code:
                self.language_list.set_selection_mode(Gtk.SelectionMode.SINGLE)
                self.language_list.select_row(row)
                return

    # --- Keyboard -------------------------------------------------------------

    def keyboard_page(self) -> Gtk.Widget:
        box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=14)
        box.append(label(_("Your keyboard"), "title"))
        box.append(label(_("The layout of the keyboard plugged into the box. It's used at once, "
                           "so the admin's password is typed with it."), "lead"))
        self.layout_ids: list[str] = []
        self.layouts = Gtk.DropDown.new_from_strings([])
        box.append(field(_("Keyboard"), self.layouts))
        return box

    def show_layouts(self, answer: dict[str, Any]) -> None:
        layouts = answer.get("layouts") or []
        self.layout_ids = [layout["id"] for layout in layouts]
        self.layouts.set_model(Gtk.StringList.new([layout["name"] for layout in layouts]))
        if answer.get("layout") in self.layout_ids:
            self.layouts.set_selected(self.layout_ids.index(answer["layout"]))

    # --- Network --------------------------------------------------------------

    def network_page(self) -> Gtk.Widget:
        box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=14)
        box.append(label(_("Connect the box"), "title"))
        self.net_lead = label(_("Checking the box's network port…"), "lead")
        box.append(self.net_lead)

        self.net_rows = Gtk.Grid(column_spacing=24, row_spacing=8)
        self.net_rows.add_css_class("facts")
        box.append(self.net_rows)
        self.net_state = label("", "state")
        box.append(self.net_state)

        self.fixed_toggle = Gtk.CheckButton(label=_("Use a static address instead of the router's"))
        self.fixed_toggle.connect("toggled", lambda b: self.fixed_form.set_visible(b.get_active()))
        box.append(self.fixed_toggle)

        self.fixed_form = Gtk.Grid(column_spacing=12, row_spacing=12)
        self.fixed_entries: dict[str, Gtk.Entry] = {}
        for i, (key, caption, hint) in enumerate([
            ("address", _("Box's address"), "192.168.1.40"),
            ("mask", _("Subnet mask"), "255.255.255.0"),
            ("router", _("Router"), "192.168.1.1"),
            ("dns", _("Name servers"), "192.168.1.1"),
        ]):
            entry = Gtk.Entry(placeholder_text=hint, hexpand=True)
            self.fixed_entries[key] = entry
            self.fixed_form.attach(field(caption, entry), i % 2, i // 2, 1, 1)
        apply = Gtk.Button(label=_("Use these settings"))
        apply.connect("clicked", lambda _b: self.apply_fixed())
        self.fixed_form.attach(apply, 0, 2, 1, 1)
        self.fixed_form.set_visible(False)
        box.append(self.fixed_form)
        return box

    def ethernet(self) -> dict[str, Any] | None:
        links = [link for link in self.network.get("links", []) if link.get("kind") == "ethernet"]
        return next((link for link in links if link.get("connected")), links[0] if links else None)

    def address(self) -> str:
        link = self.ethernet()
        return (link or {}).get("ipv4", [""])[0] if (link or {}).get("ipv4") else ""

    def refresh_network(self) -> bool:
        def got(answer: dict[str, Any]) -> None:
            had = self.address()
            self.network = answer
            self.show_network()
            if self.address() and (not had or self.online is None):
                self.check_internet()

        def run() -> None:
            try:
                answer = self.agent.ask("network")
            except AgentError:
                return
            GLib.idle_add(lambda: got(answer) and False)

        threading.Thread(target=run, daemon=True).start()
        return True

    def show_network(self) -> None:
        link = self.ethernet()
        while child := self.net_rows.get_first_child():
            self.net_rows.remove(child)
        if link is None:
            self.net_lead.set_text(_("The box has no Ethernet port the setup can use."))
            self.net_state.set_text("")
        elif not link.get("connected"):
            self.net_lead.set_text(_("Plug the box into your router with a network cable. "
                                     "The setup goes on by itself once the cable is in."))
            self.net_state.set_text(_("Waiting for the cable…"))
            self.online = None
        elif not self.address():
            self.net_lead.set_text(_("The cable is in. Getting an address from your router…"))
            self.net_state.set_text("")
        else:
            self.net_lead.set_text(_("The box is on your network."))
            facts = [
                (_("Address"), self.address().split("/")[0]),
                (_("Router"), link.get("gateway") or _("None")),
                (_("Name servers"), ", ".join(self.network.get("dns") or []) or _("None")),
                (_("Settings"), _("Static address") if link.get("mode") == "fixed"
                 else _("From the router (DHCP)")),
            ]
            if link.get("speed_mbps"):
                facts.append((_("Speed"), f"{link['speed_mbps']} Mbit/s"))
            for row, (name, value) in enumerate(facts):
                self.net_rows.attach(label(name, "fact-name", wrap=False), 0, row, 1, 1)
                self.net_rows.attach(label(value, "fact", wrap=False), 1, row, 1, 1)
            self.net_state.set_text({
                None: _("Checking the internet…"),
                True: _("Internet: connected."),
                False: _("Internet: not reached. You can carry on; the box catches up "
                         "with updates and the clock once it's online."),
            }[self.online])
        self.update_next()

    def check_internet(self) -> None:
        self.online = None

        def run() -> None:
            reached = internet_reachable()
            GLib.idle_add(lambda: self.got_internet(reached) and False)

        threading.Thread(target=run, daemon=True).start()

    def got_internet(self, reached: bool) -> None:
        self.online = reached
        self.show_network()

    def apply_fixed(self) -> None:
        link = self.ethernet()
        if link is None:
            return
        try:
            settings = fixed_settings(*(self.fixed_entries[k].get_text()
                                        for k in ("address", "mask", "router", "dns")))
        except ValueError as exc:
            self.say(str(exc))
            return
        self.online = None
        self.call(lambda: self.agent.ask("network-set", link=link["name"], **settings),
                  lambda _r: self.say(""))

    def keep_network(self) -> dict[str, Any]:
        # A change waits two minutes to be kept, then goes back by itself.
        try:
            return self.agent.ask("network-keep")
        except AgentError:
            return {}

    # --- Region ---------------------------------------------------------------

    def region_page(self) -> Gtk.Widget:
        box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=14)
        box.append(label(_("Which country or region?"), "title"))
        box.append(label(_("Where the box is. It sets which time zones come first, and the "
                           "rules the box follows there."), "lead"))
        self.region_search = Gtk.SearchEntry(placeholder_text=_("Search, for example Canada"))
        self.region_search.connect("search-changed", lambda _e: self.region_list.invalidate_filter())
        box.append(self.region_search)
        self.region_list = Gtk.ListBox(selection_mode=Gtk.SelectionMode.SINGLE)
        self.region_list.add_css_class("zones")
        for code, name in regions():
            row = Gtk.ListBoxRow()
            row.code, row.name = code, name
            row.set_child(label(name, "zone", wrap=False))
            self.region_list.append(row)
        self.region_list.set_filter_func(
            lambda row: self.region_search.get_text().lower() in row.name.lower())
        self.region_list.connect("row-selected", self.region_selected)
        scroll = Gtk.ScrolledWindow(min_content_height=260, vexpand=True)
        scroll.set_child(self.region_list)
        box.append(scroll)
        self.region_state = label(_("Choose one to go on."), "state")
        box.append(self.region_state)
        # The state or province, for a country that has them; not needed to go on.
        self.sub_codes: list[str] = []
        self.sub_pick = Gtk.DropDown.new_from_strings([])
        self.sub_field = field(_("State or province (optional)"), self.sub_pick)
        self.sub_field.set_visible(False)
        box.append(self.sub_field)
        return box

    def show_subdivisions(self, region: str, chosen: str = "") -> None:
        listed = subdivisions(region)
        self.sub_codes = [""] + [code for code, _name in listed]
        self.sub_pick.set_model(Gtk.StringList.new([_("Not chosen")] + [name for _code, name in listed]))
        self.sub_pick.set_selected(self.sub_codes.index(chosen) if chosen in self.sub_codes else 0)
        self.sub_field.set_visible(bool(listed))

    def subdivision_choice(self) -> str:
        picked = self.sub_pick.get_selected()
        return self.sub_codes[picked] if 0 <= picked < len(self.sub_codes) else ""

    def region_selected(self, _list: Gtk.ListBox, row: Gtk.ListBoxRow | None) -> None:
        if row is not None:
            if row.code != self.region_choice:
                # A new region: its time zone is chosen again.
                self.zone_choice = ""
            if row.code != self.region_choice:
                self.show_subdivisions(row.code)
            self.region_choice = row.code
            self.region_state.set_text(_("Chosen: {name}").format(name=row.name))
        self.update_next()

    def select_region(self, code: str) -> None:
        for row in _rows(self.region_list):
            if row.code == code:
                self.region_list.select_row(row)
                return

    # --- Time zone ------------------------------------------------------------

    def zone_page(self) -> Gtk.Widget:
        box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=14)
        box.append(label(_("Which time zone?"), "title"))
        self.zone_lead = label("", "lead")
        box.append(self.zone_lead)
        self.zone_search = Gtk.SearchEntry(placeholder_text=_("Search, for example Toronto"))
        self.zone_search.connect("search-changed", lambda _e: self.zone_list.invalidate_filter())
        box.append(self.zone_search)
        self.all_zones = Gtk.CheckButton(label=_("Show every time zone"))
        self.all_zones.connect("toggled", lambda _b: self.zone_list.invalidate_filter())
        box.append(self.all_zones)

        self.zone_list = Gtk.ListBox(selection_mode=Gtk.SelectionMode.SINGLE)
        self.zone_list.add_css_class("zones")
        for zone in time_zones():
            row = Gtk.ListBoxRow()
            row.zone = zone
            row.set_child(label(zone.replace("_", " ").replace("/", " / "), "zone", wrap=False))
            self.zone_list.append(row)
        self.region_zones: list[str] = []
        self.zone_list.set_filter_func(self.zone_matches)
        self.zone_list.connect("row-selected", self.zone_selected)
        scroll = Gtk.ScrolledWindow(min_content_height=220, vexpand=True)
        scroll.set_child(self.zone_list)
        box.append(scroll)
        self.clock = label("", "state")
        box.append(self.clock)
        GLib.timeout_add_seconds(1, self.tick)
        return box

    def show_zones(self) -> None:
        """The region's zones only, unless it has none; nothing chosen yet
        unless the person chose one."""
        self.region_zones = zones_in(self.region_choice)
        name = next((name for code, name in regions() if code == self.region_choice), "")
        self.zone_lead.set_text(
            _("The time zones in {region}. The box's clock, its maintenance hour and the "
              "apps go by it.").format(region=name) if self.region_zones else
            _("The box's clock, its maintenance hour and the apps go by it."))
        self.all_zones.set_active(not self.region_zones)
        self.zone_list.invalidate_filter()
        self.zone_list.unselect_all()
        if self.zone_choice:
            self.select_zone(self.zone_choice)
        self.tick()

    def zone_matches(self, row: Gtk.ListBoxRow) -> bool:
        if not self.all_zones.get_active() and row.zone not in self.region_zones:
            return False
        words = self.zone_search.get_text().lower().replace(" ", "_").split()
        return all(word in row.zone.lower() for word in words)

    def zone_selected(self, _list: Gtk.ListBox, row: Gtk.ListBoxRow | None) -> None:
        if row is not None:
            self.zone_choice = row.zone
        self.tick()
        self.update_next()

    def select_zone(self, zone: str) -> None:
        for row in _rows(self.zone_list):
            if row.zone == zone:
                self.zone_list.select_row(row)
                return

    def tick(self) -> bool:
        if not self.zone_choice:
            self.clock.set_text(_("Choose one to go on."))
            return True
        try:
            now = datetime.now(zoneinfo.ZoneInfo(self.zone_choice))
            # The time alone: the box's language isn't built yet to name the day.
            self.clock.set_text(_("The time there now: {time}").format(time=f"{now:%H:%M}"))
        except zoneinfo.ZoneInfoNotFoundError:
            self.clock.set_text("")
        return True

    # --- Admin ----------------------------------------------------------------

    def admin_page(self) -> Gtk.Widget:
        box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=14)
        box.append(label(_("The box's admin"), "title"))
        box.append(label(_("Your account in ZOOMRIYA ID, on this box. The same username and "
                           "password sign you in to the box and to every ZOOMRIYA app you "
                           "add to it. There is no other way in, so keep the password safe."),
                         "lead"))
        self.username = Gtk.Entry(placeholder_text=_("For example, amma"))
        self.password = Gtk.PasswordEntry(show_peek_icon=True)
        self.again = Gtk.PasswordEntry(show_peek_icon=True)
        for entry in (self.username, self.password, self.again):
            entry.connect("changed", lambda _e: self.update_next())
        self.again.connect("activate", lambda _e: self.next.get_sensitive() and self.confirm())
        box.append(field(_("Username"), self.username))
        box.append(field(_("Password (at least {count} characters)").format(count=MIN_PASSWORD), self.password))
        box.append(field(_("The password again"), self.again))
        self.admin_missing = label("", "state")
        box.append(self.admin_missing)
        return box

    def make_admin(self) -> None:
        username, password = self.username.get_text().strip(), self.password.get_text()

        def made(_answer: Any) -> None:
            self.admin = username
            self.password.set_text("")
            self.again.set_text("")
            self.go(DONE)

        def failed(problem: str) -> None:
            if "already has an admin" in problem:
                # Made before a power cut: the setup goes on from there.
                self.go(DONE)
            else:
                self.say(problem)

        self.call(lambda: self.agent.ask("first-admin", username=username, password=password),
                  made, failed)

    # --- Done -----------------------------------------------------------------

    def done_page(self) -> Gtk.Widget:
        box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=14)
        tick = Gtk.DrawingArea(content_width=96, content_height=96, halign=Gtk.Align.START)
        tick.set_draw_func(draw_done)
        box.append(tick)
        box.append(label(_("Your box is ready"), "title"))
        self.done_lead = label("", "lead")
        box.append(self.done_lead)
        self.done_rows = Gtk.Grid(column_spacing=24, row_spacing=8)
        self.done_rows.add_css_class("facts")
        box.append(self.done_rows)
        return box

    def show_done(self) -> None:
        def got(answer: dict[str, Any]) -> None:
            self.network = answer
            while child := self.done_rows.get_first_child():
                self.done_rows.remove(child)
            facts = [(_("Box's name"), answer.get("name", "")),
                     (_("Address"), self.address().split("/")[0] or _("Not connected")),
                     (_("Language"), next((n for c, n in languages() if c == self.language_choice), "")),
                     (_("Region"), ", ".join(filter(None, [
                         next((n for c, n in subdivisions(self.region_choice)
                               if c == self.subdivision_choice()), ""),
                         next((n for c, n in regions() if c == self.region_choice), "")]))),
                     (_("Time zone"), self.zone_choice.replace("_", " "))]
            if self.admin:
                facts.insert(0, (_("Admin"), self.admin))
            for row, (name, value) in enumerate(facts):
                self.done_rows.attach(label(name, "fact-name", wrap=False), 0, row, 1, 1)
                self.done_rows.attach(label(value, "fact", wrap=False), 1, row, 1, 1)

        self.done_lead.set_text(_("Press Finish to close the setup. Sign in with the admin's "
                                  "username and password from now on; to set the box up again, "
                                  "use Erase everything."))
        self.call(lambda: self.agent.ask("network"), got, lambda _p: None)


def _rows(listbox: Gtk.ListBox) -> list[Gtk.ListBoxRow]:
    rows, row = [], listbox.get_first_child()
    while row is not None:
        rows.append(row)
        row = row.get_next_sibling()
    return rows


class SetupApp(Gtk.Application):
    def __init__(self, agent: Any) -> None:
        # Not unique: the box's screen has no session bus to register on.
        super().__init__(application_id="com.zoomriya.Setup",
                         flags=Gio.ApplicationFlags.NON_UNIQUE)
        self.agent = agent

    def do_activate(self) -> None:
        css = Gtk.CssProvider()
        css.load_from_path(str(Path(__file__).with_name("style.css")))
        Gtk.StyleContext.add_provider_for_display(
            Gdk.Display.get_default(), css, Gtk.STYLE_PROVIDER_PRIORITY_APPLICATION
        )
        # In the language already picked, when a power cut cut the setup short.
        if progress().get("language"):
            i18n.use(progress()["language"])
        self.show_window()

    def speak(self, code: str) -> None:
        """The setup in the language just picked (English for one our screens
        don't come in), drawn again with it chosen."""
        i18n.use(code)
        old = self.get_active_window()
        self.show_window(code)
        if old is not None:
            old.destroy()

    def show_window(self, chosen: str = "") -> None:
        window = SetupWindow(self, self.agent, chosen)
        if os.environ.get("ZOOMRIYA_SETUP_WINDOWED"):
            window.set_default_size(1280, 800)
        else:
            window.fullscreen()
        window.present()
