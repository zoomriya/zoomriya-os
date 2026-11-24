package main

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// NWS: the US National Weather Service (api.weather.gov), for a box in the
// US: the forecast for the town's point, the latest observation at its
// nearest station, and the active alerts for the point. Public domain; in
// English only, so each Words holds the same English twice.
type NWS struct {
	F    *Fetcher
	Base string // https://api.weather.gov

	mu     sync.Mutex
	points map[int64]nwsPoint
}

type nwsPoint struct {
	Forecast string
	Stations string
}

const nwsAccept = "application/geo+json"

var nwsCredit = Words{"From the National Weather Service.", "Du National Weather Service des États-Unis."}

// point: the forecast and station links for the town, asked once.
func (n *NWS) point(ctx context.Context, t Town) (nwsPoint, error) {
	n.mu.Lock()
	found, ok := n.points[t.ID]
	n.mu.Unlock()
	if ok {
		return found, nil
	}
	var p struct {
		Properties struct {
			Forecast            string `json:"forecast"`
			ObservationStations string `json:"observationStations"`
		} `json:"properties"`
	}
	if _, err := n.F.getJSON(ctx, fmt.Sprintf("%s/points/%.2f,%.2f", n.Base, t.Lat, t.Lon), nwsAccept, &p); err != nil {
		return nwsPoint{}, err
	}
	if p.Properties.Forecast == "" {
		return nwsPoint{}, fmt.Errorf("the National Weather Service has no forecast for the town")
	}
	found = nwsPoint{p.Properties.Forecast, p.Properties.ObservationStations}
	n.mu.Lock()
	if n.points == nil {
		n.points = map[int64]nwsPoint{}
	}
	n.points[t.ID] = found
	n.mu.Unlock()
	return found, nil
}

func (n *NWS) Forecast(ctx context.Context, t Town, _ string) (Forecast, error) {
	p, err := n.point(ctx, t)
	if err != nil {
		return Forecast{}, err
	}
	var fc struct {
		Properties struct {
			Periods []struct {
				Name     string  `json:"name"`
				Start    string  `json:"startTime"`
				Daytime  bool    `json:"isDaytime"`
				Temp     float64 `json:"temperature"`
				TempUnit string  `json:"temperatureUnit"`
				Short    string  `json:"shortForecast"`
				Detailed string  `json:"detailedForecast"`
				Precip   struct {
					Value *float64 `json:"value"`
				} `json:"probabilityOfPrecipitation"`
			} `json:"periods"`
		} `json:"properties"`
	}
	a, err := n.F.getJSON(ctx, p.Forecast, nwsAccept, &fc)
	if err != nil {
		return Forecast{}, err
	}
	f := Forecast{Unit: "F", Expires: a.Expires,
		Official:     same(fmt.Sprintf("https://forecast.weather.gov/MapClick.php?lat=%.2f&lon=%.2f", t.Lat, t.Lon)),
		OfficialName: Words{"the National Weather Service's alerts", "les alertes du National Weather Service"}}
	byDate := map[string]*Day{}
	var order []string
	for _, period := range fc.Properties.Periods {
		start, err := time.Parse(time.RFC3339, period.Start)
		if err != nil {
			continue
		}
		temp := period.Temp
		if strings.EqualFold(period.TempUnit, "C") {
			temp = toF(temp)
		}
		date := start.Format("2006-01-02")
		d := byDate[date]
		if d == nil {
			d = &Day{Date: date}
			byDate[date] = d
			order = append(order, date)
		}
		if period.Daytime {
			d.High = ptr(temp)
			d.Text = same(period.Detailed)
			d.Condition, d.Icon, d.Name = same(period.Short), iconFor(period.Short), same(period.Name)
		} else {
			d.Low = ptr(temp)
			d.Night = same(period.Detailed)
			if d.Condition.En == "" {
				d.Condition, d.Icon, d.Name = same(period.Short), iconFor(period.Short), same(period.Name)
			}
		}
		if period.Precip.Value != nil && (d.Chance == nil || int(*period.Precip.Value) > *d.Chance) {
			d.Chance = ptr(int(*period.Precip.Value))
		}
	}
	for _, date := range order {
		f.Days = append(f.Days, *byDate[date])
	}
	if len(f.Days) == 0 {
		return Forecast{}, fmt.Errorf("the National Weather Service's forecast had no days")
	}
	f.Now = n.observed(ctx, p)
	return f, nil
}

// observed: the nearest station's latest observation, in °F; nil when there
// is none, or it's over two hours old.
func (n *NWS) observed(ctx context.Context, p nwsPoint) *Now {
	if p.Stations == "" {
		return nil
	}
	var stations struct {
		Features []struct {
			Properties struct {
				ID   string `json:"stationIdentifier"`
				Name string `json:"name"`
			} `json:"properties"`
		} `json:"features"`
	}
	if _, err := n.F.getJSON(ctx, p.Stations, nwsAccept, &stations); err != nil || len(stations.Features) == 0 {
		return nil
	}
	station := stations.Features[0].Properties
	var obs struct {
		Properties struct {
			Timestamp   string `json:"timestamp"`
			Description string `json:"textDescription"`
			Temperature struct {
				Unit  string   `json:"unitCode"`
				Value *float64 `json:"value"`
			} `json:"temperature"`
		} `json:"properties"`
	}
	if _, err := n.F.getJSON(ctx, n.Base+"/stations/"+station.ID+"/observations/latest", nwsAccept, &obs); err != nil {
		return nil
	}
	o := obs.Properties
	when, err := time.Parse(time.RFC3339, o.Timestamp)
	if err != nil || o.Temperature.Value == nil || time.Since(when) > 2*time.Hour {
		return nil
	}
	temp := *o.Temperature.Value
	if strings.HasSuffix(o.Temperature.Unit, "degC") {
		temp = toF(temp)
	}
	return &Now{Time: o.Timestamp, Temp: ptr(round1(temp)), Condition: same(o.Description),
		Icon: iconFor(o.Description), Station: same(station.Name)}
}

func (n *NWS) Alerts(ctx context.Context, t Town, now time.Time) ([]Alert, error) {
	var found struct {
		Features []struct {
			Properties struct {
				ID          string `json:"id"`
				Event       string `json:"event"`
				Headline    string `json:"headline"`
				Description string `json:"description"`
				Instruction string `json:"instruction"`
				Area        string `json:"areaDesc"`
				Sent        string `json:"sent"`
				Effective   string `json:"effective"`
				Onset       string `json:"onset"`
				Ends        string `json:"ends"`
				Expires     string `json:"expires"`
				Status      string `json:"status"`
				MessageType string `json:"messageType"`
			} `json:"properties"`
		} `json:"features"`
	}
	u := fmt.Sprintf("%s/alerts/active?point=%.2f,%.2f", n.Base, t.Lat, t.Lon)
	if _, err := n.F.getJSON(ctx, u, nwsAccept, &found); err != nil {
		return nil, err
	}
	alerts := []Alert{}
	for _, f := range found.Features {
		a := f.Properties
		if a.Status != "Actual" || a.MessageType == "Cancel" {
			continue
		}
		ends := a.Ends
		if ends == "" {
			ends = a.Expires
		}
		if end, err := time.Parse(time.RFC3339, ends); err == nil && !end.After(now) {
			continue
		}
		starts := a.Onset
		if starts == "" {
			starts = a.Effective
		}
		alerts = append(alerts, Alert{
			ID: a.ID, Kind: kind(a.Event), Event: same(a.Event), Headline: same(a.Headline),
			Text: same(a.Description), Instruction: same(a.Instruction), Area: same(a.Area),
			Issued: a.Sent, Starts: starts, Ends: ends,
		})
	}
	return alerts, nil
}
