package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDisplay(t *testing.T) {
	agent, box := newFakeBox(t)
	size, refresh, scale, turn := "1920x1080", 60000, 1.25, "normal"
	set := func(output string) error {
		_, err := agent.Handle(Request{Op: "display-set", Output: str(output), Size: &size, Refresh: &refresh,
			Scale: &scale, Transform: &turn})
		return err
	}
	if err := set("HDMI-A-1"); err != nil {
		t.Fatal(err)
	}
	if err := set("DP-2"); err != nil {
		t.Fatal(err)
	}
	read := func() string {
		data, _ := os.ReadFile(filepath.Join(box.root, "etc/zoomriya/display.conf"))
		return string(data)
	}
	if got := read(); !strings.Contains(got, "output HDMI-A-1 mode 1920x1080@60.000Hz scale 1.25 transform normal\n") ||
		!strings.Contains(got, "output DP-2 ") {
		t.Errorf("got:\n%s", got)
	}

	if _, err := agent.Handle(Request{Op: "display-set", Output: str("DP-2"), Action: str("forget")}); err != nil {
		t.Fatal(err)
	}
	if got := read(); strings.Contains(got, "DP-2") || !strings.Contains(got, "HDMI-A-1") {
		t.Errorf("forgetting DP-2 left:\n%s", got)
	}

	// Nothing but sway's own words reaches the file.
	for _, bad := range []func(){
		func() { size = "1920x1080; exec rm" },
		func() { size, refresh = "1920x1080", 10 },
		func() { refresh, scale = 60000, 3 },
		func() { scale, turn = 1, "sideways" },
	} {
		bad()
		if err := set("HDMI-A-1"); err == nil {
			t.Errorf("took %s %d %g %s", size, refresh, scale, turn)
		}
	}
	turn = "normal"
	if err := set("HDMI-A-1\nexec x"); err == nil {
		t.Error("took a made-up output name")
	}
}
