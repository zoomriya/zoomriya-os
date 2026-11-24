// Advanced access (design §7.1): SSH for the box's own admins, as `admin`
// with sudo, by key only, from the home network only. Off by default; an
// admin turns it on in Box Desktop and it stays on until an admin turns it
// off. Its keys are kept under /data, so restoring the OS doesn't lock anyone
// out; Reset the system and Erase everything remove them.
//
// USB sticks are here too: listing them, and mounting one for a moment to
// read SSH keys from it or to save a diagnostics bundle to it.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	accessDir      = "data/zoomriya-os/ssh"
	accessKeysFile = accessDir + "/authorized_keys" // sshd's AuthorizedKeysFile
	accessKeysMeta = accessDir + "/keys.json"
	accessState    = accessDir + "/on.json"
	accessUser     = "admin"
	accessMaxKeys  = 20
	// The box's own host key, made the first time advanced access is on.
	hostKey = "etc/ssh/ssh_host_ed25519_key"
	// Where a key is checked with ssh-keygen before it's added.
	keyCheckFile = "run/zoomriya/key-check.pub"
	// Where a USB stick is mounted for a moment.
	usbMount = "run/zoomriya/usb"
	// The folder on a stick that keys are read from.
	usbKeysFolder = "zoomriya/ssh"
	usbKeyMax     = 16 << 10
)

// A public key's line: its type, the key, and maybe a comment. Options aren't
// taken: the agent writes the file.
var keyLine = regexp.MustCompile(`^(ssh-ed25519|ecdsa-sha2-nistp(?:256|384|521)|sk-ssh-ed25519@openssh\.com|sk-ecdsa-sha2-nistp256@openssh\.com|ssh-rsa) ([A-Za-z0-9+/]+={0,2})(?: (.*))?$`)

// ssh-keygen -l's line: "256 SHA256:… comment (ED25519)".
var keygenLine = regexp.MustCompile(`^(\d+) (SHA256:[A-Za-z0-9+/]+) .*\(([A-Z0-9-]+)\)\s*$`)

// sshd's line for a sign-in by key.
var acceptedLine = regexp.MustCompile(`^(\S+) .*Accepted publickey for ` + accessUser + ` from (\S+) port \d+ \S+: \S+ (SHA256:[A-Za-z0-9+/]+)`)

// The firewall's remote_access chain (zoomriya-base's nftables.conf) while
// advanced access is on: SSH from private addresses on the Ethernet ports,
// never through ZOOMRIYA Connect or from the internet.
var remoteAccessRules = [][]string{
	{"iifname", "en*", "ip", "saddr", "{ 10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16 }", "tcp", "dport", "22", "accept"},
	{"iifname", "en*", "ip6", "saddr", "{ fe80::/10, fc00::/7 }", "tcp", "dport", "22", "accept"},
}

type AccessKey struct {
	Name        string `json:"name"`
	Fingerprint string `json:"fingerprint"`
	Type        string `json:"type"`
	Bits        int    `json:"bits"`
	Added       string `json:"added"`
	By          string `json:"by"`
	LastUsed    string `json:"last_used,omitempty"`
	Line        string `json:"line,omitempty"`
}

type accessOn struct {
	Since string `json:"since"`
	By    string `json:"by"`
}

func (a *Agent) notify(kind, level, message string) {
	if a.Notes != nil {
		a.Notes.Notify(kind, level, message)
	}
}

func (a *Agent) accessIsOn() *accessOn {
	data, err := os.ReadFile(a.path(accessState))
	if err != nil {
		return nil
	}
	var on accessOn
	if json.Unmarshal(data, &on) != nil {
		return nil
	}
	return &on
}

func (a *Agent) accessKeys() []AccessKey {
	keys := []AccessKey{}
	if data, err := os.ReadFile(a.path(accessKeysMeta)); err == nil {
		_ = json.Unmarshal(data, &keys)
	}
	return keys
}

// saveKeys writes the keys' list and sshd's file from it.
func (a *Agent) saveKeys(keys []AccessKey) error {
	data, _ := json.MarshalIndent(keys, "", "  ")
	if err := writeFileAtomic(a.path(accessKeysMeta), data, 0o600); err != nil {
		return err
	}
	var lines strings.Builder
	for _, k := range keys {
		lines.WriteString(k.Line + "\n")
	}
	return writeFileAtomic(a.path(accessKeysFile), []byte(lines.String()), 0o644)
}

// advancedAccess: no action for how it is; "on", "off"; "key-add" (Key and
// Name), "key-remove" (Fingerprint); "usb-keys" reads the keys on the USB
// sticks plugged in, to add with key-add.
func (a *Agent) advancedAccess(req Request) (Response, error) {
	str := func(p *string) string {
		if p == nil {
			return ""
		}
		return strings.TrimSpace(*p)
	}
	switch str(req.Action) {
	case "":
		return a.accessStatus(), nil
	case "on":
		if len(a.accessKeys()) == 0 {
			return nil, refuse("Add an SSH key first.")
		}
		on := accessOn{Since: a.now().UTC().Format(time.RFC3339), By: req.By}
		if before := a.accessIsOn(); before != nil {
			on = *before
		}
		data, _ := json.Marshal(on)
		if err := writeFileAtomic(a.path(accessState), data, 0o600); err != nil {
			return nil, err
		}
		if err := a.openSSH(); err != nil {
			_ = os.Remove(a.path(accessState))
			_ = a.closeSSH()
			return nil, err
		}
		a.notify("access", "warning", fmt.Sprintf("Advanced access was turned on by %s. Admins with a key can sign in over SSH from the home network.", req.By))
		return a.accessStatus(), nil
	case "off":
		wasOn := a.accessIsOn() != nil
		if err := os.Remove(a.path(accessState)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		if err := a.closeSSH(); err != nil {
			return nil, err
		}
		if wasOn {
			a.notify("access", "info", fmt.Sprintf("Advanced access was turned off by %s.", req.By))
		}
		return a.accessStatus(), nil
	case "key-add":
		key, err := a.checkKey(str(req.Key))
		if err != nil {
			return nil, err
		}
		keys := a.accessKeys()
		for _, k := range keys {
			if k.Fingerprint == key.Fingerprint {
				return nil, refuse("That key is already added, as %q.", k.Name)
			}
		}
		if len(keys) >= accessMaxKeys {
			return nil, refuse("There are already %d keys. Remove one first.", accessMaxKeys)
		}
		key.Name = keyName(str(req.Name), key.Name)
		key.Added, key.By = a.now().UTC().Format(time.RFC3339), req.By
		if err := a.saveKeys(append(keys, key)); err != nil {
			return nil, err
		}
		return a.accessStatus(), nil
	case "key-remove":
		keys, kept := a.accessKeys(), []AccessKey{}
		for _, k := range keys {
			if k.Fingerprint != str(req.Fingerprint) {
				kept = append(kept, k)
			}
		}
		if len(kept) == len(keys) {
			return nil, refuse("There's no such key.")
		}
		if err := a.saveKeys(kept); err != nil {
			return nil, err
		}
		return a.accessStatus(), nil
	case "usb-keys":
		found, err := a.usbKeys()
		if err != nil {
			return nil, err
		}
		return Response{"found": found}, nil
	}
	return nil, refuse("Turn advanced access on or off, or add or remove a key?")
}

// keyName: what the admin typed, else the key's comment, else its type; on
// one line and short.
func keyName(typed, comment string) string {
	name := typed
	if name == "" {
		name = comment
	}
	name = strings.Join(strings.Fields(name), " ")
	if r := []rune(name); len(r) > 40 {
		name = string(r[:40])
	}
	if name == "" {
		name = "key"
	}
	return name
}

// checkKey: one public key of a kind design §7.1 allows, RSA of 3072 bits or
// more, with its fingerprint from ssh-keygen.
func (a *Agent) checkKey(text string) (AccessKey, error) {
	lines := []string{}
	for _, l := range strings.Split(strings.TrimSpace(text), "\n") {
		if l = strings.TrimSpace(l); l != "" && !strings.HasPrefix(l, "#") {
			lines = append(lines, l)
		}
	}
	if len(lines) != 1 {
		return AccessKey{}, refuse("Paste one public key, on one line.")
	}
	m := keyLine.FindStringSubmatch(lines[0])
	if m == nil {
		if strings.Contains(lines[0], "PRIVATE KEY") {
			return AccessKey{}, refuse("That's a private key. Paste the .pub file instead.")
		}
		return AccessKey{}, refuse("That isn't a public key the box takes.")
	}
	line := m[1] + " " + m[2]
	if m[3] != "" {
		line += " " + strings.Join(strings.Fields(m[3]), " ")
	}
	if err := writeFileAtomic(a.path(keyCheckFile), []byte(line+"\n"), 0o600); err != nil {
		return AccessKey{}, err
	}
	defer os.Remove(a.path(keyCheckFile))
	out, err := a.Run("ssh-keygen", "-l", "-f", a.path(keyCheckFile))
	if err != nil {
		return AccessKey{}, refuse("That key isn't valid.")
	}
	g := keygenLine.FindStringSubmatch(strings.TrimSpace(string(out)))
	if g == nil {
		return AccessKey{}, refuse("That key isn't valid.")
	}
	bits, _ := strconv.Atoi(g[1])
	if g[3] == "RSA" && bits < 3072 {
		return AccessKey{}, refuse("RSA keys need 3072 bits or more (this one has %d). Ed25519 is better.", bits)
	}
	return AccessKey{Name: m[3], Fingerprint: g[2], Type: g[3], Bits: bits, Line: line}, nil
}

func (a *Agent) accessStatus() Response {
	keys := a.accessKeys()
	used := a.lastSignIns()
	for i := range keys {
		keys[i].LastUsed = used[keys[i].Fingerprint]
		keys[i].Line = ""
	}
	resp := Response{"on": false, "since": nil, "by": nil, "keys": keys, "user": accessUser,
		"address": a.boxAddress(), "host_key": a.hostKeyFingerprint()}
	if on := a.accessIsOn(); on != nil {
		resp["on"], resp["since"], resp["by"] = true, on.Since, on.By
	}
	return resp
}

// lastSignIns: each key's last sign-in, from sshd's log.
func (a *Agent) lastSignIns() map[string]string {
	used := map[string]string{}
	out, err := a.Run("journalctl", "--no-pager", "--quiet", "--output=short-iso", "--unit=ssh.service",
		"--grep=Accepted publickey for "+accessUser)
	if err != nil {
		return used
	}
	for _, line := range strings.Split(string(out), "\n") {
		if m := acceptedLine.FindStringSubmatch(line); m != nil {
			used[m[3]] = m[1] // oldest first: the last one stays
		}
	}
	return used
}

// boxAddress is the box's name on the home network.
func (a *Agent) boxAddress() string {
	name := a.readTrimmed("etc/hostname")
	if name == "" {
		return ""
	}
	return name + ".local"
}

// hostKeyFingerprint: what an admin compares on first connecting.
func (a *Agent) hostKeyFingerprint() string {
	if _, err := os.Stat(a.path(hostKey + ".pub")); err != nil {
		return ""
	}
	out, err := a.Run("ssh-keygen", "-l", "-f", a.path(hostKey+".pub"))
	if err != nil {
		return ""
	}
	if fields := strings.Fields(string(out)); len(fields) > 1 {
		return fields[1]
	}
	return ""
}

// openSSH: the box's own host key, SSH let in from the home network, sshd.
func (a *Agent) openSSH() error {
	if _, err := os.Stat(a.path(hostKey)); err != nil {
		if _, err := a.Run("ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-C", "", "-f", a.path(hostKey)); err != nil {
			return err
		}
	}
	if _, err := a.Run("nft", "flush", "chain", "inet", "zoomriya", "remote_access"); err != nil {
		return err
	}
	for _, rule := range remoteAccessRules {
		if _, err := a.Run("nft", append([]string{"add", "rule", "inet", "zoomriya", "remote_access"}, rule...)...); err != nil {
			return err
		}
	}
	_, err := a.Run("systemctl", "start", "ssh.service")
	return err
}

// closeSSH: SSH shut out, sshd stopped, admin's sessions ended; each step
// tried even when one before it fails.
func (a *Agent) closeSSH() error {
	var first error
	_, err := a.Run("nft", "flush", "chain", "inet", "zoomriya", "remote_access")
	first = err
	if _, err := a.Run("systemctl", "stop", "ssh.service", "ssh.socket"); err != nil && first == nil {
		first = err
	}
	_, _ = a.Run("loginctl", "terminate-user", accessUser) // no sessions is fine
	return first
}

// sshApply puts SSH as advanced access says: at each start and each firewall
// reload (zoomriya-ssh-apply.service), which empties remote_access.
func (a *Agent) sshApply() error {
	if a.accessIsOn() != nil {
		return a.openSSH()
	}
	return a.closeSSH()
}

func sshApplyJob() int {
	if err := NewAgent().sshApply(); err != nil {
		NewJournal().Record(AuditEntry{Op: "ssh-apply", By: "advanced access", Result: err.Error()})
		return 1
	}
	return 0
}

// accessReset: Reset the system and Erase everything leave no keys and SSH off.
func (a *Agent) accessReset() error {
	if err := os.RemoveAll(a.path(accessDir)); err != nil {
		return err
	}
	return a.closeSSH()
}

func writeFileAtomic(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	temp := path + ".new"
	if err := os.WriteFile(temp, data, mode); err != nil {
		return err
	}
	if err := os.Chmod(temp, mode); err != nil {
		return err
	}
	return os.Rename(temp, path)
}

// --- USB sticks -----------------------------------------------------------------

type Stick struct {
	Device string `json:"device"`
	Label  string `json:"label"`
	FSType string `json:"fstype"`
	Size   int64  `json:"size_bytes"`
}

// The filesystems a stick may have, and the kernel's name for each.
var stickFilesystems = map[string]string{"vfat": "vfat", "exfat": "exfat", "ntfs": "ntfs3", "ext4": "ext4"}

type lsblkTree struct {
	Name        string      `json:"name"`
	Type        string      `json:"type"`
	Tran        *string     `json:"tran"`
	FSType      *string     `json:"fstype"`
	Label       *string     `json:"label"`
	Size        int64       `json:"size"`
	Mountpoints []*string   `json:"mountpoints"`
	Children    []lsblkTree `json:"children"`
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// usbSticks: the partitions (or whole sticks) on USB drives with a
// filesystem the box can read, not mounted anywhere.
func (a *Agent) usbSticks() ([]Stick, error) {
	out, err := a.Run("lsblk", "--json", "--bytes", "--paths", "--output",
		"NAME,TYPE,TRAN,FSTYPE,LABEL,SIZE,MOUNTPOINTS")
	if err != nil {
		return nil, err
	}
	var list struct {
		Blockdevices []lsblkTree `json:"blockdevices"`
	}
	if err := json.Unmarshal(out, &list); err != nil {
		return nil, fmt.Errorf("lsblk's list: %w", err)
	}
	system := a.systemDisk()
	sticks := []Stick{}
	for _, disk := range list.Blockdevices {
		if disk.Type != "disk" || deref(disk.Tran) != "usb" || filepath.Base(disk.Name) == system {
			continue
		}
		for _, part := range append([]lsblkTree{disk}, disk.Children...) {
			if _, ok := stickFilesystems[deref(part.FSType)]; !ok {
				continue
			}
			mounted := false
			for _, m := range part.Mountpoints {
				mounted = mounted || m != nil
			}
			if !mounted {
				sticks = append(sticks, Stick{Device: part.Name, Label: deref(part.Label), FSType: deref(part.FSType), Size: part.Size})
			}
		}
	}
	return sticks, nil
}

// withStick mounts a stick for fn, read-only unless writable, never running
// anything from it, and unmounts it after.
func (a *Agent) withStick(device string, writable bool, fn func(dir string) error) error {
	sticks, err := a.usbSticks()
	if err != nil {
		return err
	}
	var stick *Stick
	for i := range sticks {
		if sticks[i].Device == device {
			stick = &sticks[i]
		}
	}
	if stick == nil {
		return refuse("That USB stick isn't plugged in any more.")
	}
	dir := a.path(usbMount)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	options := "nosuid,nodev,noexec,noatime"
	if !writable {
		options += ",ro"
	}
	if _, err := a.Run("mount", "-t", stickFilesystems[stick.FSType], "-o", options, stick.Device, dir); err != nil {
		return refuse("The box couldn't open that USB stick.")
	}
	defer a.Run("umount", dir)
	return fn(dir)
}

type FoundKey struct {
	AccessKey
	Stick string `json:"stick"`
	File  string `json:"file"`
	Key   string `json:"key"`
}

// usbKeys reads zoomriya/ssh/*.pub on each stick: regular files only, small,
// nothing run from them. Adding one is key-add, after the admin says so.
func (a *Agent) usbKeys() ([]FoundKey, error) {
	sticks, err := a.usbSticks()
	if err != nil {
		return nil, err
	}
	found := []FoundKey{}
	for _, stick := range sticks {
		_ = a.withStick(stick.Device, false, func(dir string) error {
			folder := filepath.Join(dir, usbKeysFolder)
			entries, _ := os.ReadDir(folder)
			sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
			for _, e := range entries {
				info, err := os.Lstat(filepath.Join(folder, e.Name()))
				if err != nil || !info.Mode().IsRegular() || info.Size() > usbKeyMax || !strings.HasSuffix(e.Name(), ".pub") {
					continue
				}
				data, _ := os.ReadFile(filepath.Join(folder, e.Name()))
				key, err := a.checkKey(string(data))
				if err != nil {
					continue
				}
				line := key.Line
				key.Name = keyName("", strings.TrimSuffix(e.Name(), ".pub"))
				key.Line = ""
				found = append(found, FoundKey{AccessKey: key, Stick: stick.Label, File: e.Name(), Key: line})
				if len(found) >= accessMaxKeys {
					break
				}
			}
			return nil
		})
	}
	return found, nil
}
