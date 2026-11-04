"""Box Desktop's system tools (box-desktop.md §4), as their windows' sections:
Restarts and Restore & Reset (in Control Panel), Storage Manager's overview and
drives, Resource Monitor, Log Center's logs, and asking for the password again
before anything risky (a restore, a reset, a restart)."""

from __future__ import annotations

from collections.abc import Callable
from datetime import datetime
from typing import Any

import gi

gi.require_version("Gtk", "4.0")
from gi.repository import GLib, Gtk  # noqa: E402

LOG_LEVELS = [("errors", "Errors"), ("warnings", "Warnings and errors"), ("everything", "Everything")]
HEALTH = {"healthy": "Healthy", "warning": "Needs checking", "failing": "Failing"}


def size(n: float) -> str:
    for unit in ("bytes", "KB", "MB", "GB", "TB"):
        if abs(n) < 1000 or unit == "TB":
            return f"{n:.0f} {unit}" if unit == "bytes" else f"{n:.1f} {unit}"
        n /= 1000
    return ""


def rate(bps: float) -> str:
    return f"{size(bps)}/s"


def when(stamp: str) -> str:
    """An ISO time as the box's people read it, or "Never"."""
    if not stamp:
        return "Never"
    try:
        moment = datetime.fromisoformat(stamp.replace("Z", "+00:00")).astimezone()
    except ValueError:
        return stamp
    return moment.strftime("%a %-d %b %Y, %H:%M")


def meter(fraction: float, *classes: str) -> Gtk.LevelBar:
    bar = Gtk.LevelBar(min_value=0, max_value=1, value=max(0.0, min(1.0, fraction)))
    bar.add_css_class("meter")
    for name in classes:
        bar.add_css_class(name)
    if fraction > 0.9:
        bar.add_css_class("full")
    return bar


def heading(title: str, aside: str = "") -> Gtk.Widget:
    from .app import label

    row = Gtk.Box(spacing=12)
    row.append(label(title, "page-title", hexpand=True))
    if aside:
        row.append(label(aside, "lead"))
    return row


def badge(state: str) -> Gtk.Widget:
    from .app import label

    shown = label(HEALTH.get(state, "Unknown"), "badge", state if state in HEALTH else "unknown")
    shown.set_valign(Gtk.Align.CENTER)
    return shown


class Tools:
    """Mixed into BoxDesktop: self.ask, self.zid, self.member, self.overlay and
    the helpers in app.py."""

    # --- asking again -------------------------------------------------------------

    def confirm(self, title: str, text: str, button: str,
                action: Callable[..., dict[str, Any]], done: Callable[[dict[str, Any]], None],
                extra: str | None = None) -> None:
        """A sheet over the desktop: the admin's password again (and, with extra,
        one more field, such as the setup code), then action."""
        from .app import in_background, label

        shade = Gtk.Box(halign=Gtk.Align.FILL, valign=Gtk.Align.FILL)
        shade.add_css_class("shade")
        sheet = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=12,
                        halign=Gtk.Align.CENTER, valign=Gtk.Align.CENTER, hexpand=True)
        sheet.add_css_class("card")
        sheet.set_size_request(440, -1)
        sheet.append(label(title, "title"))
        sheet.append(label(text, "lead", wrap=True))
        more = None
        if extra:
            more = Gtk.Entry(placeholder_text=extra)
            sheet.append(more)
        password = Gtk.PasswordEntry(show_peek_icon=True, placeholder_text="Your password")
        sheet.append(password)
        problem = label("", "error", wrap=True)
        problem.set_visible(False)
        sheet.append(problem)
        row = Gtk.Box(spacing=10, halign=Gtk.Align.END)
        cancel = Gtk.Button(label="Cancel")
        go = Gtk.Button(label=button)
        go.add_css_class("danger")
        row.append(cancel)
        row.append(go)
        sheet.append(row)
        shade.append(sheet)

        def close() -> None:
            self.overlay.remove_overlay(shade)

        def run() -> None:
            typed = password.get_text()
            also = more.get_text().strip() if more is not None else None
            go.set_sensitive(False)

            def work() -> dict[str, Any]:
                again = self.zid.sign_in(self.member.username, typed)
                # Only a check: the session it made isn't needed.
                self.zid.sign_out(again)
                if again.id != self.member.id:
                    raise PermissionError("That isn't your password.")
                return action(also) if more is not None else action()

            def finished(answer: dict[str, Any]) -> None:
                close()
                done(answer)

            def refused(text: str) -> None:
                problem.set_text(text)
                problem.set_visible(True)
                password.set_text("")
                go.set_sensitive(True)

            in_background(work, finished, refused)

        cancel.connect("clicked", lambda _b: close())
        go.connect("clicked", lambda _b: run())
        password.connect("activate", lambda _e: run())
        self.overlay.add_overlay(shade)
        (more or password).grab_focus()

    def restart(self) -> None:
        self.confirm("Restart the box?",
                     "Everything on the box stops for a minute or two, the apps included, "
                     "then starts again by itself.", "Restart",
                     lambda: self.ask("restart"), lambda _a: None)

    def restart_now(self) -> None:
        """Straight after a restore or a reset, whose password was just asked:
        not asked again."""
        from .app import in_background

        in_background(lambda: self.ask("restart"), lambda _a: None)

    # --- Restarts (Control Panel) -----------------------------------------------------

    def restarts(self) -> Gtk.Widget:
        from .app import in_background
        from .settings import group, page, row, text, value

        box = page()
        holder = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=22)
        box.append(holder)

        def got(status: dict[str, Any]) -> None:
            while child := holder.get_first_child():
                holder.remove(child)
            if status.get("restart_needed"):
                reasons = ", ".join(status.get("restart_reasons") or []) or "an update"
                now = Gtk.Button(label="Restart now")
                now.add_css_class("primary")
                now.connect("clicked", lambda _b: self.restart())
                card, rows = group("Restart needed")
                rows.append(row(f"A restart finishes {reasons}",
                                "The box does it in the maintenance hour, or now.", now))
                holder.append(card)

            card, rows = group("Updates and restarts")
            rows.append(row("Security fixes", "", value(
                f"By themselves; the last on {when(status.get('last_security_update', ''))}")))
            rows.append(row("Last checked", "", value(when(status.get("last_update_check", "")))))
            hours = [f"{h:02d}:30" for h in range(24)]
            pick = Gtk.DropDown.new_from_strings(hours)
            current = status.get("maintenance_hour", "03:30")
            if current in hours:
                pick.set_selected(hours.index(current))
            save = Gtk.Button(label="Move")
            save.connect("clicked", lambda _b: in_background(
                lambda: self.ask("maintenance-hour", hour=hours[pick.get_selected()]),
                lambda _a: load()))
            rows.append(row("Maintenance hour", "When restarts and updates happen", pick, save))
            holder.append(card)

            card, rows = group("Power")
            shut = Gtk.Button(label="Shut down…")
            shut.connect("clicked", lambda _b: self.shut_down())
            rows.append(row("Shut down", "The box is meant to stay on. Once it's off, nobody can turn "
                            "it on from far away: someone has to press its power button.", shut))
            holder.append(card)

        def load() -> None:
            in_background(lambda: self.ask("status"), got,
                          lambda p: holder.append(text(p, "error", wrap=True)))

        load()
        return box

    def shut_down(self) -> None:
        self.confirm("Shut the box down?",
                     "Every app on the box stops, Family Hub and the apps can't be reached, and "
                     "backups don't run until someone presses the box's power button.",
                     "Shut down", lambda: self.ask("shutdown"), lambda _a: None)

    # --- Restore points (Control Panel) -----------------------------------------------

    def restore_points(self) -> Gtk.Widget:
        """Restore & Reset: from the mildest fix to the most drastic, the OS as
        it was before an update, the system as it left the factory, then
        erasing everything, apart at the bottom."""
        from .app import in_background
        from .settings import group, page, row, text

        box = page()
        card, points = group("Restore points", "The OS as it was before and after each update. "
                             "Restoring brings the OS back; the family's files and the apps' data "
                             "stay as they are. Try this first.")
        box.append(card)

        def got(listed: dict[str, Any]) -> None:
            for point in reversed(listed.get("restore_points") or []):
                ends = []
                if point.get("kind") != "current":
                    back = Gtk.Button(label="Restore")
                    back.connect("clicked", lambda _b, p=point: self.restore(p))
                    ends.append(back)
                points.append(row(when(point.get("date", "")),
                                  point.get("description") or point.get("kind", ""), *ends))

        in_background(lambda: self.ask("restore-point-list"), got,
                      lambda p: box.append(text(p, "error", wrap=True)))

        card, rows = group("Reset")
        go = Gtk.Button(label="Reset the system…")
        go.connect("clicked", lambda _b: self.reset_system())
        rows.append(row("Reset the system",
                        "The system goes back to how it left the factory when the box restarts, then "
                        "installs the latest security fixes. The family's apps, accounts, photos and "
                        "messages stay. For when something is wrong and a restore point doesn't help.",
                        go))
        box.append(card)

        card, rows = group("Erase everything")
        erase = Gtk.Button(label="Erase everything…")
        erase.add_css_class("danger")
        erase.connect("clicked", lambda _b: self.erase_everything())
        rows.append(row("Erase everything",
                        "Every app, account, photo and message is deleted, and the box's history. Only "
                        "the system as it left the factory stays, ready to be set up again, as when it "
                        "was new. It needs the setup code from the sticker.", erase))
        box.append(card)
        return box

    def restore(self, point: dict[str, Any]) -> None:
        self.confirm(
            "Restore the OS?",
            f"The OS goes back to {when(point.get('date', ''))} ({point.get('description', '')}) "
            "when the box next restarts. The family's files and the apps' data stay as they are.",
            "Restore", lambda: self.ask("restore-point-restore", number=point["number"]),
            lambda _a: self.restart_now())

    # --- Storage Manager --------------------------------------------------------------

    def storage_overview(self) -> Gtk.Widget:
        from .app import in_background, label

        box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=16)

        def space(title: str, sub: str, part: dict[str, Any], state: str, kind: str) -> Gtk.Widget:
            group = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=8)
            head = Gtk.Box(spacing=12)
            words = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, hexpand=True)
            words.append(label(title, "page-title"))
            words.append(label(sub, "lead"))
            head.append(words)
            head.append(badge(state))
            group.append(head)
            total, used = part.get("size_bytes") or 0, part.get("used_bytes") or 0
            line = Gtk.Box()
            line.append(label(f"{size(used)} used", "fact", hexpand=True))
            line.append(label(f"{size(part.get('free_bytes', 0))} free of {size(total)}", "fact-name"))
            group.append(line)
            group.append(meter(used / total if total else 0, kind))
            return group

        def got(answers: tuple[dict[str, Any], dict[str, Any]]) -> None:
            storage, health = answers
            drives = health.get("drives") or []
            library = storage.get("library") or {}
            if library.get("path"):
                names = [d.get("name", "") for d in library.get("drives") or []]
                states = [d.get("health") for d in drives if not d.get("system")]
                state = "healthy" if states and all(s == "healthy" for s in states) else \
                    ("warning" if states else "unknown")
                sub = (f"{len(names)} drives, each a copy of the other" if library.get("mirror")
                       else ", ".join(names))
                box.append(space("Library", sub, library, state, "library"))
                box.append(self.library_check(library.get("check") or {}))
            system_state = next((d.get("health") for d in drives if d.get("system")), "unknown")
            box.append(space("The box's own disk", "The OS, the apps and their data",
                             storage.get("system") or {}, system_state, "system"))
            parts = storage.get("system_parts") or []
            if parts:
                legend = Gtk.FlowBox(selection_mode=Gtk.SelectionMode.NONE, max_children_per_line=3)
                for part in parts:
                    legend.append(label(f"{part.get('kind', '').replace('_', ' ').capitalize()} · "
                                        f"{size(part.get('bytes', 0))}", "fact-name"))
                box.append(legend)

        in_background(lambda: (self.ask("storage"), self.ask("disk-health")), got,
                      lambda p: box.append(label(p, "error", wrap=True)))
        return box

    def storage_drives(self) -> Gtk.Widget:
        from .app import in_background, label

        box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=12)
        box.append(heading("Drives"))
        cards = Gtk.FlowBox(selection_mode=Gtk.SelectionMode.NONE, max_children_per_line=2,
                            min_children_per_line=1, column_spacing=12, row_spacing=12,
                            homogeneous=True)
        box.append(cards)

        def got(health: dict[str, Any]) -> None:
            for drive in health.get("drives") or []:
                card = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=8)
                card.add_css_class("drive-card")
                head = Gtk.Box(spacing=8)
                head.append(label(drive.get("model") or drive.get("name", ""), "fact", hexpand=True))
                head.append(badge(drive.get("health", "unknown")))
                card.append(head)
                rows = [("Size", size(drive.get("size_bytes", 0))),
                        ("Place", "The box's own" if drive.get("system") else drive.get("name", ""))]
                if drive.get("temperature_c") is not None:
                    rows.append(("Temperature", f"{drive['temperature_c']} °C"))
                if drive.get("power_on_hours") is not None:
                    rows.append(("In use for", f"{drive['power_on_hours'] // 24} days"))
                grid = Gtk.Grid(column_spacing=16, row_spacing=4)
                for i, (name, value) in enumerate(rows):
                    grid.attach(label(name, "fact-name"), 0, i, 1, 1)
                    grid.attach(label(value, "small-fact"), 1, i, 1, 1)
                card.append(grid)
                for reason in drive.get("reasons") or []:
                    card.append(label(reason, "lead", wrap=True, max_width_chars=34))
                cards.append(card)

        in_background(lambda: self.ask("disk-health"), got,
                      lambda p: box.append(label(p, "error", wrap=True)))
        return box

    # --- Resource Monitor -------------------------------------------------------------

    def resources(self) -> Gtk.Widget:
        from .app import alive, facts, in_background, label

        box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=12)
        cpu_label = label("Processor", "field-label")
        cpu = meter(0)
        mem_label = label("Memory", "field-label")
        mem = meter(0, "memory")
        for widget in (cpu_label, cpu, mem_label, mem):
            box.append(widget)
        holder = Gtk.Box()
        box.append(holder)

        def got(answer: dict[str, Any]) -> None:
            nonlocal holder
            percent = answer.get("cpu_percent", 0)
            cpu_label.set_text(f"Processor: {percent:.0f}%")
            cpu.set_value(percent / 100)
            total, used = answer.get("memory_total_bytes", 0), answer.get("memory_used_bytes", 0)
            mem_label.set_text(f"Memory: {size(used)} of {size(total)}")
            mem.set_value(used / total if total else 0)
            net = answer.get("network") or {}
            disks = answer.get("disks") or {}
            temps = [t.get("celsius") for t in answer.get("temperatures") or [] if t.get("celsius")]
            rows = [
                ("Network", f"in {rate(net.get('in_bps', 0))}, out {rate(net.get('out_bps', 0))}"),
                ("Disks", f"reading {rate(disks.get('read_bps', 0))}, writing {rate(disks.get('write_bps', 0))}"),
                ("Hottest sensor", f"{max(temps):.0f} °C" if temps else "No sensor"),
            ]
            box.remove(holder)
            holder = facts(rows)
            box.append(holder)

        def tick() -> bool:
            if not alive(box):
                return False
            in_background(lambda: self.ask("resources"), got)
            return True

        in_background(lambda: self.ask("resources"), got)
        GLib.timeout_add_seconds(3, tick)
        return box

    # --- Log Center -------------------------------------------------------------------

    def logs(self, kind: str) -> Gtk.Widget:
        from .app import in_background, label

        box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=10)
        bar = Gtk.Box(spacing=8)
        levels = Gtk.DropDown.new_from_strings([name for _k, name in LOG_LEVELS])
        levels.set_selected(1)
        levels.set_visible(kind == "system")
        search = Gtk.SearchEntry(placeholder_text="Search", hexpand=True)
        bar.append(levels)
        bar.append(search)
        box.append(bar)
        entries = Gtk.ListBox(selection_mode=Gtk.SelectionMode.NONE)
        entries.add_css_class("list")
        box.append(entries)
        older = Gtk.Button(label="Show older", halign=Gtk.Align.CENTER)
        older.set_visible(False)
        box.append(older)
        page = {"next": "", "more": False}

        def got(answer: dict[str, Any], more: bool = False) -> None:
            if not more:
                while child := entries.get_first_child():
                    entries.remove(child)
            shown = answer.get("entries") or []
            page["next"] = answer.get("next", "") if answer.get("more") else ""
            older.set_visible(bool(page["next"]))
            if not shown and not more:
                entries.append(label("Nothing here.", "lead"))
            for entry in shown:
                row = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=2)
                head = f"{when(entry.get('time', ''))}  ·  {entry.get('source', '')}"
                if entry.get("by"):
                    head += f"  ·  {entry['by']}"
                row.append(label(head, "fact-name", entry.get("level", "")))
                row.append(label(entry.get("message", ""), "log-line", wrap=True))
                entries.append(row)

        def load(*_a: Any, more: bool = False) -> None:
            fields = {"kind": kind, "search": search.get_text() or None}
            if kind == "system":
                fields["priority"] = LOG_LEVELS[levels.get_selected()][0]
            if more:
                fields["cursor"] = page["next"]
            in_background(lambda: self.ask("logs", **fields), lambda a: got(a, more),
                          lambda p: entries.append(label(p, "error", wrap=True)))

        older.connect("clicked", lambda _b: load(more=True))
        levels.connect("notify::selected", load)
        search.connect("search-changed", load)
        load()
        return box


    def log_notifications(self) -> Gtk.Widget:
        """Log Center's Notifications: the last month's, newest first, each with
        its kind, the ones this admin turned off included."""
        from .app import in_background, label
        from .control import KINDS

        box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=10)
        box.append(label("What the box told its admins in the last month. Which kinds show on "
                         "the bell is in Control Panel › Notifications.", "lead", wrap=True))
        entries = Gtk.ListBox(selection_mode=Gtk.SelectionMode.NONE)
        entries.add_css_class("list")
        box.append(entries)

        def got(answer: dict[str, Any]) -> None:
            listed = answer.get("notifications") or []
            if not listed:
                entries.append(label("Nothing in the last month.", "lead"))
            for n in reversed(sorted(listed, key=lambda n: n.get("time", ""))):
                row = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=2)
                kind = n.get("kind", "")
                row.append(label(f"{when(n.get('time', ''))}  ·  {KINDS.get(kind, kind.capitalize())}",
                                 "fact-name", n.get("level", "")))
                row.append(label(n.get("message", ""), "fact", wrap=True))
                entries.append(row)

        in_background(lambda: self.ask("notifications"), got,
                      lambda p: entries.append(label(p, "error", wrap=True)))
        return box


class MoreTools:
    """The rest of the tools' sections: the library check, Resource Monitor's
    apps and temperatures, and Reset."""

    # --- the library check (Storage Manager) -------------------------------------------

    def library_check(self, check: dict[str, Any]) -> Gtk.Widget:
        """That both drives of the library hold the same: start, pause, resume."""
        from .app import alive, in_background, label

        box = Gtk.Box(spacing=12)
        box.add_css_class("panel-box")
        words = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=4, hexpand=True)
        box.append(words)
        button = Gtk.Button(valign=Gtk.Align.CENTER)
        box.append(button)
        action = {"name": ""}

        def show(state: dict[str, Any]) -> None:
            while child := words.get_first_child():
                words.remove(child)
            percent = state.get("percent", 0)
            if state.get("running") and not state.get("paused"):
                words.append(label(f"Checking that both copies match · {percent:.0f}%", "fact"))
                words.append(label(f"Started {when(state.get('started', ''))}.", "fact-name"))
                words.append(meter(percent / 100, "library"))
                button.set_label("Pause")
                action["name"] = "pause"
            elif state.get("paused"):
                words.append(label(f"The check is paused at {percent:.0f}%", "fact"))
                words.append(label("It carries on from there when you resume it.", "fact-name"))
                button.set_label("Resume")
                action["name"] = "resume"
            else:
                if state.get("never_run"):
                    last = "It hasn't been checked yet."
                elif state.get("errors"):
                    last = f"The last check found {state['errors']} problems and repaired what it could."
                else:
                    last = "The last check found nothing needing repair."
                words.append(label("Check that both copies match", "fact"))
                words.append(label(last + " The box also checks once a month by itself.", "fact-name",
                                   wrap=True, max_width_chars=50))
                button.set_label("Check now")
                action["name"] = "start"

        def act() -> None:
            button.set_sensitive(False)
            in_background(lambda: self.ask("storage-check", action=action["name"]),
                          lambda _a: (button.set_sensitive(True), refresh()),
                          lambda p: (button.set_sensitive(True), words.append(label(p, "error", wrap=True))))

        def refresh() -> bool:
            if not alive(box):
                return False
            in_background(lambda: self.ask("storage"),
                          lambda s: show(((s.get("library") or {}).get("check")) or {}))
            return True

        button.connect("clicked", lambda _b: act())
        show(check)
        GLib.timeout_add_seconds(10, refresh)
        return box

    # --- Storage Manager: Space -------------------------------------------------------------

    def storage_space(self) -> Gtk.Widget:
        from .app import in_background, label

        names = {"databases": "The apps' databases", "app-images": "The apps themselves",
                 "models-and-maps": "Myira's models and the maps", "app-data": "Photos, files and the apps' data",
                 "restore-points": "Restore points", "os": "ZOOMRIYA OS"}
        box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=12)
        box.append(heading("Space"))
        box.append(label("What takes the space on the box's own disk.", "lead", wrap=True))
        grid = Gtk.Grid(column_spacing=18, row_spacing=10)
        grid.add_css_class("facts")
        box.append(grid)

        def got(storage: dict[str, Any]) -> None:
            system = storage.get("system") or {}
            total = system.get("used_bytes") or 0
            parts = sorted(storage.get("system_parts") or [], key=lambda p: -(p.get("bytes") or 0))
            if not parts:
                grid.attach(label("Not counted yet: it's counted every few hours.", "lead"), 0, 0, 1, 1)
            for row, part in enumerate(parts):
                share = (part.get("bytes") or 0) / total if total else 0
                grid.attach(label(names.get(part.get("kind", ""), part.get("kind", "")), "fact-name"),
                            0, row, 1, 1)
                bar = meter(share)
                bar.set_hexpand(True)
                bar.set_valign(Gtk.Align.CENTER)
                grid.attach(bar, 1, row, 1, 1)
                grid.attach(label(size(part.get("bytes") or 0), "small-fact"), 2, row, 1, 1)

        in_background(lambda: self.ask("storage"), got,
                      lambda p: box.append(label(p, "error", wrap=True)))
        return box

    # --- Resource Monitor: by app, temperature --------------------------------------------

    def resources_by_app(self) -> Gtk.Widget:
        from .app import alive, in_background, label

        box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=10)
        box.append(heading("By app"))
        box.append(label("The apps and the box's own services, busiest first. CPU is a share of the "
                         "whole box.", "lead", wrap=True))
        grid = Gtk.Grid(column_spacing=18, row_spacing=8)
        grid.add_css_class("facts")
        box.append(grid)

        def got(answer: dict[str, Any]) -> None:
            while child := grid.get_first_child():
                grid.remove(child)
            for col, title in enumerate(("", "CPU", "Memory")):
                grid.attach(label(title, "field-label"), col, 0, 1, 1)
            for row, use in enumerate(answer.get("services") or [], start=1):
                name = use.get("name") or use.get("unit", "")
                kind = "an app" if use.get("kind") == "app" else "the box"
                grid.attach(label(f"{name}", "fact", hexpand=True, tooltip_text=kind), 0, row, 1, 1)
                grid.attach(label(f"{use.get('cpu_percent', 0):.1f}%", "small-fact"), 1, row, 1, 1)
                grid.attach(label(size(use.get("memory_bytes", 0)), "small-fact"), 2, row, 1, 1)

        def tick() -> bool:
            if not alive(box):
                return False
            in_background(lambda: self.ask("resources"), got)
            return True

        in_background(lambda: self.ask("resources"), got)
        GLib.timeout_add_seconds(5, tick)
        return box

    def temperatures(self) -> Gtk.Widget:
        from .app import alive, in_background, label

        box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=10)
        box.append(heading("Temperature"))
        grid = Gtk.Grid(column_spacing=18, row_spacing=8)
        grid.add_css_class("facts")
        box.append(grid)

        def got(answer: dict[str, Any]) -> None:
            while child := grid.get_first_child():
                grid.remove(child)
            temps = answer.get("temperatures") or []
            fans = answer.get("fans") or []
            if not temps and not fans:
                grid.attach(label("This box reports no sensors.", "lead"), 0, 0, 1, 1)
            row = 0
            for sensor in temps:
                name = sensor.get("label") or sensor.get("sensor", "")
                grid.attach(label(name, "fact-name", hexpand=True), 0, row, 1, 1)
                value = label(f"{sensor.get('celsius', 0):.0f} °C", "fact",
                              "warn" if sensor.get("celsius", 0) >= 80 else "ok")
                grid.attach(value, 1, row, 1, 1)
                row += 1
            for fan in fans:
                grid.attach(label(fan.get("label") or fan.get("sensor", "Fan"), "fact-name", hexpand=True),
                            0, row, 1, 1)
                grid.attach(label(f"{fan.get('rpm', 0)} rpm", "fact"), 1, row, 1, 1)
                row += 1

        def tick() -> bool:
            if not alive(box):
                return False
            in_background(lambda: self.ask("resources"), got)
            return True

        in_background(lambda: self.ask("resources"), got)
        GLib.timeout_add_seconds(5, tick)
        return box

    # --- Reset (Control Panel) ------------------------------------------------------------

    def reset_system(self) -> None:
        self.confirm("Reset the system?",
                     "The system goes back to how it left the factory when the box restarts. The family's "
                     "apps and data stay.", "Reset",
                     lambda: self.ask("factory-reset", level="system"), lambda _a: self.restart_now())

    def erase_everything(self) -> None:
        self.confirm("Erase everything?",
                     "Everything on the box is deleted when it restarts: every app, account, photo and "
                     "message. This can't be undone. Type the setup code from the sticker and your "
                     "password.", "Erase everything",
                     lambda code: self.ask("factory-reset", level="everything", setup_code=code),
                     lambda _a: self.restart_now(), extra="Setup code, like ABCD-EFGH")
