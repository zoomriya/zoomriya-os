#!/bin/sh
# Install what building and testing ZOOMRIYA OS needs, on a Debian 13 machine.
# Installs only the missing packages (asks for sudo), then checks KVM and the
# subordinate IDs mkosi uses to build without root.
# Usage: scripts/setup.sh
set -eu

packages="
    mkosi
    systemd-repart
    systemd-container
    systemd-ukify
    python3-pefile
    apt
    dpkg-dev
    golang-go
    debian-archive-keyring
    reprepro
    btrfs-progs
    dosfstools
    mtools
    e2fsprogs
    cpio
    zstd
    kmod
    uidmap
    qemu-system-x86
    qemu-utils
    ovmf
    erofs-utils
"

missing=""
for p in $packages; do
    if ! dpkg-query -W -f='${Status}' "$p" 2>/dev/null | grep -q '^install ok installed$'; then
        missing="$missing $p"
    fi
done

if [ -n "$missing" ]; then
    echo "Installing:$missing"
    sudo apt-get update
    # shellcheck disable=SC2086
    sudo apt-get install -y --no-install-recommends $missing
else
    echo "All packages are installed."
fi

version=$(mkosi --version | awk '{print $2}')
if [ "${version%%.*}" -lt 25 ]; then
    echo "mkosi $version is too old; 25 or later is needed (Debian 13 has it)." >&2
    exit 1
fi

user=$(id -un)
if ! grep -q "^$user:" /etc/subuid || ! grep -q "^$user:" /etc/subgid; then
    echo "Adding subordinate user and group IDs for $user (mkosi builds without root)."
    sudo usermod --add-subuids 100000-165535 --add-subgids 100000-165535 "$user"
fi

if [ ! -e /dev/kvm ]; then
    echo "Warning: no /dev/kvm. VMs will run, but very slowly." >&2
elif [ ! -r /dev/kvm ] || [ ! -w /dev/kvm ]; then
    echo "Adding $user to the kvm group; log out and in again for it to apply."
    sudo usermod -aG kvm "$user"
fi

echo "Ready. Next: scripts/build-image.sh"
