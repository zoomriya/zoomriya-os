#!/bin/sh
# The USB installer with sample drives, in a browser on this computer: nothing
# is written anywhere. Builds the page (packages/box-desktop/web, installer.html),
# then serves it as the stick does, and prints the address.
# Usage: scripts/preview-installer.sh [--no-build]
#   ZOOMRIYA_INSTALLER_FAKE=fail scripts/preview-installer.sh      the copy fails
#   ZOOMRIYA_INSTALLER_FAKE=offline scripts/preview-installer.sh   no internet
set -eu

root=$(cd "$(dirname "$0")/.." && pwd)
web="$root/packages/box-desktop/web"
locales=$(mktemp -d)
trap 'rm -rf "$locales"' EXIT INT TERM

if [ "${1:-}" != --no-build ]; then
    (cd "$web" && { [ -d node_modules ] || npm ci --no-audit --no-fund; } && npm run build:installer >/dev/null)
fi
# The server's own words (the drives, the install) in each language.
python3 -I "$root/scripts/i18n.py" locales "$locales"

export ZOOMRIYA_INSTALLER_FAKE="${ZOOMRIYA_INSTALLER_FAKE:-1}" ZOOMRIYA_INSTALLER_WEB="$web/dist-installer"
export ZOOMRIYA_LOCALEDIR="$locales" ZOOMRIYA_INSTALLER_ADDRESS=127.0.0.1:7445
export PYTHONPATH="$root/packages/zoomriya-installer/usr/lib/zoomriya-installer"
echo "The installer: http://127.0.0.1:7445/"
exec python3 -m zoomriya_installer
