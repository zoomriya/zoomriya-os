// disk-health (design §7): every drive's SMART, as one word an admin can act on
// and the reasons for it. The words: healthy, check-soon, replace-now, and
// unknown for a drive that doesn't report its health.
package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
)

// Drive is one physical drive, as Box Desktop shows it.
type Drive struct {
	Name         string   `json:"name"`
	Path         string   `json:"path"`
	Model        string   `json:"model"`
	Serial       string   `json:"serial"`
	SizeBytes    int64    `json:"size_bytes"`
	Connection   string   `json:"connection"`
	System       bool     `json:"system"`
	Health       string   `json:"health"`
	Reasons      []string `json:"reasons"`
	TemperatureC *int     `json:"temperature_c"`
	PowerOnHours *int     `json:"power_on_hours"`
	WearPercent  *int     `json:"wear_percent"`
}

// The thresholds for each word. NVMe drives report wear and spare blocks; SATA
// drives report sectors they moved or couldn't read.
const (
	wearCheckSoon      = 80
	hotCheckSoon       = 70
	reallocatedReplace = 100
	healthHealthy      = "healthy"
	healthCheckSoon    = "check-soon"
	healthReplaceNow   = "replace-now"
	healthUnknown      = "unknown"
)

type lsblkList struct {
	Blockdevices []struct {
		Name   string `json:"name"`
		Path   string `json:"path"`
		Type   string `json:"type"`
		Size   int64  `json:"size"`
		Model  string `json:"model"`
		Serial string `json:"serial"`
		Tran   string `json:"tran"`
	} `json:"blockdevices"`
}

// smartReport is the part of `smartctl --json --all` the agent reads.
type smartReport struct {
	SmartStatus *struct {
		Passed bool `json:"passed"`
	} `json:"smart_status"`
	Temperature *struct {
		Current int `json:"current"`
	} `json:"temperature"`
	PowerOnTime *struct {
		Hours int `json:"hours"`
	} `json:"power_on_time"`
	NVMe *struct {
		CriticalWarning         int `json:"critical_warning"`
		AvailableSpare          int `json:"available_spare"`
		AvailableSpareThreshold int `json:"available_spare_threshold"`
		PercentageUsed          int `json:"percentage_used"`
		MediaErrors             int `json:"media_errors"`
	} `json:"nvme_smart_health_information_log"`
	ATA *struct {
		Table []struct {
			ID  int `json:"id"`
			Raw struct {
				Value int64 `json:"value"`
			} `json:"raw"`
		} `json:"table"`
	} `json:"ata_smart_attributes"`
}

func (a *Agent) diskHealth() (Response, error) {
	out, err := a.Run("lsblk", "--json", "--bytes", "--nodeps",
		"--output", "NAME,PATH,TYPE,SIZE,MODEL,SERIAL,TRAN")
	if err != nil {
		return nil, err
	}
	var list lsblkList
	if err := json.Unmarshal(out, &list); err != nil {
		return nil, fmt.Errorf("lsblk's list: %w", err)
	}
	system := a.systemDisk()

	drives := []Drive{}
	for _, d := range list.Blockdevices {
		if d.Type != "disk" || strings.HasPrefix(d.Name, "zram") {
			continue
		}
		drive := Drive{
			Name:       d.Name,
			Path:       d.Path,
			Model:      strings.TrimSpace(d.Model),
			Serial:     strings.TrimSpace(d.Serial),
			SizeBytes:  d.Size,
			Connection: d.Tran,
			System:     d.Name == system,
		}
		// smartctl's exit status is a bit mask that is often not zero for a
		// healthy drive, so only its report counts.
		report, _ := a.Run("smartctl", "--json", "--all", d.Path)
		judge(&drive, report)
		drives = append(drives, drive)
	}
	return Response{"drives": drives}, nil
}

// systemDisk is the drive the system partition is on.
func (a *Agent) systemDisk() string {
	out, err := a.Run("lsblk", "--noheadings", "--nodeps", "--output", "PKNAME", "/dev/disk/by-partlabel/system")
	if err != nil {
		return ""
	}
	return filepath.Base(strings.TrimSpace(string(out)))
}

// judge gives a drive its word and reasons from smartctl's report.
func judge(drive *Drive, raw []byte) {
	var r smartReport
	if json.Unmarshal(raw, &r) != nil || (r.SmartStatus == nil && r.NVMe == nil && r.ATA == nil) {
		drive.Health = healthUnknown
		drive.Reasons = []string{"This drive doesn't report its health."}
		return
	}

	replace, soon := []string{}, []string{}
	if r.SmartStatus != nil && !r.SmartStatus.Passed {
		replace = append(replace, "The drive's own check failed.")
	}
	if r.Temperature != nil {
		t := r.Temperature.Current
		drive.TemperatureC = &t
		if t >= hotCheckSoon {
			soon = append(soon, fmt.Sprintf("It's running hot: %d °C.", t))
		}
	}
	if r.PowerOnTime != nil {
		h := r.PowerOnTime.Hours
		drive.PowerOnHours = &h
	}
	if n := r.NVMe; n != nil {
		used := n.PercentageUsed
		drive.WearPercent = &used
		switch {
		case n.CriticalWarning != 0:
			replace = append(replace, "The drive reports a critical warning.")
		case used >= 100:
			replace = append(replace, "It has reached the end of its rated life.")
		case used >= wearCheckSoon:
			soon = append(soon, fmt.Sprintf("It's %d%% through its rated life.", used))
		}
		if n.AvailableSpare < n.AvailableSpareThreshold {
			replace = append(replace, "It's running out of spare blocks.")
		}
		if n.MediaErrors > 0 {
			soon = append(soon, fmt.Sprintf("It had %d errors reading or writing.", n.MediaErrors))
		}
	}
	if r.ATA != nil {
		attr := map[int]int64{}
		for _, a := range r.ATA.Table {
			attr[a.ID] = a.Raw.Value
		}
		// 5: reallocated sectors; 197: waiting to be reallocated;
		// 198: couldn't be read at all.
		switch moved := attr[5]; {
		case moved >= reallocatedReplace:
			replace = append(replace, fmt.Sprintf("It has moved %d worn-out sectors.", moved))
		case moved > 0:
			soon = append(soon, fmt.Sprintf("It has moved %d worn-out sectors.", moved))
		}
		if lost := attr[198]; lost > 0 {
			replace = append(replace, fmt.Sprintf("%d sectors couldn't be read.", lost))
		}
		if pending := attr[197]; pending > 0 {
			soon = append(soon, fmt.Sprintf("%d sectors are waiting to be checked.", pending))
		}
	}

	switch {
	case len(replace) > 0:
		drive.Health, drive.Reasons = healthReplaceNow, append(replace, soon...)
	case len(soon) > 0:
		drive.Health, drive.Reasons = healthCheckSoon, soon
	default:
		drive.Health, drive.Reasons = healthHealthy, []string{}
	}
}
