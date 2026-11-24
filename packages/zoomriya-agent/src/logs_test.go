package main

import (
	"slices"
	"strings"
	"testing"
)

const journalLines = `{"__CURSOR":"s=ab;i=2","__REALTIME_TIMESTAMP":"1793700000000000","PRIORITY":"5","SYSLOG_IDENTIFIER":"zoomriya-agent","_SYSTEMD_UNIT":"zoomriya-agent.service","MESSAGE":"m-anna asked for restart: done","ZOOMRIYA_BY":"m-anna","ZOOMRIYA_OP":"restart","ZOOMRIYA_DETAIL":"","ZOOMRIYA_RESULT":"done"}
{"__CURSOR":"s=ab;i=1","__REALTIME_TIMESTAMP":"1793690000000000","PRIORITY":"3","_COMM":"kernel","MESSAGE":[104,105,255]}
`

func journalArgs(box *fakeBox) []string {
	for _, ran := range box.ran {
		if ran[0] == "journalctl" {
			return ran[1:]
		}
	}
	return nil
}

func TestLogsAudit(t *testing.T) {
	agent, box := newFakeBox(t)
	box.journal = journalLines
	resp, err := agent.Handle(Request{Op: "logs", Kind: str("audit")})
	if err != nil {
		t.Fatal(err)
	}
	args := journalArgs(box)
	for _, want := range []string{"--reverse", "--output=json", "--priority=notice", "ZOOMRIYA_AUDIT=1"} {
		if !slices.Contains(args, want) {
			t.Errorf("journalctl %v, missing %s", args, want)
		}
	}
	entries := resp["entries"].([]LogEntry)
	if len(entries) != 2 || resp["more"] != false {
		t.Fatalf("entries = %+v", entries)
	}
	first := entries[0]
	if first.By != "m-anna" || first.Op != "restart" || first.Level != "notice" || first.Source != "zoomriya-agent" ||
		first.Time != "2026-11-03T10:00:00Z" || first.Cursor != "s=ab;i=2" {
		t.Errorf("first = %+v", first)
	}
	// Not valid UTF-8: bytes in the journal's JSON, shown with a mark.
	if second := entries[1]; second.Message != "hi\uFFFD" || second.Source != "kernel" || second.Level != "error" {
		t.Errorf("second = %+v", second)
	}
}

func TestLogsEvents(t *testing.T) {
	agent, box := newFakeBox(t)
	if _, err := agent.Handle(Request{Op: "logs", Kind: str("events"), Cursor: str("s=ab;i=1")}); err != nil {
		t.Fatal(err)
	}
	args := strings.Join(journalArgs(box), " ")
	for _, want := range []string{"--after-cursor=s=ab;i=1", "--priority=info", "_SYSTEMD_UNIT=box-desktop.service + ", "SYSLOG_IDENTIFIER=zoomriya-agent"} {
		if !strings.Contains(args, want) {
			t.Errorf("journalctl %s, missing %s", args, want)
		}
	}
}

func TestLogsSystemSearch(t *testing.T) {
	agent, box := newFakeBox(t)
	if _, err := agent.Handle(Request{Op: "logs", Kind: str("system"), Priority: str("warnings"), Search: str(" eth0 (down) ")}); err != nil {
		t.Fatal(err)
	}
	args := journalArgs(box)
	for _, want := range []string{"--priority=warning", `--grep=eth0 \(down\)`, "--case-sensitive=false"} {
		if !slices.Contains(args, want) {
			t.Errorf("journalctl %v, missing %s", args, want)
		}
	}
}

func TestLogsRefused(t *testing.T) {
	agent, box := newFakeBox(t)
	for name, req := range map[string]Request{
		"no log":          {Op: "logs"},
		"unknown log":     {Op: "logs", Kind: str("secrets")},
		"unknown level":   {Op: "logs", Kind: str("system"), Priority: str("debug")},
		"forged cursor":   {Op: "logs", Kind: str("system"), Cursor: str("s=ab --file=/etc/shadow")},
		"long search":     {Op: "logs", Kind: str("system"), Search: str(strings.Repeat("x", 101))},
		"two-line search": {Op: "logs", Kind: str("system"), Search: str("a\nb")},
	} {
		if _, err := agent.Handle(req); err == nil {
			t.Errorf("%s: answered", name)
		}
	}
	if journalArgs(box) != nil {
		t.Error("journalctl ran for a refused request")
	}
}

func TestLogsPage(t *testing.T) {
	agent, box := newFakeBox(t)
	line := `{"__CURSOR":"s=ab;i=9","__REALTIME_TIMESTAMP":"1793700000000000","PRIORITY":"6","MESSAGE":"x"}` + "\n"
	box.journal = strings.Repeat(line, logPage)
	resp, _ := agent.Handle(Request{Op: "logs", Kind: str("system")})
	if resp["more"] != true || resp["next"] != "s=ab;i=9" {
		t.Errorf("more %v next %v", resp["more"], resp["next"])
	}
}
