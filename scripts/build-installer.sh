#!/bin/sh
# Build the USB installer into build/installer/zoomriya-installer.raw, carrying
# the system image from scripts/build-image.sh. Write it to a stick with
#   sudo dd if=build/installer/zoomriya-installer.raw of=/dev/sdX bs=4M conv=fsync
# Usage: scripts/build-installer.sh [extra mkosi options]
set -eu

root=$(cd "$(dirname "$0")/.." && pwd)

if [ ! -f "$root/build/image/zoomriya-os.raw" ]; then
    echo "No image yet: run scripts/build-image.sh first." >&2
    exit 1
fi
"$root/scripts/build-packages.sh"

mkdir -p "$root/build/installer" "$root/build/cache"
mkosi -C "$root/mkosi/installer" --image-version="$(date -u +%Y.%-m.%-d)" --force "$@" build

echo "Built $root/build/installer/zoomriya-installer.raw"
