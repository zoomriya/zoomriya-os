// A backup (`zoomriya-agent backup-job run`, as zoomriya-backup.service), onto
// the drive the admin chose (backups.go):
//
//  1. Each part in turn: ZOOMRIYA ID first, then each installed app an admin
//     didn't leave out.
//  2. Files the app lists (its backup_list unit writes
//     /data/apps/<id>/backup/files.jsonl: {"path": …, "as": "Photos/Priya/…"})
//     are copied as they are, under the names it gives. A copy is never
//     overwritten: when the file changed, the copy before goes to Older
//     versions; when it's gone from the box, its copy goes to Deleted. Both
//     are kept 30 days. Each copy is read back and its SHA-256 checked.
//  3. Everything else of the app (its databases, dumped with row security on
//     so Family Chat's private chats stay out, and its data folder) goes into
//     one sealed archive in App data/<date>/, with the list of its copies.
//  4. A part that fails is named, and the others still go; its earlier
//     copies stay where they are.
//
// Old backups in App data: every one of the last 14 days, then the first of
// each month for a year.
package main

import (
	"archive/tar"
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	backupListFile = "backup/files.jsonl"
	keepDailyDays  = 14
	keepMonths     = 12
	idDataDir      = "data/zoomriya-id"
	idDatabase     = "zoomriya_id"
	idService      = "zoomriya-id.service"
	backupRole     = "zoomriya-backup"
)

// boxKeysDir: where an app keeps its box key, the one that seals its data at
// rest (Health's records, Tracker's places). It goes into the app's sealed
// archive, so a new box can read what comes back.
const boxKeysDir = "etc/zoomriya"

// boxKeyNames: an app's box key files, by name: <id>.key, and Myira's
// myira-app.key.
func boxKeyNames(id string) []string {
	if id == idPart {
		return nil
	}
	return []string{id + ".key", id + "-app.key"}
}

// A part of the box backed up on its own.
type backupPart struct {
	ID, Name, Version string
	Data              string   // its folder, from Root; "" for none
	Databases         []string // its PostgreSQL databases
	List              string   // the unit that writes its list of files, or ""
	Release           *Release // an app's release, to stop and start it
}

// A file an app lists, as written in files.jsonl and in its archive.
type listedFile struct {
	Path   string `json:"path"`
	As     string `json:"as"`
	SHA256 string `json:"sha256,omitempty"`
	Size   int64  `json:"size,omitempty"`
}

// The drive's index of its copies: where each came from, so an unchanged
// file isn't read again.
type copyEntry struct {
	App      string `json:"app"`
	Path     string `json:"path"`
	Size     int64  `json:"size"`
	Modified int64  `json:"modified"`
	SHA256   string `json:"sha256"`
}

// backupParts: ZOOMRIYA ID, then the apps in the backup, by name.
func (a *Agent) backupParts(s BackupSettings) []backupPart {
	version, _ := a.Run("dpkg-query", "-W", "-f=${Version}", "zoomriya-id")
	parts := []backupPart{{ID: idPart, Name: "ZOOMRIYA ID", Version: strings.TrimSpace(string(version)),
		Data: idDataDir, Databases: []string{idDatabase}}}
	apps := a.loadApps()
	ids := []string{}
	for id := range apps {
		if !contains(s.LeftOut, id) {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return apps[ids[i]].Name < apps[ids[j]].Name })
	for _, id := range ids {
		app := apps[id]
		r := app.Releases[app.Active]
		part := backupPart{ID: id, Name: app.Name, Version: app.Active, Databases: app.Databases, Release: &r}
		if app.Data {
			part.Data = filepath.Join(appDataRoot, id)
		}
		if r.BackupList != "" {
			part.List = instances([]string{r.BackupList}, r.Version)[0]
		}
		parts = append(parts, part)
	}
	return parts
}

// backupJobMain is `zoomriya-agent backup-job run|restore`.
func backupJobMain(args []string) int {
	a := NewAgent()
	if len(args) != 1 {
		return 2
	}
	var err error
	switch args[0] {
	case "run":
		err = a.runBackup()
	case "restore":
		err = a.runRestore()
	default:
		return 2
	}
	if err != nil {
		return 1
	}
	return 0
}

// runBackup makes one backup, writing each step to the job file.
func (a *Agent) runBackup() error {
	job := BackupJob{Action: "backup", By: "nightly", Problems: []string{}}
	if asked := a.readBackupJob(backupJobFile); asked != nil && !asked.Done && asked.Action == "backup" {
		job.By = asked.By
	}
	job.Started = a.now().UTC().Format(time.RFC3339)
	step := func(s string) {
		job.Step = s
		a.writeBackupJob(job)
	}
	finish := func(err error) error {
		job.Done, job.Ended = true, a.now().UTC().Format(time.RFC3339)
		job.OK = err == nil && len(job.Problems) == 0
		job.Step = ""
		if err != nil {
			job.Error = err.Error()
			var r errRequest
			if !errors.As(err, &r) {
				job.Error = "The backup didn't finish: " + err.Error()
			}
		}
		switch {
		case err != nil:
			a.notify("backup", "error", job.Error)
		case len(job.Problems) > 0:
			a.notify("backup", "warning", "The backup finished, but not everything was backed up: "+strings.Join(job.Problems, " "))
		}
		a.writeBackupJob(job)
		NewJournal().Record(AuditEntry{Op: "backup", By: job.By, Result: map[bool]string{true: "backed up", false: "failed"}[job.OK]})
		return err
	}
	step("Starting")

	s := a.loadBackupSettings()
	if s.Drive == nil {
		return finish(refuse("Choose a backup drive first."))
	}
	device := a.backupDriveDevice(s.Drive.UUID)
	if device == "" {
		return finish(refuse("The backup drive (%s) isn't plugged in. Plug it into the box, then back up again.", driveName(*s.Drive)))
	}
	keyText, err := a.backupKey(false)
	if err != nil {
		return finish(err)
	}
	key, err := parseBackupKey(keyText)
	if err != nil {
		return finish(err)
	}
	stage := a.path(backupStage)
	_ = os.RemoveAll(stage)
	if err := os.MkdirAll(stage, 0o711); err != nil {
		return finish(err)
	}
	defer os.RemoveAll(stage)
	a.ensureBackupRole()

	err = a.withBackupDrive(device, s.Drive.FSType, true, func(dir string) error {
		root := filepath.Join(dir, backupFolder)
		if err := keyMatches(root, key); err != nil {
			return refuse("The drive plugged in has another key's backups. Choose the drive again in Backups.")
		}
		now := a.now()
		today := now.Format("2006-01-02")
		set := BackupSet{Date: now.UTC().Format(time.RFC3339), Box: a.serial(), OS: a.osRelease()["IMAGE_VERSION"], By: job.By}
		name := now.Format("2006-01-02 1504")
		setDir := filepath.Join(root, backupSets, name)
		partial := setDir + ".partial"
		_ = os.RemoveAll(partial)
		if err := os.MkdirAll(partial, 0o700); err != nil {
			return refuse("The box couldn't write to the backup drive. Is it full?")
		}
		index := readCopyIndex(root)
		next := map[string]copyEntry{}
		failed := map[string]bool{}
		for _, part := range a.backupParts(s) {
			step("Backing up " + part.Name)
			done, err := a.backupPartTo(part, root, partial, key, today, index, next)
			if err != nil {
				job.Problems = append(job.Problems, fmt.Sprintf("%s: %s", part.Name, err.Error()))
				set.Missing = append(set.Missing, part.ID)
				failed[part.ID] = true
				continue
			}
			set.Parts = append(set.Parts, done)
		}
		if accounts, err := a.adminAccounts(); err == nil {
			if sealed, err := sealBytes(key, accounts); err == nil {
				_ = os.WriteFile(filepath.Join(partial, "accounts.zbak"), sealed, 0o600)
			}
		}
		// Copies whose files left the box go to Deleted; a failed part's stay.
		step("Keeping deleted files")
		for as, entry := range index {
			if _, ok := next[as]; ok {
				continue
			}
			if failed[entry.App] || !a.partInBackup(entry.App, s) {
				next[as] = entry
				continue
			}
			_ = moveAside(root, backupDeleted, today, as)
		}
		if err := writeCopyIndex(root, next); err != nil {
			return err
		}
		data, _ := json.MarshalIndent(set, "", "  ")
		if err := writeSynced(filepath.Join(partial, "backup.json"), data); err != nil {
			return refuse("The box couldn't write to the backup drive. Is it full?")
		}
		_ = os.RemoveAll(setDir)
		if err := os.Rename(partial, setDir); err != nil {
			return err
		}
		step("Tidying older backups")
		pruneBackupSets(root, now)
		pruneDated(filepath.Join(root, backupDeleted), now)
		pruneDated(filepath.Join(root, backupOlder), now)
		_ = os.WriteFile(filepath.Join(root, "README.txt"), []byte(backupReadme), 0o644)
		return nil
	})
	return finish(err)
}

// partInBackup: the part is still backed up (an app removed or left out has
// its copies kept as they are, not moved to Deleted).
func (a *Agent) partInBackup(id string, s BackupSettings) bool {
	if id == idPart {
		return true
	}
	_, installed := a.loadApps()[id]
	return installed && !contains(s.LeftOut, id)
}

// ensureBackupRole: the role the dumps are made as. It reads every database
// and is no superuser, so row security applies to it.
func (a *Agent) ensureBackupRole() {
	sql := `DO $$ BEGIN
	IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = '` + backupRole + `') THEN
		CREATE ROLE "` + backupRole + `" NOLOGIN;
	END IF;
END $$;
GRANT pg_read_all_data TO "` + backupRole + `";`
	_, _ = a.Run("runuser", "-u", "postgres", "--", "psql", "-v", "ON_ERROR_STOP=1", "-qc", sql)
}

// adminAccounts: the admins' usernames and names, for "I forgot my old
// password" when this backup is brought back.
func (a *Agent) adminAccounts() ([]byte, error) {
	out, err := a.Run("runuser", "-u", "postgres", "--", "psql", "-d", idDatabase, "-qtAc",
		`SELECT coalesce(json_agg(json_build_object('username', username, 'name', coalesce(nullif(trim(display_name), ''), username)) ORDER BY username), '[]') FROM users WHERE role = 'admin' AND is_active`)
	if err != nil {
		return nil, err
	}
	var check []map[string]string
	if json.Unmarshal(out, &check) != nil {
		return nil, errors.New("the accounts couldn't be read")
	}
	return out, nil
}

// backupPartTo backs one part up: its listed files as they are, the rest
// sealed.
func (a *Agent) backupPartTo(part backupPart, root, setDir string, key []byte, today string,
	index, next map[string]copyEntry) (BackupSetPart, error) {
	done := BackupSetPart{ID: part.ID, Name: part.Name, Version: part.Version}
	listed := []listedFile{}
	skip := map[string]bool{}
	if part.List != "" && part.Data != "" {
		if _, err := a.Run("systemctl", "start", part.List); err != nil {
			return done, refuse("it couldn't list its files")
		}
		var err error
		if listed, err = readListedFiles(a.path(filepath.Join(part.Data, backupListFile))); err != nil {
			return done, err
		}
	}
	used := map[string]bool{}
	for i := range listed {
		as, err := a.copyListed(part, &listed[i], root, today, index, next, used)
		if err != nil {
			return done, err
		}
		listed[i].As = as
		if !filepath.IsAbs(listed[i].Path) {
			skip[filepath.Clean(listed[i].Path)] = true
		}
	}
	done.Copies = len(listed)

	// The databases, dumped where only root and postgres reach.
	dumps := map[string]string{}
	for _, db := range part.Databases {
		folder := filepath.Join(a.path(backupStage), db)
		if _, err := a.Run("install", "-d", "-o", "postgres", "-m", "0700", folder); err != nil {
			return done, err
		}
		file := filepath.Join(folder, db+".dump")
		if _, err := a.Run("runuser", "-u", "postgres", "--", "env", "PGOPTIONS=-c zoomriya.backup=on",
			"pg_dump", "-Fc", "--enable-row-security", "--role="+backupRole, "-f", file, db); err != nil {
			return done, refuse("its database couldn't be read")
		}
		dumps[db] = file
	}
	defer func() {
		for _, file := range dumps {
			_ = os.RemoveAll(filepath.Dir(file))
		}
	}()

	base := filepath.Join(setDir, part.ID+".zbak")
	parts := newPartWriter(base, backupPartBytes)
	sealed, err := newSealWriter(parts, key)
	if err != nil {
		return done, err
	}
	tw := tar.NewWriter(sealed)
	write := func() error {
		list, _ := json.Marshal(listed)
		if err := tarBytes(tw, "files.json", list); err != nil {
			return err
		}
		for _, db := range part.Databases {
			if err := tarFile(tw, "databases/"+db+".dump", dumps[db]); err != nil {
				return err
			}
		}
		if part.Data != "" {
			if err := tarTree(tw, a.path(part.Data), "data", skip); err != nil {
				return err
			}
		}
		for _, name := range boxKeyNames(part.ID) {
			if _, err := os.Stat(a.path(filepath.Join(boxKeysDir, name))); err == nil {
				if err := tarFile(tw, "keys/"+name, a.path(filepath.Join(boxKeysDir, name))); err != nil {
					return err
				}
			}
		}
		if err := tw.Close(); err != nil {
			return err
		}
		return sealed.Close()
	}
	err = write()
	if closeErr := parts.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return done, refuse("its data couldn't be written to the drive (%s)", plainError(err))
	}
	for _, p := range parts.Parts {
		done.Files = append(done.Files, filepath.Base(p))
	}
	return done, nil
}

func plainError(err error) string {
	var pathErr *fs.PathError
	if errors.As(err, &pathErr) {
		return pathErr.Err.Error()
	}
	return err.Error()
}

func readListedFiles(path string) ([]listedFile, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, refuse("it didn't list its files")
	}
	defer f.Close()
	listed := []listedFile{}
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64<<10), 1<<20)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var l listedFile
		if json.Unmarshal([]byte(line), &l) != nil || !listedPathValid(l.Path) || l.As == "" {
			return nil, refuse("its list of files isn't readable")
		}
		listed = append(listed, l)
	}
	return listed, scanner.Err()
}

// listedPathValid: a listed file is in the app's folder (a path relative to
// it) or on the library drives (an absolute path under
// /data/zoomriya/volumes/, where Photos keeps the family's originals).
func listedPathValid(p string) bool {
	if filepath.IsAbs(p) {
		clean := filepath.Clean(p)
		return clean == p && strings.HasPrefix(clean, libraryMounts) && len(clean) > len(libraryMounts)
	}
	return safeRelative(p)
}

// listedPlace: where a listed file is, from the box's root.
func listedPlace(data, p string) string {
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	return filepath.Join(data, p)
}

// safeRelative: a path inside a folder, never above it.
func safeRelative(p string) bool {
	clean := filepath.Clean(p)
	return p != "" && !filepath.IsAbs(p) && clean != "." && clean != ".." && !strings.HasPrefix(clean, "../")
}

// driveName: a listed name as the drive can hold it: no characters FAT, exFAT
// or NTFS refuse, no folder above, none of the drive's own folders.
func drivePath(as string) string {
	parts := []string{}
	for _, p := range strings.Split(filepath.ToSlash(as), "/") {
		p = strings.Map(func(r rune) rune {
			if r < 32 || strings.ContainsRune(`<>:"\|?*`, r) {
				return '_'
			}
			return r
		}, p)
		p = strings.TrimRight(strings.TrimSpace(p), ".")
		if p == "" || p == "." || p == ".." {
			continue
		}
		parts = append(parts, p)
	}
	if len(parts) == 0 {
		return ""
	}
	switch parts[0] {
	case backupMeta, backupSets, backupOlder, backupDeleted, "README.txt":
		parts[0] = "_" + parts[0]
	}
	return strings.Join(parts, "/")
}

// copyListed copies one listed file to the drive, unless its copy there is
// already the same; the copy before a change goes to Older versions.
func (a *Agent) copyListed(part backupPart, l *listedFile, root, today string,
	index, next map[string]copyEntry, used map[string]bool) (string, error) {
	as := drivePath(l.As)
	if as == "" {
		return "", refuse("it listed a file with no name")
	}
	// Two files given the same name: the second is "name (2).ext".
	ext := filepath.Ext(as)
	stem := strings.TrimSuffix(as, ext)
	for n := 2; used[strings.ToLower(as)]; n++ {
		as = fmt.Sprintf("%s (%d)%s", stem, n, ext)
	}
	used[strings.ToLower(as)] = true

	src := a.path(listedPlace(part.Data, l.Path))
	info, err := os.Lstat(src)
	if err != nil || !info.Mode().IsRegular() {
		return "", refuse("%s is listed but isn't there", l.Path)
	}
	dest := filepath.Join(root, filepath.FromSlash(as))
	if old, ok := index[as]; ok && old.App == part.ID && old.Path == l.Path &&
		old.Size == info.Size() && old.Modified == info.ModTime().Unix() {
		if have, err := os.Stat(dest); err == nil && have.Size() == old.Size {
			next[as] = old
			l.SHA256, l.Size = old.SHA256, old.Size
			return as, nil
		}
	}
	if _, err := os.Lstat(dest); err == nil {
		if err := moveAside(root, backupOlder, today, as); err != nil {
			return "", refuse("the copy of %s couldn't be kept aside", as)
		}
	}
	sum, err := copyChecked(src, dest)
	if err != nil {
		return "", err
	}
	next[as] = copyEntry{App: part.ID, Path: l.Path, Size: info.Size(), Modified: info.ModTime().Unix(), SHA256: sum}
	l.SHA256, l.Size = sum, info.Size()
	return as, nil
}

// copyChecked copies a file, syncs it, reads the copy back and checks it
// matches; its SHA-256.
func copyChecked(src, dest string) (string, error) {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return "", refuse("the backup drive couldn't take a folder (is it full?)")
	}
	in, err := os.Open(src)
	if err != nil {
		return "", err
	}
	defer in.Close()
	temp := dest + ".part"
	out, err := os.OpenFile(temp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return "", refuse("the backup drive couldn't take a file (is it full?)")
	}
	hash := sha256.New()
	_, err = io.Copy(io.MultiWriter(out, hash), in)
	if err == nil {
		err = out.Sync()
	}
	if closeErr := out.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		os.Remove(temp)
		return "", refuse("the backup drive couldn't take a file (is it full?)")
	}
	want := hex.EncodeToString(hash.Sum(nil))
	if got, err := fileSHA256(temp); err != nil || got != want {
		os.Remove(temp)
		return "", refuse("a copy on the backup drive didn't read back the same: the drive may be failing")
	}
	if err := os.Rename(temp, dest); err != nil {
		os.Remove(temp)
		return "", err
	}
	return want, nil
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// moveAside moves a copy to <kind>/<today>/<as>; one already there that day
// is replaced by the newer one (the day's last copy before the change).
func moveAside(root, kind, today, as string) error {
	from := filepath.Join(root, filepath.FromSlash(as))
	to := filepath.Join(root, kind, today, filepath.FromSlash(as))
	if _, err := os.Lstat(from); err != nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
		return err
	}
	_ = os.Remove(to)
	return os.Rename(from, to)
}

func readCopyIndex(root string) map[string]copyEntry {
	index := map[string]copyEntry{}
	if data, err := os.ReadFile(filepath.Join(root, backupMeta, "copies.json")); err == nil {
		_ = json.Unmarshal(data, &index)
	}
	return index
}

func writeCopyIndex(root string, index map[string]copyEntry) error {
	data, _ := json.Marshal(index)
	return writeSynced(filepath.Join(root, backupMeta, "copies.json"), data)
}

// writeSynced writes a file whole, then renames it into place.
func writeSynced(path string, data []byte) error {
	temp := path + ".part"
	f, err := os.OpenFile(temp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	if err == nil {
		err = f.Sync()
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		os.Remove(temp)
		return err
	}
	return os.Rename(temp, path)
}

// --- the archive -------------------------------------------------------------------

func tarBytes(tw *tar.Writer, name string, data []byte) error {
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o600, Size: int64(len(data)), Typeflag: tar.TypeReg,
		ModTime: time.Now()}); err != nil {
		return err
	}
	_, err := tw.Write(data)
	return err
}

func tarFile(tw *tar.Writer, name, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o600, Size: info.Size(), Typeflag: tar.TypeReg,
		ModTime: info.ModTime()}); err != nil {
		return err
	}
	_, err = io.Copy(tw, f)
	return err
}

// tarTree adds a folder under prefix, with owners, modes and times, leaving
// out the files in skip (relative to it).
func tarTree(tw *tar.Writer, dir, prefix string, skip map[string]bool) error {
	return filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, path)
		if skip[rel] {
			return nil
		}
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		link := ""
		if info.Mode()&fs.ModeSymlink != 0 {
			link, _ = os.Readlink(path)
		} else if !info.Mode().IsRegular() && !info.IsDir() {
			return nil // sockets and pipes aren't data
		}
		header, err := tar.FileInfoHeader(info, link)
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(filepath.Join(prefix, rel))
		if info.IsDir() {
			header.Name += "/"
		}
		header.Uname, header.Gname = "", ""
		if err := tw.WriteHeader(header); err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.CopyN(tw, f, info.Size())
		return err
	})
}

// --- keeping and letting go ------------------------------------------------------------

// pruneBackupSets keeps every backup of the last 14 days and the first of
// each month for a year; the newest is always kept. Folders cut short
// (.partial) go.
func pruneBackupSets(root string, now time.Time) {
	dir := filepath.Join(root, backupSets)
	entries, _ := os.ReadDir(dir)
	names := []string{}
	for _, e := range entries {
		switch {
		case strings.HasSuffix(e.Name(), ".partial"):
			_ = os.RemoveAll(filepath.Join(dir, e.Name()))
		case setPattern.MatchString(e.Name()):
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	months := map[string]bool{}
	for i, name := range names {
		at, err := time.ParseInLocation("2006-01-02 1504", name, now.Location())
		if err != nil {
			continue
		}
		month := name[:7]
		firstOfMonth := !months[month]
		months[month] = true
		keep := i == len(names)-1 ||
			now.Sub(at) < keepDailyDays*24*time.Hour ||
			(firstOfMonth && now.Sub(at) < keepMonths*31*24*time.Hour)
		if !keep {
			_ = os.RemoveAll(filepath.Join(dir, name))
		}
	}
}

// pruneDated removes <dir>/<YYYY-MM-DD> folders older than 30 days.
func pruneDated(dir string, now time.Time) {
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		day, err := time.ParseInLocation("2006-01-02", e.Name(), now.Location())
		if err == nil && now.Sub(day) > backupKeepDays*24*time.Hour {
			_ = os.RemoveAll(filepath.Join(dir, e.Name()))
		}
	}
}
