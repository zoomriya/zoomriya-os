package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const debianKeyboard = `# KEYBOARD CONFIGURATION FILE

# Consult the keyboard(5) manual page.

XKBMODEL="pc105"
XKBLAYOUT="us"
XKBVARIANT=""
XKBOPTIONS=""

BACKSPACE="guess"
`

func TestKeyboard(t *testing.T) {
	agent, box := newFakeBox(t)
	box.write("etc/default/keyboard", debianKeyboard)
	resp, _ := agent.Handle(Request{Op: "keyboard"})
	if resp["layout"] != "us" || len(resp["layouts"].([]Layout)) < 10 {
		t.Errorf("got %v", resp)
	}

	if _, err := agent.Handle(Request{Op: "keyboard-set", Layout: str("ch:fr")}); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(box.root, "etc/default/keyboard"))
	got := string(data)
	for _, want := range []string{`XKBLAYOUT="ch"` + "\n", `XKBVARIANT="fr"` + "\n", `XKBMODEL="pc105"`, `BACKSPACE="guess"`, "# Consult"} {
		if !strings.Contains(got, want) {
			t.Errorf("lacks %q:\n%s", want, got)
		}
	}
	if !ranCommand(box, "setupcon --force --save-only --keyboard-only") {
		t.Errorf("setupcon didn't run: %v", box.ran)
	}
	if resp, _ := agent.Handle(Request{Op: "keyboard"}); resp["layout"] != "ch:fr" {
		t.Errorf("read back %v", resp["layout"])
	}
}

func TestKeyboardRefused(t *testing.T) {
	agent, box := newFakeBox(t)
	box.write("etc/default/keyboard", debianKeyboard)
	for _, layout := range []string{"", "xx", `fr"` + "\nXKBOPTIONS=\"x", "fr:"} {
		if _, err := agent.Handle(Request{Op: "keyboard-set", Layout: str(layout)}); err == nil {
			t.Errorf("%q accepted", layout)
		}
	}
	if _, err := agent.Handle(Request{Op: "keyboard-set"}); err == nil {
		t.Error("no layout accepted")
	}
	if data, _ := os.ReadFile(filepath.Join(box.root, "etc/default/keyboard")); string(data) != debianKeyboard {
		t.Errorf("changed:\n%s", data)
	}
}
