package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRedact(t *testing.T) {
	for in, want := range map[string]string{
		`login failed password=hunter22 for amma`:                                      `login failed password=[removed] for amma`,
		`{"token": "abc.def.ghi", "user": "amma"}`:                                     `{"token": "[removed]", "user": "amma"}`,
		`Authorization: Bearer eyJhbGciOiJIUzI1NiJ9.e30.x`:                             `Authorization: Bearer [removed]`,
		"-----BEGIN OPENSSH PRIVATE KEY-----\nb3Bl\n-----END OPENSSH PRIVATE KEY-----": "[private key removed]",
		`setup_code=ABCD-EFGH`:                                                         `setup_code=[removed]`,
		`Started zoomriya-agent.service.`:                                              `Started zoomriya-agent.service.`,
	} {
		if got, _ := redact(in); got != want {
			t.Errorf("redact(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDiagnostics(t *testing.T) {
	agent, box, _ := accessBox(t)
	box.answers = map[string]string{
		"sticks": sticksJSON,
		"systemctl --failed --plain --no-legend --no-pager": "zoomriya-photos.service loaded failed failed Photos\n",
	}
	box.journal = "2026-11-08T09:59:00+0000 box zoomriya-agent[1]: restart by amma\n" +
		"2026-11-08T09:59:01+0000 box app[2]: connecting with token=s3cr3tvalue\n"

	resp, err := agent.Handle(Request{Op: "diagnostics", Action: str("make"), Include: []string{"journal"},
		Note: str("Photos won't upload since Tuesday.")})
	if err != nil {
		t.Fatal(err)
	}
	name := "zoomriya-diagnostics-20261108-100000.tar.gz"
	if resp["made"] != name {
		t.Fatalf("made = %v", resp["made"])
	}
	bundles := resp["bundles"].([]Bundle)
	if len(bundles) != 1 || bundles[0].Errors < 1 {
		t.Errorf("bundles = %+v", bundles)
	}
	if info, err := os.Stat(filepath.Join(box.root, diagnosticsDir, name)); err != nil || info.Mode().Perm() != 0o600 {
		t.Errorf("bundle: %v %v", info, err)
	}

	// Review: the manifest, then a part.
	resp, err = agent.Handle(Request{Op: "diagnostics", Action: str("show"), Name: str(name)})
	if err != nil {
		t.Fatal(err)
	}
	m := resp["manifest"].(Manifest)
	if m.Format != 1 || m.Note != "Photos won't upload since Tuesday." || strings.Join(m.LeftOut, ",") != "network" {
		t.Errorf("manifest = %+v", m)
	}
	if len(m.Checks) == 0 || m.Checks[0].Severity != "error" || m.Checks[0].Summary != "zoomriya-photos.service failed." {
		t.Errorf("checks = %+v", m.Checks)
	}
	files := map[string]BundlePart{}
	for _, p := range m.Parts {
		files[p.File] = p
	}
	for _, want := range []string{"versions.txt", "status.json", "disk-health.json", "failed-units.txt", "journal.txt",
		"previous-boot.txt", "advanced-access.txt", "connect.txt"} {
		if _, ok := files[want]; !ok {
			t.Errorf("no %s; parts %v", want, m.Parts)
		}
	}
	if _, ok := files["network.json"]; ok {
		t.Error("network.json is in, though left out")
	}
	if !files["journal.txt"].Sensitive || m.Redacted < 1 {
		t.Errorf("journal %+v, redacted %d", files["journal.txt"], m.Redacted)
	}
	resp, err = agent.Handle(Request{Op: "diagnostics", Action: str("read"), Name: str(name), Part: str("journal.txt")})
	if err != nil {
		t.Fatal(err)
	}
	text := resp["text"].(string)
	if !strings.Contains(text, "restart by amma") || strings.Contains(text, "s3cr3tvalue") {
		t.Errorf("journal.txt = %q", text)
	}
	for _, bad := range []string{"../../etc/shadow", "missing.txt"} {
		if _, err := agent.Handle(Request{Op: "diagnostics", Action: str("read"), Name: str(name), Part: str(bad)}); err == nil {
			t.Errorf("read %q", bad)
		}
	}
	if _, err := agent.Handle(Request{Op: "diagnostics", Action: str("show"), Name: str("../agent.json")}); err == nil {
		t.Error("showed a file that isn't a bundle")
	}

	// Save to the stick.
	resp, err = agent.Handle(Request{Op: "diagnostics", Action: str("export"), Name: str(name), Device: str("/dev/sdb1")})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(box.root, usbMount, "ZOOMRIYA", name)); err != nil {
		t.Errorf("not on the stick: %v", err)
	}
	if !ranCommand(box, "mount -t vfat -o nosuid,nodev,noexec,noatime /dev/sdb1 "+filepath.Join(box.root, usbMount)) {
		t.Errorf("ran %v", box.ran)
	}
	if _, err := agent.Handle(Request{Op: "diagnostics", Action: str("export"), Name: str(name), Device: str("/dev/vda2")}); err == nil {
		t.Error("saved to the system disk")
	}

	// The list has the sticks.
	resp, _ = agent.Handle(Request{Op: "diagnostics"})
	if sticks := resp["usb"].([]Stick); len(sticks) != 1 {
		t.Errorf("usb = %v", sticks)
	}

	// Only the newest few are kept; delete removes one.
	for i := 1; i <= diagnosticsKeep+1; i++ {
		at := time.Date(2026, 11, 8, 11, i, 0, 0, time.UTC)
		agent.Now = func() time.Time { return at }
		if _, err := agent.Handle(Request{Op: "diagnostics", Action: str("make")}); err != nil {
			t.Fatal(err)
		}
	}
	resp, _ = agent.Handle(Request{Op: "diagnostics"})
	if b := resp["bundles"].([]Bundle); len(b) != diagnosticsKeep || b[0].Name != "zoomriya-diagnostics-20261108-110600.tar.gz" {
		t.Errorf("bundles = %+v", b)
	}
	resp, err = agent.Handle(Request{Op: "diagnostics", Action: str("delete"), Name: str("zoomriya-diagnostics-20261108-110600.tar.gz")})
	if err != nil || len(resp["bundles"].([]Bundle)) != diagnosticsKeep-1 {
		t.Errorf("delete: %v %v", resp, err)
	}
	if _, err := agent.Handle(Request{Op: "diagnostics", Action: str("make"), Include: []string{"photos"}}); err == nil {
		t.Error("made a bundle with a part that doesn't exist")
	}
	data, _ := json.Marshal(m)
	if strings.Contains(string(data), `"line"`) {
		t.Error("the manifest has a key's line")
	}
}
