#!/bin/sh
# Runs inside the VM (scripts/test-vm.sh), after each boot: one check per thing
# the image promises. Prints PASS or FAIL lines. The test takes three boots:
#   1. the image as built; then a restore point, and a kernel update;
#   2. the new kernel, started once from its entry and marked good; then a
#      restore to the point before the kernel update;
#   3. the restored OS, on the kernel it had; then a newer kernel entry that
#      can't start, with two tries;
#   4. the good kernel again, after the broken one used up its tries; then
#      Erase everything in Box Desktop;
#   5. the box as it left the factory, with nothing of the family's left.
# The phase is kept in /data, which a restore never touches. After the last
# phase it prints ZOOMRIYA-CHECK-DONE and powers the VM off.

fails=0
pass() { echo "PASS $1"; }
fail() { echo "FAIL $1: $2"; fails=$((fails + 1)); }
check() {  # check NAME EXPECTED ACTUAL
    if [ "$2" = "$3" ]; then pass "$1"; else fail "$1" "expected '$2', got '$3'"; fi
}

state=$(systemctl is-system-running --wait)
check "boot finished cleanly" running "$state"
if [ "$state" != running ]; then
    systemctl --failed --no-legend --plain
    for unit in $(systemctl --failed --no-legend --plain | awk '{ print $1 }'); do
        journalctl -q -b -u "$unit" -o cat | tail -5 | sed "s/^/INFO $unit: /"
        # Who keeps starting it (a unit hitting its start limit).
        journalctl -q -b -o short-monotonic _PID=1 | grep -E "auditd|audit-rules" | head -12 | sed "s/^/INFO $unit: /"
        journalctl -q -b -o cat _COMM=auditd | head -12 | sed "s/^/INFO $unit (auditd): /"
    done
fi

# The test's own notes live on the EFI partition, which no restore or reset
# touches; $testdir is in /data, to see what a restore keeps and an erase
# doesn't.
esp=$(bootctl --print-esp-path 2>/dev/null)
state=$esp/zoomriya-test
testdir=/data/zoomriya-test
mkdir -p "$state"
phase=$(cat "$state/phase" 2>/dev/null || echo 1)
[ "$phase" -ge 5 ] || mkdir -p "$testdir"
echo "PHASE $phase"
agent=/usr/lib/zoomriya-agent/zoomriya-agent

# A change as Box Desktop asks for it: the agent, in a signed-in admin's name.
act() {  # act REQUEST: "ok" or "refused"
    "$agent" ask "$1" >/dev/null 2>&1 && echo ok || echo refused
}

# ZOOMRIYA ID (its own repository, in the image as zoomriya-id): set up at the
# first start, and again after an erase, with no account and no default password.
id_status() {  # id_status METHOD PATH [JSON]: the HTTP code (python3: curl comes and goes)
    python3 - "$@" <<'PY'
import sys, urllib.error, urllib.request
method, path = sys.argv[1], sys.argv[2]
data = sys.argv[3].encode() if len(sys.argv) > 3 else None
url = "http://127.0.0.1:8050/id/api/v1" + path
request = urllib.request.Request(url, data=data, method=method, headers={"Content-Type": "application/json"})
try:
    print(urllib.request.urlopen(request, timeout=10).status)
except urllib.error.HTTPError as error:
    print(error.code)
except OSError as error:
    print(type(error).__name__)
PY
}
id_checks() {
check "PostgreSQL's cluster is online" online "$(pg_lsclusters -h | awk '{ print $4 }')"
check "Valkey running" active "$(systemctl is-active valkey-server.service)"
check "ZOOMRIYA ID set up" yes "$([ -f /var/lib/zoomriya-id/set-up ] && echo yes)"
check "ZOOMRIYA ID running" active "$(systemctl is-active zoomriya-id.service)"
for _ in 1 2 3 4 5 6 7 8 9 10; do
    [ "$(id_status GET /health)" = 200 ] && break
    sleep 2
done
check "ZOOMRIYA ID answers" 200 "$(id_status GET /health)"
check "ZOOMRIYA ID has no accounts" 0 "$(runuser -u postgres -- psql -qtA zoomriya_id -c 'SELECT count(*) FROM users' 2>&1)"
check "ZOOMRIYA ID has no default password" 401 \
    "$(id_status POST /auth/login '{"username": "admin", "password": "admin123"}')"
check "ZOOMRIYA ID's settings are root's alone" "600 root" "$(stat -c '%a %U' /etc/zoomriya/id.env 2>&1)"
check "ZOOMRIYA ID's folder is its own" "700 zoomriya-id" "$(stat -c '%a %U' /data/zoomriya-id 2>&1)"
}

# The first setup on the box's screen (design §5), driven here as its app
# drives it: through the agent, as a member of zoomriya-box.
ask() { setpriv --reuid=zoomriya-setup --regid=zoomriya-setup --init-groups "$agent" ask "$1" >/dev/null 2>&1; }
setup_checks() {
check "first setup on the box's screen" active "$(systemctl is-active zoomriya-setup.service)"
check "no login prompt on tty1 before setup" inactive "$(systemctl is-active getty@tty1.service)"
ask '{"op": "setup-status", "by": "test"}' && pass "setup app may ask the agent" || fail "setup app may ask the agent" "refused"
ask '{"op": "first-admin", "by": "test", "username": "amma", "password": "short"}' \
    && fail "first admin with a short password refused" "made" || pass "first admin with a short password refused"
ask '{"op": "setup-finish", "by": "test"}' && fail "setup can't finish without an admin" "finished" \
    || pass "setup can't finish without an admin"
check "no language until one is chosen" C.UTF-8 "$(. /etc/default/locale 2>/dev/null; echo "${LANG:-C.UTF-8}")"
ask '{"op": "language-set", "by": "test", "locale": "fr_CA.UTF-8"}'
check "language set" "LANG=fr_CA.UTF-8" "$(grep '^LANG=' /etc/default/locale 2>&1)"
check "only the chosen language built" "fr_CA.utf8" "$(locale -a 2>&1 | grep -v -E '^(C|C.utf8|POSIX)$' | tr '\n' ' ' | sed 's/ $//')"
check "the language kept when locales is updated" "fr_CA.UTF-8 UTF-8" "$(grep -x 'fr_CA.UTF-8 UTF-8' /etc/locale.gen 2>&1)"
ask '{"op": "region-set", "by": "test", "region": "CA"}'
check "region set" CA "$(cat /etc/zoomriya/region 2>&1)"
# Always on (design §8): nothing can put the box to sleep.
check "suspend is masked" masked "$(systemctl is-enabled suspend.target 2>&1)"
check "hibernate is masked" masked "$(systemctl is-enabled hibernate.target 2>&1)"
"$agent" ask '{"op": "status"}' | grep -q '"always_on":true' \
    && pass "status says the box is always on" || fail "status says the box is always on" "$("$agent" ask '{"op": "status"}')"
check "every script has a font" yes "$(fc-list :lang=ta | grep -q . && fc-list :lang=zh | grep -q . && echo yes)"
ask '{"op": "time-zone-set", "by": "test", "zone": "America/Toronto"}'
check "time zone set" America/Toronto "$(timedatectl show --property=Timezone --value)"
ask '{"op": "first-admin", "by": "test", "username": "amma", "password": "a long passphrase"}' \
    && pass "first admin made in ZOOMRIYA ID" || fail "first admin made in ZOOMRIYA ID" "refused"
check "the admin signs in to ZOOMRIYA ID" 200 \
    "$(id_status POST /auth/login '{"username": "amma", "password": "a long passphrase"}')"
ask '{"op": "first-admin", "by": "test", "username": "appa", "password": "another passphrase"}' \
    && fail "a second first admin refused" "made" || pass "a second first admin refused"
check "the password is in no log" 0 "$(journalctl -q -b | grep -c 'a long passphrase')"
ask '{"op": "setup-finish", "by": "test"}' && pass "setup finished" || fail "setup finished" "refused"
ask '{"op": "first-admin", "by": "test", "username": "appa", "password": "another passphrase"}' \
    && fail "no first admin after setup" "made" || pass "no first admin after setup"
systemctl stop zoomriya-setup.service
sleep 5
box_desktop_checks
}

# Box Desktop on the box's screen once it's set up (box-desktop.md), with its
# own session and sound server.
box_desktop_checks() {
check "Box Desktop on the box's screen" active "$(systemctl is-active box-desktop-app.service)"
check "no login prompt on tty1" inactive "$(systemctl is-active getty@tty1.service)"
check "Box Desktop's account may ask the agent and use the sound" "audio zoomriya-box" \
    "$(id -nG box-desktop-app | tr ' ' '\n' | grep -x -e audio -e zoomriya-box | sort | tr '\n' ' ' | sed 's/ $//')"
for _ in 1 2 3 4 5 6 7 8 9 10; do pgrep -u box-desktop-app -x wireplumber >/dev/null && break; sleep 2; done
pgrep -u box-desktop-app -x pipewire >/dev/null && pass "PipeWire in Box Desktop's session" \
    || fail "PipeWire in Box Desktop's session" "not running"
layout=$(tr '\0' '\n' < "/proc/$(pgrep -u box-desktop-app -x sway | head -1)/environ" 2>/dev/null | sed -n 's/^XKB_DEFAULT_LAYOUT=//p')
check "the box's screen has the box's keyboard layout" "$(sed -n 's/^XKBLAYOUT="\(.*\)"$/\1/p' /etc/default/keyboard)" "$layout"
# The screen as an admin keeps it in Control Panel › Display: the agent saves
# it for the box, and the screen starts that way (screen-session).
setpriv --reuid=box-desktop-app --regid=box-desktop-app --init-groups "$agent" ask \
    '{"op": "display-set", "by": "test", "output": "Virtual-1", "size": "1280x800", "refresh": 60000, "scale": 1, "transform": "normal"}' >/dev/null 2>&1
check "display kept for the box" "output Virtual-1 mode 1280x800@60.000Hz scale 1 transform normal" \
    "$(grep '^output' /etc/zoomriya/display.conf 2>&1)"
systemctl restart box-desktop-app.service
for _ in 1 2 3 4 5 6 7 8 9 10; do pgrep -u box-desktop-app -x sway >/dev/null && break; sleep 1; done
check "the screen starts as kept" "output Virtual-1 mode 1280x800@60.000Hz scale 1 transform normal" \
    "$(grep '^output' "/run/user/$(id -u box-desktop-app)/sway.conf" 2>&1)"
setpriv --reuid=box-desktop-app --regid=box-desktop-app --init-groups "$agent" ask \
    '{"op": "display-set", "by": "test", "output": "Virtual-1", "action": "forget"}' >/dev/null 2>&1
check "display forgotten" "" "$(grep '^output' /etc/zoomriya/display.conf 2>&1)"
}

first_boot() {
# os-release (design §9)
osr() { (. /usr/lib/os-release && eval echo "\$$1"); }
check "os-release NAME" "ZOOMRIYA OS" "$(osr NAME)"
check "os-release ID" zoomriya "$(osr ID)"
check "os-release ID_LIKE" debian "$(osr ID_LIKE)"
check "os-release VERSION_ID" 13 "$(osr VERSION_ID)"
debian_codename=$( (. /usr/lib/os-release.debian && echo "$VERSION_CODENAME") )
check "os-release VERSION_CODENAME is Debian's" "$debian_codename" "$(osr VERSION_CODENAME)"
[ -n "$(osr IMAGE_VERSION)" ] && pass "os-release IMAGE_VERSION" || fail "os-release IMAGE_VERSION" "missing"
check "/etc/os-release is ours" zoomriya "$( (. /etc/os-release && echo "$ID") )"

# The box's name and login banner (design §5, §9)
case "$(hostname)" in
    zoomriya-[0-9a-f][0-9a-f][0-9a-f][0-9a-f]) pass "name is zoomriya-<4 characters>" ;;
    *) fail "name is zoomriya-<4 characters>" "$(hostname)" ;;
esac
grep -q 'ZOOMRIYA Box' /etc/issue && ! grep -q Debian /etc/issue \
    && pass "login banner is ours" || fail "login banner is ours" "$(head -1 /etc/issue)"
after_login=$(cat /etc/motd; run-parts /etc/update-motd.d 2>/dev/null)
echo "$after_login" | grep -q 'ZOOMRIYA OS 13' && ! echo "$after_login" | grep -q Debian \
    && pass "message after login is ours" || fail "message after login is ours" "$(echo "$after_login" | grep -m1 Debian)"

# The disk (design §3)
check "/ is Btrfs subvolume @" "btrfs /@" "$(findmnt -no FSTYPE,FSROOT / | tr -s ' ')"
for m in /var/lib/machines:/@machines /var/lib/postgresql:/@postgres /var/lib/valkey:/@valkey \
         /data:/@data /var/log:/@log /.snapshots:/@snapshots; do
    check "$m is subvolume ${m#*:}" "${m#*:}" "$(findmnt -no FSROOT "${m%%:*}")"
done
check "EFI partition" vfat "$(lsblk -no FSTYPE /dev/disk/by-partlabel/EFI)"
[ -e /dev/disk/by-partlabel/recovery ] && pass "recovery partition" || fail "recovery partition" "missing"

# First boot (design §5): the box announces itself, and has a setup code.
check "mDNS (Avahi) running" active "$(systemctl is-active avahi-daemon.service)"
grep -qE '^[2-9A-HJKMNP-Z]{4}-[2-9A-HJKMNP-Z]{4}$' /efi/zoomriya/setup-code 2>/dev/null \
    && pass "a setup code" || fail "a setup code" "$(cat /efi/zoomriya/setup-code 2>&1)"
[ -e "$state/setup-code" ] || cp /efi/zoomriya/setup-code "$state/setup-code"

# Network and time
check "systemd-networkd running" active "$(systemctl is-active systemd-networkd.service)"
[ -n "$(ip -4 route show default)" ] && pass "default route by DHCP" || fail "default route by DHCP" "none"

# Security fixes by themselves, and nothing else (design §6.1)
origins=$(apt-config dump Unattended-Upgrade::Origins-Pattern | grep -c '"origin=')
check "one update origin" 1 "$origins"
apt-config dump Unattended-Upgrade::Origins-Pattern | grep -q 'codename=.*-security,label=Debian-Security' \
    && pass "the origin is Debian's security archive" || fail "the origin is Debian's security archive" "$(apt-config dump Unattended-Upgrade::Origins-Pattern)"
check "automatic restart off" false "$(apt-config dump Unattended-Upgrade::Automatic-Reboot | sed -n 's/.*"\(.*\)";/\1/p')"
check "daily security updates on" enabled "$(systemctl is-enabled apt-daily-upgrade.timer)"

# Restore points (design §4, §6.1)
snapper -c root list --columns description | grep -qx ' *factory *' \
    && pass "factory restore point" || fail "factory restore point" "$(snapper -c root list 2>&1 | tail -3)"
if apt-get -q update >/dev/null 2>&1 && apt-get -q install -y tree >/dev/null 2>&1; then
    check "restore points before and after apt" 2 "$(snapper -c root list --columns description | grep -cx ' *apt *')"
else
    fail "restore points before and after apt" "apt-get couldn't install from Debian (no network?)"
fi

# The agent (design §7): its socket, who may connect, and status.
check "agent's socket is root:zoomriya-box 660" "660 root zoomriya-box" "$(stat -c '%a %U %G' /run/zoomriya/agent.sock 2>&1)"
setpriv --reuid=nobody --regid=nogroup --clear-groups "$agent" ask '{"op": "status"}' >/dev/null 2>&1 \
    && fail "others can't ask the agent" "nobody got an answer" || pass "others can't ask the agent"
status=$("$agent" ask '{"op": "status", "by": "test"}')
echo "$status" | grep -q '"os":"ZOOMRIYA OS 13"' && pass "agent status" || fail "agent status" "$status"
echo "$status" | grep -q '"kind":"factory"' && pass "agent lists the factory restore point" || fail "agent lists the factory restore point" "$status"
"$agent" ask '{"op": "status", "command": "reboot"}' >/dev/null && fail "agent refuses extra arguments" "it answered ok" || pass "agent refuses extra arguments"

apt-get -q install -y curl >/dev/null 2>&1
id_checks
setup_checks

# Drives' health (design §7): the two NVMe stand-ins report SMART; the
# system's virtio disk doesn't.
drives=$("$agent" ask '{"op": "disk-health", "by": "test"}')
for serial in ZLIB0001 ZLIB0002; do
    echo "$drives" | grep -q "\"serial\":\"$serial\"[^}]*\"health\":\"healthy\"" \
        && pass "drive $serial healthy" || fail "drive $serial healthy" "$drives"
done
echo "$drives" | grep -q '"name":"vda"[^}]*"system":true[^}]*"health":"unknown"' \
    && pass "the system drive is marked, not reporting" || fail "the system drive is marked, not reporting" "$drives"
check "smartd stays off" inactive "$(systemctl is-active smartd.service)"

# The network (design §5, §7): QEMU's Ethernet carries the traffic.
net=$("$agent" ask '{"op": "network", "by": "test"}')
echo "$net" | grep -q '"online":true' && echo "$net" | grep -q '"via":"ethernet"' \
    && pass "online by Ethernet" || fail "online by Ethernet" "$net"
echo "$net" | grep -q '"ipv4":\["10\.0\.2\.15/24"\]' && echo "$net" | grep -q '"gateway":"10\.0\.2\.2"' \
    && pass "the address and the router" || fail "the address and the router" "$net"
check "both Ethernet ports listed" 2 "$(echo "$net" | grep -o '"kind":"ethernet"' | wc -l)"
echo "$net" | grep -q "\"name\":\"$(hostname)\.local\"" && pass "the box's name on the network" || fail "the box's name on the network" "$net"

# CPU and memory (design §7), for the widget.
res=$("$agent" ask '{"op": "resources", "by": "test"}')
echo "$res" | grep -qE '"cpu_percent":[0-9.]+' && echo "$res" | grep -qE '"memory_total_bytes":[1-9][0-9]{8,}' \
    && pass "CPU and memory" || fail "CPU and memory" "$res"
check "reads are audited at debug level" 7 \
    "$(journalctl -q -o export SYSLOG_IDENTIFIER=zoomriya-agent ZOOMRIYA_OP=resources | sed -n 's/^PRIORITY=//p' | tail -1)"

# Resource Monitor's readings: the ports' and drives' activity, and each
# service's use.
echo "$res" | grep -q '"network":{"in_bps":' && echo "$res" | grep -q '"disks":{"drives":\[[^]]*"name":"vda"' \
    && pass "network and drive activity" || fail "network and drive activity" "$res"
echo "$res" | grep -q '"unit":"zoomriya-agent.service"' && pass "each service's use" || fail "each service's use" "$res"

# A fixed address for the second port, kept; then a change nobody keeps,
# which undoes itself (design §5.1). QEMU's second network: 10.0.2.0/24.
second=$(grep -l 52:54:00:12:34:57 /sys/class/net/*/address | cut -d/ -f5)
fixed='{"mode": "fixed", "address": "10.0.2.50/24", "gateway": "10.0.2.2", "dns": ["10.0.2.3"]}'
check "a fixed address, from Box Desktop" ok "$(act "{\"op\": \"network-set\", \"by\": \"test-admin\", \"link\": \"$second\", ${fixed#\{}")"
for _ in $(seq 20); do
    ip -4 -o address show dev "$second" | grep -q ' 10\.0\.2\.50/24 ' && break
    sleep 1
done
ip -4 -o address show dev "$second" | grep -q ' 10\.0\.2\.50/24 ' \
    && pass "the port takes the fixed address" || fail "the port takes the fixed address" "$(ip -4 -o address show dev "$second")"
check "the change undoes itself unless kept" active "$(systemctl is-active "zoomriya-network-undo-$second.timer")"
"$agent" ask '{"op": "network", "by": "test"}' | grep -q "\"name\":\"$second\"[^}]*\"mode\":\"fixed\"" \
    && pass "the network shows the fixed address" || fail "the network shows the fixed address" "$("$agent" ask '{"op": "network"}')"
check "keep the new settings" ok "$(act '{"op": "network-keep", "by": "test-admin"}')"
check "kept: no undo waiting" inactive "$(systemctl is-active "zoomriya-network-undo-$second.timer")"
check "an address on another network is refused" refused \
    "$(act "{\"op\": \"network-set\", \"by\": \"test-admin\", \"link\": \"$second\", \"mode\": \"fixed\", \"address\": \"10.0.2.50/24\", \"gateway\": \"192.168.9.1\", \"dns\": [\"10.0.2.3\"]}")"
check "back to DHCP, not kept" ok "$(act "{\"op\": \"network-set\", \"by\": \"test-admin\", \"link\": \"$second\", \"mode\": \"dhcp\"}")"
[ ! -e "/etc/systemd/network/50-zoomriya-$second.network" ] && pass "DHCP has no file of its own" || fail "DHCP has no file of its own" "still there"
systemctl start "zoomriya-network-undo-$second.service"  # the two minutes, now
grep -q '^Address=10\.0\.2\.50/24$' "/etc/systemd/network/50-zoomriya-$second.network" 2>/dev/null \
    && pass "not kept: the fixed address is back" || fail "not kept: the fixed address is back" "$(cat "/etc/systemd/network/50-zoomriya-$second.network" 2>&1)"
check "back to DHCP, kept" ok "$(act "{\"op\": \"network-set\", \"by\": \"test-admin\", \"link\": \"$second\", \"mode\": \"dhcp\"}")"
check "kept again" ok "$(act '{"op": "network-keep", "by": "test-admin"}')"

# Log Center (design §7: logs): the admins' actions, the box's events and the
# system log, newest first.
audit=$("$agent" ask '{"op": "logs", "by": "test-admin", "kind": "audit"}')
echo "$audit" | grep -q '"op":"network-undo"' && echo "$audit" | grep -q '"detail":"'"$second"' fixed 10.0.2.50/24 router 10.0.2.2' \
    && pass "admin actions, with what changed" || fail "admin actions, with what changed" "$audit"
echo "$audit" | grep -q '"op":"resources"' && fail "reads kept out of admin actions" "$audit" || pass "reads kept out of admin actions"
events=$("$agent" ask '{"op": "logs", "by": "test-admin", "kind": "events"}')
echo "$events" | grep -q '"message":"Startup finished' \
    && pass "box events: the box started" || fail "box events: the box started" "$(echo "$events" | head -c 600)"
found=$("$agent" ask '{"op": "logs", "by": "test-admin", "kind": "system", "search": "startup FINISHED", "priority": "everything"}')
echo "$found" | grep -q '"message":"Startup finished' \
    && pass "searching the system log" || fail "searching the system log" "$(echo "$found" | head -c 600)"
"$agent" ask '{"op": "logs", "by": "test-admin", "kind": "system", "search": "zzq-nothing-like-this"}' | grep -q '"entries":\[\]' \
    && pass "a search that finds nothing" || fail "a search that finds nothing" "$("$agent" ask '{"op": "logs", "kind": "system", "search": "zzq-nothing-like-this"}')"
check "an unknown log is refused" refused "$(act '{"op": "logs", "by": "test-admin", "kind": "secrets"}')"

# The library (design §3, §7): the two NVMe stand-ins as one Btrfs filesystem,
# each a copy of the other, mounted where Photos' drive helper mounts it.
library=/data/zoomriya/volumes/test-library
set -- $(lsblk -dn -o PATH,SERIAL | awk '$2 ~ /^ZLIB/ { print $1 }')
mkfs.btrfs -q -f -L library -d raid1 -m raid1 "$1" "$2" >/dev/null 2>&1
mkdir -p "$library" && mount "$1" "$library"
dd if=/dev/urandom of="$library/photo.bin" bs=1M count=256 status=none && sync
storage=$("$agent" ask '{"op": "storage", "by": "test"}')
echo "$storage" | grep -q '"mirror":true' && pass "the library is a mirror" || fail "the library is a mirror" "$storage"
check "the library's two drives" 2 "$(echo "$storage" | grep -o '"path":"/dev/nvme[0-9]n1"' | sort -u | wc -l)"
echo "$storage" | grep -q '"never_run":true' && pass "the library's copies not checked yet" || fail "the library's copies not checked yet" "$storage"
check "check the copies, from Box Desktop" ok "$(act '{"op": "storage-check", "by": "test-admin", "action": "start"}')"
for _ in $(seq 60); do
    "$agent" ask '{"op": "storage"}' | grep -q '"finished":true' && break
    sleep 2
done
"$agent" ask '{"op": "storage"}' | grep -q '"finished":true,"never_run":false' \
    && pass "both copies match" || fail "both copies match" "$("$agent" ask '{"op": "storage"}')"
journalctl -q -b -u zoomriya-storage-check.service | grep -q . \
    && pass "the check ran in its own unit" || fail "the check ran in its own unit" "nothing in its journal"
umount "$library"

# Notifications (box-desktop.md §3): the watcher's timer, and what it wrote:
# the library's check, finished, and the network settings nobody kept.
check "the watcher's timer" enabled "$(systemctl is-enabled zoomriya-watch.timer)"
for _ in $(seq 15); do
    "$agent" ask '{"op": "notifications", "by": "test-admin"}' | grep -q '"kind":"check"' && break
    sleep 1
done
notes=$("$agent" ask '{"op": "notifications", "by": "test-admin"}')
echo "$notes" | grep -q '"kind":"check","level":"info","message":"The library'"'"'s check finished: both copies match."' \
    && pass "told when the library's check finished" || fail "told when the library's check finished" "$notes"
echo "$notes" | grep -q '"kind":"network"' && pass "told when network settings weren't kept" || fail "told when network settings weren't kept" "$notes"
"$agent" watch >/dev/null 2>&1; check "the watcher keeps what it saw" 600 "$(stat -c '%a' /var/lib/zoomriya-agent/watch.json 2>&1)"

# Security & Privacy (design §8), read only.
sec=$("$agent" ask '{"op": "security", "by": "test"}')
echo "$sec" | grep -q '"firewall":{"on":true,"open":\[' && echo "$sec" | grep -q '"port":443,"protocol":"tcp"' && echo "$sec" | grep -q '"port":5353,"protocol":"udp"' \
    && pass "what the firewall lets in" || fail "what the firewall lets in" "$sec"
echo "$sec" | grep -q '"apparmor":{"enforced":[1-9][0-9]*,"on":true}' && pass "AppArmor on, with profiles enforced" || fail "AppArmor on, with profiles enforced" "$sec"
echo "$sec" | grep -qE '"secure_boot":"(on|off)"' && pass "Secure Boot's state" || fail "Secure Boot's state" "$sec"
echo "$sec" | grep -q '"host":"deb.debian.org"' && pass "where the box connects to" || fail "where the box connects to" "$sec"
echo "$sec" | grep -q '"automatic":true' && pass "security updates automatic" || fail "security updates automatic" "$sec"
dpkg -s popularity-contest >/dev/null 2>&1 && fail "nothing reports usage" "popularity-contest is installed" || pass "nothing reports usage"

# The keyboard layout of the box's own screen, Debian's way.
check "keyboard layout to French, from Box Desktop" ok "$(act '{"op": "keyboard-set", "by": "test-admin", "layout": "fr"}')"
grep -q '^XKBLAYOUT="fr"$' /etc/default/keyboard && pass "kept in /etc/default/keyboard" || fail "kept in /etc/default/keyboard" "$(cat /etc/default/keyboard)"
check "a layout the box doesn't offer" refused "$(act '{"op": "keyboard-set", "by": "test-admin", "layout": "klingon"}')"
check "keyboard back to English (US)" ok "$(act '{"op": "keyboard-set", "by": "test-admin", "layout": "us"}')"

# Found on the home network as the phone app finds it.
apt-get -q install -y avahi-utils >/dev/null 2>&1
avahi-browse -rpt _zoomriya._tcp 2>/dev/null | grep -q "^=;.*;_zoomriya\._tcp;local;$(hostname)\.local;.*;443;" \
    && pass "announced as _zoomriya._tcp at $(hostname).local" || fail "announced as _zoomriya._tcp" "$(avahi-browse -rpt _zoomriya._tcp 2>&1 | head -3)"

# A restore point before the kernel update, to restore in the second boot,
# and a change to the OS after it, which the restore must undo.
snapper -c root create -p --description "test restore point" > "$state/restore-point"
uname -r > "$state/old-kernel"
touch /etc/zoomriya-test-after-restore-point

# A kernel update, through apt as unattended-upgrades does it: dpkg, the
# initrd, then systemd-boot's hook runs kernel-install, which gives the new
# entry three tries (design §4). Debian's small cloud kernel stands in for a
# newer kernel; the next boot starts from it once.
check "kernel command line for new kernels" "root=PARTLABEL=system rootflags=subvol=@ rw panic=10" "$(cat /etc/kernel/cmdline)"
apt-get -q install -y linux-image-cloud-amd64 >/dev/null 2>&1 || fail "kernel installs through apt" "apt-get failed"
entry=$(ls "$esp"/loader/entries/*-cloud-amd64+3.conf 2>/dev/null | head -1)
if [ -n "$entry" ]; then
    pass "new kernel's entry has three tries"
    grep -q '^initrd ' "$entry" && pass "new kernel's entry has an initrd" || fail "new kernel's entry has an initrd" "$(cat "$entry")"
    id=$(basename "$entry"); id="${id%+3.conf}.conf"
    echo "$id" > "$state/entry"
    sed -n 's/^version *//p' "$entry" > "$state/kernel"
    bootctl set-oneshot "$id"
else
    fail "new kernel's entry has three tries" "$(ls "$esp"/loader/entries/)"
fi

# The maintenance hour (design §6.1), moved in Box Desktop.
check "maintenance hour is 03:30 to start with" 03:30 "$("$agent" ask '{"op": "maintenance-hour"}' | sed -n 's/.*"maintenance_hour":"\([0-9:]*\)".*/\1/p')"
check "moving the maintenance hour" ok "$(act '{"op": "maintenance-hour", "by": "test-admin", "hour": "02:00"}')"
systemctl show -p TimersCalendar zoomriya-maintenance.timer | grep -q '02:00:00' \
    && pass "the maintenance timer moved" || fail "the maintenance timer moved" "$(systemctl show -p TimersCalendar zoomriya-maintenance.timer)"
check "an hour that isn't one is refused" refused "$(act '{"op": "maintenance-hour", "by": "test-admin", "hour": "25:00"}')"

# The hardening list (design §8), now that everything has run once.
. "$CREDENTIALS_DIRECTORY/zoomriya-hardening.sh"

# Lynis's view of general hardening drift beyond our list (design §8):
# reported, not a pass or fail.
if apt-get -q install -y lynis >/dev/null 2>&1; then
    lynis audit system --quick --no-colors --quiet >/dev/null 2>&1
    echo "INFO Lynis hardening index: $(sed -n 's/^hardening_index=//p' /var/log/lynis-report.dat)"
fi
}

second_boot() {
box_desktop_checks
journalctl -q -b -1 -o cat SYSLOG_IDENTIFIER=zoomriya-agent ZOOMRIYA_OP=restart ZOOMRIYA_BY=test-admin | grep -q ': done' \
    && pass "restarted by Restart now, in the audit log" || fail "restarted by Restart now, in the audit log" "no entry"
id=$(cat "$state/entry" 2>/dev/null)
current=$(bootctl status 2>/dev/null | sed -n 's/^ *Current Entry: //p')
check "started from the new kernel's entry" "$id" "$current"
check "running the new kernel" "$(cat "$state/kernel" 2>/dev/null)" "$(uname -r)"
check "the box came up before the kernel was marked good" active "$(systemctl is-active zoomriya-boot-check.service)"
systemctl start systemd-bless-boot.service >/dev/null 2>&1
[ -e "$esp/loader/entries/$id" ] && pass "new kernel's entry marked good" || fail "new kernel's entry marked good" "$(ls "$esp"/loader/entries/)"

# Restore, as an admin does it in Box Desktop, to the point before the kernel
# update; /data must not change. Box Desktop asks the agent.
touch "$testdir/kept"
point=$(cat "$state/restore-point")
check "restore through Box Desktop" ok \
    "$(act "{\"op\": \"restore-point-restore\", \"by\": \"test-admin\", \"number\": $point}")"
"$agent" ask '{"op": "status"}' | grep -q "\"restart_reasons\":\[\"restore point $point\"\]" \
    && pass "status says the restore needs a restart" || fail "status says the restore needs a restart" "$("$agent" ask '{"op": "status"}')"
ausearch -k zoomriya-agent -f restore-snapshot >/dev/null 2>&1 \
    && pass "auditd recorded the restore" || fail "auditd recorded the restore" "no record"
journalctl -q -o cat SYSLOG_IDENTIFIER=zoomriya-agent ZOOMRIYA_OP=restore-point-restore ZOOMRIYA_BY=test-admin | grep -q "test-admin asked for restore-point-restore $point: done" \
    && pass "the restore is in the audit log" || fail "the restore is in the audit log" "$(journalctl -q -o cat SYSLOG_IDENTIFIER=zoomriya-agent | tail -3)"
}

third_boot() {
journalctl -q -b -1 -o cat SYSLOG_IDENTIFIER=zoomriya-agent ZOOMRIYA_OP=maintenance | grep -q 'restarting for restore point' \
    && pass "the maintenance hour restarted for the restore" || fail "the maintenance hour restarted for the restore" "$(journalctl -q -b -1 -o cat SYSLOG_IDENTIFIER=zoomriya-agent | tail -3)"
[ ! -e /etc/zoomriya-test-after-restore-point ] && pass "OS restored" || fail "OS restored" "the change after the restore point is still there"
[ -e "$testdir/kept" ] && pass "/data untouched by restore" || fail "/data untouched by restore" "$testdir/kept is gone"
check "/ is Btrfs subvolume @ after restore" "btrfs /@" "$(findmnt -no FSTYPE,FSROOT / | tr -s ' ')"
btrfs subvolume list / | grep -q '@\.replaced-' && fail "replaced OS removed" "$(btrfs subvolume list / | grep '@\.replaced-')" || pass "replaced OS removed"
snapper -c root list --columns description | grep -q 'Before restoring' \
    && pass "restore can be undone" || fail "restore can be undone" "no 'Before restoring' restore point"
/usr/lib/zoomriya-agent/zoomriya-agent ask '{"op": "status"}' | grep -q '"restart_needed":false' \
    && pass "no restart needed after it" || fail "no restart needed after it" "$(/usr/lib/zoomriya-agent/zoomriya-agent ask '{"op": "status"}')"
check "running the kernel the restored OS has" "$(cat "$state/old-kernel")" "$(uname -r)"
ls "$esp"/loader/entries/ | grep -q cloud-amd64 \
    && fail "the removed kernel's entry is gone" "$(ls "$esp"/loader/entries/)" || pass "the removed kernel's entry is gone"

# A newer kernel that can't start (design §4): the same kernel with Debian's
# initrd, looking for a root partition that isn't there, with two tries.
# panic=5 restarts it each time; then systemd-boot should start the good one.
kver=$(uname -r)
uname -r > "$state/good-kernel"
good=$(ls "$esp"/loader/entries/ | head -1)
mkdir -p "$esp/zoomriya-broken"
cp "/boot/vmlinuz-$kver" "$esp/zoomriya-broken/linux"
# mkinitramfs, not update-initramfs: it runs no hooks, so no real entry is added.
if mkinitramfs -o "$esp/zoomriya-broken/initrd" "$kver" >/dev/null 2>&1; then
    {
        echo "title ZOOMRIYA OS (a kernel that can't start)"
        grep '^sort-key ' "$esp/loader/entries/$good"
        echo "version 99"
        echo "linux /zoomriya-broken/linux"
        echo "initrd /zoomriya-broken/initrd"
        echo "options root=PARTLABEL=not-here rw panic=5 rootdelay=5"
    } > "$esp/loader/entries/zoomriya-99-broken+2.conf"
    sync
    pass "a kernel that can't start, with two tries"
else
    fail "a kernel that can't start, with two tries" "mkinitramfs failed"
fi
}

fourth_boot() {
check "back on the good kernel by itself" "$(cat "$state/good-kernel")" "$(uname -r)"
check "the broken kernel used up its tries" zoomriya-99-broken+0-2.conf \
    "$(ls "$esp"/loader/entries/ | grep broken)"
current=$(bootctl status 2>/dev/null | sed -n 's/^ *Current Entry: //p')
case "$current" in
    *broken*) fail "started from the good entry" "$current" ;;
    *) pass "started from the good entry" ;;
esac

# Erase everything (design §4), as an admin does it in Box Desktop: the
# password again, and the setup code from the sticker.
code=$(cat /efi/zoomriya/setup-code)
check "erase with the wrong setup code is refused" refused \
    "$(act '{"op": "factory-reset", "by": "test-admin", "level": "everything", "setup_code": "AAAA-AAAA"}')"
check "erase everything in Box Desktop" ok \
    "$(act "{\"op\": \"factory-reset\", \"by\": \"test-admin\", \"level\": \"everything\", \"setup_code\": \"$code\"}")"
"$agent" ask '{"op": "status"}' | grep -q '"restart_reasons":\["erase everything"\]' \
    && pass "status says the erase waits for a restart" || fail "status says the erase waits for a restart" "$("$agent" ask '{"op": "status"}')"
}

fifth_boot() {
# What comes back by itself: ZOOMRIYA ID, and Box Desktop's empty admin folder.
check "/data erased" "" "$(ls -A /data | grep -vx 'zoomriya-id\|zoomriya' | tr '\n' ' ')$(ls -A /data/zoomriya | grep -vx admin | tr '\n' ' ')"
id_checks
setup_checks
check "Box Desktop starts again, with no admin's desktop" "" "$(ls -A /data/zoomriya/admin 2>/dev/null)"
journalctl -q -b -1 >/dev/null 2>&1 && fail "logs erased" "an earlier boot's journal is still there" || pass "logs erased"
[ -d "/var/log/journal/$(cat /etc/machine-id)" ] && pass "journal is persistent after the erase" \
    || fail "journal is persistent after the erase" "$(ls -la /var/log/journal 2>&1 | head -3)"
check "auditd running after the erase" active "$(systemctl is-active auditd.service)"
check "only the factory restore point is left" factory \
    "$(snapper -c root list --columns description | tail -n +3 | sed 's/^ *//; s/ *$//' | grep -v '^current$' | tr '\n' ' ' | sed 's/ $//')"
check "the setup code stays" "$(cat "$state/setup-code")" "$(cat /efi/zoomriya/setup-code)"
command -v curl >/dev/null && fail "the OS as it left the factory" "curl, installed by the test, is still there" \
    || pass "the OS as it left the factory"
check "no erase left waiting" "" "$(ls /var/lib/zoomriya-base/erase-pending 2>/dev/null)"
}

# The restarts between boots are the box's own: Restart now in Box Desktop
# after the first, the maintenance hour's job after the second (a restore waits
# for it). If either doesn't restart the box within a minute, that's a failure.
case "$phase" in
    1) first_boot; echo 2 > "$state/phase"; sync
       check "Restart now in Box Desktop" ok "$(act '{"op": "restart", "by": "test-admin"}')"
       sleep 60; fail "Restart now in Box Desktop" "the box didn't restart"; systemctl reboot --no-block ;;
    2) second_boot; echo 3 > "$state/phase"; sync
       systemctl start --no-block zoomriya-maintenance.service
       sleep 60; fail "the maintenance hour restarts for a restore" "the box didn't restart"; systemctl reboot --no-block ;;
    3) third_boot; echo 4 > "$state/phase"; sync; systemctl reboot --no-block ;;
    4) fourth_boot; echo 5 > "$state/phase"; sync
       "$agent" ask '{"op": "restart", "by": "test"}' >/dev/null
       sleep 60; fail "restart for the erase" "the box didn't restart"; systemctl reboot --no-block ;;
    *) fifth_boot
       echo ZOOMRIYA-CHECK-DONE
       sync; systemctl poweroff --no-block ;;
esac
