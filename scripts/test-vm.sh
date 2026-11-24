#!/bin/sh
# Boot the image, run tests/vm/check.sh and tests/hardening/check.sh inside it
# (five boots: a kernel update, a restore, a kernel that can't start and
# Erase everything between them), and fail if any check fails. The checks and the unit that runs
# them are given to this VM only, as systemd credentials; the image itself is
# never changed for the test.
# Usage: scripts/test-vm.sh
set -eu

root=$(cd "$(dirname "$0")/.." && pwd)
vm="$root/build/vm"
log="$vm/test.log"
mkdir -p "$vm"

cat > "$vm/check.service" <<'EOF'
[Unit]
Description=ZOOMRIYA OS VM checks
[Service]
Type=simple
ImportCredential=zoomriya-*
StandardOutput=tty
StandardError=tty
TTYPath=/dev/ttyS0
ExecStart=/bin/sh -c 'exec /bin/sh "$CREDENTIALS_DIRECTORY/zoomriya-check.sh"'
EOF
printf '[Unit]\nWants=zoomriya-check.service\n' > "$vm/check-wants.conf"

# Two stand-ins for the library's drives: QEMU's NVMe drives report SMART.
truncate -s 8G "$vm/library-1.raw" "$vm/library-2.raw"

echo "Booting; the log is $log"
status=0
timeout 1800 "$root/scripts/qemu.sh" -display none -serial file:"$log" \
    -fw_cfg name=opt/io.systemd.credentials/zoomriya-check.sh,file="$root/tests/vm/check.sh" \
    -fw_cfg name=opt/io.systemd.credentials/zoomriya-hardening.sh,file="$root/tests/hardening/check.sh" \
    -netdev user,id=second -device virtio-net-pci,netdev=second \
    -drive if=none,id=library1,format=raw,file="$vm/library-1.raw" -device nvme,serial=ZLIB0001,drive=library1 \
    -drive if=none,id=library2,format=raw,file="$vm/library-2.raw" -device nvme,serial=ZLIB0002,drive=library2 \
    -smbios type=11,value=io.systemd.credential.binary:systemd.extra-unit.zoomriya-check.service="$(base64 -w0 "$vm/check.service")" \
    -smbios type=11,value=io.systemd.credential.binary:systemd.unit-dropin.multi-user.target~zoomriya-check="$(base64 -w0 "$vm/check-wants.conf")" \
    || status=$?

grep -a -E '^(PASS|FAIL|INFO) ' "$log" || true

if [ "$status" -eq 124 ]; then
    echo "The VM didn't finish within 30 minutes." >&2
    exit 1
fi
if ! grep -a -q '^ZOOMRIYA-CHECK-DONE' "$log"; then
    echo "The checks didn't run; see $log" >&2
    exit 1
fi
fails=$(grep -a -c '^FAIL ' "$log" || true)
if [ "$fails" -ne 0 ]; then
    echo "$fails check(s) failed." >&2
    exit 1
fi
echo "All checks passed."
