package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestTimeZone(t *testing.T) {
	agent, box := newFakeBox(t)
	box.write("usr/share/zoneinfo/America/Toronto", "TZif")
	if err := os.Symlink("/usr/share/zoneinfo/Etc/UTC", filepath.Join(box.root, "etc/localtime")); err != nil {
		t.Fatal(err)
	}
	box.answers = map[string]string{"timedatectl show --property=NTPSynchronized --value": "yes\n"}
	resp, _ := agent.Handle(Request{Op: "setup-status"})
	if resp["zone"] != "Etc/UTC" || resp["clock_synced"] != true || resp["finished"] != false || resp["has_admin"] != false {
		t.Errorf("got %v", resp)
	}
	if _, err := agent.Handle(Request{Op: "time-zone-set", Zone: str("America/Toronto")}); err != nil {
		t.Fatal(err)
	}
	if !ranCommand(box, "timedatectl set-timezone America/Toronto") {
		t.Errorf("didn't set it: %v", box.ran)
	}
	for _, zone := range []string{"", "Mars/Olympus", "../../etc/passwd", "America", "America/Toronto\n"} {
		if _, err := agent.Handle(Request{Op: "time-zone-set", Zone: str(zone)}); err == nil {
			t.Errorf("%q accepted", zone)
		}
	}
}

func TestRegion(t *testing.T) {
	agent, box := newFakeBox(t)
	box.write("usr/share/zoneinfo/iso3166.tab", "# comment\nCA\tCanada\nLK\tSri Lanka\n")
	resp, _ := agent.Handle(Request{Op: "setup-status"})
	if resp["region"] != "" {
		t.Errorf("a region before one was chosen: %v", resp["region"])
	}
	if _, err := agent.Handle(Request{Op: "region-set", Region: str("LK")}); err != nil {
		t.Fatal(err)
	}
	if resp, _ := agent.Handle(Request{Op: "setup-status"}); resp["region"] != "LK" {
		t.Errorf("got %v", resp["region"])
	}
	for _, code := range []string{"", "XX", "ca", "CAN", "C\tA", "../"} {
		if _, err := agent.Handle(Request{Op: "region-set", Region: str(code)}); err == nil {
			t.Errorf("%q accepted", code)
		}
	}
}

func TestFirstAdmin(t *testing.T) {
	agent, _ := newFakeBox(t)
	var input string
	var command []string
	agent.RunInput = func(in, name string, args ...string) ([]byte, error) {
		input, command = in, append([]string{name}, args...)
		return nil, nil
	}
	resp, err := agent.Handle(Request{Op: "first-admin", Username: str(" amma "), Password: str("a long passphrase")})
	if err != nil || resp["username"] != "amma" {
		t.Fatalf("got %v, %v", resp, err)
	}
	if input != "a long passphrase" {
		t.Errorf("password not on standard input: %q", input)
	}
	got := strings.Join(command, " ")
	if !strings.Contains(got, "--property=User=zoomriya-id") || !strings.HasSuffix(got, "-m app.first_admin amma") {
		t.Errorf("ran %s", got)
	}
	if strings.Contains(got, "passphrase") {
		t.Errorf("the password is in the command: %s", got)
	}
}

func TestFirstAdminRefused(t *testing.T) {
	agent, box := newFakeBox(t)
	agent.RunInput = func(string, string, ...string) ([]byte, error) {
		return nil, exec.Command("sh", "-c", "exit 3").Run()
	}
	if _, err := agent.Handle(Request{Op: "first-admin", Username: str("amma"), Password: str("a long passphrase")}); err == nil || !strings.Contains(err.Error(), "already has an admin") {
		t.Errorf("accounts exist: %v", err)
	}
	for _, r := range []Request{
		{Op: "first-admin", Username: str("amma"), Password: str("short")},
		{Op: "first-admin", Username: str("  "), Password: str("a long passphrase")},
		{Op: "first-admin", Username: str("amma"), Password: str("a long\npassphrase")},
		{Op: "first-admin", Password: str("a long passphrase")},
	} {
		if _, err := agent.Handle(r); err == nil {
			t.Errorf("%+v accepted", r)
		}
	}
	box.write(setupDoneFile, "")
	if _, err := agent.Handle(Request{Op: "first-admin", Username: str("amma"), Password: str("a long passphrase")}); err == nil {
		t.Error("accepted after setup")
	}
}

func TestSetupFinish(t *testing.T) {
	agent, box := newFakeBox(t)
	query := "runuser -u postgres -- psql -qtA zoomriya_id -c SELECT count(*) FROM users WHERE role = 'admin' AND is_active"
	box.answers = map[string]string{query: "0\n"}
	if _, err := agent.Handle(Request{Op: "setup-finish"}); err == nil {
		t.Error("finished with no admin")
	}
	box.answers[query] = "1\n"
	if _, err := agent.Handle(Request{Op: "setup-finish"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(box.root, setupDoneFile)); err != nil {
		t.Errorf("not marked: %v", err)
	}
	if _, err := agent.Handle(Request{Op: "setup-finish"}); err == nil {
		t.Error("finished twice")
	}
}

func TestFirstAdminPasswordNeverLogged(t *testing.T) {
	got := detail(Request{Op: "first-admin", Username: str("amma"), Password: str("a long passphrase")})
	if got != "amma" {
		t.Errorf("logged %q", got)
	}
}
