package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func portsBox(t *testing.T) (*Agent, *fakeBox) {
	agent, box := networkBox(t)
	agent.Now = func() time.Time { return time.Date(2026, 11, 3, 10, 0, 0, 0, time.UTC) }
	for _, port := range []string{"enp1s0", "enp2s0", "wlan0"} {
		box.write("sys/class/net/"+port+"/type", "1\n")
	}
	return agent, box
}

func ranCommand(box *fakeBox, want string) bool {
	for _, ran := range box.ran {
		if strings.Join(ran, " ") == want {
			return true
		}
	}
	return false
}

func TestFixedAddress(t *testing.T) {
	agent, box := portsBox(t)
	resp, err := agent.Handle(Request{Op: "network-set", Link: str("enp2s0"), Mode: str("fixed"),
		Address: str("192.168.1.50/24"), Gateway: str("192.168.1.1"), DNS: []string{"192.168.1.1", " 9.9.9.9"}})
	if err != nil {
		t.Fatal(err)
	}
	if resp["undo_at"] != "2026-11-03T10:02:00Z" {
		t.Errorf("undo_at = %v", resp["undo_at"])
	}
	config, err := os.ReadFile(filepath.Join(box.root, "etc/systemd/network/50-zoomriya-enp2s0.network"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Name=enp2s0\n", "Address=192.168.1.50/24\n", "Gateway=192.168.1.1\n", "DNS=192.168.1.1 9.9.9.9\n", "Metric=100\n"} {
		if !strings.Contains(string(config), want) {
			t.Errorf("config lacks %q:\n%s", want, config)
		}
	}
	for _, want := range []string{
		"systemd-run --quiet --collect --unit=zoomriya-network-undo-enp2s0 --on-active=120 --timer-property=AccuracySec=1s --property=Type=oneshot --description=Undo network settings that weren't kept -- /usr/lib/zoomriya-agent/zoomriya-agent network-undo enp2s0",
		"networkctl reload",
		"networkctl reconfigure enp2s0",
	} {
		if !ranCommand(box, want) {
			t.Errorf("didn't run %s; ran %v", want, box.ran)
		}
	}

	// Network shows it, with the undo waiting.
	net, _ := agent.Handle(Request{Op: "network"})
	for _, l := range net["links"].([]Link) {
		if l.Name == "enp2s0" && (l.Mode != "fixed" || l.Fixed.Address != "192.168.1.50/24" || l.Fixed.Gateway != "192.168.1.1" ||
			!slices.Equal(l.Fixed.DNS, []string{"192.168.1.1", "9.9.9.9"}) || l.UndoAt != "2026-11-03T10:02:00Z") {
			t.Errorf("enp2s0 = %+v", l)
		}
		if l.Name == "enp1s0" && (l.Mode != "dhcp" || l.Fixed != nil || l.UndoAt != "") {
			t.Errorf("enp1s0 = %+v", l)
		}
		if l.Name == "wlan0" && l.Mode != "" {
			t.Errorf("wlan0 = %+v", l)
		}
	}

	// Not kept: the undo goes back to DHCP, the port's settings before.
	if undone, err := agent.networkUndo("enp2s0"); err != nil || !undone {
		t.Fatal(undone, err)
	}
	if _, err := os.Stat(filepath.Join(box.root, "etc/systemd/network/50-zoomriya-enp2s0.network")); !os.IsNotExist(err) {
		t.Error("the fixed address is still there after the undo")
	}
	if _, err := os.Stat(filepath.Join(box.root, undoDir, "enp2s0")); !os.IsNotExist(err) {
		t.Error("the undo is still waiting after it ran")
	}
}

func TestUndoKeepsFirstSettings(t *testing.T) {
	agent, box := portsBox(t)
	box.write("etc/systemd/network/50-zoomriya-enp1s0.network", "[Address]\nAddress=10.0.0.5/24\n")
	set := func(address string) {
		if _, err := agent.Handle(Request{Op: "network-set", Link: str("enp1s0"), Mode: str("fixed"),
			Address: str(address), Gateway: str("10.0.0.1"), DNS: []string{"10.0.0.1"}}); err != nil {
			t.Fatal(err)
		}
	}
	set("10.0.0.6/24")
	set("10.0.0.7/24")
	var u undo
	data, _ := os.ReadFile(filepath.Join(box.root, undoDir, "enp1s0"))
	if json.Unmarshal(data, &u) != nil || !u.HadFile || !strings.Contains(u.Previous, "10.0.0.5/24") {
		t.Errorf("undo = %+v, want the settings from before both changes", u)
	}
	if !ranCommand(box, "systemctl stop zoomriya-network-undo-enp1s0.timer") {
		t.Error("the second change didn't start the two minutes again")
	}
	if undone, err := agent.networkUndo("enp1s0"); err != nil || !undone {
		t.Fatal(undone, err)
	}
	if got, _ := os.ReadFile(filepath.Join(box.root, "etc/systemd/network/50-zoomriya-enp1s0.network")); !strings.Contains(string(got), "10.0.0.5/24") {
		t.Errorf("after the undo: %s", got)
	}
}

func TestKeep(t *testing.T) {
	agent, box := portsBox(t)
	if _, err := agent.Handle(Request{Op: "network-keep"}); err == nil {
		t.Error("kept with nothing waiting")
	}
	if _, err := agent.Handle(Request{Op: "network-set", Link: str("enp1s0"), Mode: str("dhcp")}); err != nil {
		t.Fatal(err)
	}
	resp, err := agent.Handle(Request{Op: "network-keep"})
	if err != nil {
		t.Fatal(err)
	}
	if kept := resp["kept"].([]string); len(kept) != 1 || kept[0] != "enp1s0" {
		t.Errorf("kept = %v", kept)
	}
	if !ranCommand(box, "systemctl stop zoomriya-network-undo-enp1s0.timer") {
		t.Error("keeping didn't stop the undo")
	}
	// The timer firing after the change was kept: nothing to undo.
	if undone, err := agent.networkUndo("enp1s0"); err != nil || undone {
		t.Fatal(undone, err)
	}
}

func TestFixedAddressRefused(t *testing.T) {
	agent, box := portsBox(t)
	fixed := func(link, address, gateway string, dns ...string) Request {
		return Request{Op: "network-set", Link: str(link), Mode: str("fixed"), Address: str(address), Gateway: str(gateway), DNS: dns}
	}
	for name, req := range map[string]Request{
		"no port":          {Op: "network-set", Mode: str("dhcp")},
		"a path":           fixed("../../etc", "192.168.1.50/24", "192.168.1.1", "192.168.1.1"),
		"no such port":     fixed("enp9s0", "192.168.1.50/24", "192.168.1.1", "192.168.1.1"),
		"Wi-Fi":            fixed("wlan0", "192.168.1.50/24", "192.168.1.1", "192.168.1.1"),
		"no mode":          {Op: "network-set", Link: str("enp1s0")},
		"no mask":          fixed("enp1s0", "192.168.1.50", "192.168.1.1", "192.168.1.1"),
		"IPv6":             fixed("enp1s0", "fd00::50/64", "fd00::1", "fd00::1"),
		"network address":  fixed("enp1s0", "192.168.1.0/24", "192.168.1.1", "192.168.1.1"),
		"broadcast":        fixed("enp1s0", "192.168.1.255/24", "192.168.1.1", "192.168.1.1"),
		"loopback":         fixed("enp1s0", "127.0.0.5/8", "127.0.0.1", "192.168.1.1"),
		"mask too small":   fixed("enp1s0", "192.168.1.50/31", "192.168.1.51", "192.168.1.1"),
		"router elsewhere": fixed("enp1s0", "192.168.1.50/24", "192.168.2.1", "192.168.1.1"),
		"router is box":    fixed("enp1s0", "192.168.1.50/24", "192.168.1.50", "192.168.1.1"),
		"no DNS":           fixed("enp1s0", "192.168.1.50/24", "192.168.1.1"),
		"bad DNS":          fixed("enp1s0", "192.168.1.50/24", "192.168.1.1", "1.1.1.1\nDNS=6.6.6.6"),
		"newline":          fixed("enp1s0", "192.168.1.50/24\n[Match]", "192.168.1.1", "192.168.1.1"),
	} {
		if _, err := agent.Handle(req); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if len(box.ran) > 0 && ranCommand(box, "networkctl reload") {
		t.Error("networkctl ran for a refused change")
	}
	if entries, _ := os.ReadDir(filepath.Join(box.root, "etc/systemd/network")); len(entries) > 0 {
		t.Errorf("wrote %v for a refused change", entries)
	}
}
