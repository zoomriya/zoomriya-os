// Box Desktop's own lines in the box's journal, as the native desktop's
// journal.py writes them: admins' sign-ins, for Log Center's Sign-ins, and
// People's changes, for Admin actions. The journal's own fields
// (SYSLOG_IDENTIFIER=box-desktop, ZOOMRIYA_SIGNIN=1 or ZOOMRIYA_AUDIT=1,
// ZOOMRIYA_BY), over its socket; away from the box, nowhere. Never a password.
package main

import (
	"net"
	"strings"
)

const journalSocket = "/run/systemd/journal/socket"

// Journal writes the lines; Lines keeps them instead, in tests.
type Journal struct {
	Socket string
	Lines  *[]map[string]string
}

func clean(text string) string {
	text = strings.Join(strings.Fields(text), " ")
	if len(text) > 200 {
		text = text[:200]
	}
	return text
}

// where: the box's screen, or the browser's address on the home network.
func where(l Listener, remote string) string {
	if l == ListenerScreen {
		return "on the box's screen"
	}
	if host, _, err := net.SplitHostPort(remote); err == nil {
		remote = host
	}
	return "from " + clean(remote)
}

func (j Journal) SignIn(username string, l Listener, remote string, signedIn bool, why string) {
	who := clean(username)
	if who == "" {
		who = "someone"
	}
	message := who + " signed in " + where(l, remote)
	priority := "6"
	if !signedIn {
		message = "A sign-in as " + who + " " + where(l, remote) + " was refused: " + clean(why)
		priority = "5"
	}
	j.send(map[string]string{
		"MESSAGE":           message,
		"PRIORITY":          priority,
		"SYSLOG_IDENTIFIER": "box-desktop",
		"ZOOMRIYA_SIGNIN":   "1",
		"ZOOMRIYA_BY":       who,
	})
}

// Change: a change People made in ZOOMRIYA ID, in the admin's name.
func (j Journal) Change(by, op, message string) {
	who := clean(by)
	if who == "" {
		who = "someone"
	}
	j.send(map[string]string{
		"MESSAGE":           clean(message),
		"PRIORITY":          "5",
		"SYSLOG_IDENTIFIER": "box-desktop",
		"ZOOMRIYA_AUDIT":    "1",
		"ZOOMRIYA_OP":       clean(op),
		"ZOOMRIYA_BY":       who,
		"ZOOMRIYA_RESULT":   "ok",
	})
}

func (j Journal) send(fields map[string]string) {
	if j.Lines != nil {
		*j.Lines = append(*j.Lines, fields)
		return
	}
	var b strings.Builder
	for _, key := range []string{"MESSAGE", "PRIORITY", "SYSLOG_IDENTIFIER", "ZOOMRIYA_SIGNIN", "ZOOMRIYA_AUDIT",
		"ZOOMRIYA_OP", "ZOOMRIYA_BY", "ZOOMRIYA_RESULT"} {
		if v, ok := fields[key]; ok {
			b.WriteString(key + "=" + v + "\n")
		}
	}
	socket := j.Socket
	if socket == "" {
		socket = journalSocket
	}
	conn, err := net.Dial("unixgram", socket)
	if err != nil {
		return
	}
	defer conn.Close()
	conn.Write([]byte(b.String()))
}
