// The box's backup for Box Desktop (Storage Manager › Backups): zoomriya-apps'
// backup service (platform/backup: restic, zoomriya-backup.service, and its
// nightly timer, off until an admin turns it on), as systemd runs it. The
// agent reads its state and starts it; it never reads restic's password or
// the backups themselves. Setting it up stays with Family Hub's settings.
package main

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	backupService = "zoomriya-backup.service"
	backupTimer   = "zoomriya-backup.timer"
	// Its settings: where the backups go (RESTIC_REPOSITORY) and the password's file.
	backupEnv = "etc/zoomriya/backup.env"
	// How many of the last run's problems are shown.
	backupProblems = 10
)

// The backup's own lines for a problem: "[backup <time>] ERROR: <what>".
var backupProblem = regexp.MustCompile(`^\[backup [^\]]*\] ERROR: (.+)$`)

// backupsRequest: no action is the backup's state; "now" starts one; "nightly"
// turns the nightly backup on or off (On).
func (a *Agent) backupsRequest(req Request) (Response, error) {
	switch deref(req.Action) {
	case "":
		return a.backups()
	case "now":
		if !a.backupsSetUp() {
			return nil, refuse("Backups aren't set up on this box yet.")
		}
		if _, err := a.Run("systemctl", "start", "--no-block", backupService); err != nil {
			return nil, refuse("The backup couldn't be started.")
		}
		return a.backups()
	case "nightly":
		if req.On == nil {
			return nil, refuse("On or off?")
		}
		if !a.backupsSetUp() {
			return nil, refuse("Backups aren't set up on this box yet.")
		}
		verb := "disable"
		if *req.On {
			verb = "enable"
		}
		if _, err := a.Run("systemctl", verb, "--now", backupTimer); err != nil {
			return nil, refuse("The nightly backup couldn't be changed.")
		}
		return a.backups()
	}
	return nil, refuse("Not something backups can do.")
}

func (a *Agent) backupsSetUp() bool {
	if a.readTrimmed(backupEnv) == "" {
		return false
	}
	return a.unitShow(backupService, "LoadState")["LoadState"] == "loaded"
}

func (a *Agent) backups() (Response, error) {
	if !a.backupsSetUp() {
		return Response{"set_up": false}, nil
	}
	service := a.unitShow(backupService, "ActiveState", "Result", "ExecMainStartTimestamp",
		"ExecMainExitTimestamp", "ExecMainStatus", "InvocationID")
	running := service["ActiveState"] == "activating" || service["ActiveState"] == "active"
	enabled, _ := a.Run("systemctl", "is-enabled", backupTimer)
	nightly := strings.TrimSpace(string(enabled)) == "enabled"
	resp := Response{
		"set_up":  true,
		"running": running,
		"nightly": nightly,
		"where":   backupPlace(envValue(a.readTrimmed(backupEnv), "RESTIC_REPOSITORY")),
	}
	if started := unixTime(service["ExecMainStartTimestamp"]); started != "" {
		resp["last_start"] = started
		if !running {
			resp["last_end"] = unixTime(service["ExecMainExitTimestamp"])
			ok := service["Result"] == "success" && service["ExecMainStatus"] == "0"
			resp["last_ok"] = ok
			if !ok {
				resp["problems"] = a.backupProblems(service["InvocationID"])
			}
		}
	}
	if nightly {
		if next := unixTime(a.unitShow(backupTimer, "NextElapseUSecRealtime")["NextElapseUSecRealtime"]); next != "" {
			resp["next"] = next
		}
	}
	return resp, nil
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
	seconds, err := strconv.ParseInt(strings.TrimPrefix(value, "@"), 10, 64)
	if err != nil || seconds <= 0 || !strings.HasPrefix(value, "@") {
		return ""
	}
	return time.Unix(seconds, 0).UTC().Format(time.RFC3339)
}

// The last run's problems, in the backup's own words: each app that failed and why.
func (a *Agent) backupProblems(invocation string) []string {
	problems := []string{}
	if !regexp.MustCompile(`^[0-9a-f]{32}$`).MatchString(invocation) {
		return problems
	}
	out, _ := a.Run("journalctl", "_SYSTEMD_INVOCATION_ID="+invocation, "-o", "cat", "--no-pager")
	for _, line := range strings.Split(string(out), "\n") {
		if m := backupProblem.FindStringSubmatch(strings.TrimSpace(line)); m != nil && len(problems) < backupProblems {
			problems = append(problems, m[1])
		}
	}
	return problems
}

// envValue: KEY=value from an environment file, unquoted.
func envValue(env, key string) string {
	for _, line := range strings.Split(env, "\n") {
		if k, v, ok := strings.Cut(strings.TrimSpace(line), "="); ok && k == key {
			return strings.Trim(strings.TrimSpace(v), `"'`)
		}
	}
	return ""
}

// backupPlace: where the backups go, said without any password in it: a
// folder ("/data/zoomriya/volumes/backup/zoomriya-backup"), or a kind and a host
// ("sftp: nas.home").
func backupPlace(repository string) string {
	if repository == "" || strings.HasPrefix(repository, "/") {
		return repository
	}
	kind, rest, ok := strings.Cut(repository, ":")
	if !ok {
		return ""
	}
	rest = strings.TrimPrefix(strings.TrimPrefix(rest, "https://"), "http://")
	rest = strings.TrimPrefix(rest, "//")
	if at := strings.LastIndex(rest, "@"); at >= 0 {
		rest = rest[at+1:]
	}
	host := strings.FieldsFunc(rest, func(r rune) bool { return r == '/' || r == ':' })
	if len(host) == 0 {
		return kind
	}
	return kind + ": " + host[0]
}
