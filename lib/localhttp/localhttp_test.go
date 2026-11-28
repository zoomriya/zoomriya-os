package localhttp

import (
	"io"
	"net/http"
	"path/filepath"
	"testing"
	"time"
)

func TestSplit(t *testing.T) {
	t.Setenv(SocketsEnv, "")
	cases := []struct {
		address, socket, path string
		ok                    bool
	}{
		{"unix:id.sock:/id/api/v1/auth/me", "/run/zoomriya/id.sock", "/id/api/v1/auth/me", true},
		{"unix:/elsewhere/x.sock:/a?b=1", "/elsewhere/x.sock", "/a?b=1", true},
		{"unix:id.sock", "/run/zoomriya/id.sock", "/", true},
		{"http://127.0.0.1:8050/id", "", "", false},
	}
	for _, c := range cases {
		socket, path, ok, err := Split(c.address)
		if err != nil || ok != c.ok || socket != c.socket || path != c.path {
			t.Errorf("Split(%q) = %q %q %v %v", c.address, socket, path, ok, err)
		}
	}
	for _, bad := range []string{"unix:", "unix::/id", "unix:id.sock:id"} {
		if _, _, _, err := Split(bad); err == nil {
			t.Errorf("Split(%q) gave no error", bad)
		}
	}
	t.Setenv(SocketsEnv, "/tmp/dev")
	if socket, _, _, _ := Split("unix:id.sock:/"); socket != "/tmp/dev/id.sock" {
		t.Errorf("the sockets folder from the environment: %q", socket)
	}
}

func TestClientOverASocket(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(SocketsEnv, dir)
	t.Setenv("http_proxy", "http://127.0.0.1:9")
	listener, err := Listen(filepath.Join(dir, "app.sock"))
	if err != nil {
		t.Fatal(err)
	}
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, r.Method+" "+r.URL.RequestURI())
	})}
	go server.Serve(listener)
	defer server.Close()

	base, client, err := Client("unix:app.sock:/myira/api/v1/", 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if base != "http://localhost/myira/api/v1" {
		t.Fatalf("base %q", base)
	}
	response, err := client.Get(base + "/model/current?x=1")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(response.Body)
	if string(body) != "GET /myira/api/v1/model/current?x=1" {
		t.Fatalf("answer %q", body)
	}

	// A socket left by a killed run is replaced.
	listener.Close()
	again, err := Listen(filepath.Join(dir, "app.sock"))
	if err != nil {
		t.Fatalf("a second Listen: %v", err)
	}
	again.Close()
}

func TestClientPlainURL(t *testing.T) {
	base, client, err := Client("http://127.0.0.1:8070/myira/api/v1/", time.Second)
	if err != nil || base != "http://127.0.0.1:8070/myira/api/v1" || client.Timeout != time.Second {
		t.Fatalf("%q %v %v", base, client, err)
	}
}
