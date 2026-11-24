// The backup drive (backups.go, backuprun.go, backuprestore.go): a folder
// stands in for the drive (the fake runner's mount does nothing), and
// pg_dump and pg_restore write and read small files.
package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// --- the key and the sealed files ------------------------------------------------------

func TestBackupKeyIsTypedBackTheSame(t *testing.T) {
	key := newBackupKey()
	if len(key) != 29 || strings.Count(key, "-") != 5 {
		t.Fatalf("key %q isn't six groups of four", key)
	}
	raw, err := parseBackupKey(key)
	if err != nil || formatBackupKey(raw) != key {
		t.Fatalf("parse(%q) = %x, %v", key, raw, err)
	}
	// Lower case, spaces, O for 0 and L for 1 read the same.
	typed := strings.ToLower(strings.NewReplacer("-", " ", "0", "O", "1", "l").Replace(key))
	if again, err := parseBackupKey(typed); err != nil || !bytes.Equal(again, raw) {
		t.Errorf("typed %q: %x, %v", typed, again, err)
	}
	for _, bad := range []string{"", "ABCD", strings.Repeat("U", 24)} {
		if _, err := parseBackupKey(bad); err == nil {
			t.Errorf("%q was taken as a key", bad)
		}
	}
}

func TestSealedFilesOpenOnlyWithTheirKeyAndWhole(t *testing.T) {
	key, _ := parseBackupKey(newBackupKey())
	other, _ := parseBackupKey(newBackupKey())
	plain := bytes.Repeat([]byte("family photos and accounts "), 100000) // a few chunks
	sealed, err := sealBytes(key, plain)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(sealed, []byte("family photos")) {
		t.Fatal("the sealed file holds the words in the clear")
	}
	if got, err := openBytes(key, sealed); err != nil || !bytes.Equal(got, plain) {
		t.Fatalf("opened %d bytes, %v", len(got), err)
	}
	if _, err := openBytes(other, sealed); err == nil {
		t.Error("another key opened it")
	}
	changed := append([]byte{}, sealed...)
	changed[len(changed)/2] ^= 1
	if _, err := openBytes(key, changed); err == nil {
		t.Error("a changed file opened")
	}
	// Cut after a whole chunk: still refused, as its last chunk never came.
	cut := sealed[:len(backupMagic)+backupSaltBytes+4+backupChunk+16]
	if _, err := openBytes(key, cut); err == nil {
		t.Error("a file cut short opened")
	}
	empty, _ := sealBytes(key, nil)
	if got, err := openBytes(key, empty); err != nil || len(got) != 0 {
		t.Errorf("an empty file: %q, %v", got, err)
	}
}

func TestLargeFilesAreSplitInParts(t *testing.T) {
	base := filepath.Join(t.TempDir(), "photos.zbak")
	w := newPartWriter(base, 1000)
	data := bytes.Repeat([]byte("0123456789"), 250)
	if _, err := w.Write(data); err != nil {
		t.Fatal(err)
	}
	w.Close()
	if len(w.Parts) != 3 || filepath.Base(w.Parts[2]) != "photos.zbak.003" {
		t.Fatalf("parts %v", w.Parts)
	}
	r, err := openParts(base)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if got, _ := io.ReadAll(r); !bytes.Equal(got, data) {
		t.Error("the parts don't read back as the whole")
	}
}

func TestListedFilesStayInTheirPlaces(t *testing.T) {
	for p, want := range map[string]bool{
		"originals/ab/abcd.jpg": true, "/data/zoomriya/volumes/library/originals/x.jpg": true,
		"../../etc/shadow": false, "/etc/shadow": false, "/data/zoomriya/volumes/": false,
		"/data/zoomriya/volumes/library/../../../etc/shadow": false, "": false,
	} {
		if listedPathValid(p) != want {
			t.Errorf("listedPathValid(%q) = %v", p, !want)
		}
	}
}

func TestDriveNamesAreSafe(t *testing.T) {
	for in, want := range map[string]string{
		"Photos/Priya/2026/2026-11/IMG 1.jpg": "Photos/Priya/2026/2026-11/IMG 1.jpg",
		"Photos/../../etc/passwd":             "Photos/etc/passwd",
		`Vault/Bala: "tax"?/a*b.pdf`:          "Vault/Bala_ _tax__/a_b.pdf",
		"App data/x.zbak.001":                 "_App data/x.zbak.001",
		".zoomriya/copies.json":               "_.zoomriya/copies.json",
		"/":                                   "",
	} {
		if got := drivePath(in); got != want {
			t.Errorf("drivePath(%q) = %q, want %q", in, got, want)
		}
	}
}

// --- a backup and bringing it back ---------------------------------------------------------

const backupLsblk = `{"blockdevices": [
  {"name": "/dev/vda", "type": "disk", "tran": null, "fstype": null, "label": null, "uuid": null, "size": 64000000000, "mountpoints": [null],
   "children": [{"name": "/dev/vda2", "type": "part", "tran": null, "fstype": "btrfs", "label": null, "uuid": "1111", "size": 60000000000, "mountpoints": ["/"]}]},
  {"name": "/dev/sdb", "type": "disk", "tran": "usb", "fstype": null, "label": null, "uuid": null, "size": 2000000000000, "mountpoints": [null],
   "children": [{"name": "/dev/sdb1", "type": "part", "tran": null, "fstype": "exfat", "label": "Family backup", "uuid": "AB12-CD34", "size": 2000000000000, "mountpoints": [null]}]}
]}`

type backupFake struct {
	agent *Agent
	box   *fakeBox
	now   time.Time
	// The databases' contents: pg_dump writes it, pg_restore reads it back.
	databases map[string]string
}

func newBackupFake(t *testing.T) *backupFake {
	agent, box := newFakeBox(t)
	f := &backupFake{agent: agent, box: box, now: time.Date(2026, 11, 22, 4, 0, 0, 0, time.UTC),
		databases: map[string]string{"zoomriya_id": "accounts v1", "photos": "albums v1"}}
	agent.Now = func() time.Time { return f.now }
	agent.Sleep = func(time.Duration) {}
	agent.RunInput = func(input, name string, args ...string) ([]byte, error) {
		box.ran = append(box.ran, append([]string{name}, args...))
		return nil, nil
	}
	agent.Run = func(name string, args ...string) ([]byte, error) {
		line := strings.Join(append([]string{name}, args...), " ")
		switch {
		case name == "lsblk" && strings.Contains(line, "UUID"):
			box.ran = append(box.ran, append([]string{name}, args...))
			return []byte(backupLsblk), nil
		case strings.Contains(line, " pg_dump "):
			box.ran = append(box.ran, append([]string{name}, args...))
			file, db := args[len(args)-2], args[len(args)-1]
			os.MkdirAll(filepath.Dir(file), 0o700)
			return nil, os.WriteFile(file, []byte(f.databases[db]), 0o600)
		case strings.Contains(line, " pg_restore "):
			box.ran = append(box.ran, append([]string{name}, args...))
			file, db := args[len(args)-1], args[len(args)-2]
			data, err := os.ReadFile(file)
			f.databases[db] = string(data)
			return nil, err
		case strings.Contains(line, "pg_get_userbyid"):
			return []byte("owner\n"), nil
		case strings.Contains(line, "json_agg"):
			return []byte(`[{"username": "priya", "name": "Priya"}]`), nil
		case name == "systemctl" && len(args) > 1 && args[0] == "is-active":
			return []byte("active\n"), nil
		}
		return box.run(name, args...)
	}
	box.write(appsState, `{"photos": {"id": "photos", "name": "Family Photos", "active": "1.4.0",
		"releases": {"1.4.0": {"version": "1.4.0", "start": ["zoomriya-photos-api@.service"], "backup_list": "zoomriya-photos-backup-list@.service"}},
		"data": true, "databases": ["photos"]}}`)
	box.write("data/zoomriya-id/signing.key", "the ID's signing key")
	box.write("etc/zoomriya/photos.key", "Photos' box key")
	box.write("data/apps/photos/originals/ab/abcd.jpg", "a photo of Grandma")
	box.write("data/apps/photos/originals/cd/cdef.jpg", "a photo of the lake")
	box.write("data/apps/photos/thumbs/ab/abcd.webp", "small Grandma")
	box.write("data/zoomriya/volumes/library/originals/ef/efgh.jpg", "a photo on the library drive")
	box.write("data/apps/photos/backup/files.jsonl",
		`{"path": "originals/ab/abcd.jpg", "as": "Photos/Priya/2026/2026-11/Grandma.jpg"}`+"\n"+
			`{"path": "originals/cd/cdef.jpg", "as": "Photos/Bala/2025/2025-07/Lake.jpg"}`+"\n"+
			`{"path": "/data/zoomriya/volumes/library/originals/ef/efgh.jpg", "as": "Photos/Bala/2026/2026-01/Snow.jpg"}`+"\n")
	return f
}

func (f *backupFake) drive() string { return filepath.Join(f.box.root, backupMount, backupFolder) }

func (f *backupFake) read(rel string) string {
	data, err := os.ReadFile(filepath.Join(f.box.root, rel))
	if err != nil {
		return ""
	}
	return string(data)
}

func TestBackupToADriveAndBringItBack(t *testing.T) {
	f := newBackupFake(t)
	a := f.agent

	// Choosing the drive makes the key, shown once to print.
	resp, err := a.Handle(Request{Op: "backups", Action: ptr("setup"), Device: ptr("/dev/sdb1"), By: "priya"})
	if err != nil {
		t.Fatal(err)
	}
	key, _ := resp["key"].(string)
	if _, err := parseBackupKey(key); err != nil || strings.TrimSpace(f.read(backupKeyFile)) != key {
		t.Fatalf("key %q, kept %q", key, f.read(backupKeyFile))
	}
	if again, _ := a.Handle(Request{Op: "backups", Action: ptr("key")}); again["key"] != key {
		t.Error("the key shown again isn't the one printed")
	}

	if _, err := a.Handle(Request{Op: "backups", Action: ptr("now"), By: "priya"}); err != nil {
		t.Fatal(err)
	}
	if err := a.runBackup(); err != nil {
		t.Fatal(err)
	}
	last := a.readBackupJob(backupLast)
	if last == nil || !last.OK || last.By != "priya" {
		t.Fatalf("last backup %+v", last)
	}

	// Photos as they are, under the names Photos gave; nothing else plain.
	drive := f.drive()
	if got, _ := os.ReadFile(filepath.Join(drive, "Photos/Priya/2026/2026-11/Grandma.jpg")); string(got) != "a photo of Grandma" {
		t.Errorf("Grandma's photo on the drive: %q", got)
	}
	set := filepath.Join(drive, backupSets, "2026-11-22 0400")
	for _, want := range []string{"backup.json", "accounts.zbak", "id.zbak.001", "photos.zbak.001"} {
		if _, err := os.Stat(filepath.Join(set, want)); err != nil {
			t.Errorf("%s isn't in the backup", want)
		}
	}
	sealed, _ := os.ReadFile(filepath.Join(set, "photos.zbak.001"))
	for _, plain := range []string{"albums v1", "small Grandma", "a photo of", "Photos' box key"} {
		if bytes.Contains(sealed, []byte(plain)) {
			t.Errorf("%q is in the clear in the sealed archive", plain)
		}
	}
	// Private chats stay out: every dump is made with row security on, as a role that isn't a superuser.
	if !ranCommand(f.box, "runuser -u postgres -- env PGOPTIONS=-c zoomriya.backup=on pg_dump -Fc --enable-row-security --role=zoomriya-backup -f "+
		filepath.Join(f.box.root, backupStage, "photos", "photos.dump")+" photos") {
		t.Error("Photos' database wasn't dumped with row security on")
	}

	// A day later: one photo changed and one deleted.
	f.now = f.now.Add(24 * time.Hour)
	f.box.write("data/apps/photos/originals/ab/abcd.jpg", "Grandma, brightened")
	os.Chtimes(filepath.Join(f.box.root, "data/apps/photos/originals/ab/abcd.jpg"), f.now, f.now)
	f.box.write("data/apps/photos/backup/files.jsonl", `{"path": "originals/ab/abcd.jpg", "as": "Photos/Priya/2026/2026-11/Grandma.jpg"}`+"\n"+
		`{"path": "/data/zoomriya/volumes/library/originals/ef/efgh.jpg", "as": "Photos/Bala/2026/2026-01/Snow.jpg"}`+"\n")
	f.databases["photos"] = "albums v2"
	if err := a.runBackup(); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(filepath.Join(drive, backupOlder, "2026-11-23", "Photos/Priya/2026/2026-11/Grandma.jpg")); string(got) != "a photo of Grandma" {
		t.Errorf("the copy before the change: %q", got)
	}
	if got, _ := os.ReadFile(filepath.Join(drive, backupDeleted, "2026-11-23", "Photos/Bala/2025/2025-07/Lake.jpg")); string(got) != "a photo of the lake" {
		t.Errorf("the deleted photo: %q", got)
	}

	// Listing and checking the backups, as a new box would.
	list, err := a.Handle(Request{Op: "backups", Action: ptr("list"), Device: ptr("/dev/sdb1")})
	if err != nil {
		t.Fatal(err)
	}
	sets := list["backups"].([]BackupSet)
	if len(sets) != 2 || sets[1].Name != "2026-11-22 0400" {
		t.Fatalf("backups %+v", sets)
	}
	if _, err := a.Handle(Request{Op: "backups", Action: ptr("check"), Device: ptr("/dev/sdb1"), Name: ptr(sets[1].Name), Key: ptr(newBackupKey())}); err == nil ||
		!strings.Contains(err.Error(), "isn't this backup's key") {
		t.Errorf("another key: %v", err)
	}
	check, err := a.Handle(Request{Op: "backups", Action: ptr("check"), Device: ptr("/dev/sdb1"), Name: ptr(sets[1].Name), Key: ptr(strings.ToLower(key))})
	if err != nil {
		t.Fatal(err)
	}
	if accounts := check["accounts"].([]restoreAccount); len(accounts) != 1 || accounts[0].Username != "priya" {
		t.Errorf("accounts %+v", accounts)
	}

	// The box loses its data; the first day's backup comes back, the
	// lake photo from Deleted and Grandma's from Older versions.
	os.RemoveAll(filepath.Join(f.box.root, "data/apps/photos"))
	os.RemoveAll(filepath.Join(f.box.root, "data/zoomriya-id"))
	os.RemoveAll(filepath.Join(f.box.root, "data/zoomriya/volumes/library/originals"))
	f.box.write("etc/zoomriya/photos.key", "a new box's own key")
	f.databases = map[string]string{}
	if _, err := a.Handle(Request{Op: "backups", Action: ptr("restore"), Device: ptr("/dev/sdb1"), Name: ptr(sets[1].Name),
		Key: ptr(key), Username: ptr("priya"), Password: ptr("a new password"), By: "setup-admin"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(f.read(restoreSecrets), "a new password") {
		t.Fatal("the restore's order wasn't kept for its unit")
	}
	if err := a.runRestore(); err != nil {
		t.Fatal(err)
	}
	job := a.readBackupJob(backupJobFile)
	if job == nil || !job.OK || !job.AccountsIn {
		t.Fatalf("restore %+v", job)
	}
	if _, err := os.Stat(filepath.Join(f.box.root, restoreSecrets)); err == nil {
		t.Error("the restore's key and password were left on the box")
	}
	for rel, want := range map[string]string{
		"data/apps/photos/originals/ab/abcd.jpg":              "a photo of Grandma",
		"data/apps/photos/originals/cd/cdef.jpg":              "a photo of the lake",
		"data/apps/photos/thumbs/ab/abcd.webp":                "small Grandma",
		"data/zoomriya-id/signing.key":                        "the ID's signing key",
		"data/zoomriya/volumes/library/originals/ef/efgh.jpg": "a photo on the library drive",
	} {
		if got := f.read(rel); got != want {
			t.Errorf("%s: %q, want %q", rel, got, want)
		}
	}
	if f.databases["photos"] != "albums v1" || f.databases["zoomriya_id"] != "accounts v1" {
		t.Errorf("databases %v", f.databases)
	}
	var recovered, noticed bool
	for _, c := range f.box.ran {
		line := strings.Join(c, " ")
		recovered = recovered || strings.HasSuffix(line, "app.recover_admin priya")
		noticed = noticed || strings.Contains(line, "app.restored --date "+time.Date(2026, 11, 22, 4, 0, 0, 0, time.UTC).Local().Format("2 January 2006")+" --by setup-admin")
	}
	if !recovered || !noticed {
		t.Errorf("new password %v, notice %v", recovered, noticed)
	}
}

func TestRestoreWaitsForTheAppsToBeInstalled(t *testing.T) {
	f := newBackupFake(t)
	a := f.agent
	resp, _ := a.Handle(Request{Op: "backups", Action: ptr("setup"), Device: ptr("/dev/sdb1"), By: "priya"})
	key := resp["key"].(string)
	if err := a.runBackup(); err != nil {
		t.Fatal(err)
	}
	// A new box: no Photos yet.
	f.box.write(appsState, `{}`)
	check, err := a.Handle(Request{Op: "backups", Action: ptr("check"), Device: ptr("/dev/sdb1"), Name: ptr("2026-11-22 0400"), Key: ptr(key)})
	if err != nil {
		t.Fatal(err)
	}
	states := map[string]string{}
	for _, p := range check["parts"].([]RestorePart) {
		states[p.ID] = p.State
	}
	if states["id"] != "ready" || states["photos"] != "not-installed" {
		t.Errorf("states %v", states)
	}
	_, err = a.Handle(Request{Op: "backups", Action: ptr("restore"), Device: ptr("/dev/sdb1"), Name: ptr("2026-11-22 0400"),
		Key: ptr(key), Include: []string{"photos"}})
	if err == nil || !strings.Contains(err.Error(), "App Center first") {
		t.Errorf("restoring an app not installed: %v", err)
	}
}

func TestBackupNamesAPartThatFailedAndKeepsTheRest(t *testing.T) {
	f := newBackupFake(t)
	a := f.agent
	a.Handle(Request{Op: "backups", Action: ptr("setup"), Device: ptr("/dev/sdb1"), By: "priya"})
	os.Remove(filepath.Join(f.box.root, "data/apps/photos/originals/cd/cdef.jpg"))
	if err := a.runBackup(); err != nil {
		t.Fatal(err)
	}
	last := a.readBackupJob(backupLast)
	if last.OK || len(last.Problems) != 1 || !strings.HasPrefix(last.Problems[0], "Family Photos: ") {
		t.Fatalf("last %+v", last)
	}
	data, _ := os.ReadFile(filepath.Join(f.drive(), backupSets, "2026-11-22 0400", "backup.json"))
	var set BackupSet
	json.Unmarshal(data, &set)
	if len(set.Parts) != 1 || set.Parts[0].ID != "id" || len(set.Missing) != 1 {
		t.Errorf("set %+v", set)
	}
}

func TestBackupWithoutItsDrive(t *testing.T) {
	f := newBackupFake(t)
	a := f.agent
	a.Handle(Request{Op: "backups", Action: ptr("setup"), Device: ptr("/dev/sdb1"), By: "priya"})
	f.agent.saveBackupSettings(BackupSettings{Drive: &BackupDrive{UUID: "GONE-0000", Label: "Old drive", FSType: "exfat"}})
	if err := a.runBackup(); err == nil || !strings.Contains(err.Error(), "isn't plugged in") {
		t.Errorf("no drive: %v", err)
	}
	if last := a.readBackupJob(backupLast); last == nil || last.OK {
		t.Errorf("last %+v", last)
	}
}

func TestOldBackupsAreLetGo(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 11, 22, 4, 0, 0, 0, time.UTC)
	keep := []string{"2026-11-22 0400", "2026-11-10 0400", "2026-10-01 0400", "2026-01-03 0400"}
	gone := []string{"2026-10-02 0400", "2025-10-01 0400", "2026-11-21 0400.partial"}
	for _, name := range append(append([]string{}, keep...), gone...) {
		os.MkdirAll(filepath.Join(root, backupSets, name), 0o700)
	}
	os.MkdirAll(filepath.Join(root, backupDeleted, "2026-10-01"), 0o700)
	os.MkdirAll(filepath.Join(root, backupDeleted, "2026-11-01"), 0o700)
	pruneBackupSets(root, now)
	pruneDated(filepath.Join(root, backupDeleted), now)
	for _, name := range keep {
		if _, err := os.Stat(filepath.Join(root, backupSets, name)); err != nil {
			t.Errorf("%s was let go", name)
		}
	}
	for _, name := range gone {
		if _, err := os.Stat(filepath.Join(root, backupSets, name)); err == nil {
			t.Errorf("%s was kept", name)
		}
	}
	if _, err := os.Stat(filepath.Join(root, backupDeleted, "2026-10-01")); err == nil {
		t.Error("deleted files older than 30 days were kept")
	}
}

func ptr(s string) *string { return &s }
