package main

import "testing"

func TestOutboundGroupsByWhoAndWhere(t *testing.T) {
	agent, box := newFakeBox(t)
	box.write("etc/passwd", "root:x:0:0::/root:/bin/sh\n_apt:x:42:65534::/nonexistent:/usr/sbin/nologin\nzoomriya-maps:x:990:990::/:/usr/sbin/nologin\n")
	box.journal = `{"MESSAGE":"zoomriya-out: IN= OUT=enp1s0 SRC=192.168.1.20 DST=151.101.2.132 LEN=60 PROTO=TCP SPT=40112 DPT=80 UID=42 GID=65534","__REALTIME_TIMESTAMP":"1794200000000000"}
{"MESSAGE":"zoomriya-out: IN= OUT=enp1s0 SRC=192.168.1.20 DST=151.101.2.132 LEN=60 PROTO=TCP SPT=40114 DPT=80 UID=42 GID=65534","__REALTIME_TIMESTAMP":"1794200100000000"}
{"MESSAGE":"zoomriya-out: IN= OUT=enp1s0 SRC=192.168.1.20 DST=2606:4700::1 LEN=80 PROTO=TCP SPT=40200 DPT=443 UID=990 GID=990","__REALTIME_TIMESTAMP":"1794100000000000"}
{"MESSAGE":"zoomriya-out: IN= OUT=enp1s0 SRC=192.168.1.20 DST=9.9.9.9 LEN=60 PROTO=UDP SPT=5000 DPT=53 UID=1234 GID=1234","__REALTIME_TIMESTAMP":"1794000000000000"}
{"MESSAGE":"something else entirely"}
`
	resp, err := agent.Handle(Request{Op: "outbound"})
	if err != nil {
		t.Fatal(err)
	}
	rows := resp["connections"].([]Outbound)
	if len(rows) != 3 {
		t.Fatalf("rows = %+v", rows)
	}
	apt := rows[0]
	if apt.Who != "OS updates" || apt.Destination != "151.101.2.132" || apt.Port != 80 || apt.Count != 2 || apt.Last != "2026-11-09T04:55:00Z" {
		t.Errorf("apt = %+v", apt)
	}
	if rows[1].Who != "Street map" || rows[1].Protocol != "tcp" || rows[1].Port != 443 {
		t.Errorf("maps = %+v", rows[1])
	}
	if rows[2].Who != "uid 1234" || rows[2].Protocol != "udp" {
		t.Errorf("unknown = %+v", rows[2])
	}
}

func TestOutboundWithNothingLogged(t *testing.T) {
	agent, box := newFakeBox(t)
	box.journal = ""
	resp, err := agent.Handle(Request{Op: "outbound"})
	if err != nil || len(resp["connections"].([]Outbound)) != 0 || resp["days"] != outboundDays {
		t.Fatalf("resp = %v, %v", resp, err)
	}
}
