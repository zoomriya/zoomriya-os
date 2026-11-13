package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type notified struct{ kind, level, message string }

type notes struct{ list []notified }

func (n *notes) Notify(kind, level, message string) {
	n.list = append(n.list, notified{kind, level, message})
}

func TestWatch(t *testing.T) {
	agent, box := newFakeBox(t)
	agent.Now = func() time.Time { return time.Date(2026, 11, 3, 10, 0, 0, 0, time.Local) }
	box.lsblk = lsblkDrives
	box.smart = map[string]string{"/dev/vda": noSmart, "/dev/nvme0n1": nvmeHealthy, "/dev/sda": sataHealthy}
	box.write("var/log/unattended-upgrades/unattended-upgrades-dpkg.log", "x\n")

	// The first look: healthy drives and the update log are only noted.
	n := &notes{}
	if err := agent.watch(n); err != nil {
		t.Fatal(err)
	}
	if len(n.list) != 0 {
		t.Fatalf("first look: %+v", n.list)
	}
	if n.list = nil; agent.watch(n) != nil || len(n.list) != 0 {
		t.Fatalf("nothing changed: %+v", n.list)
	}

	// A drive gets worse, an update installs and needs a restart.
	box.smart["/dev/sda"] = strings.Replace(sataHealthy, `{"id": 5, "raw": {"value": 0}}`, `{"id": 5, "raw": {"value": 8}}`, 1)
	box.write("var/log/unattended-upgrades/unattended-upgrades.log", "2026-11-03 04:12:00 INFO Packages that were upgraded: libc6 openssl\n")
	box.write("var/log/unattended-upgrades/unattended-upgrades-dpkg.log", "y\n")
	future := time.Now().Add(time.Minute)
	_ = touch(box, "var/log/unattended-upgrades/unattended-upgrades-dpkg.log", future)
	box.write("run/reboot-required.pkgs", "libc6\n")
	if err := agent.watch(n); err != nil {
		t.Fatal(err)
	}
	kinds := map[string]notified{}
	for _, x := range n.list {
		kinds[x.kind] = x
	}
	if d := kinds["drive"]; d.level != "warning" || !strings.Contains(d.message, "ST8000VN004 should be checked soon") {
		t.Errorf("drive: %+v", d)
	}
	if u := kinds["update"]; u.message != "Security fixes were installed: libc6 openssl." {
		t.Errorf("update: %+v", u)
	}
	if r := kinds["restart"]; !strings.Contains(r.message, "restart by itself at 03:30 to finish: update: libc6") {
		t.Errorf("restart: %+v", r)
	}
	// Told once.
	if n.list = nil; agent.watch(n) != nil || len(n.list) != 0 {
		t.Errorf("told again: %+v", n.list)
	}
}

func TestNotifications(t *testing.T) {
	agent, box := newFakeBox(t)
	box.journal = `{"__CURSOR":"s=n;i=1","__REALTIME_TIMESTAMP":"1793700000000000","PRIORITY":"4","MESSAGE":"Drive should be checked soon.","ZOOMRIYA_KIND":"drive","ZOOMRIYA_LEVEL":"warning"}` + "\n"
	resp, err := agent.Handle(Request{Op: "notifications"})
	if err != nil {
		t.Fatal(err)
	}
	list := resp["notifications"].([]Notification)
	if len(list) != 1 || list[0].Kind != "drive" || list[0].Level != "warning" || list[0].ID != "s=n;i=1" {
		t.Errorf("notifications = %+v", list)
	}
	if args := strings.Join(journalArgs(box), " "); !strings.Contains(args, "ZOOMRIYA_NOTIFY=1") || !strings.Contains(args, "--since=-30d") {
		t.Errorf("journalctl %s", args)
	}
}

func touch(box *fakeBox, rel string, at time.Time) error {
	return os.Chtimes(filepath.Join(box.root, rel), at, at)
}
