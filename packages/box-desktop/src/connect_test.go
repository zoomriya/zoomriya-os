package main

import (
	"encoding/json"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// A pretend connectd on a Unix socket: answers every request with what it saw.
func fakeConnect(t *testing.T, socket string) *[]string {
	t.Helper()
	l, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { l.Close() })
	var mu sync.Mutex
	seen := &[]string{}
	go http.Serve(l, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		*seen = append(*seen, r.Method+" "+r.URL.Path+" "+string(body))
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"on": true, "path": r.URL.Path})
	}))
	return seen
}

func TestConnectPassesOnToConnectd(t *testing.T) {
	b := newBox(t)
	socket := filepath.Join(t.TempDir(), "c.sock")
	b.srv.Cfg.Connect = socket
	seen := fakeConnect(t, socket)

	// Signed-in admins only.
	if w, _ := b.do("GET", "/api/connect/v1/status", nil); w.Code != http.StatusUnauthorized {
		t.Fatalf("signed out: %d", w.Code)
	}
	b.signIn()
	w, a := b.do("GET", "/api/connect/v1/status", nil)
	if w.Code != 200 || a["path"] != "/v1/status" {
		t.Fatalf("status: %d %v", w.Code, a)
	}
	w, _ = b.do("POST", "/api/connect/v1/pairing", map[string]any{"name": "Anna's laptop", "laptop": true})
	if w.Code != 200 || !strings.Contains((*seen)[len(*seen)-1], `"laptop":true`) {
		t.Fatalf("pairing: %d %v", w.Code, *seen)
	}
	w, _ = b.do("DELETE", "/api/connect/v1/devices/a1b2c3", map[string]string{"password": "right"})
	if w.Code != 200 {
		t.Fatalf("remove: %d", w.Code)
	}

	// Only the tool's routes pass.
	for _, path := range []string{"/api/connect/v1/anything", "/api/connect/v1/devices/../status", "/api/connect/v1/devices/zz"} {
		if w, _ := b.do("DELETE", path, nil); w.Code == 200 {
			t.Fatalf("%s passed", path)
		}
	}
	// ZOOMRIYA Connect's old routes are gone.
	for _, path := range []string{"/api/connect/v1/setup", "/api/connect/v1/server", "/api/connect/v1/off"} {
		if w, _ := b.do("POST", path, map[string]string{"password": "right"}); w.Code != http.StatusNotFound {
			t.Fatalf("%s: %d", path, w.Code)
		}
	}

	// Changes go in the admin actions; reading doesn't.
	var ops []string
	for _, line := range b.lines {
		if line["ZOOMRIYA_AUDIT"] == "1" {
			ops = append(ops, line["MESSAGE"])
		}
	}
	if len(ops) != 2 || !strings.Contains(ops[0], "Showed a pairing code for a laptop") || !strings.Contains(ops[1], "Removed a paired device") {
		t.Fatalf("admin actions: %v", ops)
	}
}

func TestRemovingADeviceAsksThePassword(t *testing.T) {
	b := newBox(t)
	socket := filepath.Join(t.TempDir(), "c.sock")
	b.srv.Cfg.Connect = socket
	seen := fakeConnect(t, socket)
	b.signIn()
	if w, a := b.do("DELETE", "/api/connect/v1/devices/a1b2c3", map[string]string{}); w.Code != http.StatusForbidden || a["code"] != "password_needed" {
		t.Fatalf("no password: %d %v", w.Code, a)
	}
	if w, a := b.do("DELETE", "/api/connect/v1/devices/a1b2c3", map[string]string{"password": "wrong"}); w.Code != http.StatusForbidden || a["code"] != "not_your_password" {
		t.Fatalf("wrong password: %d %v", w.Code, a)
	}
	if len(*seen) != 0 {
		t.Fatalf("connectd was asked: %v", *seen)
	}
	w, _ := b.do("DELETE", "/api/connect/v1/devices/a1b2c3", map[string]string{"password": "right"})
	if w.Code != 200 {
		t.Fatalf("right password: %d", w.Code)
	}
	if last := (*seen)[len(*seen)-1]; strings.Contains(last, "right") {
		t.Fatalf("the password went on to connectd: %s", last)
	}
}

func TestConnectNotRunning(t *testing.T) {
	b := newBox(t)
	b.srv.Cfg.Connect = filepath.Join(t.TempDir(), "missing.sock")
	b.signIn()
	w, a := b.do("GET", "/api/connect/v1/status", nil)
	if w.Code != http.StatusServiceUnavailable || a["code"] != "connect_unavailable" {
		t.Fatalf("%d %v", w.Code, a)
	}
}
