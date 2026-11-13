// box-desktopd: Box Desktop's server (box-desktop.md §6). It serves the web
// desktop to browsers on the home network over HTTPS (TCP 7443) and to the
// box's own screen (HTTPS on 127.0.0.1:7444, the same names and
// certificates), signs admins in with ZOOMRIYA ID, and
// passes their requests on to zoomriya-agent. It runs as its own account,
// never root, and keeps every sign-in in memory only.
package main

import (
	"context"
	"crypto/tls"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// Config is where everything is; the box's paths by default, the
// environment's elsewhere (the preview, tests).
type Config struct {
	// TLSAddr is the home network's HTTPS address; ScreenAddr the box's
	// screen's, on 127.0.0.1 only. Either may be "" to leave it off.
	TLSAddr    string
	ScreenAddr string
	CertsDir   string
	AdminDir   string
	IDAPI      string
	Agent      string
	// Web is a folder of the built desktop to serve instead of the one built
	// into the program (development).
	Web string
	// Insecure serves both sides as plain HTTP (the preview only), and
	// accepts any name it's asked for.
	Insecure bool
	// Root is "/" on the box: the files Places reads.
	Root string
	// Fake: the preview's sample hardware, not the box's.
	Fake bool
	// SetupDone: the agent's mark that the box is set up; until it's there,
	// the box's screen shows the first setup. StateDir: where the setup keeps
	// its progress.
	SetupDone string
	StateDir  string
	// AppBase: where the apps' pages are, the preview's sample box's; on the
	// box, "" (https://<the box's name>/<app>/).
	AppBase string
}

func env(name, fallback string) string {
	if v, ok := os.LookupEnv(name); ok {
		return v
	}
	return fallback
}

func configFromEnv() Config {
	return Config{
		TLSAddr:    env("BOX_DESKTOP_LISTEN", ":7443"),
		ScreenAddr: env("BOX_DESKTOP_SCREEN", "127.0.0.1:7444"),
		CertsDir:   env("BOX_DESKTOP_CERTS", "/etc/zoomriya/tls/certs"),
		AdminDir:   env("BOX_DESKTOP_ADMIN_DIR", "/data/zoomriya/admin"),
		IDAPI:      env("BOX_DESKTOP_ID_API", "http://127.0.0.1:8050/id/api/v1"),
		Agent:      env("BOX_DESKTOP_AGENT", "/run/zoomriya/agent.sock"),
		Web:        os.Getenv("BOX_DESKTOP_WEB"),
		Insecure:   os.Getenv("BOX_DESKTOP_INSECURE") == "1",
		Root:       env("BOX_DESKTOP_ROOT", "/"),
		Fake:       os.Getenv("BOX_DESKTOP_FAKE") == "1",
		AppBase:    os.Getenv("BOX_DESKTOP_APP_BASE"),
		SetupDone:  env("BOX_DESKTOP_SETUP_DONE", "/var/lib/zoomriya-os/setup-done"),
		StateDir:   env("STATE_DIRECTORY", "/var/lib/box-desktop"),
	}
}

func main() {
	log.SetFlags(0)
	cfg := configFromEnv()
	srv, err := NewServer(cfg)
	if err != nil {
		log.Fatal(err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go srv.Sessions.Sweep(ctx)

	var servers []*http.Server
	errs := make(chan error, 2)
	if !cfg.Insecure {
		go srv.Certs.Watch(ctx)
	}
	serve := func(addr string, l Listener, loopbackOnly bool) {
		hs := &http.Server{
			Addr:              addr,
			Handler:           srv.Handler(l),
			ReadHeaderTimeout: 10 * time.Second,
			IdleTimeout:       2 * time.Minute,
		}
		servers = append(servers, hs)
		go func() {
			if loopbackOnly {
				host, _, err := net.SplitHostPort(addr)
				if ip := net.ParseIP(host); err != nil || ip == nil || !ip.IsLoopback() {
					errs <- errors.New("the box's screen's address must be on 127.0.0.1")
					return
				}
			}
			ln, err := net.Listen("tcp", addr)
			if err != nil {
				errs <- err
				return
			}
			if cfg.Insecure {
				log.Printf("box-desktopd: %s side on http://%s (preview: no TLS)", l, addr)
				errs <- hs.Serve(ln)
				return
			}
			hs.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS13, GetCertificate: srv.Certs.Get}
			log.Printf("box-desktopd: %s side on https://%s", l, addr)
			errs <- hs.ServeTLS(ln, "", "")
		}()
	}
	if cfg.TLSAddr != "" {
		serve(cfg.TLSAddr, ListenerHome, false)
	}
	if cfg.ScreenAddr != "" {
		serve(cfg.ScreenAddr, ListenerScreen, true)
	}

	select {
	case <-ctx.Done():
	case err := <-errs:
		if !errors.Is(err, http.ErrServerClosed) {
			log.Print(err)
			defer os.Exit(1)
		}
	}
	shut, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, hs := range servers {
		hs.Shutdown(shut)
	}
}
