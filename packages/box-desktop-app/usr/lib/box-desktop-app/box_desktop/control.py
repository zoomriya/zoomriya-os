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

from zoomriya_setup.app import fixed_settings, languages, locale_for, regions, time_zones  # noqa: E402
from zoomriya_setup.brand import MARK, logo  # noqa: E402
from zoomriya_setup.screen import set_keyboard, set_left_handed  # noqa: E402

from .display import ORIENTATIONS, SCALES, Mode, ratio  # noqa: E402
from .icons import glyph, set_glyph  # noqa: E402
from .settings import group, page, row, segmented, slider, text, tile, value, wide  # noqa: E402
from .tools import size, when  # noqa: E402

#: The agent's kinds of notification, as the person reads them.
KINDS = {
    "drive": "A drive needs checking",
    "update": "Updates",
    "restart": "Restarts",
    "check": "The library check",
    "network": "The network",
}
#: How long a new screen setting waits to be kept before going back.
KEEP_SECONDS = 15


def dropdown(names: list[str], search: bool = False) -> Gtk.DropDown:
    pick = Gtk.DropDown.new_from_strings(names)
    pick.set_enable_search(search)
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


class ControlPanel:
    """Mixed into BoxDesktop, like Tools."""

    # --- Network ------------------------------------------------------------------

    def cp_network(self) -> Gtk.Widget:
        """Each Ethernet port, then how the chosen one gets its address."""
        from .app import in_background

        box = page()
        ports = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=22)
        box.append(ports)

        settings_group, rows = group("Settings", "The box tries new settings for a few minutes first: "
                                         "if it can't be reached, they go back by themselves.")
        title = settings_group.get_first_child().get_first_child()
        box.append(settings_group)
        port = {"name": "", "links": []}
        how = {"mode": "dhcp"}

        def chose_how(mode: str) -> None:
            how["mode"] = mode
            for r in fixed_rows:
                r.set_sensitive(mode == "fixed")

        modes = segmented([("dhcp", "From the router"), ("fixed", "Fixed")], "dhcp", chose_how)
        rows.append(row("How it gets its address", "From the router (DHCP), or one that never "
                        "changes", modes))
        entries: dict[str, Gtk.Entry] = {}
        fixed_rows = []
        for key, caption, hint in [
            ("address", "Box's address", "192.168.1.40"), ("mask", "Subnet mask", "255.255.255.0"),
            ("router", "Router", "192.168.1.1"), ("dns", "Name servers", "192.168.1.1"),
        ]:
            entry = Gtk.Entry(placeholder_text=hint)
            entries[key] = entry
            fixed_rows.append(row(caption, "", entry))
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
            title.set_text(f"Settings of {name}" if name else "Settings")
            is_fixed = link.get("mode") == "fixed"
            modes.get_first_child().set_active(not is_fixed)
            modes.get_last_child().set_active(is_fixed)
            chose_how("fixed" if is_fixed else "dhcp")
            fixed = link.get("fixed") or {}
            if fixed:
                address, _, bits = fixed.get("address", "").partition("/")
                entries["address"].set_text(address)
                if bits:
                    entries["mask"].set_text(str(ipaddress.IPv4Network(f"0.0.0.0/{bits}").netmask))
                entries["router"].set_text(fixed.get("gateway", ""))
                entries["dns"].set_text(", ".join(fixed.get("dns") or []))
            undo = link.get("undo_at")
            keep.set_visible(bool(undo))
            now = (link.get("ipv4") or ["none"])[0].split("/")[0]
            state.set_text(f"Now: {now}." + (f" New settings go back by themselves at {when(undo)} "
                                             "unless you keep them." if undo else ""))

        def got(answer: dict[str, Any]) -> None:
            links = [x for x in answer.get("links", []) if x.get("kind") == "ethernet"]
            port["links"] = links
            while child := ports.get_first_child():
                ports.remove(child)
            for link in links:
                card, card_rows = group(f"Ethernet ({link.get('name', '')})")
                if link.get("connected"):
                    speed = link.get("speed_mbps") or 0
                    status = "Connected" + (f" – {speed:,} Mb/s" if speed else "")
                    address = (link.get("ipv4") or [""])[0].split("/")[0]
                    sub = f"{address} · " if address else ""
                    sub += "fixed address" if link.get("mode") == "fixed" else "address from the router"
                else:
                    status, sub = "Cable unplugged", link.get("mac", "")
                gear = Gtk.Button(tooltip_text=f"Settings of {link.get('name', '')}")
                gear.add_css_class("bar-button")
                gear.set_child(glyph("gear", 18))
                gear.connect("clicked", lambda _b, n=link.get("name", ""): show_port(n))
                card_rows.append(row(status, sub, gear))
                ports.append(card)
            if not links:
                card, card_rows = group("Ethernet")
                card_rows.append(row("No network port found", "The box needs a cable to the router."))
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
            card, rows = group("Display")
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
                               "For the box's own screen, whoever signs in. A change shows at once: "
                               f"keep it, or it goes back by itself after {KEEP_SECONDS} seconds.")
            sizes = out.sizes()
            size_pick = dropdown([f"{w} × {h}" + (f" ({ratio(w, h)})" if ratio(w, h) else "")
                                  for w, h in sizes])
            rate_pick = dropdown([])
            turn_pick = dropdown([name for _k, name in ORIENTATIONS])
            scale_pick = dropdown([f"{s * 100:.0f} %" for s in SCALES])
            rows.append(row("Orientation", "Which way round the screen stands", turn_pick))
            rows.append(row("Resolution", "More shows more, smaller", size_pick))
            rows.append(row("Refresh rate", "How often the picture is drawn each second", rate_pick))
            rows.append(row("Scale", "Bigger text and controls on a big, sharp screen", scale_pick))
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
            listed = self.sound.devices(device)
            if len(listed) < 2:
                return value(listed[0][1] if listed else "None found")
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

        card, rows = group("Output")
        test = Gtk.Button(label="Test")
        test.connect("clicked", lambda _b: self.sound.play_test())
        rows.append(row("Speakers", "", device_pick(OUTPUT), test))
        rows.append(row("Volume", "", volume(OUTPUT, "volume", "muted")))
        box.append(card)

        card, rows = group("Input")
        rows.append(row("Microphone", "", device_pick(INPUT)))
        rows.append(row("Volume", "", volume(INPUT, "mic", "mic-off")))
        self.mic_level = Gtk.LevelBar(min_value=0, max_value=1, hexpand=True)
        self.mic_level.add_css_class("mic")
        self.mic_level.set_size_request(260, -1)
        rows.append(row("What it hears", "Speak to see it move", self.mic_level))
        box.append(card)
        return box

    # --- Keyboard and mouse ---------------------------------------------------------

    def cp_keyboard(self) -> Gtk.Widget:
        """Keyboard & Mouse: each admin's own, like the theme (box-desktop.md §4)."""
        from .app import in_background

        box = page()
        card, rows = group("Keyboard", "The box uses the last layout an admin saved, and yours "
                                       "whenever you sign in here.")
        pick = dropdown([])
        rows.append(row("Layout", "Of a keyboard plugged into the box", pick))
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

        card, rows = group("Mouse", "For a mouse plugged into the box. Yours alone, like the theme.")
        primary = "right" if self.prefs.get("handed") == "left" else "left"
        rows.append(row("Primary button", "The button that clicks; the other one opens menus",
                        segmented([("left", "Left"), ("right", "Right")], primary, handed)))
        rows.append(row("Pointer speed", "", slider("Slow", "Fast", mouse.get("speed", 0.0),
                                                    lambda v: save_mouse(speed=round(v, 2)), -1, 1)))
        accel = Gtk.Switch(active=mouse.get("accel", True))
        accel.connect("notify::active", lambda s, _p: save_mouse(accel=s.get_active()))
        rows.append(row("Mouse acceleration", "The pointer goes further when you move the mouse "
                        "quickly; right for most people", accel))
        box.append(card)

        card, rows = group("Scroll direction")
        traditional = Gtk.CheckButton(active=not mouse.get("natural", False))
        natural = Gtk.CheckButton(active=mouse.get("natural", False), group=traditional)
        natural.connect("toggled", lambda b: save_mouse(natural=b.get_active()))
        rows.append(row("Traditional", "Scrolling moves the view", traditional))
        rows.append(row("Natural", "Scrolling moves the content, as on a phone", natural))
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
        card, rows = group("Language & Region", "ZOOMRIYA's own screens are in English for now.")
        langs = languages()
        places = regions()
        lang_pick = dropdown([name for _c, name in langs], search=True)
        region_pick = dropdown([name for _c, name in places], search=True)
        for pick in (lang_pick, region_pick):
            pick.set_selected(Gtk.INVALID_LIST_POSITION)
        rows.append(row("Language", "The box's language", lang_pick))
        rows.append(row("Country or region", "Where the box is: its dates and numbers follow it",
                        region_pick))
        box.append(card)
        done = text("", "group-note")
        box.append(done)

        def save() -> None:
            if Gtk.INVALID_LIST_POSITION in (lang_pick.get_selected(), region_pick.get_selected()):
                return
            lang = langs[lang_pick.get_selected()][0]
            region = places[region_pick.get_selected()][0]
            locale = locale_for(lang, region)
            in_background(lambda: (self.ask("region-set", region=region),
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
            for pick in (lang_pick, region_pick):
                pick.connect("notify::selected", lambda *_a: save())

        in_background(lambda: self.ask("setup-status"), got)
        return box

    # --- Date and time ------------------------------------------------------------

    def cp_time(self) -> Gtk.Widget:
        from .app import in_background

        box = page()
        card, rows = group("Date and time")
        zones = time_zones()
        pick = dropdown([z.replace("_", " ") for z in zones], search=True)
        pick.set_selected(Gtk.INVALID_LIST_POSITION)
        clock = value("")
        now = value(GLib.DateTime.new_now_local().format("%a %-d %b %Y, %H:%M"))
        rows.append(row("Now", "", now))
        rows.append(row("Time zone", "", pick))
        rows.append(row("Clock", "", clock))
        box.append(card)
        done = text("", "group-note")
        box.append(done)

        def got(status: dict[str, Any]) -> None:
            zone = utc(status.get("zone", "UTC"))
            if zone in zones:
                pick.set_selected(zones.index(zone))
            clock.set_text("Set by itself from the internet" if status.get("clock_synced") else
                           "Not yet set from the internet (it is once the box is online)")
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
            opened = ", ".join(f"{p['port']}/{p['protocol']} ({p['what']})" for p in firewall.get("open") or [])
            if integrity.get("checked") is None:
                files = "Not checked yet (every night)"
            elif integrity.get("clean"):
                files = f"As the last update left them ({when(integrity['checked'])})"
            else:
                files = f"Changed outside an update: {', '.join(integrity.get('changes') or [])}"

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

            card, rows = group("Protection")
            for name, said in (
                ("Security fixes", "Installed by themselves" if updates.get("automatic") else "Off"),
                ("Firewall", ("On" if firewall.get("on") else "Off") + (f"; open: {opened}" if opened else "; nothing open")),
                ("The OS's files", files),
                ("AppArmor", f"On, {apparmor.get('enforced', 0)} programs confined" if apparmor.get("on") else "Off"),
                ("Secure Boot", secure_boot.capitalize()),
            ):
                rows.append(row(name, "", value(said)))
            box.append(card)
            card, rows = group("Who the box talks to on the internet")
            for c in answer.get("connections") or []:
                rows.append(row(c.get("host", ""), c.get("what", "")))
            box.append(card)

        in_background(lambda: self.ask("security"), got,
                      lambda p: box.append(text(p, "error", wrap=True)))
        return box

    # --- Notifications ------------------------------------------------------------

    def cp_notifications(self) -> Gtk.Widget:
        """Which kinds this admin sees; the notifications themselves are in Log
        Center."""
        from .app import in_background

        box = page()
        past = Gtk.Button(label="See past notifications")
        past.add_css_class("flat-link")
        past.connect("clicked", lambda _b: self.open_tool("logs", "notifications"))
        switches_card, switches = group("Show notifications for", "Yours alone, like the theme. "
                                        "The last month's are in Log Center.", past)
        box.append(switches_card)

        def got(answer: dict[str, Any]) -> None:
            off = set((self.prefs.get("notifications") or {}).get("off") or [])
            for kind in answer.get("kinds") or []:
                switch = Gtk.Switch(active=kind not in off)
                switch.connect("notify::active",
                               lambda s, _p, k=kind: self.turn_notifications(k, s.get_active()))
                switches.append(row(KINDS.get(kind, kind.capitalize()), "", switch))

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
            status, resources, setup, storage = answers
            temps = [t.get("celsius") for t in resources.get("temperatures") or []
                     if t.get("celsius") is not None]
            used, total = resources.get("memory_used_bytes", 0), resources.get("memory_total_bytes", 0)
            disks = [f"{size((storage.get('system') or {}).get('size_bytes', 0))} for the box"]
            library = storage.get("library") or {}
            if library.get("size_bytes"):
                disks.append(f"{size(library['size_bytes'])} library")
            card, rows = group()
            listed = [
                ("Name", self.box_name or "Unknown"),
                ("Model", status.get("model", "")),
                ("Serial number", status.get("serial", "")),
                ("System OS", f"{status.get('os', '')} ({status.get('image_version', '')})"),
                ("Processor", processor()),
                ("Memory", f"{size(used)} of {size(total)} in use" if total else "Unknown"),
                ("Disk capacity", " + ".join(disks)),
                ("Temperature", f"{max(temps):.0f} °C" if temps else "No sensor"),
                ("Running for", duration(status.get("uptime_seconds", 0))),
                ("Time zone", utc(setup.get("zone", "")).replace("_", " ")),
                ("Signed in", f"{self.member.name} ({self.member.username})" if self.member else ""),
            ]
            for caption, said in listed:
                if said:
                    rows.append(row(caption, "", value(said)))
            holder.append(card)

            details_card, details = group("Details")
            for caption, said in (("System release", status.get("debian", "")),
                                  ("zoomriya-base", status.get("base", "")),
                                  ("Factory image", status.get("image_version", "")),
                                  ("Kernel", status.get("kernel", ""))):
                details.append(row(caption, "", value(said)))
            holder.append(details_card)

        in_background(lambda: (self.ask("status"), self.ask("resources"), self.ask("setup-status"),
                               self.ask("storage")),
                      got, lambda p: holder.append(text(p, "error", wrap=True)))
        return box
