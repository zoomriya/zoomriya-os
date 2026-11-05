"""The USB installer, full screen on the computer it's installing (design §10).

Six steps down the side: Language picks the installer's language, Welcome
checks the computer can take ZOOMRIYA OS,
Drive picks where it goes and shows the drive before and after, Summary asks
once more before anything is erased, Install copies it while a slideshow tells
what the box does, and Done restarts into the box's first setup.
"""

from __future__ import annotations

import math
import os
import threading
from collections.abc import Callable
from pathlib import Path
from typing import Any

import cairo
import gi

gi.require_version("Gtk", "4.0")
gi.require_version("Gdk", "4.0")
gi.require_foreign("cairo")
from gi.repository import Gdk, Gio, GLib, Gtk, Pango  # noqa: E402

from .brand import MARK, WORDMARK, logo  # noqa: E402
from .disks import MIN_BYTES, Disk, layout_after, size_text  # noqa: E402
from .i18n import LANGUAGES, N_, _  # noqa: E402
from .icons import icon as line_icon  # noqa: E402
from . import i18n  # noqa: E402
from .install import LOG, STAGES, InstallError, overall  # noqa: E402

STEPS = [N_("Language"), N_("Welcome"), N_("Drive"), N_("Summary"), N_("Install"), N_("Done")]
LANGUAGE, WELCOME, DRIVE, SUMMARY, INSTALL, DONE = range(len(STEPS))
MIN_MEMORY = 4 * 1024**3
#: Narrower than this, the side shows the steps' numbers only.
COMPACT_WIDTH = 900
SLIDE_SECONDS = 7

#: The bars' colours: the drive's partitions now, then ZOOMRIYA OS's.
NOW_COLOURS = ["#8e7cc3", "#e69138", "#6aa84f", "#c27ba0", "#45818e", "#a64d79"]
FREE_COLOUR = "#dfe5ec"
AFTER_COLOURS = {"EFI": "#7fb2ea", "Recovery": "#f0a95b", "ZOOMRIYA OS": "#1765c1"}

SLIDES = [
    ("home", N_("Your files stay at home"),
     N_("Everything stays on the box, at home, not in someone else's cloud."),
     ("#1765c1", "#3b8ce0")),
    ("shield", N_("Security fixes install by themselves"),
     N_("OS security updates install automatically."),
     ("#0f766e", "#14a597")),
    ("restore", N_("Restore points"),
     N_("A restore point before every update. One click in Box Desktop goes back; your files stay."),
     ("#5b3fb8", "#8462e0")),
    ("fallback", N_("A kernel that won't start falls back"),
     N_("If a new kernel won't start, the box falls back to the previous one."),
     ("#b4531c", "#e07a3b")),
    ("account", N_("One account: ZOOMRIYA ID"),
     N_("One sign-in for the box and every ZOOMRIYA app."),
     ("#1d6f42", "#2f9a5e")),
    ("lock", N_("Closed unless you open it"),
     N_("A firewall blocks anything uninvited; remote access stays off until an admin enables it."),
     ("#24364b", "#3d5874")),
]


def rgb(colour: str) -> tuple[float, float, float]:
    colour = colour.lstrip("#")
    return tuple(int(colour[i:i + 2], 16) / 255 for i in (0, 2, 4))  # type: ignore[return-value]


def rounded(cr: Any, x: float, y: float, w: float, h: float, r: float) -> None:
    r = min(r, w / 2, h / 2)
    cr.new_sub_path()
    cr.arc(x + w - r, y + r, r, -math.pi / 2, 0)
    cr.arc(x + w - r, y + h - r, r, 0, math.pi / 2)
    cr.arc(x + r, y + h - r, r, math.pi / 2, math.pi)
    cr.arc(x + r, y + r, r, math.pi, 3 * math.pi / 2)
    cr.close_path()


def bar_segments(disk: Disk) -> list[tuple[str, int, str]]:
    """The drive now: its partitions, and the space no partition uses."""
    segments = [(part.what, part.size, NOW_COLOURS[i % len(NOW_COLOURS)])
                for i, part in enumerate(disk.parts)]
    free = disk.size - sum(part.size for part in disk.parts)
    if free > disk.size * 0.01:
        segments.append((_("Free space") if disk.parts else _("Empty"), free, FREE_COLOUR))
    return segments


def after_segments(disk: Disk) -> list[tuple[str, int, str]]:
    return [(name, size, AFTER_COLOURS[name]) for name, size in layout_after(disk.size)]


def memory() -> int:
    try:
        for line in Path("/proc/meminfo").read_text().splitlines():
            if line.startswith("MemTotal:"):
                return int(line.split()[1]) * 1024
    except (OSError, ValueError):
        pass
    return 0


def memory_text(size: int) -> str:
    """Memory as it's sold: in whole GB of 1024³ bytes, less what the firmware keeps."""
    return f"{round(size / 1024**3)} GB"


def label(text: str, *classes: str, wrap: bool = True, shorten: bool = True) -> Gtk.Label:
    widget = Gtk.Label(label=text, xalign=0, wrap=wrap)
    if wrap:
        widget.set_max_width_chars(64)
    elif shorten:
        # A small screen shortens it rather than widening the page.
        widget.set_ellipsize(Pango.EllipsizeMode.END)
    for name in classes:
        widget.add_css_class(name)
    return widget


def swatch(colour: str, size: int = 12) -> Gtk.DrawingArea:
    area = Gtk.DrawingArea(content_width=size, content_height=size, valign=Gtk.Align.CENTER)

    def draw(_a: Any, cr: Any, w: int, h: int) -> None:
        rounded(cr, 0, 0, w, h, 3)
        cr.set_source_rgb(*rgb(colour))
        cr.fill()

    area.set_draw_func(draw)
    return area


class Bar(Gtk.Box):
    """A drive drawn as one bar, a part per partition, with its legend under it."""

    def __init__(self, caption: str) -> None:
        super().__init__(orientation=Gtk.Orientation.VERTICAL, spacing=8)
        self.segments: list[tuple[str, int, str]] = []
        self.append(label(caption, "field-label"))
        self.area = Gtk.DrawingArea(content_height=30, hexpand=True)
        self.area.set_draw_func(self.draw)
        self.append(self.area)
        self.legend = Gtk.FlowBox(selection_mode=Gtk.SelectionMode.NONE, column_spacing=18,
                                  row_spacing=4, max_children_per_line=3)
        self.append(self.legend)

    def show(self, segments: list[tuple[str, int, str]]) -> None:
        self.segments = segments
        self.area.queue_draw()
        while child := self.legend.get_first_child():
            self.legend.remove(child)
        for name, size, colour in segments:
            item = Gtk.Box(spacing=8)
            item.append(swatch(colour))
            item.append(label(_(name), "legend-name", wrap=False))
            item.append(label(size_text(size), "legend-size", wrap=False))
            self.legend.append(item)

    def draw(self, _area: Any, cr: Any, width: int, height: int) -> None:
        total = sum(size for _, size, _ in self.segments) or 1
        rounded(cr, 0, 0, width, height, 8)
        cr.clip()
        cr.set_source_rgb(*rgb(FREE_COLOUR))
        cr.paint()
        x = 0.0
        # Small partitions still show: a few pixels each, taken from the biggest.
        widths = [max(width * size / total, 6) for _, size, _ in self.segments]
        if widths:
            biggest = widths.index(max(widths))
            widths[biggest] -= sum(widths) - width
        for (_name, _size, colour), w in zip(self.segments, widths):
            cr.rectangle(x, 0, w, height)
            cr.set_source_rgb(*rgb(colour))
            cr.fill()
            x += w
            if x < width:
                cr.rectangle(x - 1, 0, 2, height)
                cr.set_source_rgb(1, 1, 1)
                cr.fill()


class InstallerWindow(Gtk.ApplicationWindow):
    def __init__(self, app: Gtk.Application, backend: Any) -> None:
        super().__init__(application=app, title=_("Install ZOOMRIYA OS"))
        self.backend = backend
        self.step = LANGUAGE
        self.disks: list[Disk] = []
        self.disk: Disk | None = None
        self.installing = False
        self.slide = 0

        self.compact = False
        self.whole = whole = Gtk.Box()
        whole.add_css_class("installer")
        self.set_child(whole)
        whole.append(self.sidebar())

        main = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, hexpand=True)
        main.add_css_class("main")
        whole.append(main)

        self.stack = Gtk.Stack(transition_type=Gtk.StackTransitionType.CROSSFADE,
                               vexpand=True, hhomogeneous=False, vhomogeneous=False)
        for step, page in enumerate([self.language_page(), self.welcome_page(), self.drive_page(),
                                     self.summary_page(), self.install_page(), self.done_page()]):
            scroll = Gtk.ScrolledWindow(hscrollbar_policy=Gtk.PolicyType.NEVER)
            page.add_css_class("content")
            scroll.set_child(page)
            self.stack.add_named(scroll, str(step))
        main.append(self.stack)

        self.buttons = buttons = Gtk.Box(spacing=10)
        buttons.add_css_class("buttons")
        self.error = label("", "error")
        self.error.set_hexpand(True)
        self.error.set_visible(False)
        buttons.append(self.error)
        spacer = Gtk.Box(hexpand=True)
        buttons.append(spacer)
        self.busy = Gtk.Spinner()
        buttons.append(self.busy)
        self.back = Gtk.Button(label=_("Back"))
        self.back.connect("clicked", lambda _b: self.go(self.step - 1))
        # Done: switched off instead of restarted, as at the factory, so the
        # box's first start is its owner's.
        self.power = Gtk.Button(label=_("Power off"))
        self.power.connect("clicked", lambda _b: self.backend.power_off())
        buttons.append(self.power)
        self.next = Gtk.Button(label=_("Next"))
        self.next.add_css_class("primary")
        self.next.connect("clicked", lambda _b: self.confirm())
        buttons.append(self.back)
        buttons.append(self.next)
        main.append(buttons)

        #: The internet: None while checking, then True or False.
        self.online: bool | None = None
        self.look_for_disks()
        self.check_internet()
        self.go(LANGUAGE)

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
        sub = label(_("OS Installer"), "brand-sub", wrap=False, shorten=False)
        brand.append(sub)
        self.side_words.append(sub)
        side.append(brand)

        self.step_rows: list[tuple[Gtk.Box, Gtk.Label]] = []
        for i, name in enumerate(STEPS):
            row = Gtk.Box(spacing=12)
            row.add_css_class("step")
            marker = label(str(i + 1), "step-marker", wrap=False, shorten=False)
            marker.set_xalign(0.5)
            row.append(marker)
            words = label(_(name), "step-name", wrap=False, shorten=False)
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

    def go(self, step: int) -> None:
        self.step = max(LANGUAGE, min(step, DONE))
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
            marker.set_yalign(0.5)
        self.back.set_visible(self.step in (WELCOME, DRIVE, SUMMARY))
        self.power.set_visible(self.step == DONE and not self.failed)
        self.buttons.set_visible(self.step != INSTALL)
        self.next.remove_css_class("danger")
        done_label = _("Try again") if self.failed else _("Restart now")
        self.next.set_label({SUMMARY: _("Erase and install"), DONE: done_label}.get(self.step, _("Next")))
        if self.step == SUMMARY:
            self.next.add_css_class("danger")
            self.show_summary()
        if self.step == INSTALL:
            self.start_install()
        self.update_next()

    def confirm(self) -> None:
        if self.step == DONE:
            if self.failed:
                self.go(DRIVE)
            else:
                self.backend.restart()
        else:
            self.go(self.step + 1)

    def update_next(self) -> None:
        ready = {
            WELCOME: all(ok for _, ok in self.requirements()),
            DRIVE: self.disk is not None,
            SUMMARY: self.disk is not None and self.erase_ok.get_active(),
        }.get(self.step, True)
        self.next.set_sensitive(ready and not self.busy.get_spinning())

    def say(self, text: str) -> None:
        self.error.set_text(text)
        self.error.set_visible(bool(text))

    def call(self, work: Callable[[], Any], done: Callable[[Any], None]) -> None:
        """Runs work away from the screen, then done (or shows the error)."""
        self.busy.start()
        self.next.set_sensitive(False)

        def finish(result: Any, problem: str | None) -> bool:
            self.busy.stop()
            if problem is None:
                done(result)
            else:
                self.say(problem)
            self.update_next()
            return False

        def run() -> None:
            try:
                result, problem = work(), None
            except Exception as exc:  # noqa: BLE001 - shown to the person
                result, problem = None, str(exc) or _("Something went wrong.")
            GLib.idle_add(finish, result, problem)

        threading.Thread(target=run, daemon=True).start()

    # --- Language -------------------------------------------------------------

    def language_page(self) -> Gtk.Widget:
        """The installer's languages, each in itself: choosing one shows the
        installer in it at once."""
        box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=16)
        box.append(label(_("Choose your language"), "title"))
        box.append(label(_("For the installer. The box's language is chosen at first setup."), "lead"))
        choices = Gtk.ListBox(selection_mode=Gtk.SelectionMode.SINGLE)
        choices.add_css_class("disks")
        for code, name in LANGUAGES:
            row = Gtk.ListBoxRow()
            row.code = code
            line = Gtk.Box(spacing=16)
            line.add_css_class("disk")
            line.append(label(name, "disk-name", wrap=False))
            row.set_child(line)
            choices.append(row)
            if code == i18n.language:
                choices.select_row(row)
        choices.connect("row-activated", lambda _l, row: self.get_application().speak(row.code))
        box.append(choices)
        return box

    # --- Welcome --------------------------------------------------------------

    def welcome_page(self) -> Gtk.Widget:
        box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=16)
        hero = Gtk.DrawingArea(content_height=220, hexpand=True)
        hero.set_draw_func(self.draw_hero)
        hero.add_css_class("hero")
        box.append(hero)
        box.append(label(_("Install ZOOMRIYA OS"), "title"))
        box.append(label(_("Turns this computer into a ZOOMRIYA Box in about five minutes. The drive "
                           "you choose next is erased."), "lead"))
        self.checks = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=10)
        self.checks.add_css_class("checks")
        box.append(self.checks)
        return box

    def requirements(self) -> list[tuple[str, bool]]:
        fake = bool(os.environ.get("ZOOMRIYA_INSTALLER_FAKE"))
        ram = memory()
        return [
            (_("Started in UEFI mode"), fake or Path("/sys/firmware/efi").is_dir()),
            (_("Memory: {have} ({need} needed)").format(have=memory_text(ram), need=memory_text(MIN_MEMORY)),
             fake or ram >= MIN_MEMORY * 0.95),
            (_("A drive of {size} or more").format(size=size_text(MIN_BYTES)), any(d.usable for d in self.disks)),
        ]

    def check_internet(self) -> None:
        """Looks for the internet, again every few seconds until it's there (a
        cable plugged in later). It never stops the install."""
        def run() -> None:
            found = self.backend.online()
            GLib.idle_add(lambda: got(found) and False)

        def got(found: bool) -> None:
            self.online = found
            self.show_checks()
            if not found:
                GLib.timeout_add_seconds(5, lambda: self.check_internet() and False)

        threading.Thread(target=run, daemon=True).start()

    def show_checks(self) -> None:
        while child := self.checks.get_first_child():
            self.checks.remove(child)

        def add(mark_text: str, kind: str, text: str, wrap: bool = False) -> None:
            row = Gtk.Box(spacing=12)
            mark = label(mark_text, kind, wrap=False, shorten=False)
            # Centred in its circle.
            mark.set_xalign(0.5)
            mark.set_valign(Gtk.Align.CENTER)
            row.append(mark)
            row.append(label(text, "check-text", wrap=wrap))
            self.checks.append(row)

        for text, ok in self.requirements():
            add("✓" if ok else "!", "check-ok" if ok else "check-bad", text)
        # The internet: good to have, not needed.
        if self.online is None:
            add("…", "check-wait", _("Checking the internet…"))
        elif self.online:
            add("✓", "check-ok", _("Internet: connected"))
        else:
            add("!", "check-warn", _("No internet. Installing anyway; updates arrive once the box is "
                                     "online. Plug in a network cable to connect."), wrap=True)

    def draw_hero(self, _area: Any, cr: Any, w: int, h: int) -> None:
        """The box on a soft blue field, its light on."""
        rounded(cr, 0, 0, w, h, 18)
        cr.clip()
        cr.set_source_rgb(*rgb("#e3eefb"))
        cr.paint()
        for x, y, r, alpha in [(0.12, 0.2, 90, 0.35), (0.86, 0.85, 120, 0.3), (0.7, 0.05, 60, 0.25)]:
            cr.arc(w * x, h * y, r, 0, 2 * math.pi)
            cr.set_source_rgba(*rgb("#b9d4f3"), alpha)
            cr.fill()
        bw, bh = 220, 120
        bx, by = (w - bw) / 2, (h - bh) / 2 + 4
        # Its shadow, then the box.
        for spread in range(14, 0, -2):
            rounded(cr, bx - spread / 2, by + 10 - spread / 2, bw + spread, bh + spread, 22)
            cr.set_source_rgba(0.06, 0.2, 0.4, 0.025)
            cr.fill()
        rounded(cr, bx, by, bw, bh, 20)
        cr.set_source_rgb(1, 1, 1)
        cr.fill()
        rounded(cr, bx + 18, by + 18, bw - 36, 10, 5)
        cr.set_source_rgb(*rgb("#eef2f6"))
        cr.fill()
        for i in range(5):
            rounded(cr, bx + 24 + i * 16, by + bh - 34, 8, 14, 3)
            cr.set_source_rgb(*rgb("#dfe5ec"))
            cr.fill()
        cr.arc(bx + bw - 34, by + bh - 27, 12, 0, 2 * math.pi)
        cr.set_source_rgba(*rgb("#1765c1"), 0.18)
        cr.fill()
        cr.arc(bx + bw - 34, by + bh - 27, 6, 0, 2 * math.pi)
        cr.set_source_rgb(*rgb("#1765c1"))
        cr.fill()

    # --- Drive ----------------------------------------------------------------

    def drive_page(self) -> Gtk.Widget:
        box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=16)
        head = Gtk.Box(spacing=12)
        head.append(label(_("Where should ZOOMRIYA OS go?"), "title"))
        head.append(Gtk.Box(hexpand=True))
        again = Gtk.Button(label=_("Look again"), valign=Gtk.Align.CENTER)
        again.connect("clicked", lambda _b: self.look_for_disks())
        head.append(again)
        box.append(head)
        box.append(label(_("The chosen drive is erased; others aren't touched. Plugged one in? Press "
                           "Look again."),
                         "lead"))
        self.disk_list = Gtk.ListBox(selection_mode=Gtk.SelectionMode.SINGLE)
        self.disk_list.add_css_class("disks")
        self.disk_list.connect("row-selected", self.disk_selected)
        box.append(self.disk_list)

        self.bars = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=18)
        self.bars.add_css_class("bars")
        self.bar_now = Bar(_("Now"))
        self.bar_after = Bar(_("After installing"))
        self.bars.append(self.bar_now)
        self.bars.append(self.bar_after)
        box.append(self.bars)
        return box

    def look_for_disks(self) -> None:
        def got(disks: list[Disk]) -> None:
            chosen = self.disk.path if self.disk else None
            self.disks = disks
            self.disk = None
            self.show_disks(chosen)
            self.show_checks()

        self.call(self.backend.list_disks, got)

    def show_disks(self, chosen: str | None) -> None:
        while child := self.disk_list.get_first_child():
            self.disk_list.remove(child)
        pick = None
        for disk in self.disks:
            row = Gtk.ListBoxRow()
            row.disk = disk
            line = Gtk.Box(spacing=16)
            line.add_css_class("disk")
            kind = {"USB": "usb", "SD card": "sd-card"}.get(disk.bus, "disk")
            # Blue, or grey on a drive that can't be chosen.
            image = line_icon(kind, 32, "#1765c1" if disk.usable else "#56636f")
            image.add_css_class("disk-icon")
            line.append(image)
            words = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=2, hexpand=True)
            words.append(label(disk.title, "disk-name", wrap=False))
            words.append(label(f"{disk.kind} · {disk.path}", "disk-detail", wrap=False))
            if disk.reason:
                words.append(label(disk.reason, "disk-reason"))
            line.append(words)
            line.append(label(size_text(disk.size), "disk-size", wrap=False))
            row.set_child(line)
            row.set_sensitive(disk.usable)
            row.set_selectable(disk.usable)
            self.disk_list.append(row)
            if disk.usable and (pick is None or disk.path == chosen):
                pick = row
        self.bars.set_visible(pick is not None)
        if pick is not None:
            self.disk_list.select_row(pick)

    def disk_selected(self, _list: Gtk.ListBox, row: Gtk.ListBoxRow | None) -> None:
        self.disk = getattr(row, "disk", None)
        if self.disk is not None:
            self.bar_now.show(bar_segments(self.disk))
            self.bar_after.show(after_segments(self.disk))
        self.update_next()

    # --- Summary --------------------------------------------------------------

    def summary_page(self) -> Gtk.Widget:
        box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=16)
        box.append(label(_("Ready to install"), "title"))
        box.append(label(_("Check the drive. Nothing has changed yet."), "lead"))
        self.facts = Gtk.Grid(column_spacing=28, row_spacing=10)
        self.facts.add_css_class("facts")
        box.append(self.facts)
        self.summary_bar = Bar(_("The drive after installing"))
        box.append(self.summary_bar)
        warning = Gtk.Box(spacing=14)
        warning.add_css_class("warning")
        sign = line_icon("alert", 24, "#7a4510")
        warning.append(sign)
        self.erase_ok = Gtk.CheckButton()
        self.erase_ok.connect("toggled", lambda _b: self.update_next())
        warning.append(self.erase_ok)
        box.append(warning)
        return box

    def show_summary(self) -> None:
        disk = self.disk
        if disk is None:
            return
        while child := self.facts.get_first_child():
            self.facts.remove(child)
        erased = ", ".join(part.what for part in disk.parts) or _("Nothing: the drive is empty")
        facts = [(_("Drive"), f"{disk.title} ({disk.kind}, {disk.path})"),
                 (_("Size"), size_text(disk.size)),
                 (_("Erased"), erased),
                 (_("Installed"), "ZOOMRIYA OS 13")]
        for row, (name, value) in enumerate(facts):
            self.facts.attach(label(name, "fact-name", wrap=False), 0, row, 1, 1)
            self.facts.attach(label(value, "fact"), 1, row, 1, 1)
        self.summary_bar.show(after_segments(disk))
        self.erase_ok.set_active(False)
        self.erase_ok.set_label(_("Erase everything on {drive} ({size})").format(
            drive=disk.title, size=size_text(disk.size)))

    # --- Install --------------------------------------------------------------

    def install_page(self) -> Gtk.Widget:
        box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=18)
        box.append(label(_("Installing ZOOMRIYA OS"), "title"))
        self.slides = Gtk.Stack(transition_type=Gtk.StackTransitionType.SLIDE_LEFT,
                                transition_duration=600)
        for i, (icon, title, text, colours) in enumerate(SLIDES):
            self.slides.add_named(self.slide_card(icon, _(title), _(text), colours), str(i))
        box.append(self.slides)
        self.slide_dots = Gtk.Box(spacing=8, halign=Gtk.Align.CENTER)
        for _slide in SLIDES:
            self.slide_dots.append(label("", "slide-dot", wrap=False))
        box.append(self.slide_dots)

        self.stage = label("", "stage", wrap=False)
        box.append(self.stage)
        self.progress = Gtk.ProgressBar()
        box.append(self.progress)
        box.append(label(_("Keep the computer on and the USB stick in until done."), "hint"))
        return box

    def slide_card(self, icon: str, title: str, text: str, colours: tuple[str, str]) -> Gtk.Widget:
        overlay = Gtk.Overlay()
        art = Gtk.DrawingArea(content_height=230, hexpand=True)

        def draw(_a: Any, cr: Any, w: int, h: int) -> None:
            rounded(cr, 0, 0, w, h, 18)
            cr.clip()
            fill = cairo.LinearGradient(0, 0, w, h)
            fill.add_color_stop_rgb(0, *rgb(colours[0]))
            fill.add_color_stop_rgb(1, *rgb(colours[1]))
            cr.set_source(fill)
            cr.paint()
            for x, y, r in [(0.92, 0.1, 140), (0.78, 1.0, 90), (0.05, 0.95, 60)]:
                cr.arc(w * x, h * y, r, 0, 2 * math.pi)
                cr.set_source_rgba(1, 1, 1, 0.08)
                cr.fill()

        art.set_draw_func(draw)
        overlay.set_child(art)
        words = Gtk.Box(spacing=28, valign=Gtk.Align.CENTER)
        words.add_css_class("slide")
        image = line_icon(icon, 72, "#ffffff")
        image.add_css_class("slide-icon")
        words.append(image)
        text_box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=8, valign=Gtk.Align.CENTER)
        text_box.append(label(title, "slide-title"))
        body = label(text, "slide-text")
        body.set_max_width_chars(48)
        text_box.append(body)
        words.append(text_box)
        overlay.add_overlay(words)
        return overlay

    def next_slide(self) -> bool:
        if not self.installing:
            return False
        self.show_slide((self.slide + 1) % len(SLIDES))
        return True

    def show_slide(self, index: int) -> None:
        self.slide = index
        self.slides.set_visible_child_name(str(index))
        for i, dot in enumerate(_children(self.slide_dots)):
            if i == index:
                dot.add_css_class("current")
            else:
                dot.remove_css_class("current")

    def start_install(self) -> None:
        if self.installing or self.disk is None:
            return
        self.installing = True
        self.failed = ""
        self.show_slide(0)
        self.progress.set_fraction(0)
        GLib.timeout_add_seconds(SLIDE_SECONDS, self.next_slide)
        disk = self.disk.path

        def progress(stage: int, fraction: float) -> None:
            GLib.idle_add(self.show_progress, stage, fraction)

        def run() -> None:
            try:
                self.backend.install(disk, progress)
                problem = ""
            except InstallError as exc:
                problem = str(exc)
            except Exception as exc:  # noqa: BLE001 - shown to the person
                problem = _("The install stopped: {problem}").format(problem=exc)
            GLib.idle_add(self.finished, problem)

        threading.Thread(target=run, daemon=True).start()

    def show_progress(self, stage: int, fraction: float) -> bool:
        done = overall(stage, fraction)
        self.progress.set_fraction(done)
        self.stage.set_text(f"{_(STAGES[stage][0])}…  {done:.0%}")
        return False

    def finished(self, problem: str) -> bool:
        self.installing = False
        self.failed = problem
        self.show_done()
        self.go(DONE)
        return False

    # --- Done -----------------------------------------------------------------

    def done_page(self) -> Gtk.Widget:
        box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=16)
        self.done_art = Gtk.DrawingArea(content_width=96, content_height=96, halign=Gtk.Align.START)
        self.done_art.set_draw_func(self.draw_done)
        box.append(self.done_art)
        self.done_title = label("", "title")
        box.append(self.done_title)
        self.done_lead = label("", "lead")
        box.append(self.done_lead)
        self.done_steps = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=10)
        self.done_steps.add_css_class("facts")
        box.append(self.done_steps)
        self.failed = ""
        return box

    def show_done(self) -> None:
        while child := self.done_steps.get_first_child():
            self.done_steps.remove(child)
        if self.failed:
            self.done_title.set_text(_("The install didn't finish"))
            self.done_lead.set_text(self.failed + " " + _("The drive may not start now; installing "
                                                          "again fixes it."))
            steps = [_("Check the drive's connection, or try another drive."),
                     _("Press Try again."),
                     _("If it fails again, support notes are in {file}.").format(file=LOG)]
        else:
            self.done_title.set_text(_("ZOOMRIYA OS is installed"))
            self.done_lead.set_text(_("Your ZOOMRIYA Box is ready for its first start."))
            steps = [_("Take out the USB stick."),
                     _("Press Restart now to set it up, or Power off to leave it for its owner."),
                     _("On its next start, the box opens its first setup on this screen.")]
        for i, text in enumerate(steps):
            row = Gtk.Box(spacing=12)
            number = label(str(i + 1), "done-number", wrap=False, shorten=False)
            number.set_valign(Gtk.Align.START)
            number.set_xalign(0.5)
            row.append(number)
            row.append(label(text, "fact"))
            self.done_steps.append(row)
        self.done_art.queue_draw()

    def draw_done(self, _area: Any, cr: Any, w: int, h: int) -> None:
        colour = "#c0392b" if self.failed else "#1f9d55"
        cr.arc(w / 2, h / 2, w / 2, 0, 2 * math.pi)
        cr.set_source_rgba(*rgb(colour), 0.15)
        cr.fill()
        cr.arc(w / 2, h / 2, w / 2 - 12, 0, 2 * math.pi)
        cr.set_source_rgb(*rgb(colour))
        cr.fill()
        cr.set_source_rgb(1, 1, 1)
        cr.set_line_width(6)
        cr.set_line_cap(1)  # round
        cr.set_line_join(1)
        if self.failed:
            cr.move_to(w / 2, h * 0.32)
            cr.line_to(w / 2, h * 0.56)
            cr.stroke()
            cr.arc(w / 2, h * 0.68, 3.5, 0, 2 * math.pi)
            cr.fill()
        else:
            cr.move_to(w * 0.33, h * 0.52)
            cr.line_to(w * 0.45, h * 0.64)
            cr.line_to(w * 0.68, h * 0.38)
            cr.stroke()


def _children(box: Gtk.Widget) -> list[Gtk.Widget]:
    children, child = [], box.get_first_child()
    while child is not None:
        children.append(child)
        child = child.get_next_sibling()
    return children


class InstallerApp(Gtk.Application):
    def __init__(self, backend: Any) -> None:
        super().__init__(application_id="com.zoomriya.Installer",
                         flags=Gio.ApplicationFlags.NON_UNIQUE)
        self.backend = backend

    def speak(self, code: str) -> None:
        """The installer in another language: drawn again, still on Language."""
        if code == i18n.language:
            return
        i18n.use(code)
        old = self.get_active_window()
        self.show_window()
        if old is not None:
            old.destroy()

    def do_activate(self) -> None:
        css = Gtk.CssProvider()
        css.load_from_path(str(Path(__file__).with_name("style.css")))
        Gtk.StyleContext.add_provider_for_display(
            Gdk.Display.get_default(), css, Gtk.STYLE_PROVIDER_PRIORITY_APPLICATION
        )
        self.show_window()

    def show_window(self) -> None:
        window = InstallerWindow(self, self.backend)
        if os.environ.get("ZOOMRIYA_INSTALLER_WINDOWED"):
            window.set_default_size(1280, 800)
        else:
            window.fullscreen()
        window.present()
