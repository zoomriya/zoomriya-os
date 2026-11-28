// Package localhttp: the box's services, reached over Unix sockets.
//
// On the box no service has a TCP port of its own. Each one listens on a
// socket file, which only the accounts meant to open it can (a port on
// 127.0.0.1 is open to every program on the machine). A service's address
// names its socket and the path under it:
//
//	unix:id.sock:/id/api/v1                  in the sockets folder
//	unix:/run/zoomriya/id.sock:/id/api/v1    a socket anywhere
//
// A bare name is in the sockets folder: $ZOOMRIYA_SOCKETS, or /run/zoomriya
// when that is not set (the dev scripts set a folder of their own). A path is
// added to an address as to a URL. http:// and https:// addresses still work,
// for tests. The same format as zoomriya-id's Python helper
// (zoomriya_identity.local_http).
package localhttp

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// SocketsEnv names the folder bare socket names are in; DefaultSockets is it
// when the variable is not set.
const (
	SocketsEnv     = "ZOOMRIYA_SOCKETS"
	DefaultSockets = "/run/zoomriya"
)

// SocketsDir is the folder a bare socket name is in.
func SocketsDir() string {
	if dir := os.Getenv(SocketsEnv); dir != "" {
		return dir
	}
	return DefaultSockets
}

// SocketPath is name itself when it is a path, else name in SocketsDir.
func SocketPath(name string) string {
	if strings.HasPrefix(name, "/") {
		return name
	}
	return filepath.Join(SocketsDir(), name)
}

// Split gives the socket file and the HTTP path of a unix: address
// (ok false for any other address). With no path, the path is /.
func Split(address string) (socket, path string, ok bool, err error) {
	rest, found := strings.CutPrefix(address, "unix:")
	if !found {
		return "", "", false, nil
	}
	name, path, _ := strings.Cut(rest, ":")
	if name == "" {
		return "", "", true, fmt.Errorf("no socket in %q", address)
	}
	if path == "" {
		path = "/"
	}
	if !strings.HasPrefix(path, "/") {
		return "", "", true, fmt.Errorf("the path in %q must start with /", address)
	}
	return SocketPath(name), path, true, nil
}

// Client gives the base URL to add paths to, and a client that reaches it:
// over the socket for a unix: address (no proxy, ever), plainly otherwise.
// A timeout of 0 means none (answers that take as long as they take).
func Client(address string, timeout time.Duration) (string, *http.Client, error) {
	socket, path, ok, err := Split(address)
	if err != nil {
		return "", nil, err
	}
	if !ok {
		return strings.TrimRight(address, "/"), &http.Client{Timeout: timeout}, nil
	}
	return "http://localhost" + strings.TrimRight(path, "/"), &http.Client{
		Timeout:   timeout,
		Transport: Transport(socket),
	}, nil
}

// Transport sends every request to the one socket, whatever the URL's host.
func Transport(socket string) *http.Transport {
	var dialer net.Dialer
	return &http.Transport{
		Proxy: nil,
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return dialer.DialContext(ctx, "unix", socket)
		},
		MaxIdleConns:    4,
		IdleConnTimeout: 90 * time.Second,
	}
}

// Listen takes the one socket systemd passes (socket activation, fd 3), or,
// without one, makes the socket at path itself (development), first removing
// one a killed run left behind. A socket it makes is mode 0660 at most.
func Listen(path string) (net.Listener, error) {
	if os.Getenv("LISTEN_PID") == strconv.Itoa(os.Getpid()) && os.Getenv("LISTEN_FDS") == "1" {
		file := os.NewFile(3, "systemd socket")
		listener, err := net.FileListener(file)
		file.Close()
		return listener, err
	}
	if path == "" {
		return nil, errors.New("no socket from systemd and no socket path")
	}
	if info, err := os.Lstat(path); err == nil && info.Mode()&os.ModeSocket != 0 {
		_ = os.Remove(path)
	}
	listener, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(path, 0o660); err != nil {
		listener.Close()
		return nil, err
	}
	return listener, nil
}
