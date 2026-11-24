#!/bin/sh
# Download a Gemma 4 chat model and its vision file into /data/models/llms,
# where the box's model service (zoomriya-model) finds them. For development
# and for a box set up by hand; Box Desktop will offer the download itself
# (box-desktop.md BD-17).
#
#   e2b (default): gemma-4-E2B-it-Q4_K_M.gguf  (about 3.4 GB, faster)
#   e4b:           gemma-4-E4B-it-Q4_K_M.gguf  (about 5.3 GB, better at multi-step requests)
#
# each with mmproj-gemma-4-<variant>-it-BF16.gguf (about 1 GB), which lets it
# look at pictures. From lmstudio-community's GGUF builds (Apache-2.0,
# docs/licences.md). A file already there is kept.
#
# Usage: scripts/download-gemma.sh [e2b|e4b]
set -eu

case "${1:-e2b}" in
    e2b | E2B) variant=E2B ;;
    e4b | E4B) variant=E4B ;;
    *) echo "Unknown model '$1' (e2b or e4b)" >&2; exit 1 ;;
esac
url="https://huggingface.co/lmstudio-community/gemma-4-$variant-it-GGUF/resolve/main"
dir=/data/models/llms

sudo=""
if ! mkdir -p "$dir" 2>/dev/null || [ ! -w "$dir" ]; then
    sudo=sudo
    sudo mkdir -p "$dir"
fi

for name in "gemma-4-$variant-it-Q4_K_M.gguf" "mmproj-gemma-4-$variant-it-BF16.gguf"; do
    if [ -s "$dir/$name" ]; then
        echo "already there: $dir/$name"
        continue
    fi
    echo "downloading $name"
    $sudo wget -q --show-progress -O "$dir/$name.part" "$url/$name"
    $sudo mv "$dir/$name.part" "$dir/$name"
    $sudo chmod a+r "$dir/$name"
done
ls -lh "$dir"
