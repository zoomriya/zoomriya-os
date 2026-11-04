// Restarts (design §6.1): when a security fix or a restore needs one, the box
// restarts by itself in the maintenance hour; an admin can restart now or move
// the hour. zoomriya-maintenance.timer runs `zoomriya-agent maintenance` at the
// hour, which restarts only when a restart is needed.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	// The timer's own unit has the default hour; an admin's choice is a
	// drop-in over it.
	defaultMaintenanceHour = "03:30"
	maintenanceDropIn      = "etc/systemd/system/zoomriya-maintenance.timer.d/hour.conf"
	maintenanceTimer       = "zoomriya-maintenance.timer"
	// The restart is a transient timer a few seconds out, so the answer
	// reaches Box Desktop first.
	restartDelay = "5s"
)

var hourPattern = regexp.MustCompile(`^([01][0-9]|2[0-3]):[0-5][0-9]$`)

// restartReasons says why a restart is needed: Debian lists the packages that
// need one, and a restore or a reset waits for one. Empty when none is needed.
func (a *Agent) restartReasons() []string {
	reasons := []string{}
	for _, pkg := range strings.Fields(a.readTrimmed("run/reboot-required.pkgs")) {
		reasons = append(reasons, "update: "+pkg)
	}
	switch level := a.readTrimmed(resetPending); {
	case level == "system":
		reasons = append(reasons, "reset the system")
	case level == "everything":
		reasons = append(reasons, "erase everything")
	default:
		if number := a.readTrimmed(restorePending); number != "" {
			reasons = append(reasons, "restore point "+number)
		}
	}
	if len(reasons) == 0 {
		if _, err := os.Stat(a.path("run/reboot-required")); err == nil {
			reasons = append(reasons, "update")
		}
	}
	return reasons
}

// maintenanceHour is the hour set by an admin, or the default.
func (a *Agent) maintenanceHour() string {
	for _, line := range strings.Split(a.readTrimmed(maintenanceDropIn), "\n") {
		if value, found := strings.CutPrefix(line, "OnCalendar=*-*-* "); found {
			if hour := strings.TrimSuffix(value, ":00"); hourPattern.MatchString(hour) {
				return hour
			}
		}
	}
	return defaultMaintenanceHour
}

// plannedRestart is when the next maintenance hour comes, in the box's time.
func (a *Agent) plannedRestart() time.Time {
	now := a.now()
	hour, _ := time.ParseInLocation("15:04", a.maintenanceHour(), now.Location())
	at := time.Date(now.Year(), now.Month(), now.Day(), hour.Hour(), hour.Minute(), 0, 0, now.Location())
	if !at.After(now) {
		at = at.AddDate(0, 0, 1)
	}
	return at
}

func (a *Agent) now() time.Time {
	if a.Now != nil {
		return a.Now()
	}
	return time.Now()
}

func (a *Agent) restart() (Response, error) {
	_, err := a.Run("systemd-run", "--quiet", "--unit=zoomriya-agent-restart",
		"--description=Restart asked for in Box Desktop", "--on-active="+restartDelay,
		"--timer-property=AccuracySec=1s", "/usr/bin/systemctl", "reboot")
	if err != nil {
		return nil, err
	}
	return Response{"restarting": true}, nil
}

// shutdown turns the box off, a few seconds from now so the answer gets back.
// Box Desktop offers it only on the box's own screen: nobody can turn the box
// on again from far away.
func (a *Agent) shutdown() (Response, error) {
	_, err := a.Run("systemd-run", "--quiet", "--unit=zoomriya-agent-shutdown",
		"--description=Shut down asked for in Box Desktop", "--on-active="+restartDelay,
		"--timer-property=AccuracySec=1s", "/usr/bin/systemctl", "poweroff")
	if err != nil {
		return nil, err
	}
	return Response{"shutting_down": true}, nil
}

func (a *Agent) setMaintenanceHour(hour *string) (Response, error) {
	if hour == nil {
		return Response{"maintenance_hour": a.maintenanceHour()}, nil
	}
	if !hourPattern.MatchString(*hour) {
		return nil, refuse("That isn't a time of day: %q. Use HH:MM, like 03:30.", *hour)
	}
	path := a.path(maintenanceDropIn)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	dropIn := fmt.Sprintf("# Written by zoomriya-agent: the hour an admin chose in Box Desktop.\n"+
		"[Timer]\nOnCalendar=\nOnCalendar=*-*-* %s:00\n", *hour)
	if err := os.WriteFile(path, []byte(dropIn), 0o644); err != nil {
		return nil, err
	}
	if _, err := a.Run("systemctl", "daemon-reload"); err != nil {
		return nil, err
	}
	if _, err := a.Run("systemctl", "restart", maintenanceTimer); err != nil {
		return nil, err
	}
	return Response{"maintenance_hour": *hour}, nil
}

// maintenance runs at the maintenance hour: restart if something waits for it.
func (a *Agent) maintenance() (restarted bool, err error) {
	if len(a.restartReasons()) == 0 {
		return false, nil
	}
	_, err = a.Run("systemctl", "reboot")
	return err == nil, err
}
