package main

import (
	"strings"
	"testing"
)

const testPasswd = `root:x:0:0:root:/root:/bin/bash
_apt:x:42:65534::/nonexistent:/usr/sbin/nologin
systemd-timesync:x:996:996::/:/usr/sbin/nologin
zoomriya:x:990:990::/opt/zoomriya:/usr/sbin/nologin
zoomriya-chat:x:989:989::/:/usr/sbin/nologin
zoomriya-maps:x:988:988::/:/usr/sbin/nologin
zoomriya-setup:x:987:987::/:/usr/sbin/nologin
caddy:x:986:986::/var/lib/caddy:/usr/sbin/nologin
`

const streetMap = `{"id": "street-map", "account": "zoomriya-maps", "name": "Street map downloads",
 "feature": "Family Tracker's map", "what": "Which province's map", "why": "The map is drawn on the box",
 "when": "When an admin asks", "hosts": ["build.protomaps.com"], "optional": true}`

// The rules the agent put in outbound_policy, one line each.
func policyRules(box *fakeBox) []string {
	var rules []string
	for _, cmd := range box.ran {
		line := strings.Join(cmd, " ")
		if strings.HasPrefix(line, "nft flush chain inet zoomriya outbound_policy") {
			rules = nil
		} else if strings.HasPrefix(line, "nft add rule inet zoomriya outbound_policy ") {
			rules = append(rules, strings.TrimPrefix(line, "nft add rule inet zoomriya outbound_policy "))
		}
	}
	return rules
}

func TestInternetAccess(t *testing.T) {
	agent, box := newFakeBox(t)
	box.write("etc/passwd", testPasswd)
	box.write(outboundDir+"/street-map.json", streetMap)
	box.write(outboundDir+"/broken.json", "{not json")

	// By default: every app blocked except the declared street map; the OS
	// (root, _apt, the clock) and the setup screen untouched.
	if err := agent.internetApply(); err != nil {
		t.Fatal(err)
	}
	rules := policyRules(box)
	if len(rules) != 4 || !strings.Contains(rules[0], "meta skuid { 986, 989, 990 } ip daddr != @home_v4") ||
		!strings.Contains(rules[0], `log prefix "zoomriya-blocked: "`) || !strings.HasSuffix(rules[1], "drop") ||
		!strings.Contains(rules[2], "ip6 daddr != @home_v6") {
		t.Fatalf("rules: %q", rules)
	}

	resp, err := agent.Handle(Request{Op: "internet"})
	if err != nil {
		t.Fatal(err)
	}
	declared := resp["declared"].([]Declared)
	if len(declared) != 1 || !declared[0].Allowed || declared[0].What == "" {
		t.Fatalf("declared: %+v", declared)
	}

	// The street map switched off: its account is blocked too.
	off, on := false, true
	if _, err := agent.Handle(Request{Op: "internet-set", Action: str("stop"), Name: str("street-map")}); err != nil {
		t.Fatal(err)
	}
	if rules := policyRules(box); !strings.Contains(rules[0], "{ 986, 988, 989, 990 }") {
		t.Fatalf("map off: %q", rules)
	}
	resp, _ = agent.Handle(Request{Op: "internet"})
	if d := resp["declared"].([]Declared)[0]; d.Allowed || d.Blocked != "Switched off" {
		t.Fatalf("shown: %+v", d)
	}
	if _, err := agent.Handle(Request{Op: "internet-set", Action: str("allow"), Name: str("street-map")}); err != nil {
		t.Fatal(err)
	}

	// Apps offline: every app account, the map's too.
	if _, err := agent.Handle(Request{Op: "internet-set", Action: str("apps-offline"), On: &on}); err != nil {
		t.Fatal(err)
	}
	if rules := policyRules(box); !strings.Contains(rules[0], "{ 986, 988, 989, 990 }") {
		t.Fatalf("apps offline: %q", rules)
	}

	// Everything offline: nothing leaves, whoever asks.
	if _, err := agent.Handle(Request{Op: "internet-set", Action: str("all-offline"), On: &on}); err != nil {
		t.Fatal(err)
	}
	if rules := policyRules(box); len(rules) != 4 || strings.Contains(rules[0], "skuid {") ||
		!strings.HasPrefix(rules[0], "ip daddr != @home_v4") {
		t.Fatalf("all offline: %q", rules)
	}
	resp, _ = agent.Handle(Request{Op: "internet"})
	if resp["all_offline"] != true || resp["apps_offline"] != true {
		t.Fatalf("switches: %v", resp)
	}
	for _, action := range []string{"all-offline", "apps-offline"} {
		if _, err := agent.Handle(Request{Op: "internet-set", Action: str(action), On: &off}); err != nil {
			t.Fatal(err)
		}
	}
	if rules := policyRules(box); !strings.Contains(rules[0], "{ 986, 989, 990 }") {
		t.Fatalf("back on: %q", rules)
	}
}

func TestInternetRefusals(t *testing.T) {
	agent, box := newFakeBox(t)
	box.write("etc/passwd", testPasswd)
	box.write(outboundDir+"/needed.json", strings.Replace(streetMap, `"optional": true`, `"optional": false`, 1))
	for _, r := range []Request{
		{Op: "internet-set"},
		{Op: "internet-set", Action: str("stop"), Name: str("street-map")},
		{Op: "internet-set", Action: str("stop"), Name: str("nothing")},
		{Op: "internet-set", Action: str("apps-offline")},
		{Op: "internet-set", Action: str("explode"), On: new(bool)},
	} {
		if _, err := agent.Handle(r); err == nil {
			t.Errorf("%+v accepted", r)
		}
	}
}
