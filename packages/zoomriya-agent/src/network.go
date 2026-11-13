// network (design §7): the box on the home network: its Ethernet and Wi-Fi
// links, their addresses, which one carries the traffic, the gateway and the
// DNS servers, and the box's own name.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Link struct {
	Name      string   `json:"name"`
	Kind      string   `json:"kind"` // "ethernet" or "wifi"
	Connected bool     `json:"connected"`
	MAC       string   `json:"mac"`
	SpeedMbps int      `json:"speed_mbps"`
	IPv4      []string `json:"ipv4"`
	IPv6      []string `json:"ipv6"`
	Default   bool     `json:"default"`
	Gateway   string   `json:"gateway"`
	// Ethernet: "dhcp" or "fixed", the fixed settings, and when a change
	// that's not kept yet undoes itself (netconfig.go).
	Mode   string `json:"mode,omitempty"`
	Fixed  *Fixed `json:"fixed,omitempty"`
	UndoAt string `json:"undo_at,omitempty"`
}

type ipAddr struct {
	Ifname    string `json:"ifname"`
	LinkType  string `json:"link_type"`
	Address   string `json:"address"`
	Operstate string `json:"operstate"`
	AddrInfo  []struct {
		Family    string `json:"family"`
		Local     string `json:"local"`
		Prefixlen int    `json:"prefixlen"`
		Scope     string `json:"scope"`
	} `json:"addr_info"`
}

type ipRoute struct {
	Gateway string `json:"gateway"`
	Dev     string `json:"dev"`
	Metric  int    `json:"metric"`
}

func (a *Agent) network() (Response, error) {
	out, err := a.Run("ip", "-json", "address", "show")
	if err != nil {
		return nil, err
	}
	var addrs []ipAddr
	if err := json.Unmarshal(out, &addrs); err != nil {
		return nil, fmt.Errorf("ip's addresses: %w", err)
	}

	// The default route with the lowest metric carries the traffic (Ethernet,
	// when both are connected: design §5.1).
	var best *ipRoute
	if out, err := a.Run("ip", "-json", "route", "show", "default"); err == nil {
		var routes []ipRoute
		if json.Unmarshal(out, &routes) == nil {
			for i := range routes {
				if best == nil || routes[i].Metric < best.Metric {
					best = &routes[i]
				}
			}
		}
	}

	links := []Link{}
	via := ""
	for _, ad := range addrs {
		kind := a.linkKind(ad)
		if kind == "" {
			continue
		}
		link := Link{
			Name:      ad.Ifname,
			Kind:      kind,
			Connected: ad.Operstate == "UP",
			MAC:       ad.Address,
			IPv4:      []string{},
			IPv6:      []string{},
		}
		if speed, err := strconv.Atoi(a.readTrimmed(filepath.Join("sys/class/net", ad.Ifname, "speed"))); err == nil && speed > 0 {
			link.SpeedMbps = speed
		}
		for _, info := range ad.AddrInfo {
			if info.Scope != "global" {
				continue
			}
			address := fmt.Sprintf("%s/%d", info.Local, info.Prefixlen)
			if info.Family == "inet" {
				link.IPv4 = append(link.IPv4, address)
			} else if info.Family == "inet6" {
				link.IPv6 = append(link.IPv6, address)
			}
		}
		if kind == "ethernet" {
			link.Mode = "dhcp"
			if link.Fixed = a.readFixed(ad.Ifname); link.Fixed != nil {
				link.Mode = "fixed"
			}
			link.UndoAt = a.undoUntil(ad.Ifname)
		}
		if best != nil && best.Dev == ad.Ifname {
			link.Default = true
			link.Gateway = best.Gateway
			via = kind
		}
		links = append(links, link)
	}

	hostname, _ := os.Hostname()
	return Response{
		"name":   hostname + ".local",
		"online": via != "",
		"via":    via,
		"links":  links,
		"dns":    a.dnsServers(),
	}, nil
}

// linkKind is "ethernet" or "wifi" for the box's own network hardware, and ""
// for everything else (loopback, bridges, the VPN, containers' links).
func (a *Agent) linkKind(ad ipAddr) string {
	if ad.LinkType != "ether" {
		return ""
	}
	return a.kindOf(ad.Ifname)
}

// kindOf tells an Ethernet-type link's hardware apart: "ethernet", "wifi", or
// "" when there's no hardware behind it.
func (a *Agent) kindOf(name string) string {
	dir := filepath.Join("sys/class/net", name)
	if _, err := os.Stat(a.path(filepath.Join(dir, "device"))); err != nil {
		return "" // virtual: no hardware behind it
	}
	if _, err := os.Stat(a.path(filepath.Join(dir, "wireless"))); err == nil {
		return "wifi"
	}
	if strings.Contains(a.readTrimmed(filepath.Join(dir, "uevent")), "DEVTYPE=wlan") {
		return "wifi"
	}
	return "ethernet"
}

// dnsServers are the ones systemd-resolved asks, from its own resolv.conf.
func (a *Agent) dnsServers() []string {
	servers := []string{}
	for _, line := range strings.Split(a.readTrimmed("run/systemd/resolve/resolv.conf"), "\n") {
		if server, found := strings.CutPrefix(strings.TrimSpace(line), "nameserver "); found {
			servers = append(servers, strings.TrimSpace(server))
		}
	}
	return servers
}
