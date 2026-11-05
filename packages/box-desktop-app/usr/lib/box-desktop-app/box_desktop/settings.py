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


def group(title: str = "", note: str = "", aside: Gtk.Widget | None = None,
          lead: str = "", icon: str = "") -> tuple[Gtk.Box, Gtk.ListBox]:
    """A titled group and its rows (the ListBox to append rows to): lead says what
    it is under the title, note adds to it under the rows, and icon goes before
    the title. The title's label is the box's title_label (Network renames its own)."""
    box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=8)
    box.add_css_class("group")
    if title or aside:
        head = Gtk.Box(spacing=12)
        named = text(title, "group-title", hexpand=True)
        box.title_label = named
        if icon:
            line = Gtk.Box(spacing=8, hexpand=True)
            line.append(glyph(icon, 20, "accent"))
            line.append(named)
            head.append(line)
        else:
            head.append(named)
        if lead:
            head.set_orientation(Gtk.Orientation.VERTICAL)
            head.set_spacing(2)
            head.append(text(lead, "group-note", wrap=True))
        if aside:
            head.append(aside)
        box.append(head)
    rows = Gtk.ListBox(selection_mode=Gtk.SelectionMode.NONE)
    rows.add_css_class("rows")
    box.append(rows)
    if note:
        box.append(text(note, "group-note", wrap=True))
    return box, rows


def row(title: str, subtitle: str = "", *ends: Gtk.Widget, icon: str = "", icon_role: str = "accent",
        icon_px: int = 20, tag: str = "") -> Gtk.ListBoxRow:
    """A row: an icon maybe (in the accent, like every row's), the title (and a tag
    beside it) and its explanation, then the controls."""
    line = Gtk.Box(spacing=14)
    if icon:
        line.append(glyph(icon, icon_px, icon_role))
    words = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=2, hexpand=True,
                    valign=Gtk.Align.CENTER)
    if tag:
        named = Gtk.Box(spacing=8)
        named.append(text(title, "row-title", wrap=True))
        chip = text(tag, "tag", valign=Gtk.Align.CENTER)
        named.append(chip)
        words.append(named)
    elif title:
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


def fixed_switch(on: bool) -> Gtk.Switch:
    """A switch that only shows a state: greyed out, since it isn't changed here."""
    switch = Gtk.Switch(active=on, sensitive=False, valign=Gtk.Align.CENTER,
                        tooltip_text="Always on" if on else "Off")
    switch.add_css_class("fixed")
    return switch


def segmented(choices: list[tuple[str, ...]], chosen: str,
              changed: Callable[[str], None]) -> Gtk.Box:
    """Two or three buttons joined into one, one of them pressed (Left | Right);
    each choice is (key, name) or (key, name, icon)."""
    box = Gtk.Box()
    box.add_css_class("segmented")
    first: Gtk.ToggleButton | None = None
    for key, name, *icon in choices:
        button = Gtk.ToggleButton(active=key == chosen)
        if icon:
            face = Gtk.Box(spacing=6)
            face.append(glyph(icon[0], 16))
            face.append(Gtk.Label(label=name))
            button.set_child(face)
        else:
            button.set_label(name)
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
