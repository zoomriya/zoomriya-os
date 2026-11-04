#!/bin/sh
# Box Desktop as on the box's screen, in a window here, with sample data: no
# box, agent or ZOOMRIYA ID needed, and nothing on this computer changes. Sign
# in with any username and a password of 8 characters or more. Needs the
# system's Python with GTK 4 (python3-gi, python3-gi-cairo, gir1.2-gtk-4.0).
# Usage: scripts/preview-desktop.sh
set -eu

root=$(cd "$(dirname "$0")/.." && pwd)
packages="$root/packages"
export BOX_DESKTOP_FAKE=1 BOX_DESKTOP_WINDOWED=1
export PYTHONPATH="$packages/box-desktop-app/usr/lib/box-desktop-app:$packages/zoomriya-setup/usr/lib/zoomriya-setup"
exec /usr/bin/python3 -m box_desktop
