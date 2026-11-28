#!/bin/sh
# Run the box's model service (packages/zoomriya-model) for development on its
# socket, as on the box: model.sock in $ZOOMRIYA_SOCKETS (default
# $XDG_RUNTIME_DIR/zoomriya), where the ZOOMRIYA apps' dev servers ask it
# (zoomriya-apps' dev-all.sh starts it). No TCP port. Needs ZOOMRIYA ID's dev
# server (id.sock in the same folder) for its keys, and a model in
# /data/models/llms (scripts/download-gemma.sh).
#
# The first run builds llama.cpp (a few minutes); later runs reuse it.
# Usage: scripts/dev-model.sh
set -eu

root=$(cd "$(dirname "$0")/.." && pwd)
tree="$root/build/dev-model"

mkdir -p "$tree"
"$root/packages/zoomriya-model/build" "$tree" >/dev/null
lib="$tree/usr/lib/zoomriya-model"

export ZOOMRIYA_SOCKETS="${ZOOMRIYA_SOCKETS:-${XDG_RUNTIME_DIR:-/tmp}/zoomriya}"
install -d -m 0700 "$ZOOMRIYA_SOCKETS" "$tree/run"
export ZOOMRIYA_MODEL_SOCKET="${ZOOMRIYA_MODEL_SOCKET:-$ZOOMRIYA_SOCKETS/model.sock}"
# llama-server's own socket, in a folder only this account can open.
chmod 0700 "$tree/run"
export ZOOMRIYA_MODEL_SERVER_SOCKET="$tree/run/llama.sock"
export ZOOMRIYA_MODEL_SERVER="$lib/llama-server"
export ZOOMRIYA_MODEL_STATE="${ZOOMRIYA_MODEL_STATE:-$tree/state}"
echo "The box's model service on $ZOOMRIYA_MODEL_SOCKET (/myira/)"
exec "$lib/zoomriya-modeld"
