package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	edKey  = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOMqqnkVzrm0SdG6UOoqKLsabgH5C9okWi0dh2l9GKJl anna@laptop"
	edFp   = "SHA256:q8fV2mH0bWQyZ3Rk9cNnXlP1sE4uJ7aD6tY5rB2oKiM"
	rsaKey = "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC7 old@pc"
)

type sentNotes struct{ sent []string }

func (n *sentNotes) Notify(kind, level, message string) {
	n.sent = append(n.sent, kind+" "+level+": "+message)
}

// accessBox: ssh-keygen as it would answer for the keys above, and sticks as
// lsblk would list them.
func accessBox(t *testing.T) (*Agent, *fakeBox, *sentNotes) {
	agent, box := newFakeBox(t)
	agent.Now = func() time.Time { return time.Date(2026, 11, 8, 10, 0, 0, 0, time.UTC) }
	agent.Sleep = func(time.Duration) {}
	sent := &sentNotes{}
	agent.Notes = sent
	box.write("etc/hostname", "zoomriya-7k2q\n")
	check := filepath.Join(box.root, keyCheckFile)
	fake := box.run
	agent.Run = func(name string, args ...string) ([]byte, error) {
		line := strings.Join(append([]string{name}, args...), " ")
		switch {
		case line == "ssh-keygen -l -f "+check:
			data, _ := os.ReadFile(check)
			box.ran = append(box.ran, append([]string{name}, args...))
			if strings.HasPrefix(string(data), "ssh-rsa") {
				return []byte("2048 SHA256:oldRSAkeyFingerprint old@pc (RSA)\n"), nil
			}
			return []byte("256 " + edFp + " anna@laptop (ED25519)\n"), nil
		case strings.HasPrefix(line, "lsblk --json --bytes --paths"):
			box.ran = append(box.ran, append([]string{name}, args...))
			return []byte(box.answers["sticks"]), nil
		}
		return fake(name, args...)
	}
	return agent, box, sent
}

func TestAdvancedAccess(t *testing.T) {
	agent, box, sent := accessBox(t)

	if _, err := agent.Handle(Request{Op: "advanced-access", Action: str("on"), By: "amma"}); err == nil {
		t.Error("turned on with no key")
	}
	for _, bad := range []string{"", "not a key", edKey + "\n" + edKey, "-----BEGIN OPENSSH PRIVATE KEY-----", rsaKey,
		`command="rm -rf /" ` + edKey} {
		if _, err := agent.Handle(Request{Op: "advanced-access", Action: str("key-add"), Key: str(bad)}); err == nil {
			t.Errorf("added %q", bad)
		}
	}

	resp, err := agent.Handle(Request{Op: "advanced-access", Action: str("key-add"), Key: str(edKey), Name: str("Anna's laptop"), By: "amma"})
	if err != nil {
		t.Fatal(err)
	}
	keys := resp["keys"].([]AccessKey)
	if len(keys) != 1 || keys[0].Name != "Anna's laptop" || keys[0].Fingerprint != edFp || keys[0].Type != "ED25519" || keys[0].Line != "" {
		t.Errorf("keys = %+v", keys)
	}
	if file, _ := os.ReadFile(filepath.Join(box.root, accessKeysFile)); string(file) != edKey+"\n" {
		t.Errorf("authorized_keys = %q", file)
	}
	if _, err := agent.Handle(Request{Op: "advanced-access", Action: str("key-add"), Key: str(edKey)}); err == nil {
		t.Error("added the same key twice")
	}

	resp, err = agent.Handle(Request{Op: "advanced-access", Action: str("on"), By: "amma"})
	if err != nil {
		t.Fatal(err)
	}
	if resp["on"] != true || resp["by"] != "amma" || resp["address"] != "zoomriya-7k2q.local" {
		t.Errorf("on: %v", resp)
	}
	for _, want := range []string{
		"ssh-keygen -q -t ed25519 -N  -C  -f " + filepath.Join(box.root, hostKey),
		"nft flush chain inet zoomriya remote_access",
		"nft add rule inet zoomriya remote_access iifname en* ip saddr { 10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16 } tcp dport 22 accept",
		"nft add rule inet zoomriya remote_access iifname en* ip6 saddr { fe80::/10, fc00::/7 } tcp dport 22 accept",
		"systemctl start ssh.service",
	} {
		if !ranCommand(box, want) {
			t.Errorf("didn't run %s", want)
		}
	}
	if status, _ := agent.Handle(Request{Op: "status"}); status["advanced_access"] != true {
		t.Errorf("status's advanced_access = %v", status["advanced_access"])
	}

	// A sign-in shows on its key.
	box.journal = "2026-11-08T10:05:00+0000 zoomriya-7k2q sshd-session[812]: Accepted publickey for admin from 192.168.1.20 port 50122 ssh2: ED25519 " + edFp + "\n"
	resp, _ = agent.Handle(Request{Op: "advanced-access"})
	if k := resp["keys"].([]AccessKey); k[0].LastUsed != "2026-11-08T10:05:00+0000" {
		t.Errorf("last used = %q", k[0].LastUsed)
	}

	// At each start, it's put back as it was.
	box.ran = nil
	if err := agent.sshApply(); err != nil || !ranCommand(box, "systemctl start ssh.service") {
		t.Errorf("apply while on: %v, ran %v", err, box.ran)
	}

	box.ran = nil
	resp, err = agent.Handle(Request{Op: "advanced-access", Action: str("off"), By: "amma"})
	if err != nil || resp["on"] != false {
		t.Fatal(resp, err)
	}
	for _, want := range []string{"nft flush chain inet zoomriya remote_access", "systemctl stop ssh.service ssh.socket",
		"loginctl terminate-user admin"} {
		if !ranCommand(box, want) {
			t.Errorf("didn't run %s", want)
		}
	}
	if len(sent.sent) != 2 || !strings.HasPrefix(sent.sent[0], "access warning:") || !strings.HasPrefix(sent.sent[1], "access info:") {
		t.Errorf("notifications: %v", sent.sent)
	}

	resp, err = agent.Handle(Request{Op: "advanced-access", Action: str("key-remove"), Fingerprint: str(edFp)})
	if err != nil || len(resp["keys"].([]AccessKey)) != 0 {
		t.Errorf("remove: %v %v", resp, err)
	}
}

func TestAdvancedAccessReset(t *testing.T) {
	agent, box, _ := accessBox(t)
	if _, err := agent.Handle(Request{Op: "advanced-access", Action: str("key-add"), Key: str(edKey)}); err != nil {
		t.Fatal(err)
	}
	if _, err := agent.Handle(Request{Op: "advanced-access", Action: str("on")}); err != nil {
		t.Fatal(err)
	}
	if err := agent.accessReset(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(box.root, accessDir)); !os.IsNotExist(err) {
		t.Error("the keys are still there after a reset")
	}
	if resp, _ := agent.Handle(Request{Op: "advanced-access"}); resp["on"] != false {
		t.Errorf("still on: %v", resp)
	}
}

const sticksJSON = `{"blockdevices": [
  {"name": "/dev/vda", "type": "disk", "tran": null, "fstype": null, "label": null, "size": 64000000000, "mountpoints": [null],
   "children": [{"name": "/dev/vda2", "type": "part", "tran": null, "fstype": "btrfs", "label": null, "size": 60000000000, "mountpoints": ["/"]}]},
  {"name": "/dev/sdb", "type": "disk", "tran": "usb", "fstype": null, "label": null, "size": 32000000000, "mountpoints": [null],
   "children": [{"name": "/dev/sdb1", "type": "part", "tran": null, "fstype": "vfat", "label": "ANNA", "size": 32000000000, "mountpoints": [null]},
                {"name": "/dev/sdb2", "type": "part", "tran": null, "fstype": "crypto_LUKS", "label": null, "size": 1000, "mountpoints": [null]}]},
  {"name": "/dev/sdc", "type": "disk", "tran": "usb", "fstype": "exfat", "label": "BUSY", "size": 8000000000, "mountpoints": ["/media/x"]}
]}`

func TestUSBKeys(t *testing.T) {
	agent, box, _ := accessBox(t)
	box.answers = map[string]string{"sticks": sticksJSON}
	sticks, err := agent.usbSticks()
	if err != nil {
		t.Fatal(err)
	}
	if len(sticks) != 1 || sticks[0].Device != "/dev/sdb1" || sticks[0].Label != "ANNA" {
		t.Fatalf("sticks = %+v", sticks)
	}

	// What the stick has, seen where the agent mounts it.
	box.write(usbMount+"/zoomriya/ssh/anna-laptop.pub", edKey+"\n")
	box.write(usbMount+"/zoomriya/ssh/notes.txt", "hello\n")
	box.write(usbMount+"/zoomriya/ssh/broken.pub", "nonsense\n")
	if err := os.Symlink("/etc/shadow", filepath.Join(box.root, usbMount, "zoomriya/ssh/link.pub")); err != nil {
		t.Fatal(err)
	}
	resp, err := agent.Handle(Request{Op: "advanced-access", Action: str("usb-keys")})
	if err != nil {
		t.Fatal(err)
	}
	found := resp["found"].([]FoundKey)
	if len(found) != 1 || found[0].Name != "anna-laptop" || found[0].Key != edKey || found[0].Stick != "ANNA" {
		t.Errorf("found = %+v", found)
	}
	if !ranCommand(box, "mount -t vfat -o nosuid,nodev,noexec,noatime,ro /dev/sdb1 "+filepath.Join(box.root, usbMount)) ||
		!ranCommand(box, "umount "+filepath.Join(box.root, usbMount)) {
		t.Errorf("ran %v", box.ran)
	}
	// Found isn't added: the admin adds it.
	if len(agent.accessKeys()) != 0 {
		t.Error("a key on a stick was added by itself")
	}
}
