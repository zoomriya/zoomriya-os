#!/bin/sh
# A screenshot of what the running VM (scripts/run-vm.sh) shows, as a PNG.
# Usage: scripts/vm-screenshot.sh [file.png]   (default build/vm/screen.png)
set -eu

root=$(cd "$(dirname "$0")/.." && pwd)
out=$(realpath -m "${1:-$root/build/vm/screen.png}")
sock="$root/build/vm/qmp.sock"
[ -S "$sock" ] || { echo "No VM running (start it with scripts/run-vm.sh)." >&2; exit 1; }

python3 - "$sock" "$out" <<'PY'
import json, socket, sys
sock, out = sys.argv[1], sys.argv[2]
with socket.socket(socket.AF_UNIX) as s:
    s.connect(sock)
    f = s.makefile("rw")
    f.readline()  # QEMU's greeting
    for command in ({"execute": "qmp_capabilities"},
                    {"execute": "screendump", "arguments": {"filename": out, "format": "png"}}):
        f.write(json.dumps(command) + "\n"); f.flush()
        while True:
            answer = json.loads(f.readline())
            if "return" in answer or "error" in answer:
                break
        if "error" in answer:
            sys.exit(answer["error"]["desc"])
print(out)
PY
