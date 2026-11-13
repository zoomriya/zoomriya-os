#!/bin/sh
# Build our .deb packages from packages/<name>/ into build/packages/.
# Packages with code are built first: the agent in Go, and ZOOMRIYA ID.
# Usage: scripts/build-packages.sh
set -eu

root=$(cd "$(dirname "$0")/.." && pwd)
out="$root/build/packages"
work="$root/build/package-trees"

rm -rf "$out" "$work"
mkdir -p "$out" "$work"

for dir in "$root"/packages/*/; do
    name=$(basename "$dir")
    # Copy, so modes are set here and not by the checkout's or the build's
    # umask: readable by all, writable by root, executable where git (or the
    # build) made it executable.
    cp -a "$dir" "$work/$name"
    # A package with code to build has a build script and its source in src/;
    # neither goes into the package.
    if [ -x "$dir/build" ]; then
        "$dir/build" "$work/$name"
        rm -rf "$work/$name/build" "$work/$name/src" "$work/$name/web" "$work/$name/preview" "$work/$name/.gitignore"
    fi
    # Translations of our screens (po/<lang>.po) go in compiled, where gettext
    # looks; the .po files themselves don't. A web page has its own, built in.
    if [ -d "$dir/po" ] && [ ! -d "$dir/web" ]; then
        for po in "$dir"/po/*.po; do
            lang=$(basename "$po" .po)
            python3 -I "$root/scripts/i18n.py" compile "$po" \
                "$work/$name/usr/share/locale/$lang/LC_MESSAGES/$name.mo"
        done
    fi
    rm -rf "$work/$name/po"
    # Python's caches from running the code here aren't part of a package.
    find "$work/$name" -name __pycache__ -prune -exec rm -rf {} +
    chmod -R u+rwX,go+rX,go-w "$work/$name"
    for script in preinst postinst prerm postrm; do
        [ ! -f "$work/$name/DEBIAN/$script" ] || chmod 0755 "$work/$name/DEBIAN/$script"
    done
    dpkg-deb --root-owner-group -Zxz --build "$work/$name" "$out" >/dev/null
done

ls -1 "$out"
