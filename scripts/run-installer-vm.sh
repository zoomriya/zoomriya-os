#!/bin/sh
# Boot the USB installer (scripts/build-installer.sh) in a window, as a USB
# stick, with an empty drive to install to (build/vm/install-target.raw, 64 GB,
# made once and kept, so what was installed can be booted afterwards with
# --target). The stick is opened read-only underneath (snapshot=on).
# build/vm/installer-qmp.sock is QEMU's control socket, to drive it from a
# script (screendump, input-send-event).
# Usage: scripts/run-installer-vm.sh [--target]   (--target: boot the drive
# installed to, without the stick)
set -eu

root=$(cd "$(dirname "$0")/.." && pwd)
stick="$root/build/installer/zoomriya-installer.raw"
vm="$root/build/vm"
target="$vm/install-target.raw"

if [ ! -f "$stick" ]; then
    echo "No installer yet: run scripts/build-installer.sh first." >&2
    exit 1
fi
mkdir -p "$vm"
[ -f "$target" ] || truncate -s 64G "$target"
cp /usr/share/OVMF/OVMF_VARS_4M.fd "$vm/installer-efivars.fd" 2>/dev/null || true

accel=tcg
[ -r /dev/kvm ] && [ -w /dev/kvm ] && accel=kvm

usb=""
if [ "${1:-}" != --target ]; then
    usb="-drive if=none,id=stick,format=raw,file=$stick,snapshot=on -device usb-storage,drive=stick,bootindex=0"
fi

# shellcheck disable=SC2086
exec qemu-system-x86_64 \
    -machine q35,accel="$accel" -cpu max -smp 2 -m 4096 \
    -drive if=pflash,format=raw,readonly=on,file=/usr/share/OVMF/OVMF_CODE_4M.fd \
    -drive if=pflash,format=raw,file="$vm/installer-efivars.fd" \
    -drive if=none,id=target,format=raw,file="$target" -device nvme,serial=ZTARGET1,drive=target,bootindex=1 \
    -nic user,model=virtio-net-pci \
    -device virtio-rng-pci \
    -vga none -device VGA,xres=1280,yres=800,edid=on -display gtk,show-menubar=off,zoom-to-fit=on \
    -device qemu-xhci -device usb-tablet \
    -qmp unix:"$vm/installer-qmp.sock",server,nowait \
    -serial file:"$vm/installer-serial.log" \
    $usb
