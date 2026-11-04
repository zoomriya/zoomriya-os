#!/bin/sh
# Boot the image to look around. Log in on the console as root, password
# "rootroot". The password is given to this VM only, by a unit passed in as a
# systemd credential, and is never in the image (the box has no default
# password). Quit with Ctrl-A then X; with --gui, close the window.
# Usage: scripts/run-vm.sh [--gui]
set -eu

root=$(cd "$(dirname "$0")/.." && pwd)
vm="$root/build/vm"
mkdir -p "$vm"

cat > "$vm/root-password.service" <<'EOF'
[Unit]
Description=Set root's password for this VM
DefaultDependencies=no
After=local-fs.target
Before=sysinit.target systemd-user-sessions.service
[Service]
Type=oneshot
ExecStart=/bin/sh -c 'echo root:rootroot | chpasswd'
EOF
printf '[Unit]\nWants=zoomriya-vm-root-password.service\n' > "$vm/root-password-wants.conf"
printf '[Unit]\nWants=serial-getty@ttyS0.service\n' > "$vm/getty.conf"

display="-nographic"
[ "${1:-}" = --gui ] && display="-serial mon:vc"

cred() {  # cred NAME FILE: a systemd credential, through SMBIOS
    printf 'type=11,value=io.systemd.credential.binary:%s=%s' "$1" "$(base64 -w0 "$2")"
}

# shellcheck disable=SC2086
exec "$root/scripts/qemu.sh" $display \
    -smbios "$(cred systemd.extra-unit.zoomriya-vm-root-password.service "$vm/root-password.service")" \
    -smbios "$(cred systemd.unit-dropin.sysinit.target~zoomriya-vm-root-password "$vm/root-password-wants.conf")" \
    -smbios "$(cred systemd.unit-dropin.multi-user.target~zoomriya-getty "$vm/getty.conf")"
