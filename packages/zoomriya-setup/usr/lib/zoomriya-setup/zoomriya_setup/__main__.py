"""python3 -m zoomriya_setup: the first setup (ZOOMRIYA_SETUP_FAKE=1: without a box)."""

import os
import sys

from .app import SetupApp


def main() -> int:
    if os.environ.get("ZOOMRIYA_SETUP_FAKE"):
        from .fake import FakeAgent

        agent = FakeAgent()
    else:
        from .agent import Agent

        agent = Agent()
    return SetupApp(agent).run([sys.argv[0]])


if __name__ == "__main__":
    sys.exit(main())
