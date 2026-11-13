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
// and the agent's own exceptions while they're on): Box Desktop's words, the
// protocol's name and the icon Box Desktop shows.
var openPorts = map[string]struct{ What, Detail, Icon string }{
	"tcp 443":  {"Local Web Access (e.g., Family Hub, apps)", "Secure Web", "web"},
	"tcp 80":   {"Web, home network only (sends browsers to the secure address)", "Web", "web"},
	"tcp 7443": {"Box Desktop, home network only", "Secure Web", "web"},
	"udp 5353": {"Network Discovery (mDNS)", "Multicast DNS", "discovery"},
	"udp 68":   {"Network Address Configuration (DHCP)", "Client DHCP", "address"},
	"udp 546":  {"Network Address Configuration (DHCPv6)", "Client DHCPv6", "address"},
	"tcp 22":   {"Advanced Access (SSH)", "Secure Shell", "terminal"},
}

var dportPattern = regexp.MustCompile(`\b(tcp|udp) (?:sport \d+ (?:tcp|udp) )?dport (\{[^}]*\}|\d+)[^\n]*\baccept\b`)

type OpenPort struct {
	Port     int    `json:"port"`
	Protocol string `json:"protocol"`
	What     string `json:"what"`
	Detail   string `json:"detail"`
	Icon     string `json:"icon"`
}

// ourSource is our own apt repository's source (design §6.8), outside
// sources.list.d: apt never reads it by itself, only for an admin's Update.
const ourSource = "etc/apt/zoomriya-updates/zoomriya.sources"

// sourceURIs are the URIs of a .sources file's sources that are on: one per
// paragraph, and one with Enabled: no isn't used.
func sourceURIs(file string) []string {
	data, _ := os.ReadFile(file)
	var all []string
	for _, stanza := range strings.Split(string(data), "\n\n") {
		var uris []string
		on := true
		for _, line := range strings.Split(stanza, "\n") {
			key, value, _ := strings.Cut(strings.TrimSpace(line), ":")
			switch key {
			case "URIs":
				uris = strings.Fields(value)
			case "Enabled":
				on = strings.TrimSpace(value) != "no"
			}
		}
		if on {
			all = append(all, uris...)
		}
	}
	return all
}

type Connection struct {
	Host  string `json:"host"`
	Title string `json:"title"`
	What  string `json:"what"`
	When  string `json:"when"`
	Icon  string `json:"icon"`
}

// securityNow is the security fixes an admin starts with Check now.
const securityNow = "zoomriya-security-now.service"

// securityCheckNow starts the nightly security fixes now (design §6.1). It
// returns at once; security's running says when they're done.
func (a *Agent) securityCheckNow() (Response, error) {
	for _, unit := range []string{securityNow, "apt-daily-upgrade.service"} {
		if out, _ := a.Run("systemctl", "is-active", unit); strings.TrimSpace(string(out)) == "active" {
			return nil, refuse("The box is already installing security fixes.")
		}
	}
	if _, err := a.Run("systemctl", "start", "--no-block", securityNow); err != nil {
		return nil, err
	}
	return Response{"started": true}, nil
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
			"running":        active(securityNow) || active("apt-daily-upgrade.service"),
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
		"certificate": a.certificateStatus(),
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
			known, ok := openPorts[key]
			if !ok {
				known.What, known.Icon = "Opened by an app", "app"
			}
			open = append(open, OpenPort{Port: port, Protocol: m[1], What: known.What, Detail: known.Detail, Icon: known.Icon})
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
// Profile is one program AppArmor knows: confined ("enforce") or only watched
// ("complain").
type Profile struct {
	Name     string `json:"name"`
	Enforced bool   `json:"enforced"`
}

func (a *Agent) apparmor() Response {
	on := a.readTrimmed("sys/module/apparmor/parameters/enabled") == "Y"
	enforced := 0
	profiles := []Profile{}
	for _, line := range strings.Split(a.readTrimmed("sys/kernel/security/apparmor/profiles"), "\n") {
		i := strings.LastIndex(line, " (")
		if i < 0 {
			continue
		}
		p := Profile{Name: line[:i], Enforced: strings.HasSuffix(line, "(enforce)")}
		if p.Enforced {
			enforced++
		}
		profiles = append(profiles, p)
	}
	sort.Slice(profiles, func(i, j int) bool {
		if profiles[i].Enforced != profiles[j].Enforced {
			return profiles[i].Enforced
		}
		return profiles[i].Name < profiles[j].Name
	})
	return Response{"on": on, "enforced": enforced, "profiles": profiles}
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
// apt's sources and the time servers, each with Box Desktop's words for it,
// when the box connects and its icon.
func (a *Agent) connections() []Connection {
	hosts := map[string]Connection{}
	add := func(raw string, c Connection) {
		if u, err := url.Parse(raw); err == nil && u.Hostname() != "" {
			c.Host = u.Hostname()
			hosts[c.Host] = c
		}
	}
	osSource := func(uri string) {
		c := Connection{Title: "OS Package Server", What: "System updates & security fixes",
			When: "Every night, and when an admin presses Check now", Icon: "package"}
		if strings.Contains(uri, "security") {
			c.Title, c.What = "OS Security Fix Server", "Security fixes"
		}
		add(uri, c)
	}
	files, _ := filepath.Glob(a.path("etc/apt/sources.list.d/*.sources"))
	for _, f := range files {
		for _, uri := range sourceURIs(f) {
			osSource(uri)
		}
	}
	for _, uri := range sourceURIs(a.path(ourSource)) {
		add(uri, Connection{Title: "ZOOMRIYA Updates", What: "The box's own software",
			When: "Only when an admin presses Update", Icon: "package"})
	}
	for _, line := range strings.Split(a.readTrimmed("etc/apt/sources.list"), "\n") {
		if f := strings.Fields(line); len(f) > 1 && f[0] == "deb" {
			osSource(f[1])
		}
	}

	out, _ := a.Run("timedatectl", "show-timesync", "--property=ServerName", "--value")
	servers := strings.Fields(string(out))
	if len(servers) == 0 {
		out, _ = a.Run("timedatectl", "show-timesync", "--property=FallbackNTPServers", "--value")
		servers = strings.Fields(string(out))
	}
	for _, s := range servers {
		hosts[s] = Connection{Host: s, Title: "Time Server", What: "The time, which certificates and updates depend on",
			When: "Every so often, to keep the clock right", Icon: "clock"}
	}

	list := []Connection{}
	for _, c := range hosts {
		list = append(list, c)
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].Title != list[j].Title {
			return list[i].Title < list[j].Title
		}
		return list[i].Host < list[j].Host
	})
	return list
}
