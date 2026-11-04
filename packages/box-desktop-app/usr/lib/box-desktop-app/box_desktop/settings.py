"""Control Panel's look: its settings in rounded groups of rows, each row a
title with a line of explanation under it and its control on the right, as a
phone's or a laptop's settings show them (style.css's .group and .rows)."""

from __future__ import annotations

from collections.abc import Callable
from typing import Any

import gi

gi.require_version("Gtk", "4.0")
from gi.repository import Gtk  # noqa: E402

from .icons import glyph  # noqa: E402


def text(words: str, *classes: str, wrap: bool = False, **props: Any) -> Gtk.Label:
    props.setdefault("xalign", 0)
    widget = Gtk.Label(label=words, wrap=wrap, **props)
    for name in classes:
        widget.add_css_class(name)
    return widget


def page() -> Gtk.Box:
    """A section's page: its groups one under the other."""
    box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=22)
    box.add_css_class("settings")
    return box


def group(title: str = "", note: str = "", aside: Gtk.Widget | None = None) -> tuple[Gtk.Box, Gtk.ListBox]:
    """A titled group and its rows (the ListBox to append rows to)."""
    box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=8)
    box.add_css_class("group")
    if title or aside:
        head = Gtk.Box(spacing=12)
        head.append(text(title, "group-title", hexpand=True))
        if aside:
            head.append(aside)
        box.append(head)
    rows = Gtk.ListBox(selection_mode=Gtk.SelectionMode.NONE)
    rows.add_css_class("rows")
    box.append(rows)
    if note:
        box.append(text(note, "group-note", wrap=True))
    return box, rows


def row(title: str, subtitle: str = "", *ends: Gtk.Widget, icon: str = "") -> Gtk.ListBoxRow:
    """A row: an icon maybe, the title and its explanation, then the controls."""
    line = Gtk.Box(spacing=14)
    if icon:
        line.append(glyph(icon, 20))
    words = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=2, hexpand=True,
                    valign=Gtk.Align.CENTER)
    words.append(text(title, "row-title", wrap=True))
    if subtitle:
        words.append(text(subtitle, "row-sub", wrap=True))
    line.append(words)
    for end in ends:
        end.set_valign(Gtk.Align.CENTER)
        line.append(end)
    item = Gtk.ListBoxRow(activatable=False, selectable=False)
    item.set_child(line)
    return item


def wide(*children: Gtk.Widget) -> Gtk.ListBoxRow:
    """A row whose whole width is its content (a slider, a level, a form)."""
    box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=10)
    for child in children:
        box.append(child)
    item = Gtk.ListBoxRow(activatable=False, selectable=False)
    item.set_child(box)
    return item


def value(words: str) -> Gtk.Label:
    """A row's value, on the right, quieter than its title."""
    return text(words, "row-value", wrap=True, xalign=1, max_width_chars=36)


def segmented(choices: list[tuple[str, str]], chosen: str,
              changed: Callable[[str], None]) -> Gtk.Box:
    """Two or three buttons joined into one, one of them pressed (Left | Right)."""
    box = Gtk.Box()
    box.add_css_class("segmented")
    first: Gtk.ToggleButton | None = None
    for key, name in choices:
        button = Gtk.ToggleButton(label=name, active=key == chosen)
        if first is None:
            first = button
        else:
            button.set_group(first)
        button.connect("toggled", lambda b, k=key: b.get_active() and changed(k))
        box.append(button)
    return box


def slider(low: str, high: str, start: float, changed: Callable[[float], None],
           lowest: float = 0, highest: float = 1) -> Gtk.Box:
    """A slider with its two ends named under it (Slow … Fast)."""
    box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=2)
    box.set_size_request(260, -1)
    scale = Gtk.Scale.new_with_range(Gtk.Orientation.HORIZONTAL, lowest, highest,
                                     (highest - lowest) / 100)
    scale.set_value(start)
    scale.set_draw_value(False)
    scale.connect("value-changed", lambda s: changed(s.get_value()))
    box.append(scale)
    ends = Gtk.Box()
    ends.append(text(low, "row-sub", hexpand=True))
    ends.append(text(high, "row-sub"))
    box.append(ends)
    return box


def tile(icon: str, title: str, words: str, state: str) -> Gtk.Box:
    """A summary at the top of a page: an icon in a circle, ok or not."""
    box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=6, hexpand=True, homogeneous=False)
    box.add_css_class("tile-card")
    ring = Gtk.Box(halign=Gtk.Align.CENTER)
    ring.add_css_class("ring")
    ring.add_css_class(state)
    ring.set_size_request(56, 56)
    ring.append(glyph(icon, 26, state if state in ("ok", "warn") else "muted"))
    ring.get_first_child().set_hexpand(True)
    box.append(ring)
    heading = text(title, "row-title", wrap=True, justify=Gtk.Justification.CENTER)
    heading.set_xalign(0.5)
    box.append(heading)
    sub = text(words, "row-sub", wrap=True, justify=Gtk.Justification.CENTER)
    sub.set_xalign(0.5)
    box.append(sub)
    return box
