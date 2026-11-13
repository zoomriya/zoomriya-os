package main

import (
	"strings"
	"testing"
)

const lsblkDrives = `{"blockdevices": [
  {"name": "vda", "path": "/dev/vda", "type": "disk", "size": 128035676160, "model": null, "serial": null, "tran": null},
  {"name": "nvme0n1", "path": "/dev/nvme0n1", "type": "disk", "size": 8001563222016, "model": "WD Red SN700 8TB ", "serial": "ZLIB0001", "tran": "nvme"},
  {"name": "sda", "path": "/dev/sda", "type": "disk", "size": 8001563222016, "model": "ST8000VN004", "serial": "ZA1B2C3D", "tran": "sata"},
  {"name": "zram0", "path": "/dev/zram0", "type": "disk", "size": 1073741824, "model": null, "serial": null, "tran": null},
  {"name": "sr0", "path": "/dev/sr0", "type": "rom", "size": 1073741824, "model": null, "serial": null, "tran": "sata"}
]}`

const nvmeHealthy = `{"smart_status": {"passed": true}, "temperature": {"current": 38}, "power_on_time": {"hours": 10321},
  "nvme_smart_health_information_log": {"critical_warning": 0, "available_spare": 100, "available_spare_threshold": 10, "percentage_used": 4, "media_errors": 0}}`

const sataHealthy = `{"smart_status": {"passed": true}, "temperature": {"current": 41}, "power_on_time": {"hours": 10200},
  "ata_smart_attributes": {"table": [{"id": 5, "raw": {"value": 0}}, {"id": 197, "raw": {"value": 0}}, {"id": 198, "raw": {"value": 0}}]}}`

// A virtio disk, as in a VM: smartctl can't even tell what it is.
const noSmart = `{"smartctl": {"exit_status": 1, "messages": [{"string": "/dev/vda: Unable to detect device type", "severity": "error"}]}}`

func drivesFrom(t *testing.T, agent *Agent) map[string]Drive {
	t.Helper()
	resp, err := agent.Handle(Request{Op: "disk-health"})
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]Drive{}
	for _, d := range resp["drives"].([]Drive) {
		byName[d.Name] = d
	}
	return byName
}

func TestDiskHealth(t *testing.T) {
	agent, box := newFakeBox(t)
	box.lsblk = lsblkDrives
	box.smart = map[string]string{"/dev/vda": noSmart, "/dev/nvme0n1": nvmeHealthy, "/dev/sda": sataHealthy}
	drives := drivesFrom(t, agent)

	if len(drives) != 3 {
		t.Fatalf("drives = %v, want vda, nvme0n1 and sda (not zram or the optical drive)", drives)
	}
	if d := drives["vda"]; d.Health != healthUnknown || !d.System {
		t.Errorf("vda = %+v, want unknown, the system drive", d)
	}
	nvme := drives["nvme0n1"]
	if nvme.Health != healthHealthy || nvme.System || nvme.Model != "WD Red SN700 8TB" ||
		*nvme.TemperatureC != 38 || *nvme.WearPercent != 4 || *nvme.PowerOnHours != 10321 {
		t.Errorf("nvme0n1 = %+v", nvme)
	}
	if d := drives["sda"]; d.Health != healthHealthy || d.WearPercent != nil {
		t.Errorf("sda = %+v", d)
	}
}

func TestDiskHealthWords(t *testing.T) {
	cases := map[string]struct {
		report string
		health string
		reason string
	}{
		"SMART failed": {`{"smart_status": {"passed": false}}`, healthReplaceNow, "own check failed"},
		"NVMe critical warning": {`{"smart_status": {"passed": true}, "nvme_smart_health_information_log": {"critical_warning": 4, "available_spare": 100, "available_spare_threshold": 10}}`,
			healthReplaceNow, "critical warning"},
		"worn out": {`{"smart_status": {"passed": true}, "nvme_smart_health_information_log": {"percentage_used": 103, "available_spare": 100, "available_spare_threshold": 10}}`,
			healthReplaceNow, "end of its rated life"},
		"no spare blocks left": {`{"smart_status": {"passed": true}, "nvme_smart_health_information_log": {"available_spare": 5, "available_spare_threshold": 10}}`,
			healthReplaceNow, "spare blocks"},
		"mostly worn": {`{"smart_status": {"passed": true}, "nvme_smart_health_information_log": {"percentage_used": 85, "available_spare": 100, "available_spare_threshold": 10}}`,
			healthCheckSoon, "85% through"},
		"media errors": {`{"smart_status": {"passed": true}, "nvme_smart_health_information_log": {"media_errors": 3, "available_spare": 100, "available_spare_threshold": 10}}`,
			healthCheckSoon, "3 errors"},
		"hot": {`{"smart_status": {"passed": true}, "temperature": {"current": 74}}`, healthCheckSoon, "74 °C"},
		"a few sectors moved": {`{"smart_status": {"passed": true}, "ata_smart_attributes": {"table": [{"id": 5, "raw": {"value": 8}}]}}`,
			healthCheckSoon, "moved 8"},
		"many sectors moved": {`{"smart_status": {"passed": true}, "ata_smart_attributes": {"table": [{"id": 5, "raw": {"value": 240}}]}}`,
			healthReplaceNow, "moved 240"},
		"sectors waiting": {`{"smart_status": {"passed": true}, "ata_smart_attributes": {"table": [{"id": 197, "raw": {"value": 2}}]}}`,
			healthCheckSoon, "2 sectors are waiting"},
		"sectors lost": {`{"smart_status": {"passed": true}, "ata_smart_attributes": {"table": [{"id": 198, "raw": {"value": 1}}]}}`,
			healthReplaceNow, "couldn't be read"},
		"not JSON": {`smartctl: command not found`, healthUnknown, "doesn't report"},
	}
	for name, c := range cases {
		var d Drive
		judge(&d, []byte(c.report))
		if d.Health != c.health || !strings.Contains(strings.Join(d.Reasons, " "), c.reason) {
			t.Errorf("%s: %s %v, want %s with %q", name, d.Health, d.Reasons, c.health, c.reason)
		}
	}
}
