package main

import (
	"strings"
	"testing"
	"time"
)

func str(s string) *string { return &s }

func ran(box *fakeBox, name string) [][]string {
	var found [][]string
	for _, cmd := range box.ran {
		if cmd[0] == name {
			found = append(found, cmd)
		}
	}
	return found
}

func TestRestartIsATimerSecondsAway(t *testing.T) {
	agent, box := newFakeBox(t)
	if _, err := agent.Handle(Request{Op: "restart"}); err != nil {
		t.Fatal(err)
	}
	cmds := ran(box, "systemd-run")
	if len(cmds) != 1 || !strings.Contains(strings.Join(cmds[0], " "), "--on-active=5s") ||
		cmds[0][len(cmds[0])-1] != "reboot" {
		t.Errorf("ran %v", box.ran)
	}
}

func TestMaintenanceHour(t *testing.T) {
	agent, box := newFakeBox(t)
	resp, _ := agent.Handle(Request{Op: "maintenance-hour"})
	if resp["maintenance_hour"] != "03:30" {
		t.Errorf("default = %v", resp["maintenance_hour"])
	}

	if _, err := agent.Handle(Request{Op: "maintenance-hour", Hour: str("02:15")}); err != nil {
		t.Fatal(err)
	}
	if got := agent.maintenanceHour(); got != "02:15" {
		t.Errorf("after setting = %s", got)
	}
	if len(ran(box, "systemctl")) != 2 {
		t.Errorf("ran %v, want daemon-reload and a timer restart", box.ran)
	}
}

func TestMaintenanceHourRefusals(t *testing.T) {
	agent, box := newFakeBox(t)
	for _, hour := range []string{"24:00", "3:30", "03:60", "03:30\nExecStart=/bin/sh", ""} {
		if _, err := agent.Handle(Request{Op: "maintenance-hour", Hour: str(hour)}); err == nil {
			t.Errorf("%q accepted", hour)
		}
	}
	if len(box.ran) != 0 {
		t.Errorf("ran %v", box.ran)
	}
}

func TestPlannedRestart(t *testing.T) {
	agent, box := newFakeBox(t)
	box.write("run/reboot-required", "")
	box.write("run/reboot-required.pkgs", "libc6\n")
	for now, want := range map[string]string{
		"2026-11-03T22:00:00Z": "2026-11-04T03:30:00Z", // tonight's hour is still to come: tomorrow early
		"2026-11-03T01:00:00Z": "2026-11-03T03:30:00Z", // before the hour: today
		"2026-11-03T03:30:00Z": "2026-11-04T03:30:00Z", // at the hour: the next one
	} {
		at, _ := time.Parse(time.RFC3339, now)
		agent.Now = func() time.Time { return at }
		resp, _ := agent.Handle(Request{Op: "status"})
		if resp["restart_planned"] != want {
			t.Errorf("at %s: planned %v, want %s", now, resp["restart_planned"], want)
		}
	}
}

func TestNoRestartPlannedWhenNoneNeeded(t *testing.T) {
	agent, _ := newFakeBox(t)
	resp, _ := agent.Handle(Request{Op: "status"})
	if resp["restart_planned"] != nil || resp["restart_needed"] != false {
		t.Errorf("got %v", resp)
	}
}

func TestMaintenanceRestartsOnlyWhenNeeded(t *testing.T) {
	agent, box := newFakeBox(t)
	if restarted, err := agent.maintenance(); restarted || err != nil || len(box.ran) != 0 {
		t.Errorf("nothing waiting: restarted=%v err=%v ran %v", restarted, err, box.ran)
	}
	box.write("run/zoomriya/restore-pending", "7\n")
	if restarted, err := agent.maintenance(); !restarted || err != nil {
		t.Errorf("a restore waiting: restarted=%v err=%v", restarted, err)
	}
	if cmds := ran(box, "systemctl"); len(cmds) != 1 || cmds[0][1] != "reboot" {
		t.Errorf("ran %v", box.ran)
	}
}
