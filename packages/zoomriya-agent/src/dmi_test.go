package main

import (
	"strings"
	"testing"
)

func TestModelAndSerial(t *testing.T) {
	agent, box := newFakeBox(t)
	box.write(dmiDir+"/sys_vendor", "ASUSTeK COMPUTER INC.\n")
	box.write(dmiDir+"/product_name", "NUC14RVH\n")
	box.write(dmiDir+"/product_serial", "G6RVH40712\n")
	resp, err := agent.Handle(Request{Op: "status"})
	if err != nil || resp["model"] != "ASUSTeK COMPUTER INC. NUC14RVH" || resp["serial"] != "G6RVH40712" {
		t.Errorf("got %v %v / %v", resp["model"], resp["serial"], err)
	}
	box.write(dmiDir+"/product_serial", "To be filled by O.E.M.\n")
	box.write(dmiDir+"/product_name", "Default string\n")
	resp, _ = agent.Handle(Request{Op: "status"})
	if resp["serial"] != "" || resp["model"] != "ASUSTeK COMPUTER INC." {
		t.Errorf("placeholders shown: %v %v", resp["model"], resp["serial"])
	}
}

func TestShutdown(t *testing.T) {
	agent, box := newFakeBox(t)
	if _, err := agent.Handle(Request{Op: "shutdown"}); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, cmd := range box.ran {
		found = found || strings.Contains(strings.Join(cmd, " "), "systemctl poweroff")
	}
	if !found {
		t.Errorf("didn't power off: %v", box.ran)
	}
}

func TestSignInsLog(t *testing.T) {
	agent, box := newFakeBox(t)
	if _, err := agent.Handle(Request{Op: "logs", Kind: str("signins")}); err != nil {
		t.Fatal(err)
	}
	last := strings.Join(box.ran[len(box.ran)-1], " ")
	if !strings.Contains(last, "SYSLOG_IDENTIFIER=box-desktop") || !strings.Contains(last, "ZOOMRIYA_SIGNIN=1") {
		t.Errorf("ran %s", last)
	}
}
