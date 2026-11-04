// security (design §7, §8): Control Panel's Security & Privacy, read only:
// the security updates, what the firewall lets in, the last file integrity
// check, AppArmor, Secure Boot, and every place the box connects to on the
// internet.
package main

import (
	"bytes"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// What each port the firewall opens is for (zoomriya-base's nftables.conf,
// and the agent's own exceptions while they're on).
var openPorts = map[string]string{
	"tcp 443":  "The box's web address: Box Desktop, Family Hub and the apps",
	"udp 5353": "Being found on the home network (mDNS)",
	"udp 68":   "Getting an address from the router (DHCP)",
	"udp 546":  "Getting an IPv6 address from the router (DHCPv6)",
	"tcp 22":   "Advanced access or support mode (SSH)",
}

var dportPattern = regexp.MustCompile(`\b(tcp|udp) (?:sport \d+ (?:tcp|udp) )?dport (\{[^}]*\}|\d+)[^\n]*\baccept\b`)

type OpenPort struct {
	Port     int    `json:"port"`
	Protocol string `json:"protocol"`
	What     string `json:"what"`
}

type Connection struct {
	Host string `json:"host"`
	What string `json:"what"`
}

// The EFI variable that says whether Secure Boot is on.
const secureBootVar = "sys/firmware/efi/efivars/SecureBoot-8be4df61-93ca-11d2-aa0d-e0e8e3e0d4c3"

func (a *Agent) security() (Response, error) {
	enabled := func(unit string) bool {
		out, _ := a.Run("systemctl", "is-enabled", unit)
		return strings.TrimSpace(string(out)) == "enabled"
	}
	active := func(unit string) bool {
		out, _ := a.Run("systemctl", "is-active", unit)
		return strings.TrimSpace(string(out)) == "active"
	}

	return Response{
		"updates": Response{
			"automatic":      enabled("apt-daily-upgrade.timer"),
			"last_installed": a.modified("var/log/unattended-upgrades/unattended-upgrades-dpkg.log"),
			"last_check":     a.modified("var/lib/apt/periodic/unattended-upgrades-stamp"),
		},
		"firewall": Response{
			"on":   active("nftables.service"),
			"open": a.firewallOpen(),
		},
		"integrity":   a.integrity(),
		"apparmor":    a.apparmor(),
		"secure_boot": a.secureBoot(),
		"connections": a.connections(),
	}, nil
}

// firewallOpen reads the input chain's accept rules by port.
func (a *Agent) firewallOpen() []OpenPort {
	open := []OpenPort{}
	out, err := a.Run("nft", "list", "chain", "inet", "zoomriya", "input")
	if err != nil {
		return open
	}
	seen := map[string]bool{}
	for _, m := range dportPattern.FindAllStringSubmatch(string(out), -1) {
		for _, p := range strings.FieldsFunc(m[2], func(r rune) bool { return r == '{' || r == '}' || r == ',' || r == ' ' }) {
			port, err := strconv.Atoi(p)
			key := m[1] + " " + p
			if err != nil || seen[key] {
				continue
			}
			seen[key] = true
			what := openPorts[key]
			if what == "" {
				what = "Opened by an app"
			}
			open = append(open, OpenPort{Port: port, Protocol: m[1], What: what})
		}
	}
	sort.Slice(open, func(i, j int) bool { return open[i].Port < open[j].Port })
	return open
}

// integrity is the last file integrity check: when, and what changed outside
// an update (zoomriya-base's aide-check writes it to the journal).
func (a *Agent) integrity() Response {
	result := Response{"checked": nil, "clean": nil, "changes": []string{}}
	out, err := a.Run("journalctl", "--output=json", "--no-pager", "--quiet", "--reverse", "--lines=200",
		"_SYSTEMD_UNIT=zoomriya-aide-check.service")
	if err != nil {
		return result
	}
	invocation := ""
	var newer []map[string]json.RawMessage
	for _, line := range bytes.Split(out, []byte("\n")) {
		var fields map[string]json.RawMessage
		if json.Unmarshal(line, &fields) != nil {
			continue
		}
		message := journalField(fields["MESSAGE"])
		switch {
		case strings.HasPrefix(message, "The OS's files are as"):
			result["clean"] = true
		case strings.HasPrefix(message, "Files in the OS changed outside an update"):
			result["clean"] = false
		default:
			newer = append(newer, fields)
			continue
		}
		invocation = journalField(fields["_SYSTEMD_INVOCATION_ID"])
		if us, err := strconv.ParseInt(journalField(fields["__REALTIME_TIMESTAMP"]), 10, 64); err == nil {
			result["checked"] = time.UnixMicro(us).UTC().Format(time.RFC3339)
		}
		break
	}
	if result["clean"] == false {
		changes := []string{}
		for i := len(newer) - 1; i >= 0; i-- { // oldest first, as AIDE listed them
			if journalField(newer[i]["_SYSTEMD_INVOCATION_ID"]) == invocation {
				if line := strings.TrimSpace(journalField(newer[i]["MESSAGE"])); line != "" {
					changes = append(changes, line)
				}
			}
		}
		if len(changes) > 50 {
			changes = changes[:50]
		}
		result["changes"] = changes
	}
	return result
}

// apparmor: on, and how many profiles it enforces.
func (a *Agent) apparmor() Response {
	on := a.readTrimmed("sys/module/apparmor/parameters/enabled") == "Y"
	enforced := 0
	for _, line := range strings.Split(a.readTrimmed("sys/kernel/security/apparmor/profiles"), "\n") {
		if strings.HasSuffix(line, "(enforce)") {
			enforced++
		}
	}
	return Response{"on": on, "enforced": enforced}
}

// secureBoot: "on", "off", or "unavailable" when the box didn't start by
// UEFI.
func (a *Agent) secureBoot() string {
	if _, err := os.Stat(a.path("sys/firmware/efi")); err != nil {
		return "unavailable"
	}
	data, err := os.ReadFile(a.path(secureBootVar))
	// Four bytes of attributes, then the value.
	if err != nil || len(data) < 5 {
		return "off"
	}
	if data[4] == 1 {
		return "on"
	}
	return "off"
}

// connections are the places on the internet ZOOMRIYA OS itself connects to:
// apt's sources and the time servers.
func (a *Agent) connections() []Connection {
	hosts := map[string]string{}
	add := func(raw, what string) {
		if u, err := url.Parse(raw); err == nil && u.Hostname() != "" {
			hosts[u.Hostname()] = what
		}
	}
	files, _ := filepath.Glob(a.path("etc/apt/sources.list.d/*.sources"))
	for _, f := range files {
		data, _ := os.ReadFile(f)
		for _, line := range strings.Split(string(data), "\n") {
			if rest, found := strings.CutPrefix(strings.TrimSpace(line), "URIs:"); found {
				for _, uri := range strings.Fields(rest) {
					add(uri, "The OS's packages and their security fixes")
				}
			}
		}
	}
	for _, line := range strings.Split(a.readTrimmed("etc/apt/sources.list"), "\n") {
		if f := strings.Fields(line); len(f) > 1 && f[0] == "deb" {
			add(f[1], "The OS's packages and their security fixes")
		}
	}

	out, _ := a.Run("timedatectl", "show-timesync", "--property=ServerName", "--value")
	servers := strings.Fields(string(out))
	if len(servers) == 0 {
		out, _ = a.Run("timedatectl", "show-timesync", "--property=FallbackNTPServers", "--value")
		servers = strings.Fields(string(out))
	}
	for _, s := range servers {
		hosts[s] = "The time, which certificates and updates depend on"
	}

	list := []Connection{}
	for host, what := range hosts {
		list = append(list, Connection{Host: host, What: what})
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].What != list[j].What {
			return list[i].What < list[j].What
		}
		return list[i].Host < list[j].Host
	})
	return list
}
