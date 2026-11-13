# The hardening list (design §8), one check per item, run inside the booted
# image by tests/vm/check.sh, which gives it pass, fail and check. They check
# what is actually running, not only the settings files.

# Separate, locked-down mounts; /tmp in memory
case ",$(findmnt -no OPTIONS /var/log)," in
    *,nodev,*nosuid*noexec*|*nosuid*nodev*noexec*) pass "/var/log is nodev,nosuid,noexec" ;;
    *) fail "/var/log is nodev,nosuid,noexec" "$(findmnt -no OPTIONS /var/log)" ;;
esac
check "/tmp is tmpfs" tmpfs "$(findmnt -no FSTYPE /tmp)"

# No default password (design §5): root's is locked or absent.
case "$( (grep '^root:' /etc/shadow || grep '^root:' /etc/passwd) | cut -d: -f2)" in
    \!*|\*) pass "root has no password" ;;
    *) fail "root has no password" "root's password field is set" ;;
esac

# Firewall: inbound denied but for what's listed; no forwarding
check "nftables running" active "$(systemctl is-active nftables.service)"
nft list chain inet zoomriya input 2>/dev/null | grep -q 'policy drop' \
    && pass "inbound denied by default" || fail "inbound denied by default" "no drop policy"
nft list chain inet zoomriya forward 2>/dev/null | grep -q 'policy drop' \
    && pass "forwarding denied" || fail "forwarding denied" "no drop policy"
# Outbound connections are logged, by account, for Control Panel (design §8)
nft list chain inet zoomriya output 2>/dev/null | grep -q 'log prefix "zoomriya-out: " flags skuid' \
    && pass "outbound connections logged" || fail "outbound connections logged" "no log rule"

# Nothing listening beyond the loopback; no SSH (design §7.1). Avahi's own UDP
# sockets (mDNS, and random ports for one-off queries) are fine: the firewall
# lets only 5353 in.
open=$(ss -Hltnup | grep -v '"avahi-daemon"' | awk '{print $5}' | grep -v -E '^(127\.|\[::1\]|\[::ffff:127\.)' | grep -v -E ':(5353|68|546)$' | tr '\n' ' ')
check "no open ports" "" "$open"

# SSH (design §7.1): installed for advanced access, but off and shut out until
# an admin turns it on; key-only as admin; no host key from the image.
check "sshd not running" inactive "$(systemctl is-active ssh.service)"
case "$(systemctl is-enabled ssh.service ssh.socket 2>/dev/null | tr '\n' ' ')" in
    *enabled*) fail "sshd not started at boot" "ssh.service or ssh.socket is enabled" ;;
    *) pass "sshd not started at boot" ;;
esac
# Internet access (design §8.2): app accounts (ZOOMRIYA ID's, on every box)
# can't reach the internet unless they declare it; what's stopped is logged.
nft list chain inet zoomriya outbound_policy 2>/dev/null | grep -q 'meta skuid .* drop' \
    && pass "apps are blocked from the internet by default" \
    || fail "apps are blocked from the internet by default" "$(nft list chain inet zoomriya outbound_policy 2>&1)"
nft list chain inet zoomriya output 2>/dev/null | grep -q 'jump outbound_policy' \
    && pass "outbound traffic goes through the internet access rules" \
    || fail "outbound traffic goes through the internet access rules" "no jump to outbound_policy"
nft list chain inet zoomriya remote_access 2>/dev/null | grep -q 'dport 22' \
    && fail "SSH shut out by the firewall" "remote_access lets port 22 in" || pass "SSH shut out by the firewall"
# After the VM test turned advanced access on once, so the box has its host key.
# sshd -T needs its privilege separation folder, which systemd makes only while
# SSH runs (advanced access is off by now).
mkdir -p /run/sshd
sshd_t=$(sshd -T 2>&1)
echo "$sshd_t" | grep -q '^port ' || fail "sshd -T reads the settings" "$(echo "$sshd_t" | head -3)"
for kv in "passwordauthentication no" "kbdinteractiveauthentication no" "permitrootlogin no" \
          "authenticationmethods publickey" "allowusers admin" "authorizedkeysfile /data/zoomriya-os/ssh/authorized_keys" \
          "x11forwarding no" "allowtcpforwarding no" "permittunnel no"; do
    echo "$sshd_t" | grep -qx "$kv" && pass "sshd: $kv" || fail "sshd: $kv" "$(echo "$sshd_t" | grep "^${kv%% *} ")"
done
check "admin has no password" "" "$(getent shadow admin | cut -d: -f2 | grep -v -E '^(!|\*)')"
check "sudoers for admin is root's, 0440" "0440 root" "$(stat -c '%a %U' /etc/sudoers.d/zoomriya-admin | sed 's/^/0/')"

# sysctl
for kv in net.ipv4.ip_forward=0 net.ipv6.conf.all.forwarding=0 net.ipv4.conf.all.rp_filter=2 \
          net.ipv4.conf.all.accept_redirects=0 net.ipv4.conf.all.send_redirects=0 \
          net.ipv6.conf.all.accept_redirects=0 kernel.randomize_va_space=2 fs.suid_dumpable=0; do
    key=${kv%%=*}
    check "sysctl $key" "${kv#*=}" "$(cat "/proc/sys/$(echo "$key" | tr . /)")"
done

# Logs kept, on @log
[ -d "/var/log/journal/$(cat /etc/machine-id)" ] && pass "journal is persistent" || fail "journal is persistent" "no /var/log/journal"

# auditd: running, with our rules
check "auditd running" active "$(systemctl is-active auditd.service)"
for key in accounts sudo time modules zoomriya-agent; do
    auditctl -l 2>/dev/null | grep -qE "(-k |key=)$key\$" && pass "audit rules: $key" || fail "audit rules: $key" "not loaded"
done

# Unused kernel modules off
for module in cramfs freevxfs hfs hfsplus udf dccp sctp rds tipc; do
    if modprobe -q "$module" 2>/dev/null || grep -q "^$module " /proc/modules; then
        fail "module $module can't load" "it loaded"
    else
        pass "module $module can't load"
    fi
done

# AppArmor
check "AppArmor on" Y "$(cat /sys/module/apparmor/parameters/enabled 2>/dev/null)"

# Time: certificates and update signatures depend on it
check "systemd-timesyncd running" active "$(systemctl is-active systemd-timesyncd.service)"

# File integrity (AIDE): a baseline from the first start, a nightly check, and
# a changed file is noticed. First the baseline is brought up to date, since
# this test's own apt runs changed /usr.
[ -s /var/lib/zoomriya-aide/aide.db.gz ] && pass "AIDE baseline made at first start" || fail "AIDE baseline made at first start" "no database"
check "AIDE nightly check on" enabled "$(systemctl is-enabled zoomriya-aide-check.timer 2>&1)"
[ ! -e /usr/lib/systemd/system/dailyaidecheck.timer ] && pass "only our AIDE check" || fail "only our AIDE check" "Debian's dailyaidecheck is installed"
while systemctl is-active --quiet zoomriya-aide-update.service; do sleep 2; done
/usr/lib/zoomriya-base/aide-baseline >/dev/null
echo probe > /etc/zoomriya-aide-probe
/usr/lib/zoomriya-base/aide-check | grep -q '/etc/zoomriya-aide-probe' \
    && pass "AIDE notices a file added to /etc" || fail "AIDE notices a file added to /etc" "$(/usr/lib/zoomriya-base/aide-check | head -5)"
rm -f /etc/zoomriya-aide-probe
/usr/lib/zoomriya-base/aide-baseline >/dev/null
aide_report=$(/usr/lib/zoomriya-base/aide-check)
echo "$aide_report" | grep -q 'as the last update left them' \
    && pass "AIDE clean after the baseline is refreshed" \
    || fail "AIDE clean after the baseline is refreshed" "$(echo "$aide_report" | grep -v -e '^<4>-*$' -e '^<4>$' | head -8 | tr '\n' ' ')"
