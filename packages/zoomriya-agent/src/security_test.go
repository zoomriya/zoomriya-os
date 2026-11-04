package main

import (
	"strconv"
	"strings"
	"testing"
)

const nftInput = `table inet zoomriya {
	chain input {
		type filter hook input priority filter; policy drop;
		iif "lo" accept
		ct state established,related accept
		udp sport 67 udp dport 68 accept
		ip6 saddr fe80::/10 udp sport 547 udp dport 546 accept
		udp dport 5353 accept
		tcp dport 443 accept
		tcp dport { 22, 8443 } accept
	}
}`

func TestSecurity(t *testing.T) {
	agent, box := newFakeBox(t)
	box.answers = map[string]string{
		"nft list chain inet zoomriya input":                              nftInput,
		"systemctl is-enabled apt-daily-upgrade.timer":                    "enabled\n",
		"systemctl is-active nftables.service":                            "active\n",
		"timedatectl show-timesync --property=ServerName --value":         "\n",
		"timedatectl show-timesync --property=FallbackNTPServers --value": "0.debian.pool.ntp.org 1.debian.pool.ntp.org\n",
	}
	box.journal = `{"MESSAGE":"/etc/cron.d/extra","_SYSTEMD_INVOCATION_ID":"b","PRIORITY":"4"}
{"MESSAGE":"Added entries:","_SYSTEMD_INVOCATION_ID":"b","PRIORITY":"4"}
{"MESSAGE":"Files in the OS changed outside an update:","_SYSTEMD_INVOCATION_ID":"b","__REALTIME_TIMESTAMP":"1793700000000000"}
{"MESSAGE":"The OS's files are as the last update left them.","_SYSTEMD_INVOCATION_ID":"a"}
`
	box.write("sys/module/apparmor/parameters/enabled", "Y\n")
	box.write("sys/kernel/security/apparmor/profiles", "box-desktop (enforce)\nnvidia_modprobe (enforce)\nlsb_release (complain)\n")
	box.write("sys/firmware/efi/efivars/SecureBoot-8be4df61-93ca-11d2-aa0d-e0e8e3e0d4c3", "\x06\x00\x00\x00\x01")
	box.write("etc/apt/sources.list.d/debian.sources", "Types: deb\nURIs: http://deb.debian.org/debian\nSuites: trixie\n\nTypes: deb\nURIs: http://security.debian.org/debian-security\n")

	resp, err := agent.Handle(Request{Op: "security"})
	if err != nil {
		t.Fatal(err)
	}
	if u := resp["updates"].(Response); u["automatic"] != true {
		t.Errorf("updates = %v", u)
	}
	fw := resp["firewall"].(Response)
	var ports []string
	for _, p := range fw["open"].([]OpenPort) {
		ports = append(ports, p.Protocol+" "+strconv.Itoa(p.Port))
	}
	if fw["on"] != true || strings.Join(ports, ",") != "tcp 22,udp 68,tcp 443,udp 546,udp 5353,tcp 8443" {
		t.Errorf("firewall = %v %v", fw["on"], ports)
	}
	in := resp["integrity"].(Response)
	if in["clean"] != false || in["checked"] != "2026-11-03T10:00:00Z" || strings.Join(in["changes"].([]string), "|") != "Added entries:|/etc/cron.d/extra" {
		t.Errorf("integrity = %v", in)
	}
	if aa := resp["apparmor"].(Response); aa["on"] != true || aa["enforced"] != 2 {
		t.Errorf("apparmor = %v", aa)
	}
	if resp["secure_boot"] != "on" {
		t.Errorf("secure boot = %v", resp["secure_boot"])
	}
	var hosts []string
	for _, c := range resp["connections"].([]Connection) {
		hosts = append(hosts, c.Host)
	}
	if strings.Join(hosts, ",") != "deb.debian.org,security.debian.org,0.debian.pool.ntp.org,1.debian.pool.ntp.org" {
		t.Errorf("connections = %v", hosts)
	}
}

func TestSecureBootWithoutUEFI(t *testing.T) {
	agent, _ := newFakeBox(t)
	if got := agent.secureBoot(); got != "unavailable" {
		t.Errorf("secure boot = %s", got)
	}
}
