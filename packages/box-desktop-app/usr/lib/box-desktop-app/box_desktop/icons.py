"""The tools' icons, as in the mockup: a white line drawing on the tool's own
colour, in a rounded square; and the line icons of the top bar and Control
Panel's rows."""

from __future__ import annotations

from functools import cache

import gi

gi.require_version("Gtk", "4.0")
gi.require_version("Gdk", "4.0")
from gi.repository import Gdk, GLib, Gtk  # noqa: E402

COLOURS = {
    "control": "#475569",
    "storage": "#0F766E",
    "resources": "#7C3AED",
    "logs": "#B45309",
}

GLYPHS = {
    "control": '<path d="M4 7h9M17 7h3M4 17h4M12 17h8"/><circle cx="15" cy="7" r="2"/>'
               '<circle cx="10" cy="17" r="2"/>',
    "storage": '<rect x="3" y="5" width="18" height="14" rx="2"/><path d="M3 13h18M7 16h4"/>'
               '<circle cx="17" cy="16" r="0.8"/>',
    "resources": '<path d="M3 12h4l3-7 4 14 3-7h4"/>',
    "logs": '<path d="M7 3h7l4 4v14H7z"/><path d="M14 3v4h4M10 11h5M10 15h5"/>',
    # The top bar's icons; {c} is the icon's colour, for the shapes drawn filled.
    "bell": '<path d="M6 16V11a6 6 0 0 1 12 0v5l1.5 2h-15z"/><path d="M10 20.5a2 2 0 0 0 4 0"/>',
    "search": '<circle cx="11" cy="11" r="7"/><path d="M21 21l-4.35-4.35"/>',
    "widgets": '<rect x="3" y="4" width="18" height="16" rx="2"/><path d="M14 4v16"/>',
    "volume": '<path d="M4 9.5h3.5L12 6v12l-4.5-3.5H4z"/><path d="M15.5 9a4 4 0 0 1 0 6M18 6.5a7.5 7.5 0 0 1 0 11"/>',
    "muted": '<path d="M4 9.5h3.5L12 6v12l-4.5-3.5H4z"/><path d="M16 9.5l5 5M21 9.5l-5 5"/>',
    # Control Panel's rows.
    "mic": '<rect x="9" y="3" width="6" height="11" rx="3"/><path d="M5.5 11a6.5 6.5 0 0 0 13 0M12 17.5V21"/>',
    "mic-off": '<rect x="9" y="3" width="6" height="11" rx="3"/><path d="M5.5 11a6.5 6.5 0 0 0 13 0M12 17.5V21M4 4l16 16"/>',
    "gear": '<circle cx="12" cy="12" r="3"/><path d="M12 2.5v3M12 18.5v3M2.5 12h3M18.5 12h3M5.3 5.3l2.1 2.1'
            'M16.6 16.6l2.1 2.1M5.3 18.7l2.1-2.1M16.6 7.4l2.1-2.1"/>',
    "lock": '<rect x="5" y="10.5" width="14" height="10" rx="2"/><path d="M8 10.5V7.5a4 4 0 0 1 8 0v3"/>',
    "alert": '<path d="M12 3.5l9.5 16.5h-19z"/><path d="M12 10v4.5M12 17.5v.01"/>',
    "shield": '<path d="M12 3l7.5 3v5.5c0 4.5-3.2 8-7.5 9.5-4.3-1.5-7.5-5-7.5-9.5V6z"/><path d="M8.5 12l2.5 2.5 4.5-5"/>',
    "minimise": '<path d="M6 12h12"/>',
    "maximise": '<rect x="6" y="6" width="12" height="12" rx="1.5"/>',
    "close": '<path d="M7 7l10 10M17 7L7 17"/>',
    "check": '<path d="M5 12.5l4.5 4.5L19 7.5"/>',
    # The theme switch: the moon in Daylight (to Midnight), the sun in Midnight.
    "moon": '<path d="M20 14.5A8 8 0 0 1 9.5 4a8 8 0 1 0 10.5 10.5z"/>',
    "sun": '<circle cx="12" cy="12" r="4"/><path d="M12 2v2M12 20v2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4'
           'M2 12h2M20 12h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4"/>',
}

#: The line icons' colours in each theme (app.css's --text, --muted, --ok and --warn).
THEMES = {
    "daylight": {"text": "#1b2733", "muted": "#56636f", "ok": "#1f7a35", "warn": "#c93400"},
    "midnight": {"text": "#d6dbe5", "muted": "#838b9f", "ok": "#3ecf8e", "warn": "#f2c14e"},
}
_theme = "daylight"
# Held here, not weakly: Python's handle on a GTK image can go while the image
# stays on screen, and the theme switch must still reach it.
_shown: list[tuple[Gtk.Image, str, int, str]] = []


@cache
def texture(name: str, px: int, colour: str = "#ffffff", width: float = 1.8) -> Gdk.Texture:
    svg = (f'<svg xmlns="http://www.w3.org/2000/svg" width="{px}" height="{px}" viewBox="0 0 24 24" '
           f'fill="none" stroke="{colour}" stroke-width="{width}" stroke-linecap="round" '
           f'stroke-linejoin="round">{GLYPHS[name].replace("{c}", colour)}</svg>')
    return Gdk.Texture.new_from_bytes(GLib.Bytes.new(svg.encode()))


def glyph(name: str, px: int = 19, role: str = "text") -> Gtk.Image:
    """A line icon on its own (the top bar, the window buttons), drawn in the
    theme's colour for role, and again whenever the theme changes."""
    image = Gtk.Image.new_from_paintable(texture(name, px, THEMES[_theme][role], 1.8))
    _shown.append((image, name, px, role))
    image.set_pixel_size(px)
    # Centred at its own size: given more room, GTK would scale the drawing up.
    image.set_halign(Gtk.Align.CENTER)
    image.set_valign(Gtk.Align.CENTER)
    return image


def set_theme(theme: str) -> None:
    """Draws every line icon again in theme's colours."""
    global _theme
    _theme = theme
    # Images of closed windows (no longer on screen) are let go.
    _shown[:] = [entry for entry in _shown if entry[0].get_root() is not None or entry[0].get_parent()]
    for image, name, px, role in _shown:
        image.set_from_paintable(texture(name, px, THEMES[theme][role], 1.8))


def set_glyph(image: Gtk.Image, name: str) -> None:
    """Changes which drawing a line icon shows (the theme switch's moon or sun)."""
    for i, (shown, _name, px, role) in enumerate(_shown):
        if shown is image:
            _shown[i] = (shown, name, px, role)
            image.set_from_paintable(texture(name, px, THEMES[_theme][role], 1.8))
            return


def tile(kind: str, size: int = 52) -> Gtk.Widget:
    """A tool's icon: its glyph in white on its colour, corners a quarter round."""
    box = Gtk.Box(halign=Gtk.Align.CENTER, valign=Gtk.Align.CENTER, hexpand=False, vexpand=False)
    box.set_size_request(size, size)
    box.add_css_class("tile")
    box.add_css_class(f"tile-{kind}")
    box.add_css_class("tile-small" if size < 30 else "tile-large")
    px = round(size * 0.56)
    inner = Gtk.Image.new_from_paintable(texture(kind, px))
    inner.set_pixel_size(px)
    inner.set_halign(Gtk.Align.CENTER)
    inner.set_valign(Gtk.Align.CENTER)
    inner.set_size_request(px, px)
    inner.set_hexpand(True)
    box.append(inner)
    # The glyph may fill the tile; the tile itself stays its size.
    box.set_hexpand(False)
    return box


def tile_css() -> str:
    return "\n".join(f".tile-{kind} {{ background: {colour}; }}" for kind, colour in COLOURS.items())
