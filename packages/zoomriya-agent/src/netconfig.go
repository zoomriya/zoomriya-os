// network-set and network-keep (design §5.2): a fixed address for an Ethernet
// port, or back to DHCP, the default. A change undoes itself after two
// minutes unless an admin keeps it, from wherever Box Desktop still answers,
// so a wrong address can't shut them out of the box.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	networkDir = "etc/systemd/network"
	// What each port was before a change that's not kept yet.
	undoDir   = "run/zoomriya/network-undo"
	undoAfter = 2 * time.Minute
	agentPath = "/usr/lib/zoomriya-agent/zoomriya-agent"
)

// A port's name as the kernel gives it; also safe in a path and a unit name.
var linkName = regexp.MustCompile(`^[A-Za-z0-9_-]{1,15}$`)

// The box's address, set by hand.
type Fixed struct {
	Address string   `json:"address"` // with its prefix: 192.168.1.40/24
	Gateway string   `json:"gateway"`
	DNS     []string `json:"dns"`
}

type undo struct {
	Until    string `json:"until"`
	HadFile  bool   `json:"had_file"`
	Previous string `json:"previous"`
}

func fixedFile(link string) string {
	return filepath.Join(networkDir, "50-zoomriya-"+link+".network")
}

func undoUnit(link string) string { return "zoomriya-network-undo-" + link }

// fixedConfig checks the settings and writes them as systemd.network(5).
func fixedConfig(link, address, gateway string, dns []string) (string, error) {
	prefix, err := netip.ParsePrefix(strings.TrimSpace(address))
	if err != nil || !prefix.Addr().Is4() {
		return "", refuse("The address should look like 192.168.1.40, with its subnet mask.")
	}
	if prefix.Bits() < 8 || prefix.Bits() > 30 {
		return "", refuse("That subnet mask leaves no room for the box and the router.")
	}
	addr := prefix.Addr()
	network := prefix.Masked().Addr()
	broadcast := lastAddress(prefix)
	usable := func(a netip.Addr) bool {
		return a.IsGlobalUnicast() && a != network && a != broadcast
	}
	if !usable(addr) {
		return "", refuse("%s can't be the box's address on that network.", addr)
	}
	gw, err := netip.ParseAddr(strings.TrimSpace(gateway))
	if err != nil || !gw.Is4() {
		return "", refuse("The router's address should look like 192.168.1.1.")
	}
	if !prefix.Contains(gw) || !usable(gw) || gw == addr {
		return "", refuse("The router %s isn't on the same network as %s.", gw, prefix)
	}
	if len(dns) == 0 || len(dns) > 3 {
		return "", refuse("Give one to three name servers (often the router's address).")
	}
	servers := []string{}
	for _, d := range dns {
		server, err := netip.ParseAddr(strings.TrimSpace(d))
		if err != nil || server.IsUnspecified() || server.IsMulticast() || server.IsLoopback() {
			return "", refuse("%q isn't a name server's address.", d)
		}
		servers = append(servers, server.String())
	}

	return fmt.Sprintf(`# A fixed address for %[1]s, set in Box Desktop (zoomriya-agent's
# network-set). Without this file the port takes an address by DHCP.
# Format: systemd.network(5).
[Match]
Name=%[1]s

[Network]
DNS=%[4]s
LinkLocalAddressing=ipv6
IPv6AcceptRA=yes

[Address]
Address=%[2]s
DuplicateAddressDetection=ipv4

[Route]
Gateway=%[3]s
Metric=100

[IPv6AcceptRA]
RouteMetric=100
`, link, prefix, gw, strings.Join(servers, " ")), nil
}

func lastAddress(p netip.Prefix) netip.Addr {
	b := p.Masked().Addr().As4()
	for i := p.Bits(); i < 32; i++ {
		b[i/8] |= 1 << (7 - i%8)
	}
	return netip.AddrFrom4(b)
}

// readFixed reads back the settings this agent wrote; nil for DHCP.
func (a *Agent) readFixed(link string) *Fixed {
	data, err := os.ReadFile(a.path(fixedFile(link)))
	if err != nil {
		return nil
	}
	f := &Fixed{DNS: []string{}}
	for _, line := range strings.Split(string(data), "\n") {
		key, value, _ := strings.Cut(strings.TrimSpace(line), "=")
		switch key {
		case "Address":
			f.Address = value
		case "Gateway":
			f.Gateway = value
		case "DNS":
			f.DNS = strings.Fields(value)
		}
	}
	return f
}

// undoUntil is when a change not kept yet undoes itself, or "".
func (a *Agent) undoUntil(link string) string {
	data, err := os.ReadFile(a.path(filepath.Join(undoDir, link)))
	if err != nil {
		return ""
	}
	var u undo
	if json.Unmarshal(data, &u) != nil {
		return ""
	}
	return u.Until
}

func (a *Agent) ethernetPort(link *string) (string, error) {
	if link == nil || !linkName.MatchString(*link) {
		return "", refuse("Which port? None was given.")
	}
	dir := filepath.Join("sys/class/net", *link)
	if a.readTrimmed(filepath.Join(dir, "type")) != "1" {
		return "", refuse("The box has no port called %s.", *link)
	}
	if kind := a.kindOf(*link); kind != "ethernet" {
		return "", refuse("Fixed addresses are for the box's Ethernet ports.")
	}
	return *link, nil
}

func (a *Agent) networkSet(link, mode, address, gateway *string, dns []string) (Response, error) {
	port, err := a.ethernetPort(link)
	if err != nil {
		return nil, err
	}
	file := a.path(fixedFile(port))
	var config string
	switch {
	case mode != nil && *mode == "dhcp":
	case mode != nil && *mode == "fixed":
		if address == nil || gateway == nil {
			return nil, refuse("A fixed address needs the address, the router and a name server.")
		}
		if config, err = fixedConfig(port, *address, *gateway, dns); err != nil {
			return nil, err
		}
	default:
		return nil, refuse("By DHCP or a fixed address?")
	}

	// Keep what the port had before the first change not yet kept, so the
	// undo goes back to settings that worked.
	until := a.now().Add(undoAfter)
	keep := a.path(filepath.Join(undoDir, port))
	var u undo
	if data, err := os.ReadFile(keep); err != nil || json.Unmarshal(data, &u) != nil {
		previous, err := os.ReadFile(file)
		u = undo{HadFile: err == nil, Previous: string(previous)}
	}
	u.Until = until.UTC().Format(time.RFC3339)
	data, _ := json.Marshal(u)
	if err := os.MkdirAll(filepath.Dir(keep), 0o700); err != nil {
		return nil, err
	}
	if err := os.WriteFile(keep, data, 0o600); err != nil {
		return nil, err
	}

	if err := a.writeNetworkFile(port, config); err != nil {
		return nil, err
	}
	// A change before the last one was kept starts the two minutes again.
	_, _ = a.Run("systemctl", "stop", undoUnit(port)+".timer")
	if _, err := a.Run("systemd-run", "--quiet", "--collect", "--unit="+undoUnit(port),
		"--on-active="+fmt.Sprint(int(undoAfter.Seconds())), "--timer-property=AccuracySec=1s",
		// Done before it counts as started, so nothing overlaps it.
		"--property=Type=oneshot",
		"--description=Undo network settings that weren't kept", "--", agentPath, "network-undo", port); err != nil {
		return nil, err
	}
	if err := a.applyNetwork(port); err != nil {
		return nil, err
	}
	return Response{"link": port, "mode": *mode, "undo_at": u.Until}, nil
}

// writeNetworkFile puts a port's settings in place; "" is DHCP.
func (a *Agent) writeNetworkFile(port, config string) error {
	file := a.path(fixedFile(port))
	if config == "" {
		if err := os.Remove(file); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return err
	}
	temp := file + ".new"
	if err := os.WriteFile(temp, []byte(config), 0o644); err != nil {
		return err
	}
	return os.Rename(temp, file)
}

func (a *Agent) applyNetwork(port string) error {
	if _, err := a.Run("networkctl", "reload"); err != nil {
		return err
	}
	_, err := a.Run("networkctl", "reconfigure", port)
	return err
}

// networkKeep keeps every change waiting to be kept.
func (a *Agent) networkKeep() (Response, error) {
	entries, _ := os.ReadDir(a.path(undoDir))
	kept := []string{}
	for _, e := range entries {
		if !linkName.MatchString(e.Name()) {
			continue
		}
		_, _ = a.Run("systemctl", "stop", undoUnit(e.Name())+".timer")
		if err := os.Remove(a.path(filepath.Join(undoDir, e.Name()))); err == nil {
			kept = append(kept, e.Name())
		}
	}
	if len(kept) == 0 {
		return nil, refuse("There are no new network settings waiting to be kept.")
	}
	return Response{"kept": kept}, nil
}

// networkUndo is the timer's job when a change wasn't kept in time; false
// when there was nothing left to undo.
func (a *Agent) networkUndo(port string) (bool, error) {
	if !linkName.MatchString(port) {
		return false, fmt.Errorf("no port %q", port)
	}
	// Started early (support, the VM test), the timer mustn't run it again.
	_, _ = a.Run("systemctl", "stop", undoUnit(port)+".timer")
	keep := a.path(filepath.Join(undoDir, port))
	data, err := os.ReadFile(keep)
	if err != nil {
		return false, nil // kept in the meantime
	}
	var u undo
	if err := json.Unmarshal(data, &u); err != nil {
		return false, err
	}
	config := ""
	if u.HadFile {
		config = u.Previous
	}
	if err := a.writeNetworkFile(port, config); err != nil {
		return false, err
	}
	if err := os.Remove(keep); err != nil {
		return false, err
	}
	return true, a.applyNetwork(port)
}
