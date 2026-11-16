package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"sort"
	"strings"
	"time"
)

// ECCC: Environment and Climate Change Canada's MSC GeoMet (api.weather.gc.ca),
// for a box in Canada: the nearest city page's current conditions and
// forecast, and the weather alerts whose area holds the town. Every word in
// English and French, as ECCC writes them. ECCC's Data Servers End-use
// Licence 2.1.
type ECCC struct {
	F    *Fetcher
	Base string // https://api.weather.gc.ca
}

const ecccCredit = "Contains information licenced under the Data Server End-use Licence of Environment and Climate Change Canada."

var ecccWords = Words{ecccCredit,
	"Contient de l'information visée par la Licence d'utilisation finale des serveurs de données d'Environnement et Changement climatique Canada."}

type ecccWord struct {
	En any `json:"en"`
	Fr any `json:"fr"`
}

func (w ecccWord) words() Words { return Words{text(w.En), text(w.Fr)} }

func (w ecccWord) number() *float64 {
	switch v := w.En.(type) {
	case float64:
		return &v
	case string:
		var f float64
		if _, err := fmt.Sscanf(v, "%g", &f); err == nil {
			return &f
		}
	}
	return nil
}

func text(v any) string {
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case float64:
		return fmt.Sprintf("%g", t)
	}
	return ""
}

type ecccValue struct {
	Value ecccWord `json:"value"`
}

type ecccCity struct {
	Geometry struct {
		Coordinates []float64 `json:"coordinates"`
	} `json:"geometry"`
	Properties struct {
		Identifier string   `json:"identifier"`
		Name       ecccWord `json:"name"`
		URL        ecccWord `json:"url"`
		Current    *struct {
			Timestamp   ecccWord  `json:"timestamp"`
			Temperature ecccValue `json:"temperature"`
			Condition   ecccWord  `json:"condition"`
			Station     ecccValue `json:"station"`
		} `json:"currentConditions"`
		ForecastGroup struct {
			Timestamp ecccWord `json:"timestamp"`
			Forecasts []struct {
				Period struct {
					Name  ecccWord `json:"textForecastName"`
					Value ecccWord `json:"value"`
				} `json:"period"`
				Temperatures struct {
					Temperature []struct {
						Class ecccWord `json:"class"`
						Value ecccWord `json:"value"`
					} `json:"temperature"`
				} `json:"temperatures"`
				Abbreviated struct {
					TextSummary ecccWord `json:"textSummary"`
					Pop         *struct {
						Value ecccWord `json:"value"`
					} `json:"pop"`
				} `json:"abbreviatedForecast"`
				TextSummary ecccWord `json:"textSummary"`
			} `json:"forecasts"`
		} `json:"forecastGroup"`
	} `json:"properties"`
}

type geoCollection[T any] struct {
	Features []T `json:"features"`
}

func box(lat, lon, d float64) string {
	return fmt.Sprintf("%.2f,%.2f,%.2f,%.2f", lon-d, lat-d, lon+d, lat+d)
}

// city: the city page nearest the town, looked for close by first.
func (e *ECCC) city(ctx context.Context, t Town) (ecccCity, error) {
	for _, d := range []float64{0.5, 2} {
		var found geoCollection[ecccCity]
		u := e.Base + "/collections/citypageweather-realtime/items?f=json&limit=50&bbox=" + url.QueryEscape(box(t.Lat, t.Lon, d))
		if _, err := e.F.getJSON(ctx, u, "application/geo+json", &found); err != nil {
			return ecccCity{}, err
		}
		best, bestKM := -1, math.Inf(1)
		for i, c := range found.Features {
			if len(c.Geometry.Coordinates) < 2 {
				continue
			}
			if km := distanceKM(t.Lat, t.Lon, c.Geometry.Coordinates[1], c.Geometry.Coordinates[0]); km < bestKM {
				best, bestKM = i, km
			}
		}
		if best >= 0 {
			return found.Features[best], nil
		}
	}
	return ecccCity{}, fmt.Errorf("Environment Canada has no forecast near the town")
}

// Forecast is what a service gives for the town.
type Forecast struct {
	Now          *Now
	Days         []Day
	Expires      time.Time
	Official     Words
	OfficialName Words
	// Unit of the temperatures given: "C" or "F".
	Unit string
	// LastModified: MET Norway's, sent back next time.
	LastModified string
}

func (e *ECCC) Forecast(ctx context.Context, t Town, _ string) (Forecast, error) {
	c, err := e.city(ctx, t)
	if err != nil {
		return Forecast{}, err
	}
	p := c.Properties
	f := Forecast{Unit: "C", Official: p.URL.words(),
		OfficialName: Words{"Environment Canada's alerts", "les alertes d'Environnement Canada"}}
	if p.Current != nil && p.Current.Temperature.Value.number() != nil {
		cond := p.Current.Condition.words()
		f.Now = &Now{Time: p.Current.Timestamp.words().En, Temp: p.Current.Temperature.Value.number(),
			Condition: cond, Icon: iconFor(cond.En), Station: p.Current.Station.Value.words()}
	}
	issued, err := time.Parse(time.RFC3339, p.ForecastGroup.Timestamp.words().En)
	if err != nil {
		return Forecast{}, fmt.Errorf("Environment Canada's forecast has no time")
	}
	zone := townZone(t)
	first := issued.In(zone)
	byDate := map[string]*Day{}
	var order []string
	for _, period := range p.ForecastGroup.Forecasts {
		value := period.Period.Value.words().En // "Monday", "Monday night"
		weekday := strings.TrimSuffix(value, " night")
		night := weekday != value || strings.EqualFold(value, "tonight")
		date := nextWeekday(first, weekday)
		if date == "" {
			continue
		}
		d := byDate[date]
		if d == nil {
			d = &Day{Date: date}
			byDate[date] = d
			order = append(order, date)
		}
		for _, temp := range period.Temperatures.Temperature {
			switch temp.Class.words().En {
			case "high":
				d.High = temp.Value.number()
			case "low":
				d.Low = temp.Value.number()
			}
		}
		summary := period.Abbreviated.TextSummary.words()
		if night {
			d.Night = period.TextSummary.words()
			if d.Condition.En == "" {
				d.Condition, d.Icon, d.Name = summary, iconFor(summary.En), period.Period.Name.words()
			}
		} else {
			d.Text = period.TextSummary.words()
			d.Condition, d.Icon, d.Name = summary, iconFor(summary.En), period.Period.Name.words()
		}
		if period.Abbreviated.Pop != nil && d.Chance == nil {
			if v := period.Abbreviated.Pop.Value.number(); v != nil {
				d.Chance = ptr(int(*v))
			}
		}
	}
	sort.Strings(order)
	for _, date := range order {
		f.Days = append(f.Days, *byDate[date])
	}
	if len(f.Days) == 0 {
		return Forecast{}, fmt.Errorf("Environment Canada's forecast had no days")
	}
	return f, nil
}

// nextWeekday: the first date from `from` (inclusive, within a week) that is
// that weekday, in English.
func nextWeekday(from time.Time, weekday string) string {
	for i := 0; i < 8; i++ {
		d := from.AddDate(0, 0, i)
		if strings.EqualFold(d.Weekday().String(), weekday) {
			return d.Format("2006-01-02")
		}
	}
	if strings.EqualFold(weekday, "tonight") || strings.EqualFold(weekday, "today") {
		return from.Format("2006-01-02")
	}
	return ""
}

type ecccAlert struct {
	ID       string          `json:"id"`
	Geometry json.RawMessage `json:"geometry"`
	P        struct {
		Type        string `json:"alert_type"`
		NameEn      string `json:"alert_name_en"`
		NameFr      string `json:"alert_name_fr"`
		TextEn      string `json:"alert_text_en"`
		TextFr      string `json:"alert_text_fr"`
		AreaEn      string `json:"feature_name_en"`
		AreaFr      string `json:"feature_name_fr"`
		Status      string `json:"status_en"`
		Published   string `json:"publication_datetime"`
		Valid       string `json:"validity_datetime"`
		Expires     string `json:"expiration_datetime"`
		EventEnd    string `json:"event_end_datetime"`
		ShortNameEn string `json:"alert_short_name_en"`
	} `json:"properties"`
}

func (e *ECCC) Alerts(ctx context.Context, t Town, now time.Time) ([]Alert, error) {
	var found geoCollection[ecccAlert]
	u := e.Base + "/collections/weather-alerts/items?f=json&limit=200&bbox=" + url.QueryEscape(box(t.Lat, t.Lon, 0.02))
	if _, err := e.F.getJSON(ctx, u, "application/geo+json", &found); err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	alerts := []Alert{}
	for _, a := range found.Features {
		switch strings.ToLower(a.P.Status) {
		case "ended", "cancelled", "expired", "terminé":
			continue
		}
		if !holds(a.Geometry, t.Lat, t.Lon) {
			continue
		}
		// One alert covers many areas, each its own feature: id_area.
		id, _, _ := strings.Cut(a.ID, "_")
		if seen[id] {
			continue
		}
		ends := a.P.EventEnd
		if ends == "" {
			ends = a.P.Expires
		}
		if end, err := time.Parse(time.RFC3339, ends); err == nil && !end.After(now) {
			continue
		}
		seen[id] = true
		alerts = append(alerts, Alert{
			ID: id, Kind: kind(a.P.Type),
			Event:    Words{capital(a.P.NameEn), capital(a.P.NameFr)},
			Headline: Words{capital(a.P.NameEn), capital(a.P.NameFr)},
			Text:     Words{a.P.TextEn, a.P.TextFr},
			Area:     Words{a.P.AreaEn, a.P.AreaFr},
			Issued:   a.P.Published, Starts: a.P.Valid, Ends: ends,
		})
	}
	return alerts, nil
}

func capital(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	return strings.ToUpper(string(r[0])) + string(r[1:])
}

// kind: warning, watch, advisory or statement, from the service's word.
func kind(words string) string {
	w := strings.ToLower(words)
	for _, k := range []string{"warning", "watch", "advisory", "statement"} {
		if strings.Contains(w, k) {
			return k
		}
	}
	return "statement"
}
