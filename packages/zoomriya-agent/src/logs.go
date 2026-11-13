// logs (design §7, box-desktop.md §4 Log Center): the journal, newest first, a
// page at a time: the box's own events, the admins' actions (the audit log),
// or the whole system log with a level and a search.
package main

import (
	"bytes"
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	logPage = 100
	// A message longer than this is cut; the journal keeps it whole.
	logMessageMax = 2000
	logSearchMax  = 100
)

// The box's own events: the OS's units, security updates, and each start.
// Exact matches, as journalctl's unit globs fail when one matches nothing.
var eventMatches = []string{
	"_SYSTEMD_UNIT=zoomriya-agent.service",
	"_SYSTEMD_UNIT=zoomriya-maintenance.service",
	"_SYSTEMD_UNIT=zoomriya-boot-check.service",
	"_SYSTEMD_UNIT=zoomriya-aide-check.service",
	"_SYSTEMD_UNIT=zoomriya-restore-cleanup.service",
	"_SYSTEMD_UNIT=zoomriya-factory-snapshot.service",
	"_SYSTEMD_UNIT=zoomriya-dpkg-recover.service",
	"_SYSTEMD_UNIT=zoomriya-erase.service",
	"_SYSTEMD_UNIT=" + checkUnit + ".service",
	"_SYSTEMD_UNIT=box-desktop.service",
	"_SYSTEMD_UNIT=box-desktop-screen.service",
	"_SYSTEMD_UNIT=unattended-upgrades.service",
	"_SYSTEMD_UNIT=apt-daily-upgrade.service",
	// The audit log, also from network-undo's own runs.
	"SYSLOG_IDENTIFIER=zoomriya-agent",
	// systemd's "Startup finished": the box started.
	"MESSAGE_ID=b07a249cd024414a82dd00cd181378ff",
}

// The system log's levels: what each shows, at least this important.
var logLevels = map[string]string{"errors": "err", "warnings": "warning", "everything": "info"}

var levelNames = []string{"emergency", "alert", "critical", "error", "warning", "notice", "info", "debug"}

// A journal cursor: "s=...;i=...;b=...;m=...;t=...;x=...".
var cursorPattern = regexp.MustCompile(`^[a-z]=[0-9a-f]+(;[a-z]=[0-9a-f]+)*$`)

type LogEntry struct {
	Time    string `json:"time"`
	Level   string `json:"level"`
	Source  string `json:"source"`
	Unit    string `json:"unit,omitempty"`
	Message string `json:"message"`
	// The audit log's fields.
	By     string `json:"by,omitempty"`
	Op     string `json:"op,omitempty"`
	Detail string `json:"detail,omitempty"`
	Result string `json:"result,omitempty"`
	Cursor string `json:"cursor"`
}

func (a *Agent) logs(kind, cursor, search, priority *string) (Response, error) {
	args := []string{"--output=json", "--no-pager", "--quiet", "--reverse", "--lines=" + strconv.Itoa(logPage)}
	if cursor != nil && *cursor != "" {
		if len(*cursor) > 256 || !cursorPattern.MatchString(*cursor) {
			return nil, refuse("That page of the log isn't one the box knows.")
		}
		// Reversed, "after" the cursor is older than it.
		args = append(args, "--after-cursor="+*cursor)
	}
	switch {
	case kind == nil:
		return nil, refuse("Which log? Box events, admin actions or the system log?")
	case *kind == "events":
		args = append(args, "--priority=info")
		for i, match := range eventMatches {
			if i > 0 {
				args = append(args, "+")
			}
			args = append(args, match)
		}
	case *kind == "audit":
		// Changes only: reads are kept at debug level.
		args = append(args, "--priority=notice", "ZOOMRIYA_AUDIT=1")
	case *kind == "signins":
		// Box Desktop's own: an admin signed in on the box's screen, or was refused.
		args = append(args, "--priority=info", "SYSLOG_IDENTIFIER=box-desktop", "ZOOMRIYA_SIGNIN=1")
	case *kind == "system":
		shown := "info"
		if priority != nil && *priority != "" {
			p, ok := logLevels[*priority]
			if !ok {
				return nil, refuse("Show errors, warnings or everything?")
			}
			shown = p
		}
		args = append(args, "--priority="+shown)
	default:
		return nil, refuse("There's no log called %q.", *kind)
	}
	text := ""
	if search != nil {
		text = strings.TrimSpace(*search)
	}
	if text != "" {
		if len(text) > logSearchMax || strings.ContainsAny(text, "\n\r") {
			return nil, refuse("Search for something shorter.")
		}
		// Words, not a pattern.
		args = append(args, "--grep="+regexp.QuoteMeta(text), "--case-sensitive=false")
	}

	out, err := a.Run("journalctl", args...)
	// A search that finds nothing fails, with nothing printed.
	if err != nil && !(text != "" && len(bytes.TrimSpace(out)) == 0) {
		return nil, err
	}
	entries := []LogEntry{}
	for _, line := range bytes.Split(out, []byte("\n")) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var fields map[string]json.RawMessage
		if json.Unmarshal(line, &fields) != nil {
			continue
		}
		entries = append(entries, logEntry(fields))
	}
	more := len(entries) == logPage
	next := ""
	if more {
		next = entries[len(entries)-1].Cursor
	}
	return Response{"entries": entries, "more": more, "next": next}, nil
}

func logEntry(fields map[string]json.RawMessage) LogEntry {
	f := func(key string) string { return journalField(fields[key]) }
	e := LogEntry{
		Source:  f("SYSLOG_IDENTIFIER"),
		Unit:    f("_SYSTEMD_UNIT"),
		Message: f("MESSAGE"),
		Cursor:  f("__CURSOR"),
		By:      f("ZOOMRIYA_BY"),
		Op:      f("ZOOMRIYA_OP"),
		Detail:  f("ZOOMRIYA_DETAIL"),
		Result:  f("ZOOMRIYA_RESULT"),
	}
	if e.Source == "" {
		e.Source = f("_COMM")
	}
	if us, err := strconv.ParseInt(f("__REALTIME_TIMESTAMP"), 10, 64); err == nil {
		e.Time = time.UnixMicro(us).UTC().Format(time.RFC3339Nano)
	}
	if p, err := strconv.Atoi(f("PRIORITY")); err == nil && p >= 0 && p < len(levelNames) {
		e.Level = levelNames[p]
	} else {
		e.Level = "info"
	}
	if len(e.Message) > logMessageMax {
		e.Message = e.Message[:logMessageMax] + "…"
	}
	return e
}

// journalField reads one field of journalctl's JSON: a string, an array of
// bytes (not valid UTF-8), an array of either (the field repeated; the first
// is shown), or null (too long to show).
func journalField(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var b []byte
	var numbers []int
	if json.Unmarshal(raw, &numbers) == nil {
		for _, n := range numbers {
			b = append(b, byte(n))
		}
		return strings.ToValidUTF8(string(b), "�")
	}
	var many []json.RawMessage
	if json.Unmarshal(raw, &many) == nil && len(many) > 0 {
		return journalField(many[0])
	}
	return ""
}
