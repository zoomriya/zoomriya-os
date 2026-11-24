// Internet access (design §8.2): what may leave the box, not only what did.
//
//   - Apps are blocked by default. An app's account reaches the internet only
//     for a connection it declares in /etc/zoomriya/outbound.d/<id>.json: in
//     plain words what leaves, why, and for which feature. An optional one has
//     a switch in Box Desktop's Control Panel › Security.
//   - Apps offline blocks every app, declared or not; the OS still fetches its
//     security fixes and the time.
//   - Everything offline blocks the OS too: nothing leaves the home network.
//
// The firewall's outbound_policy chain (zoomriya-base's nftables.conf) holds
// the rules; the agent fills it at each start and each change, as it does
// SSH's. What it stops is logged ("zoomriya-blocked: "), so Box Desktop shows
// what was blocked beside what went out.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const (
	outboundDir   = "etc/zoomriya/outbound.d"
	internetState = "etc/zoomriya/internet.json"
	blockedPrefix = "zoomriya-blocked: "
)

// A connection an app declares.
type Declared struct {
	ID       string   `json:"id"`
	Account  string   `json:"account"`
	Name     string   `json:"name"`
	Feature  string   `json:"feature"`
	What     string   `json:"what"`
	Why      string   `json:"why"`
	When     string   `json:"when"`
	Hosts    []string `json:"hosts"`
	Optional bool     `json:"optional"`
	// Worked out here: allowed now, and if not, why.
	Allowed bool   `json:"allowed"`
	Blocked string `json:"blocked,omitempty"`
	// Its own switch is off (whatever the offline switches say).
	Off bool `json:"off"`
}

type internetChoice struct {
	AppsOffline bool     `json:"apps_offline"`
	AllOffline  bool     `json:"all_offline"`
	Off         []string `json:"off"`
}

func (a *Agent) internetChoice() internetChoice {
	var c internetChoice
	if data, err := os.ReadFile(a.path(internetState)); err == nil {
		_ = json.Unmarshal(data, &c)
	}
	return c
}

// declared reads the apps' declarations; a broken one is skipped (and so its
// account stays blocked).
func (a *Agent) declared(c internetChoice) []Declared {
	files, _ := filepath.Glob(filepath.Join(a.path(outboundDir), "*.json"))
	sort.Strings(files)
	list := []Declared{}
	for _, file := range files {
		var d Declared
		data, err := os.ReadFile(file)
		if err != nil || json.Unmarshal(data, &d) != nil || d.ID == "" || d.Account == "" || d.Name == "" {
			continue
		}
		d.Allowed = true
		d.Off = d.Optional && contains(c.Off, d.ID)
		switch {
		case c.AllOffline:
			d.Allowed, d.Blocked = false, "Everything offline is on"
		case c.AppsOffline:
			d.Allowed, d.Blocked = false, "Apps offline is on"
		case d.Off:
			d.Allowed, d.Blocked = false, "Switched off"
		}
		list = append(list, d)
	}
	return list
}

// appAccount: every app runs as its own account, zoomriya or zoomriya-…, and
// the apps' gateway as caddy. The OS's own setup screen isn't an app.
func appAccount(name string) bool {
	if name == "zoomriya-setup" {
		return false
	}
	return name == "zoomriya" || name == "caddy" || strings.HasPrefix(name, "zoomriya-")
}

// blockedUIDs: the app accounts with no connection allowed now.
func (a *Agent) blockedUIDs(c internetChoice) []int {
	allowed := map[string]bool{}
	for _, d := range a.declared(c) {
		if d.Allowed {
			allowed[d.Account] = true
		}
	}
	var uids []int
	for uid, name := range a.accounts() {
		n, err := strconv.Atoi(uid)
		if err == nil && appAccount(name) && !allowed[name] {
			uids = append(uids, n)
		}
	}
	sort.Ints(uids)
	return uids
}

// outboundRules is the outbound_policy chain for this choice.
func (a *Agent) outboundRules(c internetChoice) [][]string {
	block := func(match []string) [][]string {
		var rules [][]string
		for _, family := range [][2]string{{"ip", "@home_v4"}, {"ip6", "@home_v6"}} {
			away := append(append([]string{}, match...), family[0], "daddr", "!=", family[1])
			rules = append(rules,
				append(append([]string{}, away...), "ct", "state", "new", "limit", "rate", "60/minute",
					"burst", "120", "packets", "log", "prefix", `"`+blockedPrefix+`"`, "flags", "skuid"),
				append(append([]string{}, away...), "drop"))
		}
		return rules
	}
	if c.AllOffline {
		return block(nil)
	}
	uids := a.blockedUIDs(c)
	if len(uids) == 0 {
		return nil
	}
	parts := make([]string, len(uids))
	for i, uid := range uids {
		parts[i] = strconv.Itoa(uid)
	}
	return block([]string{"meta", "skuid", "{ " + strings.Join(parts, ", ") + " }"})
}

// internetApply fills outbound_policy: at each start, each firewall reload
// (zoomriya-internet-apply.service) and each change.
func (a *Agent) internetApply() error {
	if _, err := a.Run("nft", "flush", "chain", "inet", "zoomriya", "outbound_policy"); err != nil {
		return err
	}
	for _, rule := range a.outboundRules(a.internetChoice()) {
		if _, err := a.Run("nft", append([]string{"add", "rule", "inet", "zoomriya", "outbound_policy"}, rule...)...); err != nil {
			return err
		}
	}
	return nil
}

func internetApplyJob() int {
	if err := NewAgent().internetApply(); err != nil {
		NewJournal().Record(AuditEntry{Op: "internet-apply", By: "internet access", Result: err.Error()})
		return 1
	}
	return 0
}

// internet is Control Panel › Security's Internet access (read only).
func (a *Agent) internet() (Response, error) {
	c := a.internetChoice()
	return Response{
		"apps_offline": c.AppsOffline,
		"all_offline":  c.AllOffline,
		"declared":     a.declared(c),
		"blocked":      a.logged(blockedPrefix),
		"days":         outboundDays,
	}, nil
}

// internetSet: Action "apps-offline" or "all-offline" with On; or "allow" /
// "stop" with Name, an optional connection's id.
func (a *Agent) internetSet(action, name *string, on *bool) (Response, error) {
	c := a.internetChoice()
	switch {
	case action == nil:
		return nil, refuse("What should change?")
	case *action == "apps-offline" && on != nil:
		c.AppsOffline = *on
	case *action == "all-offline" && on != nil:
		c.AllOffline = *on
	case (*action == "allow" || *action == "stop") && name != nil:
		var found *Declared
		for _, d := range a.declared(internetChoice{}) {
			if d.ID == *name {
				found = &d
				break
			}
		}
		if found == nil {
			return nil, refuse("No app declares a connection called %s.", *name)
		}
		if !found.Optional {
			return nil, refuse("%s is needed for the app to work; turn the app off instead.", found.Name)
		}
		off := []string{}
		for _, id := range c.Off {
			if id != *name {
				off = append(off, id)
			}
		}
		if *action == "stop" {
			off = append(off, *name)
		}
		c.Off = off
	default:
		return nil, refuse("Unknown change: %s.", *action)
	}
	data, _ := json.MarshalIndent(c, "", "  ")
	if err := writeFileAtomic(a.path(internetState), append(data, '\n'), 0o644); err != nil {
		return nil, err
	}
	if err := a.internetApply(); err != nil {
		return nil, fmt.Errorf("saved, but the firewall didn't take it: %w", err)
	}
	return a.internet()
}
