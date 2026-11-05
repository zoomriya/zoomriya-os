"""Control Panel's own sections (box-desktop.md §4), in its settings look
(settings.py): the box's network, its screen, its sound, the keyboard and
mouse, language and region, date and time, security at a glance, the
notifications and System Info. The window (app.py) lists them with the
restarts, restore points and reset (tools.py)."""

from __future__ import annotations

import ipaddress
import os
from pathlib import Path
from typing import Any

import gi

gi.require_version("Gtk", "4.0")
from gi.repository import GLib, Gtk  # noqa: E402

from zoomriya_setup.app import (fixed_settings, languages, locale_for, regions, subdivisions,  # noqa: E402
                                time_zones)
from zoomriya_setup.brand import MARK, logo  # noqa: E402
from zoomriya_setup.screen import set_keyboard, set_left_handed  # noqa: E402

from .display import ORIENTATIONS, SCALES, Mode, ratio  # noqa: E402
from .icons import glyph, set_glyph  # noqa: E402
from .settings import fixed_switch, group, page, row, segmented, slider, text, tile, value, wide  # noqa: E402
from .tools import day, size, when  # noqa: E402

#: The agent's kinds of notification, as the person reads them.
#: Each kind of notification: its name, what sends one and its icon.
KINDS = {
    "drive": ("Drive Health", "When a drive should be checked or replaced, and when it's healthy again.",
              "disk"),
    "update": ("Updates", "When security fixes are installed.", "updates"),
    "restart": ("Restarts", "When the box will restart by itself to finish an update or a restore.",
                "power"),
    "check": ("Library Check", "When the library's check finishes, and what it found.", "scan"),
    "network": ("Network", "When new network settings weren't kept and went back.", "ethernet"),
}
#: How long a new screen setting waits to be kept before going back.
KEEP_SECONDS = 15


#: Every dropdown's narrowest; a longer choice makes it wider.
PICK_WIDTH = 170


def dropdown(names: list[str], search: bool = False) -> Gtk.DropDown:
    pick = Gtk.DropDown.new_from_strings(names)
    pick.set_enable_search(search)
    pick.set_size_request(PICK_WIDTH, -1)
    return pick


def utc(zone: str) -> str:
    """The database's other names for UTC, as UTC."""
    return "UTC" if zone in ("Etc/UTC", "Etc/Universal", "UCT") else zone


def processor() -> str:
    """The processor's name and how many threads it runs, from the kernel."""
    try:
        info = Path("/proc/cpuinfo").read_text()
    except OSError:
        return ""
    name = next((line.split(":", 1)[1].strip() for line in info.splitlines()
                 if line.startswith("model name")), "")
    return f"{name} × {os.cpu_count()}" if name else ""


# Security's rows: a larger, coloured icon for each.
BIG = {"icon_px": 28}


class ControlPanel:
    """Mixed into BoxDesktop, like Tools."""

    # --- Network ------------------------------------------------------------------

    def cp_network(self) -> Gtk.Widget:
        """Each Ethernet port, then how the chosen one gets its address. Wi-Fi, on a box
        that has it, is shown turned off: the box uses the cable (OS-14)."""
        from .app import in_background

        box = page()
        ports = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=22)
        box.append(ports)

        settings_group, rows = group("Configuration Details", "The box tries new settings for a few minutes: "
                                     "unless you keep them, they go back by themselves.", icon="gear")
        title = settings_group.title_label
        box.append(settings_group)
        port = {"name": "", "links": []}
        how = {"mode": "dhcp"}

        def chose_how(mode: str) -> None:
            how["mode"] = mode
            for r in fixed_rows:
                r.set_sensitive(mode == "fixed")

        modes = segmented([("dhcp", "DHCP"), ("fixed", "Static")], "dhcp", chose_how)
        rows.append(row("IP Address Assignment Method", "From the router (DHCP), or a static address that "
                        "never changes.", modes, **BIG, icon="swap"))
        entries: dict[str, Gtk.Entry] = {}
        fixed_rows = []
        for key, caption, hint, icon in [
            ("address", "Box's address", "192.168.1.40", "ip-screen"),
            ("mask", "Subnet mask", "255.255.255.0", "layers"),
            ("router", "Router", "192.168.1.1", "router"), ("dns", "Name servers", "192.168.1.1", "dns"),
        ]:
            entry = Gtk.Entry(placeholder_text=hint)
            entry.set_size_request(260, -1)
            entries[key] = entry
            fixed_rows.append(row(caption, "", entry, **BIG, icon=icon))
            rows.append(fixed_rows[-1])
        problem = text("", "error", wrap=True)
        problem.set_visible(False)
        state = text("", "row-sub", wrap=True, hexpand=True)
        apply = Gtk.Button(label="Use these settings")
        apply.add_css_class("primary")
        keep = Gtk.Button(label="Keep them")
        keep.set_visible(False)
        actions = Gtk.Box(spacing=10)
        actions.append(state)
        actions.append(keep)
        actions.append(apply)
        rows.append(wide(problem, actions))

        def show_port(name: str) -> None:
            port["name"] = name
            link = next((x for x in port["links"] if x.get("name") == name), {})
            title.set_text(f"Configuration Details ({name})" if name else "Configuration Details")
            is_fixed = link.get("mode") == "fixed"
            modes.get_first_child().set_active(not is_fixed)
            modes.get_last_child().set_active(is_fixed)
            chose_how("fixed" if is_fixed else "dhcp")
            fixed = link.get("fixed") or {}
            if fixed:
                address, _slash, bits = fixed.get("address", "").partition("/")
                entries["address"].set_text(address)
                if bits:
                    entries["mask"].set_text(str(ipaddress.IPv4Network(f"0.0.0.0/{bits}").netmask))
                entries["router"].set_text(fixed.get("gateway", ""))
                entries["dns"].set_text(", ".join(fixed.get("dns") or []))
            undo = link.get("undo_at")
            keep.set_visible(bool(undo))
            now = (link.get("ipv4") or ["none"])[0].split("/")[0]
            state.set_text(f"Current Address: {now}." + (f" New settings go back by themselves at "
                                                         f"{when(undo)} unless you keep them." if undo else ""))

        def got(answer: dict[str, Any]) -> None:
            links = [x for x in answer.get("links", []) if x.get("kind") == "ethernet"]
            port["links"] = links
            while child := ports.get_first_child():
                ports.remove(child)
            for link in links:
                card, card_rows = group(f"Ethernet Status ({link.get('name', '')})", icon="ethernet")
                speed = link.get("speed_mbps") or 0
                if link.get("connected"):
                    status = "Connection: Connected"
                    address = (link.get("ipv4") or [""])[0].split("/")[0] or "none yet"
                    how = "Static" if link.get("mode") == "fixed" else "Assigned by Router"
                    sub = f"IPv4 Address: {address} ({how})"
                else:
                    status, sub = "Connection: Cable unplugged", f"MAC address: {link.get('mac', '')}"
                # The speed and a dot for the connection, on the right.
                side = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=6, halign=Gtk.Align.END)
                if link.get("connected") and speed:
                    fast = Gtk.Box(spacing=6, halign=Gtk.Align.END)
                    fast.append(text(f"{speed:,} Mb/s", "row-title"))
                    fast.append(glyph("speed", 20, "accent"))
                    side.append(fast)
                dot = Gtk.Box(halign=Gtk.Align.END,
                              tooltip_text="Connected" if link.get("connected") else "Not connected")
                dot.add_css_class("allowed-dot" if link.get("connected") else "off-dot")
                side.append(dot)
                ends: list[Gtk.Widget] = [side]
                if len(links) > 1:
                    gear = Gtk.Button(tooltip_text=f"Configuration Details ({link.get('name', '')})")
                    gear.add_css_class("bar-button")
                    gear.set_child(glyph("gear", 18))
                    gear.connect("clicked", lambda _b, n=link.get("name", ""): show_port(n))
                    ends.append(gear)
                card_rows.append(row(status, sub, *ends, **BIG, icon="ethernet"))
                ports.append(card)
            if not links:
                card, card_rows = group("Ethernet", icon="ethernet")
                card_rows.append(row("No network port found", "The box needs a cable to the router."))
                ports.append(card)
            if any(x.get("kind") == "wifi" for x in answer.get("links", [])):
                card, card_rows = group("Wi-Fi (Wireless Network)", icon="wifi-off")
                card_rows.append(row("Status: Off", "The box connects to the router by a cable.",
                                     fixed_switch(False), **BIG, icon="wifi-off"))
                ports.append(card)
            chosen = port["name"] if any(x.get("name") == port["name"] for x in links) else \
                next((x.get("name", "") for x in links if x.get("connected")),
                     links[0].get("name", "") if links else "")
            show_port(chosen)

        def load() -> None:
            in_background(lambda: self.ask("network"), got)

        def apply_now() -> None:
            problem.set_visible(False)
            try:
                if how["mode"] == "fixed":
                    settings = fixed_settings(*(entries[k].get_text()
                                                for k in ("address", "mask", "router", "dns")))
                else:
                    settings = {"mode": "dhcp"}
            except ValueError as exc:
                problem.set_text(str(exc))
                problem.set_visible(True)
                return

            def failed(words: str) -> None:
                problem.set_text(words)
                problem.set_visible(True)

            in_background(lambda: self.ask("network-set", link=port["name"], **settings),
                          lambda _a: GLib.timeout_add_seconds(3, lambda: load() and False), failed)

        apply.connect("clicked", lambda _b: apply_now())
        keep.connect("clicked", lambda _b: in_background(lambda: self.ask("network-keep"),
                                                          lambda _a: load()))
        load()
        return box

    # --- Display --------------------------------------------------------------------

    def cp_display(self) -> Gtk.Widget:
        """The box's own screen, for whoever signs in: tried at once, then kept or undone."""
        from .app import in_background

        box = page()
        outputs = self.display.outputs()
        if not outputs:
            card, rows = group("Display", icon="monitor")
            rows.append(row("No screen to set here", "These settings are for the screen plugged into "
                            "the box itself."))
            box.append(card)
            return box

        bar = Gtk.Box(spacing=12)
        bar.add_css_class("keep-bar")
        question = text("", wrap=True, hexpand=True)
        bar.append(question)
        undo = Gtk.Button(label="Go back")
        keep = Gtk.Button(label="Keep")
        keep.add_css_class("primary")
        bar.append(undo)
        bar.append(keep)
        bar.set_visible(False)
        box.append(bar)
        saved = text("", "group-note", wrap=True)

        for out in outputs:
            card, rows = group(out.title if out.title == out.name else f"{out.title} ({out.name})",
                               "These settings are for the box's own screen, whoever signs in. A change "
                               f"shows at once: keep it, or it goes back by itself in {KEEP_SECONDS} seconds.",
                               icon="monitor")
            sizes = out.sizes()
            size_pick = dropdown([f"{w} × {h}" + (f" ({ratio(w, h)})" if ratio(w, h) else "")
                                  for w, h in sizes])
            rate_pick = dropdown([])
            turn_pick = dropdown([name for _k, name in ORIENTATIONS])
            scale_pick = dropdown([f"{s * 100:.0f} %" for s in SCALES])
            rows.append(row("Orientation", "Which way round the screen stands: landscape or portrait.",
                            turn_pick, **BIG, icon="rotate"))
            rows.append(row("Resolution", "More pixels mean a sharper picture, with smaller things on it.",
                            size_pick, **BIG, icon="resolution"))
            rows.append(row("Screen Update Rate", "How smoothly things move on the screen.", rate_pick,
                            **BIG, icon="rate"))
            rows.append(row("Text & UI Scaling", "The size of text and everything else on the screen.",
                            scale_pick, **BIG, icon="text-size"))
            box.append(card)
            shown = {"busy": False, "rates": [], "before": None, "timer": 0, "left": 0}

            def fill(o=out, size_pick=size_pick, rate_pick=rate_pick, turn_pick=turn_pick,
                     scale_pick=scale_pick, shown=shown, sizes=sizes) -> None:
                shown["busy"] = True
                here = (o.mode.width, o.mode.height)
                if here in sizes:
                    size_pick.set_selected(sizes.index(here))
                fill_rates(o, rate_pick, shown, sizes, size_pick)
                keys = [k for k, _n in ORIENTATIONS]
                turn_pick.set_selected(keys.index(o.transform) if o.transform in keys else 0)
                nearest = min(range(len(SCALES)), key=lambda i: abs(SCALES[i] - o.scale))
                scale_pick.set_selected(nearest)
                shown["busy"] = False

            def fill_rates(o, rate_pick, shown, sizes, size_pick) -> None:
                w, h = sizes[size_pick.get_selected()]
                shown["rates"] = o.rates(w, h)
                rate_pick.set_model(Gtk.StringList.new([f"{r / 1000:.2f} Hz" for r in shown["rates"]]))
                if o.mode.refresh in shown["rates"]:
                    rate_pick.set_selected(shown["rates"].index(o.mode.refresh))

            def changed(*_a, o=out, size_pick=size_pick, rate_pick=rate_pick, turn_pick=turn_pick,
                        scale_pick=scale_pick, shown=shown, sizes=sizes, what="") -> None:
                if shown["busy"]:
                    return
                if what == "size":
                    shown["busy"] = True
                    fill_rates(o, rate_pick, shown, sizes, size_pick)
                    if rate_pick.get_selected() >= len(shown["rates"]):
                        rate_pick.set_selected(0)
                    shown["busy"] = False
                w, h = sizes[size_pick.get_selected()]
                rates = shown["rates"] or [o.mode.refresh]
                mode = Mode(w, h, rates[min(rate_pick.get_selected(), len(rates) - 1)])
                if shown["before"] is None:
                    shown["before"] = (o.mode, o.scale, o.transform)
                turn = ORIENTATIONS[turn_pick.get_selected()][0]
                scale = SCALES[scale_pick.get_selected()]
                self.display.apply(o.name, mode, scale, turn)
                o.mode, o.scale, o.transform = mode, scale, turn
                ask_to_keep(o, shown, fill)

            size_pick.connect("notify::selected", lambda *a, c=changed: c(what="size"))
            for pick in (rate_pick, turn_pick, scale_pick):
                pick.connect("notify::selected", changed)
            fill()

        box.append(saved)

        def ask_to_keep(o, shown, fill) -> None:
            if shown["timer"]:
                GLib.source_remove(shown["timer"])
            shown["left"] = KEEP_SECONDS

            def tick() -> bool:
                shown["left"] -= 1
                if shown["left"] <= 0:
                    go_back()
                    return False
                question.set_text(f"Keep these display settings? They go back in {shown['left']} s.")
                return True

            def go_back() -> None:
                if shown["timer"]:
                    GLib.source_remove(shown["timer"])
                    shown["timer"] = 0
                mode, scale, turn = shown["before"]
                self.display.apply(o.name, mode, scale, turn)
                o.mode, o.scale, o.transform = mode, scale, turn
                shown["before"] = None
                bar.set_visible(False)
                fill()

            def kept() -> None:
                GLib.source_remove(shown["timer"])
                shown["timer"] = 0
                shown["before"] = None
                bar.set_visible(False)
                in_background(lambda: self.ask(
                    "display-set", output=o.name, size=f"{o.mode.width}x{o.mode.height}",
                    refresh=o.mode.refresh, scale=o.scale, transform=o.transform),
                    lambda _a: saved.set_text("Kept for the box: the screen starts this way from now on."),
                    lambda p: saved.set_text(p))

            question.set_text(f"Keep these display settings? They go back in {KEEP_SECONDS} s.")
            shown["timer"] = GLib.timeout_add_seconds(1, tick)
            for button, act in ((undo, go_back), (keep, kept)):
                if getattr(button, "_handler", None):
                    button.disconnect(button._handler)
                button._handler = button.connect("clicked", lambda _b, a=act: a())
            bar.set_visible(True)

        return box

    # --- Sound ------------------------------------------------------------------------

    def cp_sound(self) -> Gtk.Widget:
        from .sound import INPUT, OUTPUT

        box = page()

        def device_pick(device: str) -> Gtk.Widget:
            """A dropdown even with one device, as the OS's own sound settings show it."""
            listed = self.sound.devices(device)
            if not listed:
                return value("None found")
            pick = dropdown([name for _id, name, _now in listed])
            pick.set_selected(next((i for i, d in enumerate(listed) if d[2]), 0))
            pick.connect("notify::selected", lambda p, _x: self.sound.use(listed[p.get_selected()][0]))
            return pick

        def volume(device: str, on: str, off: str) -> Gtk.Widget:
            level, muted = self.sound.volume(device)
            line = Gtk.Box(spacing=12)
            mute = Gtk.ToggleButton(active=muted, tooltip_text="Mute")
            mute.add_css_class("bar-button")
            icon = glyph(off if muted else on, 20)
            mute.set_child(icon)

            def muting(b: Gtk.ToggleButton) -> None:
                self.sound.set_muted(device, b.get_active())
                set_glyph(icon, off if b.get_active() else on)

            mute.connect("toggled", muting)
            line.append(mute)
            scale = Gtk.Scale.new_with_range(Gtk.Orientation.HORIZONTAL, 0, 100, 1)
            scale.set_hexpand(True)
            scale.set_size_request(260, -1)
            scale.set_value(level * 100)
            scale.set_draw_value(False)
            scale.connect("value-changed", lambda s: self.sound.set_volume(device, s.get_value() / 100))
            line.append(scale)
            return line

        card, rows = group("Output", icon="volume")
        test = Gtk.Button(tooltip_text="Play a test sound")
        face = Gtk.Box(spacing=6)
        face.append(glyph("play", 14))
        face.append(Gtk.Label(label="Test"))
        test.set_child(face)
        test.connect("clicked", lambda _b: self.sound.play_test())
        rows.append(row("Output Device", "", device_pick(OUTPUT), test, **BIG, icon="speaker"))
        rows.append(row("Output Volume", "", volume(OUTPUT, "volume", "muted"), **BIG, icon="sliders"))
        box.append(card)

        card, rows = group("Input", icon="mic")
        rows.append(row("Input Device", "", device_pick(INPUT), **BIG, icon="mic"))
        rows.append(row("Input Volume", "", volume(INPUT, "mic", "mic-off"), **BIG, icon="sliders"))
        self.mic_level = Gtk.LevelBar(min_value=0, max_value=1, hexpand=True)
        self.mic_level.add_css_class("mic")
        self.mic_level.set_size_request(260, -1)
        rows.append(row("Input Level", "Speak to see it move.", self.mic_level, **BIG, icon="wave"))
        box.append(card)
        return box

    # --- Keyboard and mouse ---------------------------------------------------------

    def cp_keyboard(self) -> Gtk.Widget:
        """Keyboard & Mouse: each admin's own, like the theme (box-desktop.md §4)."""
        from .app import in_background

        box = page()
        card, rows = group("Keyboard", icon="keyboard")
        pick = dropdown([])
        rows.append(row("Input Layout", "", pick, **BIG, icon="keyboard"))
        box.append(card)

        mouse = dict(self.prefs.get("mouse") or {"speed": 0.0, "accel": True, "natural": False})

        def save_mouse(**changed: Any) -> None:
            mouse.update(changed)
            self.prefs["mouse"] = dict(mouse)
            self.save_prefs()
            self.display.pointer(mouse["speed"], mouse["accel"], mouse["natural"])

        def handed(which: str) -> None:
            self.prefs["handed"] = "left" if which == "right" else "right"
            self.save_prefs()
            set_left_handed(which == "right")

        card, rows = group("Mouse", icon="mouse")
        primary = "right" if self.prefs.get("handed") == "left" else "left"
        rows.append(row("Primary click side", "The button that clicks; the other one opens menus.",
                        segmented([("left", "Left", "mouse"), ("right", "Right", "mouse")], primary, handed)))
        rows.append(row("Cursor Speed", "", slider("Slow", "Fast", mouse.get("speed", 0.0),
                                                   lambda v: save_mouse(speed=round(v, 2)), -1, 1),
                        **BIG, icon="pointer"))
        accel = Gtk.Switch(active=mouse.get("accel", True))
        accel.connect("notify::active", lambda s, _p: save_mouse(accel=s.get_active()))
        rows.append(row("Dynamic Cursor Acceleration", "The cursor goes further when you move the mouse "
                        "quickly, for a natural feel.", accel, **BIG, icon="accel"))
        box.append(card)

        card, rows = group("Scroll direction", icon="scroll")
        traditional = Gtk.CheckButton(active=not mouse.get("natural", False))
        natural = Gtk.CheckButton(active=mouse.get("natural", False), group=traditional)
        natural.connect("toggled", lambda b: save_mouse(natural=b.get_active()))
        rows.append(row("Screen moves (view)", "Scrolling moves the visible part of the page.", traditional,
                        **BIG, icon="screen-move"))
        rows.append(row("Content moves (finger)", "The content follows your scroll, like on a phone.", natural,
                        **BIG, icon="content-move"))
        box.append(card)
        ids: list[str] = []

        def got(answer: dict[str, Any]) -> None:
            ids[:] = [x["id"] for x in answer.get("layouts") or []]
            pick.set_model(Gtk.StringList.new([x["name"] for x in answer.get("layouts") or []]))
            mine = self.prefs.get("keyboard") or answer.get("layout")
            if mine in ids:
                pick.set_selected(ids.index(mine))
            pick.connect("notify::selected", lambda *_a: choose(ids[pick.get_selected()]))

        def choose(layout: str) -> None:
            self.prefs["keyboard"] = layout
            self.save_prefs()
            set_keyboard(layout)
            in_background(lambda: self.ask("keyboard-set", layout=layout), lambda _a: None)

        in_background(lambda: self.ask("keyboard"), got)
        return box

    # --- Language and region --------------------------------------------------------

    def cp_language(self) -> Gtk.Widget:
        from .app import in_background

        box = page()
        card, rows = group("Language & Region", icon="globe")
        langs = languages()
        places = regions()
        lang_pick = dropdown([name for _c, name in langs], search=True)
        region_pick = dropdown([name for _c, name in places], search=True)
        for pick in (lang_pick, region_pick):
            pick.set_selected(Gtk.INVALID_LIST_POSITION)
        rows.append(row("Language", "The language of the box's screens and messages.", lang_pick,
                        **BIG, icon="globe"))
        rows.append(row("Country or Region", "Where the box is: its dates, times and numbers follow it.",
                        region_pick, **BIG, icon="pin"))
        # Its state or province, for a country that has them; not needed.
        sub_codes: list[str] = []
        sub_pick = dropdown([])
        sub_row = row("State or Province", "Optional.", sub_pick, **BIG, icon="pin")
        sub_row.set_visible(False)
        rows.append(sub_row)
        box.append(card)

        def show_subdivisions(region: str, chosen: str = "") -> None:
            listed = subdivisions(region)
            sub_codes[:] = [""] + [code for code, _name in listed]
            sub_pick.set_model(Gtk.StringList.new(["Not chosen"] + [name for _code, name in listed]))
            sub_pick.set_selected(sub_codes.index(chosen) if chosen in sub_codes else 0)
            sub_row.set_visible(bool(listed))
        done = text("", "group-note")
        box.append(done)

        def save() -> None:
            if Gtk.INVALID_LIST_POSITION in (lang_pick.get_selected(), region_pick.get_selected()):
                return
            lang = langs[lang_pick.get_selected()][0]
            region = places[region_pick.get_selected()][0]
            picked = sub_pick.get_selected()
            sub = sub_codes[picked] if 0 <= picked < len(sub_codes) else ""
            locale = locale_for(lang, region)
            in_background(lambda: (self.ask("region-set", region=region, subdivision=sub),
                                   self.ask("language-set", locale=locale)),
                          lambda _a: done.set_text("Saved."), done.set_text)

        def got(status: dict[str, Any]) -> None:
            code = (status.get("language") or "").split("_")[0]
            codes = [c for c, _n in langs]
            if code in codes:
                lang_pick.set_selected(codes.index(code))
            places_codes = [c for c, _n in places]
            if status.get("region") in places_codes:
                region_pick.set_selected(places_codes.index(status["region"]))
                show_subdivisions(status["region"], status.get("subdivision", ""))

            def region_changed(*_a: Any) -> None:
                # A new country: its own provinces, none chosen yet.
                if region_pick.get_selected() != Gtk.INVALID_LIST_POSITION:
                    show_subdivisions(places[region_pick.get_selected()][0])
                save()

            lang_pick.connect("notify::selected", lambda *_a: save())
            region_pick.connect("notify::selected", region_changed)
            sub_pick.connect("notify::selected", lambda *_a: save())

        in_background(lambda: self.ask("setup-status"), got)
        return box

    # --- Date and time ------------------------------------------------------------

    def cp_time(self) -> Gtk.Widget:
        from .app import in_background

        box = page()
        card, rows = group("Date & Time", icon="clock")
        zones = time_zones()
        pick = dropdown([z.replace("_", " ") for z in zones], search=True)
        pick.set_selected(Gtk.INVALID_LIST_POSITION)
        # Short enough never to wrap, unlike other values.
        now = text(day(GLib.DateTime.new_now_local().format_iso8601(), at=True), "row-value")
        rows.append(row("Current Time", "On the box's clock.", now, **BIG, icon="clock"))
        rows.append(row("Time Zone", "The box's dates and times follow it.", pick, **BIG, icon="globe"))
        synced = Gtk.Box(valign=Gtk.Align.CENTER)
        synced.add_css_class("off-dot")
        clock = row("Time Sync", "", synced, **BIG, icon="refresh")
        said = text("", "row-sub", wrap=True)
        clock.get_child().get_first_child().get_next_sibling().append(said)
        rows.append(clock)
        box.append(card)
        done = text("", "group-note")
        box.append(done)

        def got(status: dict[str, Any]) -> None:
            zone = utc(status.get("zone", "UTC"))
            if zone in zones:
                pick.set_selected(zones.index(zone))
            on = bool(status.get("clock_synced"))
            said.set_text("Set by itself from the internet." if on else
                          "Not yet set from the internet: it is once the box is online.")
            synced.remove_css_class("off-dot")
            synced.add_css_class("allowed-dot" if on else "off-dot")
            synced.set_tooltip_text("In sync" if on else "Not yet in sync")
            pick.connect("notify::selected", lambda *_a: in_background(
                lambda: self.ask("time-zone-set", zone=zones[pick.get_selected()]),
                lambda _a: done.set_text("Saved.")))

        in_background(lambda: self.ask("setup-status"), got)
        return box

    # --- Security -----------------------------------------------------------------

    def cp_security(self) -> Gtk.Widget:
        from .app import in_background

        box = page()

        def got(answer: dict[str, Any]) -> None:
            updates = answer.get("updates") or {}
            firewall = answer.get("firewall") or {}
            integrity = answer.get("integrity") or {}
            apparmor = answer.get("apparmor") or {}
            secure_boot = str(answer.get("secure_boot", ""))
            wrong = [name for name, fine in (
                ("security fixes are off", updates.get("automatic")),
                ("the firewall is off", firewall.get("on")),
                ("the OS's files changed", integrity.get("clean") is not False),
                ("AppArmor is off", apparmor.get("on"))) if not fine]
            tiles = Gtk.Box(spacing=14, homogeneous=True)
            if wrong:
                tiles.append(tile("alert", "Needs a look", wrong[0][:1].upper() + wrong[0][1:] + ".", "warn"))
            else:
                tiles.append(tile("shield", "Protected", "Security fixes install by themselves and "
                                  "the firewall is on.", "ok"))
            if secure_boot == "on":
                tiles.append(tile("lock", "Secure Boot is on", "Only signed software can start "
                                  "the box.", "ok"))
            else:
                tiles.append(tile("lock", "Secure Boot is " + ("off" if secure_boot == "off" else "unavailable"),
                                  "Turn it on in the computer's firmware settings, if it has it.", "off"))
            box.append(tiles)

            # What protects the box, each shown on and greyed out: none of it is
            # turned off here (design §8).
            card, rows = group("Protection", icon="shield")
            rows.append(self.security_fixes_row(updates))
            rows.append(row("Firewall", "", fixed_switch(bool(firewall.get("on"))), **BIG, icon="firewall"))
            rows.append(self.apparmor_row(apparmor))
            rows.append(row("Secure Boot", "", fixed_switch(secure_boot == "on"), **BIG, icon="key"))
            last = updates.get("last_installed")
            rows.append(row("OS Update", "", value(day(last) if last else "None yet"), **BIG, icon="refresh"))
            # The nightly check of the OS's files (design §8).
            if integrity.get("checked") is None:
                scanned = "Not yet: the box checks its files every night."
            elif integrity.get("clean"):
                scanned = f"Completed on {when(integrity['checked'])}: the OS's files are as the last update " \
                          "left them."
            else:
                scanned = (f"Completed on {when(integrity['checked'])}: files changed outside an update: "
                           + ", ".join(integrity.get("changes") or []))
            rows.append(row("Last System Scan", scanned, **BIG, icon="scan"))
            box.append(card)

            # Opened by the OS, and later by the apps for what they need (design §8);
            # never by hand.
            card, rows = group("Allowed Inbound Connections", "The firewall turns everything else away.",
                               lead="What other devices on the network may reach on the box.", icon="firewall")
            for port in firewall.get("open") or []:
                how = ", ".join(x for x in (str(port.get("protocol", "")).upper(), port.get("detail", "")) if x)
                allowed = Gtk.Box(valign=Gtk.Align.CENTER, tooltip_text="Allowed")
                allowed.add_css_class("allowed-dot")
                rows.append(row(port.get("what", ""), f"Port {port.get('port', '')} ({how})", allowed,
                                icon=port.get("icon") or "app", icon_px=26))
            if not firewall.get("open"):
                rows.append(row("Nothing", "No other device can reach the box."))
            box.append(card)
            card, rows = group("Outbound Internet Connections",
                               lead="What the box itself connects to, for its updates and the time. It "
                                    "sends nothing that names the box or the family.", icon="globe")
            for c in answer.get("connections") or []:
                rows.append(self.connection_row(c))
            box.append(card)

        in_background(lambda: self.ask("security"), got,
                      lambda p: box.append(text(p, "error", wrap=True)))
        return box

    def connection_row(self, c: dict[str, Any]) -> Gtk.Widget:
        """One place the box connects to: what it is and why, allowed, and under the
        chevron its address and when the box connects."""
        allowed = Gtk.Box(valign=Gtk.Align.CENTER, tooltip_text="Allowed")
        allowed.add_css_class("allowed-dot")
        chevron = glyph("chevron-down", 18, "muted")
        more = Gtk.Button(child=chevron, tooltip_text="Details")
        more.add_css_class("bar-button")
        title = c.get("title") or c.get("host", "")
        shown = row(f"{title} ({c['host']})" if c.get("title") else title, c.get("what", ""), allowed, more,
                    **BIG, icon=c.get("icon") or "package")
        line = shown.get_child()
        shown.set_child(None)
        column = Gtk.Box(orientation=Gtk.Orientation.VERTICAL)
        column.append(line)
        details = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=2, margin_top=8, margin_start=40)
        for words in (f"Address: {c.get('host', '')}", f"When: {c.get('when', '')}" if c.get("when") else "",
                      "Sends nothing that names the box or the family."):
            if words:
                details.append(text(words, "row-sub", wrap=True))
        reveal = Gtk.Revealer(child=details, transition_type=Gtk.RevealerTransitionType.SLIDE_DOWN)
        column.append(reveal)
        shown.set_child(column)

        def toggle(_b: Gtk.Button) -> None:
            reveal.set_reveal_child(not reveal.get_reveal_child())
            set_glyph(chevron, "chevron-up" if reveal.get_reveal_child() else "chevron-down")

        more.connect("clicked", toggle)
        return shown

    def apparmor_row(self, apparmor: dict[str, Any]) -> Gtk.Widget:
        """AppArmor, on and greyed out, with the programs it knows under it: a check
        for each it confines, a cross for each it only watches."""
        profiles = apparmor.get("profiles") or []
        shown = row("AppArmor", f"{apparmor.get('enforced', 0)} programs confined" if apparmor.get("on") else "",
                    fixed_switch(bool(apparmor.get("on"))), **BIG, icon="apparmor")
        if not profiles:
            return shown
        line = shown.get_child()
        shown.set_child(None)
        column = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=6)
        column.append(line)
        names = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=4, margin_top=6)
        for profile in profiles:
            on = bool(profile.get("enforced"))
            item = Gtk.Box(spacing=10)
            item.append(glyph("check" if on else "close", 16, "ok" if on else "warn"))
            item.append(text(profile.get("name", ""), "row-sub", hexpand=True))
            item.append(text("Confined" if on else "Watched only", "row-sub"))
            names.append(item)
        more = Gtk.Expander(label=f"The programs ({len(profiles)})", child=names)
        more.add_css_class("row-sub")
        column.append(more)
        shown.set_child(column)
        return shown

    def security_fixes_row(self, updates: dict[str, Any]) -> Gtk.Widget:
        """Security fixes: installed by themselves every night, or now with Check now
        (design §6.1). There's no turning them off."""
        from .app import alive, in_background

        check = Gtk.Button(label="Check now")
        shown = row("Security Updates", "", check, **BIG, icon="updates")
        sub = text("", "row-sub", wrap=True)
        shown.get_child().get_first_child().get_next_sibling().append(sub)

        def show(state: dict[str, Any]) -> None:
            running = bool(state.get("running"))
            check.set_sensitive(not running)
            check.set_label("Checking…" if running else "Check now")
            last = state.get("last_installed")
            said = "Auto updates every night." if state.get("automatic") else "Auto updates are off."
            said += f"\nLast update was {day(last, at=True)}." if last else "\nNo update yet."
            sub.set_text("Checking for security fixes and installing them now." if running else said)

        def poll() -> bool:
            if not alive(shown):
                return False
            in_background(lambda: self.ask("security"), lambda a: show(a.get("updates") or {}))
            return check.get_label() == "Checking…"

        def started(_answer: dict[str, Any]) -> None:
            show({**updates, "running": True})
            GLib.timeout_add_seconds(5, poll)

        check.connect("clicked", lambda _b: in_background(
            lambda: self.ask("security-check-now"), started, lambda p: sub.set_text(p)))
        show(updates)
        if updates.get("running"):
            GLib.timeout_add_seconds(5, poll)
        return shown

    # --- Notifications ------------------------------------------------------------

    def cp_notifications(self) -> Gtk.Widget:
        """Which kinds this admin sees; the notifications themselves are in Log
        Center."""
        from .app import in_background

        box = page()
        past = Gtk.Button(label="See past notifications")
        past.add_css_class("flat-link")
        past.connect("clicked", lambda _b: self.open_tool("logs", "notifications"))
        switches_card, switches = group("Show Notifications For", "Yours alone, like the theme. "
                                        "The last month's are in Log Center.", past, icon="bell")
        box.append(switches_card)

        def got(answer: dict[str, Any]) -> None:
            off = set((self.prefs.get("notifications") or {}).get("off") or [])
            for kind in answer.get("kinds") or []:
                switch = Gtk.Switch(active=kind not in off)
                switch.connect("notify::active",
                               lambda s, _p, k=kind: self.turn_notifications(k, s.get_active()))
                name, about, icon = KINDS.get(kind, (kind.capitalize(), "", "bell"))
                switches.append(row(name, about, switch, **BIG, icon=icon))

        in_background(lambda: self.ask("notifications"), got,
                      lambda p: box.append(text(p, "error", wrap=True)))
        return box

    # --- System Info -----------------------------------------------------------------

    def cp_about(self) -> Gtk.Widget:
        from .app import duration, in_background

        box = page()
        top = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=8, halign=Gtk.Align.CENTER)
        mark = logo(MARK, 52)
        mark.set_halign(Gtk.Align.CENTER)
        top.append(mark)
        name = text(self.box_name or "ZOOMRIYA Box", "about-name")
        name.set_xalign(0.5)
        top.append(name)
        box.append(top)
        holder = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=22)
        box.append(holder)

        def got(answers: tuple[dict[str, Any], ...]) -> None:
            status, resources, storage = answers
            temps = [t.get("celsius") for t in resources.get("temperatures") or []
                     if t.get("celsius") is not None]
            used, total = resources.get("memory_used_bytes", 0), resources.get("memory_total_bytes", 0)
            disks = [f"{size((storage.get('system') or {}).get('size_bytes', 0))} for the box"]
            library = storage.get("library") or {}
            if library.get("size_bytes"):
                disks.append(f"{size(library['size_bytes'])} library")
            card, rows = group()
            listed = [
                ("Name", self.box_name or "Unknown", "tag"),
                ("Model", status.get("model", ""), "box"),
                ("Serial Number", status.get("serial", ""), "barcode"),
                ("Operating System", f"{status.get('os', '')} ({status.get('image_version', '')})", "layers"),
                ("Processor", processor(), "cpu"),
                ("Memory", f"{size(used)} of {size(total)} in use" if total else "Unknown", "memory"),
                ("Disk Capacity", " + ".join(disks), "disk"),
                ("Temperature", f"{max(temps):.0f} °C" if temps else "No sensor", "thermometer"),
                ("Running For", duration(status.get("uptime_seconds", 0)), "maintenance"),
            ]
            for caption, said, icon in listed:
                if said:
                    rows.append(row(caption, "", value(said), **BIG, icon=icon))
            holder.append(card)

            details_card, details = group("Details", icon="info")
            for caption, said, icon in (("System Release", status.get("debian", ""), "layers"),
                                        ("Base Package", status.get("base", ""), "package"),
                                        ("Factory Image", status.get("image_version", ""), "factory"),
                                        ("Kernel", status.get("kernel", ""), "cpu")):
                details.append(row(caption, "", value(said), **BIG, icon=icon))
            holder.append(details_card)

        in_background(lambda: (self.ask("status"), self.ask("resources"), self.ask("storage")),
                      got, lambda p: holder.append(text(p, "error", wrap=True)))
        return box
