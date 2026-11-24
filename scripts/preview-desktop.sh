#!/bin/sh
# Box Desktop with sample data, in a browser on this computer: no box, agent or
# ZOOMRIYA ID needed, and nothing on this computer changes. Builds the page,
# starts the sample box (packages/box-desktop/preview) and box-desktopd, and
# prints the address. Sign in with any username and a password of 8
# characters or more. Needs Go, Node and the system's Python.
# Usage: scripts/preview-desktop.sh [--no-build] [--setup] [--lan]
#   --setup: the box not set up yet: the first setup, on the box's screen's side
#   --lan: also from other devices on this network (a phone, another computer),
#          at this computer's address; plain HTTP, sample data only
#   --no-model: Myira's panel with no model yet, to try Download (a sample)
#   BOX_DESKTOP_LANGUAGE=fr scripts/preview-desktop.sh   the page in French
# Myira's panel answers with a sample model: picks by a few words, and a
# fixed sentence said to be the preview's (preview/samplemodel.py).
set -eu

root=$(cd "$(dirname "$0")/.." && pwd)
pkg="$root/packages/box-desktop"
state=$(mktemp -d)
trap 'kill $(jobs -p) 2>/dev/null; rm -rf "$state"' EXIT INT TERM

build=yes setup=no host=127.0.0.1
for arg in "$@"; do
    case "$arg" in
        --no-build) build=no ;;
        --setup) setup=yes ;;
        --lan) host=0.0.0.0 ;;
        --no-model) export FAKEBOX_NO_MODEL=1 ;;
    esac
done
[ "$setup" = yes ] || touch "$state/setup-done"
if [ "$build" = yes ]; then
    (cd "$pkg/web" && { [ -d node_modules ] || npm ci --no-audit --no-fund; } && npm run build >/dev/null)
fi

FAKEBOX_SETUP_DONE="$state/setup-done" PYTHONPATH="$pkg/preview" \
    python3 "$pkg/preview/fakebox.py" "$state/agent.sock" 17050 "$host" &
# The address other devices reach this computer at (--lan), else this one.
here=127.0.0.1
[ "$host" = 127.0.0.1 ] || here=$(ip -4 route get 1.1.1.1 2>/dev/null | sed -n 's/.* src \([0-9.]*\).*/\1/p')

export BOX_DESKTOP_FAKE=1 BOX_DESKTOP_INSECURE=1
export BOX_DESKTOP_LISTEN=$host:17443 BOX_DESKTOP_SCREEN=127.0.0.1:17444
export BOX_DESKTOP_AGENT="$state/agent.sock" BOX_DESKTOP_ID_API=http://127.0.0.1:17050/id/api/v1
export BOX_DESKTOP_ADMIN_DIR="$state/admin" BOX_DESKTOP_CERTS="$state/certs"
export BOX_DESKTOP_APP_BASE=http://$here:17050/app/
# Myira: the OS's own, with the sample model; never this computer's Myira app.
export BOX_DESKTOP_MODEL_API=http://127.0.0.1:17050/model/myira/api/v1 BOX_DESKTOP_MYIRA_APP=
# The page as built, from disk: a rebuild shows at the next reload.
export BOX_DESKTOP_WEB="$pkg/src/dist"
export BOX_DESKTOP_SETUP_DONE="$state/setup-done" STATE_DIRECTORY="$state/state"
echo "Box Desktop: http://$here:17443/  (the box's screen's side, on this computer only: http://127.0.0.1:17444/)"
cd "$pkg/src" && go run .
