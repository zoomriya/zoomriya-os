// The audit log: one journal entry per request, with who asked (design §7).
// Written with the journal's native protocol, so Log Center can filter on the
// fields (SYSLOG_IDENTIFIER=zoomriya-agent, ZOOMRIYA_OP, ZOOMRIYA_BY).
package main

import (
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
)

const journalSocket = "/run/systemd/journal/socket"

type AuditEntry struct {
	// ReadOnly entries are kept at debug level, changes at notice.
	ReadOnly bool
	Op       string
	By       string
	Number   string
	// What was changed, when the request says more than its name.
	Detail  string
	PeerUID int
	Result  string
}

type Auditor interface {
	Record(AuditEntry)
}

type Journal struct{}

func NewJournal() Journal { return Journal{} }

// clean keeps a field on one line: the journal's simple format can't hold a
// newline, and a caller mustn't be able to forge another field.
func clean(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' {
			return ' '
		}
		return r
	}, s)
}

func priority(readOnly bool) string {
	if readOnly {
		return "7"
	}
	return "5"
}

func (Journal) Record(e AuditEntry) {
	who := e.By
	if who == "" {
		who = "uid " + strconv.Itoa(e.PeerUID)
	}
	what := e.Op
	if e.Number != "" {
		what += " " + e.Number
	}
	if e.Detail != "" {
		what += " (" + e.Detail + ")"
	}
	message := fmt.Sprintf("%s asked for %s: %s", who, what, e.Result)

	fields := []string{
		"MESSAGE=" + clean(message),
		"PRIORITY=" + priority(e.ReadOnly),
		"SYSLOG_IDENTIFIER=zoomriya-agent",
		"ZOOMRIYA_AUDIT=1",
		"ZOOMRIYA_OP=" + clean(e.Op),
		"ZOOMRIYA_BY=" + clean(e.By),
		"ZOOMRIYA_NUMBER=" + clean(e.Number),
		"ZOOMRIYA_DETAIL=" + clean(e.Detail),
		"ZOOMRIYA_PEER_UID=" + strconv.Itoa(e.PeerUID),
		"ZOOMRIYA_RESULT=" + clean(e.Result),
	}
	writeJournal(fields, message)
}

// writeJournal sends one entry with its fields; without the journal's socket,
// the message alone still reaches it, through stderr.
func writeJournal(fields []string, message string) {
	conn, err := net.Dial("unixgram", journalSocket)
	if err != nil {
		log.Print(message)
		return
	}
	defer conn.Close()
	if _, err := conn.Write([]byte(strings.Join(fields, "\n") + "\n")); err != nil {
		log.Print(message)
	}
}
