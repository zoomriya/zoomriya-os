// zoomriya-agent: the one root helper ZOOMRIYA OS adds (design §7).
//
// Socket-activated on /run/zoomriya/agent.sock, mode 0660, group
// zoomriya-box. Its clients are the box's own screen: the first setup and
// Box Desktop, which signs an admin in with ZOOMRIYA ID before it asks; the
// agent checks that the process connecting is root or in zoomriya-box.
//
// Protocol: one JSON object per line in, one per line out
// ({"op": "status", "by": "<member>"} -> {"ok": true, ...} or
// {"ok": false, "error": "..."}), one request per connection. Requests are
// answered one at a time, so two restores can never run together. Every
// request goes into the audit log with who asked.
//
// `zoomriya-agent ask '<request>'` sends one request and prints the answer,
// for the VM tests and for support over SSH. `zoomriya-agent maintenance` is
// the maintenance hour's job (restarts.go); `zoomriya-agent network-undo
// <port>` puts back network settings that weren't kept (netconfig.go);
// `zoomriya-agent watch` writes the notifications (watch.go).
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"os/user"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	socketPath = "/run/zoomriya/agent.sock"
	group      = "zoomriya-box"
	// A request line is small; anything longer is refused.
	maxRequest  = 64 << 10
	readTimeout = 10 * time.Second
)

func main() {
	if len(os.Args) == 3 && os.Args[1] == "ask" {
		os.Exit(ask(os.Args[2]))
	}
	if os.Geteuid() != 0 {
		log.Fatal("zoomriya-agent must run as root")
	}
	if len(os.Args) == 2 && os.Args[1] == "maintenance" {
		os.Exit(maintenance())
	}
	if len(os.Args) == 2 && os.Args[1] == "watch" {
		if err := NewAgent().watch(NewJournal()); err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}
	if len(os.Args) == 3 && os.Args[1] == "network-undo" {
		os.Exit(networkUndo(os.Args[2]))
	}
	listener, err := listen()
	if err != nil {
		log.Fatal(err)
	}
	agent := NewAgent()
	audit := NewJournal()
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Print(err)
			continue
		}
		serve(agent, audit, conn.(*net.UnixConn), allowedPeer)
	}
}

// ask sends one request and prints the answer; it fails when the answer isn't ok.
func ask(request string) int {
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	defer conn.Close()
	if _, err := conn.Write([]byte(request + "\n")); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	line, err := bufio.NewReader(conn).ReadBytes('\n')
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	os.Stdout.Write(line)
	var resp Response
	if json.Unmarshal(line, &resp) != nil || resp["ok"] != true {
		return 1
	}
	return 0
}

// maintenance is zoomriya-maintenance.service's job at the maintenance hour.
func maintenance() int {
	agent := NewAgent()
	reasons := agent.restartReasons()
	restarted, err := agent.maintenance()
	audit := NewJournal()
	switch {
	case err != nil:
		audit.Record(AuditEntry{Op: "maintenance", By: "maintenance hour", Result: err.Error()})
		return 1
	case restarted:
		audit.Record(AuditEntry{Op: "maintenance", By: "maintenance hour", Result: "restarting for " + strings.Join(reasons, ", ")})
	}
	return 0
}

// networkUndo is the undo timer's job, two minutes after a change nobody kept.
func networkUndo(port string) int {
	audit := NewJournal()
	undone, err := NewAgent().networkUndo(port)
	if err != nil {
		audit.Record(AuditEntry{Op: "network-undo", By: "undo timer", Detail: port, Result: err.Error()})
		return 1
	}
	if undone {
		audit.Notify("network", "warning", "New network settings for "+port+" weren't kept, so the box went back to the ones from before.")
		audit.Record(AuditEntry{Op: "network-undo", By: "undo timer", Detail: port, Result: "put back the settings from before, as nobody kept the new ones"})
	}
	return 0
}

// listen takes the socket systemd passes (zoomriya-agent.socket), or opens one
// when started by hand.
func listen() (net.Listener, error) {
	if os.Getenv("LISTEN_PID") == strconv.Itoa(os.Getpid()) && os.Getenv("LISTEN_FDS") == "1" {
		return net.FileListener(os.NewFile(3, "zoomriya-agent.socket"))
	}
	_ = os.Remove(socketPath)
	return net.Listen("unix", socketPath)
}

// allowedPeer: root, or a member of zoomriya-box.
func allowedPeer(cred *syscall.Ucred) bool {
	if cred.Uid == 0 {
		return true
	}
	g, err := user.LookupGroup(group)
	if err != nil {
		return false
	}
	if strconv.Itoa(int(cred.Gid)) == g.Gid {
		return true
	}
	u, err := user.LookupId(strconv.Itoa(int(cred.Uid)))
	if err != nil {
		return false
	}
	ids, err := u.GroupIds()
	if err != nil {
		return false
	}
	for _, id := range ids {
		if id == g.Gid {
			return true
		}
	}
	return false
}

func peerCred(conn *net.UnixConn) (*syscall.Ucred, error) {
	raw, err := conn.SyscallConn()
	if err != nil {
		return nil, err
	}
	var cred *syscall.Ucred
	var credErr error
	err = raw.Control(func(fd uintptr) {
		cred, credErr = syscall.GetsockoptUcred(int(fd), syscall.SOL_SOCKET, syscall.SO_PEERCRED)
	})
	if err != nil {
		return nil, err
	}
	return cred, credErr
}

// serve answers one connection's one request.
func serve(agent *Agent, audit Auditor, conn *net.UnixConn, allowed func(*syscall.Ucred) bool) {
	defer conn.Close()
	reply := func(resp Response) {
		line, _ := json.Marshal(resp)
		_, _ = conn.Write(append(line, '\n'))
	}

	cred, err := peerCred(conn)
	if err != nil || !allowed(cred) {
		uid := -1
		if cred != nil {
			uid = int(cred.Uid)
		}
		audit.Record(AuditEntry{PeerUID: uid, Result: "refused: not allowed to ask"})
		reply(Response{"ok": false, "error": "Not allowed."})
		return
	}

	_ = conn.SetReadDeadline(time.Now().Add(readTimeout))
	reader := bufio.NewReaderSize(conn, maxRequest)
	line, err := reader.ReadSlice('\n')
	if err != nil {
		audit.Record(AuditEntry{PeerUID: int(cred.Uid), Result: "refused: no complete request"})
		reply(Response{"ok": false, "error": "No complete request."})
		return
	}

	var req Request
	decoder := json.NewDecoder(bytes.NewReader(line))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		audit.Record(AuditEntry{PeerUID: int(cred.Uid), Result: "refused: not a request"})
		reply(Response{"ok": false, "error": "Not a request this box understands."})
		return
	}

	resp := answer(agent.Handle(req))
	result := "done"
	if resp["ok"] != true {
		result = fmt.Sprint(resp["error"])
	}
	readOnlyRequest := readOnly[req.Op] || (req.Op == "maintenance-hour" && req.Hour == nil)
	entry := AuditEntry{ReadOnly: readOnlyRequest, Op: req.Op, By: req.By, PeerUID: int(cred.Uid), Result: result}
	if req.Number != nil {
		entry.Number = strconv.Itoa(*req.Number)
	}
	entry.Detail = detail(req)
	audit.Record(entry)
	reply(resp)
}

// detail is what a change was about, for the audit log.
func detail(req Request) string {
	str := func(p *string) string {
		if p == nil {
			return ""
		}
		return *p
	}
	switch req.Op {
	case "network-set":
		parts := []string{str(req.Link), str(req.Mode)}
		if str(req.Mode) == "fixed" {
			parts = append(parts, str(req.Address), "router "+str(req.Gateway), "DNS "+strings.Join(req.DNS, " "))
		}
		return strings.Join(parts, " ")
	case "maintenance-hour", "factory-reset", "storage-check", "keyboard-set":
		return str(req.Hour) + str(req.Level) + str(req.Action) + str(req.Layout)
	case "time-zone-set":
		return str(req.Zone)
	case "display-set":
		if str(req.Action) == "forget" {
			return str(req.Output) + " forget"
		}
		refresh, scale := 0, 0.0
		if req.Refresh != nil {
			refresh = *req.Refresh
		}
		if req.Scale != nil {
			scale = *req.Scale
		}
		return fmt.Sprintf("%s %s@%d scale %g %s", str(req.Output), str(req.Size), refresh, scale,
			str(req.Transform))
	case "region-set":
		return str(req.Region)
	case "language-set":
		return str(req.Locale)
	case "first-admin":
		// The username only: the password is never written down.
		return str(req.Username)
	}
	return ""
}
