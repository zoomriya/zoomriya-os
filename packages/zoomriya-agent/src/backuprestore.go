// Bringing a backup back (design §6.10, Box Desktop: Storage Manager ›
// Backups › Bring a backup back), once the apps are installed: never in the
// first setup.
//
// The admin plugs the drive in, picks a backup, and types the backup key.
// "check" opens it and says what would come back: each part, whether its app
// is installed (if not, App Center first) and new enough, and the admins in
// it. "restore" (with the password again) runs as zoomriya-restore: for each
// part, ZOOMRIYA ID first, stop it, open its archive, put its data folder and
// its listed files back (each checked against its SHA-256, from the copy on
// the drive, its older versions or Deleted), recreate its databases from the
// dumps, and start it again.
//
// The accounts come back as they were: every password, second sign-in step
// and signed-in device; the admin made at the first setup goes. An admin who
// forgot their old password names their account and a new one: the backup
// key proves the backup is theirs, as the setup code does at the box. Every
// restore is in the accounts' activity and on the bell.
package main

import (
	"archive/tar"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"
)

// What a restore would do with one part of a backup.
type RestorePart struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Version   string `json:"version"`
	Installed string `json:"installed"`
	// "ready", "not-installed" (App Center first) or "update-first" (the
	// backup is from a newer version than the one installed).
	State string `json:"state"`
	// Copies: files it has on the drive as they are (photos, documents).
	Copies int `json:"copies"`
}

type restoreAccount struct {
	Username string `json:"username"`
	Name     string `json:"name"`
}

// The restore's request, kept for its unit in a root-only file, never on a
// command line.
type restoreOrder struct {
	Device   string   `json:"device"`
	FSType   string   `json:"fstype"`
	Set      string   `json:"set"`
	Key      string   `json:"key"`
	Include  []string `json:"include"`
	Username string   `json:"username,omitempty"`
	Password string   `json:"password,omitempty"`
	By       string   `json:"by"`
}

// openSet: a backup on a mounted drive, once the key is checked.
func openSet(root, name string, key []byte) (BackupSet, error) {
	if err := keyMatches(root, key); err != nil {
		return BackupSet{}, err
	}
	for _, set := range readBackupSets(root) {
		if set.Name == name {
			return set, nil
		}
	}
	return BackupSet{}, refuse("That backup isn't on the drive any more.")
}

func (a *Agent) restoreParts(set BackupSet) []RestorePart {
	apps := a.loadApps()
	parts := []RestorePart{}
	for _, p := range set.Parts {
		rp := RestorePart{ID: p.ID, Name: p.Name, Version: p.Version, Copies: p.Copies, State: "ready"}
		if p.ID == idPart {
			rp.Installed = "built in"
		} else if app, ok := apps[p.ID]; !ok {
			rp.State = "not-installed"
		} else {
			rp.Installed = app.Active
			if compareVersions(p.Version, app.Active) > 0 {
				rp.State = "update-first"
			}
		}
		parts = append(parts, rp)
	}
	return parts
}

func (a *Agent) restoreCheck(device, name, typed string) (Response, error) {
	if job := a.runningBackupJob(); job != nil {
		return nil, refuse("Wait for the %s to finish first.", job.Action)
	}
	if !setPattern.MatchString(name) {
		return nil, refuse("Which backup?")
	}
	key, err := parseBackupKey(typed)
	if err != nil {
		return nil, refuse("That isn't a backup key: it's 24 letters and digits, like 7KQ2-M9XA-….")
	}
	drive, err := a.findBackupDrive(device)
	if err != nil {
		return nil, err
	}
	var resp Response
	err = a.withBackupDrive(drive.Device, drive.FSType, false, func(dir string) error {
		root := filepath.Join(dir, backupFolder)
		set, err := openSet(root, name, key)
		if err != nil {
			return err
		}
		accounts := []restoreAccount{}
		if sealed, err := os.ReadFile(filepath.Join(root, backupSets, name, "accounts.zbak")); err == nil {
			if plain, err := openBytes(key, sealed); err == nil {
				_ = json.Unmarshal(plain, &accounts)
			}
		}
		resp = Response{"backup": set, "parts": a.restoreParts(set), "accounts": accounts,
			"same_box": set.Box != "" && set.Box == a.serial()}
		return nil
	})
	return resp, err
}

func (a *Agent) restoreStart(req Request) (Response, error) {
	if job := a.runningBackupJob(); job != nil {
		return nil, refuse("Wait for the %s to finish first.", job.Action)
	}
	order := restoreOrder{Device: deref(req.Device), Set: deref(req.Name), Key: deref(req.Key), By: req.By}
	check, err := a.restoreCheck(order.Device, order.Set, order.Key)
	if err != nil {
		return nil, err
	}
	parts := check["parts"].([]RestorePart)
	ready := map[string]RestorePart{}
	for _, p := range parts {
		ready[p.ID] = p
	}
	include := req.Include
	if len(include) == 0 {
		for _, p := range parts {
			if p.State == "ready" {
				include = append(include, p.ID)
			}
		}
	}
	for _, id := range include {
		p, ok := ready[id]
		switch {
		case !ok:
			return nil, refuse("That backup has no %s.", id)
		case p.State == "not-installed":
			return nil, refuse("Install %s from App Center first, then bring its backup back.", p.Name)
		case p.State == "update-first":
			return nil, refuse("The backup of %s is from version %s: update it to that version or newer first.", p.Name, p.Version)
		}
	}
	if len(include) == 0 {
		return nil, refuse("There's nothing in that backup to bring back yet: install its apps first.")
	}
	if !contains(include, idPart) && !check["same_box"].(bool) {
		return nil, refuse("Bring the accounts (ZOOMRIYA ID) back too: this box's accounts aren't the ones in the backup.")
	}
	order.Include = include
	if req.Username != nil || req.Password != nil {
		if !contains(include, idPart) {
			return nil, refuse("A new password comes with bringing the accounts back.")
		}
		order.Username, order.Password = strings.TrimSpace(deref(req.Username)), deref(req.Password)
		known := false
		for _, account := range check["accounts"].([]restoreAccount) {
			known = known || account.Username == order.Username
		}
		if !known {
			return nil, refuse("There's no admin called %s in that backup.", order.Username)
		}
		if n := utf8.RuneCountInString(order.Password); n < minPassword || n > 256 || strings.ContainsAny(order.Password, "\r\n") {
			return nil, refuse("The password must be %d to 256 characters.", minPassword)
		}
	}
	drive, err := a.findBackupDrive(order.Device)
	if err != nil {
		return nil, err
	}
	order.FSType = drive.FSType
	data, _ := json.Marshal(order)
	if err := os.MkdirAll(filepath.Dir(a.path(restoreSecrets)), 0o755); err != nil {
		return nil, err
	}
	if err := writeFileAtomic(a.path(restoreSecrets), data, 0o600); err != nil {
		return nil, err
	}
	set := check["backup"].(BackupSet)
	a.writeBackupJob(BackupJob{Action: "restore", Step: "Starting", By: req.By, From: set.Date,
		Started: a.now().UTC().Format(time.RFC3339), Problems: []string{}})
	if _, err := a.Run("systemd-run", "--quiet", "--collect", "--unit="+restoreUnit, "--property=Type=oneshot",
		"--description=Bring a ZOOMRIYA backup back", "--", agentPath, "backup-job", "restore"); err != nil {
		_ = os.Remove(a.path(restoreSecrets))
		_ = os.Remove(a.path(backupJobFile))
		return nil, err
	}
	return a.backups(), nil
}

// runRestore is the restore's unit.
func (a *Agent) runRestore() error {
	job := BackupJob{Action: "restore", Problems: []string{}}
	if asked := a.readBackupJob(backupJobFile); asked != nil {
		job.By, job.From = asked.By, asked.From
	}
	job.Started = a.now().UTC().Format(time.RFC3339)
	step := func(s string) {
		job.Step = s
		a.writeBackupJob(job)
	}
	finish := func(err error) error {
		job.Done, job.Ended, job.Step = true, a.now().UTC().Format(time.RFC3339), ""
		job.OK = err == nil && len(job.Problems) == 0
		if err != nil {
			job.Error = err.Error()
			var r errRequest
			if !errors.As(err, &r) {
				job.Error = "The backup wasn't brought back: " + err.Error()
			}
			a.notify("backup", "error", job.Error)
		}
		a.writeBackupJob(job)
		NewJournal().Record(AuditEntry{Op: "backup-restore", By: job.By, Detail: job.From,
			Result: map[bool]string{true: "brought back", false: "failed"}[job.OK]})
		return err
	}
	step("Starting")
	data, err := os.ReadFile(a.path(restoreSecrets))
	_ = os.Remove(a.path(restoreSecrets))
	var order restoreOrder
	if err != nil || json.Unmarshal(data, &order) != nil {
		return finish(refuse("The restore was asked for, but what to bring back was lost. Try again."))
	}
	key, err := parseBackupKey(order.Key)
	if err != nil {
		return finish(err)
	}
	stage := a.path(backupStage)
	_ = os.RemoveAll(stage)
	if err := os.MkdirAll(stage, 0o711); err != nil {
		return finish(err)
	}
	defer os.RemoveAll(stage)

	err = a.withBackupDrive(order.Device, order.FSType, false, func(dir string) error {
		root := filepath.Join(dir, backupFolder)
		set, err := openSet(root, order.Set, key)
		if err != nil {
			return err
		}
		job.From = set.Date
		// ZOOMRIYA ID first: every app's data names its accounts.
		ordered := []BackupSetPart{}
		for _, p := range set.Parts {
			if p.ID == idPart && contains(order.Include, p.ID) {
				ordered = append([]BackupSetPart{p}, ordered...)
			} else if contains(order.Include, p.ID) {
				ordered = append(ordered, p)
			}
		}
		for _, p := range ordered {
			step("Bringing back " + p.Name)
			if err := a.restorePart(p, root, filepath.Join(root, backupSets, set.Name), key); err != nil {
				if p.ID == idPart {
					return refuse("The accounts couldn't be brought back (%s), so nothing else was.", err.Error())
				}
				job.Problems = append(job.Problems, fmt.Sprintf("%s: %s", p.Name, err.Error()))
				continue
			}
			if p.ID == idPart {
				job.AccountsIn = true
				a.writeBackupJob(job)
			}
		}
		return nil
	})
	if err != nil {
		return finish(err)
	}
	when := job.From
	if t, err := time.Parse(time.RFC3339, job.From); err == nil {
		when = t.Local().Format("2 January 2006")
	}
	if job.AccountsIn {
		if order.Username != "" {
			step("Giving " + order.Username + " a new password")
			if err := a.idRun(order.Password, "app.recover_admin", order.Username); err != nil {
				job.Problems = append(job.Problems, fmt.Sprintf(
					"%s's new password wasn't set: sign in with the old one, or use Can't sign in? at the box.", order.Username))
			}
		}
		_ = a.idRun("", "app.restored", "--date", when, "--by", order.By)
	}
	a.notify("backup", "warning", fmt.Sprintf("This box was restored from the backup of %s by %s.", when, order.By))
	return finish(nil)
}

// idRun runs one of ZOOMRIYA ID's commands as its own account, with input on
// standard input (a password never shows in the list of processes).
func (a *Agent) idRun(input string, module string, args ...string) error {
	runInput := a.RunInput
	if runInput == nil {
		runInput = runWithInput
	}
	_, err := runInput(input, "systemd-run", append([]string{"--quiet", "--pipe", "--wait", "--collect",
		"--property=User=zoomriya-id", "--property=EnvironmentFile=/etc/zoomriya/id.env",
		"--property=WorkingDirectory=/opt/zoomriya/id/backend",
		"--", idPython, "-m", module}, args...)...)
	return err
}

// restorePart brings one part back. Its data is put together beside the
// current folder first; the databases and the swap come only once all of it
// read back right.
func (a *Agent) restorePart(p BackupSetPart, root, setDir string, key []byte) error {
	part, release, err := a.currentPart(p.ID)
	if err != nil {
		return err
	}
	archive, err := openParts(filepath.Join(setDir, p.ID+".zbak"))
	if err != nil {
		return refuse("its backup isn't complete on the drive")
	}
	defer archive.Close()
	opened, err := newOpenReader(archive, key)
	if err != nil {
		return refuse("its backup isn't readable")
	}

	target := ""
	if part.Data != "" {
		target = a.path(part.Data)
	}
	fresh := target + ".restoring"
	if target != "" {
		_ = os.RemoveAll(fresh)
		if err := os.MkdirAll(fresh, 0o700); err != nil {
			return err
		}
		defer os.RemoveAll(fresh)
	}
	owner := ownerOf(target)

	// Stopped first, so nothing writes while it's brought back.
	if release != nil {
		a.stopRelease(*release)
	} else {
		_, _ = a.Run("systemctl", "stop", idService)
	}
	started := false
	defer func() {
		if !started {
			a.startPart(p.ID, release)
		}
	}()

	listed := []listedFile{}
	dumps := map[string]string{}
	keys := map[string][]byte{}
	tr := tar.NewReader(opened)
	var topUID, topGID = -1, -1
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			if errors.Is(err, errWrongKey) {
				return refuse("its backup was changed or is damaged")
			}
			return refuse("its backup isn't readable")
		}
		switch {
		case h.Name == "files.json":
			if err := json.NewDecoder(tr).Decode(&listed); err != nil {
				return refuse("its list of files isn't readable")
			}
		case strings.HasPrefix(h.Name, "databases/") && strings.HasSuffix(h.Name, ".dump"):
			db := strings.TrimSuffix(strings.TrimPrefix(h.Name, "databases/"), ".dump")
			if !contains(part.Databases, db) {
				continue
			}
			folder := filepath.Join(a.path(backupStage), db)
			if err := os.MkdirAll(folder, 0o700); err != nil {
				return err
			}
			file := filepath.Join(folder, db+".dump")
			if err := writeFrom(tr, file, 0o600); err != nil {
				return err
			}
			_, _ = a.Run("chown", "-R", "postgres", folder)
			dumps[db] = file
		case strings.HasPrefix(h.Name, "keys/") && contains(boxKeyNames(p.ID), strings.TrimPrefix(h.Name, "keys/")):
			// The backup's box key replaces the new box's: the data coming
			// back was sealed with it.
			data, err := io.ReadAll(io.LimitReader(tr, 4096))
			if err != nil {
				return refuse("its backup was changed or is damaged")
			}
			keys[strings.TrimPrefix(h.Name, "keys/")] = data
		case h.Name == "data/" && target != "":
			topUID, topGID = h.Uid, h.Gid
			_ = os.Chmod(fresh, os.FileMode(h.Mode).Perm())
		case strings.HasPrefix(h.Name, "data/") && target != "":
			rel := strings.TrimPrefix(h.Name, "data/")
			if !safeRelative(strings.TrimSuffix(rel, "/")) {
				return refuse("its backup names a file outside its folder")
			}
			if err := extractEntry(tr, h, filepath.Join(fresh, filepath.FromSlash(rel)), mapOwner(h, topUID, topGID, owner)); err != nil {
				return err
			}
		}
	}
	if _, err := io.Copy(io.Discard, opened); err != nil {
		return refuse("its backup was changed or is damaged")
	}

	// Its files kept as they are, each checked.
	missing := 0
	for _, l := range listed {
		if !listedPathValid(l.Path) || (target == "" && !filepath.IsAbs(l.Path)) {
			continue
		}
		// In the app's folder: into the new one. On the library drives: in
		// place, unless the same file is already there.
		dest := filepath.Join(fresh, filepath.FromSlash(l.Path))
		if filepath.IsAbs(l.Path) {
			dest = a.path(l.Path)
			if sum, err := fileSHA256(dest); err == nil && sum == l.SHA256 {
				continue
			}
		}
		owner := ownerOf(nearestFolder(filepath.Dir(dest)))
		if !restoreCopy(root, l, dest) {
			missing++
			continue
		}
		_ = os.Lchown(dest, owner[0], owner[1])
		_ = os.Chmod(dest, 0o640)
	}
	if missing > 0 {
		return refuse("%d of its files aren't on the drive any more", missing)
	}

	for name, data := range keys {
		path := a.path(filepath.Join(boxKeysDir, name))
		owner := ownerOf(path)
		if _, err := os.Stat(path); err != nil {
			owner = ownerOf(a.path(boxKeysDir))
		}
		if err := writeFileAtomic(path, data, 0o400); err != nil {
			return refuse("its box key couldn't be put back")
		}
		_ = os.Chown(path, owner[0], owner[1])
	}
	for _, db := range part.Databases {
		file, ok := dumps[db]
		if !ok {
			return refuse("its database %s isn't in the backup", db)
		}
		if err := a.restoreDatabase(db, file); err != nil {
			return err
		}
	}
	if target != "" {
		before := target + ".before-restore"
		_ = os.RemoveAll(before)
		if _, err := os.Lstat(target); err == nil {
			if err := os.Rename(target, before); err != nil {
				return err
			}
		}
		if err := os.Rename(fresh, target); err != nil {
			_ = os.Rename(before, target)
			return err
		}
		_ = os.Chown(target, owner[0], owner[1])
		defer os.RemoveAll(before)
	}
	started = true
	if err := a.startPart(p.ID, release); err != nil {
		return refuse("it was brought back but didn't start again")
	}
	return nil
}

// currentPart: the part as this box has it now, and its release.
func (a *Agent) currentPart(id string) (backupPart, *Release, error) {
	if id == idPart {
		return backupPart{ID: idPart, Name: "ZOOMRIYA ID", Data: idDataDir, Databases: []string{idDatabase}}, nil, nil
	}
	app, ok := a.loadApps()[id]
	if !ok {
		return backupPart{}, nil, refuse("it isn't installed")
	}
	r := app.Releases[app.Active]
	part := backupPart{ID: id, Name: app.Name, Version: app.Active, Databases: app.Databases, Release: &r}
	if app.Data {
		part.Data = filepath.Join(appDataRoot, id)
	}
	return part, &r, nil
}

func (a *Agent) startPart(id string, release *Release) error {
	if release == nil {
		_, err := a.Run("systemctl", "start", idService)
		return err
	}
	return a.startRelease(id, *release)
}

// restoreDatabase recreates a database from its dump, owned by the role that
// owns it now.
func (a *Agent) restoreDatabase(db, dump string) error {
	out, err := a.Run("runuser", "-u", "postgres", "--", "psql", "-qtAc",
		"SELECT pg_catalog.pg_get_userbyid(datdba) FROM pg_database WHERE datname = '"+db+"'")
	owner := strings.TrimSpace(string(out))
	if err != nil || owner == "" {
		return refuse("its database %s isn't on this box", db)
	}
	for _, args := range [][]string{
		{"dropdb", "--if-exists", "--force", db},
		{"createdb", "-O", owner, db},
		{"pg_restore", "--no-owner", "--role=" + owner, "--exit-on-error", "--single-transaction", "-d", db, dump},
	} {
		if _, err := a.Run("runuser", append([]string{"-u", "postgres", "--"}, args...)...); err != nil {
			return refuse("its database %s couldn't be brought back", db)
		}
	}
	return nil
}

// restoreCopy finds a listed file on the drive by its SHA-256: its copy, or
// one kept in Older versions or Deleted, and copies it to dest.
func restoreCopy(root string, l listedFile, dest string) bool {
	as := filepath.FromSlash(l.As)
	candidates := []string{filepath.Join(root, as)}
	for _, kind := range []string{backupOlder, backupDeleted} {
		days, _ := os.ReadDir(filepath.Join(root, kind))
		for i := len(days) - 1; i >= 0; i-- {
			candidates = append(candidates, filepath.Join(root, kind, days[i].Name(), as))
		}
	}
	for _, c := range candidates {
		if sum, err := fileSHA256(c); err == nil && (l.SHA256 == "" || sum == l.SHA256) {
			if _, err := copyChecked(c, dest); err == nil {
				return true
			}
		}
	}
	return false
}

// nearestFolder: the folder itself, or the nearest one above it that exists.
func nearestFolder(dir string) string {
	for {
		if _, err := os.Stat(dir); err == nil || dir == filepath.Dir(dir) {
			return dir
		}
		dir = filepath.Dir(dir)
	}
}

// ownerOf: a file's owner and group, or root's when it isn't there.
func ownerOf(path string) [2]int {
	info, err := os.Stat(path)
	if err != nil {
		return [2]int{0, 0}
	}
	if st, ok := info.Sys().(*syscall.Stat_t); ok {
		return [2]int{int(st.Uid), int(st.Gid)}
	}
	return [2]int{0, 0}
}

// mapOwner: the backup's owner of the app's folder becomes this box's (an
// account's number may differ on a new box); any other owner stays.
func mapOwner(h *tar.Header, topUID, topGID int, owner [2]int) [2]int {
	uid, gid := h.Uid, h.Gid
	if uid == topUID {
		uid = owner[0]
	}
	if gid == topGID {
		gid = owner[1]
	}
	return [2]int{uid, gid}
}

func extractEntry(tr *tar.Reader, h *tar.Header, dest string, owner [2]int) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o700); err != nil {
		return err
	}
	switch h.Typeflag {
	case tar.TypeDir:
		if err := os.MkdirAll(dest, 0o700); err != nil {
			return err
		}
		_ = os.Chmod(dest, os.FileMode(h.Mode).Perm())
	case tar.TypeSymlink:
		if err := os.Symlink(h.Linkname, dest); err != nil {
			return err
		}
	case tar.TypeReg:
		if err := writeFrom(tr, dest, os.FileMode(h.Mode).Perm()); err != nil {
			return err
		}
		_ = os.Chtimes(dest, h.ModTime, h.ModTime)
	default:
		return nil
	}
	_ = os.Lchown(dest, owner[0], owner[1])
	return nil
}

func writeFrom(r io.Reader, dest string, mode os.FileMode) error {
	f, err := os.OpenFile(dest, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	_, err = io.Copy(f, r)
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if errors.Is(err, errWrongKey) {
		return refuse("its backup was changed or is damaged")
	}
	return err
}
