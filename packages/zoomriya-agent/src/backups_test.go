package main

import (
	"reflect"
	"testing"
)

const backupShow = "systemctl show " + backupService + " --timestamp=unix -p "

func backupBox(t *testing.T) (*Agent, *fakeBox) {
	agent, box := newFakeBox(t)
	box.write(backupEnv, "RESTIC_REPOSITORY=rest:https://family:s3cret@nas.home:8000/box\nRESTIC_PASSWORD_FILE=/etc/zoomriya/restic.password\n")
	box.answers = map[string]string{
		backupShow + "LoadState": "LoadState=loaded\n",
		backupShow + "ActiveState,Result,ExecMainStartTimestamp,ExecMainExitTimestamp,ExecMainStatus,InvocationID": "ActiveState=inactive\nResult=exit-code\nExecMainStartTimestamp=@1794283200\n" +
			"ExecMainExitTimestamp=@1794286800\nExecMainStatus=1\nInvocationID=0123456789abcdef0123456789abcdef\n",
		"systemctl is-enabled " + backupTimer:                                           "enabled\n",
		"systemctl show " + backupTimer + " --timestamp=unix -p NextElapseUSecRealtime": "NextElapseUSecRealtime=@1794369600\n",
	}
	box.journal = "[backup 2026-11-10T04:00:01+00:00] photos: snapshot\n" +
		"[backup 2026-11-10T04:20:00+00:00] ERROR: photos: /data/apps/photos/originals can't be read\n" +
		"[backup 2026-11-10T04:21:00+00:00] done with failures\n"
	return agent, box
}

func TestBackupsState(t *testing.T) {
	agent, _ := backupBox(t)
	resp, err := agent.Handle(Request{Op: "backups"})
	if err != nil {
		t.Fatal(err)
	}
	want := Response{
		"set_up": true, "running": false, "nightly": true, "where": "rest: nas.home",
		"last_start": "2026-11-10T04:00:00Z", "last_end": "2026-11-10T05:00:00Z", "last_ok": false,
		"problems": []string{"photos: /data/apps/photos/originals can't be read"},
		"next":     "2026-11-11T04:00:00Z",
	}
	if !reflect.DeepEqual(resp, want) {
		t.Errorf("got  %v\nwant %v", resp, want)
	}
}

func TestBackupsNeverShowThePassword(t *testing.T) {
	for repo, want := range map[string]string{
		"/data/zoomriya/volumes/backup/zoomriya-backup": "/data/zoomriya/volumes/backup/zoomriya-backup",
		"rest:https://user:pass@host.example:8000/x":    "rest: host.example",
		"sftp:anna:pw@nas.home:/srv/restic":             "sftp: nas.home",
		"s3:s3.amazonaws.com/bucket":                    "s3: s3.amazonaws.com",
		"nonsense":                                      "",
	} {
		if got := backupPlace(repo); got != want {
			t.Errorf("%s: %q, want %q", repo, got, want)
		}
	}
}

func TestBackupsNotSetUp(t *testing.T) {
	agent, _ := newFakeBox(t)
	resp, err := agent.Handle(Request{Op: "backups"})
	if err != nil || resp["set_up"] != false || len(resp) != 1 {
		t.Errorf("%v %v", resp, err)
	}
	if _, err := agent.Handle(Request{Op: "backups", Action: str("now")}); err == nil {
		t.Error("a backup started on a box without them")
	}
}

func TestBackupNowAndNightly(t *testing.T) {
	agent, box := backupBox(t)
	if _, err := agent.Handle(Request{Op: "backups", Action: str("now")}); err != nil {
		t.Fatal(err)
	}
	if !ranCommand(box, "systemctl start --no-block "+backupService) {
		t.Errorf("ran %v", box.ran)
	}
	off := false
	if _, err := agent.Handle(Request{Op: "backups", Action: str("nightly"), On: &off}); err != nil {
		t.Fatal(err)
	}
	if !ranCommand(box, "systemctl disable --now "+backupTimer) {
		t.Errorf("ran %v", box.ran)
	}
	if _, err := agent.Handle(Request{Op: "backups", Action: str("nightly")}); err == nil {
		t.Error("nightly without on or off")
	}
	if _, err := agent.Handle(Request{Op: "backups", Action: str("restore")}); err == nil {
		t.Error("an unknown action was done")
	}
}
