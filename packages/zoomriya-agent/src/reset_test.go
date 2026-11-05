package main

import (
	"strings"
	"testing"
)

func lastRun(box *fakeBox) string {
	if len(box.ran) == 0 {
		return ""
	}
	return strings.Join(box.ran[len(box.ran)-1], " ")
}

func TestResetTheSystem(t *testing.T) {
	agent, box := newFakeBox(t)
	resp, err := agent.Handle(Request{Op: "factory-reset", Level: str("system")})
	if err != nil {
		t.Fatal(err)
	}
	if lastRun(box) != restoreSnapshot+" --reset 1" || resp["restart_needed"] != true {
		t.Errorf("ran %q, got %v", lastRun(box), resp)
	}
	status, _ := agent.Handle(Request{Op: "status"})
	if strings.Join(status["restart_reasons"].([]string), ",") != "reset the system" {
		t.Errorf("restart reasons %v", status["restart_reasons"])
	}
}

func TestEraseEverythingNeedsTheSetupCode(t *testing.T) {
	agent, box := newFakeBox(t)
	box.write("efi/zoomriya/setup-code", "MP68-N8VD\n")

	for _, code := range []string{"", "MP68-N8VE", "MP68"} {
		if _, err := agent.Handle(Request{Op: "factory-reset", Level: str("everything"), SetupCode: str(code)}); err == nil {
			t.Errorf("code %q accepted", code)
		}
	}
	if _, err := agent.Handle(Request{Op: "factory-reset", Level: str("everything")}); err == nil {
		t.Error("no code accepted")
	}
	if len(ran(box, restoreSnapshot)) != 0 {
		t.Fatalf("ran %v", box.ran)
	}

	// As people type it: lower case, no dash.
	if _, err := agent.Handle(Request{Op: "factory-reset", Level: str("everything"), SetupCode: str(" mp68n8vd ")}); err != nil {
		t.Fatal(err)
	}
	if lastRun(box) != restoreSnapshot+" --reset --erase-everything 1" {
		t.Errorf("ran %q", lastRun(box))
	}
}

func TestEraseEverythingWithoutASetupCodeOnTheBox(t *testing.T) {
	agent, _ := newFakeBox(t)
	if _, err := agent.Handle(Request{Op: "factory-reset", Level: str("everything"), SetupCode: str("")}); err == nil {
		t.Error("an empty code matched a box with none")
	}
}

func TestResetRefusals(t *testing.T) {
	agent, box := newFakeBox(t)
	for _, level := range []*string{nil, str("factory"), str("")} {
		if _, err := agent.Handle(Request{Op: "factory-reset", Level: level}); err == nil {
			t.Errorf("level %v accepted", level)
		}
	}
	if len(ran(box, restoreSnapshot)) != 0 {
		t.Errorf("ran %v", box.ran)
	}
}
