"""The image installs our packages offline (mkosi/host/mkosi.postinst.chroot:
dpkg -i, no network), so everything they depend on must already be in
mkosi/host/mkosi.conf's Packages=, or the image doesn't build.

    python3 -m unittest discover -s tests/packages
"""

import re
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
#: In every Debian system already (Priority: required).
ESSENTIAL = {"base-files", "dpkg", "coreutils", "bash", "sed", "grep", "util-linux"}


def image_packages() -> set[str]:
    block = (ROOT / "mkosi/host/mkosi.conf").read_text().split("Packages=", 1)[1]
    found = set()
    for line in block.splitlines()[1:]:
        if line and not line[0].isspace():
            break
        word = line.strip()
        if word and not word.startswith("#"):
            found.add(word)
    return found


def depends(control: Path) -> list[list[str]]:
    """Each dependency, as its alternatives (a | b), names only."""
    found = re.search(r"^Depends:(.*(?:\n .*)*)", control.read_text(), re.M)
    if not found:
        return []
    return [[re.split(r"[\s(]", alt.strip())[0] for alt in dep.split("|")]
            for dep in found.group(1).replace("\n", " ").split(",") if dep.strip()]


class ImagePackages(unittest.TestCase):
    def test_every_dependency_is_in_the_image(self):
        listed = image_packages()
        ours = {c.parent.parent.name for c in (ROOT / "packages").glob("*/DEBIAN/control")}
        for control in sorted((ROOT / "packages").glob("*/DEBIAN/control")):
            package = control.parent.parent.name
            if package == "zoomriya-installer":
                continue  # on the USB stick only (mkosi/installer)
            for alternatives in depends(control):
                with self.subTest(package=package, needs=" | ".join(alternatives)):
                    self.assertTrue(any(a in listed or a in ours or a in ESSENTIAL for a in alternatives),
                                    f"{package} needs {alternatives}: add it to mkosi/host/mkosi.conf's Packages=")


if __name__ == "__main__":
    unittest.main()
