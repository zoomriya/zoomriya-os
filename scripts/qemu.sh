#!/bin/sh
# Boot the built image in QEMU with UEFI. Shared by run-vm.sh and test-vm.sh.
# The disk is opened read-only underneath (snapshot=on), so every boot starts
# from the image as built.
# Usage: scripts/qemu.sh [extra qemu options]
set -eu

root=$(cd "$(dirname "$0")/.." && pwd)
image="$root/build/image/zoomriya-os.raw"
vm="$root/build/vm"

if [ ! -f "$image" ]; then
    echo "No image yet: run scripts/build-image.sh first." >&2
    exit 1
fi

mkdir -p "$vm"
cp /usr/share/OVMF/OVMF_VARS_4M.fd "$vm/efivars.fd"

accel=tcg
[ -r /dev/kvm ] && [ -w /dev/kvm ] && accel=kvm

exec qemu-system-x86_64 \
    -machine q35,accel="$accel" -cpu max -smp 2 -m 2048 \
    -drive if=pflash,format=raw,readonly=on,file=/usr/share/OVMF/OVMF_CODE_4M.fd \
    -drive if=pflash,format=raw,file="$vm/efivars.fd" \
    -drive if=virtio,format=raw,file="$image",snapshot=on \
    -nic user,model=virtio-net-pci \
    -device virtio-rng-pci \
    "$@"
