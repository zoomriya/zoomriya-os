"""The ZOOMRIYA logo, drawn as the apps draw it (@zoomriya/ui BrandLogo and
BrandMark, whose images logo/ copies): the letters, or the infinity's loops, in
the widget's CSS colour (the brand navy on light pages, white or light on dark
ones); the swoosh in its own blue-to-cyan. The wordmark goes where you sign in
or start; the mark alone where there's no room for it.

A copy of zoomriya_setup/brand.py: the installer doesn't have that package.
"""

from __future__ import annotations

from pathlib import Path
from typing import Any

import cairo
import gi

gi.require_version("Gtk", "4.0")
gi.require_foreign("cairo")
from gi.repository import Gtk  # noqa: E402

LOGO = Path(__file__).parent / "logo"
#: The images' names: one colour (a mask), then the swoosh.
WORDMARK = ("wordmark-letters.png", "wordmark-swoosh.png")
MARK = ("mark-loops.png", "mark-swoosh.png")


def logo(images: tuple[str, str], height: int, *classes: str) -> Gtk.DrawingArea:
    """The wordmark or the mark, `height` pixels high; its width follows."""
    shape, swoosh = (cairo.ImageSurface.create_from_png(str(LOGO / name)) for name in images)
    scale = height / shape.get_height()

    def draw(area: Gtk.DrawingArea, cr: Any, w: int, h: int) -> None:
        colour = area.get_color()
        # Centred in the room it's given (whole pixels can't always be).
        cr.translate((w - shape.get_width() * scale) / 2, (h - height) / 2)
        cr.scale(scale, scale)
        cr.set_source_rgba(colour.red, colour.green, colour.blue, colour.alpha)
        cr.mask_surface(shape, 0, 0)
        cr.set_source_surface(swoosh, 0, 0)
        cr.paint()

    area = Gtk.DrawingArea(content_width=round(shape.get_width() * scale), content_height=height,
                           halign=Gtk.Align.START, valign=Gtk.Align.CENTER)
    area.set_draw_func(draw)
    area.update_property([Gtk.AccessibleProperty.LABEL], ["ZOOMRIYA"])
    area.add_css_class("logo")
    for name in classes:
        area.add_css_class(name)
    return area
