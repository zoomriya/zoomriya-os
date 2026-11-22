// The box's backup (design §6.10, Box Desktop: Storage Manager › Backups):
// a drive the admin plugs into the box, mounted only while a backup runs or
// one is brought back. On it:
//
//	ZOOMRIYA Backup/
//	  README.txt
//	  Photos/Priya/2026/2026-11/IMG_2041.jpg   files an app lists, copied as they are
//	  Vault/Priya/…                            (Vault's are sealed by each member's PIN)
//	  Older versions/2026-11-22/…              a listed file's copy before it changed
//	  Deleted/2026-11-22/…                     a listed file gone from the box, 30 days
//	  App data/2026-11-22 0400/                one backup: backup.json, and each app's
//	                                           database and data, sealed with the key
//	  .zoomriya/                               the key check and the list of copies
//
// The backup key is made when the admin chooses the drive and is shown to
// print; the box keeps it (/etc/zoomriya/backup.key) so it can be shown
// again. Without it the sealed files can't be opened, on this box or another.
//
// A backup runs as zoomriya-backup.service (`zoomriya-agent backup-job run`):
// "Back up now", or nightly once an admin turns that on
// (zoomriya-backup.timer). Bringing one back runs as zoomriya-restore, after
// the apps are installed (backuprestore.go).
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	backupService  = "zoomriya-backup.service"
	backupTimer    = "zoomriya-backup.timer"
	restoreUnit    = "zoomriya-restore"
	backupConfig   = "var/lib/zoomriya-agent/backup.json"
	backupLast     = "var/lib/zoomriya-agent/backup-last.json"
	backupKeyFile  = "etc/zoomriya/backup.key"
	backupJobFile  = "run/zoomriya/backup-job.json"
	backupMount    = "run/zoomriya/backup-drive"
	backupStage    = "var/lib/zoomriya-agent/backup-stage"
	restoreSecrets = "run/zoomriya/restore-secrets.json"
	// The folder on the drive, and its parts.
	backupFolder  = "ZOOMRIYA Backup"
	backupMeta    = ".zoomriya"
	backupSets    = "App data"
	backupOlder   = "Older versions"
	backupDeleted = "Deleted"
	// Deleted files and older versions are kept this long.
	backupKeepDays = 30
	// How many of the last run's problems are shown.
	backupProblems = 10
	// ZOOMRIYA ID's part of every backup.
	idPart = "id"
)

// The drive chosen for backups, known by its filesystem's UUID.
type BackupDrive struct {
	UUID   string `json:"uuid"`
	Label  string `json:"label"`
	FSType string `json:"fstype"`
	Size   int64  `json:"size_bytes"`
}

type BackupSettings struct {
	Drive   *BackupDrive `json:"drive"`
	Nightly bool         `json:"nightly"`
	// Apps an admin left out; ZOOMRIYA ID is always in.
	LeftOut []string `json:"left_out"`
	Since   string   `json:"since"`
	By      string   `json:"by"`
}

// BackupJob is a backup or a restore running, or the last one's end.
type BackupJob struct {
	Action   string   `json:"action"` // "backup" or "restore"
	Step     string   `json:"step"`
	By       string   `json:"by"`
	Started  string   `json:"started"`
	Ended    string   `json:"ended,omitempty"`
	Done     bool     `json:"done"`
	OK       bool     `json:"ok"`
	Error    string   `json:"error,omitempty"`
	Problems []string `json:"problems"`
	// A restore: the backup's date, and whether ZOOMRIYA ID came back (Box
	// Desktop then signs the admin out: their accounts changed).
	From       string `json:"from,omitempty"`
	AccountsIn bool   `json:"accounts_in,omitempty"`
}

var (
	uuidPattern = regexp.MustCompile(`^[0-9A-Fa-f-]{4,36}$`)
	setPattern  = regexp.MustCompile(`^\d{4}-\d{2}-\d{2} \d{4}$`)
)

func (a *Agent) loadBackupSettings() BackupSettings {
	var s BackupSettings
	if data, err := os.ReadFile(a.path(backupConfig)); err == nil {
		_ = json.Unmarshal(data, &s)
	}
	return s
}

func (a *Agent) saveBackupSettings(s BackupSettings) error {
	data, _ := json.MarshalIndent(s, "", "  ")
	if err := os.MkdirAll(filepath.Dir(a.path(backupConfig)), 0o700); err != nil {
		return err
	}
	return writeFileAtomic(a.path(backupConfig), data, 0o600)
}

// backupKey: the box's key, made once.
func (a *Agent) backupKey(make bool) (string, error) {
	if text := a.readTrimmed(backupKeyFile); text != "" {
		return text, nil
	}
	if !make {
		return "", refuse("This box has no backup key yet: choose a backup drive first.")
	}
	key := newBackupKey()
	if err := os.MkdirAll(filepath.Dir(a.path(backupKeyFile)), 0o755); err != nil {
		return "", err
	}
	return key, writeFileAtomic(a.path(backupKeyFile), []byte(key+"\n"), 0o600)
}

func (a *Agent) readBackupJob(rel string) *BackupJob {
	data, err := os.ReadFile(a.path(rel))
	if err != nil {
		return nil
	}
	var job BackupJob
	if json.Unmarshal(data, &job) != nil {
		return nil
	}
	return &job
}

func (a *Agent) writeBackupJob(job BackupJob) {
	data, _ := json.Marshal(job)
	_ = os.MkdirAll(filepath.Dir(a.path(backupJobFile)), 0o755)
	_ = writeFileAtomic(a.path(backupJobFile), data, 0o644)
	if job.Done && job.Action == "backup" {
		_ = os.MkdirAll(filepath.Dir(a.path(backupLast)), 0o700)
		_ = writeFileAtomic(a.path(backupLast), data, 0o600)
	}
}

// runningBackupJob: a job not done whose unit still runs.
func (a *Agent) runningBackupJob() *BackupJob {
	job := a.readBackupJob(backupJobFile)
	if job == nil || job.Done {
		return nil
	}
	unit := backupService
	if job.Action == "restore" {
		unit = restoreUnit + ".service"
	}
	out, _ := a.Run("systemctl", "is-active", unit)
	if state := strings.TrimSpace(string(out)); state == "active" || state == "activating" {
		return job
	}
	if job.Step == "Starting" && a.now().Sub(parseTime(job.Started)) < time.Minute {
		return job // just asked; its unit may not be up yet
	}
	job.Done, job.Error = true, "It stopped before it finished."
	job.Ended = a.now().UTC().Format(time.RFC3339)
	a.writeBackupJob(*job)
	return nil
}

func parseTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}

// backupsRequest:
//
//	(none)    the backup's state
//	drives    the drives plugged in that could take backups, or hold them
//	setup     Device: the drive to back up to; makes the key and shows it
//	key       the key again, to print
//	now       a backup now
//	nightly   On: the nightly backup on or off
//	app       Name, On: an app in the backup or left out
//	list      Device: the backups on that drive
//	check     Device, Name (a backup), Key: the key is right; what a restore
//	          would bring back, and the accounts in it
//	restore   Device, Name, Key, Include (the parts; none: all); for an admin
//	          who forgot their password, Username and Password
func (a *Agent) backupsRequest(req Request) (Response, error) {
	switch deref(req.Action) {
	case "":
		return a.backups(), nil
	case "drives":
		return a.backupDrivesList()
	case "setup":
		return a.backupSetup(deref(req.Device), req.By)
	case "key":
		key, err := a.backupKey(false)
		if err != nil {
			return nil, err
		}
		return Response{"key": key}, nil
	case "now":
		return a.backupNow(req.By)
	case "nightly":
		if req.On == nil {
			return nil, refuse("On or off?")
		}
		return a.backupNightly(*req.On, req.By)
	case "app":
		if req.On == nil || req.Name == nil {
			return nil, refuse("Which app, in or out?")
		}
		return a.backupApp(*req.Name, *req.On)
	case "list":
		return a.backupList(deref(req.Device))
	case "check":
		return a.restoreCheck(deref(req.Device), deref(req.Name), deref(req.Key))
	case "restore":
		return a.restoreStart(req)
	}
	return nil, refuse("Not something backups can do.")
}

// backups: the state Box Desktop shows.
func (a *Agent) backups() Response {
	s := a.loadBackupSettings()
	resp := Response{"set_up": s.Drive != nil, "running": false, "job": nil, "last": nil}
	if job := a.runningBackupJob(); job != nil {
		resp["running"], resp["job"] = true, job
	} else if job := a.readBackupJob(backupJobFile); job != nil {
		resp["job"] = job
	}
	if last := a.readBackupJob(backupLast); last != nil {
		resp["last"] = last
		resp["last_end"], resp["last_ok"] = last.Ended, last.OK
		if len(last.Problems) > 0 {
			resp["problems"] = last.Problems
		}
	}
	resp["apps"] = a.backupAppsChoice(s)
	if s.Drive == nil {
		return resp
	}
	resp["drive"] = s.Drive
	resp["drive_here"] = a.backupDriveDevice(s.Drive.UUID) != ""
	resp["nightly"] = s.Nightly
	if s.Nightly {
		if next := unixTime(a.unitShow(backupTimer, "NextElapseUSecRealtime")["NextElapseUSecRealtime"]); next != "" {
			resp["next"] = next
		}
	}
	return resp
}

type BackupAppChoice struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	In       bool   `json:"in"`
	Required bool   `json:"required"`
}

func (a *Agent) backupAppsChoice(s BackupSettings) []BackupAppChoice {
	list := []BackupAppChoice{{ID: idPart, Name: "ZOOMRIYA ID", In: true, Required: true}}
	apps := a.loadApps()
	ids := make([]string, 0, len(apps))
	for id := range apps {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return apps[ids[i]].Name < apps[ids[j]].Name })
	for _, id := range ids {
		list = append(list, BackupAppChoice{ID: id, Name: apps[id].Name, In: !contains(s.LeftOut, id)})
	}
	return list
}

// unitShow: systemctl show's properties of a unit, times as @<seconds>.
func (a *Agent) unitShow(unit string, properties ...string) map[string]string {
	out, _ := a.Run("systemctl", "show", unit, "--timestamp=unix", "-p", strings.Join(properties, ","))
	found := map[string]string{}
	for _, line := range strings.Split(string(out), "\n") {
		if key, value, ok := strings.Cut(line, "="); ok {
			found[key] = strings.TrimSpace(value)
		}
	}
	return found
}

// unixTime: "@1762747200" as RFC 3339 in UTC, or "" for none.
func unixTime(value string) string {
	var seconds int64
	if _, err := fmt.Sscanf(value, "@%d", &seconds); err != nil || seconds <= 0 {
		return ""
	}
	return time.Unix(seconds, 0).UTC().Format(time.RFC3339)
}

// --- drives -------------------------------------------------------------------------

// A drive plugged in that could take backups: a USB drive's filesystem the
// box can write, not mounted, not the system drive.
type BackupDriveFound struct {
	Device string `json:"device"`
	UUID   string `json:"uuid"`
	Label  string `json:"label"`
	FSType string `json:"fstype"`
	Size   int64  `json:"size_bytes"`
	// HasBackups: a ZOOMRIYA Backup folder is on it; Chosen: it's this box's.
	HasBackups bool `json:"has_backups"`
	Chosen     bool `json:"chosen"`
}

func (a *Agent) backupDrives() ([]BackupDriveFound, error) {
	out, err := a.Run("lsblk", "--json", "--bytes", "--paths", "--output",
		"NAME,TYPE,TRAN,FSTYPE,LABEL,UUID,SIZE,MOUNTPOINTS")
	if err != nil {
		return nil, err
	}
	var list struct {
		Blockdevices []struct {
			lsblkTree
			UUID     *string `json:"uuid"`
			Children []struct {
				lsblkTree
				UUID *string `json:"uuid"`
			} `json:"children"`
		} `json:"blockdevices"`
	}
	if err := json.Unmarshal(out, &list); err != nil {
		return nil, fmt.Errorf("lsblk's list: %w", err)
	}
	system := a.systemDisk()
	chosen := a.loadBackupSettings().Drive
	found := []BackupDriveFound{}
	for _, disk := range list.Blockdevices {
		if disk.Type != "disk" || deref(disk.Tran) != "usb" || filepath.Base(disk.Name) == system {
			continue
		}
		type part struct {
			t    lsblkTree
			uuid string
		}
		parts := []part{{disk.lsblkTree, deref(disk.UUID)}}
		for _, c := range disk.Children {
			parts = append(parts, part{c.lsblkTree, deref(c.UUID)})
		}
		for _, p := range parts {
			if _, ok := stickFilesystems[deref(p.t.FSType)]; !ok || p.uuid == "" {
				continue
			}
			mounted := false
			for _, m := range p.t.Mountpoints {
				mounted = mounted || (m != nil && *m != a.path(backupMount))
			}
			if mounted {
				continue
			}
			found = append(found, BackupDriveFound{Device: p.t.Name, UUID: p.uuid, Label: deref(p.t.Label),
				FSType: deref(p.t.FSType), Size: p.t.Size, Chosen: chosen != nil && chosen.UUID == p.uuid})
		}
	}
	return found, nil
}

// backupDriveDevice: where the drive with this UUID is plugged in, or "".
func (a *Agent) backupDriveDevice(uuid string) string {
	drives, _ := a.backupDrives()
	for _, d := range drives {
		if d.UUID == uuid {
			return d.Device
		}
	}
	return ""
}

func (a *Agent) backupDrivesList() (Response, error) {
	if job := a.runningBackupJob(); job != nil {
		return nil, refuse("Wait for the backup to finish: the drive is in use.")
	}
	drives, err := a.backupDrives()
	if err != nil {
		return nil, err
	}
	for i := range drives {
		_ = a.withBackupDrive(drives[i].Device, drives[i].FSType, false, func(dir string) error {
			_, err := os.Stat(filepath.Join(dir, backupFolder, backupMeta, "key-check"))
			drives[i].HasBackups = err == nil
			return nil
		})
	}
	return Response{"drives": drives}, nil
}

// withBackupDrive mounts the drive for fn, never running anything from it, and
// unmounts it after: the drive is open only while it's used.
func (a *Agent) withBackupDrive(device, fstype string, writable bool, fn func(dir string) error) error {
	kernel, ok := stickFilesystems[fstype]
	if !ok {
		return refuse("The box can't use a drive formatted that way.")
	}
	dir := a.path(backupMount)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	options := "nosuid,nodev,noexec,noatime"
	if !writable {
		options += ",ro"
	}
	if _, err := a.Run("mount", "-t", kernel, "-o", options, device, dir); err != nil {
		return refuse("The box couldn't open the backup drive.")
	}
	defer func() {
		_, _ = a.Run("sync")
		_, _ = a.Run("umount", dir)
	}()
	return fn(dir)
}

// findBackupDrive: the plugged-in drive with this device name.
func (a *Agent) findBackupDrive(device string) (BackupDriveFound, error) {
	drives, err := a.backupDrives()
	if err != nil {
		return BackupDriveFound{}, err
	}
	for _, d := range drives {
		if d.Device == device {
			return d, nil
		}
	}
	return BackupDriveFound{}, refuse("That drive isn't plugged in any more.")
}

// backupSetup chooses the drive, and makes the key the first time. The key
// is written on the drive only as a check (a small file sealed with it).
func (a *Agent) backupSetup(device, by string) (Response, error) {
	if job := a.runningBackupJob(); job != nil {
		return nil, refuse("Wait for the backup to finish first.")
	}
	drive, err := a.findBackupDrive(device)
	if err != nil {
		return nil, err
	}
	key, err := a.backupKey(true)
	if err != nil {
		return nil, err
	}
	raw, err := parseBackupKey(key)
	if err != nil {
		return nil, err
	}
	err = a.withBackupDrive(drive.Device, drive.FSType, true, func(dir string) error {
		root := filepath.Join(dir, backupFolder)
		check := filepath.Join(root, backupMeta, "key-check")
		if old, err := os.ReadFile(check); err == nil {
			if _, err := openBytes(raw, old); err != nil {
				return refuse("That drive has another box's backups, made with another key. Use another drive, or bring those back first.")
			}
		}
		if err := os.MkdirAll(filepath.Join(root, backupMeta), 0o700); err != nil {
			return refuse("The box couldn't write to that drive. Is it full, or locked?")
		}
		sealed, err := sealBytes(raw, []byte(keyCheckWords))
		if err != nil {
			return err
		}
		if err := os.WriteFile(check, sealed, 0o600); err != nil {
			return refuse("The box couldn't write to that drive. Is it full, or locked?")
		}
		return os.WriteFile(filepath.Join(root, "README.txt"), []byte(backupReadme), 0o644)
	})
	if err != nil {
		return nil, err
	}
	s := a.loadBackupSettings()
	s.Drive = &BackupDrive{UUID: drive.UUID, Label: drive.Label, FSType: drive.FSType, Size: drive.Size}
	s.Since, s.By = a.now().UTC().Format(time.RFC3339), by
	if err := a.saveBackupSettings(s); err != nil {
		return nil, err
	}
	a.notify("backup", "info", fmt.Sprintf("%s chose %s for the box's backups.", by, driveName(*s.Drive)))
	resp := a.backups()
	resp["key"] = key
	return resp, nil
}

const keyCheckWords = "ZOOMRIYA backup key check"

const backupReadme = `ZOOMRIYA Backup

This drive holds a ZOOMRIYA Box's backup.

Photos and other files are here as they are: open them on any computer.
Family Vault's documents stay sealed by each member's PIN.

"App data" holds each app's database and data, and the accounts, sealed
with the backup key printed when this drive was chosen. To bring them
back, plug this drive into a ZOOMRIYA Box, install the apps, and open
Box Desktop > Storage Manager > Backups > Bring a backup back.

"Older versions" keeps a file's copy before it changed, and "Deleted"
keeps files removed from the box, each for 30 days.

Keep this drive somewhere safe, and the backup key somewhere else.
`

func driveName(d BackupDrive) string {
	if d.Label != "" {
		return d.Label
	}
	return "the backup drive"
}

func (a *Agent) backupNow(by string) (Response, error) {
	s := a.loadBackupSettings()
	if s.Drive == nil {
		return nil, refuse("Choose a backup drive first.")
	}
	if job := a.runningBackupJob(); job != nil {
		return nil, refuse("A backup is already running.")
	}
	a.writeBackupJob(BackupJob{Action: "backup", Step: "Starting", By: by,
		Started: a.now().UTC().Format(time.RFC3339), Problems: []string{}})
	if _, err := a.Run("systemctl", "start", "--no-block", backupService); err != nil {
		_ = os.Remove(a.path(backupJobFile))
		return nil, refuse("The backup couldn't be started.")
	}
	return a.backups(), nil
}

func (a *Agent) backupNightly(on bool, by string) (Response, error) {
	s := a.loadBackupSettings()
	if s.Drive == nil {
		return nil, refuse("Choose a backup drive first.")
	}
	verb := "disable"
	if on {
		verb = "enable"
	}
	if _, err := a.Run("systemctl", verb, "--now", backupTimer); err != nil {
		return nil, refuse("The nightly backup couldn't be changed.")
	}
	s.Nightly = on
	if err := a.saveBackupSettings(s); err != nil {
		return nil, err
	}
	return a.backups(), nil
}

func (a *Agent) backupApp(id string, in bool) (Response, error) {
	if id == idPart {
		return nil, refuse("The accounts are always backed up: no app opens without them.")
	}
	if _, ok := a.loadApps()[id]; !ok {
		return nil, refuse("There's no such app.")
	}
	s := a.loadBackupSettings()
	kept := []string{}
	for _, left := range s.LeftOut {
		if left != id {
			kept = append(kept, left)
		}
	}
	if !in {
		kept = append(kept, id)
	}
	s.LeftOut = kept
	if err := a.saveBackupSettings(s); err != nil {
		return nil, err
	}
	return a.backups(), nil
}

// --- backups on a drive ---------------------------------------------------------------

// BackupSet is one backup's backup.json, in its App data folder.
type BackupSet struct {
	Name    string          `json:"name"` // its folder: "2026-11-22 0400"
	Date    string          `json:"date"`
	Box     string          `json:"box"`
	OS      string          `json:"os"`
	By      string          `json:"by"`
	Parts   []BackupSetPart `json:"parts"`
	Missing []string        `json:"missing,omitempty"` // apps that failed that time
}

type BackupSetPart struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Version string   `json:"version"`
	Files   []string `json:"files"` // the sealed archive's parts
	// Copies: how many listed files it had on the drive as they are.
	Copies int `json:"copies"`
}

func readBackupSets(root string) []BackupSet {
	sets := []BackupSet{}
	entries, _ := os.ReadDir(filepath.Join(root, backupSets))
	for _, e := range entries {
		if !e.IsDir() || !setPattern.MatchString(e.Name()) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, backupSets, e.Name(), "backup.json"))
		if err != nil {
			continue // one cut short: never finished
		}
		var set BackupSet
		if json.Unmarshal(data, &set) == nil {
			set.Name = e.Name()
			sets = append(sets, set)
		}
	}
	sort.Slice(sets, func(i, j int) bool { return sets[i].Name > sets[j].Name })
	return sets
}

func (a *Agent) backupList(device string) (Response, error) {
	if job := a.runningBackupJob(); job != nil {
		return nil, refuse("Wait for the backup to finish: the drive is in use.")
	}
	drive, err := a.findBackupDrive(device)
	if err != nil {
		return nil, err
	}
	var sets []BackupSet
	err = a.withBackupDrive(drive.Device, drive.FSType, false, func(dir string) error {
		root := filepath.Join(dir, backupFolder)
		if _, err := os.Stat(filepath.Join(root, backupMeta, "key-check")); err != nil {
			return refuse("There are no ZOOMRIYA backups on that drive.")
		}
		sets = readBackupSets(root)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return Response{"backups": sets}, nil
}

// keyMatches: the key typed opens this drive's key check.
func keyMatches(root string, key []byte) error {
	sealed, err := os.ReadFile(filepath.Join(root, backupMeta, "key-check"))
	if err != nil {
		return refuse("There are no ZOOMRIYA backups on that drive.")
	}
	plain, err := openBytes(key, sealed)
	if err != nil || string(plain) != keyCheckWords {
		return refuse("That isn't this backup's key. It's the one printed when the drive was chosen.")
	}
	return nil
}

var errNoSuchSet = errors.New("no such backup")
