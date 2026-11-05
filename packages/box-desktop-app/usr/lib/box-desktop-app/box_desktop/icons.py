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
    # Security's Protection rows.
    "updates": '<path d="M12 3l7.5 3v5.5c0 4.5-3.2 8-7.5 9.5-4.3-1.5-7.5-5-7.5-9.5V6z"/><path d="M12 8v6.5M9 11.5l3 3 3-3"/>',
    "firewall": '<path d="M3 5h18v14H3zM3 9.7h18M3 14.3h18M8 5v4.7M15 5v4.7M11.5 9.7v4.6M6 14.3V19M17 14.3V19"/>',
    "apparmor": '<path d="M12 3l7.5 3v5.5c0 4.5-3.2 8-7.5 9.5-4.3-1.5-7.5-5-7.5-9.5V6z"/>'
                '<rect x="9" y="11" width="6" height="5" rx="1"/><path d="M10.2 11V9.5a1.8 1.8 0 0 1 3.6 0V11"/>',
    "key": '<circle cx="8" cy="12" r="4"/><path d="M12 12h9M18 12v3M21 12v2"/>',
    "refresh": '<path d="M20 11a8 8 0 0 0-14.3-4.9L4 8M4 4v4h4M4 13a8 8 0 0 0 14.3 4.9L20 16M20 20v-4h-4"/>',
    "chevron-down": '<path d="M6 9.5l6 6 6-6"/>',
    "chevron-up": '<path d="M6 14.5l6-6 6 6"/>',
    # Keyboard & Mouse.
    "keyboard": '<rect x="2.5" y="6" width="19" height="12" rx="2"/><path d="M6 9.5h.01M9 9.5h.01M12 9.5h.01'
                'M15 9.5h.01M18 9.5h.01M6 12.5h.01M9 12.5h.01M12 12.5h.01M15 12.5h.01M18 12.5h.01M8 15.5h8"/>',
    "mouse": '<rect x="6.5" y="3" width="11" height="18" rx="5.5"/><path d="M12 6.5v3"/>',
    "pointer": '<path d="M6 3.5l12 8.5-5.2 1 3 5.6-2.4 1.3-3-5.6L6.5 17.5z"/>',
    "accel": '<path d="M10 4.5l10 7.2-4.4.9 2.5 4.7-2 1.1-2.5-4.7-3.4 3z"/><path d="M2.5 8h4.5M3.5 11.5h4M2.5 15h4"/>',
    "scroll": '<rect x="7.5" y="5.5" width="9" height="13" rx="4.5"/><path d="M12 8.5v2.5M10 3l2-1.5L14 3M10 21l2 1.5 2-1.5"/>',
    "screen-move": '<rect x="3" y="4" width="18" height="12.5" rx="1.5"/><path d="M8 20.5h8M12 16.5v4'
                   'M7.5 12l3-3M7.5 12v-2.2M7.5 12h2.2M16.5 8.5l-3 3M16.5 8.5v2.2M16.5 8.5h-2.2"/>',
    "content-move": '<path d="M10 21v-8.5a1.5 1.5 0 0 1 3 0v3.5l4 .8a2 2 0 0 1 1.6 2.3L18 21"/>'
                    '<path d="M13 11V6.5a1.5 1.5 0 0 0-3 0v6M15.5 4.5h5M18.5 2.5l2 2-2 2"/>',
    # Network.
    "ethernet": '<rect x="7" y="3" width="10" height="9" rx="1"/><path d="M9.5 3v2.5M12 3v2.5M14.5 3v2.5'
                'M9 12v3h6v-3M12 15v6"/>',
    "speed": '<path d="M4.5 17a8.5 8.5 0 1 1 15 0"/><path d="M12 13l4-4.5"/><circle cx="12" cy="13" r="1.2"/>',
    "wifi-off": '<path d="M5 12.5a10 10 0 0 1 4-2.3M2 9a14.5 14.5 0 0 1 4.5-2.8M14.5 10.2A10 10 0 0 1 19 12.5'
                'M17.5 6.2A14.5 14.5 0 0 1 22 9M8.5 16a5 5 0 0 1 7 0M12 19.5h.01M3 3l18 18"/>',
    "swap": '<path d="M4 8h15M15 4l4 4-4 4M20 16H5M9 12l-4 4 4 4"/>',
    "ip-screen": '<rect x="2.5" y="4" width="14" height="10.5" rx="1.5"/><path d="M6 18.5h5M9.5 14.5v4"/>'
                 '<rect x="13.5" y="12" width="8" height="8" rx="1.5"/>'
                 '<path d="M15.7 14v4M17.8 18v-4h1.2a1.1 1.1 0 0 1 0 2.2h-1.2"/>',
    "layers": '<path d="M12 3.5l9 4.5-9 4.5-9-4.5z"/><path d="M3 12l9 4.5 9-4.5M3 16l9 4.5 9-4.5"/>',
    "router": '<rect x="3" y="13" width="18" height="6.5" rx="1.5"/><path d="M7 16.2h.01M10 16.2h.01M12 13V9'
              'M8.5 7.5a5 5 0 0 1 7 0M6 5a8.5 8.5 0 0 1 12 0"/>',
    "dns": '<rect x="3" y="4" width="11" height="7" rx="1.2"/><rect x="3" y="13" width="11" height="7" rx="1.2"/>'
           '<path d="M6 7.5h.01M6 16.5h.01M17 4.5v6h1a3 3 0 0 0 0-6zM21 14.5a2 2 0 0 0-2-1.5 1.8 1.8 0 0 0 0 3.6'
           ' 1.8 1.8 0 0 1 0 3.6 2 2 0 0 1-2-1.5"/>',
    # Display.
    "monitor": '<rect x="3" y="4" width="18" height="12.5" rx="1.5"/><path d="M8 20.5h8M12 16.5v4"/>',
    "rotate": '<rect x="8" y="8" width="10" height="13" rx="1.5"/><path d="M4 11a8 8 0 0 1 10-7.5M14 3.5l-2.5-1.5'
              'M14 3.5l-1.5 2.5"/>',
    "resolution": '<path d="M4 8V4h4M16 4h4v4M20 16v4h-4M8 20H4v-4"/>'
                  '<path d="M7.5 15V9h2a1.6 1.6 0 0 1 0 3.2h-2M13 9l3.5 6M16.5 9L13 15"/>',
    "rate": '<circle cx="12" cy="13.5" r="7.5"/><path d="M10 2.5h4M12 2.5v3.5M7.5 13.5h2.5l1.2-2.5 1.6 5'
            ' 1.2-2.5h2.5"/>',
    "text-size": '<path d="M3 17l4-11 4 11M4.5 13h5M17.5 17v-5.5a2.5 2.5 0 0 0-5 0M17.5 14.5h-2.5a2 2 0 0 0 0 4h2.5'
                 'M3 20.5h18"/>',
    # Restarts.
    "info": '<circle cx="12" cy="12" r="8.5"/><path d="M12 11v5M12 8h.01"/>',
    "alert-gear": '<path d="M12 2.5l2 2.2 2.9-.6.6 2.9 2.7 1.3-1.2 2.7 1.2 2.7-2.7 1.3-.6 2.9-2.9-.6-2 2.2-2-2.2'
                  '-2.9.6-.6-2.9-2.7-1.3 1.2-2.7-1.2-2.7 2.7-1.3.6-2.9 2.9.6z"/><path d="M12 8v4.5M12 15.5h.01"/>',
    "maintenance": '<path d="M4 12a8 8 0 1 0 2.3-5.7L4 8.5M4 4.5v4h4"/><path d="M12 8v4.5l3 2"/>',
    "scan": '<rect x="3.5" y="5" width="15" height="14" rx="2"/><path d="M3.5 9.5h15M7.5 3v4M14.5 3v4"/>'
            '<circle cx="18" cy="18" r="3.5"/><path d="M16.5 18l1 1 2-2"/>',
    "power": '<path d="M12 3v8M7 6.3a7.5 7.5 0 1 0 10 0"/>',
    # Restore & Reset.
    "calendar-clock": '<rect x="3.5" y="5" width="13" height="13" rx="2"/><path d="M3.5 9.5h13M7 3v4M13 3v4"/>'
                      '<circle cx="17" cy="17" r="4"/><path d="M17 15v2l1.3 1"/>',
    "restore-point": '<path d="M6 3.5h8l4 4v6M6 3.5v17h6"/><path d="M14 3.5v4h4"/>'
                     '<path d="M15 21a3.5 3.5 0 1 0-3.2-4.8M11.5 14v2.5H14"/>',
    "factory": '<path d="M3 13.5l3-8h12l3 8v5a1.5 1.5 0 0 1-1.5 1.5h-15A1.5 1.5 0 0 1 3 18.5z"/>'
               '<path d="M3 13.5h5l1.5 2.5h5l1.5-2.5h5"/>',
    "held": '<circle cx="6" cy="5.5" r="2"/><circle cx="6" cy="18.5" r="2"/><circle cx="18" cy="12" r="2"/>'
            '<path d="M6 7.5v9M8 5.5h3a5 5 0 0 1 5 5v-.5"/>',
    "reset": '<path d="M9 4L5 8l4 4"/><path d="M5 8h9a5 5 0 0 1 0 10H8"/>',
    "erase": '<rect x="3.5" y="3.5" width="12" height="15" rx="1.5"/><path d="M7 8h5M7 11.5h5"/>'
             '<path d="M16 15.5l5 5M21 15.5l-5 5"/>',
    "trash": '<path d="M4.5 6.5h15M9.5 6.5V4h5v2.5M6.5 6.5l1 13.5h9l1-13.5M10 10.5v6M14 10.5v6"/>',
    # Sound.
    "speaker": '<rect x="5" y="2.5" width="14" height="19" rx="2"/><circle cx="12" cy="14.5" r="3.5"/>'
               '<path d="M12 7h.01"/>',
    "sliders": '<path d="M5 4v16M12 4v16M19 4v16"/><circle cx="5" cy="14" r="2"/><circle cx="12" cy="8" r="2"/>'
               '<circle cx="19" cy="16" r="2"/>',
    "wave": '<path d="M3 12h2M7 8v8M10 5v14M13 9v6M16 7v10M19 10v4M21 12h.01"/>',
    "play": '<path d="M7 4.5v15l12-7.5z"/>',
    # Language, time and System Info.
    "globe": '<circle cx="12" cy="12" r="8.5"/><path d="M3.5 12h17M12 3.5a13 13 0 0 1 0 17M12 3.5a13 13 0 0 0 0 17"/>',
    "pin": '<path d="M12 21s-6.5-6-6.5-11a6.5 6.5 0 0 1 13 0c0 5-6.5 11-6.5 11z"/><circle cx="12" cy="10" r="2.3"/>',
    "tag": '<path d="M3.5 12.5V4.5a1 1 0 0 1 1-1h8l8 8-9 9z"/><circle cx="8" cy="8" r="1.4"/>',
    "box": '<rect x="3" y="8" width="18" height="9" rx="2"/><path d="M7 12.5h.01M10 12.5h.01M15 12.5h3"/>',
    "barcode": '<path d="M4 5v14M7 5v14M10.5 5v14M13 5v14M17 5v14M20 5v14"/>',
    "cpu": '<rect x="6" y="6" width="12" height="12" rx="1.5"/><rect x="9.5" y="9.5" width="5" height="5"/>'
           '<path d="M9 3v3M15 3v3M9 18v3M15 18v3M3 9h3M3 15h3M18 9h3M18 15h3"/>',
    "memory": '<rect x="2.5" y="7" width="19" height="9" rx="1"/><path d="M6 10v3M9.5 10v3M13 10v3M16.5 10v3'
              'M5 16v2.5M19 16v2.5"/>',
    "disk": '<ellipse cx="12" cy="6" rx="8" ry="2.8"/><path d="M4 6v12c0 1.5 3.6 2.8 8 2.8s8-1.3 8-2.8V6'
            'M4 12c0 1.5 3.6 2.8 8 2.8s8-1.3 8-2.8"/>',
    "thermometer": '<path d="M10 14.5V5a2 2 0 0 1 4 0v9.5a4 4 0 1 1-4 0z"/><path d="M12 9v7"/>',
    "user": '<circle cx="12" cy="8.5" r="4"/><path d="M4.5 20.5a7.5 7.5 0 0 1 15 0"/>',
    # Its outbound connections.
    "package": '<path d="M12 3l8 4.5v9L12 21l-8-4.5v-9z"/><path d="M4 7.5l8 4.5 8-4.5M12 12v9"/>',
    "clock": '<circle cx="12" cy="12" r="8.5"/><path d="M12 7.5V12l3 2"/>',
    # Security's Allowed Network Connections, by what each port is for.
    "web": '<rect x="3" y="4.5" width="18" height="15" rx="2"/><path d="M3 9h18M6 6.75h.01M8.5 6.75h.01M11 6.75h.01"/>',
    "discovery": '<circle cx="12" cy="10" r="1.6"/><path d="M12 11.6L9 21h6l-3-9.4"/>'
                 '<path d="M8.5 6.5a5 5 0 0 0 0 7M15.5 6.5a5 5 0 0 1 0 7M5.7 3.7a9 9 0 0 0 0 12.6'
                 'M18.3 3.7a9 9 0 0 1 0 12.6"/>',
    "address": '<rect x="4" y="3.5" width="16" height="17" rx="2.5"/><path d="M4 14.5h16M15 17.5h2"/>'
               '<path d="M8.5 6.5v5M11.5 11.5v-5h2.5a1.5 1.5 0 0 1 0 3h-2.5"/>',
    "terminal": '<rect x="3" y="4.5" width="18" height="15" rx="2"/><path d="M7 10l3 2.5L7 15M12.5 15H17"/>',
    "app": '<rect x="4" y="4" width="7" height="7" rx="1.5"/><rect x="13" y="4" width="7" height="7" rx="1.5"/>'
           '<rect x="4" y="13" width="7" height="7" rx="1.5"/><rect x="13" y="13" width="7" height="7" rx="1.5"/>',
    # The theme switch: the moon in Daylight (to Midnight), the sun in Midnight.
    "moon": '<path d="M20 14.5A8 8 0 0 1 9.5 4a8 8 0 1 0 10.5 10.5z"/>',
    "sun": '<circle cx="12" cy="12" r="4"/><path d="M12 2v2M12 20v2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4'
           'M2 12h2M20 12h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4"/>',
}

#: The line icons' colours in each theme (app.css's --text, --muted, --ok, --warn and --accent, and
#: an orange for AppArmor's shield and white for icons on a coloured button).
THEMES = {
    "daylight": {"text": "#1b2733", "muted": "#56636f", "ok": "#1f7a35", "warn": "#c93400", "accent": "#1765c1",
                 "orange": "#d9480f", "white": "#ffffff"},
    "midnight": {"text": "#d6dbe5", "muted": "#838b9f", "ok": "#3ecf8e", "warn": "#f2c14e", "accent": "#6f86e8",
                 "orange": "#fb923c", "white": "#ffffff"},
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
