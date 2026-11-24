package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// The box's files this service reads and writes, under Root ("/" on the box).
const (
	// townFile: the town chosen in the first setup or Control Panel (the
	// agent's town-set). The image has none: until there is one, nothing is
	// fetched and no weather.json is written (weather.md §1).
	townFile = "etc/zoomriya/town"
	// internetFile: Security › Internet Access's switches (the agent's).
	internetFile = "etc/zoomriya/internet.json"
	// weatherFile: what every app and Box Desktop reads, 0644.
	weatherFile = "var/lib/zoomriya-weather/weather.json"
	// connectionID: the declared connection's id (outbound.d/weather.json).
	connectionID = "weather"
)

// How often (weather.md §5).
const (
	forecastEvery = time.Hour
	alertsEvery   = 10 * time.Minute
	longestWait   = time.Hour
)

// Provider is one weather service.
type Provider interface {
	Forecast(ctx context.Context, t Town, lastModified string) (Forecast, error)
}

// Alerter is a weather service that also has alerts (ECCC, NWS).
type Alerter interface {
	Alerts(ctx context.Context, t Town, now time.Time) ([]Alert, error)
}

type serviceInfo struct {
	Name    string
	Credit  Words
	Licence string
}

var services = map[string]serviceInfo{
	"eccc":  {"eccc", ecccWords, "ECCC Data Servers End-use Licence 2.1"},
	"nws":   {"nws", nwsCredit, "Public domain (US government work)"},
	"metno": {"metno", metCredit, "CC BY 4.0"},
}

// serviceFor: the town's country's own service; MET Norway elsewhere.
func serviceFor(country string) string {
	switch country {
	case "CA":
		return "eccc"
	case "US":
		return "nws"
	}
	return "metno"
}

// AlertSource is a service of alerts only, with its own credit and its own
// page of official warnings (MetAlerts, MeteoAlarm).
type AlertSource interface {
	Alerter
	Credit() Words
	Official(t Town) (Words, Words)
}

// alertsFor: where the town's country's alerts come from (weather.md §2):
// the forecast's own service in Canada and the US, MET Norway's MetAlerts in
// Norway, MeteoAlarm in the rest of Europe; elsewhere, none yet (W-4).
func alertsFor(country string) string {
	switch {
	case country == "CA":
		return "eccc"
	case country == "US":
		return "nws"
	case country == "NO":
		return "metalerts"
	case meteoAlarmFeeds[country] != "":
		return "meteoalarm"
	}
	return ""
}

// Service keeps the town's weather.
type Service struct {
	Root      string
	Providers map[string]Provider
	// Alerters: every alert service, by name, the forecast's own included.
	Alerters map[string]Alerter
	Now      func() time.Time
	Logf     func(format string, args ...any)

	w            Weather
	lastModified string
	nextForecast time.Time
	nextAlerts   time.Time
	failures     int
	alertFails   int
	written      []byte
}

func (s *Service) path(rel string) string { return filepath.Join(s.Root, rel) }

func (s *Service) logf(format string, args ...any) {
	if s.Logf != nil {
		s.Logf(format, args...)
	} else {
		log.Printf(format, args...)
	}
}

// readTown: the chosen town; ok false when none is chosen (or the file
// doesn't read: then it's as if none were).
func (s *Service) readTown() (Town, bool) {
	data, err := os.ReadFile(s.path(townFile))
	if err != nil {
		return Town{}, false
	}
	var t Town
	if json.Unmarshal(data, &t) != nil || t.ID == 0 || len(t.Country) != 2 ||
		t.Lat < -90 || t.Lat > 90 || t.Lon < -180 || t.Lon > 180 {
		return Town{}, false
	}
	return t, true
}

// switchedOff: weather's own switch is off, or Apps offline or Everything
// offline is on (Security › Internet Access).
func (s *Service) switchedOff() bool {
	data, err := os.ReadFile(s.path(internetFile))
	if err != nil {
		return false
	}
	var c struct {
		AppsOffline bool     `json:"apps_offline"`
		AllOffline  bool     `json:"all_offline"`
		Off         []string `json:"off"`
	}
	if json.Unmarshal(data, &c) != nil {
		return false
	}
	if c.AppsOffline || c.AllOffline {
		return true
	}
	for _, id := range c.Off {
		if id == connectionID {
			return true
		}
	}
	return false
}

// Restore takes the last file back at start, so a restart neither forgets
// the forecast nor asks again before it's due.
func (s *Service) Restore() {
	data, err := os.ReadFile(s.path(weatherFile))
	if err != nil {
		return
	}
	var w Weather
	if json.Unmarshal(data, &w) != nil || w.Version != fileVersion {
		return
	}
	s.w, s.written = w, data
	if fetched, err := time.Parse(time.RFC3339, w.Fetched); err == nil {
		next := fetched.Add(forecastEvery)
		if expires, err := time.Parse(time.RFC3339, w.Expires); err == nil && expires.After(next) {
			next = expires
		}
		s.nextForecast = next
	}
}

// Tick does what is due now, and says when to look again.
func (s *Service) Tick(ctx context.Context) time.Duration {
	now := s.Now()
	town, ok := s.readTown()
	if !ok {
		// Not activated: no fetching, no file.
		if err := os.Remove(s.path(weatherFile)); err == nil {
			s.logf("no town chosen: weather removed")
		}
		s.w, s.written = Weather{}, nil
		return time.Minute
	}
	name := serviceFor(town.Country)
	alertsName := alertsFor(town.Country)
	alerter := s.Alerters[alertsName]
	if s.w.Town.ID != town.ID || s.w.Service != name {
		info := services[name]
		units := "C"
		if town.Country == "US" {
			units = "F"
		}
		credit := info.Credit
		if extra, ok := alerter.(AlertSource); ok {
			credit = Words{credit.En + " " + extra.Credit().En, credit.Fr + " " + extra.Credit().Fr}
		}
		s.w = Weather{Version: fileVersion, State: StateWaiting, Town: town, Service: name,
			Credit: credit, Licence: info.Licence, Units: units, AlertsAvailable: alerter != nil,
			Days: []Day{}, Alerts: []Alert{}}
		if alerter != nil {
			s.w.AlertsService = alertsName
		}
		s.lastModified, s.failures, s.alertFails = "", 0, 0
		s.nextForecast, s.nextAlerts = now, now
	}
	s.w.Town = town // a new name for the same town

	if s.switchedOff() {
		s.w.State = StateOff
		s.dropEnded(now)
		s.write()
		return time.Minute
	}
	if s.w.State == StateOff {
		// Switched on again: fetch now.
		s.w.State = StateStale
		if s.w.Fetched == "" {
			s.w.State = StateWaiting
		}
		s.nextForecast, s.nextAlerts = now, now
	}
	if !now.Before(s.nextForecast) {
		s.fetchForecast(ctx, now)
	}
	if alerter != nil && !now.Before(s.nextAlerts) {
		s.fetchAlerts(ctx, alerter, town, now)
	}
	s.dropEnded(now)
	s.write()
	wait := min(s.nextForecast.Sub(now), longestWait)
	if s.w.AlertsAvailable {
		wait = min(wait, s.nextAlerts.Sub(now))
	}
	// Each minute at most, to see a new town or a switch soon.
	return max(min(wait, time.Minute), time.Second)
}

func (s *Service) fetchForecast(ctx context.Context, now time.Time) {
	started := time.Now()
	p := s.Providers[s.w.Service]
	f, err := p.Forecast(ctx, s.w.Town, s.lastModified)
	switch {
	case errors.Is(err, errNotModified):
		s.failures = 0
		s.w.State, s.w.Problem = StateOK, ""
		s.nextForecast = later(now.Add(forecastEvery), f.Expires)
		s.w.Expires = s.nextForecast.UTC().Format(time.RFC3339)
		s.logf("forecast unchanged (%s)", time.Since(started).Round(time.Millisecond))
		return
	case err != nil:
		s.failures++
		s.nextForecast = now.Add(backoff(s.failures))
		s.w.Problem = err.Error()
		if s.w.Fetched != "" {
			s.w.State = StateStale
		}
		s.logf("forecast failed: %v", err)
		return
	}
	if f.Unit != s.w.Units {
		convert(&f, s.w.Units)
	}
	s.failures = 0
	s.lastModified = f.LastModified
	s.nextForecast = later(now.Add(forecastEvery), f.Expires)
	s.w.State, s.w.Problem = StateOK, ""
	s.w.Now, s.w.Days = f.Now, f.Days
	s.w.Official, s.w.OfficialName = f.Official, f.OfficialName
	if source, ok := s.Alerters[s.w.AlertsService].(AlertSource); ok {
		s.w.Official, s.w.OfficialName = source.Official(s.w.Town)
	}
	s.w.Fetched = now.UTC().Format(time.RFC3339)
	s.w.Expires = s.nextForecast.UTC().Format(time.RFC3339)
	s.logf("forecast fetched (%s)", time.Since(started).Round(time.Millisecond))
}

func (s *Service) fetchAlerts(ctx context.Context, a Alerter, t Town, now time.Time) {
	alerts, err := a.Alerts(ctx, t, now)
	if err != nil {
		s.alertFails++
		s.nextAlerts = now.Add(min(backoff(s.alertFails), alertsEvery))
		s.logf("alerts failed: %v", err)
		return
	}
	s.alertFails = 0
	s.w.Alerts = alerts
	s.w.AlertsFetched = now.UTC().Format(time.RFC3339)
	s.nextAlerts = now.Add(alertsEvery)
}

// dropEnded: an alert past its end goes at once, even with no new answer.
func (s *Service) dropEnded(now time.Time) {
	kept := s.w.Alerts[:0]
	for _, a := range s.w.Alerts {
		if end, err := time.Parse(time.RFC3339, a.Ends); err == nil && !end.After(now) {
			continue
		}
		kept = append(kept, a)
	}
	s.w.Alerts = kept
}

// write: the whole file, written then moved into place, only when it changed.
func (s *Service) write() {
	data, err := json.MarshalIndent(s.w, "", " ")
	if err != nil {
		return
	}
	data = append(data, '\n')
	if bytes.Equal(data, s.written) {
		return
	}
	file := s.path(weatherFile)
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		s.logf("can't keep the weather: %v", err)
		return
	}
	tmp, err := os.CreateTemp(filepath.Dir(file), ".weather-*")
	if err != nil {
		s.logf("can't keep the weather: %v", err)
		return
	}
	_, werr := tmp.Write(data)
	cerr := tmp.Close()
	if werr != nil || cerr != nil || os.Chmod(tmp.Name(), 0o644) != nil || os.Rename(tmp.Name(), file) != nil {
		os.Remove(tmp.Name())
		s.logf("can't keep the weather")
		return
	}
	s.written = data
}

// backoff: a longer wait after each failure, with some randomness, an hour
// at most.
func backoff(failures int) time.Duration {
	wait := time.Minute << min(failures-1, 6)
	wait = min(wait, longestWait)
	return wait + time.Duration(rand.Int64N(int64(wait/4)+1))
}

func later(a, b time.Time) time.Time {
	if b.After(a) {
		return b
	}
	return a
}

// convert: the forecast's temperatures into the box's units.
func convert(f *Forecast, units string) {
	to := toC
	if units == "F" {
		to = toF
	}
	conv := func(v *float64) *float64 {
		if v == nil {
			return nil
		}
		return ptr(round1(to(*v)))
	}
	if f.Now != nil {
		f.Now.Temp = conv(f.Now.Temp)
	}
	for i := range f.Days {
		f.Days[i].High, f.Days[i].Low = conv(f.Days[i].High), conv(f.Days[i].Low)
	}
	f.Unit = strings.ToUpper(units)
}
