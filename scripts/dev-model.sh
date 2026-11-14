#!/bin/sh
# Run the box's model service (packages/zoomriya-model) for development on
# 127.0.0.1:8070, as on the box: the ZOOMRIYA apps' dev servers ask it there
# (zoomriya-apps' dev-all.sh starts it). Needs ZOOMRIYA ID on :8050 for its
# keys, and a model in /data/models/llms (scripts/download-gemma.sh).
#
# The first run builds llama.cpp (a few minutes); later runs reuse it.
# Usage: scripts/dev-model.sh
set -eu

root=$(cd "$(dirname "$0")/.." && pwd)
tree="$root/build/dev-model"

mkdir -p "$tree"
"$root/packages/zoomriya-model/build" "$tree" >/dev/null
lib="$tree/usr/lib/zoomriya-model"

export ZOOMRIYA_MODEL_ADDR="${ZOOMRIYA_MODEL_ADDR:-127.0.0.1:8070}"
export ZOOMRIYA_MODEL_SERVER="$lib/llama-server"
export ZOOMRIYA_MODEL_STATE="${ZOOMRIYA_MODEL_STATE:-$tree/state}"
echo "The box's model service on $ZOOMRIYA_MODEL_ADDR/myira/"
exec "$lib/zoomriya-modeld"
