"""The installer's line icons, drawn for ZOOMRIYA OS in Box Desktop's style
(box_desktop/icons.py): a few strokes on a 24-unit grid, drawn at any size in
one colour. Ours, like the rest of this repository, so nothing here comes from
an icon theme."""

from __future__ import annotations

from functools import cache

import gi

gi.require_version("Gtk", "4.0")
gi.require_version("Gdk", "4.0")
from gi.repository import Gdk, GLib, Gtk  # noqa: E402

GLYPHS = {
    # The slideshow.
    "home": '<path d="M4 11l8-6.5 8 6.5"/><path d="M6 9.5V19.5h12V9.5"/><path d="M10 19.5v-5h4v5"/>',
    "shield": '<path d="M12 3l7.5 3v5.5c0 4.5-3.2 8-7.5 9.5-4.3-1.5-7.5-5-7.5-9.5V6z"/><path d="M8.5 12l2.5 2.5 4.5-5"/>',
    "restore": '<path d="M4 12a8 8 0 1 0 2.3-5.7L4 8.5M4 4.5v4h4"/><path d="M12 8v4.5l3 2"/>',
    "fallback": '<path d="M20 11a8 8 0 0 0-14.3-4.9L4 8M4 4v4h4M4 13a8 8 0 0 0 14.3 4.9L20 16M20 20v-4h-4"/>',
    "account": '<circle cx="12" cy="8.5" r="4"/><path d="M4.5 20.5a7.5 7.5 0 0 1 15 0"/>',
    "lock": '<rect x="5" y="10.5" width="14" height="10" rx="2"/><path d="M8 10.5V7.5a4 4 0 0 1 8 0v3"/>',
    # The drives.
    "disk": '<rect x="3" y="6" width="18" height="12" rx="2"/><path d="M3 13.5h18M7 16h.01M15 16h2"/>',
    "usb": '<rect x="7" y="9" width="10" height="12" rx="2"/><path d="M9 9V3h6v6M11 5.5h.01M13 5.5h.01"/>',
    "sd-card": '<path d="M7 3h7.5L18 6.5V21H7z"/><path d="M10 3v3.5M12.5 3v3.5"/>',
    # The summary's warning.
    "alert": '<path d="M12 3.5l9.5 16.5h-19z"/><path d="M12 10v4.5M12 17.5v.01"/>',
}


@cache
def texture(name: str, px: int, colour: str, width: float = 1.8) -> Gdk.Texture:
    svg = (f'<svg xmlns="http://www.w3.org/2000/svg" width="{px}" height="{px}" viewBox="0 0 24 24" '
           f'fill="none" stroke="{colour}" stroke-width="{width}" stroke-linecap="round" '
           f'stroke-linejoin="round">{GLYPHS[name]}</svg>')
    return Gdk.Texture.new_from_bytes(GLib.Bytes.new(svg.encode()))


def icon(name: str, px: int, colour: str) -> Gtk.Image:
    image = Gtk.Image.new_from_paintable(texture(name, px, colour))
    image.set_pixel_size(px)
    return image
