package main

import (
	"strings"
	"testing"
)

const libraryPath = "/data/zoomriya/volumes/6b1f0c2e"

func libraryBox(t *testing.T, scrub string) (*Agent, *fakeBox) {
	agent, box := newFakeBox(t)
	box.answers = map[string]string{
		"findmnt --json --list --types btrfs --output TARGET":       `{"filesystems": [{"target": "/"}, {"target": "/data"}, {"target": "` + libraryPath + `"}]}`,
		"df --block-size=1 --output=size,used,avail " + libraryPath: "1B-blocks Used Avail\n8000000000000 2100000000000 5900000000000\n",
		"df --block-size=1 --output=size,used,avail /":              "1B-blocks Used Avail\n1000000000000 300000000000 700000000000\n",
		"btrfs filesystem df " + libraryPath:                        "Data, RAID1: total=2.00TiB, used=1.91TiB\nMetadata, RAID1: total=8.00GiB, used=4.10GiB\n",
		"btrfs filesystem show " + libraryPath: "Label: none  uuid: 6b1f0c2e\n\tTotal devices 2 FS bytes used 1.91TiB\n" +
			"\tdevid    1 size 7.28TiB used 2.01TiB path /dev/sda\n\tdevid    2 size 7.28TiB used 2.01TiB path /dev/sdb\n",
		"btrfs device stats " + libraryPath:                            "[/dev/sda].write_io_errs    0\n[/dev/sda].corruption_errs  0\n[/dev/sdb].write_io_errs    0\n[/dev/sdb].corruption_errs  2\n",
		"btrfs scrub status " + libraryPath:                            scrub,
		"du --summarize --bytes --one-file-system /var/lib/postgresql": "48000000000\t/var/lib/postgresql\n",
		"du --summarize --bytes --one-file-system /var/lib/machines":   "30000000000\t/var/lib/machines\n",
		"du --summarize --bytes --one-file-system /data/models":        "150000000000\t/data/models\n",
		"du --summarize --bytes --one-file-system /data/maps":          "40000000000\t/data/maps\n",
		"du --summarize --bytes --one-file-system /data":               "250000000000\t/data\n",
	}
	return agent, box
}

const scrubRunning = `UUID:             6b1f0c2e
Scrub started:    Tue Nov  3 02:00:00 2026
Status:           running
Duration:         1:12:40
Total to scrub:   3.82TiB
Bytes scrubbed:   1.60TiB  (41.90%)
Rate:             384.70MiB/s
Error summary:    no errors found
`

func TestStorage(t *testing.T) {
	agent, _ := libraryBox(t, scrubRunning)
	resp, err := agent.Handle(Request{Op: "storage"})
	if err != nil {
		t.Fatal(err)
	}
	lib := resp["library"].(*Library)
	if lib.Path != libraryPath || !lib.Mirror || lib.UsedBytes != 2100000000000 || lib.FreeBytes != 5900000000000 {
		t.Errorf("library = %+v", lib)
	}
	if len(lib.Drives) != 2 || lib.Drives[0].Name != "sda" || lib.Drives[1].Path != "/dev/sdb" {
		t.Errorf("drives = %+v", lib.Drives)
	}
	if lib.Errors != 2 {
		t.Errorf("errors = %d, want 2", lib.Errors)
	}
	if c := lib.Check; !c.Running || c.Percent != 41.9 || c.Started != "Tue Nov  3 02:00:00 2026" || c.Errors != 0 {
		t.Errorf("check = %+v", c)
	}

	parts := map[string]int64{}
	for _, p := range resp["system_parts"].([]Part) {
		parts[p.Kind] = p.Bytes
	}
	want := map[string]int64{"databases": 48e9, "app-images": 30e9, "models-and-maps": 190e9, "app-data": 60e9, "system": 300e9 - 328e9}
	want["system"] = 0 // more counted than df says is used: never below zero
	for k, v := range want {
		if parts[k] != v {
			t.Errorf("%s = %d, want %d", k, parts[k], v)
		}
	}
}

func TestStoragePartsAreCached(t *testing.T) {
	agent, box := libraryBox(t, scrubRunning)
	agent.Handle(Request{Op: "storage"})
	before := len(ran(box, "du"))
	agent.Handle(Request{Op: "storage"})
	if after := len(ran(box, "du")); after != before {
		t.Errorf("du ran again within ten minutes: %d then %d", before, after)
	}
}

func TestNoLibraryYet(t *testing.T) {
	agent, box := libraryBox(t, "")
	box.answers["findmnt --json --list --types btrfs --output TARGET"] = `{"filesystems": [{"target": "/"}, {"target": "/data"}]}`
	resp, err := agent.Handle(Request{Op: "storage"})
	if err != nil {
		t.Fatal(err)
	}
	if lib := resp["library"].(*Library); lib != nil {
		t.Errorf("library = %+v, want none", lib)
	}
	if _, err := agent.Handle(Request{Op: "storage-check", Action: str("start")}); err == nil {
		t.Error("checked a library that isn't there")
	}
}

func TestScrubStates(t *testing.T) {
	cases := map[string]Check{
		"UUID: x\n\tno stats available\n":                                                      {NeverRun: true},
		"Status:           finished\nError summary:    no errors found\n":                      {Finished: true, Percent: 100},
		"Status:           interrupted\nBytes scrubbed:   1.00TiB  (26.00%)\n":                 {Paused: true, Percent: 26},
		"Status:           finished\nError summary:    csum=3 verify=1\n  Corrected:      4\n": {Finished: true, Percent: 100, Errors: 4},
	}
	for in, want := range cases {
		if got := parseScrub(in); got != want {
			t.Errorf("%q: %+v, want %+v", in, got, want)
		}
	}
}

func TestMissingDrive(t *testing.T) {
	drives := parseDevices("\tdevid    1 size 7.28TiB used 2.01TiB path /dev/sda\n\tdevid    2 size 7.28TiB used 2.01TiB path <missing disk #2>\n\t*** Some devices missing\n")
	if len(drives) != 2 || drives[0].Name != "sda" || !drives[1].Missing {
		t.Errorf("drives = %+v", drives)
	}
}

func TestStorageCheck(t *testing.T) {
	unit := "systemd-run --quiet --collect --unit=zoomriya-storage-check --description=Check that both copies of the library match --property=ExecStopPost=/usr/lib/zoomriya-agent/zoomriya-agent watch -- "
	for action, want := range map[string]string{
		"start":  unit + "btrfs scrub start -B " + libraryPath,
		"pause":  "btrfs scrub cancel " + libraryPath,
		"resume": unit + "btrfs scrub resume -B " + libraryPath,
	} {
		agent, box := libraryBox(t, scrubRunning)
		if _, err := agent.Handle(Request{Op: "storage-check", Action: str(action)}); err != nil {
			t.Fatal(err)
		}
		if got := lastRun(box); got != want {
			t.Errorf("%s ran %q", action, got)
		}
	}
	agent, _ := libraryBox(t, scrubRunning)
	if _, err := agent.Handle(Request{Op: "storage-check", Action: str("rm")}); err == nil || !strings.Contains(err.Error(), "Start, pause or resume") {
		t.Errorf("unknown action: %v", err)
	}
}
