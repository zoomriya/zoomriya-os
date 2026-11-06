"""Box Desktop on the box's own screen (box-desktop.md): the admins' desktop.

Shown once the box is set up (design §5), and laid out as the mockup: the
wallpaper, a frosted top bar (the main menu, the open
windows, notifications, search, the widgets and the admin), the tools'
shortcuts down the left, the widgets down the right, and windows that move,
overlap, minimise and maximise. An admin signs in with the box's ZOOMRIYA ID;
everything the desktop shows or changes is asked of the agent, in that
admin's name. Remote use of the box is Family Hub's and the apps'.
"""

from __future__ import annotations

import os
import threading
from collections.abc import Callable
from datetime import datetime
from pathlib import Path
from typing import Any

import gi

gi.require_version("Gtk", "4.0")
gi.require_version("Gdk", "4.0")
from gi.repository import Gdk, Gio, GLib, Gtk  # noqa: E402

from zoomriya_setup.agent import AgentError  # noqa: E402
from zoomriya_setup.brand import MARK, WORDMARK, logo  # noqa: E402

from .control import ControlPanel  # noqa: E402
from . import i18n  # noqa: E402
from .i18n import N_, _  # noqa: E402
from .icons import glyph, set_glyph, set_theme, tile, tile_css  # noqa: E402
from .journal import note_sign_in  # noqa: E402
from .prefs import DEFAULT_THEME, DEFAULTS, Prefs  # noqa: E402
from zoomriya_setup.screen import set_keyboard, set_left_handed  # noqa: E402
from .signin import Member, SignInError  # noqa: E402
from .tools import MoreTools, Tools, size, when  # noqa: E402

HERE = Path(__file__).parent
TOP_BAR = 52
#: The room kept between an opened window and the top bar, and the screen's
#: bottom edge (a maximised window fills it all).
GAP = 24
#: The launcher along the bottom, and the room above the screen's edge it
#: keeps clear of windows.
DOCK_ROOM = 92
#: How long a window takes to grow in or shrink away (style.css's .window transition).
FADE_MS = 180

#: The tools, in the menu's order: name, and the window's first size (no taller
#: than fits between the top bar and the screen's bottom edge).
TOOLS = {
    "control": (N_("Control Panel"), 780, 900),
    "storage": (N_("Storage Manager"), 780, 560),
    "resources": (N_("Resource Monitor"), 720, 480),
    "logs": (N_("Log Center"), 820, 580),
}


def label(text: str, *classes: str, wrap: bool = False, **props: Any) -> Gtk.Label:
    widget = Gtk.Label(label=text, xalign=0, wrap=wrap, **props)
    if wrap and "max_width_chars" not in props:
        widget.set_max_width_chars(60)
    for name in classes:
        widget.add_css_class(name)
    return widget


def facts(rows: list[tuple[str, str]]) -> Gtk.Grid:
    grid = Gtk.Grid(column_spacing=28, row_spacing=10)
    grid.add_css_class("facts")
    for row, (name, value) in enumerate(rows):
        grid.attach(label(name, "fact-name"), 0, row, 1, 1)
        grid.attach(label(value, "fact", wrap=True), 1, row, 1, 1)
    return grid


def in_background(work: Callable[[], Any], done: Callable[[Any], None],
                  failed: Callable[[str], None] | None = None) -> None:
    """Runs work away from the screen, then done (or failed) on it."""

    def run() -> None:
        try:
            result, problem = work(), None
        except (AgentError, SignInError, OSError, PermissionError) as exc:
            result, problem = None, str(exc)

        def back() -> bool:
            if problem is None:
                done(result)
            elif failed is not None:
                failed(problem)
            return False

        GLib.idle_add(back)

    threading.Thread(target=run, daemon=True).start()


def duration(seconds: int) -> str:
    days, rest = divmod(int(seconds), 86400)
    hours, rest = divmod(rest, 3600)
    minutes = rest // 60
    parts = [_("1 day") if days == 1 else _("{n} days").format(n=days)] if days else []
    parts.append((_("1 hour") if hours == 1 else _("{n} hours").format(n=hours)) if hours or days
                 else _("{n} minutes").format(n=minutes))
    return ", ".join(parts)


def initials(name: str) -> str:
    words = [w for w in name.replace("_", " ").split() if w]
    letters = "".join(w[0] for w in words[:2]) if len(words) > 1 else name[:2]
    return letters.upper()


def alive(widget: Gtk.Widget) -> bool:
    """Still on screen (its window wasn't closed)."""
    return widget.get_root() is not None


def wallpaper() -> Gtk.Widget:
    picture = Gtk.Picture.new_for_filename(str(HERE / "assets" / "wallpaper.jpg"))
    picture.set_content_fit(Gtk.ContentFit.COVER)
    picture.set_can_shrink(True)
    picture.set_hexpand(True)
    picture.set_vexpand(True)
    return picture


class Layer(Gtk.Fixed):
    """Where the windows go: over the shortcuts and widgets, but only the
    windows themselves take the pointer, so the desktop under them still does."""

    def do_contains(self, x: float, y: float) -> bool:
        return False


class DeskWindow:
    """One tool's window on the desktop: where it is, and its parts."""

    def __init__(self, kind: str, frame: Gtk.Widget, tab: Gtk.Widget, pages: Gtk.Stack,
                 x: float, y: float, width: int, height: int) -> None:
        self.kind, self.frame, self.tab, self.pages = kind, frame, tab, pages
        self.x, self.y, self.width, self.height = x, y, width, height
        self.start = (x, y)
        self.maximised = False


class BoxDesktop(Tools, MoreTools, ControlPanel, Gtk.ApplicationWindow):
    def __init__(self, app: Gtk.Application, agent: Any, zid: Any, sound: Any,
                 with_sound: bool, display: Any) -> None:
        super().__init__(application=app, title="Box Desktop")
        self.agent, self.zid, self.sound, self.with_sound = agent, zid, sound, with_sound
        self.display = display
        self.member: Member | None = None
        self.box_name = ""
        self.windows: dict[str, DeskWindow] = {}
        self.active = ""
        self.stop_listening: Callable[[], None] | None = None
        self.cpu_history: list[float] = []
        self.prefs_store = Prefs()
        self.prefs: dict[str, Any] = dict(DEFAULTS)
        self.save_later: int | None = None
        self.restoring = False
        self.pill_target: tuple[str, str] = ("control", "restarts")
        self.notes: list[dict[str, Any]] = []
        self.check_poll: int | None = None

        self.pages = Gtk.Stack(transition_type=Gtk.StackTransitionType.CROSSFADE)
        self.pages.add_named(self.sign_in_page(), "sign-in")
        self.pages.add_named(self.desktop_page(), "desktop")
        self.set_child(self.pages)
        # The sign-in screen in the default theme, before anyone's own.
        self.apply_theme(DEFAULT_THEME, save=False)
        self.fitted_for = (0, 0)
        GLib.timeout_add_seconds(1, self.tick)
        self.tick()
        in_background(lambda: self.agent.ask("network"), self.got_name, lambda _p: None)

    def ask(self, op: str, **fields: Any) -> dict[str, Any]:
        """The agent, in the signed-in admin's name (its audit log)."""
        by = self.member.username if self.member else "box-desktop"
        return self.agent.ask(op, by=by, **fields)

    # --- signing in -------------------------------------------------------------

    def sign_in_page(self) -> Gtk.Widget:
        page = Gtk.Overlay()
        page.set_child(wallpaper())
        card = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=14,
                       halign=Gtk.Align.CENTER, valign=Gtk.Align.CENTER)
        card.add_css_class("card")
        card.set_size_request(400, -1)
        card.append(logo(WORDMARK, 28))
        self.signin_name = label(_("Sign in with your ZOOMRIYA ID"), "lead")
        card.append(self.signin_name)
        self.username = Gtk.Entry(placeholder_text=_("Username"))
        self.password = Gtk.PasswordEntry(show_peek_icon=True, placeholder_text=_("Password"))
        self.username.connect("activate", lambda _e: self.password.grab_focus())
        self.password.connect("activate", lambda _e: self.sign_in())
        card.append(self.username)
        card.append(self.password)
        self.signin_error = label("", "error", wrap=True)
        self.signin_error.set_visible(False)
        card.append(self.signin_error)
        row = Gtk.Box(spacing=10, halign=Gtk.Align.END)
        self.signin_busy = Gtk.Spinner()
        self.signin_button = Gtk.Button(label=_("Sign in"))
        self.signin_button.add_css_class("primary")
        self.signin_button.connect("clicked", lambda _b: self.sign_in())
        row.append(self.signin_busy)
        row.append(self.signin_button)
        card.append(row)
        page.add_overlay(card)
        return page

    def sign_in(self) -> None:
        username, password = self.username.get_text(), self.password.get_text()
        if not username.strip() or not password:
            return
        self.signin_busy.start()
        self.signin_button.set_sensitive(False)

        def settle() -> None:
            self.signin_busy.stop()
            self.signin_button.set_sensitive(True)

        def signed_in(member: Member) -> None:
            note_sign_in(member.username, True)
            self.enter(member)
            settle()

        def refused(problem: str) -> None:
            note_sign_in(username, False, problem)
            self.signin_error.set_text(problem)
            self.signin_error.set_visible(True)
            self.password.set_text("")
            self.password.grab_focus()
            settle()

        in_background(lambda: self.zid.sign_in(username, password), signed_in, refused)

    def enter(self, member: Member) -> None:
        """This admin's desktop: theirs as they left it (also when the desktop is
        drawn again in another language)."""
        self.member = member
        self.password.set_text("")
        self.signin_error.set_visible(False)
        self.avatar_letters.set_text(initials(member.name))
        self.avatar.set_tooltip_text(member.name)
        self.avatar.set_menu_model(self.account_menu(member.name))
        self.prefs = self.prefs_store.load(member.id)
        self.apply_theme(self.prefs["theme"], save=False)
        # This admin's keyboard and mouse on the box's screen.
        if self.prefs.get("keyboard"):
            set_keyboard(self.prefs["keyboard"])
        set_left_handed(self.prefs.get("handed") == "left")
        mouse = self.prefs.get("mouse") or {}
        self.display.pointer(mouse.get("speed", 0.0), mouse.get("accel", True),
                             mouse.get("natural", False))
        self.pages.set_visible_child_name("desktop")
        self.restore_layout(self.prefs.get("windows") or [])
        self.refresh_widgets()
        self.refresh_cpu()

    def sign_out(self) -> None:
        if self.member is not None:
            self.remember_layout()
            self.save_prefs_now()
        member, self.member = self.member, None
        self.restoring = True
        for kind in list(self.windows):
            self.close_window(kind)
        self.restoring = False
        self.apply_theme(DEFAULT_THEME, save=False)
        # The box's own keyboard and a right-handed mouse for the next one.
        set_left_handed(False)
        in_background(lambda: self.agent.ask("keyboard"),
                      lambda k: set_keyboard(k.get("layout") or "us"), lambda _p: None)
        if member is not None:
            threading.Thread(target=self.zid.sign_out, args=(member,), daemon=True).start()
        self.username.set_text("")
        self.pages.set_visible_child_name("sign-in")
        self.username.grab_focus()

    # --- each admin's own desktop --------------------------------------------------

    def apply_theme(self, theme: str, save: bool = True) -> None:
        (self.add_css_class if theme == "midnight" else self.remove_css_class)("midnight")
        set_theme(theme)
        set_glyph(self.theme_icon, "sun" if theme == "midnight" else "moon")
        self.theme_button.set_tooltip_text(_("Daylight") if theme == "midnight" else _("Midnight"))
        self.sparkline.queue_draw()
        if save and self.member is not None:
            self.prefs["theme"] = theme
            self.save_prefs()

    def save_prefs(self) -> None:
        """Soon, once: moving a window saves when it stops."""
        if self.save_later is not None:
            GLib.source_remove(self.save_later)
        self.save_later = GLib.timeout_add(800, self.save_prefs_now)

    def save_prefs_now(self) -> bool:
        self.save_later = None
        if self.member is not None:
            try:
                self.prefs_store.save(self.member.id, self.prefs)
            except OSError:
                pass  # the desktop works on without its memory
        return False

    def remember_layout(self) -> None:
        if self.restoring or self.member is None:
            return
        order = []
        child = self.layer.get_first_child()
        while child is not None:
            order.append(child)
            child = child.get_next_sibling()
        shown = []
        for frame in order:
            win = next((w for w in self.windows.values() if w.frame is frame), None)
            if win is not None:
                shown.append({"kind": win.kind, "x": round(win.x), "y": round(win.y),
                              "maximised": win.maximised, "minimised": not frame.get_visible(),
                              "section": win.pages.get_visible_child_name()})
        self.prefs["windows"] = shown
        self.save_prefs()

    def restore_layout(self, saved: list[dict[str, Any]]) -> None:
        self.restoring = True
        for entry in saved:
            kind = entry.get("kind")
            if kind not in TOOLS:
                continue
            self.open_tool(kind, entry.get("section"))
            win = self.windows[kind]
            win.x, win.y = float(entry.get("x", win.x)), float(entry.get("y", win.y))
            self.fit_window(win)
            if entry.get("maximised"):
                GLib.idle_add(lambda k=kind: self.maximise_window(k) and False)
            if entry.get("minimised"):
                self.minimise_window(kind)
        self.restoring = False

    def got_name(self, answer: dict[str, Any]) -> None:
        self.box_name = answer.get("name", "")
        self.signin_name.set_text(_("{box}: sign in with your ZOOMRIYA ID").format(box=self.box_name))
        self.menu_footer.set_text(f"{self.box_name} · ZOOMRIYA OS 13")

    # --- the desktop --------------------------------------------------------------

    def desktop_page(self) -> Gtk.Widget:
        # Sheets that ask for the password again go over everything.
        self.overlay = Gtk.Overlay()
        self.overlay.set_child(wallpaper())
        column = Gtk.Box(orientation=Gtk.Orientation.VERTICAL)
        column.append(self.top_bar())
        below = Gtk.Overlay(vexpand=True)
        # Shortcuts and widgets underneath; the windows' layer over them.
        under = Gtk.Box(hexpand=True, vexpand=True)
        spacer = Gtk.Box(hexpand=True)
        under.append(spacer)
        self.widget_column = self.widgets()
        under.append(self.widget_column)
        below.set_child(under)
        self.layer = Layer(hexpand=True, vexpand=True)
        below.add_overlay(self.layer)
        below.set_measure_overlay(self.layer, False)
        # The launcher over the windows, as the top bar is.
        launcher = self.dock()
        below.add_overlay(launcher)
        below.set_measure_overlay(launcher, False)
        column.append(below)
        self.overlay.add_overlay(column)
        return self.overlay

    def top_bar(self) -> Gtk.Widget:
        bar = Gtk.Box(spacing=6)
        bar.add_css_class("topbar")
        bar.set_size_request(-1, TOP_BAR)

        self.menu_button = Gtk.MenuButton(tooltip_text=_("Main menu"))
        mark = logo(MARK, 16)
        mark.set_halign(Gtk.Align.FILL)  # brand.py centres it in the button
        self.menu_button.set_child(mark)
        self.menu_button.add_css_class("bar-button")
        self.menu_button.add_css_class("menu-button")
        self.menu_button.set_popover(self.main_menu())
        bar.append(self.menu_button)
        divider = Gtk.Box(valign=Gtk.Align.CENTER)
        divider.add_css_class("divider")
        bar.append(divider)
        self.tabs = Gtk.Box(spacing=4)
        tabs = Gtk.ScrolledWindow(hexpand=True, hscrollbar_policy=Gtk.PolicyType.EXTERNAL,
                                  vscrollbar_policy=Gtk.PolicyType.NEVER)
        tabs.set_child(self.tabs)
        bar.append(tabs)

        self.pill = Gtk.Button(valign=Gtk.Align.CENTER)
        self.pill.add_css_class("pill")
        self.pill.set_visible(False)
        self.pill.connect("clicked", lambda _b: self.open_tool(*self.pill_target))
        bar.append(self.pill)

        self.bell = bell = Gtk.MenuButton(tooltip_text=_("Notifications"))
        bell.add_css_class("bar-button")
        bell_face = Gtk.Overlay()
        bell_face.set_child(glyph("bell", 22))
        self.badge = label("", "badge-count")
        self.badge.set_halign(Gtk.Align.END)
        self.badge.set_valign(Gtk.Align.START)
        self.badge.set_visible(False)
        bell_face.add_overlay(self.badge)
        bell.set_child(bell_face)
        self.bell_list = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=4)
        self.bell_list.add_css_class("popover-body")
        self.bell_list.set_size_request(340, -1)
        bell_pop = Gtk.Popover()
        bell_pop.set_child(self.bell_list)
        bell.set_popover(bell_pop)
        bar.append(bell)

        if self.with_sound:
            from .sound import OUTPUT

            volume = Gtk.MenuButton(tooltip_text=_("Sound"))
            volume.add_css_class("bar-button")
            volume.set_child(glyph("volume", 22))
            pop = Gtk.Popover()
            box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=8)
            box.add_css_class("popover-body")
            box.append(label(_("Speakers"), "field-label"))
            scale = Gtk.Scale.new_with_range(Gtk.Orientation.HORIZONTAL, 0, 100, 1)
            scale.set_size_request(220, -1)
            scale.set_draw_value(False)
            scale.connect("value-changed", lambda s: self.sound.set_volume(OUTPUT, s.get_value() / 100))
            box.append(scale)
            more = Gtk.Button(label=_("Sound settings"))
            more.connect("clicked", lambda _b: (pop.popdown(), self.open_tool("control", "sound")))
            box.append(more)
            pop.set_child(box)
            pop.connect("show", lambda _p: scale.set_value(self.sound.volume(OUTPUT)[0] * 100))
            volume.set_popover(pop)
            bar.append(volume)

        search = Gtk.Button(tooltip_text=_("Search tools"))
        search.add_css_class("bar-button")
        search.set_child(glyph("search", 22))
        search.connect("clicked", lambda _b: (self.menu_button.popup(), self.menu_search.grab_focus()))
        bar.append(search)

        self.theme_button = Gtk.Button(tooltip_text=_("Midnight"))
        self.theme_button.add_css_class("bar-button")
        self.theme_icon = glyph("moon", 21)
        self.theme_button.set_child(self.theme_icon)
        self.theme_button.connect("clicked", lambda _b: self.apply_theme(
            "daylight" if self.has_css_class("midnight") else "midnight"))
        bar.append(self.theme_button)

        self.widgets_toggle = Gtk.ToggleButton(active=True, tooltip_text=_("Widgets"))
        self.widgets_toggle.add_css_class("bar-button")
        self.widgets_toggle.set_child(glyph("widgets", 22))
        self.widgets_toggle.connect("toggled", lambda b: self.widget_column.set_visible(b.get_active()))
        bar.append(self.widgets_toggle)

        self.clock = label("", "clock")
        bar.append(self.clock)

        # The admin's initials only; their name is the tooltip and the menu's title.
        self.avatar = Gtk.MenuButton()
        self.avatar.add_css_class("bar-button")
        self.avatar_letters = label("", "avatar")
        self.avatar_letters.set_xalign(0.5)
        self.avatar_letters.set_valign(Gtk.Align.CENTER)
        self.avatar_letters.set_halign(Gtk.Align.CENTER)
        self.avatar.set_child(self.avatar_letters)
        avatar = self.avatar
        avatar.set_menu_model(self.account_menu(""))
        for name, act in (("sign-out", self.sign_out), ("restart", self.restart)):
            action = Gio.SimpleAction.new(name, None)
            action.connect("activate", lambda *_a, act=act: act())
            self.add_action(action)
        bar.append(avatar)
        # Every control sits in the middle of the bar, its own height.
        child = bar.get_first_child()
        while child is not None:
            child.set_valign(Gtk.Align.CENTER)
            child = child.get_next_sibling()
        return bar

    def account_menu(self, name: str) -> Gio.Menu:
        """The admin's menu, headed by their name."""
        items = Gio.Menu()
        items.append(_("Restart the box"), "win.restart")
        items.append(_("Sign out"), "win.sign-out")
        menu = Gio.Menu()
        menu.append_section(name or None, items)
        return menu

    def main_menu(self) -> Gtk.Popover:
        pop = Gtk.Popover(has_arrow=False)
        pop.set_position(Gtk.PositionType.BOTTOM)
        body = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=14)
        body.add_css_class("menu-body")
        self.menu_search = Gtk.SearchEntry(placeholder_text=_("Search tools"))
        body.append(self.menu_search)
        grid = Gtk.FlowBox(max_children_per_line=4, min_children_per_line=4,
                           selection_mode=Gtk.SelectionMode.NONE, homogeneous=True,
                           column_spacing=8, row_spacing=8)
        names: dict[Gtk.Widget, str] = {}
        for kind, (english, _w, _h) in TOOLS.items():
            name = _(english)
            button = Gtk.Button()
            button.add_css_class("menu-tool")
            inner = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=8)
            inner.append(tile(kind, 48))
            inner.append(Gtk.Label(label=name, justify=Gtk.Justification.CENTER))
            button.set_child(inner)
            names[button] = name
            button.connect("clicked", lambda _b, k=kind: (pop.popdown(), self.open_tool(k)))
            grid.append(button)
        grid.set_filter_func(lambda child: self.menu_search.get_text().lower()
                             in names[child.get_child()].lower())
        self.menu_search.connect("search-changed", lambda _e: grid.invalidate_filter())
        body.append(grid)
        self.menu_footer = label("ZOOMRIYA OS 13", "menu-footer")
        body.append(self.menu_footer)
        pop.set_child(body)
        pop.connect("closed", lambda _p: self.menu_search.set_text(""))
        return pop

    def dock(self) -> Gtk.Widget:
        """The launcher along the bottom: each tool's icon, its name when the
        pointer is on it, and a dot under each one open. A tool open and in
        front is minimised by it, as its tab does."""
        bar = Gtk.Box(spacing=4, halign=Gtk.Align.CENTER, valign=Gtk.Align.END, margin_bottom=14)
        bar.add_css_class("dock")
        self.dock_dots: dict[str, Gtk.Widget] = {}
        for kind, (english, _w, _h) in TOOLS.items():
            button = Gtk.Button(tooltip_text=_(english))
            button.add_css_class("dock-item")
            inner = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=4)
            inner.append(tile(kind, 48))
            dot = Gtk.Box(halign=Gtk.Align.CENTER)
            dot.add_css_class("dock-dot")
            inner.append(dot)
            self.dock_dots[kind] = dot
            button.set_child(inner)
            button.connect("clicked", lambda _b, k=kind: self.dock_clicked(k))
            bar.append(button)
        return bar

    def dock_clicked(self, kind: str) -> None:
        if kind in self.windows:
            self.tab_clicked(kind)
        else:
            self.open_tool(kind)

    def update_dock(self) -> None:
        for kind, dot in self.dock_dots.items():
            (dot.add_css_class if kind in self.windows else dot.remove_css_class)("open")

    def tick(self) -> bool:
        self.clock.set_text(datetime.now().strftime("%a %-d %b  %H:%M"))
        return True

    # --- widgets ------------------------------------------------------------------

    def widgets(self) -> Gtk.Widget:
        scroll = Gtk.ScrolledWindow(vscrollbar_policy=Gtk.PolicyType.AUTOMATIC,
                                    hscrollbar_policy=Gtk.PolicyType.NEVER,
                                    propagate_natural_height=True, valign=Gtk.Align.START)
        scroll.add_css_class("widget-column")
        # The mockup's 304 px, whatever its labels would like.
        scroll.set_hexpand(False)
        scroll.set_size_request(304, -1)
        column = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=12)
        column.set_size_request(304, -1)
        scroll.set_child(column)

        def card(title: str) -> tuple[Gtk.Box, Gtk.Label]:
            box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=8)
            box.add_css_class("widget")
            head = Gtk.Box()
            head.append(label(title.upper(), "widget-title", hexpand=True))
            aside = label("", "widget-aside")
            head.append(aside)
            box.append(head)
            column.append(box)
            return box, aside

        health, self.health_name = card(_("Box health"))
        row = Gtk.Box(spacing=12)
        self.health_mark = Gtk.Box(halign=Gtk.Align.CENTER, valign=Gtk.Align.CENTER)
        self.health_mark.add_css_class("mark")
        self.health_mark.set_size_request(40, 40)
        check = glyph("check", 22, "ok")
        check.set_hexpand(True)
        check.set_halign(Gtk.Align.CENTER)
        self.health_mark.append(check)
        row.append(self.health_mark)
        text = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, valign=Gtk.Align.CENTER)
        self.health_title = label(_("Checking the box…"), "widget-big")
        self.health_sub = label("", "widget-small")
        text.append(self.health_title)
        text.append(self.health_sub)
        row.append(text)
        health.append(row)
        self.health_rows = Gtk.Grid(column_spacing=10, row_spacing=4)
        health.append(self.health_rows)

        cpu, _rows = card(_("CPU and memory"))
        top = Gtk.Box(spacing=10)
        figures = Gtk.Box(orientation=Gtk.Orientation.VERTICAL)
        self.cpu_figure = label("–", "widget-figure")
        figures.append(self.cpu_figure)
        figures.append(label(_("CPU"), "widget-small"))
        top.append(figures)
        self.sparkline = Gtk.DrawingArea(hexpand=True)
        self.sparkline.set_size_request(-1, 44)
        self.sparkline.set_draw_func(self.draw_sparkline)
        top.append(self.sparkline)
        cpu.append(top)
        mem_row = Gtk.Box()
        mem_row.append(label(_("Memory"), "widget-small", hexpand=True))
        self.mem_figure = label("", "widget-small")
        mem_row.append(self.mem_figure)
        cpu.append(mem_row)
        self.mem_bar = Gtk.LevelBar(min_value=0, max_value=1)
        self.mem_bar.add_css_class("bar-memory")
        cpu.append(self.mem_bar)

        space, self.space_percent = card(_("Space"))
        self.space_figure = label("–", "widget-figure-line")
        space.append(self.space_figure)
        self.space_bar = Gtk.LevelBar(min_value=0, max_value=1)
        self.space_bar.add_css_class("bar-space")
        space.append(self.space_bar)
        self.space_note = label("", "widget-small", wrap=True)
        space.append(self.space_note)
        return scroll

    def draw_sparkline(self, _area: Gtk.DrawingArea, cr: Any, width: int, height: int) -> None:
        points = self.cpu_history[-30:]
        if len(points) < 2:
            return
        top = max(max(points), 10.0)
        step = width / (len(points) - 1)
        # The theme's accent.
        if self.has_css_class("midnight"):
            cr.set_source_rgb(0x6F / 255, 0x86 / 255, 0xE8 / 255)
        else:
            cr.set_source_rgb(0x17 / 255, 0x65 / 255, 0xC1 / 255)
        cr.set_line_width(1.6)
        for i, value in enumerate(points):
            x, y = i * step, height - 2 - (value / top) * (height - 6)
            (cr.move_to if i == 0 else cr.line_to)(x, y)
        cr.stroke()

    def refresh_widgets(self) -> None:
        if self.member is None:
            return

        def got(answers: tuple[dict[str, Any], ...]) -> None:
            status, storage, health, notes = answers
            drives = health.get("drives") or []
            sick = [d for d in drives if d.get("health") not in ("healthy", None)]
            needs = bool(sick) or bool(status.get("restart_needed"))
            self.health_title.set_text(_("Needs a look") if needs else _("Everything's fine"))
            (self.health_mark.add_css_class if needs else self.health_mark.remove_css_class)("warn")
            self.health_sub.set_text(_("ZOOMRIYA OS 13 · up {time}").format(
                time=duration(status.get("uptime_seconds", 0))))
            self.health_name.set_text(self.box_name.removesuffix(".local"))
            while child := self.health_rows.get_first_child():
                self.health_rows.remove(child)
            rows = [
                (_("Drives"), _("{n} need checking").format(n=len(sick)) if sick else _("All healthy"), bool(sick)),
                (_("Security fixes"), when(status.get("last_security_update", "")), False),
            ]
            if status.get("restart_needed"):
                rows.append((_("Restart"), _("Needed"), True))
            for i, (name, value, warn) in enumerate(rows):
                self.health_rows.attach(label(name, "widget-small", hexpand=True), 0, i, 1, 1)
                shown = label(value, "widget-value", "warn" if warn else "ok")
                shown.set_xalign(1)
                self.health_rows.attach(shown, 1, i, 1, 1)
            check = (storage.get("library") or {}).get("check") or {}
            self.pill.remove_css_class("busy")
            if check.get("running") and not check.get("paused"):
                self.pill.set_label(_("Checking the library · {percent}%").format(
                    percent=f"{check.get('percent', 0):.0f}"))
                self.pill.add_css_class("busy")
                self.pill_target = ("storage", "overview")
                # One follow-up at a time while the check runs, for its progress.
                if self.check_poll is None:
                    self.check_poll = GLib.timeout_add_seconds(10, self.poll_check)
            elif status.get("restart_needed"):
                self.pill.set_label(_("Restart needed"))
                self.pill_target = ("control", "restarts")
            self.pill.set_visible(bool(check.get("running") and not check.get("paused"))
                                  or bool(status.get("restart_needed")))

            library = storage.get("library") or {}
            disk = library if library.get("path") else (storage.get("system") or {})
            total, used = disk.get("size_bytes") or 0, disk.get("used_bytes") or 0
            self.space_figure.set_markup(_("<b>{used}</b> <small>of {total} used</small>").format(
                used=size(used), total=size(total)))
            self.space_percent.set_text(f"{used / total * 100:.0f}%" if total else "")
            self.space_bar.set_value(used / total if total else 0)
            self.space_note.set_text(_("The box's own disk") if not library.get("path")
                                     else _("Library drives mirrored") if library.get("mirror")
                                     else _("Library"))
            self.show_notifications(notes.get("notifications") or [])

        in_background(lambda: (self.ask("status"), self.ask("storage"), self.ask("disk-health"),
                               self.ask("notifications")), got)

    def poll_check(self) -> bool:
        self.check_poll = None
        self.refresh_widgets()
        return False

    def show_notifications(self, notes: list[dict[str, Any]] | None = None) -> None:
        """The bell's list: the kinds the admin hasn't turned off, newest first,
        the unread counted on its badge."""
        if notes is not None:
            self.notes = notes
        settings = self.prefs.get("notifications") or {}
        off, read_until = set(settings.get("off") or []), settings.get("read_until") or ""
        shown = sorted((n for n in self.notes if n.get("kind") not in off),
                       key=lambda n: n.get("time", ""), reverse=True)
        unread = [n for n in shown if n.get("time", "") > read_until]
        while child := self.bell_list.get_first_child():
            self.bell_list.remove(child)
        head = Gtk.Box()
        head.append(label(_("Notifications"), "section", hexpand=True))
        if unread:
            mark = Gtk.Button(label=_("Mark all read"))
            mark.add_css_class("flat-link")
            mark.connect("clicked", lambda _b: self.mark_read())
            head.append(mark)
        self.bell_list.append(head)
        if not shown:
            self.bell_list.append(label(_("Nothing new."), "lead"))
        for n in shown[:8]:
            row = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=2)
            row.add_css_class("notice-row")
            row.add_css_class("unread" if n in unread else "read")
            row.append(label(when(n.get("time", "")), "fact-name", n.get("level", "")))
            row.append(label(n.get("message", ""), "fact", wrap=True, max_width_chars=40))
            self.bell_list.append(row)
        everything = Gtk.Button(label=_("See all in Log Center"), halign=Gtk.Align.START)
        everything.add_css_class("flat-link")
        everything.connect("clicked", lambda _b: (self.bell.popdown(),
                                                  self.open_tool("logs", "notifications")))
        self.bell_list.append(everything)
        self.badge.set_text(str(len(unread)))
        self.badge.set_visible(bool(unread))

    def mark_read(self) -> None:
        newest = max((n.get("time", "") for n in self.notes), default="")
        self.prefs.setdefault("notifications", {})["read_until"] = newest
        self.save_prefs()
        self.show_notifications()

    def turn_notifications(self, kind: str, on: bool) -> None:
        settings = self.prefs.setdefault("notifications", {"off": [], "read_until": ""})
        off = set(settings.get("off") or [])
        (off.discard if on else off.add)(kind)
        settings["off"] = sorted(off)
        self.save_prefs()
        self.show_notifications()

    def refresh_cpu(self) -> bool:
        if self.member is None:
            return True

        def got(answer: dict[str, Any]) -> None:
            percent = answer.get("cpu_percent", 0)
            self.cpu_history.append(percent)
            del self.cpu_history[:-60]
            self.cpu_figure.set_text(f"{percent:.0f}%")
            total, used = answer.get("memory_total_bytes", 0), answer.get("memory_used_bytes", 0)
            self.mem_figure.set_text(_("{used} of {total}").format(used=size(used), total=size(total)))
            self.mem_bar.set_value(used / total if total else 0)
            self.sparkline.queue_draw()

        in_background(lambda: self.ask("resources"), got)
        return True

    # --- windows ------------------------------------------------------------------

    def sections(self, kind: str) -> list[tuple[str, str, Callable[[], Gtk.Widget]]]:
        if kind == "control":
            listed = [("network", _("Network"), self.cp_network),
                      ("display", _("Display"), self.cp_display)]
            if self.with_sound:
                listed.append(("sound", _("Sound"), self.cp_sound))
            return listed + [("keyboard", _("Keyboard & Mouse"), self.cp_keyboard),
                             ("language", _("Language & Region"), self.cp_language),
                             ("time", _("Date and time"), self.cp_time),
                             ("notifications", _("Notifications"), self.cp_notifications),
                             ("security", _("Security"), self.cp_security),
                             ("restarts", _("Restarts"), self.restarts),
                             ("restore", _("Restore & Reset"), self.restore_points),
                             ("about", _("System Info"), self.cp_about)]
        if kind == "storage":
            return [("overview", _("Overview"), self.storage_overview),
                    ("drives", _("Drives"), self.storage_drives),
                    ("space", _("Space"), self.storage_space)]
        if kind == "resources":
            return [("overview", _("Overview"), self.resources),
                    ("apps", _("By app"), self.resources_by_app),
                    ("temperature", _("Temperature"), self.temperatures)]
        return [("events", _("Box events"), lambda: self.logs("events")),
                ("notifications", _("Notifications"), self.log_notifications),
                ("audit", _("Admin actions"), lambda: self.logs("audit")),
                ("signins", _("Sign-ins"), lambda: self.logs("signins")),
                ("system", _("System log"), lambda: self.logs("system"))]

    def open_tool(self, kind: str, section: str | None = None) -> None:
        if kind not in self.windows:
            self.windows[kind] = self.make_window(kind)
            self.update_dock()
        win = self.windows[kind]
        if not win.frame.get_visible() or win.frame.has_css_class("opening"):
            self.fade_in(win.frame)
        win.frame.set_visible(True)
        # Reset is part of Restore & Reset now (a layout kept before may name it).
        section = "restore" if section == "reset" else section
        if section and win.pages.get_child_by_name(section):
            win.pages.set_visible_child_name(section)
        self.raise_window(kind)

    def make_window(self, kind: str) -> DeskWindow:
        english, width, height = TOOLS[kind]
        name = _(english)
        n = len(self.windows)
        x, y = 80 + 28 * n, GAP + 28 * n
        frame = Gtk.Box(orientation=Gtk.Orientation.VERTICAL)
        frame.add_css_class("window")
        # Faded in once shown (open_tool); a layout restored at sign-in isn't.
        if not self.restoring:
            frame.add_css_class("opening")
        frame.set_size_request(width, height)
        # Its pages' backgrounds stay inside its rounded corners.
        frame.set_overflow(Gtk.Overflow.HIDDEN)

        bar = Gtk.Box(spacing=10)
        bar.add_css_class("window-bar")
        bar.append(tile(kind, 22))
        bar.append(label(name, "window-title", hexpand=True))
        for icon, act in (("minimise", self.minimise_window), ("maximise", self.maximise_window),
                          ("close", self.close_window)):
            button = Gtk.Button(valign=Gtk.Align.CENTER)
            button.add_css_class("window-button")
            button.add_css_class(icon)
            button.set_child(glyph(icon, 12, "muted"))
            button.connect("clicked", lambda _b, a=act: a(kind))
            bar.append(button)
        frame.append(bar)

        body = Gtk.Box(vexpand=True)
        pages = Gtk.Stack(hexpand=True, vexpand=True, transition_type=Gtk.StackTransitionType.CROSSFADE)
        listed = self.sections(kind)
        side = Gtk.ListBox(selection_mode=Gtk.SelectionMode.SINGLE)
        side.add_css_class("window-sidebar")
        built: set[str] = set()
        holders: dict[str, Gtk.ScrolledWindow] = {}
        rows: dict[str, Gtk.ListBoxRow] = {}
        for section_id, title, _build in listed:
            holder = Gtk.ScrolledWindow(hscrollbar_policy=Gtk.PolicyType.NEVER)
            holders[section_id] = holder
            pages.add_named(holder, section_id)
            row = Gtk.ListBoxRow()
            # Room for the name in bold too (as when chosen), so the side is
            # as wide as its widest name and choosing one doesn't move the page.
            words = Gtk.Stack(hhomogeneous=True)
            words.add_child(label(title, "sidebar-item"))
            words.add_child(label(title, "sidebar-item", "chosen"))
            row.set_child(words)
            rows[section_id] = row
            side.append(row)

        def show(section_id: str) -> None:
            if section_id not in built:
                build = next(b for s, _t, b in listed if s == section_id)
                inner = build()
                inner.add_css_class("page-body")
                holders[section_id].set_child(inner)
                built.add(section_id)
            if side.get_selected_row() is not rows[section_id]:
                side.select_row(rows[section_id])
            self.listen_if_sound()
            if kind in self.windows:
                self.remember_layout()

        def picked(_side: Gtk.ListBox, row: Gtk.ListBoxRow | None) -> None:
            if row is not None:
                section_id = next(s for s, r in rows.items() if r is row)
                pages.set_visible_child_name(section_id)

        side.connect("row-selected", picked)
        pages.connect("notify::visible-child-name", lambda p, _x: show(p.get_visible_child_name()))
        if len(listed) > 1:
            body.append(side)
        body.append(pages)
        frame.append(body)
        show(listed[0][0])

        tab = Gtk.Button(valign=Gtk.Align.CENTER)
        tab.add_css_class("tab")
        face = Gtk.Box(spacing=8)
        face.append(tile(kind, 22))
        face.append(Gtk.Label(label=name))
        tab.set_child(face)
        tab.connect("clicked", lambda _b: self.tab_clicked(kind))
        self.tabs.append(tab)

        self.layer.put(frame, x, y)
        win = DeskWindow(kind, frame, tab, pages, x, y, width, height)
        self.fit_window(win)

        drag = Gtk.GestureDrag()
        drag.connect("drag-begin", lambda _g, _x, _y: self.begin_drag(kind))
        drag.connect("drag-update", lambda _g, dx, dy: self.drag_window(kind, dx, dy))
        drag.connect("drag-end", lambda *_a: self.remember_layout())
        bar.add_controller(drag)
        click = Gtk.GestureClick()
        click.set_propagation_phase(Gtk.PropagationPhase.CAPTURE)
        click.connect("pressed", lambda *_a: self.raise_window(kind))
        frame.add_controller(click)
        return win

    def begin_drag(self, kind: str) -> None:
        win = self.windows[kind]
        win.start = (win.x, win.y)
        self.raise_window(kind)

    def desk_size(self) -> tuple[int, int]:
        """The desktop below the top bar. From Box Desktop's own window, not the
        windows' layer, which isn't laid out yet when a sign-in restores them."""
        return self.get_width(), self.get_height() - TOP_BAR - DOCK_ROOM

    def fit_window(self, win: DeskWindow) -> None:
        """A maximised window fills the desktop. Others are no wider than the
        screen with GAP free at each side, no taller than the desktop with GAP
        free above and below, and moved up if they'd reach past the bottom."""
        width, height = self.desk_size()
        if win.maximised:
            win.frame.set_size_request(width, height)
            self.layer.move(win.frame, 0, 0)
            return
        if width > 400:
            win.width = min(TOOLS[win.kind][1], width - 2 * GAP)
        room = height - GAP
        if room > 300:
            win.height = min(TOOLS[win.kind][2], room - GAP)
            win.y = max(GAP, min(win.y, room - win.height))
        win.frame.set_size_request(win.width, win.height)
        self.layer.move(win.frame, win.x, win.y)

    def do_size_allocate(self, width: int, height: int, baseline: int) -> None:
        Gtk.ApplicationWindow.do_size_allocate(self, width, height, baseline)
        # A window that no longer fits the screen (another monitor, a new
        # resolution, or Box Desktop's own window resized away from the box) is
        # fitted again, once this layout is done; a maximised one fills it.
        if (width, height) != self.fitted_for:
            self.fitted_for = (width, height)
            GLib.idle_add(self.fit_windows)

    def fit_windows(self) -> bool:
        for win in self.windows.values():
            self.fit_window(win)
        return False

    def drag_window(self, kind: str, dx: float, dy: float) -> None:
        win = self.windows[kind]
        if win.maximised:
            return
        x0, y0 = win.start
        win.x = max(120 - win.width, min(x0 + dx, self.layer.get_width() - 120))
        win.y = max(0, min(y0 + dy, self.layer.get_height() - 46))
        self.layer.move(win.frame, win.x, win.y)

    def raise_window(self, kind: str) -> None:
        win = self.windows.get(kind)
        if win is None:
            return
        if self.layer.get_last_child() is not win.frame:
            win.frame.insert_before(self.layer, None)
        self.active = kind
        for other in self.windows.values():
            on = other.kind == kind and other.frame.get_visible()
            for widget in (other.frame, other.tab):
                (widget.add_css_class if on else widget.remove_css_class)("active")
        self.listen_if_sound()
        self.remember_layout()

    def tab_clicked(self, kind: str) -> None:
        win = self.windows[kind]
        if win.frame.get_visible() and self.active == kind:
            self.minimise_window(kind)
        else:
            if not win.frame.get_visible():
                self.fade_in(win.frame)
            win.frame.set_visible(True)
            self.raise_window(kind)

    def minimise_window(self, kind: str) -> None:
        win = self.windows[kind]
        win.tab.remove_css_class("active")
        self.active = ""
        self.listen_if_sound()
        self.fade_out(win.frame, lambda: win.frame.set_visible(False) or self.remember_layout())

    def fade_in(self, frame: Gtk.Widget) -> None:
        """From a little smaller to itself, once it's drawn (style.css)."""
        frame.remove_css_class("closing")
        frame.add_css_class("opening")
        GLib.timeout_add(30, lambda: frame.remove_css_class("opening") or False)

    def fade_out(self, frame: Gtk.Widget, then: Callable[[], Any]) -> None:
        """A little smaller, then done; at once while a layout is restored."""
        if self.restoring:
            then()
            return
        frame.add_css_class("closing")

        def done() -> bool:
            frame.remove_css_class("closing")
            then()
            return False

        GLib.timeout_add(FADE_MS, done)
    def maximise_window(self, kind: str) -> None:
        win = self.windows[kind]
        win.maximised = not win.maximised
        (win.frame.add_css_class if win.maximised else win.frame.remove_css_class)("maximised")
        self.fit_window(win)
        self.remember_layout()

    def close_window(self, kind: str) -> None:
        win = self.windows.pop(kind, None)
        if win is None:
            return
        self.tabs.remove(win.tab)
        if self.active == kind:
            self.active = ""
        self.update_dock()
        self.listen_if_sound()
        self.remember_layout()
        self.fade_out(win.frame, lambda: self.layer.remove(win.frame))

    def listen_if_sound(self) -> None:
        """The microphone's level only while the Sound section is in front."""
        win = self.windows.get("control")
        showing = (win is not None and self.active == "control" and win.frame.get_visible()
                   and win.pages.get_visible_child_name() == "sound")
        if showing and self.stop_listening is None and hasattr(self, "mic_level"):
            self.stop_listening = self.sound.listen(
                lambda v: GLib.idle_add(lambda: self.mic_level.set_value(v) and False))
        elif not showing and self.stop_listening is not None:
            self.stop_listening()
            self.stop_listening = None


class BoxDesktopApp(Gtk.Application):
    def __init__(self, agent: Any, zid: Any, sound: Any, with_sound: bool, display: Any) -> None:
        # Not unique: the box's screen has no session bus to register on.
        super().__init__(application_id="com.zoomriya.BoxDesktop",
                         flags=Gio.ApplicationFlags.NON_UNIQUE)
        self.parts = (agent, zid, sound, with_sound, display)

    def do_activate(self) -> None:
        css = Gtk.CssProvider()
        css.load_from_string((HERE / "style.css").read_text() + "\n" + tile_css())
        Gtk.StyleContext.add_provider_for_display(
            Gdk.Display.get_default(), css, Gtk.STYLE_PROVIDER_PRIORITY_APPLICATION
        )
        self.window: BoxDesktop | None = None
        self.timers: list[int] = []
        self.show_window()

    def show_window(self, member: Member | None = None) -> None:
        window = self.window = BoxDesktop(self, *self.parts)
        self.timers = [GLib.timeout_add_seconds(3, window.refresh_cpu),
                       GLib.timeout_add_seconds(60, lambda: window.refresh_widgets() or True)]
        if os.environ.get("BOX_DESKTOP_WINDOWED"):
            window.set_default_size(1440, 900)
        else:
            window.fullscreen()
        window.present()
        if member is None:
            window.username.grab_focus()
        else:
            window.enter(member)

    def speak(self, code: str) -> None:
        """The desktop in another language (a locale, such as fr_CA.UTF-8): drawn
        again, the admin still signed in, their windows where they were."""
        old = self.window
        i18n.use(code)
        if old is None:
            return
        member = old.member
        if member is not None:
            old.remember_layout()
            old.save_prefs_now()
        for timer in self.timers:
            GLib.source_remove(timer)
        if old.stop_listening is not None:
            old.stop_listening()
        self.show_window(member)
        old.destroy()
