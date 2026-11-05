package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompareVersions(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want int
	}{
		{"2.41-12+deb13u1", "2.41-12", 1},
		{"2.41-12+deb13u1", "2.41-12+deb13u2", -1},
		{"1.0~rc1", "1.0", -1},
		{"1.0", "1.0+b1", -1},
		{"1:1.0", "2.0", 1},
		{"1.10", "1.9", 1},
		{"1.0-1", "1.0-1", 0},
		{"6.12.115+deb13.1", "6.12.116+deb13.1", -1},
		{"3.0.17-1~deb13u1", "3.0.17-1", -1},
		{"1.0a", "1.0", 1},
		{"001", "1", 0},
	} {
		got := compareVersions(c.a, c.b)
		if got > 0 {
			got = 1
		} else if got < 0 {
			got = -1
		}
		if got != c.want {
			t.Errorf("compareVersions(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func heldBox(t *testing.T) (*Agent, *fakeBox) {
	agent, box := newFakeBox(t)
	box.write(heldFile, "libc6:amd64 2.41-12 2.41-12+deb13u1\nopenssl 3.5.1-1 3.5.1-1+deb13u1\n")
	return agent, box
}

func TestHeldLists(t *testing.T) {
	agent, _ := heldBox(t)
	resp, err := agent.Handle(Request{Op: "held"})
	if err != nil {
		t.Fatal(err)
	}
	held := resp["held"].([]Held)
	if len(held) != 2 || held[0] != (Held{"libc6:amd64", "2.41-12", "2.41-12+deb13u1"}) {
		t.Errorf("held = %+v", held)
	}

	agent, _ = newFakeBox(t)
	resp, _ = agent.Handle(Request{Op: "held"})
	if len(resp["held"].([]Held)) != 0 {
		t.Errorf("held without a restore = %v", resp["held"])
	}
}

func TestHeldRelease(t *testing.T) {
	agent, box := heldBox(t)
	if _, err := agent.Handle(Request{Op: "held-release", Package: str("bash")}); err == nil {
		t.Error("a package that isn't held was let go")
	}
	if _, err := agent.Handle(Request{Op: "held-release"}); err == nil {
		t.Error("no package accepted")
	}
	if len(ran(box, "apt-mark")) != 0 {
		t.Fatalf("ran %v", box.ran)
	}

	if _, err := agent.Handle(Request{Op: "held-release", Package: str("openssl")}); err != nil {
		t.Fatal(err)
	}
	if lastRun(box) != "apt-mark unhold openssl" {
		t.Errorf("ran %q", lastRun(box))
	}
	if got := agent.heldPackages(); len(got) != 1 || got[0].Package != "libc6:amd64" {
		t.Errorf("still held %+v", got)
	}

	if _, err := agent.Handle(Request{Op: "held-release", Package: str("libc6:amd64")}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(box.root, heldFile)); !os.IsNotExist(err) {
		t.Errorf("the list is still there: %v", err)
	}
}

func TestReleaseFixed(t *testing.T) {
	agent, box := heldBox(t)
	box.answers = map[string]string{
		// libc6: the security archive still has only the version taken back.
		"apt-cache madison libc6:amd64": "     libc6 | 2.41-12+deb13u1 | http://security.debian.org/debian-security stable-security/main amd64 Packages\n" +
			"     libc6 | 2.41-12 | http://deb.debian.org/debian stable/main amd64 Packages\n",
		// openssl: a newer fix, in the security archive.
		"apt-cache madison openssl": "   openssl | 3.5.1-1+deb13u2 | http://security.debian.org/debian-security stable-security/main amd64 Packages\n" +
			"   openssl | 3.5.1-1 | http://deb.debian.org/debian stable/main amd64 Packages\n",
	}
	released, err := agent.releaseFixed()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(released, ",") != "openssl" {
		t.Errorf("released %v", released)
	}
	if got := agent.heldPackages(); len(got) != 1 || got[0].Package != "libc6:amd64" {
		t.Errorf("still held %+v", got)
	}
}

func TestReleaseFixedIgnoresOtherArchives(t *testing.T) {
	agent, box := heldBox(t)
	// A newer version outside the security archive doesn't count: unattended
	// upgrades would still pick the bad one from the security archive.
	box.answers = map[string]string{
		"apt-cache madison openssl": "   openssl | 3.5.2-1 | http://deb.debian.org/debian stable-updates/main amd64 Packages\n" +
			"   openssl | 3.5.1-1+deb13u1 | http://security.debian.org/debian-security stable-security/main amd64 Packages\n",
	}
	released, _ := agent.releaseFixed()
	if len(released) != 0 {
		t.Errorf("released %v", released)
	}
}
