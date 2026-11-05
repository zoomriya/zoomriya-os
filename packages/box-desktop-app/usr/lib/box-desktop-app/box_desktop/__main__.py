"""python3 -m box_desktop: Box Desktop on the box's screen (BOX_DESKTOP_FAKE=1: without a box)."""

import os
import sys
from pathlib import Path

from . import i18n
from .app import BoxDesktopApp


def box_language() -> str:
    """The box's language, as Debian keeps it (LANG in /etc/default/locale);
    BOX_DESKTOP_LANGUAGE for a preview."""
    if os.environ.get("BOX_DESKTOP_LANGUAGE"):
        return os.environ["BOX_DESKTOP_LANGUAGE"]
    try:
        for line in Path("/etc/default/locale").read_text().splitlines():
            if line.startswith("LANG="):
                return line.split("=", 1)[1].strip().strip('"')
    except OSError:
        pass
    return os.environ.get("LANG", "en")


def main() -> int:
    i18n.use(box_language())
    if os.environ.get("BOX_DESKTOP_FAKE"):
        from .fake import FakeBoxAgent, FakeDisplay, FakeID, FakeSound

        parts = (FakeBoxAgent(), FakeID(), FakeSound(), True, FakeDisplay())
    else:
        from zoomriya_setup.agent import Agent

        from .display import Display
        from .signin import ZoomriyaID
        from .sound import Sound, has_hardware

        parts = (Agent(), ZoomriyaID(), Sound(), has_hardware(), Display())
    return BoxDesktopApp(*parts).run([sys.argv[0]])


if __name__ == "__main__":
    sys.exit(main())
