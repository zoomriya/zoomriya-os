package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// The services' answers saved from the real ones (testdata), served here.
type fake struct {
	t      *testing.T
	srv    *httptest.Server
	hits   atomic.Int32
	status atomic.Int32
	feeds  atomic.Int32
	caps   atomic.Int32
	alerts string
}

func newFake(t *testing.T) *fake {
	f := &fake{t: t}
	f.status.Store(200)
	f.srv = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.hits.Add(1)
		if !strings.HasPrefix(r.Header.Get("User-Agent"), "ZOOMRIYA-OS/") {
			t.Errorf("no User-Agent naming us: %q", r.Header.Get("User-Agent"))
		}
		if s := int(f.status.Load()); s != 200 {
			w.WriteHeader(s)
			return
		}
		path := r.URL.Path
		switch {
		case strings.HasSuffix(path, "/citypageweather-realtime/items"):
			serve(t, w, "eccc-city.json")
		case strings.HasSuffix(path, "/weather-alerts/items"):
			fmt.Fprint(w, f.alerts)
		case strings.HasPrefix(path, "/points/"):
			fmt.Fprintf(w, `{"properties":{"forecast":"%s/gridpoints/OKX/33,42/forecast"}}`, f.srv.URL)
		case strings.HasSuffix(path, "/forecast"):
			w.Header().Set("Expires", time.Now().Add(10*time.Minute).UTC().Format(http.TimeFormat))
			serve(t, w, "nws-forecast.json")
		case strings.HasPrefix(path, "/alerts/active"):
			fmt.Fprint(w, `{"features":[]}`)
		case strings.HasPrefix(path, "/weatherapi/metalerts/"):
			fmt.Fprint(w, metAlertsAnswer)
		case strings.HasPrefix(path, "/feeds/meteoalarm-legacy-atom-"):
			f.feeds.Add(1)
			fmt.Fprint(w, strings.ReplaceAll(meteoAlarmFeed, "{base}", f.srv.URL))
		case strings.HasPrefix(path, "/api/v1/warnings/"):
			f.caps.Add(1)
			fmt.Fprint(w, capDocs[filepath.Base(path)])
		case strings.HasPrefix(path, "/weatherapi/"):
			serve(t, w, "metno.json")
		default:
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func serve(t *testing.T, w http.ResponseWriter, name string) {
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	w.Write(data)
}

func (f *fake) service(t *testing.T, root string) *Service {
	u, _ := url.Parse(f.srv.URL)
	fetcher := &Fetcher{Client: f.srv.Client(), Allowed: map[string]bool{u.Host: true},
		Agent: "ZOOMRIYA-OS/test (https://zoomriya.com)", Limit: 8 << 20}
	return &Service{
		Root: root,
		Providers: map[string]Provider{
			"eccc":  &ECCC{F: fetcher, Base: f.srv.URL},
			"nws":   &NWS{F: fetcher, Base: f.srv.URL},
			"metno": &MetNo{F: fetcher, Base: f.srv.URL},
		},
		Alerters: map[string]Alerter{
			"eccc":       &ECCC{F: fetcher, Base: f.srv.URL},
			"nws":        &NWS{F: fetcher, Base: f.srv.URL},
			"metalerts":  &MetAlerts{F: fetcher, Base: f.srv.URL},
			"meteoalarm": &MeteoAlarm{F: fetcher, Base: f.srv.URL},
		},
		Now:  func() time.Time { return time.Date(2026, 11, 16, 15, 0, 0, 0, time.UTC) },
		Logf: func(string, ...any) {},
	}
}

var (
	toronto = Town{ID: 6167865, Name: "Toronto", Admin1: "Ontario", Country: "CA", Lat: 43.70, Lon: -79.42, TimeZone: "America/Toronto"}
	newYork = Town{ID: 5128581, Name: "New York City", Admin1: "New York", Country: "US", Lat: 40.71, Lon: -74.01, TimeZone: "America/New_York"}
	london  = Town{ID: 2643743, Name: "London", Admin1: "England", Country: "GB", Lat: 51.51, Lon: -0.13, TimeZone: "Europe/London"}
	tokyo   = Town{ID: 1850147, Name: "Tokyo", Admin1: "Tokyo", Country: "JP", Lat: 35.69, Lon: 139.69, TimeZone: "Asia/Tokyo"}
	berlin  = Town{ID: 2950159, Name: "Berlin", Admin1: "Berlin", Country: "DE", Lat: 52.52, Lon: 13.41, TimeZone: "Europe/Berlin", Areas: []string{"DE202"}}
	oslo    = Town{ID: 3143244, Name: "Oslo", Admin1: "Oslo", Country: "NO", Lat: 59.91, Lon: 10.75, TimeZone: "Europe/Oslo"}
)

func chooseTown(t *testing.T, root string, town Town) {
	t.Helper()
	os.MkdirAll(filepath.Join(root, "etc/zoomriya"), 0o755)
	data, _ := json.Marshal(town)
	if err := os.WriteFile(filepath.Join(root, townFile), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func kept(t *testing.T, root string) Weather {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, weatherFile))
	if err != nil {
		t.Fatal(err)
	}
	var w Weather
	if err := json.Unmarshal(data, &w); err != nil {
		t.Fatal(err)
	}
	return w
}

func TestNoTownNothingFetchedNoFile(t *testing.T) {
	f := newFake(t)
	root := t.TempDir()
	f.service(t, root).Tick(context.Background())
	if f.hits.Load() != 0 {
		t.Fatal("fetched with no town")
	}
	if _, err := os.Stat(filepath.Join(root, weatherFile)); !os.IsNotExist(err) {
		t.Fatal("a weather file with no town")
	}
}

func TestCanadaBothLanguages(t *testing.T) {
	f := newFake(t)
	f.alerts = `{"features":[]}`
	root := t.TempDir()
	chooseTown(t, root, toronto)
	f.service(t, root).Tick(context.Background())
	w := kept(t, root)
	// The nearest city page: Toronto (on-143), not Toronto Island.
	if w.State != StateOK || w.Service != "eccc" || w.Units != "C" || !w.AlertsAvailable {
		t.Fatalf("%+v", w)
	}
	if w.Now == nil || *w.Now.Temp != 7.7 || w.Now.Condition.Fr != "Généralement nuageux" {
		t.Fatalf("now: %+v", w.Now)
	}
	d := w.Days[0]
	if d.Date != "2026-11-16" || *d.High != 11 || *d.Low != 2 || d.Name.Fr != "Aujourd'hui" || d.Icon != IconPartly {
		t.Fatalf("today: %+v", d)
	}
	if w.Days[1].Name.En != "Tuesday" || w.Days[1].Date != "2026-11-17" {
		t.Fatalf("tomorrow: %+v", w.Days[1])
	}
	if !strings.HasPrefix(w.Credit.En, "Contains information licenced under the Data Server End-use Licence") {
		t.Fatal(w.Credit.En)
	}
	info, _ := os.Stat(filepath.Join(root, weatherFile))
	if info.Mode().Perm() != 0o644 {
		t.Fatalf("mode %v", info.Mode())
	}
}

// An alert's area holds the town, or doesn't; an ended one is left out.
func TestAlertsByArea(t *testing.T) {
	f := newFake(t)
	square := func(id, status, ends string, lat, lon float64) string {
		return fmt.Sprintf(`{"id":"%s_fea1","geometry":{"type":"Polygon","coordinates":[[[%g,%g],[%g,%g],[%g,%g],[%g,%g],[%g,%g]]]},
			"properties":{"alert_type":"warning","alert_name_en":"snowfall warning","alert_name_fr":"avertissement de neige",
			"alert_text_en":"Snow.","alert_text_fr":"Neige.","feature_name_en":"City of Toronto","feature_name_fr":"ville de Toronto",
			"status_en":"%s","publication_datetime":"2026-11-16T10:00:00Z","validity_datetime":"2026-11-16T10:00:00Z",
			"event_end_datetime":"%s"}}`, id, lon-0.1, lat-0.1, lon+0.1, lat-0.1, lon+0.1, lat+0.1, lon-0.1, lat+0.1, lon-0.1, lat-0.1, status, ends)
	}
	f.alerts = `{"features":[` + strings.Join([]string{
		square("A1", "active", "2026-11-17T00:00:00Z", 43.70, -79.42),
		square("A1", "active", "2026-11-17T00:00:00Z", 43.70, -79.42), // the same alert, another area
		square("B2", "active", "2026-11-17T00:00:00Z", 45.0, -75.0),   // elsewhere
		square("C3", "ended", "2026-11-17T00:00:00Z", 43.70, -79.42),
		square("D4", "active", "2026-11-16T14:00:00Z", 43.70, -79.42), // past its end
	}, ",") + `]}`
	root := t.TempDir()
	chooseTown(t, root, toronto)
	s := f.service(t, root)
	s.Tick(context.Background())
	w := kept(t, root)
	if len(w.Alerts) != 1 || w.Alerts[0].ID != "A1" || w.Alerts[0].Kind != "warning" || w.Alerts[0].Event.Fr != "Avertissement de neige" {
		t.Fatalf("%+v", w.Alerts)
	}
	// Past its end: gone at once, with no new answer.
	s.Now = func() time.Time { return time.Date(2026, 11, 17, 0, 1, 0, 0, time.UTC) }
	s.nextAlerts = s.Now().Add(time.Hour)
	s.Tick(context.Background())
	if w := kept(t, root); len(w.Alerts) != 0 {
		t.Fatalf("ended alert kept: %+v", w.Alerts)
	}
}

func TestUSInFahrenheit(t *testing.T) {
	f := newFake(t)
	root := t.TempDir()
	chooseTown(t, root, newYork)
	f.service(t, root).Tick(context.Background())
	w := kept(t, root)
	if w.Service != "nws" || w.Units != "F" || *w.Days[0].High != 56 || *w.Days[0].Low != 42 || w.Days[0].Name.En != "Today" {
		t.Fatalf("%+v", w.Days[0])
	}
	if !strings.Contains(w.Official.En, "forecast.weather.gov") {
		t.Fatal(w.Official)
	}
}

func TestElsewhereForecastOnlyInCelsius(t *testing.T) {
	f := newFake(t)
	root := t.TempDir()
	chooseTown(t, root, tokyo)
	f.service(t, root).Tick(context.Background())
	w := kept(t, root)
	if w.Service != "metno" || w.Units != "C" || w.AlertsAvailable || w.AlertsService != "" || len(w.Days) < 5 || w.Now == nil {
		t.Fatalf("%+v", w)
	}
	if w.Credit.En != "Weather data from MET Norway, CC BY 4.0." {
		t.Fatal(w.Credit)
	}
}

func TestSwitchedOffFetchesNothing(t *testing.T) {
	f := newFake(t)
	root := t.TempDir()
	chooseTown(t, root, toronto)
	os.WriteFile(filepath.Join(root, internetFile), []byte(`{"off":["weather"]}`), 0o644)
	f.service(t, root).Tick(context.Background())
	if f.hits.Load() != 0 {
		t.Fatal("fetched while switched off")
	}
	if w := kept(t, root); w.State != StateOff {
		t.Fatal(w.State)
	}
	os.WriteFile(filepath.Join(root, internetFile), []byte(`{"apps_offline":true}`), 0o644)
	f.service(t, root).Tick(context.Background())
	if f.hits.Load() != 0 {
		t.Fatal("fetched with Apps offline")
	}
}

// A failing service: the last good forecast stays, marked stale, and the
// next try waits longer.
func TestFailureKeepsTheLastForecast(t *testing.T) {
	f := newFake(t)
	f.alerts = `{"features":[]}`
	root := t.TempDir()
	chooseTown(t, root, toronto)
	s := f.service(t, root)
	s.Tick(context.Background())
	f.status.Store(503)
	s.nextForecast = s.Now()
	s.Tick(context.Background())
	w := kept(t, root)
	if w.State != StateStale || len(w.Days) == 0 || w.Problem != "the weather service answered 503" {
		t.Fatalf("%+v", w)
	}
	if wait := s.nextForecast.Sub(s.Now()); wait < time.Minute {
		t.Fatal(wait)
	}
}

// After a restart, the kept forecast is taken back and not fetched again early.
func TestRestartKeepsTheForecast(t *testing.T) {
	f := newFake(t)
	f.alerts = `{"features":[]}`
	root := t.TempDir()
	chooseTown(t, root, tokyo)
	f.service(t, root).Tick(context.Background())
	hits := f.hits.Load()
	s := f.service(t, root)
	s.Restore()
	s.Tick(context.Background())
	if f.hits.Load() != hits {
		t.Fatal("fetched again right after a restart")
	}
}

func TestOnlyTheServicesHosts(t *testing.T) {
	f := &Fetcher{Client: http.DefaultClient, Allowed: map[string]bool{"api.met.no": true}, Limit: 1}
	if _, err := f.Get(context.Background(), "https://example.com/x", "", ""); err == nil {
		t.Fatal("asked a host that isn't declared")
	}
}

func TestPointInPolygonWithAHole(t *testing.T) {
	g := json.RawMessage(`{"type":"Polygon","coordinates":[[[0,0],[10,0],[10,10],[0,10],[0,0]],[[4,4],[6,4],[6,6],[4,6],[4,4]]]}`)
	if !holds(g, 2, 2) || holds(g, 5, 5) || holds(g, 20, 20) {
		t.Fatal("polygon")
	}
}

// MeteoAlarm's Atom feed for Germany: one warning for the town's area
// (DE202), one for another area (never read), and one with no code whose
// CAP's polygon holds the town (as the UK's, Sweden's and Switzerland's).
const meteoAlarmFeed = `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom" xmlns:cap="urn:oasis:names:tc:emergency:cap:1.2">
<entry><cap:geocode><valueName>EMMA_ID</valueName><value>DE202</value></cap:geocode>
 <cap:status>Actual</cap:status><cap:identifier>ours</cap:identifier>
 <link type="application/cap+xml" href="{base}/api/v1/warnings/feeds-germany/ours?index_info=0"/></entry>
<entry><cap:geocode><valueName>EMMA_ID</valueName><value>DE136</value></cap:geocode>
 <cap:status>Actual</cap:status><cap:identifier>elsewhere</cap:identifier>
 <link type="application/cap+xml" href="{base}/api/v1/warnings/feeds-germany/elsewhere"/></entry>
<entry><cap:status>Actual</cap:status><cap:identifier>shaped</cap:identifier>
 <link type="application/cap+xml" href="{base}/api/v1/warnings/feeds-germany/shaped"/></entry>
</feed>`

func capDoc(id, level, polygon, code string) string {
	area := "<areaDesc>Berlin</areaDesc>"
	if polygon != "" {
		area += "<polygon>" + polygon + "</polygon>"
	}
	if code != "" {
		area += "<geocode><valueName>EMMA_ID</valueName><value>" + code + "</value></geocode>"
	}
	info := func(lang, event string) string {
		return `<info><language>` + lang + `</language><event>` + event + `</event><headline>` + event +
			`</headline><description>Gusts.</description><onset>2026-11-16T12:00:00+00:00</onset>` +
			`<expires>2026-11-17T06:00:00+00:00</expires><parameter><valueName>awareness_level</valueName><value>` +
			level + `</value></parameter><area>` + area + `</area></info>`
	}
	return `<alert xmlns="urn:oasis:names:tc:emergency:cap:1.2"><identifier>` + id + `</identifier>` +
		`<sent>2026-11-16T11:00:00+00:00</sent><status>Actual</status><msgType>Alert</msgType>` +
		info("de-DE", "STURMBÖEN") + info("en-GB", "wind gusts") + info("fr-FR", "rafales de vent") + `</alert>`
}

var capDocs = map[string]string{
	"ours":      capDoc("ours", "3; orange; Severe", "", "DE202"),
	"elsewhere": capDoc("elsewhere", "2; yellow; Moderate", "", "DE136"),
	"shaped":    capDoc("shaped", "2; yellow; Moderate", "52.0,13.0 53.0,13.0 53.0,14.0 52.0,14.0 52.0,13.0", ""),
}

const metAlertsAnswer = `{"features":[
 {"properties":{"id":"no-1","eventAwarenessName":"wind","title":"Wind, orange level, Oslo","description":"Strong wind.",
  "consequences":"Trees may fall.","instruction":"Stay inside.","area":"Oslo","awareness_level":"3; orange; Severe",
  "geographicDomain":"land","status":"Actual","type":"Alert","eventEndingTime":"2026-11-17T02:00:00+00:00"},
  "when":{"interval":["2026-11-16T20:00:00+00:00","2026-11-17T02:00:00+00:00"]}},
 {"properties":{"id":"no-2","eventAwarenessName":"gale","area":"Oslofjord","awareness_level":"2; yellow; Moderate",
  "geographicDomain":"marine","status":"Actual","type":"Alert","eventEndingTime":"2026-11-17T02:00:00+00:00"}}]}`

// Europe: only the town's country's feed, only the town's warnings; a CAP
// is read once.
func TestMeteoAlarmOnlyTheTownsWarnings(t *testing.T) {
	f := newFake(t)
	root := t.TempDir()
	chooseTown(t, root, berlin)
	s := f.service(t, root)
	s.Tick(context.Background())
	w := kept(t, root)
	if w.AlertsService != "meteoalarm" || !w.AlertsAvailable || len(w.Alerts) != 2 {
		t.Fatalf("%+v", w.Alerts)
	}
	byID := map[string]Alert{}
	for _, a := range w.Alerts {
		byID[a.ID] = a
	}
	ours := byID["ours"]
	if ours.Kind != "warning" || ours.Level != "orange" || ours.Event.En != "Wind gusts" || ours.Event.Fr != "Rafales de vent" {
		t.Fatalf("%+v", ours)
	}
	if byID["shaped"].Kind != "advisory" || byID["elsewhere"].ID != "" {
		t.Fatalf("%+v", w.Alerts)
	}
	if !strings.Contains(w.Credit.En, "EUMETNET – MeteoAlarm (meteoalarm.org), CC BY 4.0.") ||
		w.Official.En != "https://meteoalarm.org?region=DE" {
		t.Fatalf("%+v %+v", w.Credit, w.Official)
	}
	// Another round: the feed again, never the CAP documents.
	caps := f.caps.Load()
	s.nextAlerts = s.Now()
	s.Tick(context.Background())
	if f.feeds.Load() != 2 || f.caps.Load() != caps {
		t.Fatalf("feeds %d, CAP reads %d then %d", f.feeds.Load(), caps, f.caps.Load())
	}
	if caps != 2 {
		t.Fatalf("read %d CAP documents; the other area's should never be read", caps)
	}
}

func TestNorwayFromMetAlertsNotAtSea(t *testing.T) {
	f := newFake(t)
	root := t.TempDir()
	chooseTown(t, root, oslo)
	f.service(t, root).Tick(context.Background())
	w := kept(t, root)
	if w.AlertsService != "metalerts" || len(w.Alerts) != 1 || w.Alerts[0].Kind != "warning" || w.Alerts[0].Event.En != "Wind (orange)" {
		t.Fatalf("%+v", w.Alerts)
	}
	if !strings.Contains(w.Credit.En, "Weather alerts from MET Norway, CC BY 4.0.") {
		t.Fatal(w.Credit)
	}
}
