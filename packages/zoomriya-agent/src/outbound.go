package main

import (
	"bytes"
	"encoding/json"
	"sort"
	"strconv"
	"strings"
	"time"
)

// outboundPrefix starts the kernel's line for each new connection from the
// box to the internet (zoomriya-base's nftables.conf, chain output): the
// account that made it (every app runs as its own), where to and when.
// Home network and loopback connections are never logged.
const outboundPrefix = "zoomriya-out: "

// outboundDays is how far back Control Panel › Outbound connections looks.
const outboundDays = 7

// Outbound is one destination one account reached, put together from the
// kernel's lines: how often and when last.
type Outbound struct {
	Account     string `json:"account"`
	Who         string `json:"who"`
	Destination string `json:"destination"`
	Protocol    string `json:"protocol"`
	Port        int    `json:"port"`
	Count       int    `json:"count"`
	Last        string `json:"last"`
}

// whoFor names an account the way Box Desktop shows it.
var whoFor = map[string]string{
	"_apt":             "OS updates",
	"systemd-timesync": "The clock",
	"root":             "The OS",
	"zoomriya-agent":   "The OS",
	"zoomriya-photos":  "Family Photos",
	"zoomriya-vault":   "Family Vault",
	"zoomriya-chat":    "Family Chat",
	"zoomriya-tracker": "Family Tracker",
	"zoomriya-planner": "Family Planner",
	"zoomriya-myira":   "Myira",
	"zoomriya-id":      "ZOOMRIYA ID",
	"zoomriya-connect": "ZOOMRIYA Connect",
	"zoomriya-maps":    "Street map",
	"zoomriya-backup":  "Backups",
}

// accounts maps uid to name, from the box's /etc/passwd.
func (a *Agent) accounts() map[string]string {
	names := map[string]string{}
	for _, line := range strings.Split(a.readTrimmed("etc/passwd"), "\n") {
		if f := strings.Split(line, ":"); len(f) > 2 {
			names[f[2]] = f[0]
		}
	}
	return names
}

// logFields reads the KEY=value words of a kernel log line.
func logFields(message string) map[string]string {
	fields := map[string]string{}
	for _, word := range strings.Fields(message) {
		if key, value, ok := strings.Cut(word, "="); ok {
			fields[key] = value
		}
	}
	return fields
}

// outbound: what actually left the box for the internet in the last week,
// by who and where, most recent first (read only).
func (a *Agent) outbound() (Response, error) {
	return Response{"days": outboundDays, "connections": a.logged(outboundPrefix)}, nil
}

// logged reads the kernel's lines with this prefix from the last week: what
// went out (outboundPrefix), or what the firewall stopped (blockedPrefix).
func (a *Agent) logged(prefix string) []Outbound {
	out, err := a.Run("journalctl", "--output=json", "--no-pager", "--quiet", "--dmesg",
		"--since=-"+strconv.Itoa(outboundDays)+"d", "--grep="+strings.TrimSpace(prefix))
	if err != nil {
		// No lines at all is an error to journalctl --grep: nothing logged.
		return []Outbound{}
	}
	outboundPrefix := prefix
	names := a.accounts()
	found := map[string]*Outbound{}
	for _, line := range bytes.Split(out, []byte("\n")) {
		var entry map[string]json.RawMessage
		if json.Unmarshal(line, &entry) != nil {
			continue
		}
		message := journalField(entry["MESSAGE"])
		at := strings.Index(message, outboundPrefix)
		if at < 0 {
			continue
		}
		f := logFields(message[at+len(outboundPrefix):])
		if f["DST"] == "" {
			continue
		}
		account := names[f["UID"]]
		if account == "" {
			account = "uid " + f["UID"]
			if f["UID"] == "" {
				account = "the kernel"
			}
		}
		port, _ := strconv.Atoi(f["DPT"])
		key := account + "|" + f["DST"] + "|" + f["PROTO"] + "|" + f["DPT"]
		row := found[key]
		if row == nil {
			who := whoFor[account]
			if who == "" {
				who = account
			}
			row = &Outbound{Account: account, Who: who, Destination: f["DST"],
				Protocol: strings.ToLower(f["PROTO"]), Port: port}
			found[key] = row
		}
		row.Count++
		if micros, err := strconv.ParseInt(journalField(entry["__REALTIME_TIMESTAMP"]), 10, 64); err == nil {
			when := time.UnixMicro(micros).UTC().Format(time.RFC3339)
			if when > row.Last {
				row.Last = when
			}
		}
	}
	list := []Outbound{}
	for _, row := range found {
		list = append(list, *row)
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].Last != list[j].Last {
			return list[i].Last > list[j].Last
		}
		return list[i].Who+list[i].Destination < list[j].Who+list[j].Destination
	})
	return list
}
