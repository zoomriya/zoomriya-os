// The watcher (box-desktop.md §3, notifications): `zoomriya-agent watch`,
// every quarter of an hour, after each security update and after each check
// of the library, compares the box with what it saw last time and writes what
// an admin should hear about into the journal as a notification. The
// notifications request reads them back for Box Desktop's bell.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	watchState = "var/lib/zoomriya-agent/watch.json"
	// How far back the bell looks, and how many it shows.
	notificationsSince = "-30d"
	notificationsMax   = 50
)

// The kinds of notification, which each admin can turn off.
var notificationKinds = []string{"drive", "update", "restart", "check", "network", "secure", "signin", "access", "apps"}

// Notifier writes a notification; the journal on the box, a list in tests.
type Notifier interface {
	Notify(kind, level, message string)
}

// What the watcher saw last time.
type watched struct {
	Drives       map[string]string `json:"drives"` // serial: health
	Restart      string            `json:"restart"`
	LastUpdate   string            `json:"last_update"`
	CheckStarted string            `json:"check_started"`
	Blocked      []string          `json:"blocked"`
}

func (a *Agent) watch(n Notifier) error {
	var last watched
	first := true
	if data, err := os.ReadFile(a.path(watchState)); err == nil && json.Unmarshal(data, &last) == nil {
		first = false
	}
	now := watched{Drives: map[string]string{}}

	// A drive's health: news when it changes, or when a drive that isn't
	// healthy is first seen.
	if resp, err := a.diskHealth(); err == nil {
		for _, d := range resp["drives"].([]Drive) {
			if d.Serial == "" || d.Health == "unknown" {
				continue
			}
			now.Drives[d.Serial] = d.Health
			before, seen := last.Drives[d.Serial]
			if before == d.Health || (!seen && d.Health == "healthy") {
				continue
			}
			name := d.Model
			if d.System {
				name = "The system drive (" + d.Model + ")"
			}
			switch d.Health {
			case "replace-now":
				n.Notify("drive", "error", fmt.Sprintf("%s should be replaced now. %s", name, strings.Join(d.Reasons, " ")))
			case "check-soon":
				n.Notify("drive", "warning", fmt.Sprintf("%s should be checked soon. %s", name, strings.Join(d.Reasons, " ")))
			case "healthy":
				n.Notify("drive", "info", fmt.Sprintf("%s is healthy again.", name))
			}
		}
	}

	// A restart planned: once for each new set of reasons.
	if reasons := a.restartReasons(); len(reasons) > 0 {
		now.Restart = strings.Join(reasons, ", ")
		if now.Restart != last.Restart {
			at := a.plannedRestart().Format("15:04")
			n.Notify("restart", "info", fmt.Sprintf("The box will restart by itself at %s to finish: %s.", at, now.Restart))
		}
	}

	// Security fixes installed.
	if m, ok := a.modified("var/log/unattended-upgrades/unattended-upgrades-dpkg.log").(string); ok {
		now.LastUpdate = m
		if m != last.LastUpdate && !first {
			text := "Security fixes were installed."
			if pkgs := a.lastUpgraded(); pkgs != "" {
				text = "Security fixes were installed: " + pkgs + "."
			}
			n.Notify("update", "info", text)
		}
	}

	// The library's check, once it's over.
	if lib, err := a.library(); err == nil && lib != nil {
		c := lib.Check
		now.CheckStarted = last.CheckStarted
		if (c.Finished || c.Paused) && c.Started != "" && c.Started != last.CheckStarted {
			now.CheckStarted = c.Started
			// With two drives the check repairs one copy from the other; with
			// one (OS-4) it finds damage only.
			switch {
			case c.Errors > 0 && lib.Mirror:
				n.Notify("check", "error", fmt.Sprintf("The library's check found %d problem(s) between its two copies. Open Storage Manager.", c.Errors))
			case c.Errors > 0:
				n.Notify("check", "error", fmt.Sprintf("The library's check found %d damaged file(s). Open Storage Manager.", c.Errors))
			case c.Finished && lib.Mirror:
				n.Notify("check", "info", "The library's check finished: both copies match.")
			case c.Finished:
				n.Notify("check", "info", "The library's check finished: no damage found.")
			}
		}
	}

	// An installed release put on the revocation list by an OS update.
	revoked := a.revoked()
	for id, app := range a.loadApps() {
		if revoked.blocked(id, app.Active) {
			key := id + " " + app.Active
			now.Blocked = append(now.Blocked, key)
			if !contains(last.Blocked, key) {
				n.Notify("apps", "error", fmt.Sprintf("%s %s has a known problem. Update it in App Center.", app.Name, app.Active))
			}
		}
	}

	data, _ := json.Marshal(now)
	if err := os.MkdirAll(filepath.Dir(a.path(watchState)), 0o700); err != nil {
		return err
	}
	return os.WriteFile(a.path(watchState), data, 0o600)
}

// lastUpgraded is unattended-upgrades' last list of upgraded packages.
func (a *Agent) lastUpgraded() string {
	found := ""
	for _, line := range strings.Split(a.readTrimmed("var/log/unattended-upgrades/unattended-upgrades.log"), "\n") {
		if _, rest, ok := strings.Cut(line, "Packages that were upgraded: "); ok {
			found = strings.TrimSpace(rest)
		}
	}
	if len(found) > 300 {
		found = found[:300] + "…"
	}
	return found
}

// Notify writes a notification to the journal, for the bell.
func (Journal) Notify(kind, level, message string) {
	priority := map[string]string{"error": "3", "warning": "4"}[level]
	if priority == "" {
		priority = "5"
	}
	fields := []string{
		"MESSAGE=" + clean(message),
		"PRIORITY=" + priority,
		"SYSLOG_IDENTIFIER=zoomriya-agent",
		"ZOOMRIYA_NOTIFY=1",
		"ZOOMRIYA_KIND=" + kind,
		"ZOOMRIYA_LEVEL=" + level,
	}
	writeJournal(fields, message)
}

type Notification struct {
	ID      string `json:"id"`
	Time    string `json:"time"`
	Kind    string `json:"kind"`
	Level   string `json:"level"`
	Message string `json:"message"`
}

// notifications are the last month's, newest first.
func (a *Agent) notifications() (Response, error) {
	out, err := a.Run("journalctl", "--output=json", "--no-pager", "--quiet", "--reverse",
		"--lines="+strconv.Itoa(notificationsMax), "--since="+notificationsSince, "ZOOMRIYA_NOTIFY=1")
	if err != nil {
		return nil, err
	}
	list := []Notification{}
	for _, line := range bytes.Split(out, []byte("\n")) {
		var fields map[string]json.RawMessage
		if json.Unmarshal(line, &fields) != nil {
			continue
		}
		e := logEntry(fields)
		list = append(list, Notification{
			ID:      e.Cursor,
			Time:    e.Time,
			Kind:    journalField(fields["ZOOMRIYA_KIND"]),
			Level:   journalField(fields["ZOOMRIYA_LEVEL"]),
			Message: e.Message,
		})
	}
	return Response{"notifications": list, "kinds": notificationKinds}, nil
}
