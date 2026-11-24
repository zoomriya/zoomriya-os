#!/bin/sh
# Build the system disk image into build/image/zoomriya-os.raw.
# Runs as your own user; mkosi needs no root.
# Usage: scripts/build-image.sh [extra mkosi options]
set -eu

root=$(cd "$(dirname "$0")/.." && pwd)

"$root/scripts/build-packages.sh"

# The factory image version is the build date (design §9).
version=$(date -u +%Y.%-m.%-d)

mkdir -p "$root/build/image" "$root/build/cache"
mkosi -C "$root/mkosi/host" --image-version="$version" --force "$@" build

echo "Built $root/build/image/zoomriya-os.raw ($version)"
