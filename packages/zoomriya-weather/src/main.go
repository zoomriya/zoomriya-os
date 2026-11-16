// zoomriya-weatherd: the box's weather (docs/weather.md). It reads the
// family's town, fetches the forecast and the alerts from the town's
// country's weather service (ECCC in Canada, the NWS in the US, MET Norway
// elsewhere, forecast only), checks them, and keeps them in
// /var/lib/zoomriya-weather/weather.json for Box Desktop and every app to
// read.
//
// It needs nothing from the apps: it reads no app's data, joins none of
// their groups, never calls one and sends no notification; it only writes
// its own file. Until a town is chosen it fetches nothing.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// version: set at build (-X main.version=…); the User-Agent names it.
var version = "dev"

func env(name, fallback string) string {
	if v, ok := os.LookupEnv(name); ok && v != "" {
		return v
	}
	return fallback
}

func newService() *Service {
	f := &Fetcher{
		Client: &http.Client{Timeout: 30 * time.Second},
		// The declared connection's hosts (outbound.d/weather.json).
		Allowed: map[string]bool{"api.weather.gc.ca": true, "api.weather.gov": true, "api.met.no": true,
			"feeds.meteoalarm.org": true},
		Agent: "ZOOMRIYA-OS/" + version + " (https://zoomriya.com)",
		Limit: 8 << 20,
	}
	eccc := &ECCC{F: f, Base: "https://api.weather.gc.ca"}
	nws := &NWS{F: f, Base: "https://api.weather.gov"}
	return &Service{
		Root:      env("ZOOMRIYA_WEATHER_ROOT", "/"),
		Providers: map[string]Provider{"eccc": eccc, "nws": nws, "metno": &MetNo{F: f, Base: "https://api.met.no"}},
		Alerters: map[string]Alerter{
			"eccc": eccc, "nws": nws,
			"metalerts":  &MetAlerts{F: f, Base: "https://api.met.no"},
			"meteoalarm": &MeteoAlarm{F: f, Base: "https://feeds.meteoalarm.org"},
		},
		Now: time.Now,
	}
}

func main() {
	log.SetFlags(0)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, os.Interrupt)
	defer stop()
	s := newService()
	s.Restore()
	for {
		wait := s.Tick(ctx)
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
	}
}
