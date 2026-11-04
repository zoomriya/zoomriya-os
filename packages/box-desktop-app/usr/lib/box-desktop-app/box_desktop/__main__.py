"""python3 -m box_desktop: Box Desktop on the box's screen (BOX_DESKTOP_FAKE=1: without a box)."""

import os
import sys

from .app import BoxDesktopApp


def main() -> int:
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
