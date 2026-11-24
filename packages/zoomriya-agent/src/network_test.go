package main

import "testing"

const ipAddresses = `[
 {"ifname": "lo", "link_type": "loopback", "address": "00:00:00:00:00:00", "operstate": "UNKNOWN",
  "addr_info": [{"family": "inet", "local": "127.0.0.1", "prefixlen": 8, "scope": "host"}]},
 {"ifname": "enp1s0", "link_type": "ether", "address": "52:54:00:12:34:56", "operstate": "UP",
  "addr_info": [{"family": "inet", "local": "192.168.1.40", "prefixlen": 24, "scope": "global"},
                {"family": "inet6", "local": "fd00::40", "prefixlen": 64, "scope": "global"},
                {"family": "inet6", "local": "fe80::5054:ff:fe12:3456", "prefixlen": 64, "scope": "link"}]},
 {"ifname": "wlan0", "link_type": "ether", "address": "a4:83:e7:00:11:22", "operstate": "UP",
  "addr_info": [{"family": "inet", "local": "192.168.1.41", "prefixlen": 24, "scope": "global"}]},
 {"ifname": "enp2s0", "link_type": "ether", "address": "52:54:00:12:34:57", "operstate": "DOWN", "addr_info": []},
 {"ifname": "ve-photos", "link_type": "ether", "address": "be:ef:00:00:00:01", "operstate": "UP", "addr_info": []}
]`

func networkBox(t *testing.T) (*Agent, *fakeBox) {
	agent, box := newFakeBox(t)
	box.answers = map[string]string{
		"ip -json address show": ipAddresses,
		// Both connected: Ethernet's lower metric carries the traffic.
		"ip -json route show default": `[{"dst": "default", "gateway": "192.168.1.1", "dev": "wlan0", "metric": 600},
                                        {"dst": "default", "gateway": "192.168.1.1", "dev": "enp1s0", "metric": 100}]`,
	}
	box.write("sys/class/net/enp1s0/device/vendor", "0x8086\n")
	box.write("sys/class/net/enp1s0/speed", "1000\n")
	box.write("sys/class/net/enp2s0/device/vendor", "0x8086\n")
	box.write("sys/class/net/wlan0/device/vendor", "0x8086\n")
	box.write("sys/class/net/wlan0/wireless/.keep", "")
	box.write("sys/class/net/wlan0/speed", "-1\n")
	box.write("run/systemd/resolve/resolv.conf", "# generated\nnameserver 192.168.1.1\nnameserver fd00::1\nsearch home\n")
	return agent, box
}

func TestNetwork(t *testing.T) {
	agent, _ := networkBox(t)
	resp, err := agent.Handle(Request{Op: "network"})
	if err != nil {
		t.Fatal(err)
	}
	if resp["online"] != true || resp["via"] != "ethernet" {
		t.Errorf("online %v via %v", resp["online"], resp["via"])
	}
	links := map[string]Link{}
	for _, l := range resp["links"].([]Link) {
		links[l.Name] = l
	}
	if len(links) != 3 {
		t.Fatalf("links = %+v, want both Ethernet ports and the Wi-Fi (no loopback, no container link)", links)
	}
	if second := links["enp2s0"]; second.Kind != "ethernet" || second.Connected || second.Default {
		t.Errorf("second ethernet = %+v", second)
	}
	eth := links["enp1s0"]
	if eth.Kind != "ethernet" || !eth.Connected || !eth.Default || eth.Gateway != "192.168.1.1" || eth.SpeedMbps != 1000 ||
		len(eth.IPv4) != 1 || eth.IPv4[0] != "192.168.1.40/24" || len(eth.IPv6) != 1 || eth.MAC != "52:54:00:12:34:56" {
		t.Errorf("ethernet = %+v", eth)
	}
	wifi := links["wlan0"]
	if wifi.Kind != "wifi" || wifi.Default || wifi.SpeedMbps != 0 {
		t.Errorf("wifi = %+v", wifi)
	}
	dns := resp["dns"].([]string)
	if len(dns) != 2 || dns[0] != "192.168.1.1" {
		t.Errorf("dns = %v", dns)
	}
}

func TestOffline(t *testing.T) {
	agent, box := networkBox(t)
	box.answers["ip -json route show default"] = "[]"
	resp, _ := agent.Handle(Request{Op: "network"})
	if resp["online"] != false || resp["via"] != "" {
		t.Errorf("online %v via %v", resp["online"], resp["via"])
	}
}
