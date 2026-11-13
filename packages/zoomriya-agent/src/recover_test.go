package main

import (
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestAdminRecover(t *testing.T) {
	agent, box := newFakeBox(t)
	now := time.Date(2026, 11, 10, 9, 0, 0, 0, time.UTC)
	agent.Now = func() time.Time { return now }
	n := &notes{}
	agent.Notes = n
	box.write(setupCodeFile, "K7QM-4XRT\n")
	var input string
	var command []string
	agent.RunInput = func(in, name string, args ...string) ([]byte, error) {
		input, command = in, append([]string{name}, args...)
		return nil, nil
	}
	ask := func(code string) error {
		_, err := agent.Handle(Request{Op: "admin-recover", SetupCode: str(code), Username: str(" amma "),
			Password: str("a new long one")})
		return err
	}

	// Not before the box is set up.
	if err := ask("K7QM-4XRT"); err == nil || !strings.Contains(err.Error(), "setting up") {
		t.Fatalf("before setup: %v", err)
	}
	box.write(setupDoneFile, "")

	// A wrong code: refused, and the bell hears of it.
	if err := ask("AAAA-BBBB"); err == nil || !strings.Contains(err.Error(), "setup code") || command != nil {
		t.Fatalf("wrong code: %v %v", err, command)
	}
	if len(n.list) != 1 || n.list[0].kind != "signin" {
		t.Fatalf("told: %+v", n.list)
	}

	// The right one, typed as people do: ZOOMRIYA ID's recovery, the password on standard input.
	if err := ask("k7qm 4xrt"); err != nil {
		t.Fatal(err)
	}
	got := strings.Join(command, " ")
	if !strings.HasSuffix(got, "-m app.recover_admin amma") || strings.Contains(got, "new long") || input != "a new long one" {
		t.Errorf("ran %s with %q", got, input)
	}
	if last := n.list[len(n.list)-1]; !strings.Contains(last.message, "amma was let back in") {
		t.Errorf("told: %+v", last)
	}

	// Five wrong codes in an hour, and even the right one waits.
	for i := 0; i < recoverMax; i++ {
		_ = ask("AAAA-BBBB")
	}
	if err := ask("K7QM-4XRT"); err == nil || !strings.Contains(err.Error(), "Too many") {
		t.Fatalf("not held back: %v", err)
	}
	now = now.Add(61 * time.Minute)
	if err := ask("K7QM-4XRT"); err != nil {
		t.Fatalf("an hour later: %v", err)
	}

	// No such admin.
	agent.RunInput = func(string, string, ...string) ([]byte, error) {
		return nil, exec.Command("sh", "-c", "exit 3").Run()
	}
	if err := ask("K7QM-4XRT"); err == nil || !strings.Contains(err.Error(), "no admin called amma") {
		t.Errorf("no such admin: %v", err)
	}
}

func TestSetupShowsACodeMadeHere(t *testing.T) {
	agent, box := newFakeBox(t)
	box.write(setupCodeFile, "K7QM-4XRT\n")
	status := func() any {
		resp, err := agent.Handle(Request{Op: "setup-status"})
		if err != nil {
			t.Fatal(err)
		}
		return resp["setup_code"]
	}
	if got := status(); got != nil {
		t.Errorf("a factory code (on the sticker) is never shown: %v", got)
	}
	box.write(setupCodeMadeHere, "")
	if got := status(); got != "K7QM-4XRT" {
		t.Errorf("made here: %v", got)
	}
	box.write(setupDoneFile, "")
	if got := status(); got != nil {
		t.Errorf("after the setup: %v", got)
	}
}
