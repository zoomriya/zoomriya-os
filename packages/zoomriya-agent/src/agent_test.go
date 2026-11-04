// The agent without root: a folder of fake files stands in for the box, and a
// fake runner for snapper, dpkg-query and restore-snapshot.
package main

import (
	"bufio"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

const snapperJSON = `{"root": [
  {"subvolume": "/", "number": 0, "type": "single", "date": "", "description": "current", "userdata": null},
  {"subvolume": "/", "number": 1, "type": "single", "date": "2026-11-02 21:00:00", "description": "factory", "userdata": {"important": "yes"}},
  {"subvolume": "/", "number": 2, "type": "pre", "date": "2026-11-03 04:12:00", "description": "apt", "userdata": null},
  {"subvolume": "/", "number": 3, "type": "post", "date": "2026-11-03 04:12:30", "description": "apt", "userdata": null},
  {"subvolume": "/", "number": 4, "type": "single", "date": "2026-11-03 05:00:00", "description": "Before restoring 2", "userdata": null}
]}`

type fakeBox struct {
	t    *testing.T
	root string
	ran  [][]string
	// What lsblk lists, and smartctl's report for each drive by path.
	lsblk string
	smart map[string]string
	// Other programs' answers, by their whole command line.
	answers map[string]string
	// What journalctl prints, whatever it's asked.
	journal string
}

func newFakeBox(t *testing.T) (*Agent, *fakeBox) {
	t.Helper()
	box := &fakeBox{t: t, root: t.TempDir()}
	box.write("usr/lib/os-release", "NAME=\"ZOOMRIYA OS\"\nPRETTY_NAME=\"ZOOMRIYA OS 13\"\nIMAGE_VERSION=2026.11.3\n")
	box.write("etc/debian_version", "13.2\n")
	box.write("proc/uptime", "93784.21 180000.00\n")
	box.write("proc/sys/kernel/osrelease", "6.12.115+deb13.1-amd64\n")
	agent := &Agent{Root: box.root, Run: box.run}
	return agent, box
}

func (b *fakeBox) write(rel, content string) {
	b.t.Helper()
	path := filepath.Join(b.root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		b.t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		b.t.Fatal(err)
	}
}

func (b *fakeBox) run(name string, args ...string) ([]byte, error) {
	b.ran = append(b.ran, append([]string{name}, args...))
	switch name {
	case "snapper":
		return []byte(snapperJSON), nil
	case "dpkg-query":
		return []byte("13.0.0"), nil
	case "lsblk":
		if args[len(args)-1] == "/dev/disk/by-partlabel/system" {
			return []byte("vda\n"), nil
		}
		return []byte(b.lsblk), nil
	case "smartctl":
		return []byte(b.smart[args[len(args)-1]]), nil
	case "journalctl":
		return []byte(b.journal), nil
	}
	if out, ok := b.answers[strings.Join(append([]string{name}, args...), " ")]; ok {
		return []byte(out), nil
	}
	return nil, nil
}

func number(n int) *int { return &n }

func TestStatus(t *testing.T) {
	agent, box := newFakeBox(t)
	box.write("run/reboot-required", "*** System restart required ***\n")
	box.write("run/reboot-required.pkgs", "linux-image-6.12.116+deb13.1-amd64\nlibc6\n")

	resp, err := agent.Handle(Request{Op: "status"})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"os":             "ZOOMRIYA OS 13",
		"image_version":  "2026.11.3",
		"debian":         "13.2",
		"base":           "13.0.0",
		"kernel":         "6.12.115+deb13.1-amd64",
		"uptime_seconds": int64(93784),
		"restart_needed": true,
	}
	for key, value := range want {
		if resp[key] != value {
			t.Errorf("%s = %v, want %v", key, resp[key], value)
		}
	}
	reasons := resp["restart_reasons"].([]string)
	if strings.Join(reasons, ",") != "update: linux-image-6.12.116+deb13.1-amd64,update: libc6" {
		t.Errorf("restart_reasons = %v", reasons)
	}
	if resp["last_security_update"] != nil {
		t.Errorf("last_security_update = %v, want nil before any update", resp["last_security_update"])
	}
	if points := resp["restore_points"].([]RestorePoint); len(points) != 4 || points[0].Number != 4 {
		t.Errorf("restore_points = %+v", points)
	}
}

func TestStatusNoRestartNeeded(t *testing.T) {
	agent, _ := newFakeBox(t)
	resp, err := agent.Handle(Request{Op: "status"})
	if err != nil {
		t.Fatal(err)
	}
	if resp["restart_needed"] != false {
		t.Errorf("restart_needed = %v, want false", resp["restart_needed"])
	}
}

func TestRestorePointList(t *testing.T) {
	agent, _ := newFakeBox(t)
	resp, err := agent.Handle(Request{Op: "restore-point-list"})
	if err != nil {
		t.Fatal(err)
	}
	points := resp["restore_points"].([]RestorePoint)
	got := []string{}
	for _, p := range points {
		got = append(got, p.Kind)
	}
	if strings.Join(got, ",") != "before-restore,after-update,before-update,factory" {
		t.Errorf("kinds, newest first = %v", got)
	}
	if !points[3].Important || points[0].Important {
		t.Errorf("only the factory point is important: %+v", points)
	}
}

func TestRestore(t *testing.T) {
	agent, box := newFakeBox(t)
	resp, err := agent.Handle(Request{Op: "restore-point-restore", Number: number(2)})
	if err != nil {
		t.Fatal(err)
	}
	if resp["restart_needed"] != true {
		t.Errorf("restart_needed = %v", resp["restart_needed"])
	}
	last := box.ran[len(box.ran)-1]
	if strings.Join(last, " ") != restoreSnapshot+" 2" {
		t.Errorf("ran %v", last)
	}

	status, _ := agent.Handle(Request{Op: "status"})
	if status["restart_needed"] != true || !strings.Contains(strings.Join(status["restart_reasons"].([]string), ","), "restore point 2") {
		t.Errorf("status after restore = %v", status)
	}
}

func TestRestoreRefusals(t *testing.T) {
	agent, box := newFakeBox(t)
	for name, n := range map[string]*int{"none given": nil, "the running OS": number(0), "unknown": number(99)} {
		if _, err := agent.Handle(Request{Op: "restore-point-restore", Number: n}); err == nil {
			t.Errorf("%s: restored", name)
		}
	}
	for _, cmd := range box.ran {
		if cmd[0] == restoreSnapshot {
			t.Errorf("ran %v for a refused request", cmd)
		}
	}
}

func TestUnknownRequest(t *testing.T) {
	agent, _ := newFakeBox(t)
	resp := answer(agent.Handle(Request{Op: "rm -rf /"}))
	if resp["ok"] != false || !strings.Contains(resp["error"].(string), "Unknown request") {
		t.Errorf("got %v", resp)
	}
}

type recorder struct{ entries []AuditEntry }

func (r *recorder) Record(e AuditEntry) { r.entries = append(r.entries, e) }

// askOverSocket sends one line over a real socket pair and reads the answer.
func askOverSocket(t *testing.T, agent *Agent, audit Auditor, allowed func(*syscall.Ucred) bool, line string) map[string]any {
	t.Helper()
	path := filepath.Join(t.TempDir(), "agent.sock")
	listener, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	done := make(chan struct{})
	go func() {
		conn, err := listener.Accept()
		if err == nil {
			serve(agent, audit, conn.(*net.UnixConn), allowed)
		}
		close(done)
	}()

	client, err := net.Dial("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	if _, err := client.Write([]byte(line)); err != nil {
		t.Fatal(err)
	}
	reply, err := bufio.NewReader(client).ReadBytes('\n')
	if err != nil {
		t.Fatal(err)
	}
	<-done
	var resp map[string]any
	if err := json.Unmarshal(reply, &resp); err != nil {
		t.Fatal(err)
	}
	return resp
}

func allowAll(*syscall.Ucred) bool  { return true }
func allowNone(*syscall.Ucred) bool { return false }

func TestServeAudits(t *testing.T) {
	agent, _ := newFakeBox(t)
	audit := &recorder{}
	resp := askOverSocket(t, agent, audit, allowAll, `{"op": "restore-point-restore", "by": "anna", "number": 3}`+"\n")
	if resp["ok"] != true {
		t.Fatalf("got %v", resp)
	}
	if len(audit.entries) != 1 {
		t.Fatalf("audit = %+v", audit.entries)
	}
	e := audit.entries[0]
	if e.Op != "restore-point-restore" || e.By != "anna" || e.Number != "3" || e.Result != "done" || e.PeerUID != os.Getuid() {
		t.Errorf("audit entry = %+v", e)
	}
}

func TestServeRefusesStrangers(t *testing.T) {
	agent, box := newFakeBox(t)
	audit := &recorder{}
	resp := askOverSocket(t, agent, audit, allowNone, `{"op": "status"}`+"\n")
	if resp["ok"] != false || len(box.ran) != 0 {
		t.Errorf("got %v, ran %v", resp, box.ran)
	}
	if len(audit.entries) != 1 || !strings.HasPrefix(audit.entries[0].Result, "refused") {
		t.Errorf("audit = %+v", audit.entries)
	}
}

func TestServeRefusesExtraArguments(t *testing.T) {
	agent, box := newFakeBox(t)
	resp := askOverSocket(t, agent, &recorder{}, allowAll, `{"op": "status", "command": "reboot"}`+"\n")
	if resp["ok"] != false || len(box.ran) != 0 {
		t.Errorf("got %v, ran %v", resp, box.ran)
	}
}

func TestCleanKeepsFieldsOnOneLine(t *testing.T) {
	if got := clean("anna\nPRIORITY=0"); got != "anna PRIORITY=0" {
		t.Errorf("clean = %q", got)
	}
}

func TestReadsAuditedQuietly(t *testing.T) {
	agent, _ := newFakeBox(t)
	audit := &recorder{}
	askOverSocket(t, agent, audit, allowAll, `{"op": "status", "by": "anna"}`+"\n")
	askOverSocket(t, agent, audit, allowAll, `{"op": "restore-point-restore", "by": "anna", "number": 3}`+"\n")
	if len(audit.entries) != 2 || !audit.entries[0].ReadOnly || audit.entries[1].ReadOnly {
		t.Errorf("audit = %+v", audit.entries)
	}
	if priority(true) != "7" || priority(false) != "5" {
		t.Error("priorities")
	}
}
